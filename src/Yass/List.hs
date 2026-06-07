{-# LANGUAGE OverloadedStrings #-}

module Yass.List (runList, normalizeDescription, truncateDescription) where

import Control.Exception (IOException, try)
import Data.IORef
import Data.List (sortBy)
import Data.Ord (comparing)
import qualified Data.Set as Set
import qualified Data.Text as T
import qualified Data.Text.IO as TIO
import qualified Data.Text.Normalize as Normalize
import System.Directory (getCurrentDirectory, makeAbsolute)
import System.Environment (lookupEnv)
import System.Exit (ExitCode(..))
import System.FilePath (normalise)
import System.IO (hFlush, hIsTerminalDevice, stderr, stdout)
import System.Process (readProcess)

import Yass.Types hiding (ExitSuccess)
import Yass.ErrorLine (formatErrorLineNoLine, relativizePath)
import Yass.Parser (RawDocument(..), RawValue(..), ParseResult(..), parseYassFile)
import Yass.ProjectRoot (findProjectRoot)
import Yass.Discover (discoverSpecFiles, isYassYamlFile)
import Yass.Glob (expandGlob, hasGlobChars)

-- | Run the list subcommand.
-- Takes argv (subcommand args, NOT including "list" itself).
runList :: [String] -> IO ExitCode
runList args = do
  cwd <- getCurrentDirectory

  -- Check for colon in paths
  let colonPaths = filter (':' `elem`) args
  case colonPaths of
    (p:_) -> do
      emitErr cwd "yass" PathColonInPath
        ("path contains an unsupported colon character: " <> T.pack p)
      return (ExitFailure 2)
    [] -> do
      -- Find project root
      rootResult <- findProjectRoot cwd
      case rootResult of
        Left _ -> do
          emitErr cwd (relativizePath cwd cwd) FindrootNoMarker
            "no project root marker found"
          return (ExitFailure 2)
        Right projectRoot -> do
          -- Resolve input files (supports multiple args, globs, dirs)
          filesResult <- resolveInputFiles cwd projectRoot args
          case filesResult of
            Left err -> do
              emitYassErr cwd err
              return (ExitFailure 2)
            Right files
              | null files -> return ExitSuccess  -- no files found → exit 0, no output
              | otherwise -> do
                  -- Deduplicate by absolute path, then sort by NFC
                  deduped <- deduplicateFiles files
                  let sorted = sortByNFC deduped

                  -- Get terminal width if stdout is a TTY
                  isTTY <- hIsTerminalDevice stdout
                  termWidth <- if isTTY then getTerminalWidth else return Nothing

                  -- Process each file
                  hadParseError <- processFiles cwd sorted termWidth
                  hFlush stdout
                  if hadParseError
                    then return (ExitFailure 1)
                    else return ExitSuccess

-- | Resolve input files from arguments (handles globs, dirs, explicit files).
resolveInputFiles :: FilePath -> FilePath -> [String] -> IO (Either YassError [FilePath])
resolveInputFiles cwd projectRoot [] = do
  result <- discoverSpecFiles cwd projectRoot Nothing
  case result of
    Left errs -> return $ Left (head errs)
    Right fs  -> return $ Right fs
resolveInputFiles cwd projectRoot allArgs = do
  allFiles <- newIORef ([] :: [FilePath])
  let processArg arg
        | hasGlobChars arg = do
            globResult <- expandGlob arg
            case globResult of
              Left msg -> return $ Just $ YassError
                { yeFile = Just "yass", yeLine = Nothing
                , yeCode = GlobNoMatch, yeMessage = T.pack msg }
              Right paths -> do
                -- For glob-expanded paths, keep only .yass.yaml files
                let yassFiles = filter isYassYamlFile paths
                modifyIORef' allFiles (++ yassFiles)
                return Nothing
        | otherwise = do
            result <- discoverSpecFiles cwd projectRoot (Just arg)
            case result of
              Left errs -> return $ Just (head errs)
              Right fs -> do
                modifyIORef' allFiles (++ fs)
                return Nothing

  errs <- mapM processArg allArgs
  case [e | Just e <- errs] of
    (e:_) -> return $ Left e
    [] -> do
      fs <- readIORef allFiles
      return $ Right fs

-- | Process files and emit list rows. Returns True if any file failed to parse.
processFiles :: FilePath -> [FilePath] -> Maybe Int -> IO Bool
processFiles cwd files termWidth = do
  results <- mapM (processFile cwd termWidth) files
  return (or results)

-- | Process a single file. Returns True if parse failed.
processFile :: FilePath -> Maybe Int -> FilePath -> IO Bool
processFile cwd termWidth fp = do
  result <- parseYassFile fp
  case result of
    ParseError _ _ -> do
      let errLine = formatErrorLineNoLine cwd fp YamlMalformed
                      "YAML well-formedness error"
      TIO.hPutStrLn stderr errLine
      hFlush stderr
      return True
    ParseOK docs -> do
      let relPath = relativizePath cwd fp
      case docs of
        [] -> return False
        (preamble:specDocs) -> do
          let desc = extractDescription preamble
              normDesc = normalizeDescription desc
          mapM_ (emitListRow relPath normDesc termWidth) (extractSpecNames specDocs)
          return False

-- | Extract spec names from non-preamble documents, preserving doc order.
extractSpecNames :: [RawDocument] -> [T.Text]
extractSpecNames = concatMap getSpecName
  where
    getSpecName doc = case lookup3 "spec" (rawDocEntries doc) of
      Just (RVString name, _) -> [name]
      _ -> []

-- | Extract the preamble description.
extractDescription :: RawDocument -> T.Text
extractDescription doc = case lookup3 "description" (rawDocEntries doc) of
  Just (RVString desc, _) -> desc
  _ -> ""

-- | Normalize a description: replace all whitespace runs with single space,
-- strip leading/trailing whitespace.
normalizeDescription :: T.Text -> T.Text
normalizeDescription t
  | T.null stripped = ""
  | otherwise       = T.intercalate " " (T.words stripped)
  where
    stripped = T.strip t

-- | Truncate a description to fit within a terminal width.
--
-- @truncateDescription fileLen nameLen width desc@
--
-- The line format is: file\\tname\\tdescription
-- Prefix width = fileLen + 1 (tab) + nameLen + 1 (tab) = fileLen + nameLen + 2
-- Marker = "..." (3 chars)
--
-- * If description is empty -> empty, no marker.
-- * If prefix + marker >= width -> empty, no marker.
-- * If description fits -> return as-is.
-- * Otherwise -> truncate + "...".
truncateDescription :: Int -> Int -> Int -> T.Text -> T.Text
truncateDescription fileLen nameLen width desc
  | T.null desc = ""
  | prefixLen + markerLen >= width = ""
  | T.length desc <= available = desc
  | truncLen <= 0 = ""
  | otherwise = T.take truncLen desc <> "..."
  where
    prefixLen = fileLen + nameLen + 2  -- file + TAB + name + TAB
    markerLen = 3  -- "..."
    available = width - prefixLen
    truncLen  = available - markerLen

-- | Emit a list row for a spec.
emitListRow :: FilePath -> T.Text -> Maybe Int -> T.Text -> IO ()
emitListRow filePath desc termWidth specName = do
  let -- Replace literal tab characters in the file-path field with spaces
      fp = T.replace "\t" " " (T.pack filePath)
      -- Apply NFC normalization to the description before emission
      nfcDesc = Normalize.normalize Normalize.NFC desc
  case termWidth of
    Nothing ->
      -- Not a TTY: full output, no truncation
      TIO.putStrLn $ fp <> "\t" <> specName <> "\t" <> nfcDesc
    Just width -> do
      -- TTY: truncate description to fit width
      let truncated = truncateDescription (T.length fp) (T.length specName) width nfcDesc
      TIO.putStrLn $ fp <> "\t" <> specName <> "\t" <> truncated

-- | Get terminal width from COLUMNS env var or terminal query.
getTerminalWidth :: IO (Maybe Int)
getTerminalWidth = do
  colsEnv <- lookupEnv "COLUMNS"
  case colsEnv of
    Just s -> case reads s of
      [(n, "")] | n > 0 -> return (Just n)
      _ -> queryTerminalWidth
    Nothing -> queryTerminalWidth

-- | Query terminal width from OS using tput. Falls back to Just 80 on failure.
queryTerminalWidth :: IO (Maybe Int)
queryTerminalWidth = do
  result <- try (readProcess "tput" ["cols"] "") :: IO (Either IOException String)
  case result of
    Right s -> case reads (strip s) of
      [(n, "")] | n > 0 -> return (Just n)
      _                 -> return (Just 80)
    Left _ -> return (Just 80)
  where
    strip = reverse . dropWhile isSpace . reverse . dropWhile isSpace
    isSpace c = c == ' ' || c == '\n' || c == '\r' || c == '\t'

-- | Deduplicate files by normalized absolute path.
deduplicateFiles :: [FilePath] -> IO [FilePath]
deduplicateFiles files = do
  absFiles <- mapM (\f -> do
    abs' <- makeAbsolute f
    return (normalise abs', f)
    ) files
  let go _ [] = []
      go s ((absP, origP):rest)
        | Set.member absP s = go s rest
        | otherwise = origP : go (Set.insert absP s) rest
  return $ go (Set.empty :: Set.Set FilePath) absFiles

-- | Sort file paths by NFC-normalized Unicode code-point order.
sortByNFC :: [FilePath] -> [FilePath]
sortByNFC = sortBy (comparing nfcNorm)
  where nfcNorm = Normalize.normalize Normalize.NFC . T.pack

-- | Emit an error to stderr.
emitErr :: FilePath -> FilePath -> ErrorCode -> T.Text -> IO ()
emitErr cwd fp code msg = do
  TIO.hPutStrLn stderr $ formatErrorLineNoLine cwd fp code msg
  hFlush stderr

-- | Emit a YassError to stderr.
emitYassErr :: FilePath -> YassError -> IO ()
emitYassErr cwd err = do
  let fp = maybe "yass" id (yeFile err)
  TIO.hPutStrLn stderr $ formatErrorLineNoLine cwd fp (yeCode err) (yeMessage err)
  hFlush stderr

-- | Lookup helper for (key, value, line) triples.
lookup3 :: T.Text -> [(T.Text, RawValue, Int)] -> Maybe (RawValue, Int)
lookup3 _ [] = Nothing
lookup3 k ((k', v, ln):rest)
  | k == k'   = Just (v, ln)
  | otherwise  = lookup3 k rest
