{-# LANGUAGE OverloadedStrings #-}

module Yass.ListSpec (spec) where

import Test.Hspec
import qualified Data.Text as T
import Control.Exception (finally)
import System.IO.Temp (withSystemTempDirectory)
import System.Directory
  ( createDirectoryIfMissing
  , getCurrentDirectory
  , setCurrentDirectory
  )
import System.FilePath ((</>))
import System.Exit (ExitCode(..))
import System.IO
  ( hFlush, stdout, stderr
  , hClose, openFile, IOMode(..)
  )
import GHC.IO.Handle (hSetBuffering, BufferMode(..), hDuplicate, hDuplicateTo)

import Yass.List (runList, normalizeDescription, truncateDescription)

-- | Capture stdout and stderr from an IO action that returns ExitCode.
-- Returns (exitCode, stdoutText, stderrText).
captureOutput :: IO ExitCode -> IO (ExitCode, String, String)
captureOutput action =
  withSystemTempDirectory "yass-list-capture" $ \tmpDir -> do
    let outFile = tmpDir </> "stdout.txt"
        errFile = tmpDir </> "stderr.txt"

    -- Save original handles
    origOut <- hDuplicate stdout
    origErr <- hDuplicate stderr

    -- Open capture files
    hOut <- openFile outFile WriteMode
    hErr <- openFile errFile WriteMode
    hSetBuffering hOut LineBuffering
    hSetBuffering hErr LineBuffering

    -- Redirect
    hDuplicateTo hOut stdout
    hDuplicateTo hErr stderr

    -- Run the action
    exitCode <- action

    -- Flush and restore
    hFlush stdout
    hFlush stderr
    hDuplicateTo origOut stdout
    hDuplicateTo origErr stderr
    hClose hOut
    hClose hErr
    hClose origOut
    hClose origErr

    -- Read captured output
    outContent <- readFile outFile
    errContent <- readFile errFile
    -- Force evaluation
    _ <- return $! length outContent
    _ <- return $! length errContent
    return (exitCode, outContent, errContent)

-- | Helper: run runList in a given directory, capturing output.
runListIn :: FilePath -> [String] -> IO (ExitCode, String, String)
runListIn dir args = do
  savedCwd <- getCurrentDirectory
  setCurrentDirectory dir
  captureOutput (runList args) `finally` setCurrentDirectory savedCwd

-- | Helper to write a minimal valid preamble-only .yass.yaml file
writePreamble :: FilePath -> String -> IO ()
writePreamble path desc =
  writeFile path $ "description: " ++ desc ++ "\nversion: v1\n"

-- | Helper to write a preamble + one spec .yass.yaml file
writeSpecFile :: FilePath -> String -> String -> IO ()
writeSpecFile path desc specName =
  writeFile path $ unlines
    [ "description: " ++ desc
    , "version: v1"
    , "---"
    , "spec: " ++ specName
    ]

-- | Helper to write a preamble + multiple specs .yass.yaml file
writeMultiSpecFile :: FilePath -> String -> [String] -> IO ()
writeMultiSpecFile path desc specNames =
  writeFile path $ unlines $
    [ "description: " ++ desc
    , "version: v1"
    ] ++ concatMap (\n -> ["---", "spec: " ++ n]) specNames

-- | Helper to write an invalid YAML .yass.yaml file
writeInvalidYaml :: FilePath -> IO ()
writeInvalidYaml path =
  writeFile path "{{invalid yaml content}}"

-- | Helper to create a project root marker (.git directory)
createGitMarker :: FilePath -> IO ()
createGitMarker dir = createDirectoryIfMissing True (dir </> ".git")

spec :: Spec
spec = describe "Yass.List" $ do

  -- ── Pure function tests: normalizeDescription ─────────────────────────

  describe "normalizeDescription" $ do
    it "collapses whitespace runs to single space" $
      normalizeDescription "hello    world" `shouldBe` "hello world"

    it "strips leading and trailing whitespace" $
      normalizeDescription "  hello world  " `shouldBe` "hello world"

    it "handles newlines and tabs" $
      normalizeDescription "hello\n\tworld\tagain" `shouldBe` "hello world again"

    it "handles empty string" $
      normalizeDescription "" `shouldBe` ""

    it "collapses mixed whitespace runs" $
      normalizeDescription "  hello \t\n  world  " `shouldBe` "hello world"

    it "handles single word" $
      normalizeDescription "hello" `shouldBe` "hello"

    it "handles only whitespace" $
      normalizeDescription "   \t\n   " `shouldBe` ""

    it "preserves single interior space" $
      normalizeDescription "hello world" `shouldBe` "hello world"

  -- ── Pure function tests: truncateDescription ──────────────────────────

  describe "truncateDescription" $ do
    it "returns empty for empty description" $
      truncateDescription 10 5 80 "" `shouldBe` ""

    it "returns description as-is when it fits" $
      -- prefix = 10 + 5 + 2 = 17, available = 80 - 17 = 63
      truncateDescription 10 5 80 "short desc" `shouldBe` "short desc"

    it "truncates with '...' marker when too long" $ do
      -- prefix = 10 + 5 + 2 = 17, available = 30 - 17 = 13, truncLen = 13 - 3 = 10
      let desc = "12345678901234567890"  -- 20 chars
      truncateDescription 10 5 30 desc `shouldBe` "1234567890..."

    it "returns empty when prefix exceeds width" $ do
      -- prefix = 50 + 30 + 2 = 82 >= 80
      truncateDescription 50 30 80 "some description" `shouldBe` ""

    it "returns empty when prefix + marker == width" $ do
      -- prefix = 40 + 30 + 2 = 72, marker = 3, 72+3 = 75 >= 75
      truncateDescription 40 30 75 "some description" `shouldBe` ""

    it "returns description when exactly fits available space" $ do
      -- prefix = 5 + 5 + 2 = 12, available = 22 - 12 = 10
      let desc = "1234567890"  -- exactly 10 chars
      truncateDescription 5 5 22 desc `shouldBe` "1234567890"

    it "truncates when one char over available" $ do
      -- prefix = 5 + 5 + 2 = 12, available = 22 - 12 = 10, truncLen = 10 - 3 = 7
      let desc = "12345678901"  -- 11 chars
      truncateDescription 5 5 22 desc `shouldBe` "1234567..."

    it "returns empty when truncLen would be zero" $ do
      -- prefix = 10 + 5 + 2 = 17, width = 20, available = 3, truncLen = 0
      truncateDescription 10 5 20 "hello" `shouldBe` ""

  -- ── Integration tests: runList with temp directories ──────────────────

  describe "runList" $ do
    it "emits correct tab-separated rows for a valid .yass.yaml file" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        writeSpecFile (tmpDir </> "api.yass.yaml") "API spec" "api.endpoint"
        (exit, out, _err) <- runListIn tmpDir []
        exit `shouldBe` ExitSuccess
        let rows = lines out
        length rows `shouldBe` 1
        let cols = splitTabs (head rows)
        length cols `shouldBe` 3
        cols !! 0 `shouldBe` "api.yass.yaml"
        cols !! 1 `shouldBe` "api.endpoint"
        cols !! 2 `shouldBe` "API spec"

    it "emits no rows when no specs in file" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        writePreamble (tmpDir </> "empty.yass.yaml") "No specs here"
        (exit, out, _err) <- runListIn tmpDir []
        exit `shouldBe` ExitSuccess
        out `shouldBe` ""

    it "emits error to stderr and exits 1 for malformed YAML" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        writeInvalidYaml (tmpDir </> "bad.yass.yaml")
        (exit, _out, err) <- runListIn tmpDir []
        exit `shouldBe` ExitFailure 1
        err `shouldSatisfy` (not . null)
        err `shouldSatisfy` \e -> "YAML well-formedness error" `isInfixOfStr` e

    it "exits 0 with no output when no files found" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        (exit, out, _err) <- runListIn tmpDir []
        exit `shouldBe` ExitSuccess
        out `shouldBe` ""

    it "exits 2 for non-existent explicit path" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        (exit, _out, err) <- runListIn tmpDir [tmpDir </> "nonexistent.yass.yaml"]
        exit `shouldBe` ExitFailure 2
        err `shouldSatisfy` (not . null)

    it "exits 2 for file with bad extension" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        writeFile (tmpDir </> "test.yaml") "description: test\nversion: v1\n"
        (exit, _out, err) <- runListIn tmpDir [tmpDir </> "test.yaml"]
        exit `shouldBe` ExitFailure 2
        err `shouldSatisfy` \e -> "bad_extension" `isInfixOfStr` e

    it "preserves document order of specs within a file" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        writeMultiSpecFile (tmpDir </> "multi.yass.yaml") "Multi spec"
          ["z.spec", "a.spec", "m.spec"]
        (exit, out, _err) <- runListIn tmpDir []
        exit `shouldBe` ExitSuccess
        let rows = lines out
        length rows `shouldBe` 3
        -- Specs should be in document order, not sorted
        let specNames = map (\r -> splitTabs r !! 1) rows
        specNames `shouldBe` ["z.spec", "a.spec", "m.spec"]

    it "exits 2 for colon in path" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        (exit, _out, err) <- runListIn tmpDir ["some:path"]
        exit `shouldBe` ExitFailure 2
        err `shouldSatisfy` \e -> "colon" `isInfixOfStr` e

    it "exits 2 when no project root found" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        -- No .git marker
        (exit, _out, err) <- runListIn tmpDir []
        exit `shouldBe` ExitFailure 2
        err `shouldSatisfy` (not . null)

    it "processes multiple files sorted by NFC order" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        writeSpecFile (tmpDir </> "z-api.yass.yaml") "Z API" "z.spec"
        writeSpecFile (tmpDir </> "a-api.yass.yaml") "A API" "a.spec"
        (exit, out, _err) <- runListIn tmpDir []
        exit `shouldBe` ExitSuccess
        let rows = lines out
        length rows `shouldBe` 2
        -- Files should be sorted: a-api before z-api
        let fileNames = map (\r -> splitTabs r !! 0) rows
        fileNames `shouldBe` ["a-api.yass.yaml", "z-api.yass.yaml"]

    it "continues processing after parse error in one file" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        writeSpecFile (tmpDir </> "good.yass.yaml") "Good" "good.spec"
        writeInvalidYaml (tmpDir </> "bad.yass.yaml")
        (exit, out, err) <- runListIn tmpDir []
        -- Should be ExitFailure 1 because one file failed parse
        exit `shouldBe` ExitFailure 1
        -- The good file should still produce output
        out `shouldSatisfy` \o -> "good.spec" `isInfixOfStr` o
        -- The bad file should produce an error
        err `shouldSatisfy` (not . null)

    it "handles explicit file path argument" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        writeSpecFile (tmpDir </> "target.yass.yaml") "Target" "target.spec"
        writeSpecFile (tmpDir </> "other.yass.yaml") "Other" "other.spec"
        (exit, out, _err) <- runListIn tmpDir [tmpDir </> "target.yass.yaml"]
        exit `shouldBe` ExitSuccess
        let rows = lines out
        length rows `shouldBe` 1
        splitTabs (head rows) !! 1 `shouldBe` "target.spec"

    it "handles directory argument" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        let subDir = tmpDir </> "specs"
        createDirectoryIfMissing True subDir
        writeSpecFile (subDir </> "sub.yass.yaml") "Sub" "sub.spec"
        (exit, out, _err) <- runListIn tmpDir [subDir]
        exit `shouldBe` ExitSuccess
        let rows = lines out
        length rows `shouldBe` 1
        splitTabs (head rows) !! 1 `shouldBe` "sub.spec"

    it "normalizes description whitespace in output" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        writeFile (tmpDir </> "ws.yass.yaml") $ unlines
          [ "description: \"  hello   world  \""
          , "version: v1"
          , "---"
          , "spec: ws.spec"
          ]
        (exit, out, _err) <- runListIn tmpDir []
        exit `shouldBe` ExitSuccess
        let rows = lines out
        length rows `shouldBe` 1
        let desc = splitTabs (head rows) !! 2
        desc `shouldBe` "hello world"

    it "handles file with empty description" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        writeFile (tmpDir </> "nodesc.yass.yaml") $ unlines
          [ "description: \"\""
          , "version: v1"
          , "---"
          , "spec: no.desc"
          ]
        (exit, out, _err) <- runListIn tmpDir []
        exit `shouldBe` ExitSuccess
        let rows = lines out
        length rows `shouldBe` 1
        -- The third column (description) should be empty
        let cols = splitTabs (head rows)
        cols !! 2 `shouldBe` ""

    it "emits multiple rows for multi-spec file with correct file column" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        writeMultiSpecFile (tmpDir </> "multi.yass.yaml") "Multi"
          ["alpha", "beta"]
        (exit, out, _err) <- runListIn tmpDir []
        exit `shouldBe` ExitSuccess
        let rows = lines out
        length rows `shouldBe` 2
        -- All rows should reference the same file
        let fileNames = map (\r -> splitTabs r !! 0) rows
        fileNames `shouldBe` ["multi.yass.yaml", "multi.yass.yaml"]
        -- And the same description
        let descs = map (\r -> splitTabs r !! 2) rows
        descs `shouldBe` ["Multi", "Multi"]

    it "handles file with zero YAML documents (comment-only, ParseOK [])" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        -- Write a comment-only file (parser returns ParseOK [])
        writeFile (tmpDir </> "commentonly.yass.yaml") "# just a comment\n"
        (exit, out, _err) <- runListIn tmpDir []
        -- Zero docs => no output, no error
        exit `shouldBe` ExitSuccess
        out `shouldBe` ""

    it "exits 1 when a parse failure occurs among multiple files" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        writeSpecFile (tmpDir </> "a-good.yass.yaml") "Good" "good.spec"
        writeInvalidYaml (tmpDir </> "b-bad.yass.yaml")
        writeSpecFile (tmpDir </> "c-also-good.yass.yaml") "Also good" "also.spec"
        (exit, out, err) <- runListIn tmpDir []
        exit `shouldBe` ExitFailure 1
        -- Good files should still produce output
        out `shouldSatisfy` \o -> "good.spec" `isInfixOfStr` o
        out `shouldSatisfy` \o -> "also.spec" `isInfixOfStr` o
        -- Bad file should produce error
        err `shouldSatisfy` \e -> "malformed" `isInfixOfStr` e

    it "handles file path with subdirectory in list output" $
      withSystemTempDirectory "yass-list-test" $ \tmpDir -> do
        createGitMarker tmpDir
        let sub = tmpDir </> "subdir"
        createDirectoryIfMissing True sub
        writeSpecFile (sub </> "nested.yass.yaml") "Nested" "nested.spec"
        (exit, out, _err) <- runListIn tmpDir []
        exit `shouldBe` ExitSuccess
        let rows = lines out
        length rows `shouldBe` 1
        -- File path should include subdirectory
        splitTabs (head rows) !! 0 `shouldBe` "subdir/nested.yass.yaml"

  -- ── Additional normalizeDescription tests ─────────────────────────────

  describe "normalizeDescription edge cases" $ do
    it "handles vertical tab and form feed" $
      normalizeDescription "hello\x0B\x0Cworld" `shouldBe` "hello world"

    it "handles carriage return" $
      normalizeDescription "hello\r\nworld" `shouldBe` "hello world"

    it "handles multiple spaces between multiple words" $
      normalizeDescription "one   two   three   four" `shouldBe` "one two three four"

    it "handles non-breaking space (Unicode)" $
      -- \x00A0 is non-breaking space; T.words treats it as whitespace
      normalizeDescription "hello\x00A0world" `shouldBe` "hello world"

  -- ── Additional truncateDescription tests ──────────────────────────────

  describe "truncateDescription edge cases" $ do
    it "returns empty when truncLen is negative" $ do
      -- prefix = 10 + 10 + 2 = 22, width = 24, available = 2, truncLen = 2 - 3 = -1
      truncateDescription 10 10 24 "some text" `shouldBe` ""

    it "truncates to exactly 1 char plus marker" $ do
      -- prefix = 5 + 5 + 2 = 12, width = 16, available = 4, truncLen = 1
      truncateDescription 5 5 16 "abcde" `shouldBe` "a..."

    it "handles very wide terminal" $ do
      -- With huge width, nothing gets truncated
      truncateDescription 10 10 1000 "a long description here" `shouldBe` "a long description here"

    it "handles width of 0" $ do
      -- prefix = 5 + 5 + 2 = 12, 12 + 3 = 15 >= 0
      truncateDescription 5 5 0 "hello" `shouldBe` ""

    it "handles description with exactly prefix + marker length" $ do
      -- prefix = 3 + 3 + 2 = 8, marker = 3, 8 + 3 = 11 >= 11
      truncateDescription 3 3 11 "hello" `shouldBe` ""

    it "handles single character description that fits" $ do
      -- prefix = 3 + 3 + 2 = 8, available = 80 - 8 = 72, desc len 1 <= 72
      truncateDescription 3 3 80 "x" `shouldBe` "x"

-- | Split a string on tab characters.
splitTabs :: String -> [String]
splitTabs "" = [""]
splitTabs s  = case break (== '\t') s of
  (before, [])     -> [before]
  (before, _:rest) -> before : splitTabs rest

-- | Check if a substring occurs in a string.
isInfixOfStr :: String -> String -> Bool
isInfixOfStr needle haystack = needle `elem` substrings haystack (length needle)
  where
    substrings str n
      | length str < n = []
      | otherwise      = take n str : substrings (tail str) n
