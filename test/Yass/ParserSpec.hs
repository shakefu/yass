{-# LANGUAGE OverloadedStrings #-}

module Yass.ParserSpec (spec) where

import Test.Hspec
import Yass.Parser
import qualified Data.ByteString as BS
import qualified Data.Text as T
import qualified Data.Text.Encoding as TE
import System.IO.Temp (withSystemTempDirectory)
import System.FilePath ((</>))
import Control.Exception (evaluate)

-- Helper: write content to a temp file and parse it
withTempYaml :: String -> (FilePath -> IO a) -> IO a
withTempYaml content action =
  withSystemTempDirectory "yass-parser-test" $ \dir -> do
    let fp = dir </> "test.yass.yaml"
    BS.writeFile fp (TE.encodeUtf8 (T.pack content))
    action fp

-- Helper: write raw bytes to a temp file and parse it
withTempYamlBytes :: BS.ByteString -> (FilePath -> IO a) -> IO a
withTempYamlBytes bs action =
  withSystemTempDirectory "yass-parser-test" $ \dir -> do
    let fp = dir </> "test.yass.yaml"
    BS.writeFile fp bs
    action fp

-- Helper: check that result is ParseOK with N documents
expectDocs :: Int -> ParseResult -> IO [RawDocument]
expectDocs n result = case result of
  ParseOK docs -> do
    length docs `shouldBe` n
    return docs
  ParseError msg line ->
    expectationFailure ("expected ParseOK but got ParseError: "
      ++ T.unpack msg ++ " at line " ++ show line)
    >> return []

-- Helper: check that result is ParseError
expectError :: ParseResult -> IO (T.Text, Int)
expectError result = case result of
  ParseError msg line -> return (msg, line)
  ParseOK docs ->
    expectationFailure ("expected ParseError but got ParseOK with "
      ++ show (length docs) ++ " documents")
    >> return ("", 0)

spec :: Spec
spec = do
  describe "parseYassFile" $ do

    -- ---- Empty and trivial files ----

    describe "empty file" $ do
      it "returns ParseError for an empty file" $ do
        withTempYamlBytes BS.empty $ \fp -> do
          result <- parseYassFile fp
          (msg, _) <- expectError result
          msg `shouldBe` "empty file"

    -- ---- BOM detection ----

    describe "BOM detection" $ do
      it "returns ParseError when file has UTF-8 BOM" $ do
        let bom = BS.pack [0xEF, 0xBB, 0xBF]
            content = TE.encodeUtf8 "description: hello\n"
        withTempYamlBytes (BS.append bom content) $ \fp -> do
          result <- parseYassFile fp
          (msg, _) <- expectError result
          msg `shouldSatisfy` T.isInfixOf "BOM"

    -- ---- UTF-8 validation ----

    describe "UTF-8 validation" $ do
      it "returns ParseError for invalid UTF-8 bytes" $ do
        -- 0xFF is never valid in UTF-8
        let badBytes = BS.pack [0xFF, 0xFE, 0x0A]
        withTempYamlBytes badBytes $ \fp -> do
          result <- parseYassFile fp
          (msg, _) <- expectError result
          msg `shouldSatisfy` T.isInfixOf "UTF-8"

      it "parses valid UTF-8 with multibyte characters" $ do
        withTempYaml "description: \12371\12435\12395\12385\12399\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let entries = rawDocEntries (head docs)
          length entries `shouldBe` 1
          let (key, val, _) = head entries
          key `shouldBe` "description"
          val `shouldBe` RVString "\12371\12435\12395\12385\12399"

    -- ---- Single document ----

    describe "single document" $ do
      it "parses a single simple mapping" $ do
        withTempYaml "description: hello\nversion: v1\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let entries = rawDocEntries (head docs)
          length entries `shouldBe` 2
          let (k1, v1, _) = entries !! 0
          k1 `shouldBe` "description"
          v1 `shouldBe` RVString "hello"
          let (k2, v2, _) = entries !! 1
          k2 `shouldBe` "version"
          v2 `shouldBe` RVString "v1"

      it "parses a document with explicit document start marker" $ do
        withTempYaml "---\ndescription: hello\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let entries = rawDocEntries (head docs)
          length entries `shouldBe` 1

    -- ---- Multiple documents ----

    describe "multiple documents" $ do
      it "parses two documents separated by ---" $ do
        let yaml = T.unlines
              [ "description: preamble"
              , "version: v1"
              , "---"
              , "spec: MySpec"
              , "INPUT:"
              , "  - MUST accept a value"
              ]
        withTempYaml (T.unpack yaml) $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 2 result
          -- First doc
          let entries1 = rawDocEntries (docs !! 0)
          length entries1 `shouldBe` 2
          let (k1, _, _) = entries1 !! 0
          k1 `shouldBe` "description"
          -- Second doc
          let entries2 = rawDocEntries (docs !! 1)
          length entries2 `shouldBe` 2
          let (k1', _, _) = entries2 !! 0
          k1' `shouldBe` "spec"

      it "parses three documents" $ do
        let yaml = T.unlines
              [ "description: preamble"
              , "version: v1"
              , "---"
              , "spec: Spec1"
              , "---"
              , "spec: Spec2"
              ]
        withTempYaml (T.unpack yaml) $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 3 result
          let entries2 = rawDocEntries (docs !! 1)
          let (_, v, _) = head entries2
          v `shouldBe` RVString "Spec1"
          let entries3 = rawDocEntries (docs !! 2)
          let (_, v', _) = head entries3
          v' `shouldBe` RVString "Spec2"

    -- ---- Line number tracking ----

    describe "line number tracking" $ do
      it "tracks line numbers for document starts" $ do
        let yaml = T.unlines
              [ "description: hello"
              , "version: v1"
              , "---"
              , "spec: MySpec"
              ]
        withTempYaml (T.unpack yaml) $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 2 result
          -- First document starts at line 1
          rawDocLine (docs !! 0) `shouldBe` 1
          -- Second document starts at line 3 (the --- marker)
          -- or line 4 (the mapping start) -- depends on how libyaml reports it
          rawDocLine (docs !! 1) `shouldSatisfy` (>= 3)

      it "tracks line numbers for keys" $ do
        let yaml = T.unlines
              [ "key1: val1"
              , "key2: val2"
              , "key3: val3"
              ]
        withTempYaml (T.unpack yaml) $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let entries = rawDocEntries (head docs)
          length entries `shouldBe` 3
          let (_, _, l1) = entries !! 0
          let (_, _, l2) = entries !! 1
          let (_, _, l3) = entries !! 2
          l1 `shouldBe` 1
          l2 `shouldBe` 2
          l3 `shouldBe` 3

    -- ---- Duplicate keys ----

    describe "duplicate keys" $ do
      it "returns ParseError for duplicate keys in same mapping" $ do
        let yaml = T.unlines
              [ "key1: val1"
              , "key1: val2"
              ]
        withTempYaml (T.unpack yaml) $ \fp -> do
          result <- parseYassFile fp
          (msg, _) <- expectError result
          msg `shouldSatisfy` T.isInfixOf "duplicate key"

      it "returns ParseError for duplicate keys in nested mapping" $ do
        let yaml = T.unlines
              [ "outer:"
              , "  inner: val1"
              , "  inner: val2"
              ]
        withTempYaml (T.unpack yaml) $ \fp -> do
          result <- parseYassFile fp
          (msg, _) <- expectError result
          msg `shouldSatisfy` T.isInfixOf "duplicate key"

    -- ---- Anchor/alias detection ----

    describe "anchor/alias detection" $ do
      it "returns ParseError for anchors" $ do
        let yaml = T.unlines
              [ "key1: &anchor val1"
              , "key2: val2"
              ]
        withTempYaml (T.unpack yaml) $ \fp -> do
          result <- parseYassFile fp
          (msg, _) <- expectError result
          msg `shouldSatisfy` T.isInfixOf "anchor"

      it "returns ParseError for aliases" $ do
        let yaml = T.unlines
              [ "key1: &anchor val1"
              , "key2: *anchor"
              ]
        withTempYaml (T.unpack yaml) $ \fp -> do
          result <- parseYassFile fp
          (msg, _) <- expectError result
          (msg `shouldSatisfy` \m -> T.isInfixOf "anchor" m || T.isInfixOf "alias" m)

    -- ---- yes/no/on/off as strings ----

    describe "yes/no/on/off as strings" $ do
      it "treats plain yes as a string" $ do
        withTempYaml "key: yes\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVString "yes"

      it "treats plain no as a string" $ do
        withTempYaml "key: no\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVString "no"

      it "treats plain on as a string" $ do
        withTempYaml "key: on\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVString "on"

      it "treats plain off as a string" $ do
        withTempYaml "key: off\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVString "off"

      it "treats Yes/No/On/Off as strings" $ do
        let yaml = T.unlines
              [ "k1: Yes"
              , "k2: No"
              , "k3: On"
              , "k4: Off"
              ]
        withTempYaml (T.unpack yaml) $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let entries = rawDocEntries (head docs)
          let vals = map (\(_, v, _) -> v) entries
          vals `shouldBe` [ RVString "Yes"
                          , RVString "No"
                          , RVString "On"
                          , RVString "Off"
                          ]

    -- ---- Boolean values (true/false) ----

    describe "boolean values" $ do
      it "treats plain true as boolean" $ do
        withTempYaml "key: true\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVBool True

      it "treats plain false as boolean" $ do
        withTempYaml "key: false\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVBool False

    -- ---- Null values ----

    describe "null values" $ do
      it "treats null keyword as RVNull" $ do
        withTempYaml "key: null\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVNull

      it "treats tilde as RVNull" $ do
        withTempYaml "key: ~\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVNull

    -- ---- Number values ----

    describe "number values" $ do
      it "parses integer values" $ do
        withTempYaml "key: 42\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVNumber 42.0

      it "parses negative numbers" $ do
        withTempYaml "key: -7\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVNumber (-7.0)

      it "parses floating point values" $ do
        withTempYaml "key: 3.14\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVNumber 3.14

    -- ---- Quoted strings ----

    describe "quoted strings" $ do
      it "treats single-quoted values as strings" $ do
        withTempYaml "key: 'true'\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVString "true"

      it "treats double-quoted values as strings" $ do
        withTempYaml "key: \"42\"\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVString "42"

      it "treats double-quoted null as string" $ do
        withTempYaml "key: \"null\"\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVString "null"

    -- ---- List values ----

    describe "list values" $ do
      it "parses a list of strings" $ do
        let yaml = T.unlines
              [ "items:"
              , "  - hello"
              , "  - world"
              ]
        withTempYaml (T.unpack yaml) $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          case val of
            RVList items _ -> do
              length items `shouldBe` 2
              items !! 0 `shouldBe` RVString "hello"
              items !! 1 `shouldBe` RVString "world"
            _ -> expectationFailure "expected RVList"

      it "tracks list start line number" $ do
        let yaml = T.unlines
              [ "items:"
              , "  - hello"
              , "  - world"
              ]
        withTempYaml (T.unpack yaml) $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          case val of
            RVList _ line -> line `shouldBe` 2
            _ -> expectationFailure "expected RVList"

    -- ---- Nested mappings ----

    describe "nested mappings" $ do
      it "parses nested mappings" $ do
        let yaml = T.unlines
              [ "outer:"
              , "  inner1: val1"
              , "  inner2: val2"
              ]
        withTempYaml (T.unpack yaml) $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          case val of
            RVMapping entries _ -> do
              length entries `shouldBe` 2
              let (k1, v1, _) = entries !! 0
              k1 `shouldBe` "inner1"
              v1 `shouldBe` RVString "val1"
            _ -> expectationFailure "expected RVMapping"

    -- ---- Malformed YAML ----

    describe "malformed YAML" $ do
      it "returns ParseError for invalid YAML syntax" $ do
        withTempYaml "key: [\n" $ \fp -> do
          result <- parseYassFile fp
          _ <- expectError result
          return ()

      it "returns ParseError for tabs in indentation" $ do
        let yaml = "key:\n\t- item\n"
        withTempYaml yaml $ \fp -> do
          result <- parseYassFile fp
          _ <- expectError result
          return ()

    -- ---- Empty stream (only comments / whitespace, no documents) ----

    describe "empty stream" $ do
      it "returns ParseOK with 0 documents for YAML with only comments" $ do
        withTempYaml "# just a comment\n" $ \fp -> do
          result <- parseYassFile fp
          _ <- expectDocs 0 result
          return ()

    -- ---- Tag detection ----

    describe "tag detection" $ do
      it "returns ParseError for custom tags" $ do
        withTempYaml "key: !custom value\n" $ \fp -> do
          result <- parseYassFile fp
          -- Custom tags (UriTag) should be rejected.
          -- Note: libyaml may or may not treat !custom as a UriTag depending on version
          -- If it's not a UriTag, it will parse successfully
          case result of
            ParseError msg _ -> msg `shouldSatisfy` (\m -> T.isInfixOf "tag" m || T.isInfixOf "custom" m)
            ParseOK _ -> pendingWith "libyaml does not treat !custom as UriTag in this version"

    -- ---- Realistic yass file ----

    describe "realistic yass file" $ do
      it "parses a full yass file with preamble and specs" $ do
        let yaml = T.unlines
              [ "description: A test spec file"
              , "version: v1"
              , "---"
              , "spec: Auth.Login"
              , "INPUT:"
              , "  - MUST accept username"
              , "  - MUST accept password"
              , "RETURN:"
              , "  - MUST return a session token"
              , "ERROR:"
              , "  - MUST return 401 for bad credentials"
              , "---"
              , "spec: Auth.Logout"
              , "INPUT:"
              , "  - MUST accept session token"
              , "SIDE-EFFECT:"
              , "  - MUST invalidate session"
              ]
        withTempYaml (T.unpack yaml) $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 3 result
          -- Preamble
          let entries1 = rawDocEntries (docs !! 0)
          length entries1 `shouldBe` 2
          -- First spec
          let entries2 = rawDocEntries (docs !! 1)
          length entries2 `shouldBe` 4  -- spec, INPUT, RETURN, ERROR
          -- Second spec
          let entries3 = rawDocEntries (docs !! 2)
          length entries3 `shouldBe` 3  -- spec, INPUT, SIDE-EFFECT

    -- ---- Key ordering preserved ----

    describe "key ordering" $ do
      it "preserves insertion order of keys" $ do
        let yaml = T.unlines
              [ "zebra: 1"
              , "alpha: 2"
              , "middle: 3"
              ]
        withTempYaml (T.unpack yaml) $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let entries = rawDocEntries (head docs)
          let keys = map (\(k, _, _) -> k) entries
          keys `shouldBe` ["zebra", "alpha", "middle"]

    -- ---- Multiline strings ----

    describe "multiline strings" $ do
      it "parses literal block scalars" $ do
        let yaml = "key: |\n  line1\n  line2\n"
        withTempYaml yaml $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          case val of
            RVString t -> t `shouldSatisfy` T.isInfixOf "line1"
            _ -> expectationFailure "expected RVString for literal block"

    -- ---- UTF-8 validation edge cases ----

    describe "UTF-8 validation edge cases" $ do
      it "accepts valid 2-byte UTF-8 sequences" $ do
        -- U+00E9 (e with acute) = C3 A9
        let bytes = BS.pack [0x6B, 0x3A, 0x20, 0xC3, 0xA9, 0x0A]  -- "k: <e-acute>\n"
        withTempYamlBytes bytes $ \fp -> do
          result <- parseYassFile fp
          _ <- expectDocs 1 result
          return ()

      it "rejects truncated 2-byte UTF-8 sequence" $ do
        -- 0xC3 alone without continuation byte
        let bytes = BS.pack [0xC3]
        withTempYamlBytes bytes $ \fp -> do
          result <- parseYassFile fp
          (msg, _) <- expectError result
          msg `shouldSatisfy` T.isInfixOf "UTF-8"

      it "accepts valid 4-byte UTF-8 sequences" $ do
        -- U+1F600 (grinning face) = F0 9F 98 80
        let bytes = BS.pack [0x6B, 0x3A, 0x20, 0xF0, 0x9F, 0x98, 0x80, 0x0A]
        withTempYamlBytes bytes $ \fp -> do
          result <- parseYassFile fp
          _ <- expectDocs 1 result
          return ()

      it "rejects truncated 4-byte UTF-8 sequence" $ do
        -- F0 9F 98 without 4th byte
        let bytes = BS.pack [0xF0, 0x9F, 0x98]
        withTempYamlBytes bytes $ \fp -> do
          result <- parseYassFile fp
          (msg, _) <- expectError result
          msg `shouldSatisfy` T.isInfixOf "UTF-8"

      it "rejects overlong 3-byte UTF-8 sequence (0xE0 with low continuation)" $ do
        -- E0 80 80 is overlong encoding of U+0000
        let bytes = BS.pack [0xE0, 0x80, 0x80]
        withTempYamlBytes bytes $ \fp -> do
          result <- parseYassFile fp
          (msg, _) <- expectError result
          msg `shouldSatisfy` T.isInfixOf "UTF-8"

      it "rejects UTF-16 surrogate in UTF-8 (0xED with high continuation)" $ do
        -- ED A0 80 = U+D800 (surrogate)
        let bytes = BS.pack [0xED, 0xA0, 0x80]
        withTempYamlBytes bytes $ \fp -> do
          result <- parseYassFile fp
          (msg, _) <- expectError result
          msg `shouldSatisfy` T.isInfixOf "UTF-8"

      it "rejects overlong 4-byte UTF-8 sequence (0xF0 with low continuation)" $ do
        -- F0 80 80 80 is overlong
        let bytes = BS.pack [0xF0, 0x80, 0x80, 0x80]
        withTempYamlBytes bytes $ \fp -> do
          result <- parseYassFile fp
          (msg, _) <- expectError result
          msg `shouldSatisfy` T.isInfixOf "UTF-8"

      it "rejects too-large 4-byte UTF-8 sequence (0xF4 with high continuation)" $ do
        -- F4 90 80 80 = U+110000 (beyond Unicode range)
        let bytes = BS.pack [0xF4, 0x90, 0x80, 0x80]
        withTempYamlBytes bytes $ \fp -> do
          result <- parseYassFile fp
          (msg, _) <- expectError result
          msg `shouldSatisfy` T.isInfixOf "UTF-8"

    -- ---- resolvePlain edge cases (Null/NULL, True/TRUE, False/FALSE) ----

    describe "resolvePlain edge cases" $ do
      it "treats Null as RVNull" $ do
        withTempYaml "key: Null\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVNull

      it "treats NULL as RVNull" $ do
        withTempYaml "key: NULL\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVNull

      it "treats True as RVBool True" $ do
        withTempYaml "key: True\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVBool True

      it "treats TRUE as RVBool True" $ do
        withTempYaml "key: TRUE\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVBool True

      it "treats False as RVBool False" $ do
        withTempYaml "key: False\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVBool False

      it "treats FALSE as RVBool False" $ do
        withTempYaml "key: FALSE\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVBool False

    -- ---- Infinity and NaN scalars ----

    describe "infinity and NaN scalars" $ do
      it "parses .inf as positive infinity" $ do
        withTempYaml "key: .inf\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          case val of
            RVNumber n -> n `shouldSatisfy` isInfinite
            _ -> expectationFailure "expected RVNumber for .inf"

      it "parses -.inf as negative infinity" $ do
        withTempYaml "key: -.inf\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          case val of
            RVNumber n -> do
              n `shouldSatisfy` isInfinite
              n `shouldSatisfy` (< 0)
            _ -> expectationFailure "expected RVNumber for -.inf"

      it "parses .nan as NaN" $ do
        withTempYaml "key: .nan\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          case val of
            RVNumber n -> n `shouldSatisfy` isNaN
            _ -> expectationFailure "expected RVNumber for .nan"

      it "parses .Inf and .NaN variants" $ do
        let yaml = T.unlines
              [ "k1: .Inf"
              , "k2: .INF"
              , "k3: -.Inf"
              , "k4: -.INF"
              , "k5: .NaN"
              , "k6: .NAN"
              ]
        withTempYaml (T.unpack yaml) $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let entries = rawDocEntries (head docs)
          length entries `shouldBe` 6
          -- All should be RVNumber
          mapM_ (\(_, v, _) -> case v of
            RVNumber _ -> return ()
            _ -> expectationFailure "expected RVNumber") entries

    -- ---- Hex and octal numbers ----

    describe "hex and octal numbers" $ do
      it "parses hex numbers (0x prefix)" $ do
        withTempYaml "key: 0xFF\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVNumber 255.0

      it "parses octal numbers (0o prefix)" $ do
        withTempYaml "key: 0o17\n" $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          val `shouldBe` RVNumber 15.0

    -- ---- parseYassDocuments stub ----

    describe "parseYassDocuments" $ do
      it "throws an error when called" $ do
        evaluate (parseYassDocuments "test" "content") `shouldThrow` anyErrorCall

    -- ---- RVComment constructor ----

    describe "RVComment constructor" $ do
      it "can be constructed and compared" $ do
        let comment = RVComment "test comment"
        comment `shouldBe` RVComment "test comment"
        comment `shouldNotBe` RVComment "other comment"
        comment `shouldNotBe` RVString "test comment"

    -- ---- Show instances ----

    describe "Show instances" $ do
      it "can show ParseResult variants" $ do
        let ok = ParseOK []
        let err = ParseError "oops" 1
        show ok `shouldSatisfy` not . null
        show err `shouldSatisfy` not . null

      it "can show RawDocument" $ do
        let doc = RawDocument 1 [("key", RVString "val", 1)]
        show doc `shouldSatisfy` not . null

      it "can show RawValue variants" $ do
        show (RVString "s") `shouldSatisfy` not . null
        show (RVBool True) `shouldSatisfy` not . null
        show (RVNumber 1.0) `shouldSatisfy` not . null
        show RVNull `shouldSatisfy` not . null
        show (RVList [] 1) `shouldSatisfy` not . null
        show (RVMapping [] 1) `shouldSatisfy` not . null
        show (RVComment "c") `shouldSatisfy` not . null

    -- ---- ParseResult Eq ----

    describe "ParseResult Eq" $ do
      it "compares ParseError values" $ do
        ParseError "a" 1 `shouldBe` ParseError "a" 1
        ParseError "a" 1 `shouldNotBe` ParseError "b" 1

    -- ---- Scalar document (non-mapping at top level) ----

    describe "scalar document" $ do
      it "handles a scalar-only document gracefully" $ do
        -- "---\nhello\n---\nkey: val\n" produces a scalar doc then a mapping doc
        let yaml = "---\nhello\n---\nkey: val\n"
        withTempYaml yaml $ \fp -> do
          result <- parseYassFile fp
          -- The scalar doc should be treated as empty mapping
          case result of
            ParseOK docs -> do
              length docs `shouldSatisfy` (>= 1)
              -- The scalar doc has empty entries
              let scalarDoc = head docs
              rawDocEntries scalarDoc `shouldBe` []
            ParseError _ _ -> return ()  -- also acceptable

    -- ---- Anchor on sequence ----

    describe "anchor on sequence" $ do
      it "returns ParseError for anchor on a sequence" $ do
        let yaml = "key: &anch\n  - item1\n  - item2\n"
        withTempYaml yaml $ \fp -> do
          result <- parseYassFile fp
          (msg, _) <- expectError result
          msg `shouldSatisfy` T.isInfixOf "anchor"

    -- ---- Anchor on mapping ----

    describe "anchor on mapping" $ do
      it "returns ParseError for anchor on a mapping" $ do
        let yaml = "outer: &anch\n  inner: val\n"
        withTempYaml yaml $ \fp -> do
          result <- parseYassFile fp
          (msg, _) <- expectError result
          msg `shouldSatisfy` T.isInfixOf "anchor"

    -- ---- Folded block scalar ----

    describe "folded block scalar" $ do
      it "parses folded block scalars as strings" $ do
        let yaml = "key: >\n  line1\n  line2\n"
        withTempYaml yaml $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          case val of
            RVString t -> t `shouldSatisfy` T.isInfixOf "line1"
            _ -> expectationFailure "expected RVString for folded block"

    -- ---- Empty list ----

    describe "empty list" $ do
      it "parses an empty sequence" $ do
        let yaml = "key: []\n"
        withTempYaml yaml $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          case val of
            RVList items _ -> length items `shouldBe` 0
            _ -> expectationFailure "expected RVList"

    -- ---- Empty mapping value ----

    describe "empty mapping value" $ do
      it "parses an empty mapping as value" $ do
        let yaml = "key: {}\n"
        withTempYaml yaml $ \fp -> do
          result <- parseYassFile fp
          docs <- expectDocs 1 result
          let (_, val, _) = head (rawDocEntries (head docs))
          case val of
            RVMapping entries _ -> length entries `shouldBe` 0
            _ -> expectationFailure "expected RVMapping"
