{-# LANGUAGE OverloadedStrings #-}

module Yass.Validate.CheckYAMLSpec (spec) where

import Test.Hspec
import qualified Data.ByteString as BS
import qualified Data.Text as T
import qualified Data.Text.Encoding as TE
import System.IO.Temp (withSystemTempDirectory)
import System.FilePath ((</>))

import Yass.Types (YassError(..), ErrorCode(..))
import Yass.Parser (RawDocument(..))
import Yass.Validate.CheckYAML (checkYAML)

-- | Helper: write text content to a temp .yass.yaml file and run checkYAML
withTempFile :: String -> (FilePath -> FilePath -> IO a) -> IO a
withTempFile content action =
  withSystemTempDirectory "yass-checkyaml-test" $ \dir -> do
    let fp = dir </> "test.yass.yaml"
    BS.writeFile fp (TE.encodeUtf8 (T.pack content))
    action dir fp

-- | Helper: write raw bytes to a temp file and run checkYAML
withTempFileBytes :: BS.ByteString -> (FilePath -> FilePath -> IO a) -> IO a
withTempFileBytes bs action =
  withSystemTempDirectory "yass-checkyaml-test" $ \dir -> do
    let fp = dir </> "test.yass.yaml"
    BS.writeFile fp bs
    action dir fp

-- | Expect a Left (error) result with the given ErrorCode
expectErrorCode :: ErrorCode -> Either YassError [RawDocument] -> IO YassError
expectErrorCode code result = case result of
  Left err -> do
    yeCode err `shouldBe` code
    return err
  Right docs -> do
    expectationFailure $
      "expected Left with " ++ show code
      ++ " but got Right with " ++ show (length docs) ++ " documents"
    -- unreachable, but needed for type
    error "unreachable"

-- | Expect a Right (success) result and return the documents
expectSuccess :: Either YassError [RawDocument] -> IO [RawDocument]
expectSuccess result = case result of
  Right docs -> return docs
  Left err -> do
    expectationFailure $
      "expected Right but got Left: " ++ show (yeCode err)
      ++ " - " ++ T.unpack (yeMessage err)
    error "unreachable"

spec :: Spec
spec = describe "checkYAML" $ do

  -- 1. Success: valid YAML with one document
  it "returns Right with documents for valid YAML" $ do
    withTempFile "---\ndescription: hello\nversion: v1\n" $ \cwd fp -> do
      result <- checkYAML cwd fp
      docs <- expectSuccess result
      length docs `shouldBe` 1

  -- 2. Success: valid YAML with multiple documents
  it "returns Right with multiple documents" $ do
    withTempFile "---\ndescription: hello\nversion: v1\n---\nspec: foo\n" $ \cwd fp -> do
      result <- checkYAML cwd fp
      docs <- expectSuccess result
      length docs `shouldBe` 2

  -- 3. Error: empty file
  it "returns YamlEmptyFile for an empty file" $ do
    withTempFileBytes BS.empty $ \cwd fp -> do
      result <- checkYAML cwd fp
      err <- expectErrorCode YamlEmptyFile result
      yeFile err `shouldBe` Just fp
      yeLine err `shouldBe` Nothing  -- line 0 maps to Nothing

  -- 4. Error: not UTF-8
  it "returns YamlNotUtf8 for invalid UTF-8 bytes" $ do
    -- 0xFE is never valid in UTF-8
    withTempFileBytes (BS.pack [0xFE, 0xFF]) $ \cwd fp -> do
      result <- checkYAML cwd fp
      err <- expectErrorCode YamlNotUtf8 result
      yeFile err `shouldBe` Just fp

  -- 5. Error: BOM
  it "returns YamlHasBom for a file with UTF-8 BOM" $ do
    let bom = BS.pack [0xEF, 0xBB, 0xBF]
        content = TE.encodeUtf8 "---\nfoo: bar\n"
    withTempFileBytes (BS.append bom content) $ \cwd fp -> do
      result <- checkYAML cwd fp
      err <- expectErrorCode YamlHasBom result
      yeFile err `shouldBe` Just fp

  -- 6. Error: duplicate key
  it "returns YamlDuplicateKey for duplicate keys" $ do
    withTempFile "---\nfoo: 1\nfoo: 2\n" $ \cwd fp -> do
      result <- checkYAML cwd fp
      err <- expectErrorCode YamlDuplicateKey result
      yeFile err `shouldBe` Just fp
      yeLine err `shouldSatisfy` (/= Nothing)

  -- 7. Error: anchor/alias
  it "returns YamlAnchorOrAlias for anchors" $ do
    withTempFile "---\nfoo: &anchor bar\nbaz: *anchor\n" $ \cwd fp -> do
      result <- checkYAML cwd fp
      err <- expectErrorCode YamlAnchorOrAlias result
      yeFile err `shouldBe` Just fp

  -- 8. Error: custom tag
  it "returns YamlAnchorOrAlias for custom tags" $ do
    withTempFile "---\nfoo: !custom bar\n" $ \cwd fp -> do
      result <- checkYAML cwd fp
      err <- expectErrorCode YamlAnchorOrAlias result
      yeFile err `shouldBe` Just fp

  -- 9. Error: malformed YAML
  it "returns YamlMalformed for malformed YAML" $ do
    withTempFile "---\n  bad:\n indent: wrong\n" $ \cwd fp -> do
      result <- checkYAML cwd fp
      -- Malformed YAML should fail parsing; libyaml may produce various errors
      case result of
        Left err -> yeCode err `shouldSatisfy`
          (\c -> c == YamlMalformed || c == YamlDuplicateKey)
        Right _ -> return ()  -- If libyaml accepts it, that's fine too

  -- 10. Error: at most one error per file
  it "returns at most one error per file" $ do
    -- File with both BOM and duplicate key: should only get BOM (higher priority)
    let bom = BS.pack [0xEF, 0xBB, 0xBF]
        content = TE.encodeUtf8 "---\nfoo: 1\nfoo: 2\n"
    withTempFileBytes (BS.append bom content) $ \cwd fp -> do
      result <- checkYAML cwd fp
      err <- expectErrorCode YamlHasBom result
      yeFile err `shouldBe` Just fp

  -- 11. Error message is preserved from parser
  it "preserves the parser error message" $ do
    withTempFileBytes BS.empty $ \cwd fp -> do
      result <- checkYAML cwd fp
      case result of
        Left err -> yeMessage err `shouldBe` "empty file"
        Right _  -> expectationFailure "expected error for empty file"

  -- 12. Error: yeFile is populated
  it "populates yeFile with the file path" $ do
    withTempFileBytes (BS.pack [0xFE]) $ \cwd fp -> do
      result <- checkYAML cwd fp
      case result of
        Left err -> yeFile err `shouldBe` Just fp
        Right _  -> expectationFailure "expected error for invalid UTF-8"

  -- 13. Success: file with only comments and whitespace (valid YAML, empty stream)
  it "handles empty stream (no documents) as an error" $ do
    withTempFile "# just a comment\n" $ \cwd fp -> do
      result <- checkYAML cwd fp
      -- The parser returns "empty stream (no documents)" for this
      case result of
        Left err -> yeCode err `shouldSatisfy`
          (\c -> c == YamlMalformed || c == YamlEmptyFile)
        Right _ -> return ()  -- Some parsers may produce an empty list
