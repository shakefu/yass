{-# LANGUAGE OverloadedStrings #-}

module Yass.Query.InlineConforms
  ( inlineConforms
  , InlineError(..)
  -- * Exported for testing
  , parseConformsTarget
  , collapseDots
  , applyGuardCombination
  ) where

import Data.Text (Text)
import qualified Data.Text as T
import System.Directory (doesFileExist)
import System.FilePath ((</>), takeDirectory, normalise, splitDirectories, joinPath)
import Yass.Parser (RawDocument(..), RawValue(..), ParseResult(..), parseYassFile)
import Yass.Types (ErrorCode(..))

-- | Error during CONFORMS inlining
data InlineError = InlineError
  { ieCode    :: !ErrorCode
  , ieTarget  :: !Text
  , ieMessage :: !Text
  } deriving (Show, Eq)

-- | Inline CONFORMS references in a spec document.
-- Takes the project root, the directory of the file containing the spec,
-- the full list of documents in the file (for same-file ref resolution),
-- and the spec document to process.
inlineConforms :: FilePath -> FilePath -> [RawDocument] -> RawDocument -> IO (Either [InlineError] RawDocument)
inlineConforms projectRoot fileDir allDocs doc = do
  let entries = rawDocEntries doc
  result <- mapM (processEntry projectRoot fileDir allDocs) entries
  let (errors, newEntries) = partitionResults result
  if null errors
    then return $ Right $ doc { rawDocEntries = concat newEntries }
    else return $ Left (concat errors)

-- | Process a single top-level entry in the spec document.
processEntry :: FilePath -> FilePath -> [RawDocument]
             -> (Text, RawValue, Int)
             -> IO (Either [InlineError] [(Text, RawValue, Int)])
processEntry projectRoot fileDir allDocs (key, val, line)
  | isSlotKey key = case val of
      RVList items listLine -> do
        result <- processObligations projectRoot fileDir allDocs items
        case result of
          Left errs -> return $ Left errs
          Right newItems -> return $ Right [(key, RVList newItems listLine, line)]
      _ -> return $ Right [(key, val, line)]
  | otherwise = return $ Right [(key, val, line)]

-- | Process obligations in a slot, inlining CONFORMS refs.
processObligations :: FilePath -> FilePath -> [RawDocument] -> [RawValue]
                   -> IO (Either [InlineError] [RawValue])
processObligations projectRoot fileDir allDocs items = do
  results <- mapM (processObligation projectRoot fileDir allDocs) items
  let (errors, processed) = partitionResults results
  if null errors
    then return $ Right (concat processed)
    else return $ Left (concat errors)

-- | Process a single obligation, potentially inlining CONFORMS.
processObligation :: FilePath -> FilePath -> [RawDocument] -> RawValue
                  -> IO (Either [InlineError] [RawValue])
processObligation projectRoot fileDir allDocs (RVMapping entries mapLine) = do
  let conformsRefs = [(k, v, ln) | (k, v, ln) <- entries, k == "CONFORMS"]
      hasNormativity = any (\(k, _, _) -> isNormativityKey k) entries
      guardProse = case [(v, _ln) | ("WHEN", v, _ln) <- entries] of
                     ((RVString g, _):_) -> Just g
                     _ -> Nothing

  case conformsRefs of
    [] -> return $ Right [RVMapping entries mapLine]
    ((_, RVString target, _refLine):_) -> do
      -- Parse the ref target to check for ::SLOT
      case parseConformsTarget target of
        Left err -> return $ Left [err]
        Right (mFilePath, specName, slotName) -> do
          inlineResult <- resolveAndInline projectRoot fileDir allDocs mFilePath specName slotName target
          case inlineResult of
            Left errs -> return $ Left errs
            Right inlinedObligations -> do
              let strippedEntries = filter (\(k, _, _) -> k /= "CONFORMS") entries
                  inlined = map (applyGuardCombination guardProse) inlinedObligations
                  -- Provenance comment for inlined obligations
                  provComment = RVComment ("CONFORMS: " <> target)
                  -- Non-CONFORMS reference keys on the carrier
                  carrierRefs = [(k, v, ln) | (k, v, ln) <- strippedEntries
                                , isRelationKey k]
              if hasNormativity
                then do
                  let carrier = RVMapping strippedEntries mapLine
                  return $ Right (carrier : provComment : inlined)
                else
                  -- Reference-only: carrier is discarded, but preserve
                  -- any non-CONFORMS refs (USES, SEE) by attaching them
                  -- to the first inlined obligation.
                  let withRefs = case (carrierRefs, inlined) of
                        ([], obs) -> obs
                        (refs, (RVMapping es ml):rest) ->
                          RVMapping (es ++ refs) ml : rest
                        (_, obs) -> obs
                  in return $ Right (provComment : withRefs)
    _ -> return $ Right [RVMapping entries mapLine]
processObligation _ _ _ other = return $ Right [other]

-- | Parse a CONFORMS target: must have ::SLOT suffix
parseConformsTarget :: Text -> Either InlineError (Maybe Text, Text, Text)
parseConformsTarget target =
  let (beforeSlot, afterSlot) = case T.breakOn "::" target of
        (b, r) | not (T.null r) -> (b, T.drop 2 r)
        _ -> (target, "")
  in if T.null afterSlot
     then Left $ InlineError QueryConformsNoSlot target
            ("CONFORMS ref must address a slot in v1: " <> target)
     else
       let (filePath, specName) = case T.breakOn "@" beforeSlot of
             (b, r) | not (T.null r) -> (Just b, T.drop 1 r)
             _ -> (Nothing, beforeSlot)
       in Right (filePath, specName, afterSlot)

-- | Resolve a CONFORMS ref and return the obligations from the target slot.
resolveAndInline :: FilePath -> FilePath -> [RawDocument] -> Maybe Text -> Text -> Text -> Text
                 -> IO (Either [InlineError] [RawValue])
resolveAndInline _projectRoot _fileDir allDocs Nothing specName slotName target = do
  -- Same-file reference: look up in allDocs
  let specDocs = case allDocs of
        (_:rest) -> rest  -- skip preamble
        [] -> []
  case findSpecByName specName specDocs of
    Nothing -> return $ Left [InlineError QueryConformsUnresolved target
                             ("unresolvable CONFORMS ref: " <> target)]
    Just specDoc ->
      case findSlotObligations slotName specDoc of
        Nothing -> return $ Left [InlineError QueryConformsUnresolved target
                                 ("unresolvable CONFORMS ref: " <> target)]
        Just obligations -> return $ Right obligations

resolveAndInline projectRoot fileDir _allDocs (Just pathToken) specName slotName target = do
  -- Cross-file reference
  let pathStr = T.unpack pathToken
      fullPath = if "./" `T.isPrefixOf` pathToken || "../" `T.isPrefixOf` pathToken
                 then collapseDots (fileDir </> pathStr <> ".yass.yaml")
                 else collapseDots (projectRoot </> pathStr <> ".yass.yaml")
  exists <- doesFileExist fullPath
  if not exists
    then return $ Left [InlineError QueryConformsUnresolved target
                         ("unresolvable CONFORMS ref: " <> target)]
    else do
      parseResult <- parseYassFile fullPath
      case parseResult of
        ParseError _ _ -> return $ Left [InlineError QueryConformsUnresolved target
                                        ("unresolvable CONFORMS ref: " <> target)]
        ParseOK docs -> case docs of
          [] -> return $ Left [InlineError QueryConformsUnresolved target
                              ("unresolvable CONFORMS ref: " <> target)]
          (_:specDocs) ->
            case findSpecByName specName specDocs of
              Nothing -> return $ Left [InlineError QueryConformsUnresolved target
                                       ("unresolvable CONFORMS ref: " <> target)]
              Just specDoc ->
                case findSlotObligations slotName specDoc of
                  Nothing -> return $ Left [InlineError QueryConformsUnresolved target
                                           ("unresolvable CONFORMS ref: " <> target)]
                  Just obligations -> return $ Right obligations

-- | Collapse ".." and "." in paths
collapseDots :: FilePath -> FilePath
collapseDots path =
  let parts = splitDirectories (normalise path)
      collapsed = foldl go [] parts
  in if null collapsed then "/" else joinPath (reverse collapsed)
  where
    go acc "."  = acc
    go (_:rest) ".." = rest
    go [] ".." = []
    go acc p = p : acc

-- | Find a spec document by name
findSpecByName :: Text -> [RawDocument] -> Maybe RawDocument
findSpecByName _ [] = Nothing
findSpecByName name (doc:rest) =
  case lookup3 "spec" (rawDocEntries doc) of
    Just (RVString n, _) | n == name -> Just doc
    _ -> findSpecByName name rest

-- | Find the obligations for a slot in a spec document
findSlotObligations :: Text -> RawDocument -> Maybe [RawValue]
findSlotObligations slotName doc =
  case lookup3 slotName (rawDocEntries doc) of
    Just (RVList items _, _) -> Just items
    _ -> Nothing

-- | Apply guard combination when inlining
applyGuardCombination :: Maybe Text -> RawValue -> RawValue
applyGuardCombination Nothing val = val
applyGuardCombination (Just outerGuard) (RVMapping entries line) =
  let hasWhen = any (\(k, _, _) -> k == "WHEN") entries
  in if hasWhen
     then RVMapping (map (combineGuard outerGuard) entries) line
     else RVMapping (entries ++ [("WHEN", RVString outerGuard, 0)]) line
applyGuardCombination _ val = val

-- | Combine an outer WHEN guard with an entry's WHEN guard
combineGuard :: Text -> (Text, RawValue, Int) -> (Text, RawValue, Int)
combineGuard outerGuard ("WHEN", RVString innerGuard, ln) =
  ("WHEN", RVString (outerGuard <> " and " <> innerGuard), ln)
combineGuard _ entry = entry

-- | Check if a key is a slot key
isSlotKey :: Text -> Bool
isSlotKey k = k `elem` ["INPUT", "RETURN", "ERROR", "SIDE-EFFECT", "INVARIANT"]

-- | Check if a key is a normativity key
isNormativityKey :: Text -> Bool
isNormativityKey k = k `elem` ["MUST", "MUST-NOT", "SHOULD", "SHOULD-NOT", "MAY"]

-- | Check if a key is a relation/reference key (CONFORMS, USES, SEE)
isRelationKey :: Text -> Bool
isRelationKey k = k `elem` ["CONFORMS", "USES", "SEE"]

-- | Partition results into errors and successes
partitionResults :: [Either [a] [b]] -> ([[a]], [[b]])
partitionResults [] = ([], [])
partitionResults (Left e : rest) = let (es, ss) = partitionResults rest in (e:es, ss)
partitionResults (Right s : rest) = let (es, ss) = partitionResults rest in (es, s:ss)

-- | Lookup helper
lookup3 :: Text -> [(Text, RawValue, Int)] -> Maybe (RawValue, Int)
lookup3 _ [] = Nothing
lookup3 k ((k', v, ln):rest)
  | k == k'   = Just (v, ln)
  | otherwise  = lookup3 k rest
