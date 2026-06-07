module Yass.ProjectRoot (findProjectRoot) where

import System.Directory (makeAbsolute, doesDirectoryExist, doesFileExist,
                         listDirectory)
import System.FilePath  (takeDirectory, (</>), isDrive)

-- | Find the project root by searching upward for .git or .yass.yaml.
--
-- Algorithm:
--   1. Canonicalize the starting directory to an absolute path.
--   2. First pass: walk upward looking for a .git entry (file or directory).
--      Return the deepest ancestor that contains .git.
--   3. If no .git found: restart from the starting directory and walk upward
--      looking for any .yass.yaml file in the directory listing.
--      Return the deepest ancestor that contains a .yass.yaml file.
--   4. If neither marker is found, return an error.
findProjectRoot :: FilePath -> IO (Either String FilePath)
findProjectRoot startDir = do
  absDir <- makeAbsolute startDir
  mGit <- searchUpward hasGit absDir
  case mGit of
    Just root -> return (Right root)
    Nothing -> do
      mYass <- searchUpward hasYassYaml absDir
      case mYass of
        Just root -> return (Right root)
        Nothing   -> return (Left "no project root found")

-- | Walk upward from a directory toward the filesystem root, applying a
-- predicate at each level. Returns the first (deepest) directory for which
-- the predicate holds.
searchUpward :: (FilePath -> IO Bool) -> FilePath -> IO (Maybe FilePath)
searchUpward predicate dir = do
  found <- predicate dir
  if found
    then return (Just dir)
    else
      if isDrive dir
        then return Nothing
        else searchUpward predicate (takeDirectory dir)

-- | Check whether a directory contains a .git entry (file or directory).
hasGit :: FilePath -> IO Bool
hasGit dir = do
  let gitPath = dir </> ".git"
  isDir  <- doesDirectoryExist gitPath
  isFile <- doesFileExist gitPath
  return (isDir || isFile)

-- | Check whether a directory contains any file matching the .yass.yaml suffix.
hasYassYaml :: FilePath -> IO Bool
hasYassYaml dir = do
  entries <- listDirectory dir
  return $ any isYassYaml entries
  where
    isYassYaml name =
      let suffix = ".yass.yaml"
          sLen   = length suffix
          nLen   = length name
      in nLen > sLen && drop (nLen - sLen) name == suffix
