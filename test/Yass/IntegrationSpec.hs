{-# LANGUAGE OverloadedStrings #-}

module Yass.IntegrationSpec (spec) where

import Test.Hspec
import qualified Data.ByteString as BS
import qualified Data.Text as T
import qualified Data.Text.Encoding as TE
import Control.Exception (finally)
import System.Directory
  ( createDirectoryIfMissing
  , getCurrentDirectory
  , setCurrentDirectory
  , listDirectory
  )
import System.Exit (ExitCode(..))
import System.FilePath ((</>))
import System.IO
  ( hFlush, stdout, stderr
  , hClose, openFile, IOMode(..)
  , hSetBuffering, BufferMode(..)
  )
import System.IO.Temp (withSystemTempDirectory)
import GHC.IO.Handle (hDuplicate, hDuplicateTo)

import Yass.Validate (runValidate)
import Yass.List (runList)
import Yass.Query (runQuery)
import Yass.Types hiding (Spec, ExitSuccess)

-- | Capture stdout and stderr from an IO action that returns ExitCode.
-- Returns (exitCode, stdoutText, stderrText).
captureOutput :: IO ExitCode -> IO (ExitCode, String, String)
captureOutput action =
  withSystemTempDirectory "yass-integ-capture" $ \capDir -> do
    let outFile = capDir </> "stdout.txt"
        errFile = capDir </> "stderr.txt"

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

-- | Helper to write a yass file with text content
writeYassFile :: FilePath -> String -> IO ()
writeYassFile fp content =
  BS.writeFile fp (TE.encodeUtf8 (T.pack content))

-- | Run a test inside a temporary project directory (with .git marker).
-- Sets cwd to the temp dir and restores afterward.
withProject :: (FilePath -> IO a) -> IO a
withProject action =
  withSystemTempDirectory "yass-integ-test" $ \tmpDir -> do
    createDirectoryIfMissing False (tmpDir </> ".git")
    origCwd <- getCurrentDirectory
    setCurrentDirectory tmpDir
    action tmpDir `finally` setCurrentDirectory origCwd

-- | A valid minimal spec file content with a single spec.
validSpecContent :: String -> String -> String
validSpecContent desc specName = unlines
  [ "---"
  , "description: " ++ desc
  , "version: v1"
  , "---"
  , "spec: " ++ specName
  , "INPUT:"
  , "  - MUST: accept input"
  ]

spec :: Spec
spec = describe "Integration" $ do

  -- ── 1. Validate a valid .yass.yaml file -> exit 0, summary on stdout ──

  describe "validate valid file" $ do
    it "exits 0 with summary line on stdout for a valid file" $ do
      withProject $ \tmpDir -> do
        writeYassFile (tmpDir </> "good.yass.yaml") $ validSpecContent "Good spec" "good-spec"
        (exit, out, err) <- captureOutput (runValidate ["good.yass.yaml"])
        exit `shouldBe` ExitSuccess
        out `shouldContain` "checked 1 files, found 0 errors"
        err `shouldBe` ""

  -- ── 2. Validate a file with missing preamble -> exit 1, error on stderr ──

  describe "validate missing preamble" $ do
    it "exits 1 with preamble error when first doc has spec key" $ do
      withProject $ \tmpDir -> do
        writeYassFile (tmpDir </> "nopreamble.yass.yaml") $ unlines
          [ "---"
          , "spec: orphan"
          , "INPUT:"
          , "  - MUST: do stuff"
          ]
        (exit, out, err) <- captureOutput (runValidate ["nopreamble.yass.yaml"])
        exit `shouldBe` ExitFailure 1
        out `shouldContain` "checked 1 files"
        err `shouldContain` "yass.preamble"

  -- ── 3. Validate a file with bad spec name -> exit 1, error on stderr ──

  describe "validate bad spec name" $ do
    it "exits 1 with spec name error for names with spaces" $ do
      withProject $ \tmpDir -> do
        writeYassFile (tmpDir </> "badname.yass.yaml") $ unlines
          [ "---"
          , "description: test"
          , "version: v1"
          , "---"
          , "spec: \"bad name here\""
          ]
        (exit, _out, err) <- captureOutput (runValidate ["badname.yass.yaml"])
        exit `shouldBe` ExitFailure 1
        err `shouldContain` "yass.spec.name_bad_chars"

    it "exits 1 with spec name error for empty name" $ do
      withProject $ \tmpDir -> do
        writeYassFile (tmpDir </> "emptyname.yass.yaml") $ unlines
          [ "---"
          , "description: test"
          , "version: v1"
          , "---"
          , "spec: \"\""
          ]
        (exit, _out, err) <- captureOutput (runValidate ["emptyname.yass.yaml"])
        exit `shouldBe` ExitFailure 1
        err `shouldNotBe` ""

  -- ── 4. Validate a file with duplicate spec names -> exit 1, error on stderr ──

  describe "validate duplicate spec names" $ do
    it "exits 1 with duplicate name error" $ do
      withProject $ \tmpDir -> do
        writeYassFile (tmpDir </> "dupname.yass.yaml") $ unlines
          [ "---"
          , "description: test"
          , "version: v1"
          , "---"
          , "spec: dup-spec"
          , "INPUT:"
          , "  - MUST: first"
          , "---"
          , "spec: dup-spec"
          , "INPUT:"
          , "  - MUST: second"
          ]
        (exit, _out, err) <- captureOutput (runValidate ["dupname.yass.yaml"])
        exit `shouldBe` ExitFailure 1
        err `shouldContain` "yass.spec.duplicate_name"

  -- ── 5. Validate an empty file -> exit 1, error on stderr ──

  describe "validate empty file" $ do
    it "exits 1 with error for a zero-byte file" $ do
      withProject $ \tmpDir -> do
        let fp = tmpDir </> "empty.yass.yaml"
        BS.writeFile fp BS.empty
        (exit, out, err) <- captureOutput (runValidate ["empty.yass.yaml"])
        exit `shouldBe` ExitFailure 1
        out `shouldContain` "checked 1 files"
        err `shouldNotBe` ""

    it "exits 1 with error for a whitespace-only file" $ do
      withProject $ \tmpDir -> do
        writeYassFile (tmpDir </> "wsonly.yass.yaml") "   \n\n  \n"
        (exit, _out, err) <- captureOutput (runValidate ["wsonly.yass.yaml"])
        exit `shouldBe` ExitFailure 1
        err `shouldNotBe` ""

  -- ── 6. Validate with non-existent path -> exit 2 ──

  describe "validate non-existent path" $ do
    it "exits 2 when file does not exist" $ do
      withProject $ \_ -> do
        (exit, _out, err) <- captureOutput (runValidate ["does-not-exist.yass.yaml"])
        exit `shouldBe` ExitFailure 2
        err `shouldContain` "not_found"

    it "exits 2 when directory does not exist" $ do
      withProject $ \_ -> do
        (exit, _out, err) <- captureOutput (runValidate ["no-such-dir"])
        exit `shouldBe` ExitFailure 2
        err `shouldContain` "not_found"

  -- ── 7. List specs from a directory -> correct tab-separated output ──

  describe "list specs from directory" $ do
    it "produces tab-separated output with file, name, description" $ do
      withProject $ \tmpDir -> do
        let subDir = tmpDir </> "specs"
        createDirectoryIfMissing True subDir
        writeYassFile (subDir </> "api.yass.yaml") $ unlines
          [ "description: API specification"
          , "version: v1"
          , "---"
          , "spec: api.endpoint"
          ]
        (exit, out, _err) <- captureOutput (runList [subDir])
        exit `shouldBe` ExitSuccess
        -- Output should contain tab-separated fields
        let outLines = filter (not . null) (lines out)
        length outLines `shouldBe` 1
        let fields = splitOn '\t' (head outLines)
        length fields `shouldSatisfy` (>= 2)
        -- Second field is the spec name
        fields !! 1 `shouldBe` "api.endpoint"

    it "lists multiple specs from multiple files" $ do
      withProject $ \tmpDir -> do
        writeYassFile (tmpDir </> "a.yass.yaml") $ unlines
          [ "description: A file"
          , "version: v1"
          , "---"
          , "spec: alpha"
          ]
        writeYassFile (tmpDir </> "b.yass.yaml") $ unlines
          [ "description: B file"
          , "version: v1"
          , "---"
          , "spec: bravo"
          ]
        (exit, out, _err) <- captureOutput (runList [])
        exit `shouldBe` ExitSuccess
        let outLines = filter (not . null) (lines out)
        length outLines `shouldBe` 2
        -- Extract spec names (second tab-separated field)
        let specNames = map (\l -> splitOn '\t' l !! 1) outLines
        specNames `shouldContain` ["alpha"]
        specNames `shouldContain` ["bravo"]

  -- ── 8. List specs from empty directory -> exit 0, no output ──

  describe "list specs from empty directory" $ do
    it "exits 0 with no output when directory has no yass files" $ do
      withProject $ \tmpDir -> do
        let emptyDir = tmpDir </> "empty-specs"
        createDirectoryIfMissing True emptyDir
        (exit, out, _err) <- captureOutput (runList [emptyDir])
        exit `shouldBe` ExitSuccess
        out `shouldBe` ""

    it "exits 0 with no output when project has no yass files at all" $ do
      withProject $ \_ -> do
        (exit, out, _err) <- captureOutput (runList [])
        exit `shouldBe` ExitSuccess
        out `shouldBe` ""

  -- ── 9. Validate multiple files -> summary counts correct ──

  describe "validate multiple files" $ do
    it "reports correct file count for two valid files" $ do
      withProject $ \tmpDir -> do
        writeYassFile (tmpDir </> "one.yass.yaml") $ validSpecContent "First" "spec-one"
        writeYassFile (tmpDir </> "two.yass.yaml") $ validSpecContent "Second" "spec-two"
        (exit, out, err) <- captureOutput (runValidate ["one.yass.yaml", "two.yass.yaml"])
        exit `shouldBe` ExitSuccess
        out `shouldContain` "checked 2 files, found 0 errors"
        err `shouldBe` ""

    it "reports correct file count for three files with one error" $ do
      withProject $ \tmpDir -> do
        writeYassFile (tmpDir </> "ok1.yass.yaml") $ validSpecContent "OK 1" "ok-one"
        writeYassFile (tmpDir </> "ok2.yass.yaml") $ validSpecContent "OK 2" "ok-two"
        writeYassFile (tmpDir </> "bad.yass.yaml") $ unlines
          [ "---"
          , "spec: orphan"
          ]
        (exit, out, err) <- captureOutput
          (runValidate ["ok1.yass.yaml", "ok2.yass.yaml", "bad.yass.yaml"])
        exit `shouldBe` ExitFailure 1
        out `shouldContain` "checked 3 files"
        err `shouldNotBe` ""

    it "deduplicates the same file passed twice" $ do
      withProject $ \tmpDir -> do
        writeYassFile (tmpDir </> "dup.yass.yaml") $ validSpecContent "Dup" "dup-spec"
        (exit, out, _err) <- captureOutput
          (runValidate ["dup.yass.yaml", "dup.yass.yaml"])
        exit `shouldBe` ExitSuccess
        out `shouldContain` "checked 1 files, found 0 errors"

  -- ── 10. Validate the actual spec files in this repo ──

  describe "validate real repo spec files" $ do
    it "discovers and processes all spec/*.yass.yaml files in this repo" $ do
      -- Run from the real project root, targeting the spec/ directory.
      -- The repo spec files may have cross-file ref issues, so we just
      -- verify the tool discovers, parses, and reports on the right
      -- number of files.
      origCwd <- getCurrentDirectory
      let repoRoot = "/Users/shakefu/git/shakefu/yass.haskell-cli"
      setCurrentDirectory repoRoot
      flip finally (setCurrentDirectory origCwd) $ do
        specFiles <- listDirectory (repoRoot </> "spec")
        let yassFiles = filter isYassYaml specFiles
        -- Ensure we actually found spec files to validate
        length yassFiles `shouldSatisfy` (> 0)
        (exit, out, _err) <- captureOutput (runValidate ["spec"])
        -- The summary must report the correct number of files checked
        out `shouldContain` ("checked " ++ show (length yassFiles) ++ " files")
        -- The exit code should be 0 or 1 (processing), never 2 (usage/path error)
        exit `shouldNotBe` ExitFailure 2

    it "lists specs from the real repo spec directory" $ do
      origCwd <- getCurrentDirectory
      let repoRoot = "/Users/shakefu/git/shakefu/yass.haskell-cli"
      setCurrentDirectory repoRoot
      flip finally (setCurrentDirectory origCwd) $ do
        (exit, out, _err) <- captureOutput (runList ["spec"])
        exit `shouldBe` ExitSuccess
        -- Should have multiple lines of output (one per spec)
        let outLines = filter (not . null) (lines out)
        length outLines `shouldSatisfy` (> 0)
        -- Each line should be tab-separated with at least 2 fields
        mapM_ (\l -> length (splitOn '\t' l) `shouldSatisfy` (>= 2)) outLines

  -- ── Additional integration tests ──

  describe "validate and list consistency" $ do
    it "list succeeds on a file that validate accepts" $ do
      withProject $ \tmpDir -> do
        writeYassFile (tmpDir </> "consistent.yass.yaml") $ unlines
          [ "description: Consistency check"
          , "version: v1"
          , "---"
          , "spec: consist.alpha"
          , "INPUT:"
          , "  - MUST: accept alpha input"
          , "---"
          , "spec: consist.beta"
          , "RETURN:"
          , "  - MUST: return beta output"
          ]
        -- Validate should pass
        (vExit, vOut, vErr) <- captureOutput (runValidate ["consistent.yass.yaml"])
        vExit `shouldBe` ExitSuccess
        vOut `shouldContain` "found 0 errors"
        vErr `shouldBe` ""
        -- List should show both specs
        (lExit, lOut, _lErr) <- captureOutput (runList ["consistent.yass.yaml"])
        lExit `shouldBe` ExitSuccess
        let outLines = filter (not . null) (lines lOut)
        length outLines `shouldBe` 2
        let specNames = map (\l -> splitOn '\t' l !! 1) outLines
        specNames `shouldContain` ["consist.alpha"]
        specNames `shouldContain` ["consist.beta"]

  describe "query integration" $ do
    it "queries a spec by name and returns its fragment" $ do
      withProject $ \tmpDir -> do
        writeYassFile (tmpDir </> "q.yass.yaml") $ unlines
          [ "description: Query test"
          , "version: v1"
          , "---"
          , "spec: query-target"
          , "INPUT:"
          , "  - MUST: accept query input"
          ]
        (exit, out, _err) <- captureOutput (runQuery ["query-target"])
        exit `shouldBe` ExitSuccess
        out `shouldContain` "spec: query-target"
        out `shouldContain` "MUST"

    it "exits 1 when querying a non-existent spec name" $ do
      withProject $ \tmpDir -> do
        writeYassFile (tmpDir </> "q.yass.yaml") $ validSpecContent "Q" "existing-spec"
        (exit, _out, err) <- captureOutput (runQuery ["nonexistent-name"])
        exit `shouldBe` ExitFailure 1
        err `shouldContain` "yass.query.no_match"

    it "exits 2 when no spec name is given" $ do
      withProject $ \_ -> do
        (exit, _out, err) <- captureOutput (runQuery [])
        exit `shouldBe` ExitFailure 2
        err `shouldContain` "yass.query.name_missing"

  describe "cross-command path handling" $ do
    it "validate exits 2 for colon in path" $ do
      withProject $ \_ -> do
        (exit, _out, err) <- captureOutput (runValidate ["file:with:colon"])
        exit `shouldBe` ExitFailure 2
        err `shouldContain` "yass.path.colon_in_path"

    it "list exits 2 for colon in path" $ do
      withProject $ \_ -> do
        (exit, _out, err) <- captureOutput (runList ["file:with:colon"])
        exit `shouldBe` ExitFailure 2
        err `shouldContain` "yass.path.colon_in_path"

    it "validate discovers files in subdirectory" $ do
      withProject $ \tmpDir -> do
        let sub = tmpDir </> "nested" </> "specs"
        createDirectoryIfMissing True sub
        writeYassFile (sub </> "deep.yass.yaml") $ validSpecContent "Deep" "deep-spec"
        (exit, out, _err) <- captureOutput (runValidate ["nested"])
        exit `shouldBe` ExitSuccess
        out `shouldContain` "checked 1 files, found 0 errors"

  describe "error output routing" $ do
    it "sends error details to stderr and summary to stdout" $ do
      withProject $ \tmpDir -> do
        writeYassFile (tmpDir </> "routing.yass.yaml") $ unlines
          [ "---"
          , "spec: misplaced"
          ]
        (exit, out, err) <- captureOutput (runValidate ["routing.yass.yaml"])
        exit `shouldBe` ExitFailure 1
        -- Summary goes to stdout
        out `shouldContain` "checked"
        out `shouldContain` "files"
        -- Error codes go to stderr
        err `shouldNotBe` ""
        -- stdout must not contain error code brackets
        out `shouldNotContain` "[yass."

-- | Split a string on a delimiter character.
splitOn :: Char -> String -> [String]
splitOn _ [] = [""]
splitOn delim s = go s
  where
    go [] = [""]
    go xs = let (field, rest) = break (== delim) xs
            in field : case rest of
                         []     -> []
                         (_:ys) -> go ys

-- | Check if a filename has .yass.yaml suffix (simple helper for listing).
isYassYaml :: String -> Bool
isYassYaml name =
  let suffix = ".yass.yaml"
      sLen = length suffix
      nLen = length name
  in nLen > sLen && drop (nLen - sLen) name == suffix
