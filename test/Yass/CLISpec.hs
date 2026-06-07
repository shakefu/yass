{-# LANGUAGE OverloadedStrings #-}

module Yass.CLISpec (spec) where

import Test.Hspec
import System.Exit (ExitCode(..))
import System.IO (hSetBuffering, BufferMode(..), stdout, stderr, hFlush,
                  hGetContents, hClose)
import System.IO.Temp (withSystemTempDirectory)
import System.Directory (createDirectoryIfMissing)
import System.FilePath ((</>))
import System.Process (readProcessWithExitCode, createProcess, proc,
                       std_out, std_err, std_in, StdStream(..),
                       waitForProcess, cwd)

-- | Get the executable path. We use a known path from cabal list-bin.
getExePath :: IO FilePath
getExePath = do
  (_, out, _) <- readProcessWithExitCode "cabal" ["list-bin", "yass"] ""
  let path = strip out
  return path
  where
    strip = reverse . dropWhile (== '\n') . reverse

-- | Helper: run the yass exe with given args (in current directory)
runYass :: FilePath -> [String] -> IO (ExitCode, String, String)
runYass exe args = readProcessWithExitCode exe args ""

-- | Helper: run the yass exe with given args in a specific directory
runYassIn :: FilePath -> FilePath -> [String] -> IO (ExitCode, String, String)
runYassIn exe dir args = do
  let cp = (proc exe args)
        { cwd = Just dir
        , std_in = CreatePipe
        , std_out = CreatePipe
        , std_err = CreatePipe
        }
  (_, Just hOut, Just hErr, ph) <- createProcess cp
  outStr <- hGetContents hOut
  errStr <- hGetContents hErr
  -- Force evaluation before waiting
  _ <- return $! length outStr
  _ <- return $! length errStr
  code <- waitForProcess ph
  hClose hOut
  hClose hErr
  return (code, outStr, errStr)

-- | Helper: write a minimal valid .yass.yaml file with a spec
writeValidYassFile :: FilePath -> IO ()
writeValidYassFile path =
  writeFile path $ unlines
    [ "description: Test file"
    , "version: v1"
    , "---"
    , "spec: TestSpec"
    , "INPUT:"
    , "- MUST: be valid"
    ]

-- | Helper: write a preamble-only .yass.yaml file
writePreambleOnly :: FilePath -> IO ()
writePreambleOnly path =
  writeFile path $ unlines
    [ "description: Preamble only file"
    , "version: v1"
    ]

spec :: Spec
spec = do
  -- Get exe path once before all tests
  exe <- runIO getExePath

  describe "Yass.CLI" $ do

    -- ----------------------------------------------------------------
    -- --help flag
    -- ----------------------------------------------------------------
    describe "--help" $ do
      it "prints usage and exits 0" $ do
        (code, out, _err) <- runYass exe ["--help"]
        code `shouldBe` ExitSuccess
        out `shouldContain` "Usage: yass"
        out `shouldContain` "Commands:"
        out `shouldContain` "validate"
        out `shouldContain` "list"
        out `shouldContain` "query"

      it "prints flags section in usage" $ do
        (code, out, _err) <- runYass exe ["--help"]
        code `shouldBe` ExitSuccess
        out `shouldContain` "Flags:"
        out `shouldContain` "--help"
        out `shouldContain` "--version"

      it "takes precedence when mixed with other args" $ do
        (code, out, _err) <- runYass exe ["--help", "validate"]
        code `shouldBe` ExitSuccess
        out `shouldContain` "Usage: yass"

      it "takes precedence when placed after subcommand" $ do
        (code, out, _err) <- runYass exe ["validate", "--help"]
        -- --help is checked before dispatch at the top level
        code `shouldBe` ExitSuccess
        out `shouldContain` "Usage: yass"

    -- ----------------------------------------------------------------
    -- --version flag
    -- ----------------------------------------------------------------
    describe "--version" $ do
      it "prints version string and exits 0" $ do
        (code, out, _err) <- runYass exe ["--version"]
        code `shouldBe` ExitSuccess
        out `shouldContain` "yass 0.0.3"

      it "takes precedence when mixed with other args" $ do
        (code, out, _err) <- runYass exe ["--version", "validate"]
        code `shouldBe` ExitSuccess
        out `shouldContain` "yass 0.0.3"

    -- ----------------------------------------------------------------
    -- No args
    -- ----------------------------------------------------------------
    describe "no arguments" $ do
      it "exits 2 with yass.argv.no_subcommand error" $ do
        (code, _out, err) <- runYass exe []
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.no_subcommand"
        err `shouldContain` "no subcommand given"

      it "prints usage to stderr when no args" $ do
        (_, _out, err) <- runYass exe []
        err `shouldContain` "Usage: yass"

    -- ----------------------------------------------------------------
    -- Unknown subcommand
    -- ----------------------------------------------------------------
    describe "unknown subcommand" $ do
      it "exits 2 with yass.argv.unknown_subcommand error" $ do
        (code, _out, err) <- runYass exe ["frobnicate"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.unknown_subcommand"
        err `shouldContain` "unknown subcommand: frobnicate"

      it "prints usage to stderr on unknown subcommand" $ do
        (_, _out, err) <- runYass exe ["frobnicate"]
        err `shouldContain` "Usage: yass"

      it "rejects numeric subcommand" $ do
        (code, _out, err) <- runYass exe ["123"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.unknown_subcommand"

    -- ----------------------------------------------------------------
    -- Short flag
    -- ----------------------------------------------------------------
    describe "short flag" $ do
      it "exits 2 with yass.argv.short_flag for -v" $ do
        (code, _out, err) <- runYass exe ["-v"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.short_flag"
        err `shouldContain` "short-form flags are not supported"

      it "exits 2 with yass.argv.short_flag for -h" $ do
        (code, _out, err) <- runYass exe ["-h"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.short_flag"

      it "exits 2 with yass.argv.short_flag for -abc" $ do
        (code, _out, err) <- runYass exe ["-abc"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.short_flag"

    -- ----------------------------------------------------------------
    -- Unknown flag
    -- ----------------------------------------------------------------
    describe "unknown flag" $ do
      it "exits 2 with yass.argv.unknown_flag for --foo" $ do
        (code, _out, err) <- runYass exe ["--foo"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.unknown_flag"
        err `shouldContain` "unknown flag: --foo"

      it "exits 2 with yass.argv.unknown_flag for --verbose" $ do
        (code, _out, err) <- runYass exe ["--verbose"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.unknown_flag"

    -- ----------------------------------------------------------------
    -- Case mismatch
    -- ----------------------------------------------------------------
    describe "case mismatch" $ do
      it "exits 2 with yass.argv.case_mismatch for Validate" $ do
        (code, _out, err) <- runYass exe ["Validate"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.case_mismatch"
        err `shouldContain` "case mismatch"

      it "exits 2 with yass.argv.case_mismatch for VALIDATE" $ do
        (code, _out, err) <- runYass exe ["VALIDATE"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.case_mismatch"

      it "exits 2 with yass.argv.case_mismatch for List" $ do
        (code, _out, err) <- runYass exe ["List"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.case_mismatch"

      it "exits 2 with yass.argv.case_mismatch for Query" $ do
        (code, _out, err) <- runYass exe ["Query"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.case_mismatch"

    -- ----------------------------------------------------------------
    -- Abbreviation
    -- ----------------------------------------------------------------
    describe "abbreviation" $ do
      it "exits 2 with yass.argv.abbreviation for val" $ do
        (code, _out, err) <- runYass exe ["val"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.abbreviation"
        err `shouldContain` "abbreviations are not supported"

      it "exits 2 with yass.argv.abbreviation for lis" $ do
        (code, _out, err) <- runYass exe ["lis"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.abbreviation"

      it "exits 2 with yass.argv.abbreviation for qu" $ do
        (code, _out, err) <- runYass exe ["qu"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.abbreviation"

      it "exits 2 with yass.argv.abbreviation for q" $ do
        (code, _out, err) <- runYass exe ["q"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.abbreviation"

    -- ----------------------------------------------------------------
    -- Empty argument
    -- ----------------------------------------------------------------
    describe "empty argument" $ do
      it "exits 2 with yass.argv.empty_argument for empty string" $ do
        (code, _out, err) <- runYass exe [""]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.empty_argument"

    -- ----------------------------------------------------------------
    -- Bare dash (stdin)
    -- ----------------------------------------------------------------
    describe "bare dash" $ do
      it "exits 2 with yass.argv.stdin_dash for bare -" $ do
        (code, _out, err) <- runYass exe ["-"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.stdin_dash"
        err `shouldContain` "stdin marker"

    -- ----------------------------------------------------------------
    -- End-of-options "--"
    -- ----------------------------------------------------------------
    describe "end-of-options --" $ do
      it "treats -- alone as no subcommand, exits 2" $ do
        (code, _out, err) <- runYass exe ["--"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.no_subcommand"

      it "treats args after -- as positional subcommand" $ do
        withSystemTempDirectory "yass-cli-eo" $ \tmpDir -> do
          createDirectoryIfMissing False (tmpDir </> ".git")
          writeValidYassFile (tmpDir </> "test.yass.yaml")
          (code, _out, _err) <- runYassIn exe tmpDir ["--", "validate"]
          code `shouldBe` ExitSuccess

      it "treats unknown arg after -- as unknown subcommand" $ do
        (code, _out, err) <- runYass exe ["--", "frobnicate"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.unknown_subcommand"

    -- ----------------------------------------------------------------
    -- Subcommand: validate
    -- ----------------------------------------------------------------
    describe "validate subcommand" $ do
      it "validates a valid .yass.yaml file and exits 0" $ do
        withSystemTempDirectory "yass-cli-validate" $ \tmpDir -> do
          createDirectoryIfMissing False (tmpDir </> ".git")
          writeValidYassFile (tmpDir </> "test.yass.yaml")
          (code, _out, _err) <- runYassIn exe tmpDir ["validate"]
          code `shouldBe` ExitSuccess

      it "validates a specific file path" $ do
        withSystemTempDirectory "yass-cli-validate-path" $ \tmpDir -> do
          createDirectoryIfMissing False (tmpDir </> ".git")
          let filePath = tmpDir </> "test.yass.yaml"
          writeValidYassFile filePath
          (code, _out, _err) <- runYassIn exe tmpDir ["validate", filePath]
          code `shouldBe` ExitSuccess

      it "fails on nonexistent file path" $ do
        withSystemTempDirectory "yass-cli-validate-nofile" $ \tmpDir -> do
          createDirectoryIfMissing False (tmpDir </> ".git")
          -- Need a .yass.yaml so project root is found, but reference a missing file
          writeValidYassFile (tmpDir </> "test.yass.yaml")
          (code, _out, err) <- runYassIn exe tmpDir
            ["validate", tmpDir </> "nonexistent.yass.yaml"]
          code `shouldNotBe` ExitSuccess
          err `shouldNotBe` ""

    -- ----------------------------------------------------------------
    -- Subcommand: list
    -- ----------------------------------------------------------------
    describe "list subcommand" $ do
      it "lists specs from a valid .yass.yaml file" $ do
        withSystemTempDirectory "yass-cli-list" $ \tmpDir -> do
          createDirectoryIfMissing False (tmpDir </> ".git")
          writeValidYassFile (tmpDir </> "test.yass.yaml")
          (code, out, _err) <- runYassIn exe tmpDir ["list"]
          code `shouldBe` ExitSuccess
          out `shouldContain` "TestSpec"

      it "lists specs from a specific file" $ do
        withSystemTempDirectory "yass-cli-list-path" $ \tmpDir -> do
          createDirectoryIfMissing False (tmpDir </> ".git")
          let filePath = tmpDir </> "test.yass.yaml"
          writeValidYassFile filePath
          (code, out, _err) <- runYassIn exe tmpDir ["list", filePath]
          code `shouldBe` ExitSuccess
          out `shouldContain` "TestSpec"

      it "lists preamble-only file with no specs" $ do
        withSystemTempDirectory "yass-cli-list-preamble" $ \tmpDir -> do
          createDirectoryIfMissing False (tmpDir </> ".git")
          writePreambleOnly (tmpDir </> "test.yass.yaml")
          (code, _out, _err) <- runYassIn exe tmpDir ["list"]
          code `shouldBe` ExitSuccess

    -- ----------------------------------------------------------------
    -- Subcommand: query
    -- ----------------------------------------------------------------
    describe "query subcommand" $ do
      it "exits 2 with missing spec name" $ do
        withSystemTempDirectory "yass-cli-query-noname" $ \tmpDir -> do
          createDirectoryIfMissing False (tmpDir </> ".git")
          writeValidYassFile (tmpDir </> "test.yass.yaml")
          (code, _out, err) <- runYassIn exe tmpDir ["query"]
          code `shouldBe` ExitFailure 2
          err `shouldContain` "yass.query.name_missing"

      it "queries a spec by name" $ do
        withSystemTempDirectory "yass-cli-query" $ \tmpDir -> do
          createDirectoryIfMissing False (tmpDir </> ".git")
          writeValidYassFile (tmpDir </> "test.yass.yaml")
          (code, out, _err) <- runYassIn exe tmpDir ["query", "TestSpec"]
          code `shouldBe` ExitSuccess
          out `shouldContain` "TestSpec"

      it "exits nonzero for nonexistent spec name" $ do
        withSystemTempDirectory "yass-cli-query-miss" $ \tmpDir -> do
          createDirectoryIfMissing False (tmpDir </> ".git")
          writeValidYassFile (tmpDir </> "test.yass.yaml")
          (code, _out, err) <- runYassIn exe tmpDir ["query", "NoSuchSpec"]
          code `shouldNotBe` ExitSuccess
          err `shouldContain` "yass.query.no_match"

    -- ----------------------------------------------------------------
    -- Subcommand flag validation
    -- ----------------------------------------------------------------
    describe "subcommand flag validation" $ do
      it "rejects short flag after validate subcommand" $ do
        withSystemTempDirectory "yass-cli-subcmd-short" $ \tmpDir -> do
          createDirectoryIfMissing False (tmpDir </> ".git")
          writeValidYassFile (tmpDir </> "test.yass.yaml")
          (code, _out, err) <- runYassIn exe tmpDir ["validate", "-x"]
          code `shouldBe` ExitFailure 2
          err `shouldContain` "yass.argv.short_flag"

      it "rejects unknown flag after list subcommand" $ do
        withSystemTempDirectory "yass-cli-subcmd-flag" $ \tmpDir -> do
          createDirectoryIfMissing False (tmpDir </> ".git")
          writeValidYassFile (tmpDir </> "test.yass.yaml")
          (code, _out, err) <- runYassIn exe tmpDir ["list", "--unknown"]
          code `shouldBe` ExitFailure 2
          err `shouldContain` "yass.argv.unknown_flag"

      it "rejects bare dash after list subcommand" $ do
        withSystemTempDirectory "yass-cli-subcmd-dash" $ \tmpDir -> do
          createDirectoryIfMissing False (tmpDir </> ".git")
          writeValidYassFile (tmpDir </> "test.yass.yaml")
          (code, _out, err) <- runYassIn exe tmpDir ["list", "-"]
          code `shouldBe` ExitFailure 2
          err `shouldContain` "yass.argv.stdin_dash"

      it "rejects empty arg after list subcommand" $ do
        withSystemTempDirectory "yass-cli-subcmd-empty" $ \tmpDir -> do
          createDirectoryIfMissing False (tmpDir </> ".git")
          writeValidYassFile (tmpDir </> "test.yass.yaml")
          (code, _out, err) <- runYassIn exe tmpDir ["list", ""]
          code `shouldBe` ExitFailure 2
          err `shouldContain` "yass.argv.empty_argument"

    -- ----------------------------------------------------------------
    -- Edge cases and precedence
    -- ----------------------------------------------------------------
    describe "edge cases" $ do
      it "handles multiple unknown flags - first is reported" $ do
        (code, _out, err) <- runYass exe ["--foo", "--bar"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.unknown_flag"
        err `shouldContain` "--foo"

      it "--help takes precedence over unknown flags" $ do
        (code, out, _err) <- runYass exe ["--foo", "--help"]
        code `shouldBe` ExitSuccess
        out `shouldContain` "Usage: yass"

      it "--version takes precedence over unknown flags" $ do
        (code, out, _err) <- runYass exe ["--foo", "--version"]
        code `shouldBe` ExitSuccess
        out `shouldContain` "yass 0.0.3"

      it "--help takes precedence over --version when both present" $ do
        (code, out, _err) <- runYass exe ["--help", "--version"]
        code `shouldBe` ExitSuccess
        out `shouldContain` "Usage: yass"

      it "single character l is an abbreviation, not unknown" $ do
        (code, _out, err) <- runYass exe ["l"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.abbreviation"

      it "v is an abbreviation of validate, not unknown" $ do
        (code, _out, err) <- runYass exe ["v"]
        code `shouldBe` ExitFailure 2
        err `shouldContain` "yass.argv.abbreviation"
