{-# LANGUAGE OverloadedStrings #-}

module Yass.Validate (runValidate) where

import Control.Exception (IOException, try)
import Data.IORef
import Data.List (sortBy)
import Data.Ord (comparing)
import qualified Data.Set as Set
import qualified Data.Text as T
import qualified Data.Text.IO as TIO
import qualified Data.Text.Normalize as Normalize
import System.Directory (getCurrentDirectory, makeAbsolute)
import System.Exit (ExitCode(ExitSuccess, ExitFailure))
import System.FilePath (normalise)
import System.IO (hPutStrLn, hFlush, stderr, stdout)

import Yass.Types hiding (ExitSuccess)
import Yass.ErrorLine (formatErrorLine, formatErrorLineNoLine, relativizePath)
import Yass.Parser (RawDocument(..), RawValue(..), ParseResult(..), parseYassFile)
import Yass.ProjectRoot (findProjectRoot)
import Yass.Discover (discoverSpecFiles, isYassYamlFile)
import Yass.Glob (expandGlob, hasGlobChars)
import Yass.Validate.CheckYAML (checkYAML)
import Yass.Validate.CheckPreamble (checkPreamble)
import Yass.Validate.CheckSpec (checkSpec)
import Yass.Validate.CheckUniqueness (checkUniqueness)
import Yass.Validate.CheckRefs (checkRefs)

-- | Run the validate subcommand.
-- Takes argv (subcommand args, NOT including "validate" itself).
runValidate :: [String] -> IO ExitCode
runValidate args = do
  cwd <- getCurrentDirectory

  -- Check for colon in any path argument
  let colonPaths = filter (':' `elem`) args
  case colonPaths of
    (p:_) -> do
      emitError cwd "yass" PathColonInPath
        ("path contains an unsupported colon character: " <> T.pack p)
      return (ExitFailure 2)
    [] -> do
      -- Find project root
      rootResult <- findProjectRoot cwd
      case rootResult of
        Left _ -> do
          emitError cwd (relativizePath cwd cwd) FindrootNoMarker
            "no project root marker found"
          return (ExitFailure 2)
        Right projectRoot -> do
          -- Resolve input paths
          filesResult <- resolveInputFiles cwd projectRoot args
          case filesResult of
            Left err -> do
              emitYassError cwd err
              return (ExitFailure 2)
            Right files
              | null files -> do
                  emitError cwd "yass" DiscoverNoFiles "no .yass.yaml files found"
                  putStrLn "checked 0 files, found 0 errors"
                  hFlush stdout
                  return (ExitFailure 2)
              | otherwise -> do
                  -- Deduplicate by absolute path
                  deduped <- deduplicateFiles files
                  -- Sort by NFC-normalized path
                  let sorted = sortByNFC deduped
                  -- Validate each file
                  errorCount <- newIORef (0 :: Int)
                  mapM_ (validateFile cwd projectRoot errorCount) sorted
                  -- Emit summary
                  hFlush stderr
                  m <- readIORef errorCount
                  putStrLn $ "checked " ++ show (length sorted) ++ " files, found " ++ show m ++ " errors"
                  hFlush stdout
                  if m > 0
                    then return (ExitFailure 1)
                    else return ExitSuccess

-- | Resolve input files from arguments.
resolveInputFiles :: FilePath -> FilePath -> [String] -> IO (Either YassError [FilePath])
resolveInputFiles cwd projectRoot [] = do
  -- No args: discover from project root
  result <- discoverSpecFiles cwd projectRoot Nothing
  case result of
    Left errs -> return $ Left (head errs)
    Right fs -> return $ Right fs
resolveInputFiles cwd projectRoot args = do
  allFiles <- newIORef ([] :: [FilePath])
  let processArg arg
        | hasGlobChars arg = do
            globResult <- expandGlob arg
            case globResult of
              Left msg -> return $ Just $ YassError
                { yeFile = Just "yass", yeLine = Nothing
                , yeCode = GlobNoMatch, yeMessage = T.pack msg }
              Right paths -> do
                -- For glob-expanded paths, skip non-.yass.yaml files silently
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

  errs <- mapM processArg args
  case [e | Just e <- errs] of
    (e:_) -> return $ Left e
    [] -> do
      fs <- readIORef allFiles
      return $ Right fs

-- | Validate a single file.
validateFile :: FilePath -> FilePath -> IORef Int -> FilePath -> IO ()
validateFile cwd projectRoot errorCount fp = do
  absPath <- makeAbsolute fp
  -- Step 1: CheckYAML
  yamlResult <- checkYAML cwd absPath
  case yamlResult of
    Left err -> do
      emitYassError cwd err
      modifyIORef' errorCount (+ 1)
    Right docs -> do
      -- Step 2: CheckPreamble
      let preambleErrors = checkPreamble cwd absPath docs
      -- Step 3: CheckSpec (on non-first documents)
      let specDocs = case docs of
            (_:rest) -> rest
            [] -> []
      let specErrors = concatMap (checkSpec cwd absPath) specDocs
      -- Step 4: CheckUniqueness
      let uniqueErrors = checkUniqueness cwd absPath docs
      -- Step 5: CheckRefs
      refErrors <- checkRefs cwd absPath projectRoot docs
      -- Collect all errors and sort by line number ascending (Nothing first)
      let allErrors = sortBy (comparing yeLine)
                        (preambleErrors ++ specErrors ++ uniqueErrors ++ refErrors)
      mapM_ (emitYassError cwd) allErrors
      modifyIORef' errorCount (+ length allErrors)

-- | Deduplicate files by normalized absolute path.
deduplicateFiles :: [FilePath] -> IO [FilePath]
deduplicateFiles files = do
  absFiles <- mapM (\f -> do
    abs' <- makeAbsolute f
    return (normalise abs', f)
    ) files
  let seen = Set.empty :: Set.Set FilePath
      go _ [] = []
      go s ((absP, origP):rest)
        | Set.member absP s = go s rest
        | otherwise = origP : go (Set.insert absP s) rest
  return $ go seen absFiles

-- | Sort file paths by NFC-normalized Unicode code-point order.
sortByNFC :: [FilePath] -> [FilePath]
sortByNFC = sortBy (comparing nfcNorm)
  where nfcNorm = Normalize.normalize Normalize.NFC . T.pack

-- | Emit an error line to stderr.
emitError :: FilePath -> FilePath -> ErrorCode -> T.Text -> IO ()
emitError cwd fp code msg = do
  let line = formatErrorLineNoLine cwd fp code msg
  TIO.hPutStrLn stderr line
  hFlush stderr

-- | Emit a YassError to stderr.
emitYassError :: FilePath -> YassError -> IO ()
emitYassError cwd err = do
  let fp = maybe "yass" id (yeFile err)
      msg = yeMessage err
      code = yeCode err
  line <- case yeLine err of
    Just ln -> return $ formatErrorLine cwd fp ln code msg
    Nothing -> return $ formatErrorLineNoLine cwd fp code msg
  TIO.hPutStrLn stderr line
  hFlush stderr
