{-# LANGUAGE OverloadedStrings #-}
{-# LANGUAGE ScopedTypeVariables #-}

module Yass.CLI (run) where

import Control.Exception (SomeException, try)
import Data.Char (toLower)
import Data.List (isPrefixOf)
import qualified Data.Text as T
import qualified Data.Text.IO as TIO
import System.Environment (getArgs)
import qualified System.Exit
import System.Exit (ExitCode(..))
import System.IO (Handle, hFlush, hSetBuffering, stderr, stdout, BufferMode(..))
import System.Posix.Process (exitImmediately)
import System.Posix.Signals (installHandler, Handler(..), sigPIPE, sigINT, sigTERM)

import Yass.Types (ErrorCode, errorCodeText, ErrorCode(ArgvUnknownSubcommand, ArgvNoSubcommand, ArgvUnknownFlag, ArgvEmptyArgument, ArgvShortFlag, ArgvCaseMismatch, ArgvAbbreviation, ArgvStdinDash))
import Yass.Validate (runValidate)
import Yass.List (runList)
import Yass.Query (runQuery)

-- | The tool version
version :: String
version = "0.0.3"

-- | Known subcommands
knownSubcommands :: [String]
knownSubcommands = ["validate", "query", "list"]

-- | Known global flags
knownFlags :: [String]
knownFlags = ["--help", "--version"]

-- | Main CLI entry point
run :: IO ExitCode
run = do
  -- Set up line buffering on stdout
  hSetBuffering stdout LineBuffering
  -- Handle SIGPIPE
  _ <- installHandler sigPIPE (Catch (hFlush stderr >> exitImmediately ExitSuccess)) Nothing
  -- Handle SIGINT
  _ <- installHandler sigINT (Catch (hFlush stderr >> exitImmediately (ExitFailure 130))) Nothing
  -- Handle SIGTERM
  _ <- installHandler sigTERM (Catch (hFlush stderr >> exitImmediately (ExitFailure 143))) Nothing

  args <- getArgs
  dispatch args

-- | Dispatch based on arguments
dispatch :: [String] -> IO ExitCode
dispatch [] = do
  emitArgvError ArgvNoSubcommand "no subcommand given"
  printUsage stderr
  return (ExitFailure 2)

dispatch args
  -- Check for --help anywhere
  | "--help" `elem` args = do
      printUsage stdout
      return ExitSuccess
  -- Check for --version anywhere
  | "--version" `elem` args = do
      putStrLn $ "yass " ++ version
      hFlush stdout
      return ExitSuccess
  | otherwise = processArgs args

-- | Process the argument list
processArgs :: [String] -> IO ExitCode
processArgs [] = do
  emitArgvError ArgvNoSubcommand "no subcommand given"
  printUsage stderr
  return (ExitFailure 2)

processArgs ("--":rest) =
  -- End-of-options: remaining are positionals
  case rest of
    [] -> do
      emitArgvError ArgvNoSubcommand "no subcommand given"
      printUsage stderr
      return (ExitFailure 2)
    (cmd:cmdArgs) -> dispatchSubcommand cmd cmdArgs

processArgs (arg:rest)
  -- Empty argument
  | null arg = do
      emitArgvError ArgvEmptyArgument "empty argument"
      printUsage stderr
      return (ExitFailure 2)
  -- Bare "-" (stdin dash)
  | arg == "-" = do
      emitArgvError ArgvStdinDash "stdin marker `-` is not supported; pass a file path"
      printUsage stderr
      return (ExitFailure 2)
  -- Short flag (starts with - but not --)
  | "-" `isPrefixOf` arg && not ("--" `isPrefixOf` arg) = do
      emitArgvError ArgvShortFlag
        ("short-form flags are not supported in v1: " <> T.pack arg)
      printUsage stderr
      return (ExitFailure 2)
  -- Check for case mismatch with known flags
  | "--" `isPrefixOf` arg
  , any (\f -> map toLower arg == map toLower f && arg /= f) knownFlags = do
      emitArgvError ArgvCaseMismatch
        ("subcommand or flag case mismatch: " <> T.pack arg)
      printUsage stderr
      return (ExitFailure 2)
  -- Check for abbreviation of known flags
  | "--" `isPrefixOf` arg
  , length arg > 2
  , any (\f -> arg `isPrefixOf` f && arg /= f) knownFlags = do
      emitArgvError ArgvAbbreviation
        ("abbreviations are not supported: " <> T.pack arg)
      printUsage stderr
      return (ExitFailure 2)
  -- Unknown flag (starts with -- but not --help/--version)
  | "--" `isPrefixOf` arg && arg `notElem` knownFlags = do
      emitArgvError ArgvUnknownFlag ("unknown flag: " <> T.pack arg)
      printUsage stderr
      return (ExitFailure 2)
  -- Check for case mismatch with known subcommands
  | any (\cmd -> map toLower arg == cmd && arg /= cmd) knownSubcommands = do
      emitArgvError ArgvCaseMismatch
        ("subcommand or flag case mismatch: " <> T.pack arg)
      printUsage stderr
      return (ExitFailure 2)
  -- Check for abbreviation of known subcommands
  | any (\cmd -> arg `isPrefixOf` cmd && arg /= cmd && length arg > 0) knownSubcommands = do
      emitArgvError ArgvAbbreviation
        ("abbreviations are not supported: " <> T.pack arg)
      printUsage stderr
      return (ExitFailure 2)
  -- It's a subcommand
  | otherwise = dispatchSubcommand arg rest

-- | Dispatch to the appropriate subcommand handler
dispatchSubcommand :: String -> [String] -> IO ExitCode
dispatchSubcommand cmd args
  | cmd == "validate" = checkSubcmdFlags args >> runWithCatch (runValidate (stripEndOfOptions args))
  | cmd == "query"    = checkSubcmdFlags args >> runWithCatch (runQuery (stripEndOfOptions args))
  | cmd == "list"     = checkSubcmdFlags args >> runWithCatch (runList (stripEndOfOptions args))
  | otherwise = do
      emitArgvError ArgvUnknownSubcommand
        ("unknown subcommand: " <> T.pack cmd)
      printUsage stderr
      return (ExitFailure 2)

-- | Run a subcommand action with uncaught exception handling
runWithCatch :: IO ExitCode -> IO ExitCode
runWithCatch action = do
  result <- try action :: IO (Either SomeException ExitCode)
  case result of
    Left ex -> do
      TIO.hPutStrLn stderr ("yass: [yass.internal.uncaught] internal error: " <> T.pack (show ex))
      hFlush stderr
      return (ExitFailure 1)
    Right code -> return code

-- | Check for disallowed flags in subcommand args
checkSubcmdFlags :: [String] -> IO ()
checkSubcmdFlags allArgs = do
  -- Only check args before the first "--" (end-of-options marker)
  let args = takeWhile (/= "--") allArgs
      flags = filter (\a -> "--" `isPrefixOf` a) args
      shortFlags = filter (\a -> "-" `isPrefixOf` a && not ("--" `isPrefixOf` a) && a /= "-") args
      emptyArgs = filter null args
      stdinDash = filter (== "-") args
  case shortFlags of
    (f:_) -> do
      emitArgvError ArgvShortFlag
        ("short-form flags are not supported in v1: " <> T.pack f)
      printUsage stderr
      exitIO (ExitFailure 2)
    _ -> return ()
  case flags of
    [] -> return ()
    _ -> do
      let badFlags = filter (`notElem` ["--help", "--version"]) flags
      case badFlags of
        (f:_) -> do
          emitArgvError ArgvUnknownFlag ("unknown flag: " <> T.pack f)
          printUsage stderr
          exitIO (ExitFailure 2)
        _ -> return ()
  case emptyArgs of
    (_:_) -> do
      emitArgvError ArgvEmptyArgument "empty argument"
      printUsage stderr
      exitIO (ExitFailure 2)
    _ -> return ()
  case stdinDash of
    (_:_) -> do
      emitArgvError ArgvStdinDash "stdin marker `-` is not supported; pass a file path"
      printUsage stderr
      exitIO (ExitFailure 2)
    _ -> return ()

-- | Strip the "--" end-of-options marker from args, keeping tokens before and after it.
stripEndOfOptions :: [String] -> [String]
stripEndOfOptions = filter (/= "--")

-- | Emit an error line to stderr in the format: yass: [<code>] <message>
emitArgvError :: ErrorCode -> T.Text -> IO ()
emitArgvError code msg = do
  let line = "yass: [" <> errorCodeText code <> "] " <> msg
  TIO.hPutStrLn stderr line
  hFlush stderr

-- | Print usage information
printUsage :: System.IO.Handle -> IO ()
printUsage h = do
  let usage = T.unlines
        [ "Usage: yass <command> [options] [arguments]"
        , ""
        , "Commands:"
        , "  validate    Validate .yass.yaml files"
        , "  list        List specs in .yass.yaml files"
        , "  query       Query a spec by name"
        , ""
        , "Flags:"
        , "  --help      Show this help"
        , "  --version   Show version"
        ]
  TIO.hPutStr h usage
  hFlush h

-- | Exit immediately with an ExitCode (used for flag validation)
exitIO :: ExitCode -> IO ()
exitIO code = do
  hFlush stdout
  hFlush stderr
  System.Exit.exitWith code
