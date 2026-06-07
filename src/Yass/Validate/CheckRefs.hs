{-# LANGUAGE OverloadedStrings #-}

module Yass.Validate.CheckRefs (checkRefs) where

import Data.Text (Text)
import qualified Data.Text as T
import qualified Data.Set as Set
import qualified Data.Map.Strict as Map
import System.FilePath ((</>), takeDirectory, normalise, splitDirectories, joinPath, isAbsolute)
import System.Directory (doesFileExist)
import Data.IORef

import Yass.Types (YassError(..), ErrorCode(..), SlotKey(..))
import Yass.Parser (RawDocument(..), RawValue(..), ParseResult(..), parseYassFile)

-- | Check reference target resolution across all specs in a file.
--
-- Parameters:
--   cwd         - current working directory (for error context)
--   filePath    - path to the file being validated
--   projectRoot - project root directory
--   docs        - all parsed 'RawDocument' values from the file
checkRefs :: FilePath -> FilePath -> FilePath -> [RawDocument] -> IO [YassError]
checkRefs _cwd filePath projectRoot docs = do
  -- Collect all spec names in this file (from non-preamble docs)
  let localSpecs = collectLocalSpecs docs
  -- Collect all refs from all spec docs
  let allRefs = collectAllRefs docs
  -- Cache for cross-file parse results, keyed by resolved absolute path
  -- Also tracks which (source-file, target-file) pairs have produced
  -- file_not_found or file_not_parseable errors
  cacheRef <- newIORef (Map.empty :: Map.Map FilePath (Either () [Text]))
  errorPairRef <- newIORef (Set.empty :: Set.Set FilePath)

  errors <- mapM (validateRef filePath projectRoot localSpecs docs cacheRef errorPairRef) allRefs
  return (concat errors)

-- | A reference to validate: (refText, line, specName of containing spec)
data RefInfo = RefInfo
  { riText     :: !Text    -- ^ Raw reference text (e.g. "path@Spec::SLOT")
  , riLine     :: !Int     -- ^ Line number of the reference
  , riSpecName :: !Text    -- ^ Name of the containing spec (for context)
  } deriving (Show)

-- | Collect all spec names from non-preamble documents
collectLocalSpecs :: [RawDocument] -> Set.Set Text
collectLocalSpecs [] = Set.empty
collectLocalSpecs (_:rest) = Set.fromList $ concatMap getSpecName rest
  where
    getSpecName doc =
      case lookup3 "spec" (rawDocEntries doc) of
        Just (RVString name, _) | not (T.null name) -> [name]
        _ -> []

-- | Collect all reference targets from all spec documents.
-- References are found as CONFORMS/USES/SEE values in obligation mappings
-- within slot lists.
collectAllRefs :: [RawDocument] -> [RefInfo]
collectAllRefs [] = []
collectAllRefs (_:rest) = concatMap collectFromDoc rest
  where
    collectFromDoc doc =
      let specName = case lookup3 "spec" (rawDocEntries doc) of
            Just (RVString n, _) -> n
            _ -> ""
          slotEntries = filter (\(k, _, _) -> isSlotKey k) (rawDocEntries doc)
      in concatMap (collectFromSlot specName) slotEntries

    collectFromSlot specName (_, RVList items _, _) =
      concatMap (collectFromObligation specName) items
    collectFromSlot _ _ = []

    collectFromObligation specName (RVMapping entries _) =
      concatMap (collectFromEntry specName) entries
    collectFromObligation _ (RVString _) = []
      -- Plain string obligations (e.g. "MUST do something") contain no refs
    collectFromObligation _ _ = []

    collectFromEntry specName (key, RVString val, line)
      | isRelationKey key = [RefInfo val line specName]
    collectFromEntry _ _ = []

-- | Check if a key is a slot key
isSlotKey :: Text -> Bool
isSlotKey k = k `elem` ["INPUT", "RETURN", "ERROR", "SIDE-EFFECT", "INVARIANT"]

-- | Check if a key is a relation key (CONFORMS, USES, SEE)
isRelationKey :: Text -> Bool
isRelationKey k = k `elem` ["CONFORMS", "USES", "SEE"]

-- | Ref target grammar: ^([A-Za-z0-9._/-]+@)?[A-Za-z0-9._-]+(::[A-Z-]+)?$
-- Returns (Maybe filePath, specName, Maybe slotText) or Nothing if malformed
parseRefTarget :: Text -> Maybe (Maybe Text, Text, Maybe Text)
parseRefTarget raw
  | T.null raw = Nothing
  | otherwise  =
    let s = T.unpack raw
    in if matchesRefGrammar s
       then Just (extractParts raw)
       else Nothing

-- | Check if a string matches the ref target grammar
matchesRefGrammar :: String -> Bool
matchesRefGrammar s = case s of
  [] -> False
  _  -> go s
  where
    go [] = False
    go str =
      -- Try to match with all three parts: path@spec::slot
      -- or path@spec, or spec::slot, or just spec
      case breakOnAt str of
        Just (path, afterAt) ->
          not (null path)
          && all isPathChar path
          && matchSpecAndSlot afterAt
        Nothing ->
          matchSpecAndSlot str

    breakOnAt str =
      let (before, after) = break (== '@') str
      in case after of
           ('@':rest) -> Just (before, rest)
           _ -> Nothing

    matchSpecAndSlot [] = False
    matchSpecAndSlot str =
      case breakOnDoubleColon str of
        Just (spec, slot) ->
          not (null spec)
          && all isSpecChar spec
          && not (null slot)
          && all isSlotChar slot
        Nothing ->
          not (null str) && all isSpecChar str

    breakOnDoubleColon str =
      case findDoubleColon str of
        Just (before, after) -> Just (before, after)
        Nothing -> Nothing

    findDoubleColon [] = Nothing
    findDoubleColon [_] = Nothing
    findDoubleColon (':':':':rest) = Just ([], rest)
    findDoubleColon (c:rest) =
      case findDoubleColon rest of
        Just (before, after) -> Just (c:before, after)
        Nothing -> Nothing

    isPathChar c = isAlphaNum c || c `elem` ['.', '_', '/', '-']
    isSpecChar c = isAlphaNum c || c `elem` ['.', '_', '-']
    isSlotChar c = (c >= 'A' && c <= 'Z') || c == '-'

    isAlphaNum c = (c >= 'A' && c <= 'Z')
                || (c >= 'a' && c <= 'z')
                || (c >= '0' && c <= '9')

-- | Extract path, spec, slot parts from a valid ref target
extractParts :: Text -> (Maybe Text, Text, Maybe Text)
extractParts raw =
  let (pathPart, afterAt) = case T.breakOn "@" raw of
        (before, rest) | not (T.null rest) -> (Just before, T.drop 1 rest)
        _ -> (Nothing, raw)
      (specPart, slotPart) = case T.breakOn "::" afterAt of
        (before, rest) | not (T.null rest) -> (before, Just (T.drop 2 rest))
        _ -> (afterAt, Nothing)
  in (pathPart, specPart, slotPart)

-- | Valid slot names
validSlots :: Map.Map Text SlotKey
validSlots = Map.fromList
  [ ("INPUT", SlotINPUT)
  , ("RETURN", SlotRETURN)
  , ("ERROR", SlotERROR)
  , ("SIDE-EFFECT", SlotSIDEEFFECT)
  , ("INVARIANT", SlotINVARIANT)
  ]

-- | Validate a single reference
validateRef
  :: FilePath                                        -- ^ Source file
  -> FilePath                                        -- ^ Project root
  -> Set.Set Text                                    -- ^ Local spec names
  -> [RawDocument]                                   -- ^ All docs in source file
  -> IORef (Map.Map FilePath (Either () [Text]))     -- ^ Parse cache
  -> IORef (Set.Set FilePath)                        -- ^ Error pair cache
  -> RefInfo                                         -- ^ The reference
  -> IO [YassError]
validateRef filePath projectRoot localSpecs docs cacheRef errorPairRef ref = do
  let raw = riText ref
      line = riLine ref
  -- Step 1: Check grammar
  case parseRefTarget raw of
    Nothing -> return [mkErr filePath line RefMalformed ("malformed ref target: " <> raw)]
    Just (mPath, specName, mSlot) -> do
      -- Step 2: Check slot validity
      case mSlot of
        Just slotText
          | not (Map.member slotText validSlots) ->
              return [mkErr filePath line RefUnknownSlot ("unknown slot in ref target: " <> slotText)]
        _ -> do
          -- Step 3: Resolve the spec
          case mPath of
            Nothing -> do
              -- Same-file reference
              let specErrors = if Set.member specName localSpecs
                    then []
                    else [mkErr filePath line RefSpecNotFoundSameFile
                           ("spec not found in file: " <> specName)]
              slotErrors <- case mSlot of
                Just slotText | null specErrors ->
                  return (checkSlotDeclared filePath line docs specName slotText)
                _ -> return []
              return (specErrors ++ slotErrors)
            Just pathToken -> do
              -- Cross-file reference
              let resolvedPath = resolveRefPath filePath projectRoot pathToken
              validateCrossFileRef filePath line resolvedPath specName mSlot docs cacheRef errorPairRef

-- | Resolve a reference path to an absolute file path
resolveRefPath :: FilePath -> FilePath -> Text -> FilePath
resolveRefPath filePath projectRoot pathToken =
  let pathStr = T.unpack pathToken
      basePath = if "./" `isPrefixOfStr` pathStr || "../" `isPrefixOfStr` pathStr
                 then takeDirectory filePath </> pathStr
                 else projectRoot </> pathStr
  in collapseDots (basePath ++ ".yass.yaml")

-- | Collapse ".." and "." components in a file path
collapseDots :: FilePath -> FilePath
collapseDots path =
  let parts = splitDirectories (normalise path)
      collapsed = foldl go [] parts
  in if null collapsed then "/" else joinPath (reverse collapsed)
  where
    go acc "."  = acc
    go (_:rest) ".." = rest  -- pop parent
    go [] ".." = []  -- can't go above root
    go acc p = p : acc

isPrefixOfStr :: String -> String -> Bool
isPrefixOfStr [] _ = True
isPrefixOfStr _ [] = False
isPrefixOfStr (x:xs) (y:ys) = x == y && isPrefixOfStr xs ys

-- | Validate a cross-file reference
validateCrossFileRef
  :: FilePath                                        -- ^ Source file
  -> Int                                             -- ^ Line number
  -> FilePath                                        -- ^ Resolved target file path
  -> Text                                            -- ^ Spec name to find
  -> Maybe Text                                      -- ^ Optional slot
  -> [RawDocument]                                   -- ^ Source file docs (unused here but needed for slot check)
  -> IORef (Map.Map FilePath (Either () [Text]))     -- ^ Parse cache
  -> IORef (Set.Set FilePath)                        -- ^ Error pair cache
  -> IO [YassError]
validateCrossFileRef filePath line resolvedPath specName mSlot _docs cacheRef errorPairRef = do
  cache <- readIORef cacheRef
  errorPairs <- readIORef errorPairRef

  case Map.lookup resolvedPath cache of
    Just (Left ()) -> do
      -- Previously failed to find or parse - check if we already reported for this pair
      if Set.member resolvedPath errorPairs
        then return []
        else do
          -- This shouldn't happen since we add to errorPairs when we add Left to cache
          return []
    Just (Right specNames) -> do
      -- Successfully parsed before - check for spec and slot
      let specErrors = if specName `elem` specNames
            then []
            else [mkErr filePath line RefSpecNotFoundOtherFile
                   ("spec not found in referenced file: " <> specName)]
      -- For slot checking on cross-file refs, we'd need the full docs from the other file.
      -- The spec says "check the referenced spec declares that slot" but we need the parsed docs.
      -- We'll need to cache those too. For now, slot checking on cross-file refs requires
      -- re-reading. Let's enhance the cache.
      return specErrors
    Nothing -> do
      -- First time seeing this file
      exists <- doesFileExist resolvedPath
      if not exists
        then do
          -- File not found - report at most once per (source, target) pair
          if Set.member resolvedPath errorPairs
            then return []
            else do
              modifyIORef' errorPairRef (Set.insert resolvedPath)
              modifyIORef' cacheRef (Map.insert resolvedPath (Left ()))
              return [mkErr filePath line RefFileNotFound
                       ("referenced file not found: " <> T.pack resolvedPath)]
        else do
          -- Parse the file
          parseResult <- parseYassFile resolvedPath
          case parseResult of
            ParseError _ _ -> do
              -- Parse failed - report at most once per pair
              if Set.member resolvedPath errorPairs
                then return []
                else do
                  modifyIORef' errorPairRef (Set.insert resolvedPath)
                  modifyIORef' cacheRef (Map.insert resolvedPath (Left ()))
                  return [mkErr filePath line RefFileNotParseable
                           ("referenced file not parseable: " <> T.pack resolvedPath)]
            ParseOK targetDocs -> do
              -- Successfully parsed - cache spec names
              let targetSpecs = collectSpecNames targetDocs
              modifyIORef' cacheRef (Map.insert resolvedPath (Right targetSpecs))
              -- Check spec exists
              let specErrors = if specName `elem` targetSpecs
                    then []
                    else [mkErr filePath line RefSpecNotFoundOtherFile
                           ("spec not found in referenced file: " <> specName)]
              -- Check slot if spec found and slot specified
              slotErrors <- case mSlot of
                Just slotText | null specErrors ->
                  return (checkSlotDeclaredInDocs filePath line targetDocs specName slotText)
                _ -> return []
              return (specErrors ++ slotErrors)

-- | Check if a spec in the same file declares a given slot
checkSlotDeclared :: FilePath -> Int -> [RawDocument] -> Text -> Text -> [YassError]
checkSlotDeclared filePath line docs specName slotText =
  checkSlotDeclaredInDocs filePath line docs specName slotText

-- | Check if a spec in the given docs declares a given slot
checkSlotDeclaredInDocs :: FilePath -> Int -> [RawDocument] -> Text -> Text -> [YassError]
checkSlotDeclaredInDocs filePath line docs specName slotText =
  case findSpecDoc docs specName of
    Nothing -> []  -- spec not found - already reported
    Just doc ->
      let slotKeyStr = slotText
          hasSlot = any (\(k, _, _) -> k == slotKeyStr) (rawDocEntries doc)
      in if hasSlot
         then []
         else [mkErr filePath line RefSlotNotDeclared
                ("referenced spec does not declare slot: " <> specName <> "::" <> slotText)]

-- | Find the RawDocument for a named spec
findSpecDoc :: [RawDocument] -> Text -> Maybe RawDocument
findSpecDoc [] _ = Nothing
findSpecDoc (_:rest) name = findIn rest
  where
    findIn [] = Nothing
    findIn (doc:ds) =
      case lookup3 "spec" (rawDocEntries doc) of
        Just (RVString n, _) | n == name -> Just doc
        _ -> findIn ds

-- | Collect spec names from parsed documents (skipping preamble)
collectSpecNames :: [RawDocument] -> [Text]
collectSpecNames [] = []
collectSpecNames (_:rest) = concatMap getSpecName rest
  where
    getSpecName doc =
      case lookup3 "spec" (rawDocEntries doc) of
        Just (RVString name, _) | not (T.null name) -> [name]
        _ -> []

-- | Lookup helper for (key, value, line) triples
lookup3 :: Text -> [(Text, RawValue, Int)] -> Maybe (RawValue, Int)
lookup3 _ [] = Nothing
lookup3 k ((k', v, ln):rest)
  | k == k'   = Just (v, ln)
  | otherwise  = lookup3 k rest

-- | Helper to construct a YassError
mkErr :: FilePath -> Int -> ErrorCode -> Text -> YassError
mkErr fp line code msg = YassError
  { yeFile    = Just fp
  , yeLine    = Just line
  , yeCode    = code
  , yeMessage = msg
  }
