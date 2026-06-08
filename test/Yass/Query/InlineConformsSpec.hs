{-# LANGUAGE OverloadedStrings #-}

module Yass.Query.InlineConformsSpec (spec) where

import Data.Either (isLeft, isRight)
import qualified Data.ByteString as BS
import System.Directory (createDirectoryIfMissing)
import System.FilePath ((</>))
import System.IO.Temp (withSystemTempDirectory)
import Test.Hspec
import Yass.Parser (RawDocument(..), RawValue(..))
import Yass.Query.InlineConforms
import Yass.Types (ErrorCode(..))

mkPreamble :: RawDocument
mkPreamble = RawDocument 1
  [("description", RVString "test", 1), ("version", RVString "v1", 2)]

mkSimpleSpec :: RawDocument
mkSimpleSpec = RawDocument 10
  [ ("spec", RVString "MySpec", 10)
  , ("INPUT", RVList [RVMapping [("MUST", RVString "be valid", 12)] 12] 11, 11)
  ]

mkConformsRefOnly :: RawDocument
mkConformsRefOnly = RawDocument 10
  [ ("spec", RVString "MySpec", 10)
  , ("INPUT", RVList
      [RVMapping [("CONFORMS", RVString "other@Target::INPUT", 12)] 12] 11, 11)
  ]

mkConformsWithNorm :: RawDocument
mkConformsWithNorm = RawDocument 10
  [ ("spec", RVString "MySpec", 10)
  , ("INPUT", RVList
      [RVMapping [("MUST", RVString "accept input", 12)
                 ,("CONFORMS", RVString "other@Target::INPUT", 13)] 12] 11, 11)
  ]

mkConformsNoSlot :: RawDocument
mkConformsNoSlot = RawDocument 10
  [ ("spec", RVString "MySpec", 10)
  , ("INPUT", RVList
      [RVMapping [("CONFORMS", RVString "Target", 12)] 12] 11, 11)
  ]

mkParentSpec :: RawDocument
mkParentSpec = RawDocument 30
  [ ("spec", RVString "Parent", 30)
  , ("INPUT", RVList [RVMapping [("MUST", RVString "have id", 32)] 32] 31, 31)
  ]

mkConformsSameFile :: RawDocument
mkConformsSameFile = RawDocument 20
  [ ("spec", RVString "Child", 20)
  , ("INPUT", RVList
      [RVMapping [("CONFORMS", RVString "Parent::INPUT", 22)] 22] 21, 21)
  ]

writeTargetFile :: FilePath -> IO ()
writeTargetFile dir =
  BS.writeFile (dir </> "other.yass.yaml") $ encU8
    "description: target\nversion: v1\n---\nspec: Target\nINPUT:\n  - MUST: have id\n  - SHOULD: have name\n"

encU8 :: String -> BS.ByteString
encU8 = BS.pack . map (fromIntegral . fromEnum)

inlineDoc :: FilePath -> FilePath -> RawDocument -> IO (Either [InlineError] RawDocument)
inlineDoc pr fd docVal = inlineConforms pr fd [mkPreamble, docVal] docVal

spec :: Spec
spec = describe "InlineConforms" $ do
  describe "parseConformsTarget" $ do
    it "rejects target without ::SLOT suffix" $
      withSystemTempDirectory "ic" $ \d -> do
        r <- inlineDoc d d mkConformsNoSlot
        r `shouldSatisfy` isLeft
        case r of
          Left errs -> do { length errs `shouldBe` 1; ieCode (head errs) `shouldBe` QueryConformsNoSlot }
          Right _ -> expectationFailure "expected Left"

    it "rejects bare name without :: separator" $
      withSystemTempDirectory "ic" $ \d -> do
        let docVal = RawDocument 10
              [("spec", RVString "X", 10)
              ,("INPUT", RVList [RVMapping [("CONFORMS", RVString "JustAName", 12)] 12] 11, 11)]
        r <- inlineDoc d d docVal
        r `shouldSatisfy` isLeft

    it "parses path@Spec::SLOT correctly" $ do
      let r = parseConformsTarget "some/path@MySpec::INPUT"
      r `shouldBe` Right (Just "some/path", "MySpec", "INPUT")

    it "parses Spec::SLOT (no path) correctly" $ do
      let r = parseConformsTarget "MySpec::INPUT"
      r `shouldBe` Right (Nothing, "MySpec", "INPUT")

    it "parses nested path@Spec::SLOT correctly" $ do
      let r = parseConformsTarget "dir/sub@Auth.Login::RETURN"
      r `shouldBe` Right (Just "dir/sub", "Auth.Login", "RETURN")

    it "returns error for target with no :: separator" $ do
      let r = parseConformsTarget "NoSlotHere"
      r `shouldSatisfy` isLeft
      case r of
        Left err -> ieCode err `shouldBe` QueryConformsNoSlot
        Right _ -> expectationFailure "expected Left"

  describe "collapseDots" $ do
    it "normalizes a simple relative path" $ do
      collapseDots "a/b/c" `shouldBe` "a/b/c"

    it "collapses single dot in path" $ do
      -- The "." component should be removed
      let result = collapseDots "a/./b"
      -- After normalise + collapse, "." is removed
      result `shouldBe` "a/b"

    it "collapses parent (..) in path" $ do
      let result = collapseDots "a/b/../c"
      result `shouldBe` "a/c"

    it "handles leading .. gracefully" $ do
      -- Leading ".." with nothing to pop should just be dropped
      let result = collapseDots "../a"
      result `shouldBe` "a"

  describe "applyGuardCombination" $ do
    it "passes value through when no outer guard" $ do
      let val = RVMapping [("MUST", RVString "x", 1)] 1
      applyGuardCombination Nothing val `shouldBe` val

    it "injects WHEN when obligation has no inner WHEN" $ do
      let val = RVMapping [("MUST", RVString "x", 1)] 1
          result = applyGuardCombination (Just "user is admin") val
      case result of
        RVMapping es _ -> do
          let whens = [(k, v) | (k, v, _) <- es, k == "WHEN"]
          length whens `shouldBe` 1
          case whens of
            [(_, RVString g)] -> g `shouldBe` "user is admin"
            _ -> expectationFailure "expected WHEN entry"
        _ -> expectationFailure "expected RVMapping"

    it "combines outer + inner WHEN guards" $ do
      let val = RVMapping [("MUST", RVString "x", 1), ("WHEN", RVString "input valid", 2)] 1
          result = applyGuardCombination (Just "user is admin") val
      case result of
        RVMapping es _ -> do
          let whens = [v | ("WHEN", v, _) <- es]
          length whens `shouldBe` 1
          case whens of
            [RVString g] -> g `shouldBe` "user is admin and input valid"
            _ -> expectationFailure "expected combined WHEN string"
        _ -> expectationFailure "expected RVMapping"

    it "passes non-mapping through unchanged with outer guard" $ do
      let val = RVString "plain"
      applyGuardCombination (Just "some guard") val `shouldBe` val

  describe "inlineConforms" $ do
    it "passes through obligations without CONFORMS" $
      withSystemTempDirectory "ic" $ \d -> do
        r <- inlineDoc d d mkSimpleSpec
        r `shouldSatisfy` isRight
        case r of
          Right docVal -> rawDocEntries docVal `shouldBe` rawDocEntries mkSimpleSpec
          Left _ -> expectationFailure "expected Right"

    it "errors on CONFORMS without ::SLOT" $
      withSystemTempDirectory "ic" $ \d -> do
        r <- inlineDoc d d mkConformsNoSlot
        r `shouldSatisfy` isLeft

    it "inlines reference-only CONFORMS from cross-file target" $
      withSystemTempDirectory "ic" $ \d -> do
        writeTargetFile d
        createDirectoryIfMissing False (d </> ".git")
        r <- inlineDoc d d mkConformsRefOnly
        r `shouldSatisfy` isRight
        case r of
          Right docVal -> do
            let ie = filter (\(k,_,_) -> k == "INPUT") (rawDocEntries docVal)
            case ie of
              [(_,RVList items _,_)] -> do
                -- 1 provenance comment + 2 inlined obligations
                length items `shouldBe` 3
                case head items of
                  RVComment txt -> txt `shouldBe` "CONFORMS: other@Target::INPUT"
                  _ -> expectationFailure "expected provenance comment"
              _ -> expectationFailure "expected INPUT list"
          Left es -> expectationFailure $ show es

    it "keeps carrier when CONFORMS has normativity" $
      withSystemTempDirectory "ic" $ \d -> do
        writeTargetFile d
        createDirectoryIfMissing False (d </> ".git")
        r <- inlineDoc d d mkConformsWithNorm
        r `shouldSatisfy` isRight
        case r of
          Right docVal -> do
            let ie = filter (\(k,_,_) -> k == "INPUT") (rawDocEntries docVal)
            case ie of
              [(_,RVList items _,_)] -> do
                -- 1 carrier + 1 provenance comment + 2 inlined obligations
                length items `shouldBe` 4
                case head items of
                  RVMapping es _ -> do
                    any (\(k,_,_) -> k == "CONFORMS") es `shouldBe` False
                    any (\(k,_,_) -> k == "MUST") es `shouldBe` True
                  _ -> expectationFailure "expected mapping"
                case items !! 1 of
                  RVComment txt -> txt `shouldBe` "CONFORMS: other@Target::INPUT"
                  _ -> expectationFailure "expected provenance comment"
              _ -> expectationFailure "expected INPUT list"
          Left es -> expectationFailure $ show es

    it "errors when referenced file does not exist" $
      withSystemTempDirectory "ic" $ \d -> do
        createDirectoryIfMissing False (d </> ".git")
        r <- inlineDoc d d mkConformsRefOnly
        r `shouldSatisfy` isLeft
        case r of
          Left errs -> do { length errs `shouldBe` 1; ieCode (head errs) `shouldBe` QueryConformsUnresolved }
          Right _ -> expectationFailure "expected Left"

    it "errors when referenced spec not found" $
      withSystemTempDirectory "ic" $ \d -> do
        BS.writeFile (d </> "other.yass.yaml") $ encU8 "description: t\nversion: v1\n---\nspec: Other\nINPUT:\n  - MUST: x\n"
        createDirectoryIfMissing False (d </> ".git")
        r <- inlineDoc d d mkConformsRefOnly
        r `shouldSatisfy` isLeft

    it "errors when slot not found" $
      withSystemTempDirectory "ic" $ \d -> do
        BS.writeFile (d </> "other.yass.yaml") $ encU8 "description: t\nversion: v1\n---\nspec: Target\nRETURN:\n  - MUST: r\n"
        createDirectoryIfMissing False (d </> ".git")
        r <- inlineDoc d d mkConformsRefOnly
        r `shouldSatisfy` isLeft

    it "handles spec with no slot entries" $
      withSystemTempDirectory "ic" $ \d -> do
        r <- inlineDoc d d (RawDocument 10 [("spec", RVString "Simple", 10)])
        r `shouldSatisfy` isRight

    it "handles spec with non-list slot value" $
      withSystemTempDirectory "ic" $ \d -> do
        r <- inlineDoc d d (RawDocument 10 [("spec", RVString "Bad", 10), ("INPUT", RVString "not a list", 11)])
        r `shouldSatisfy` isRight

    it "passes through non-mapping obligations" $
      withSystemTempDirectory "ic" $ \d -> do
        let docVal = RawDocument 10 [("spec", RVString "S", 10), ("INPUT", RVList [RVString "plain"] 11, 11)]
        r <- inlineDoc d d docVal
        r `shouldSatisfy` isRight

    it "inlines same-file CONFORMS reference" $
      withSystemTempDirectory "ic" $ \d -> do
        let allDocs = [mkPreamble, mkParentSpec, mkConformsSameFile]
        r <- inlineConforms d d allDocs mkConformsSameFile
        r `shouldSatisfy` isRight
        case r of
          Right docVal -> do
            let ie = filter (\(k,_,_) -> k == "INPUT") (rawDocEntries docVal)
            case ie of
              [(_,RVList items _,_)] -> do
                -- 1 provenance comment + 1 inlined obligation
                length items `shouldBe` 2
                case head items of
                  RVComment txt -> txt `shouldBe` "CONFORMS: Parent::INPUT"
                  _ -> expectationFailure "expected provenance comment"
              _ -> expectationFailure "expected INPUT list"
          Left es -> expectationFailure $ show es

    it "errors for same-file ref when spec not found" $
      withSystemTempDirectory "ic" $ \d -> do
        let badRef = RawDocument 20
              [ ("spec", RVString "Child", 20)
              , ("INPUT", RVList
                  [RVMapping [("CONFORMS", RVString "Missing::INPUT", 22)] 22] 21, 21)
              ]
            allDocs = [mkPreamble, badRef]
        r <- inlineConforms d d allDocs badRef
        r `shouldSatisfy` isLeft
        case r of
          Left errs -> ieCode (head errs) `shouldBe` QueryConformsUnresolved
          Right _ -> expectationFailure "expected Left"

    it "errors for same-file ref when slot not found" $
      withSystemTempDirectory "ic" $ \d -> do
        let parentNoInput = RawDocument 30
              [ ("spec", RVString "Parent", 30)
              , ("RETURN", RVList [RVMapping [("MUST", RVString "r", 32)] 32] 31, 31)
              ]
            childRef = RawDocument 20
              [ ("spec", RVString "Child", 20)
              , ("INPUT", RVList
                  [RVMapping [("CONFORMS", RVString "Parent::INPUT", 22)] 22] 21, 21)
              ]
            allDocs = [mkPreamble, parentNoInput, childRef]
        r <- inlineConforms d d allDocs childRef
        r `shouldSatisfy` isLeft

    it "inserts RVComment for provenance on reference-only CONFORMS" $
      withSystemTempDirectory "ic" $ \d -> do
        let allDocs = [mkPreamble, mkParentSpec, mkConformsSameFile]
        r <- inlineConforms d d allDocs mkConformsSameFile
        case r of
          Right docVal -> do
            let ie = filter (\(k,_,_) -> k == "INPUT") (rawDocEntries docVal)
            case ie of
              [(_,RVList items _,_)] -> do
                -- First item should be provenance comment
                case head items of
                  RVComment txt -> txt `shouldBe` "CONFORMS: Parent::INPUT"
                  other -> expectationFailure $ "expected RVComment, got: " ++ show other
              _ -> expectationFailure "expected INPUT list"
          Left es -> expectationFailure $ show es

    it "preserves USES refs on carrier-less CONFORMS" $
      withSystemTempDirectory "ic" $ \d -> do
        -- Ref-only CONFORMS with a USES ref on the carrier
        let parentWithUses = RawDocument 30
              [ ("spec", RVString "Parent", 30)
              , ("INPUT", RVList [RVMapping [("MUST", RVString "have id", 32)] 32] 31, 31)
              ]
            childWithUses = RawDocument 20
              [ ("spec", RVString "Child", 20)
              , ("INPUT", RVList
                  [RVMapping [("CONFORMS", RVString "Parent::INPUT", 22)
                             ,("USES", RVString "some-lib", 23)] 22] 21, 21)
              ]
            allDocs = [mkPreamble, parentWithUses, childWithUses]
        r <- inlineConforms d d allDocs childWithUses
        r `shouldSatisfy` isRight
        case r of
          Right docVal -> do
            let ie = filter (\(k,_,_) -> k == "INPUT") (rawDocEntries docVal)
            case ie of
              [(_,RVList items _,_)] -> do
                -- provenance comment + inlined obligation (with USES appended)
                length items `shouldBe` 2
                -- The first inlined obligation should have USES attached
                case items !! 1 of
                  RVMapping es _ ->
                    any (\(k,_,_) -> k == "USES") es `shouldBe` True
                  _ -> expectationFailure "expected mapping with USES"
              _ -> expectationFailure "expected INPUT list"
          Left es -> expectationFailure $ show es

    it "handles guard injection with cross-file CONFORMS" $
      withSystemTempDirectory "ic" $ \d -> do
        -- Write a target file with no WHEN guards
        BS.writeFile (d </> "other.yass.yaml") $ encU8
          "description: t\nversion: v1\n---\nspec: Target\nINPUT:\n  - MUST: have id\n"
        createDirectoryIfMissing False (d </> ".git")
        let docVal = RawDocument 10
              [ ("spec", RVString "MySpec", 10)
              , ("INPUT", RVList
                  [RVMapping [("CONFORMS", RVString "other@Target::INPUT", 12)
                             ,("WHEN", RVString "user logged in", 13)] 12] 11, 11)
              ]
        r <- inlineDoc d d docVal
        r `shouldSatisfy` isRight
        case r of
          Right doc -> do
            let ie = filter (\(k,_,_) -> k == "INPUT") (rawDocEntries doc)
            case ie of
              [(_,RVList items _,_)] -> do
                -- The inlined obligation should have WHEN injected
                let mappings = [es | RVMapping es _ <- items]
                case mappings of
                  (es:_) ->
                    any (\(k,_,_) -> k == "WHEN") es `shouldBe` True
                  _ -> expectationFailure "expected at least one mapping"
              _ -> expectationFailure "expected INPUT list"
          Left es -> expectationFailure $ show es

    it "handles guard combination with cross-file CONFORMS" $
      withSystemTempDirectory "ic" $ \d -> do
        -- Write a target file WITH WHEN guards on obligations
        BS.writeFile (d </> "other.yass.yaml") $ encU8
          "description: t\nversion: v1\n---\nspec: Target\nINPUT:\n  - MUST: have id\n    WHEN: data present\n"
        createDirectoryIfMissing False (d </> ".git")
        let docVal = RawDocument 10
              [ ("spec", RVString "MySpec", 10)
              , ("INPUT", RVList
                  [RVMapping [("CONFORMS", RVString "other@Target::INPUT", 12)
                             ,("WHEN", RVString "user logged in", 13)] 12] 11, 11)
              ]
        r <- inlineDoc d d docVal
        r `shouldSatisfy` isRight
        case r of
          Right doc -> do
            let ie = filter (\(k,_,_) -> k == "INPUT") (rawDocEntries doc)
            case ie of
              [(_,RVList items _,_)] -> do
                let mappings = [es | RVMapping es _ <- items]
                case mappings of
                  (es:_) -> do
                    let whens = [v | ("WHEN", v, _) <- es]
                    case whens of
                      [RVString g] -> g `shouldBe` "user logged in and data present"
                      _ -> expectationFailure "expected combined WHEN"
                  _ -> expectationFailure "expected at least one mapping"
              _ -> expectationFailure "expected INPUT list"
          Left es -> expectationFailure $ show es

    it "InlineError Show and Eq instances work" $ do
      let e1 = InlineError QueryConformsNoSlot "t" "msg"
          e2 = InlineError QueryConformsNoSlot "t" "msg"
          e3 = InlineError QueryConformsUnresolved "t" "msg"
      e1 `shouldBe` e2
      e1 `shouldNotBe` e3
      show e1 `shouldNotBe` ""
      ieTarget e1 `shouldBe` "t"
      ieMessage e1 `shouldBe` "msg"
