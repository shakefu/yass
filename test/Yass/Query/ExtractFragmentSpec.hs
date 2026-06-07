{-# LANGUAGE OverloadedStrings #-}

module Yass.Query.ExtractFragmentSpec (spec) where

import Test.Hspec
import Yass.Parser (RawDocument(..), RawValue(..))
import Yass.Query.ExtractFragment

-- Helper to make a preamble document
mkPreamble :: RawDocument
mkPreamble = RawDocument
  { rawDocLine = 1
  , rawDocEntries = [("description", RVString "test", 2), ("version", RVString "v1", 3)]
  }

-- Helper to make a spec document with just a name
mkSpec :: RawDocument
mkSpec = RawDocument
  { rawDocLine = 10
  , rawDocEntries = [("spec", RVString "MySpec", 10)]
  }

-- Helper to make a spec document with a name and slot
mkSpecWithSlot :: RawDocument
mkSpecWithSlot = RawDocument
  { rawDocLine = 20
  , rawDocEntries =
    [ ("spec", RVString "MySpec", 20)
    , ("INPUT", RVList [RVMapping [("MUST", RVString "be valid", 22)] 22] 21, 21)
    ]
  }

-- Helper for a different spec
mkOtherSpec :: RawDocument
mkOtherSpec = RawDocument
  { rawDocLine = 30
  , rawDocEntries = [("spec", RVString "OtherSpec", 30)]
  }

spec :: Spec
spec = describe "ExtractFragment" $ do

  describe "extractFragment" $ do
    it "returns Nothing for empty docs" $ do
      extractFragment "MySpec" [] `shouldBe` Nothing

    it "returns Nothing when spec not found" $ do
      extractFragment "NonExistent" [mkPreamble, mkSpec] `shouldBe` Nothing

    it "returns the matching spec document" $ do
      extractFragment "MySpec" [mkPreamble, mkSpec] `shouldBe` Just mkSpec

    it "returns the correct spec when multiple exist" $ do
      extractFragment "OtherSpec" [mkPreamble, mkSpec, mkOtherSpec]
        `shouldBe` Just mkOtherSpec

    it "returns spec with slot data preserved" $ do
      extractFragment "MySpec" [mkPreamble, mkSpecWithSlot]
        `shouldBe` Just mkSpecWithSlot

    it "skips preamble (first doc) even if it somehow has the name" $ do
      let preambleWithSpec = RawDocument 1 [("spec", RVString "Preamble", 1)]
      extractFragment "Preamble" [preambleWithSpec, mkSpec] `shouldBe` Nothing

    it "returns first matching spec when duplicates exist" $ do
      let spec1 = RawDocument 10 [("spec", RVString "Dup", 10)]
          spec2 = RawDocument 20 [("spec", RVString "Dup", 20)]
      extractFragment "Dup" [mkPreamble, spec1, spec2] `shouldBe` Just spec1

    it "does not match non-string spec value" $ do
      let badSpec = RawDocument 10 [("spec", RVBool True, 10)]
      extractFragment "True" [mkPreamble, badSpec] `shouldBe` Nothing

    it "handles preamble-only docs (no specs)" $ do
      extractFragment "Foo" [mkPreamble] `shouldBe` Nothing

    it "handles spec with multiple entries" $ do
      let multiEntry = RawDocument 10
            [ ("spec", RVString "Multi", 10)
            , ("INPUT", RVList [] 11, 11)
            , ("RETURN", RVList [] 12, 12)
            ]
      extractFragment "Multi" [mkPreamble, multiEntry] `shouldBe` Just multiEntry

    it "does not match null spec value" $ do
      let nullSpec = RawDocument 10 [("spec", RVNull, 10)]
      extractFragment "null" [mkPreamble, nullSpec] `shouldBe` Nothing

    it "does not match numeric spec value" $ do
      let numSpec = RawDocument 10 [("spec", RVNumber 42, 10)]
      extractFragment "42" [mkPreamble, numSpec] `shouldBe` Nothing

  describe "findSpecDoc" $ do
    it "finds a spec by name in a list" $ do
      findSpecDoc "MySpec" [mkSpec] `shouldBe` Just mkSpec

    it "returns Nothing when list is empty" $ do
      findSpecDoc "MySpec" [] `shouldBe` Nothing

    it "returns Nothing when no match" $ do
      findSpecDoc "Nope" [mkSpec] `shouldBe` Nothing

    it "finds second spec in a list" $ do
      findSpecDoc "OtherSpec" [mkSpec, mkOtherSpec] `shouldBe` Just mkOtherSpec

    it "is case-sensitive" $ do
      findSpecDoc "myspec" [mkSpec] `shouldBe` Nothing
