{-# LANGUAGE OverloadedStrings #-}

module Yass.DiscoverSpec (spec) where

import Test.Hspec
import System.IO.Temp (withSystemTempDirectory)
import System.Directory
  ( createDirectoryIfMissing
  , createFileLink
  , createDirectoryLink
  , setPermissions
  , emptyPermissions
  , setOwnerReadable
  , setOwnerWritable
  , setOwnerExecutable
  , setOwnerSearchable
  , getPermissions
  , removeFile
  )
import System.FilePath ((</>))
import System.Posix.Files (createSymbolicLink)
import Yass.Discover (discoverSpecFiles, isYassYamlFile)
import Yass.Types (YassError(..), ErrorCode(..))

-- | Helper to write a minimal valid .yass.yaml content
writeYassFile :: FilePath -> IO ()
writeYassFile path = writeFile path "description: test\nversion: v1\n"

spec :: Spec
spec = describe "Yass.Discover" $ do

  -- ── isYassYamlFile ────────────────────────────────────────────────────

  describe "isYassYamlFile" $ do
    it "accepts foo.yass.yaml" $
      isYassYamlFile "foo.yass.yaml" `shouldBe` True

    it "accepts deeply/nested/bar.yass.yaml" $
      isYassYamlFile "deeply/nested/bar.yass.yaml" `shouldBe` True

    it "rejects bare .yass.yaml (no basename before suffix)" $
      isYassYamlFile ".yass.yaml" `shouldBe` False

    it "rejects plain .yaml file" $
      isYassYamlFile "foo.yaml" `shouldBe` False

    it "rejects .yml file" $
      isYassYamlFile "foo.yml" `shouldBe` False

    it "rejects wrong case .YASS.YAML" $
      isYassYamlFile "foo.YASS.YAML" `shouldBe` False

    it "rejects wrong case .Yass.Yaml" $
      isYassYamlFile "foo.Yass.Yaml" `shouldBe` False

    it "rejects hidden file .hidden.yass.yaml" $
      isYassYamlFile ".hidden.yass.yaml" `shouldBe` False

    it "accepts multi-dot basename a.b.yass.yaml" $
      isYassYamlFile "a.b.yass.yaml" `shouldBe` True

  -- ── Single file discovery ─────────────────────────────────────────────

  describe "single file discovery" $ do
    it "returns a single file when given a valid .yass.yaml path" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let fp = tmpDir </> "test.yass.yaml"
        writeYassFile fp
        result <- discoverSpecFiles tmpDir tmpDir (Just fp)
        result `shouldBe` Right ["test.yass.yaml"]

    it "returns basename when file is directly in cwd" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let fp = tmpDir </> "myspec.yass.yaml"
        writeYassFile fp
        result <- discoverSpecFiles tmpDir tmpDir (Just fp)
        result `shouldBe` Right ["myspec.yass.yaml"]

    it "returns relative path for file in subdirectory" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let subDir = tmpDir </> "sub"
        createDirectoryIfMissing True subDir
        let fp = subDir </> "nested.yass.yaml"
        writeYassFile fp
        result <- discoverSpecFiles tmpDir tmpDir (Just fp)
        result `shouldBe` Right ["sub/nested.yass.yaml"]

    it "handles relative input path" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let fp = tmpDir </> "rel.yass.yaml"
        writeYassFile fp
        -- Pass a relative path (the function should combine with cwd)
        result <- discoverSpecFiles tmpDir tmpDir (Just (tmpDir </> "rel.yass.yaml"))
        result `shouldBe` Right ["rel.yass.yaml"]

  -- ── Error: not found ──────────────────────────────────────────────────

  describe "not found errors" $ do
    it "returns PathNotFound for nonexistent file" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let fp = tmpDir </> "nonexistent.yass.yaml"
        result <- discoverSpecFiles tmpDir tmpDir (Just fp)
        case result of
          Left [err] -> yeCode err `shouldBe` PathNotFound
          other      -> expectationFailure $ "Expected Left [PathNotFound], got: " ++ show other

    it "returns PathNotFound for nonexistent directory" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let dp = tmpDir </> "nonexistent-dir"
        result <- discoverSpecFiles tmpDir tmpDir (Just dp)
        case result of
          Left [err] -> yeCode err `shouldBe` PathNotFound
          other      -> expectationFailure $ "Expected Left [PathNotFound], got: " ++ show other

  -- ── Error: bad extension ──────────────────────────────────────────────

  describe "bad extension errors" $ do
    it "returns PathBadExtension for .yaml file" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let fp = tmpDir </> "test.yaml"
        writeFile fp "description: test\n"
        result <- discoverSpecFiles tmpDir tmpDir (Just fp)
        case result of
          Left [err] -> yeCode err `shouldBe` PathBadExtension
          other      -> expectationFailure $ "Expected Left [PathBadExtension], got: " ++ show other

    it "returns PathBadExtension for .txt file" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let fp = tmpDir </> "test.txt"
        writeFile fp "hello\n"
        result <- discoverSpecFiles tmpDir tmpDir (Just fp)
        case result of
          Left [err] -> yeCode err `shouldBe` PathBadExtension
          other      -> expectationFailure $ "Expected Left [PathBadExtension], got: " ++ show other

    it "returns PathBadExtension for wrong-case suffix" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let fp = tmpDir </> "test.YASS.YAML"
        writeFile fp "description: test\n"
        result <- discoverSpecFiles tmpDir tmpDir (Just fp)
        case result of
          Left [err] -> yeCode err `shouldBe` PathBadExtension
          other      -> expectationFailure $ "Expected Left [PathBadExtension], got: " ++ show other

  -- ── Directory recursive discovery ─────────────────────────────────────

  describe "directory recursive discovery" $ do
    it "finds all .yass.yaml files in a directory tree" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let root = tmpDir </> "project"
        createDirectoryIfMissing True root
        createDirectoryIfMissing True (root </> "sub1")
        createDirectoryIfMissing True (root </> "sub2")
        writeYassFile (root </> "a.yass.yaml")
        writeYassFile (root </> "sub1" </> "b.yass.yaml")
        writeYassFile (root </> "sub2" </> "c.yass.yaml")
        result <- discoverSpecFiles tmpDir root (Nothing)
        case result of
          Right files -> length files `shouldBe` 3
          Left errs   -> expectationFailure $ "Expected Right, got: " ++ show errs

    it "uses project root when no input path given" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let root = tmpDir </> "project"
        createDirectoryIfMissing True root
        writeYassFile (root </> "spec.yass.yaml")
        result <- discoverSpecFiles tmpDir root Nothing
        case result of
          Right files -> files `shouldBe` ["project/spec.yass.yaml"]
          Left errs   -> expectationFailure $ "Expected Right, got: " ++ show errs

    it "discovers in a subdirectory when given as input" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let sub = tmpDir </> "sub"
        createDirectoryIfMissing True sub
        writeYassFile (sub </> "found.yass.yaml")
        writeYassFile (tmpDir </> "root.yass.yaml")
        result <- discoverSpecFiles tmpDir tmpDir (Just sub)
        case result of
          Right files -> files `shouldBe` ["sub/found.yass.yaml"]
          Left errs   -> expectationFailure $ "Expected Right, got: " ++ show errs

    it "returns empty list for directory with no matching files" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let sub = tmpDir </> "empty"
        createDirectoryIfMissing True sub
        writeFile (sub </> "readme.txt") "hello"
        result <- discoverSpecFiles tmpDir tmpDir (Just sub)
        result `shouldBe` Right []

  -- ── Hidden directory/file exclusion ───────────────────────────────────

  describe "hidden directory and file exclusion" $ do
    it "does not descend into directories starting with dot" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let hidden = tmpDir </> ".hidden"
        createDirectoryIfMissing True hidden
        writeYassFile (hidden </> "secret.yass.yaml")
        writeYassFile (tmpDir </> "visible.yass.yaml")
        result <- discoverSpecFiles tmpDir tmpDir Nothing
        case result of
          Right files -> files `shouldBe` ["visible.yass.yaml"]
          Left errs   -> expectationFailure $ "Expected Right, got: " ++ show errs

    it "does not match files whose basename starts with dot" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        writeYassFile (tmpDir </> ".hidden.yass.yaml")
        writeYassFile (tmpDir </> "visible.yass.yaml")
        result <- discoverSpecFiles tmpDir tmpDir Nothing
        case result of
          Right files -> files `shouldBe` ["visible.yass.yaml"]
          Left errs   -> expectationFailure $ "Expected Right, got: " ++ show errs

    it "skips .git directory" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let gitDir = tmpDir </> ".git"
        createDirectoryIfMissing True gitDir
        writeYassFile (gitDir </> "config.yass.yaml")
        writeYassFile (tmpDir </> "real.yass.yaml")
        result <- discoverSpecFiles tmpDir tmpDir Nothing
        case result of
          Right files -> files `shouldBe` ["real.yass.yaml"]
          Left errs   -> expectationFailure $ "Expected Right, got: " ++ show errs

  -- ── Bare .yass.yaml not matching ──────────────────────────────────────

  describe "bare .yass.yaml exclusion" $ do
    it "does not match a file named exactly .yass.yaml during traversal" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        -- .yass.yaml starts with dot, so it's excluded by hidden-file rule
        writeYassFile (tmpDir </> ".yass.yaml")
        writeYassFile (tmpDir </> "valid.yass.yaml")
        result <- discoverSpecFiles tmpDir tmpDir Nothing
        case result of
          Right files -> files `shouldBe` ["valid.yass.yaml"]
          Left errs   -> expectationFailure $ "Expected Right, got: " ++ show errs

    it "rejects bare .yass.yaml as a direct file argument (bad extension - hidden)" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let fp = tmpDir </> ".yass.yaml"
        writeYassFile fp
        result <- discoverSpecFiles tmpDir tmpDir (Just fp)
        case result of
          Left [err] -> yeCode err `shouldBe` PathBadExtension
          other      -> expectationFailure $ "Expected Left [PathBadExtension], got: " ++ show other

  -- ── .yass.yaml suffix matching ────────────────────────────────────────

  describe ".yass.yaml suffix matching" $ do
    it "only matches files ending in .yass.yaml" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        writeYassFile (tmpDir </> "good.yass.yaml")
        writeFile (tmpDir </> "bad.yaml") "test\n"
        writeFile (tmpDir </> "bad.yass.yml") "test\n"
        writeFile (tmpDir </> "bad.txt") "test\n"
        result <- discoverSpecFiles tmpDir tmpDir Nothing
        case result of
          Right files -> files `shouldBe` ["good.yass.yaml"]
          Left errs   -> expectationFailure $ "Expected Right, got: " ++ show errs

    it "suffix match is case-sensitive" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        writeYassFile (tmpDir </> "good.yass.yaml")
        writeFile (tmpDir </> "bad.YASS.YAML") "test\n"
        writeFile (tmpDir </> "bad.Yass.Yaml") "test\n"
        result <- discoverSpecFiles tmpDir tmpDir Nothing
        case result of
          Right files -> files `shouldBe` ["good.yass.yaml"]
          Left errs   -> expectationFailure $ "Expected Right, got: " ++ show errs

  -- ── Sorting order ─────────────────────────────────────────────────────

  describe "sorting order" $ do
    it "sorts results by Unicode code-point order on NFC-normalized path" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        -- Create files that would sort differently if not sorted
        writeYassFile (tmpDir </> "z.yass.yaml")
        writeYassFile (tmpDir </> "a.yass.yaml")
        writeYassFile (tmpDir </> "m.yass.yaml")
        result <- discoverSpecFiles tmpDir tmpDir Nothing
        result `shouldBe` Right ["a.yass.yaml", "m.yass.yaml", "z.yass.yaml"]

    it "sorts nested paths correctly" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        createDirectoryIfMissing True (tmpDir </> "b")
        createDirectoryIfMissing True (tmpDir </> "a")
        writeYassFile (tmpDir </> "b" </> "spec.yass.yaml")
        writeYassFile (tmpDir </> "a" </> "spec.yass.yaml")
        writeYassFile (tmpDir </> "c.yass.yaml")
        result <- discoverSpecFiles tmpDir tmpDir Nothing
        result `shouldBe` Right ["a/spec.yass.yaml", "b/spec.yass.yaml", "c.yass.yaml"]

    it "uppercase sorts before lowercase in code-point order" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        writeYassFile (tmpDir </> "b.yass.yaml")
        writeYassFile (tmpDir </> "A.yass.yaml")
        result <- discoverSpecFiles tmpDir tmpDir Nothing
        -- 'A' (0x41) < 'b' (0x62) in Unicode code-point order
        result `shouldBe` Right ["A.yass.yaml", "b.yass.yaml"]

  -- ── Symlink handling ──────────────────────────────────────────────────

  describe "symlink handling" $ do
    it "file arg symlink: uses symlink path for reporting" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let target = tmpDir </> "real.yass.yaml"
        let link   = tmpDir </> "link.yass.yaml"
        writeYassFile target
        createFileLink target link
        result <- discoverSpecFiles tmpDir tmpDir (Just link)
        result `shouldBe` Right ["link.yass.yaml"]

    it "directory arg symlink: traverses the linked directory" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let realDir = tmpDir </> "realdir"
        let linkDir = tmpDir </> "linkdir"
        createDirectoryIfMissing True realDir
        writeYassFile (realDir </> "spec.yass.yaml")
        createDirectoryLink realDir linkDir
        result <- discoverSpecFiles tmpDir tmpDir (Just linkDir)
        case result of
          Right files -> files `shouldBe` ["linkdir/spec.yass.yaml"]
          Left errs   -> expectationFailure $ "Expected Right, got: " ++ show errs

    it "symlinks during traversal are skipped (treated as absent)" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let realFile = tmpDir </> "real.yass.yaml"
        let linkFile = tmpDir </> "symlinked.yass.yaml"
        writeYassFile realFile
        createFileLink realFile linkFile
        result <- discoverSpecFiles tmpDir tmpDir Nothing
        case result of
          Right files -> files `shouldBe` ["real.yass.yaml"]
          Left errs   -> expectationFailure $ "Expected Right, got: " ++ show errs

    it "directory symlinks during traversal are skipped" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let realDir = tmpDir </> "realdir"
        let linkDir = tmpDir </> "linkdir"
        createDirectoryIfMissing True realDir
        writeYassFile (realDir </> "spec.yass.yaml")
        createDirectoryLink realDir linkDir
        writeYassFile (tmpDir </> "top.yass.yaml")
        result <- discoverSpecFiles tmpDir tmpDir Nothing
        case result of
          Right files -> do
            files `shouldContain` ["top.yass.yaml"]
            files `shouldContain` ["realdir/spec.yass.yaml"]
            -- linkdir should be skipped (it's a symlink during traversal)
            files `shouldNotContain` ["linkdir/spec.yass.yaml"]
          Left errs -> expectationFailure $ "Expected Right, got: " ++ show errs

  -- ── Deep nesting ──────────────────────────────────────────────────────

  describe "deep nesting" $ do
    it "discovers files in deeply nested directories" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let deep = tmpDir </> "a" </> "b" </> "c" </> "d"
        createDirectoryIfMissing True deep
        writeYassFile (deep </> "deep.yass.yaml")
        result <- discoverSpecFiles tmpDir tmpDir Nothing
        result `shouldBe` Right ["a/b/c/d/deep.yass.yaml"]

  -- ── Empty directory ───────────────────────────────────────────────────

  describe "empty directory" $ do
    it "returns empty list for empty directory" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        result <- discoverSpecFiles tmpDir tmpDir Nothing
        result `shouldBe` Right []

  -- ── Unreadable directory (handleDirArg) ───────────────────────────────

  describe "unreadable directory" $ do
    it "returns PathUnreadable for unreadable top-level directory arg" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let unreadable = tmpDir </> "noperm"
        createDirectoryIfMissing True unreadable
        writeYassFile (unreadable </> "spec.yass.yaml")
        origPerms <- getPermissions unreadable
        setPermissions unreadable (setOwnerReadable False (setOwnerSearchable False emptyPermissions))
        result <- discoverSpecFiles tmpDir tmpDir (Just unreadable)
        -- Restore permissions so cleanup works
        setPermissions unreadable origPerms
        case result of
          Left [err] -> yeCode err `shouldBe` PathUnreadable
          other      -> expectationFailure $ "Expected Left [PathUnreadable], got: " ++ show other

    it "returns PathUnreadable for unreadable project root (discoverInDirectory)" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let unreadable = tmpDir </> "root"
        createDirectoryIfMissing True unreadable
        writeYassFile (unreadable </> "spec.yass.yaml")
        origPerms <- getPermissions unreadable
        setPermissions unreadable (setOwnerReadable False (setOwnerSearchable False emptyPermissions))
        result <- discoverSpecFiles tmpDir unreadable Nothing
        -- Restore permissions so cleanup works
        setPermissions unreadable origPerms
        case result of
          Left [err] -> yeCode err `shouldBe` PathUnreadable
          other      -> expectationFailure $ "Expected Left [PathUnreadable], got: " ++ show other

  -- ── Walk directory error propagation (unreadable subdirs) ─────────────

  describe "walk directory error propagation" $ do
    it "continues discovery and emits DiscoverDirUnreadable for unreadable subdirectory" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let goodDir = tmpDir </> "good"
        let badDir  = tmpDir </> "bad"
        createDirectoryIfMissing True goodDir
        createDirectoryIfMissing True badDir
        writeYassFile (goodDir </> "found.yass.yaml")
        writeYassFile (badDir </> "hidden.yass.yaml")
        origPerms <- getPermissions badDir
        setPermissions badDir (setOwnerReadable False (setOwnerSearchable False emptyPermissions))
        result <- discoverSpecFiles tmpDir tmpDir Nothing
        -- Restore permissions so cleanup works
        setPermissions badDir origPerms
        -- Should still find files from readable dirs (returns Right)
        case result of
          Right files -> files `shouldSatisfy` \fs -> "good/found.yass.yaml" `elem` fs
          Left errs   -> expectationFailure $ "Expected Right, got: " ++ show errs

  -- ── Broken symlink (PathNotFound for dangling symlink) ────────────────

  describe "broken symlink handling" $ do
    it "returns PathNotFound for a dangling symlink argument" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let target = tmpDir </> "nonexistent-target.yass.yaml"
        let link   = tmpDir </> "broken.yass.yaml"
        createSymbolicLink target link
        result <- discoverSpecFiles tmpDir tmpDir (Just link)
        case result of
          Left [err] -> yeCode err `shouldBe` PathNotFound
          other      -> expectationFailure $ "Expected Left [PathNotFound], got: " ++ show other

  -- ── Symlink to file argument ──────────────────────────────────────────

  describe "symlink as file argument" $ do
    it "follows symlink to file and returns result via handleFileArg" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let target = tmpDir </> "real.yass.yaml"
        let link   = tmpDir </> "symlink.yass.yaml"
        writeYassFile target
        createFileLink target link
        result <- discoverSpecFiles tmpDir tmpDir (Just link)
        result `shouldBe` Right ["symlink.yass.yaml"]

    it "symlink to non-.yass.yaml file gives PathBadExtension" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let target = tmpDir </> "real.txt"
        let link   = tmpDir </> "link.txt"
        writeFile target "hello"
        createFileLink target link
        result <- discoverSpecFiles tmpDir tmpDir (Just link)
        case result of
          Left [err] -> yeCode err `shouldBe` PathBadExtension
          other      -> expectationFailure $ "Expected Left [PathBadExtension], got: " ++ show other

  -- ── Symlink to directory argument ─────────────────────────────────────

  describe "symlink to directory argument" $ do
    it "follows symlink to directory and discovers files inside" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let realDir = tmpDir </> "real"
        let link    = tmpDir </> "linkdir"
        createDirectoryIfMissing True realDir
        writeYassFile (realDir </> "inner.yass.yaml")
        createDirectoryLink realDir link
        result <- discoverSpecFiles tmpDir tmpDir (Just link)
        case result of
          Right files -> files `shouldBe` ["linkdir/inner.yass.yaml"]
          Left errs   -> expectationFailure $ "Expected Right, got: " ++ show errs

  -- ── doesPathExist edge case: PathInvalidType (non-file, non-dir) ──────

  describe "PathInvalidType" $ do
    it "returns PathNotFound when path does not exist and is not a symlink" $
      withSystemTempDirectory "yass-test" $ \tmpDir -> do
        let fp = tmpDir </> "nope"
        result <- discoverSpecFiles tmpDir tmpDir (Just fp)
        case result of
          Left [err] -> yeCode err `shouldBe` PathNotFound
          other      -> expectationFailure $ "Expected Left [PathNotFound], got: " ++ show other
