{-# LANGUAGE OverloadedStrings #-}

module Yass.Query.NameLookupSpec (spec) where

import Data.Text (Text)
import Test.Hspec
import Yass.Parser (RawDocument(..), RawValue(..))
import Yass.Query.NameLookup

-- Helper to make a preamble document
mkPreamble :: RawDocument
mkPreamble = RawDocument
  { rawDocLine = 1
  , rawDocEntries = [("description", RVString "test", 2), ("version", RVString "v1", 3)]
  }

-- Helper to make a spec document
mkSpec :: Text -> RawDocument
mkSpec name = RawDocument
  { rawDocLine = 10
  , rawDocEntries = [("spec", RVString name, 10)]
  }

spec :: Spec
spec = describe "NameLookup" $ do

  describe "matchesName" $ do
    it "matches exact full name" $ do
      matchesName "Login" "Login" `shouldBe` True

    it "matches dot-aligned trailing suffix" $ do
      matchesName "Login" "Auth.Login" `shouldBe` True

    it "does not match partial substring" $ do
      matchesName "Log" "Login" `shouldBe` False

    it "does not match partial substring (LoginHandler)" $ do
      matchesName "Login" "LoginHandler" `shouldBe` False

    it "matches multi-segment suffix" $ do
      matchesName "Auth.Login" "pkg.Auth.Login" `shouldBe` True

    it "does not match non-dot-aligned prefix" $ do
      matchesName "Auth.Login" "XAuth.Login" `shouldBe` False

    it "matches self as exact" $ do
      matchesName "Auth.Login" "Auth.Login" `shouldBe` True

    it "does not match reversed name" $ do
      matchesName "Auth.Login" "Login.Auth" `shouldBe` False

    it "does not match shorter name" $ do
      matchesName "pkg.Auth.Login" "Auth.Login" `shouldBe` False

    it "matches single segment against multi-segment name" $ do
      matchesName "Config" "App.Config" `shouldBe` True

    it "does not match empty query" $ do
      matchesName "" "Auth.Login" `shouldBe` False

    it "is case-sensitive (lowercase vs uppercase)" $ do
      matchesName "login" "Login" `shouldBe` False

    it "is case-sensitive (uppercase vs lowercase)" $ do
      matchesName "LOGIN" "Login" `shouldBe` False

    it "is case-sensitive for suffix match" $ do
      matchesName "login" "Auth.Login" `shouldBe` False

    it "matches deeply nested dot-aligned suffix" $ do
      matchesName "D" "A.B.C.D" `shouldBe` True

    it "matches intermediate dot-aligned suffix" $ do
      matchesName "C.D" "A.B.C.D" `shouldBe` True

    it "does not match a leading segment" $ do
      matchesName "A" "A.B.C" `shouldBe` False

    it "does not match middle segment alone" $ do
      matchesName "B" "A.B.C" `shouldBe` False

  describe "lookupName" $ do
    it "returns NoMatch when no files" $ do
      lookupName "Foo" [] `shouldBe` NoMatch

    it "returns NoMatch when no specs match" $ do
      let files = [("a.yass.yaml", [mkPreamble, mkSpec "Bar"])]
      lookupName "Foo" files `shouldBe` NoMatch

    it "returns SingleMatch for exact match" $ do
      let docs = [mkPreamble, mkSpec "Login"]
          files = [("a.yass.yaml", docs)]
      lookupName "Login" files `shouldBe` SingleMatch "a.yass.yaml" "Login" docs

    it "returns SingleMatch for suffix match" $ do
      let docs = [mkPreamble, mkSpec "Auth.Login"]
          files = [("a.yass.yaml", docs)]
      lookupName "Login" files `shouldBe` SingleMatch "a.yass.yaml" "Auth.Login" docs

    it "returns MultiMatch when multiple specs match" $ do
      let docs1 = [mkPreamble, mkSpec "Auth.Login"]
          docs2 = [mkPreamble, mkSpec "OAuth.Login"]
          files = [("a.yass.yaml", docs1), ("b.yass.yaml", docs2)]
      lookupName "Login" files `shouldBe`
        MultiMatch [("a.yass.yaml", "Auth.Login"), ("b.yass.yaml", "OAuth.Login")]

    it "returns SingleMatch when only one suffix matches" $ do
      let docs = [mkPreamble, mkSpec "Auth.Login", mkSpec "LoginHandler"]
          files = [("a.yass.yaml", docs)]
      lookupName "Login" files `shouldBe` SingleMatch "a.yass.yaml" "Auth.Login" docs

    it "skips files with no specs (preamble-only)" $ do
      let docs = [mkPreamble]
          files = [("a.yass.yaml", docs)]
      lookupName "Foo" files `shouldBe` NoMatch

    it "handles files with empty docs list" $ do
      let files = [("a.yass.yaml", [])]
      lookupName "Foo" files `shouldBe` NoMatch

    it "returns MultiMatch for same name in different files" $ do
      let docs1 = [mkPreamble, mkSpec "Config"]
          docs2 = [mkPreamble, mkSpec "Config"]
          files = [("a.yass.yaml", docs1), ("b.yass.yaml", docs2)]
      lookupName "Config" files `shouldBe`
        MultiMatch [("a.yass.yaml", "Config"), ("b.yass.yaml", "Config")]

    it "skips spec entries with empty name" $ do
      let docs = [mkPreamble, mkSpec ""]
          files = [("a.yass.yaml", docs)]
      lookupName "" files `shouldBe` NoMatch

    it "skips spec entries with non-string value" $ do
      let badDoc = RawDocument 10 [("spec", RVBool True, 10)]
          docs = [mkPreamble, badDoc]
          files = [("a.yass.yaml", docs)]
      lookupName "True" files `shouldBe` NoMatch

    it "matches across multiple files with mixed results" $ do
      let docs1 = [mkPreamble, mkSpec "Auth.Login", mkSpec "Auth.Logout"]
          docs2 = [mkPreamble, mkSpec "OAuth.Login"]
          files = [("a.yass.yaml", docs1), ("b.yass.yaml", docs2)]
      -- "Login" matches Auth.Login and OAuth.Login
      lookupName "Login" files `shouldBe`
        MultiMatch [("a.yass.yaml", "Auth.Login"), ("b.yass.yaml", "OAuth.Login")]
