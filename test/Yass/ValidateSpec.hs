{-# LANGUAGE OverloadedStrings #-}

module Yass.ValidateSpec (spec) where

import Test.Hspec
import qualified Data.ByteString as BS
import qualified Data.Text as T
import qualified Data.Text.Encoding as TE
import Control.Exception (finally)
import System.Directory (getCurrentDirectory, setCurrentDirectory, createDirectoryIfMissing)
import System.Exit (ExitCode(..))
import System.FilePath ((</>))
import System.IO (hFlush, stderr, stdout, hClose, openFile, IOMode(..))
import System.IO.Temp (withSystemTempDirectory)
import GHC.IO.Handle (hSetBuffering, BufferMode(..), hDuplicate, hDuplicateTo)

import Yass.Validate (runValidate)

-- | Capture stdout and stderr from an IO action that returns ExitCode.
-- Returns (exitCode, stdoutText, stderrText).
captureOutput :: IO ExitCode -> IO (ExitCode, String, String)
captureOutput action =
  withSystemTempDirectory "yass-validate-capture" $ \tmpDir -> do
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

-- | Helper to write a file with text content
writeYassFile :: FilePath -> String -> IO ()
writeYassFile fp content =
  BS.writeFile fp (TE.encodeUtf8 (T.pack content))

-- | Run a test with a temporary directory that has a .git marker (for project root detection).
-- Changes cwd to the temp dir and restores it afterward.
withProject :: (FilePath -> IO a) -> IO a
withProject action =
  withSystemTempDirectory "yass-validate-test" $ \tmpDir -> do
    -- Create .git marker for project root detection
    createDirectoryIfMissing False (tmpDir </> ".git")
    origCwd <- getCurrentDirectory
    setCurrentDirectory tmpDir
    action tmpDir `finally` setCurrentDirectory origCwd

-- | Run a test with a temp dir that has .git, write some files, run validate, return captured output.
withProjectAndFiles :: [(FilePath, String)] -> [String] -> IO (ExitCode, String, String)
withProjectAndFiles files args = withProject $ \tmpDir -> do
  -- Write all files
  mapM_ (\(name, content) -> do
    let fp = tmpDir </> name
        dir = takeDir fp
    createDirectoryIfMissing True dir
    writeYassFile fp content
    ) files
  captureOutput (runValidate args)
  where
    takeDir = reverse . dropWhile (/= '/') . reverse

spec :: Spec
spec = describe "runValidate" $ do

  -- 1. Valid file with no errors
  it "reports 0 errors for a valid file" $ do
    (exit, out, err) <- withProjectAndFiles
      [ ("test.yass.yaml", unlines
          [ "---"
          , "description: A test spec"
          , "version: v1"
          , "---"
          , "spec: my-spec"
          , "INPUT:"
          , "  - MUST: accept input"
          ])
      ]
      ["test.yass.yaml"]
    exit `shouldBe` ExitSuccess
    out `shouldContain` "checked 1 files, found 0 errors"
    err `shouldBe` ""

  -- 2. File with YAML parse error (malformed)
  it "reports 1 error for malformed YAML and exits 1" $ do
    (exit, out, err) <- withProjectAndFiles
      [ ("bad.yass.yaml", "---\n  bad:\n indent: wrong\n")
      ]
      ["bad.yass.yaml"]
    exit `shouldBe` ExitFailure 1
    out `shouldContain` "checked 1 files, found 1 errors"
    err `shouldNotBe` ""

  -- 3. File with BOM
  it "reports error for file with BOM" $ do
    withProject $ \tmpDir -> do
      let fp = tmpDir </> "bom.yass.yaml"
          bom = BS.pack [0xEF, 0xBB, 0xBF]
          content = TE.encodeUtf8 "---\ndescription: test\nversion: v1\n"
      BS.writeFile fp (BS.append bom content)
      (exit, out, err) <- captureOutput (runValidate ["bom.yass.yaml"])
      exit `shouldBe` ExitFailure 1
      out `shouldContain` "checked 1 files, found 1 errors"
      err `shouldContain` "yass.yaml.has_bom"

  -- 4. Empty file
  it "reports error for empty file" $ do
    withProject $ \tmpDir -> do
      let fp = tmpDir </> "empty.yass.yaml"
      BS.writeFile fp BS.empty
      (exit, out, err) <- captureOutput (runValidate ["empty.yass.yaml"])
      exit `shouldBe` ExitFailure 1
      out `shouldContain` "checked 1 files, found 1 errors"
      err `shouldContain` "empty"

  -- 5. Colon in path → exit 2
  it "exits 2 for colon in path argument" $ do
    withProject $ \_ -> do
      (exit, _out, err) <- captureOutput (runValidate ["some:path"])
      exit `shouldBe` ExitFailure 2
      err `shouldContain` "yass.path.colon_in_path"

  -- 6. Missing preamble (first doc has spec key)
  it "reports preamble error when first doc has spec key" $ do
    (exit, out, err) <- withProjectAndFiles
      [ ("nopreamble.yass.yaml", "---\nspec: foo\nINPUT:\n  - MUST: do stuff\n")
      ]
      ["nopreamble.yass.yaml"]
    exit `shouldBe` ExitFailure 1
    out `shouldContain` "found"
    err `shouldContain` "yass.preamble.has_spec_key"

  -- 7. Missing version in preamble
  it "reports missing version error" $ do
    (exit, _out, err) <- withProjectAndFiles
      [ ("noversion.yass.yaml", "---\ndescription: test\n---\nspec: foo\n")
      ]
      ["noversion.yass.yaml"]
    exit `shouldBe` ExitFailure 1
    err `shouldContain` "yass.preamble.missing_version"

  -- 8. Unknown version
  it "reports unknown version error" $ do
    (exit, _out, err) <- withProjectAndFiles
      [ ("badver.yass.yaml", "---\ndescription: test\nversion: v2\n---\nspec: foo\n")
      ]
      ["badver.yass.yaml"]
    exit `shouldBe` ExitFailure 1
    err `shouldContain` "yass.preamble.unknown_version"

  -- 9. Spec name with bad chars
  it "reports spec name bad chars error" $ do
    (exit, _out, err) <- withProjectAndFiles
      [ ("badchars.yass.yaml", unlines
          [ "---"
          , "description: test"
          , "version: v1"
          , "---"
          , "spec: \"foo bar\""
          ])
      ]
      ["badchars.yass.yaml"]
    exit `shouldBe` ExitFailure 1
    err `shouldContain` "yass.spec.name_bad_chars"

  -- 10. Duplicate spec names
  it "reports duplicate spec name error" $ do
    (exit, _out, err) <- withProjectAndFiles
      [ ("dupspec.yass.yaml", unlines
          [ "---"
          , "description: test"
          , "version: v1"
          , "---"
          , "spec: myspec"
          , "INPUT:"
          , "  - MUST: do stuff"
          , "---"
          , "spec: myspec"
          , "INPUT:"
          , "  - MUST: do other stuff"
          ])
      ]
      ["dupspec.yass.yaml"]
    exit `shouldBe` ExitFailure 1
    err `shouldContain` "yass.spec.duplicate_name"

  -- 11. No files found (discover finds nothing)
  it "exits 2 when no files are found" $ do
    withProject $ \_ -> do
      (exit, _out, err) <- captureOutput (runValidate [])
      exit `shouldBe` ExitFailure 2
      err `shouldContain` "yass.discover.no_files"

  -- 12. Path not found
  it "exits 2 for nonexistent path" $ do
    withProject $ \_ -> do
      (exit, _out, err) <- captureOutput (runValidate ["nonexistent.yass.yaml"])
      exit `shouldBe` ExitFailure 2
      err `shouldContain` "not_found"

  -- 13. Multiple valid files with no errors
  it "validates multiple files and reports 0 errors" $ do
    (exit, out, err) <- withProjectAndFiles
      [ ("a.yass.yaml", unlines
          [ "---"
          , "description: File A"
          , "version: v1"
          , "---"
          , "spec: spec-a"
          , "INPUT:"
          , "  - MUST: accept input"
          ])
      , ("b.yass.yaml", unlines
          [ "---"
          , "description: File B"
          , "version: v1"
          , "---"
          , "spec: spec-b"
          , "RETURN:"
          , "  - MUST: return something"
          ])
      ]
      ["a.yass.yaml", "b.yass.yaml"]
    exit `shouldBe` ExitSuccess
    out `shouldContain` "checked 2 files, found 0 errors"
    err `shouldBe` ""

  -- 14. Summary format: correct file count
  it "summary reports correct file count" $ do
    (exit, out, _err) <- withProjectAndFiles
      [ ("one.yass.yaml", unlines
          [ "---"
          , "description: test"
          , "version: v1"
          , "---"
          , "spec: s1"
          , "INPUT:"
          , "  - MUST: do stuff"
          ])
      , ("two.yass.yaml", unlines
          [ "---"
          , "description: test"
          , "version: v1"
          , "---"
          , "spec: s2"
          , "INPUT:"
          , "  - MUST: do stuff"
          ])
      , ("three.yass.yaml", unlines
          [ "---"
          , "description: test"
          , "version: v1"
          , "---"
          , "spec: s3"
          , "INPUT:"
          , "  - MUST: do stuff"
          ])
      ]
      ["one.yass.yaml", "two.yass.yaml", "three.yass.yaml"]
    exit `shouldBe` ExitSuccess
    out `shouldContain` "checked 3 files, found 0 errors"

  -- 15. YAML error skips further checks (no preamble error after YAML fail)
  it "skips preamble/spec checks when YAML fails" $ do
    withProject $ \tmpDir -> do
      let fp = tmpDir </> "yamlerr.yass.yaml"
          bom = BS.pack [0xEF, 0xBB, 0xBF]
          -- Content that would also fail preamble check (no description)
          content = TE.encodeUtf8 "---\nversion: v1\n"
      BS.writeFile fp (BS.append bom content)
      (exit, out, err) <- captureOutput (runValidate ["yamlerr.yass.yaml"])
      exit `shouldBe` ExitFailure 1
      -- Should have exactly 1 error (YAML), not 2 (YAML + preamble)
      out `shouldContain` "found 1 errors"
      err `shouldContain` "yass.yaml.has_bom"
      -- Should NOT contain preamble errors
      err `shouldNotContain` "yass.preamble"

  -- 16. Bad extension → exit 2
  it "exits 2 for bad extension file" $ do
    withProject $ \tmpDir -> do
      let fp = tmpDir </> "test.yaml"
      writeYassFile fp "---\nfoo: bar\n"
      (exit, _out, err) <- captureOutput (runValidate ["test.yaml"])
      exit `shouldBe` ExitFailure 2
      err `shouldContain` "bad_extension"

  -- 17. Directory discovery
  it "discovers .yass.yaml files in a subdirectory" $ do
    withProject $ \tmpDir -> do
      let subDir = tmpDir </> "specs"
      createDirectoryIfMissing True subDir
      writeYassFile (subDir </> "found.yass.yaml") (unlines
        [ "---"
        , "description: discovered"
        , "version: v1"
        , "---"
        , "spec: discovered-spec"
        , "INPUT:"
        , "  - MUST: be found"
        ])
      (exit, out, _err) <- captureOutput (runValidate ["specs"])
      exit `shouldBe` ExitSuccess
      out `shouldContain` "checked 1 files, found 0 errors"

  -- 18. Missing description in preamble
  it "reports missing description error" $ do
    (exit, _out, err) <- withProjectAndFiles
      [ ("nodesc.yass.yaml", "---\nversion: v1\n---\nspec: foo\n")
      ]
      ["nodesc.yass.yaml"]
    exit `shouldBe` ExitFailure 1
    err `shouldContain` "yass.preamble.missing_description"

  -- 19. Spec with unknown key
  it "reports unknown key in spec document" $ do
    (exit, _out, err) <- withProjectAndFiles
      [ ("unknownkey.yass.yaml", unlines
          [ "---"
          , "description: test"
          , "version: v1"
          , "---"
          , "spec: myspec"
          , "BOGUS: something"
          ])
      ]
      ["unknownkey.yass.yaml"]
    exit `shouldBe` ExitFailure 1
    err `shouldContain` "yass.spec.unknown_key"

  -- 20. Slot value not a list
  it "reports error when slot value is not a list" $ do
    (exit, _out, err) <- withProjectAndFiles
      [ ("notlist.yass.yaml", unlines
          [ "---"
          , "description: test"
          , "version: v1"
          , "---"
          , "spec: myspec"
          , "INPUT: not-a-list"
          ])
      ]
      ["notlist.yass.yaml"]
    exit `shouldBe` ExitFailure 1
    err `shouldContain` "yass.slot.value_not_list"

  -- 21. Multiple errors in one file (preamble OK but multiple spec issues)
  it "accumulates multiple errors from different specs" $ do
    (exit, out, err) <- withProjectAndFiles
      [ ("multi.yass.yaml", unlines
          [ "---"
          , "description: test"
          , "version: v1"
          , "---"
          , "spec: good-spec"
          , "INPUT:"
          , "  - MUST: do stuff"
          , "---"
          , "spec: \"bad spec\""
          ])
      ]
      ["multi.yass.yaml"]
    exit `shouldBe` ExitFailure 1
    -- Should have spec name bad chars error
    err `shouldContain` "yass.spec.name_bad_chars"
    -- Error count should be > 0
    out `shouldNotContain` "found 0 errors"

  -- 22. Duplicate key in YAML
  it "reports duplicate key YAML error" $ do
    (exit, _out, err) <- withProjectAndFiles
      [ ("dupkey.yass.yaml", "---\nfoo: 1\nfoo: 2\n")
      ]
      ["dupkey.yass.yaml"]
    exit `shouldBe` ExitFailure 1
    err `shouldContain` "yass.yaml.duplicate_key"

  -- 23. Ref to nonexistent spec in same file
  it "reports ref spec not found in same file" $ do
    (exit, _out, err) <- withProjectAndFiles
      [ ("badref.yass.yaml", unlines
          [ "---"
          , "description: test"
          , "version: v1"
          , "---"
          , "spec: myspec"
          , "INPUT:"
          , "  - CONFORMS: nonexistent"
          , "    MUST: do stuff"
          ])
      ]
      ["badref.yass.yaml"]
    exit `shouldBe` ExitFailure 1
    err `shouldContain` "yass.ref.spec_not_found_same_file"

  -- 24. Valid cross-file ref
  it "validates cross-file references successfully" $ do
    (exit, out, err) <- withProjectAndFiles
      [ ("target.yass.yaml", unlines
          [ "---"
          , "description: target"
          , "version: v1"
          , "---"
          , "spec: target-spec"
          , "INPUT:"
          , "  - MUST: accept input"
          ])
      , ("source.yass.yaml", unlines
          [ "---"
          , "description: source"
          , "version: v1"
          , "---"
          , "spec: source-spec"
          , "INPUT:"
          , "  - CONFORMS: target@target-spec"
          , "    MUST: do stuff"
          ])
      ]
      ["source.yass.yaml", "target.yass.yaml"]
    exit `shouldBe` ExitSuccess
    out `shouldContain` "checked 2 files, found 0 errors"
    err `shouldBe` ""

  -- 25. Error lines go to stderr, summary to stdout
  it "sends errors to stderr and summary to stdout" $ do
    (exit, out, err) <- withProjectAndFiles
      [ ("errs.yass.yaml", "---\nspec: foo\n")
      ]
      ["errs.yass.yaml"]
    exit `shouldBe` ExitFailure 1
    -- Summary is on stdout
    out `shouldContain` "checked"
    out `shouldContain` "files"
    -- Error details are on stderr
    err `shouldNotBe` ""
    -- stdout should NOT contain error code brackets
    out `shouldNotContain` "yass.preamble"
