{-# LANGUAGE OverloadedStrings #-}

module Yass.Glob (expandGlob, hasGlobChars) where

import Data.List (sortBy)
import Data.Ord (comparing)
import qualified Data.Text as T
import qualified Data.Text.Normalize as Normalize
import System.FilePath.Glob (compile, globDir1)
import System.FilePath (splitDirectories, makeRelative)
import System.Directory (doesFileExist, getCurrentDirectory)
import System.Posix.Files (FileStatus, getSymbolicLinkStatus, isSymbolicLink)
import Control.Exception (IOException, try)

-- | Check if a string contains glob metacharacters (*, ?, [)
hasGlobChars :: String -> Bool
hasGlobChars = any (`elem` ("*?[" :: String))

-- | Expand a glob pattern.
--
-- If the argument contains no glob metacharacters, return it as a single
-- literal path unchanged. Otherwise expand using doublestar semantics.
--
-- Returns sorted results by Unicode code-point order on NFC-normalized path.
-- Errors when a glob matches zero files.
-- Case-sensitive matching. No hidden files/directories. No symlink following.
-- No brace expansion, tilde expansion, or env var expansion.
expandGlob :: String -> IO (Either String [FilePath])
expandGlob arg
  | not (hasGlobChars arg) = return $ Right [arg]
  | otherwise = do
      cwd <- getCurrentDirectory
      -- Escape braces to prevent brace expansion (spec forbids it).
      -- Replace '{' with '[{]' and '}' with '[}]' so they match literally.
      let escapedArg = escapeBraces arg
      -- globDir1 expands the pattern relative to cwd, returns absolute paths
      matches <- globDir1 (compile escapedArg) cwd
      -- Make paths relative to cwd
      let relative = map (makeRelative cwd) matches
      -- Filter out hidden files/directories
      let noHidden = filter (not . hasHiddenComponent) relative
      -- Filter out symbolic links (check all path components)
      noSymlinks <- filterSymlinks cwd noHidden
      -- Filter to only regular files (not directories)
      onlyFiles <- filterM' doesFileExist noSymlinks
      -- Sort by NFC-normalised code-point order
      let sorted = sortByNFC onlyFiles
      if null sorted
        then return $ Left $ "no files matched pattern: " ++ arg
        else return $ Right sorted

-- | Check if any component of a path starts with '.'.
hasHiddenComponent :: FilePath -> Bool
hasHiddenComponent = any isHidden . splitDirectories
  where
    isHidden ('.':_) = True
    isHidden _       = False

-- | Filter out paths where any component is a symbolic link.
-- Each relative path is resolved against the given base directory.
filterSymlinks :: FilePath -> [FilePath] -> IO [FilePath]
filterSymlinks base = filterM' (fmap not . hasSymlinkComponent base)

-- | Check whether any component in the path (relative to base) is a symlink.
hasSymlinkComponent :: FilePath -> FilePath -> IO Bool
hasSymlinkComponent base relPath = do
    let dirs = splitDirectories relPath
        -- Build cumulative prefixes: "a", "a/b", "a/b/c.txt"
        prefixes = scanl1 (\a b -> a ++ "/" ++ b) dirs
        -- Resolve each prefix against the base directory
        absPrefixes = map (\p -> base ++ "/" ++ p) prefixes
    anyM safeIsSymlink absPrefixes

-- | Safe symlink check using lstat (does not follow symlinks).
safeIsSymlink :: FilePath -> IO Bool
safeIsSymlink path = do
  result <- try (getSymbolicLinkStatus path) :: IO (Either IOException FileStatus)
  case result of
    Left _     -> return False
    Right stat -> return (isSymbolicLink stat)

-- | Sort file paths by Unicode code-point order on NFC-normalized UTF-8 path.
sortByNFC :: [FilePath] -> [FilePath]
sortByNFC = sortBy (comparing nfcNormalize)
  where
    nfcNormalize :: FilePath -> T.Text
    nfcNormalize = Normalize.normalize Normalize.NFC . T.pack

-- | Escape brace characters in a glob pattern so they are treated as literals.
-- Replaces '{' with '[{]' and '}' with '[}]'.
escapeBraces :: String -> String
escapeBraces [] = []
escapeBraces ('{':rest) = '[' : '{' : ']' : escapeBraces rest
escapeBraces ('}':rest) = '[' : '}' : ']' : escapeBraces rest
escapeBraces (c:rest)   = c : escapeBraces rest

-- | Monadic filter.
filterM' :: (a -> IO Bool) -> [a] -> IO [a]
filterM' _ [] = return []
filterM' p (x:xs) = do
  keep <- p x
  rest <- filterM' p xs
  return $ if keep then x : rest else rest

-- | Monadic any.
anyM :: (a -> IO Bool) -> [a] -> IO Bool
anyM _ [] = return False
anyM p (x:xs) = do
  b <- p x
  if b then return True else anyM p xs
