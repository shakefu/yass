{-# LANGUAGE OverloadedStrings #-}

module Yass.Validate.CheckUniquenessSpec (spec) where

import Test.Hspec
import Data.Text (Text)
import qualified Data.Text as T
import Yass.Validate.CheckUniqueness (checkUniqueness)
import Yass.Types (YassError(..), ErrorCode(..))
import Yass.Parser (RawDocument(..), RawValue(..))

-- | Helper: make a preamble document (no "spec" key)
mkPreamble :: Int -> RawDocument
mkPreamble line = RawDocument line
  [ ("description", RVString "A test file", line)
  , ("version", RVString "v1", line + 1)
  ]

-- | Helper: make a spec document with just a spec key
mkSpecDoc :: Text -> Int -> RawDocument
mkSpecDoc name line = RawDocument line
  [ ("spec", RVString name, line)
  ]

spec :: Spec
spec = describe "Yass.Validate.CheckUniqueness" $ do

  let cwd = "/project"
      fp  = "/project/test.yass.yaml"

  describe "no duplicates" $ do
    it "returns no errors for unique spec names" $ do
      let docs = [ mkPreamble 1
                  , mkSpecDoc "Alpha" 4
                  , mkSpecDoc "Beta" 8
                  , mkSpecDoc "Gamma" 12
                  ]
      checkUniqueness cwd fp docs `shouldBe` []

    it "returns no errors for a single spec" $ do
      let docs = [ mkPreamble 1
                  , mkSpecDoc "OnlySpec" 4
                  ]
      checkUniqueness cwd fp docs `shouldBe` []

    it "returns no errors for no spec documents" $ do
      let docs = [ mkPreamble 1 ]
      checkUniqueness cwd fp docs `shouldBe` []

    it "returns no errors for empty document list" $ do
      checkUniqueness cwd fp [] `shouldBe` []

  describe "duplicate detection" $ do
    it "returns one error for one duplicate" $ do
      let docs = [ mkPreamble 1
                  , mkSpecDoc "Alpha" 4
                  , mkSpecDoc "Alpha" 8
                  ]
          errs = checkUniqueness cwd fp docs
      length errs `shouldBe` 1
      yeCode (head errs) `shouldBe` SpecDuplicateName
      yeLine (head errs) `shouldBe` Just 8
      yeFile (head errs) `shouldBe` Just fp

    it "returns two errors for three occurrences of same name" $ do
      let docs = [ mkPreamble 1
                  , mkSpecDoc "Alpha" 4
                  , mkSpecDoc "Alpha" 8
                  , mkSpecDoc "Alpha" 12
                  ]
          errs = checkUniqueness cwd fp docs
      length errs `shouldBe` 2
      map yeLine errs `shouldBe` [Just 8, Just 12]
      all (\e -> yeCode e == SpecDuplicateName) errs `shouldBe` True

    it "reports errors for each duplicate name independently" $ do
      let docs = [ mkPreamble 1
                  , mkSpecDoc "Alpha" 4
                  , mkSpecDoc "Beta" 8
                  , mkSpecDoc "Alpha" 12
                  , mkSpecDoc "Beta" 16
                  ]
          errs = checkUniqueness cwd fp docs
      length errs `shouldBe` 2
      all (\e -> yeCode e == SpecDuplicateName) errs `shouldBe` True

    it "errors point at subsequent occurrences, not the first" $ do
      let docs = [ mkPreamble 1
                  , mkSpecDoc "Alpha" 4
                  , mkSpecDoc "Alpha" 8
                  ]
          errs = checkUniqueness cwd fp docs
      length errs `shouldBe` 1
      -- The error should be at line 8 (the second occurrence), not line 4
      yeLine (head errs) `shouldBe` Just 8

  describe "edge cases" $ do
    it "skips preamble document (first doc)" $ do
      -- Even if preamble happens to have a "spec" key, checkUniqueness
      -- skips the first doc (preamble)
      let preambleWithSpec = RawDocument 1
            [ ("spec", RVString "Alpha", 1)
            , ("description", RVString "test", 2)
            ]
          docs = [ preambleWithSpec
                  , mkSpecDoc "Alpha" 5
                  ]
      -- No error: preamble is skipped, only one spec "Alpha" seen
      checkUniqueness cwd fp docs `shouldBe` []

    it "skips documents without valid string spec names" $ do
      let badDoc = RawDocument 8 [("spec", RVNull, 8)]
          docs = [ mkPreamble 1
                  , mkSpecDoc "Alpha" 4
                  , badDoc
                  , mkSpecDoc "Alpha" 12
                  ]
          errs = checkUniqueness cwd fp docs
      -- Only two "Alpha" specs seen (the badDoc is ignored)
      length errs `shouldBe` 1
      yeLine (head errs) `shouldBe` Just 12

    it "names are case-sensitive for uniqueness" $ do
      let docs = [ mkPreamble 1
                  , mkSpecDoc "Alpha" 4
                  , mkSpecDoc "alpha" 8
                  , mkSpecDoc "ALPHA" 12
                  ]
      -- All are distinct, no errors
      checkUniqueness cwd fp docs `shouldBe` []

    it "error message includes the duplicate spec name" $ do
      let docs = [ mkPreamble 1
                  , mkSpecDoc "MySpec" 4
                  , mkSpecDoc "MySpec" 8
                  ]
          errs = checkUniqueness cwd fp docs
      length errs `shouldBe` 1
      T.isInfixOf "MySpec" (yeMessage (head errs)) `shouldBe` True

    it "handles dotted spec names" $ do
      let docs = [ mkPreamble 1
                  , mkSpecDoc "auth.login" 4
                  , mkSpecDoc "auth.login" 8
                  ]
          errs = checkUniqueness cwd fp docs
      length errs `shouldBe` 1
      yeLine (head errs) `shouldBe` Just 8

    it "ignores docs with empty spec names" $ do
      let emptyDoc = RawDocument 8 [("spec", RVString "", 8)]
          docs = [ mkPreamble 1
                  , mkSpecDoc "Alpha" 4
                  , emptyDoc
                  ]
      checkUniqueness cwd fp docs `shouldBe` []
