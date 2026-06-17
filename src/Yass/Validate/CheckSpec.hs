{-# LANGUAGE OverloadedStrings #-}

module Yass.Validate.CheckSpec (checkSpec) where

import Data.Text (Text)
import qualified Data.Text as T
import qualified Data.Set as Set
import Yass.Types
  ( YassError(..)
  , ErrorCode(..)
  , validSpecNameChars
  , isSlotKeyword
  , isNormativityKeyword
  , isReservedKeyword
  , isRelationKeyword
  , isGuardKeyword
  )
import Yass.Parser (RawDocument(..), RawValue(..))

-- | Check spec document structure.
-- Takes cwd, file path, and a single parsed spec document.
-- Returns list of errors (one per failing rule per obligation).
checkSpec :: FilePath -> FilePath -> RawDocument -> [YassError]
checkSpec _cwd fp doc =
  let entries = rawDocEntries doc
      docLine = rawDocLine doc
  in case lookupEntry "spec" entries of
       Nothing ->
         [mkErr fp docLine SpecNoName "spec document missing spec key"]
       Just (val, line) ->
         let nameErrors = checkSpecName fp val line
         in if not (null nameErrors)
            then nameErrors
            else checkKeys fp entries
                 ++ checkSlots fp entries

-- | Look up an entry by key name.
lookupEntry :: Text -> [(Text, RawValue, Int)] -> Maybe (RawValue, Int)
lookupEntry _ [] = Nothing
lookupEntry k ((k', v, ln):rest)
  | k == k'   = Just (v, ln)
  | otherwise  = lookupEntry k rest

-- | Validate the spec name value.
checkSpecName :: FilePath -> RawValue -> Int -> [YassError]
checkSpecName fp val line = case val of
  RVString name
    | T.null name ->
        [mkErr fp line SpecNameEmpty "spec name is empty"]
    | not (T.all (`Set.member` validSpecNameChars) name) ->
        [mkErr fp line SpecNameBadChars
          ("spec name contains disallowed characters: " <> name)]
    | not (isValidSpecNameForm name) ->
        [mkErr fp line SpecNameBadForm
          ("spec name is malformed: " <> name)]
    | isReservedKeyword name ->
        [mkErr fp line SpecNameReserved
          ("spec name collides with a reserved keyword: " <> name)]
    | otherwise -> []
  _ ->
    [mkErr fp line SpecNameNotString "spec name must be a string"]

-- | Check if a spec name has valid form:
-- Must match ^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$
-- No leading/trailing dot, no consecutive dots.
isValidSpecNameForm :: Text -> Bool
isValidSpecNameForm name
  | T.null name = False
  | T.head name == '.' = False
  | T.last name == '.' = False
  | T.isInfixOf ".." name = False
  | otherwise =
      let segments = T.splitOn "." name
      in all (\s -> not (T.null s) && T.all isSegmentChar s) segments
  where
    segChars = Set.fromList $ ['A'..'Z'] ++ ['a'..'z'] ++ ['0'..'9'] ++ ['_', '-']
    isSegmentChar c = Set.member c segChars

-- | Check for unknown keys (not "spec" and not a valid slot keyword).
checkKeys :: FilePath -> [(Text, RawValue, Int)] -> [YassError]
checkKeys fp entries = concatMap checkKey entries
  where
    checkKey (k, _, ln)
      | k == "spec"     = []
      | isSlotKeyword k = []
      | otherwise       = [mkErr fp ln SpecUnknownKey
                            ("unknown spec key: " <> k)]

-- | Check all slot entries for validity.
checkSlots :: FilePath -> [(Text, RawValue, Int)] -> [YassError]
checkSlots fp entries = concatMap checkSlot entries
  where
    checkSlot (k, v, ln)
      | isSlotKeyword k = checkSlotValue fp k v ln
      | otherwise       = []

-- | Check that a slot value is a list, then check each obligation.
checkSlotValue :: FilePath -> Text -> RawValue -> Int -> [YassError]
checkSlotValue fp slotKey val line = case val of
  RVList items _ -> concatMap (checkObligationItem fp) items
  _              -> [mkErr fp line SlotValueNotList ("slot value must be a list: " <> slotKey)]

-- | Check a single obligation item from a slot list.
checkObligationItem :: FilePath -> RawValue -> [YassError]
checkObligationItem fp val = case val of
  RVMapping entries mapLine ->
    checkObligationEntries fp entries mapLine
  _ ->
    [mkErr fp (rawValueLine val) ObligationBadValueShape
      "obligation value must be a quoted scalar"]

-- | Get the line number from a RawValue (best effort).
rawValueLine :: RawValue -> Int
rawValueLine (RVList _ ln)    = ln
rawValueLine (RVMapping _ ln) = ln
rawValueLine _                = 0

-- | Check entries within an obligation mapping.
checkObligationEntries :: FilePath -> [(Text, RawValue, Int)] -> Int -> [YassError]
checkObligationEntries fp entries oblLine =
  let
    -- Classify keys
    normKeys  = [(k, v, ln) | (k, v, ln) <- entries, isNormativityKeyword k]
    refKeys   = [(k, v, ln) | (k, v, ln) <- entries, isRelationKeyword k]
    guardKeys = [(k, ln)    | (k, _, ln) <- entries, isGuardKeyword k]

    -- Unknown keys: not normativity, not relation, not guard
    unknownKeys = [(k, ln) | (k, _, ln) <- entries
                            , not (isNormativityKeyword k)
                            , not (isRelationKeyword k)
                            , not (isGuardKeyword k)]

    hasNorm = not (null normKeys)
    hasRef  = not (null refKeys)

    -- Rule 10: bad value shapes (null, mapping, sequence values)
    shapeErrors = concatMap (checkValueShape fp) entries

    -- Rule 15: unknown normativity keyword
    normUnknownErrors = [mkErr fp ln NormativityUnknown
                          ("unknown Normativity keyword: " <> k)
                        | (k, ln) <- unknownKeys]

    -- Rule 11: missing normativity or reference
    missingErrors
      | not hasNorm && not hasRef =
          [mkErr fp oblLine ObligationMissingNormativityOrRef
            "obligation must carry a Normativity keyword or a Reference"]
      | otherwise = []

    -- Rule 12: WHEN guard without normativity
    guardErrors
      | not (null guardKeys) && not hasNorm =
          [mkErr fp (snd (head guardKeys)) ObligationGuardWithoutNormativity
            "WHEN guard requires a Normativity keyword"]
      | otherwise = []

    -- Rule 13: duplicate reference relation
    dupRefErrors = findDuplicates fp ObligationDuplicateReference
                     "duplicate Reference relation in obligation" refKeys

    -- Rule 14: duplicate normativity keyword
    dupNormErrors = if length normKeys > 1
                      then [mkErr fp (let (_, _, ln) = normKeys !! 1 in ln)
                              ObligationDuplicateNormativity
                              "duplicate Normativity keyword in obligation"]
                      else []

  in shapeErrors ++ normUnknownErrors ++ missingErrors ++ guardErrors
     ++ dupRefErrors ++ dupNormErrors

-- | Check for bad value shapes in an obligation entry.
-- Obligation scalar values must not be null, mapping, or sequence.
checkValueShape :: FilePath -> (Text, RawValue, Int) -> [YassError]
checkValueShape fp (k, v, ln) = case v of
  RVNull      -> [badShape]
  RVMapping _ _ -> [badShape]
  RVList _ _  -> [badShape]
  _           -> []
  where
    badShape = mkErr fp ln ObligationBadValueShape
      "obligation value must be a quoted scalar"

-- | Find duplicate keys in a list of (key, value, line) triples.
findDuplicates :: FilePath -> ErrorCode -> Text
               -> [(Text, RawValue, Int)] -> [YassError]
findDuplicates fp code prefix items = go items Set.empty
  where
    go [] _ = []
    go ((k, _, ln):rest) seen
      | Set.member k seen =
          mkErr fp ln code (prefix <> ": " <> k) : go rest seen
      | otherwise = go rest (Set.insert k seen)

-- | Create a YassError.
mkErr :: FilePath -> Int -> ErrorCode -> Text -> YassError
mkErr fp line code msg = YassError
  { yeFile    = if null fp then Nothing else Just fp
  , yeLine    = if line > 0 then Just line else Nothing
  , yeCode    = code
  , yeMessage = msg
  }
