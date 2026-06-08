{-# LANGUAGE OverloadedStrings #-}

module Yass.QuerySpec (spec) where

import qualified Data.ByteString as BS
import System.Directory (createDirectoryIfMissing)
import System.Exit (ExitCode(..))
import System.FilePath ((</>))
import System.IO (hSetBuffering, BufferMode(..), stdout, stderr)
import System.IO.Temp (withSystemTempDirectory)
import Test.Hspec
import Yass.Query (runQuery)

-- Helper to encode a string to ByteString
encodeUtf8 :: String -> BS.ByteString
encodeUtf8 = BS.pack . map (fromIntegral . fromEnum)

-- Write a simple yass file with a preamble and spec
writeSimpleFile :: FilePath -> String -> String -> IO ()
writeSimpleFile dir filename specName = do
  let content = "description: test\nversion: v1\n---\nspec: " ++ specName ++ "\nINPUT:\n  - MUST: exist\n"
  BS.writeFile (dir </> filename) (encodeUtf8 content)

-- Write a yass file with a custom description
writeFileWithDesc :: FilePath -> String -> String -> String -> IO ()
writeFileWithDesc dir filename specName desc = do
  let content = "description: " ++ desc ++ "\nversion: v1\n---\nspec: " ++ specName ++ "\nINPUT:\n  - MUST: exist\n"
  BS.writeFile (dir </> filename) (encodeUtf8 content)

-- Set up a temp directory with .git marker and a yass file
withYassProject :: (FilePath -> IO a) -> IO a
withYassProject action =
  withSystemTempDirectory "yass-query-test" $ \tmpDir -> do
    createDirectoryIfMissing False (tmpDir </> ".git")
    writeSimpleFile tmpDir "test.yass.yaml" "MySpec"
    action tmpDir

spec :: Spec
spec = beforeAll_ (hSetBuffering stdout LineBuffering >> hSetBuffering stderr LineBuffering) $
  describe "Query orchestrator" $ do

    describe "runQuery argument validation" $ do
      it "exits 2 with no arguments (missing name)" $ do
        result <- runQuery []
        result `shouldBe` ExitFailure 2

      it "treats whitespace-only name as no-match (exit 1) per spec" $ do
        result <- runQuery ["  "]
        result `shouldBe` ExitFailure 1

      it "exits 2 for empty string name" $ do
        result <- runQuery [""]
        result `shouldBe` ExitFailure 2

      it "exits 2 for colon in scope path" $
        withYassProject $ \_ -> do
          result <- runQuery ["MySpec", "some:path"]
          result `shouldBe` ExitFailure 2

      it "exits 2 for nonexistent scope path" $
        withYassProject $ \_ -> do
          result <- runQuery ["MySpec", "/nonexistent/path/for/testing"]
          result `shouldBe` ExitFailure 2

    describe "runQuery no match" $ do
      it "exits 1 when spec name not found in scope" $
        withYassProject $ \tmpDir -> do
          result <- runQuery ["NonExistentSpec", tmpDir]
          result `shouldBe` ExitFailure 1

    describe "runQuery single match" $ do
      it "exits 0 for exact match" $
        withYassProject $ \tmpDir -> do
          result <- runQuery ["MySpec", tmpDir]
          result `shouldBe` ExitSuccess

      it "exits 0 for suffix match" $
        withYassProject $ \tmpDir -> do
          writeSimpleFile tmpDir "auth.yass.yaml" "Auth.Login"
          result <- runQuery ["Login", tmpDir]
          result `shouldBe` ExitSuccess

    describe "runQuery multi match" $ do
      it "exits 0 with disambiguation for multiple matching specs" $
        withYassProject $ \tmpDir -> do
          writeSimpleFile tmpDir "other.yass.yaml" "Auth.Login"
          writeSimpleFile tmpDir "another.yass.yaml" "OAuth.Login"
          result <- runQuery ["Login", tmpDir]
          result `shouldBe` ExitSuccess

      it "exits 0 with disambiguation when three specs match suffix" $
        withYassProject $ \tmpDir -> do
          writeSimpleFile tmpDir "a.yass.yaml" "X.Create"
          writeSimpleFile tmpDir "b.yass.yaml" "Y.Create"
          writeSimpleFile tmpDir "c.yass.yaml" "Z.Create"
          result <- runQuery ["Create", tmpDir]
          result `shouldBe` ExitSuccess

    describe "runQuery scope handling" $ do
      it "exits 2 when scope path is empty directory (no yass files)" $
        withYassProject $ \tmpDir -> do
          let emptyDir = tmpDir </> "empty"
          createDirectoryIfMissing True emptyDir
          result <- runQuery ["MySpec", emptyDir]
          result `shouldBe` ExitFailure 2

      it "exits 0 when scope is a file path" $
        withYassProject $ \tmpDir -> do
          result <- runQuery ["MySpec", tmpDir </> "test.yass.yaml"]
          result `shouldBe` ExitSuccess

      it "exits 2 for scope path that does not exist at all" $
        withYassProject $ \_ -> do
          result <- runQuery ["MySpec", "/tmp/definitely-not-a-real-path-zzz"]
          result `shouldBe` ExitFailure 2

    describe "runQuery with parse failures" $ do
      it "exits 1 when yass file has parse errors (no match)" $
        withYassProject $ \tmpDir -> do
          -- Write an unparseable file and search for a spec only in that file
          BS.writeFile (tmpDir </> "bad.yass.yaml") (encodeUtf8 "")
          result <- runQuery ["SomeSpec", tmpDir </> "bad.yass.yaml"]
          result `shouldBe` ExitFailure 1

      it "skips unparseable files and still finds valid specs" $
        withYassProject $ \tmpDir -> do
          -- Write a bad file alongside the good one
          BS.writeFile (tmpDir </> "bad.yass.yaml") (encodeUtf8 "")
          result <- runQuery ["MySpec", tmpDir]
          result `shouldBe` ExitSuccess

    describe "runQuery integration" $ do
      it "handles spec with multiple slots" $
        withYassProject $ \tmpDir -> do
          let content = "description: test\nversion: v1\n---\nspec: Multi\nINPUT:\n  - MUST: have input\nRETURN:\n  - MUST: return result\n"
          BS.writeFile (tmpDir </> "multi.yass.yaml") (encodeUtf8 content)
          result <- runQuery ["Multi", tmpDir]
          result `shouldBe` ExitSuccess

      it "handles multiple spec files in project" $
        withYassProject $ \tmpDir -> do
          writeSimpleFile tmpDir "a.yass.yaml" "SpecA"
          writeSimpleFile tmpDir "b.yass.yaml" "SpecB"
          result <- runQuery ["SpecA", tmpDir]
          result `shouldBe` ExitSuccess

      it "handles subdirectory scope" $
        withYassProject $ \tmpDir -> do
          let subDir = tmpDir </> "sub"
          createDirectoryIfMissing True subDir
          writeSimpleFile subDir "nested.yass.yaml" "Nested"
          result <- runQuery ["Nested", subDir]
          result `shouldBe` ExitSuccess

      it "exits 1 for no match across multiple files" $
        withYassProject $ \tmpDir -> do
          writeSimpleFile tmpDir "a.yass.yaml" "Alpha"
          writeSimpleFile tmpDir "b.yass.yaml" "Beta"
          result <- runQuery ["Gamma", tmpDir]
          result `shouldBe` ExitFailure 1

    describe "extractDesc coverage" $ do
      it "handles preamble with multiline description" $
        withYassProject $ \tmpDir -> do
          writeFileWithDesc tmpDir "desc.yass.yaml" "Described" "a multi\n  line description"
          writeFileWithDesc tmpDir "desc2.yass.yaml" "Described2" "another multi\n  line description"
          -- Trigger disambiguation to exercise extractDesc
          result <- runQuery ["Described", tmpDir]
          -- Should match both via suffix matching - but "Described" is exact, so single match
          result `shouldBe` ExitSuccess

      it "handles disambiguation with descriptions" $
        withYassProject $ \tmpDir -> do
          writeFileWithDesc tmpDir "d1.yass.yaml" "Ns.Thing" "first thing"
          writeFileWithDesc tmpDir "d2.yass.yaml" "Other.Thing" "second thing"
          result <- runQuery ["Thing", tmpDir]
          result `shouldBe` ExitSuccess

      it "handles preamble with no description key" $
        withYassProject $ \tmpDir -> do
          -- Write file with no description (just version)
          BS.writeFile (tmpDir </> "nodesc.yass.yaml") (encodeUtf8 "version: v1\n---\nspec: Ns.NoDesc\nINPUT:\n  - MUST: x\n")
          BS.writeFile (tmpDir </> "nodesc2.yass.yaml") (encodeUtf8 "version: v1\n---\nspec: Other.NoDesc\nINPUT:\n  - MUST: x\n")
          result <- runQuery ["NoDesc", tmpDir]
          result `shouldBe` ExitSuccess

      it "handles disambiguation where preamble description is a non-string value" $
        withYassProject $ \tmpDir -> do
          -- Write file where description is a list (non-string)
          BS.writeFile (tmpDir </> "listdesc.yass.yaml") (encodeUtf8 "description:\n  - one\n  - two\nversion: v1\n---\nspec: Ns.ListDesc\nINPUT:\n  - MUST: x\n")
          BS.writeFile (tmpDir </> "listdesc2.yass.yaml") (encodeUtf8 "description:\n  - three\nversion: v1\n---\nspec: Other.ListDesc\nINPUT:\n  - MUST: x\n")
          result <- runQuery ["ListDesc", tmpDir]
          result `shouldBe` ExitSuccess
