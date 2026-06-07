{-# LANGUAGE OverloadedStrings #-}

module Yass.Query (runQuery) where

import Data.Char (isSpace)
import qualified Data.Text as T
import qualified Data.Text.IO as TIO
import System.Directory (getCurrentDirectory, doesFileExist, doesDirectoryExist)
import System.Exit (ExitCode(..))
import System.FilePath (takeDirectory)
import System.IO (hFlush, stderr, stdout)

import Yass.Types hiding (ExitSuccess)
import Yass.ErrorLine (formatErrorLineNoLine, relativizePath)
import Yass.Parser (RawDocument(..), RawValue(..), ParseResult(..), parseYassFile)
import Yass.ProjectRoot (findProjectRoot)
import Yass.Discover (discoverSpecFiles)
import Yass.Query.NameLookup (lookupName, MatchResult(..))
import Yass.Query.ExtractFragment (extractFragment)
import Yass.Query.InlineConforms (inlineConforms, InlineError(..))
import Yass.Query.OutputProfile (emitFragment)

-- | Run the query subcommand
runQuery :: [String] -> IO ExitCode
runQuery [] = do
  emitErr "yass" QueryNameMissing "missing spec name"
  return (ExitFailure 2)
runQuery args = do
  cwd <- getCurrentDirectory

  let (specNameStr:scopeArgs) = args
      specName = T.pack specNameStr

  -- Check for blank name (literally empty string only)
  -- Whitespace-only names are treated as no-match per spec, not as blank
  if T.null specName
    then do
      emitErr "yass" QueryNameBlank "spec name is blank or contains whitespace"
      return (ExitFailure 2)
    else
      -- Check for colon in scope path
      case scopeArgs of
        (p:_) | ':' `elem` p -> do
          emitErr "yass" PathColonInPath
            ("path contains an unsupported colon character: " <> T.pack p)
          return (ExitFailure 2)
        _ -> do
          -- Find project root
          rootResult <- findProjectRoot cwd
          case rootResult of
            Left _ -> do
              emitErr cwd FindrootNoMarker
                "no project root marker found"
              return (ExitFailure 2)
            Right projectRoot -> do
              let mScope = case scopeArgs of
                    [] -> Nothing
                    (s:_) -> Just s
              -- Validate scope exists before doing name lookup
              case mScope of
                Just scope -> do
                  fileExists <- doesFileExist scope
                  dirExists <- doesDirectoryExist scope
                  if not fileExists && not dirExists
                    then do
                      emitErr "yass" QueryScopeNotFound
                        ("scope path does not exist: " <> T.pack scope)
                      return (ExitFailure 2)
                    else queryWithScope cwd projectRoot specName mScope
                Nothing -> queryWithScope cwd projectRoot specName mScope

-- | Execute query with resolved scope
queryWithScope :: FilePath -> FilePath -> T.Text -> Maybe String -> IO ExitCode
queryWithScope cwd projectRoot specName mScope = do
  -- Discover files in scope
  filesResult <- discoverSpecFiles cwd projectRoot mScope
  case filesResult of
    Left errs -> do
      mapM_ (emitYassErr cwd) errs
      return (ExitFailure 2)
    Right files
      | null files -> case mScope of
          Just scope -> do
            emitErr "yass" QueryScopeEmpty
              ("no .yass.yaml files found in scope: " <> T.pack scope)
            return (ExitFailure 2)
          Nothing -> do
            emitErr "yass" QueryNoMatch ("no spec matches: " <> specName)
            return (ExitFailure 1)
      | otherwise -> do
          -- Parse all files and collect (file, docs) pairs
          filesDocs <- parseAllFiles files
          -- Look up the spec
          case lookupName specName filesDocs of
            NoMatch -> do
              emitErr "yass" QueryNoMatch ("no spec matches: " <> specName)
              return (ExitFailure 1)
            SingleMatch fp matchedName docs -> do
              -- Extract and emit the spec fragment with CONFORMS inlining
              emitSpecFragment cwd projectRoot fp matchedName docs
            MultiMatch matches -> do
              -- Emit disambiguation rows (list format, no truncation)
              emitDisambiguation cwd matches
              return ExitSuccess

-- | Parse all discovered files
parseAllFiles :: [FilePath] -> IO [(FilePath, [RawDocument])]
parseAllFiles = mapM parseOne
  where
    parseOne fp = do
      result <- parseYassFile fp
      case result of
        ParseOK docs -> return (fp, docs)
        ParseError _ _ -> return (fp, [])

-- | Emit a spec fragment with CONFORMS inlining
emitSpecFragment :: FilePath -> FilePath -> FilePath -> T.Text -> [RawDocument] -> IO ExitCode
emitSpecFragment _cwd projectRoot fp specName docs = do
  case extractFragment specName docs of
    Nothing -> do
      emitErr "yass" QueryNoMatch ("no spec matches: " <> specName)
      return (ExitFailure 1)
    Just specDoc -> do
      let fileDir = takeDirectory fp
      inlineResult <- inlineConforms projectRoot fileDir docs specDoc
      case inlineResult of
        Left errs -> do
          mapM_ (\e -> emitErr "yass" (ieCode e) (ieMessage e)) errs
          return (ExitFailure 1)
        Right inlinedDoc -> do
          let fragment = emitFragment inlinedDoc
          TIO.putStr fragment
          hFlush stdout
          return ExitSuccess

-- | Emit disambiguation rows in list format (no truncation)
emitDisambiguation :: FilePath -> [(FilePath, T.Text)] -> IO ()
emitDisambiguation cwd matches = do
  -- Parse each file for preamble description
  mapM_ (emitDisambigRow cwd) matches

emitDisambigRow :: FilePath -> (FilePath, T.Text) -> IO ()
emitDisambigRow cwd (fp, specName) = do
  result <- parseYassFile fp
  let desc = case result of
        ParseOK (preamble:_) -> extractDesc preamble
        _ -> ""
      relPath = relativizePath cwd fp
  TIO.putStrLn $ T.pack relPath <> "\t" <> specName <> "\t" <> desc

-- | Extract description from a preamble document
extractDesc :: RawDocument -> T.Text
extractDesc doc = case lookup3 "description" (rawDocEntries doc) of
  Just (RVString d, _) -> normalizeDesc d
  _ -> ""

-- | Normalize description whitespace
normalizeDesc :: T.Text -> T.Text
normalizeDesc = T.strip . collapseWS
  where
    collapseWS t
      | T.null t = t
      | otherwise = T.intercalate " " (filter (not . T.null) $ T.split isSpace t)

-- | Emit an error to stderr using formatErrorLineNoLine for sanitization
emitErr :: FilePath -> ErrorCode -> T.Text -> IO ()
emitErr fp code msg = do
  cwd <- getCurrentDirectory
  TIO.hPutStrLn stderr $ formatErrorLineNoLine cwd fp code msg
  hFlush stderr

-- | Emit a YassError to stderr
emitYassErr :: FilePath -> YassError -> IO ()
emitYassErr cwd err = do
  let fp = maybe "yass" id (yeFile err)
  TIO.hPutStrLn stderr $ formatErrorLineNoLine cwd fp (yeCode err) (yeMessage err)
  hFlush stderr

-- | Lookup helper
lookup3 :: T.Text -> [(T.Text, RawValue, Int)] -> Maybe (RawValue, Int)
lookup3 _ [] = Nothing
lookup3 k ((k', v, ln):rest)
  | k == k'   = Just (v, ln)
  | otherwise  = lookup3 k rest
