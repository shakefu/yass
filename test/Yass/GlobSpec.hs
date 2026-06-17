module Yass.GlobSpec (spec) where

import Test.Hspec
import System.IO.Temp     (withSystemTempDirectory)
import System.Directory   (createDirectoryIfMissing, setCurrentDirectory,
                           getCurrentDirectory, doesFileExist)
import System.FilePath    ((</>))
import System.Posix.Files (createSymbolicLink)

import Yass.Glob           (expandGlob, hasGlobChars)

-- | Helper: create a file by writing empty content.
touch :: FilePath -> IO ()
touch path = writeFile path ""

-- | Run an action inside a temporary directory, restoring cwd afterwards.
-- expandGlob uses cwd internally, so we must cd into the temp dir.
withTempCwd :: String -> (FilePath -> IO a) -> IO a
withTempCwd label action = do
  origCwd <- getCurrentDirectory
  withSystemTempDirectory label $ \tmp -> do
    setCurrentDirectory tmp
    result <- action tmp
    setCurrentDirectory origCwd
    return result

spec :: Spec
spec = do
  describe "hasGlobChars" $ do
    it "returns False for a plain path" $
      hasGlobChars "foo/bar/baz.yaml" `shouldBe` False

    it "returns False for empty string" $
      hasGlobChars "" `shouldBe` False

    it "returns True for *" $
      hasGlobChars "*.yaml" `shouldBe` True

    it "returns True for ?" $
      hasGlobChars "file?.txt" `shouldBe` True

    it "returns True for [" $
      hasGlobChars "file[0-9].txt" `shouldBe` True

    it "returns True for ** in path" $
      hasGlobChars "dir/**/*.yaml" `shouldBe` True

    it "returns False for other special chars" $
      hasGlobChars "foo~bar$baz" `shouldBe` False

  describe "expandGlob" $ do
    it "returns literal path unchanged when no glob chars" $ do
      result <- expandGlob "some/path/file.yaml"
      result `shouldBe` Right ["some/path/file.yaml"]

    it "returns literal path even if it does not exist" $ do
      result <- expandGlob "nonexistent/path.txt"
      result `shouldBe` Right ["nonexistent/path.txt"]

    it "matches files with single star" $
      withTempCwd "yass-glob" $ \_ -> do
        touch "alpha.yaml"
        touch "beta.yaml"
        touch "gamma.txt"
        result <- expandGlob "*.yaml"
        case result of
          Left e  -> expectationFailure ("unexpected error: " ++ e)
          Right r -> do
            r `shouldContain` ["alpha.yaml"]
            r `shouldContain` ["beta.yaml"]
            length r `shouldBe` 2

    it "does not match files in subdirectories with single star" $
      withTempCwd "yass-glob" $ \_ -> do
        createDirectoryIfMissing True "sub"
        touch "top.yaml"
        touch "sub/deep.yaml"
        result <- expandGlob "*.yaml"
        case result of
          Left e  -> expectationFailure ("unexpected error: " ++ e)
          Right r -> do
            r `shouldContain` ["top.yaml"]
            r `shouldNotContain` ["sub/deep.yaml"]

    it "matches files recursively with doublestar" $
      withTempCwd "yass-glob" $ \_ -> do
        createDirectoryIfMissing True "a/b"
        touch "top.yaml"
        touch "a/mid.yaml"
        touch "a/b/deep.yaml"
        result <- expandGlob "**/*.yaml"
        case result of
          Left e  -> expectationFailure ("unexpected error: " ++ e)
          Right r -> do
            r `shouldContain` ["top.yaml"]
            r `shouldContain` ["a/mid.yaml"]
            r `shouldContain` ["a/b/deep.yaml"]

    it "matches single character with ?" $
      withTempCwd "yass-glob" $ \_ -> do
        touch "f1.txt"
        touch "f2.txt"
        touch "f10.txt"
        result <- expandGlob "f?.txt"
        case result of
          Left e  -> expectationFailure ("unexpected error: " ++ e)
          Right r -> do
            r `shouldContain` ["f1.txt"]
            r `shouldContain` ["f2.txt"]
            r `shouldNotContain` ["f10.txt"]

    it "excludes hidden files" $
      withTempCwd "yass-glob" $ \_ -> do
        touch "visible.txt"
        touch ".hidden.txt"
        result <- expandGlob "*.txt"
        case result of
          Left e  -> expectationFailure ("unexpected error: " ++ e)
          Right r -> do
            r `shouldContain` ["visible.txt"]
            r `shouldNotContain` [".hidden.txt"]

    it "excludes files inside hidden directories" $
      withTempCwd "yass-glob" $ \_ -> do
        createDirectoryIfMissing True ".secret"
        createDirectoryIfMissing True "visible"
        touch ".secret/file.yaml"
        touch "visible/file.yaml"
        result <- expandGlob "**/*.yaml"
        case result of
          Left e  -> expectationFailure ("unexpected error: " ++ e)
          Right r -> do
            r `shouldContain` ["visible/file.yaml"]
            r `shouldNotContain` [".secret/file.yaml"]

    it "returns error when glob matches zero files" $
      withTempCwd "yass-glob" $ \_ -> do
        touch "something.txt"
        result <- expandGlob "*.yaml"
        case result of
          Left msg -> msg `shouldContain` "no files matched"
          Right _  -> expectationFailure "expected Left for no matches"

    it "returns error for empty directory with doublestar" $
      withTempCwd "yass-glob" $ \_ -> do
        -- Empty directory, nothing to match
        result <- expandGlob "**/*.yaml"
        case result of
          Left msg -> msg `shouldContain` "no files matched"
          Right _  -> expectationFailure "expected Left for no matches"

    it "sorts results by Unicode code-point order" $
      withTempCwd "yass-glob" $ \_ -> do
        touch "charlie.txt"
        touch "alpha.txt"
        touch "bravo.txt"
        result <- expandGlob "*.txt"
        result `shouldBe` Right ["alpha.txt", "bravo.txt", "charlie.txt"]

    it "sorts results from multiple directories" $
      withTempCwd "yass-glob" $ \_ -> do
        createDirectoryIfMissing True "b"
        createDirectoryIfMissing True "a"
        touch "b/second.yaml"
        touch "a/first.yaml"
        touch "top.yaml"
        result <- expandGlob "**/*.yaml"
        case result of
          Left e  -> expectationFailure ("unexpected error: " ++ e)
          Right r -> do
            -- Code-point order: 'a' < 'b' < 't'
            r `shouldBe` ["a/first.yaml", "b/second.yaml", "top.yaml"]

    it "does not follow symbolic links (files)" $
      withTempCwd "yass-glob" $ \_ -> do
        touch "real.txt"
        createSymbolicLink "real.txt" "link.txt"
        result <- expandGlob "*.txt"
        case result of
          Left e  -> expectationFailure ("unexpected error: " ++ e)
          Right r -> do
            r `shouldContain` ["real.txt"]
            r `shouldNotContain` ["link.txt"]

    it "does not follow symbolic links (directories)" $
      withTempCwd "yass-glob" $ \_ -> do
        createDirectoryIfMissing True "realdir"
        touch "realdir/file.txt"
        createSymbolicLink "realdir" "linkdir"
        result <- expandGlob "**/*.txt"
        case result of
          Left e  -> expectationFailure ("unexpected error: " ++ e)
          Right r -> do
            r `shouldContain` ["realdir/file.txt"]
            r `shouldNotContain` ["linkdir/file.txt"]

    it "does not match directories themselves" $
      withTempCwd "yass-glob" $ \_ -> do
        createDirectoryIfMissing True "subdir"
        touch "file.txt"
        result <- expandGlob "*"
        case result of
          Left e  -> expectationFailure ("unexpected error: " ++ e)
          Right r -> do
            r `shouldContain` ["file.txt"]
            r `shouldNotContain` ["subdir"]
