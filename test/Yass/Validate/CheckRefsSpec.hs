{-# LANGUAGE OverloadedStrings #-}

module Yass.Validate.CheckRefsSpec (spec) where

import Test.Hspec
import Data.Text (Text)
import qualified Data.Text as T
import qualified Data.Text.Encoding as TE
import qualified Data.ByteString as BS
import System.IO.Temp (withSystemTempDirectory)
import System.FilePath ((</>))
import System.Directory (createDirectoryIfMissing)
import Yass.Validate.CheckRefs (checkRefs)
import Yass.Types (YassError(..), ErrorCode(..))
import Yass.Parser (RawDocument(..), RawValue(..))

-- | Helper: make a preamble document
mkPreamble :: Int -> RawDocument
mkPreamble line = RawDocument line
  [ ("description", RVString "A test file", line)
  , ("version", RVString "v1", line + 1)
  ]

-- | Helper: make a spec document with a CONFORMS ref in an INPUT slot
mkSpecWithRef :: Text -> Int -> Text -> Int -> RawDocument
mkSpecWithRef name line refText refLine = RawDocument line
  [ ("spec", RVString name, line)
  , ("INPUT", RVList
      [ RVMapping
          [ ("CONFORMS", RVString refText, refLine) ]
          refLine
      ] (line + 1), line + 1)
  ]

-- | Helper: make a spec document with multiple refs
mkSpecWithRefs :: Text -> Int -> [(Text, Text, Int)] -> RawDocument
mkSpecWithRefs name line refs = RawDocument line
  [ ("spec", RVString name, line)
  , ("INPUT", RVList
      (map (\(rel, target, rl) -> RVMapping [(rel, RVString target, rl)] rl) refs)
      (line + 1), line + 1)
  ]

-- | Helper: make a simple spec doc (no refs)
mkSpecDoc :: Text -> Int -> RawDocument
mkSpecDoc name line = RawDocument line
  [ ("spec", RVString name, line)
  ]

-- | Helper: make a spec doc with a specific slot
mkSpecDocWithSlot :: Text -> Int -> Text -> RawDocument
mkSpecDocWithSlot name line slotName = RawDocument line
  [ ("spec", RVString name, line)
  , (slotName, RVList [RVString "MUST do something"] (line + 1), line + 1)
  ]

-- | Helper: write a yass file
writeYassFile :: FilePath -> Text -> IO ()
writeYassFile fp content =
  BS.writeFile fp (TE.encodeUtf8 content)

spec :: Spec
spec = describe "Yass.Validate.CheckRefs" $ do

  describe "malformed refs" $ do
    it "reports malformed ref with empty string" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
            docs = [ mkPreamble 1
                    , mkSpecWithRef "Alpha" 4 "" 6
                    ]
        errs <- checkRefs dir fp dir docs
        length errs `shouldBe` 1
        yeCode (head errs) `shouldBe` RefMalformed

    it "reports malformed ref with invalid characters" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
            docs = [ mkPreamble 1
                    , mkSpecWithRef "Alpha" 4 "Spec Name With Spaces" 6
                    ]
        errs <- checkRefs dir fp dir docs
        length errs `shouldBe` 1
        yeCode (head errs) `shouldBe` RefMalformed

    it "reports malformed ref with trailing @" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
            docs = [ mkPreamble 1
                    , mkSpecWithRef "Alpha" 4 "path@" 6
                    ]
        errs <- checkRefs dir fp dir docs
        length errs `shouldBe` 1
        yeCode (head errs) `shouldBe` RefMalformed

  describe "unknown slot" $ do
    it "reports unknown slot in ref target" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
            docs = [ mkPreamble 1
                    , mkSpecDoc "Target" 4
                    , mkSpecWithRef "Alpha" 8 "Target::BADSLOT" 10
                    ]
        errs <- checkRefs dir fp dir docs
        length errs `shouldBe` 1
        yeCode (head errs) `shouldBe` RefUnknownSlot

    it "accepts valid slot names (INPUT, RETURN, ERROR, SIDE-EFFECT, INVARIANT)" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
            docs = [ mkPreamble 1
                    , mkSpecDocWithSlot "Target" 4 "INPUT"
                    , mkSpecWithRef "Alpha" 8 "Target::INPUT" 10
                    ]
        errs <- checkRefs dir fp dir docs
        errs `shouldBe` []

  describe "same-file spec not found" $ do
    it "reports spec not found for same-file ref" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
            docs = [ mkPreamble 1
                    , mkSpecWithRef "Alpha" 4 "NonExistent" 6
                    ]
        errs <- checkRefs dir fp dir docs
        length errs `shouldBe` 1
        yeCode (head errs) `shouldBe` RefSpecNotFoundSameFile

    it "resolves same-file ref successfully" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
            docs = [ mkPreamble 1
                    , mkSpecDoc "Target" 4
                    , mkSpecWithRef "Alpha" 8 "Target" 10
                    ]
        errs <- checkRefs dir fp dir docs
        errs `shouldBe` []

  describe "cross-file refs" $ do
    it "reports file not found for cross-file ref" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
            docs = [ mkPreamble 1
                    , mkSpecWithRef "Alpha" 4 "nonexistent@Target" 6
                    ]
        errs <- checkRefs dir fp dir docs
        length errs `shouldBe` 1
        yeCode (head errs) `shouldBe` RefFileNotFound

    it "reports file not parseable for invalid cross-file ref" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
            targetFp = dir </> "other.yass.yaml"
        -- Write an invalid YAML file
        writeYassFile targetFp "key: [\n"
        let docs = [ mkPreamble 1
                    , mkSpecWithRef "Alpha" 4 "other@Target" 6
                    ]
        errs <- checkRefs dir fp dir docs
        length errs `shouldBe` 1
        yeCode (head errs) `shouldBe` RefFileNotParseable

    it "reports spec not found in other file" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
            targetFp = dir </> "other.yass.yaml"
        writeYassFile targetFp $ T.unlines
          [ "description: Other file"
          , "version: v1"
          , "---"
          , "spec: WrongSpec"
          ]
        let docs = [ mkPreamble 1
                    , mkSpecWithRef "Alpha" 4 "other@Target" 6
                    ]
        errs <- checkRefs dir fp dir docs
        length errs `shouldBe` 1
        yeCode (head errs) `shouldBe` RefSpecNotFoundOtherFile

    it "resolves cross-file ref successfully" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
            targetFp = dir </> "other.yass.yaml"
        writeYassFile targetFp $ T.unlines
          [ "description: Other file"
          , "version: v1"
          , "---"
          , "spec: Target"
          ]
        let docs = [ mkPreamble 1
                    , mkSpecWithRef "Alpha" 4 "other@Target" 6
                    ]
        errs <- checkRefs dir fp dir docs
        errs `shouldBe` []

  describe "slot not declared" $ do
    it "reports slot not declared for same-file ref with slot" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
            -- Target has INPUT but not RETURN
            docs = [ mkPreamble 1
                    , mkSpecDocWithSlot "Target" 4 "INPUT"
                    , mkSpecWithRef "Alpha" 8 "Target::RETURN" 10
                    ]
        errs <- checkRefs dir fp dir docs
        length errs `shouldBe` 1
        yeCode (head errs) `shouldBe` RefSlotNotDeclared

    it "succeeds when slot is declared in same-file ref" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
            docs = [ mkPreamble 1
                    , mkSpecDocWithSlot "Target" 4 "INPUT"
                    , mkSpecWithRef "Alpha" 8 "Target::INPUT" 10
                    ]
        errs <- checkRefs dir fp dir docs
        errs `shouldBe` []

  describe "deduplication" $ do
    it "reports file_not_found at most once per target file" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
            docs = [ mkPreamble 1
                    , mkSpecWithRefs "Alpha" 4
                        [ ("CONFORMS", "missing@Spec1", 6)
                        , ("USES", "missing@Spec2", 7)
                        ]
                    ]
        errs <- checkRefs dir fp dir docs
        -- Only one file_not_found error despite two refs to the same missing file
        let fileNotFoundErrs = filter (\e -> yeCode e == RefFileNotFound) errs
        length fileNotFoundErrs `shouldBe` 1

  describe "relative path resolution" $ do
    it "resolves relative path with ./ prefix" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let subdir = dir </> "sub"
        createDirectoryIfMissing True subdir
        let fp = subdir </> "test.yass.yaml"
            targetFp = subdir </> "other.yass.yaml"
        writeYassFile targetFp $ T.unlines
          [ "description: Other file"
          , "version: v1"
          , "---"
          , "spec: Target"
          ]
        let docs = [ mkPreamble 1
                    , mkSpecWithRef "Alpha" 4 "./other@Target" 6
                    ]
        errs <- checkRefs dir fp dir docs
        errs `shouldBe` []

  describe "no refs" $ do
    it "returns no errors when there are no refs" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
            docs = [ mkPreamble 1
                    , mkSpecDoc "Alpha" 4
                    ]
        errs <- checkRefs dir fp dir docs
        errs `shouldBe` []

    it "returns no errors for empty doc list" $ do
      withSystemTempDirectory "yass-refs-test" $ \dir -> do
        let fp = dir </> "test.yass.yaml"
        errs <- checkRefs dir fp dir []
        errs `shouldBe` []
