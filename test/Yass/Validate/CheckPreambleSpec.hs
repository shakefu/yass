{-# LANGUAGE OverloadedStrings #-}

module Yass.Validate.CheckPreambleSpec (spec) where

import Test.Hspec
import Data.Text (Text)
import Yass.Validate.CheckPreamble (checkPreamble)
import Yass.Types (YassError(..), ErrorCode(..))
import Yass.Parser (RawDocument(..), RawValue(..))

-- | Helper: create a spec doc (has "spec" key) at a given line
mkSpecDoc :: Int -> Text -> RawDocument
mkSpecDoc line name = RawDocument line [("spec", RVString name, line)]

-- | Helper: a valid preamble with description and version v1
validPreamble :: Int -> RawDocument
validPreamble line = RawDocument line
  [ ("description", RVString "A test spec file", line)
  , ("version", RVString "v1", line + 1)
  ]

-- | Shorthand for the file path arguments
cwd :: FilePath
cwd = "/project"

fp :: FilePath
fp = "/project/test.yass.yaml"

spec :: Spec
spec = describe "Yass.Validate.CheckPreamble" $ do

  -- ---- Empty stream ----

  describe "empty stream" $ do
    it "returns YamlEmptyStream when docs list is empty" $ do
      let errs = checkPreamble cwd fp []
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` YamlEmptyStream
      yeFile (head errs) `shouldBe` Just fp
      yeLine (head errs) `shouldBe` Nothing

  -- ---- has_spec_key ----

  describe "has_spec_key" $ do
    it "returns PreambleHasSpecKey when first doc has 'spec' key" $ do
      let doc = RawDocument 1 [("spec", RVString "foo.bar", 1)]
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleHasSpecKey
      yeLine (head errs) `shouldBe` Just 1

    it "returns PreambleHasSpecKey even if first doc also has description/version" $ do
      let doc = RawDocument 1
            [ ("spec", RVString "foo.bar", 1)
            , ("description", RVString "test", 2)
            , ("version", RVString "v1", 3)
            ]
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleHasSpecKey

  -- ---- duplicate ----

  describe "duplicate preamble" $ do
    it "returns PreambleDuplicate when multiple non-first docs lack 'spec' key" $ do
      let preamble = validPreamble 1
          extra1 = RawDocument 10 [("description", RVString "extra1", 10)]
          extra2 = RawDocument 20 [("description", RVString "extra2", 20)]
      let errs = checkPreamble cwd fp [preamble, extra1, extra2]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleDuplicate
      yeLine (head errs) `shouldBe` Just 10

    it "returns PreambleDuplicate when 3 non-first docs lack 'spec' key" $ do
      let preamble = validPreamble 1
          extra1 = RawDocument 10 []
          extra2 = RawDocument 20 []
          extra3 = RawDocument 30 []
      let errs = checkPreamble cwd fp [preamble, extra1, extra2, extra3]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleDuplicate

  -- ---- misplaced ----

  describe "misplaced preamble" $ do
    it "returns PreambleDuplicate when one non-first doc lacks 'spec' key" $ do
      let preamble = validPreamble 1
          specDoc = mkSpecDoc 5 "foo.bar"
          extra = RawDocument 15 [("description", RVString "extra", 15)]
      let errs = checkPreamble cwd fp [preamble, specDoc, extra]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleDuplicate
      yeLine (head errs) `shouldBe` Just 15

  -- ---- missing_description ----

  describe "missing description" $ do
    it "returns PreambleMissingDescription when preamble lacks 'description'" $ do
      let doc = RawDocument 1 [("version", RVString "v1", 1)]
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleMissingDescription
      yeLine (head errs) `shouldBe` Just 1

    it "returns PreambleMissingDescription for completely empty preamble" $ do
      let doc = RawDocument 1 []
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleMissingDescription

  -- ---- missing_version ----

  describe "missing version" $ do
    it "returns PreambleMissingVersion when preamble has description but no version" $ do
      let doc = RawDocument 1 [("description", RVString "test", 1)]
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleMissingVersion
      yeLine (head errs) `shouldBe` Just 1

  -- ---- unknown_version ----

  describe "unknown version" $ do
    it "returns PreambleUnknownVersion for version 'v2'" $ do
      let doc = RawDocument 1
            [ ("description", RVString "test", 1)
            , ("version", RVString "v2", 2)
            ]
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleUnknownVersion

    it "returns PreambleUnknownVersion for null version" $ do
      let doc = RawDocument 1
            [ ("description", RVString "test", 1)
            , ("version", RVNull, 2)
            ]
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleUnknownVersion

    it "returns PreambleUnknownVersion for boolean version" $ do
      let doc = RawDocument 1
            [ ("description", RVString "test", 1)
            , ("version", RVBool True, 2)
            ]
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleUnknownVersion

    it "returns PreambleUnknownVersion for numeric version" $ do
      let doc = RawDocument 1
            [ ("description", RVString "test", 1)
            , ("version", RVNumber 1.0, 2)
            ]
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleUnknownVersion

  -- ---- bad_related ----

  describe "bad related" $ do
    it "returns PreambleBadRelated when related is a string" $ do
      let doc = RawDocument 1
            [ ("description", RVString "test", 1)
            , ("version", RVString "v1", 2)
            , ("related", RVString "not-a-list", 3)
            ]
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleBadRelated

    it "returns PreambleBadRelated when related list contains non-strings" $ do
      let doc = RawDocument 1
            [ ("description", RVString "test", 1)
            , ("version", RVString "v1", 2)
            , ("related", RVList [RVString "ok", RVNumber 42] 3, 3)
            ]
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleBadRelated

    it "returns PreambleBadRelated when related is a mapping" $ do
      let doc = RawDocument 1
            [ ("description", RVString "test", 1)
            , ("version", RVString "v1", 2)
            , ("related", RVMapping [("key", RVString "val", 3)] 3, 3)
            ]
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleBadRelated

  -- ---- valid preamble (no errors) ----

  describe "valid preamble" $ do
    it "returns no errors for a valid preamble with description and v1" $ do
      let doc = validPreamble 1
      let errs = checkPreamble cwd fp [doc]
      errs `shouldBe` []

    it "returns no errors for valid preamble with related list of strings" $ do
      let doc = RawDocument 1
            [ ("description", RVString "test", 1)
            , ("version", RVString "v1", 2)
            , ("related", RVList [RVString "foo.yaml", RVString "bar.yaml"] 3, 3)
            ]
      let errs = checkPreamble cwd fp [doc]
      errs `shouldBe` []

    it "returns no errors for valid preamble with empty related list" $ do
      let doc = RawDocument 1
            [ ("description", RVString "test", 1)
            , ("version", RVString "v1", 2)
            , ("related", RVList [] 3, 3)
            ]
      let errs = checkPreamble cwd fp [doc]
      errs `shouldBe` []

    it "returns no errors for valid preamble followed by spec docs" $ do
      let preamble = validPreamble 1
          specDoc1 = mkSpecDoc 5 "foo.bar"
          specDoc2 = mkSpecDoc 10 "baz.qux"
      let errs = checkPreamble cwd fp [preamble, specDoc1, specDoc2]
      errs `shouldBe` []

  -- ---- priority ordering ----

  describe "priority ordering" $ do
    it "has_spec_key takes priority over empty preamble content" $ do
      -- First doc has spec key but no description/version
      let doc = RawDocument 1 [("spec", RVString "x", 1)]
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleHasSpecKey

    it "duplicate takes priority over misplaced when multiple non-spec docs exist" $ do
      let preamble = validPreamble 1
          extra1 = RawDocument 10 [("foo", RVString "bar", 10)]
          extra2 = RawDocument 20 [("baz", RVString "qux", 20)]
      let errs = checkPreamble cwd fp [preamble, extra1, extra2]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleDuplicate

    it "missing_description takes priority over missing_version" $ do
      -- Preamble with neither description nor version
      let doc = RawDocument 1 [("foo", RVString "bar", 1)]
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleMissingDescription

    it "missing_version takes priority over unknown_version" $ do
      -- Preamble with description but no version
      let doc = RawDocument 1 [("description", RVString "test", 1)]
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleMissingVersion

    it "unknown_version takes priority over bad_related" $ do
      let doc = RawDocument 1
            [ ("description", RVString "test", 1)
            , ("version", RVString "v999", 2)
            , ("related", RVString "not-a-list", 3)
            ]
      let errs = checkPreamble cwd fp [doc]
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` PreambleUnknownVersion

  -- ---- at most one error ----

  describe "at most one error" $ do
    it "returns at most one error regardless of how many problems exist" $ do
      -- Empty list -> exactly one error
      let errs = checkPreamble cwd fp []
      length errs `shouldBe` 1
