{-# LANGUAGE OverloadedStrings #-}

module Yass.Discover
  ( discoverSpecFiles
  , isYassYamlFile
  , relativizePath
  ) where

import Control.Exception (IOException, try)
import Data.List (sortBy)
import Data.Ord (comparing)
import qualified Data.Text as T
import qualified Data.Text.Normalize as Normalize
import qualified Data.ByteString as BS
import System.Directory
  ( doesFileExist
  , doesDirectoryExist
  , doesPathExist
  , listDirectory
  , pathIsSymbolicLink
  )
import System.FilePath ((</>), takeFileName, isAbsolute)
import qualified Data.Text.IO as TIO
import System.IO (hFlush, stderr)
import Yass.Types (YassError(..), ErrorCode(..))
import qualified Yass.ErrorLine as EL

-- | Check if a filename has the .yass.yaml suffix and a non-empty basename
-- before the suffix.  The name must not start with ".".
isYassYamlFile :: FilePath -> Bool
isYassYamlFile path =
  let name = takeFileName path
  in hasSuffix ".yass.yaml" name
     && length name > length (".yass.yaml" :: String)
     && head name /= '.'

-- | Check if a string ends with a given suffix (case-sensitive)
hasSuffix :: String -> String -> Bool
hasSuffix suffix str =
  let sLen = length suffix
      strLen = length str
  in strLen >= sLen && drop (strLen - sLen) str == suffix

-- | Relativize a path: make it relative to cwd, no leading "./", basename if
-- directly in cwd, forward slashes.  Delegates to ErrorLine.relativizePath.
relativizePath :: FilePath -> FilePath -> FilePath
relativizePath = EL.relativizePath

-- | Discover .yass.yaml spec files.
--
-- @discoverSpecFiles cwd projectRoot maybeInputPath@
--
-- If @maybeInputPath@ is @Just path@:
--   - If it's a file: verify .yass.yaml suffix and existence, return singleton
--   - If it's a directory: recursively find .yass.yaml files in it
--   - Otherwise: error
--
-- If @maybeInputPath@ is @Nothing@: search from @projectRoot@.
--
-- Results are sorted by Unicode code-point order on NFC-normalized paths.
discoverSpecFiles :: FilePath -> FilePath -> Maybe FilePath -> IO (Either [YassError] [FilePath])
discoverSpecFiles cwd projectRoot Nothing =
  discoverInDirectory cwd projectRoot
discoverSpecFiles cwd _projectRoot (Just inputPath) = do
  let absPath = if isAbsolute inputPath then inputPath
                else cwd </> inputPath
  -- Check if it's a symlink first
  isLink <- safeIsSymbolicLink absPath
  if isLink
    then do
      -- Symlink: check what the target is
      targetExists <- doesFileExist absPath  -- follows symlink
      if targetExists
        then handleFileArg cwd absPath
        else do
          isDir <- doesDirectoryExist absPath  -- follows symlink
          if isDir
            then handleDirArg cwd absPath
            else do
              pathExists <- doesPathExist absPath
              if pathExists
                then return $ Left [YassError
                  { yeFile    = Just (relativizePath cwd absPath)
                  , yeLine    = Nothing
                  , yeCode    = PathInvalidType
                  , yeMessage = "path is neither a file nor a directory: " <> T.pack absPath
                  }]
                else return $ Left [YassError
                  { yeFile    = Just (relativizePath cwd absPath)
                  , yeLine    = Nothing
                  , yeCode    = PathNotFound
                  , yeMessage = "path does not exist: " <> T.pack absPath
                  }]
    else do
      isFile <- doesFileExist absPath
      isDir  <- doesDirectoryExist absPath
      case (isFile, isDir) of
        (True, _) -> handleFileArg cwd absPath
        (_, True) -> handleDirArg cwd absPath
        _ -> do
          pathExists <- doesPathExist absPath
          if pathExists
            then return $ Left [YassError
                  { yeFile    = Just (relativizePath cwd absPath)
                  , yeLine    = Nothing
                  , yeCode    = PathInvalidType
                  , yeMessage = "path is neither a file nor a directory: " <> T.pack absPath
                  }]
            else return $ Left [YassError
                  { yeFile    = Just (relativizePath cwd absPath)
                  , yeLine    = Nothing
                  , yeCode    = PathNotFound
                  , yeMessage = "path does not exist: " <> T.pack absPath
                  }]

-- | Handle a file argument: verify it has .yass.yaml suffix and return singleton.
handleFileArg :: FilePath -> FilePath -> IO (Either [YassError] [FilePath])
handleFileArg cwd absPath =
  if isYassYamlFile absPath
    then do
      readable <- safeIsReadable absPath
      if readable
        then return $ Right [relativizePath cwd absPath]
        else return $ Left [YassError
              { yeFile    = Just (relativizePath cwd absPath)
              , yeLine    = Nothing
              , yeCode    = PathUnreadable
              , yeMessage = "cannot read path: " <> T.pack absPath
              }]
    else return $ Left [YassError
          { yeFile    = Just (relativizePath cwd absPath)
          , yeLine    = Nothing
          , yeCode    = PathBadExtension
          , yeMessage = "expected a .yass.yaml file: " <> T.pack absPath
          }]

-- | Handle a directory argument: check readability first, then discover.
-- Top-level directory args that are unreadable get PathUnreadable (not DiscoverDirUnreadable).
handleDirArg :: FilePath -> FilePath -> IO (Either [YassError] [FilePath])
handleDirArg cwd dirPath = do
  result <- try (listDirectory dirPath) :: IO (Either IOException [String])
  case result of
    Left _exc -> return $ Left [YassError
      { yeFile    = Just (relativizePath cwd dirPath)
      , yeLine    = Nothing
      , yeCode    = PathUnreadable
      , yeMessage = "cannot read path: " <> T.pack dirPath
      }]
    Right entries -> do
      (files, errs) <- walkDirectory cwd dirPath entries
      -- Emit errors for unreadable subdirectories to stderr
      mapM_ (emitDiscoverError cwd) errs
      let sorted = sortByNFC files
      return $ Right sorted

-- | Recursively discover .yass.yaml files in a directory.
discoverInDirectory :: FilePath -> FilePath -> IO (Either [YassError] [FilePath])
discoverInDirectory cwd dirPath = do
  result <- try (listDirectory dirPath) :: IO (Either IOException [String])
  case result of
    Left _exc -> return $ Left [YassError
      { yeFile    = Just (relativizePath cwd dirPath)
      , yeLine    = Nothing
      , yeCode    = PathUnreadable
      , yeMessage = "cannot read path: " <> T.pack dirPath
      }]
    Right entries -> do
      (files, errs) <- walkDirectory cwd dirPath entries
      -- Emit errors for unreadable subdirectories to stderr
      mapM_ (emitDiscoverError cwd) errs
      let sorted = sortByNFC files
      return $ Right sorted

-- | Walk a directory recursively, collecting .yass.yaml files.
-- Skips hidden directories and files, does not follow symlinks during traversal.
-- Returns (foundFiles, subdirectoryErrors).
walkDirectory :: FilePath -> FilePath -> [String] -> IO ([FilePath], [YassError])
walkDirectory cwd baseDir entries = do
  results <- mapM (processEntry cwd baseDir) entries
  let (fileLists, errLists) = unzip results
  return (concat fileLists, concat errLists)

-- | Process a single directory entry during recursive traversal.
-- Returns (foundFiles, errors) to propagate unreadable-subdirectory errors.
processEntry :: FilePath -> FilePath -> String -> IO ([FilePath], [YassError])
processEntry cwd baseDir entry
  -- Skip hidden entries (starting with ".")
  | head entry == '.' = return ([], [])
  | otherwise = do
      let fullPath = baseDir </> entry
      -- Check if it's a symlink - skip symlinks during traversal
      isLink <- safeIsSymbolicLink fullPath
      if isLink
        then return ([], [])
        else do
          isFile <- doesFileExist fullPath
          isDir  <- doesDirectoryExist fullPath
          case (isFile, isDir) of
            (True, _)
              | isYassYamlFile entry -> return ([relativizePath cwd fullPath], [])
              | otherwise            -> return ([], [])
            (_, True) -> do
              subResult <- try (listDirectory fullPath) :: IO (Either IOException [String])
              case subResult of
                Left _ -> return ([], [YassError
                  { yeFile    = Just (relativizePath cwd fullPath)
                  , yeLine    = Nothing
                  , yeCode    = DiscoverDirUnreadable
                  , yeMessage = "cannot read directory: " <> T.pack fullPath
                  }])
                Right sub -> walkDirectory cwd fullPath sub
            _ -> return ([], [])

-- | Emit a discover error to stderr, formatted as an ErrorLine.
emitDiscoverError :: FilePath -> YassError -> IO ()
emitDiscoverError cwd err = do
  let fp = maybe "yass" id (yeFile err)
  TIO.hPutStrLn stderr $ EL.formatErrorLineNoLine cwd fp (yeCode err) (yeMessage err)
  hFlush stderr

-- | Sort file paths by Unicode code-point order on NFC-normalized UTF-8 path.
sortByNFC :: [FilePath] -> [FilePath]
sortByNFC = sortBy (comparing nfcNormalize)
  where
    nfcNormalize :: FilePath -> T.Text
    nfcNormalize = Normalize.normalize Normalize.NFC . T.pack

-- | Safely check if a path is a symbolic link, returning False on error.
safeIsSymbolicLink :: FilePath -> IO Bool
safeIsSymbolicLink path = do
  result <- try (pathIsSymbolicLink path) :: IO (Either IOException Bool)
  case result of
    Left _  -> return False
    Right b -> return b

-- | Safely check if a file is readable.
safeIsReadable :: FilePath -> IO Bool
safeIsReadable path = do
  result <- try (BS.readFile path >> return True) :: IO (Either IOException Bool)
  case result of
    Left _  -> return False
    Right b -> return b
