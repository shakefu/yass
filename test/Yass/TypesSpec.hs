{-# LANGUAGE OverloadedStrings #-}

module Yass.TypesSpec (spec) where

import Test.Hspec
import qualified Data.Text as T
import qualified Data.Set as Set
import Yass.Types hiding (Spec)

spec :: Spec
spec = do
  describe "errorCodeText" $ do
    it "maps all error codes to yass. namespace strings" $ do
      let allCodes = [minBound .. maxBound] :: [ErrorCode]
      mapM_ (\c -> T.isPrefixOf "yass." (errorCodeText c) `shouldBe` True) allCodes

    it "maps ExitSuccess to yass.exit.success" $
      errorCodeText ExitSuccess `shouldBe` "yass.exit.success"

    it "maps ArgvUnknownSubcommand to yass.argv.unknown_subcommand" $
      errorCodeText ArgvUnknownSubcommand `shouldBe` "yass.argv.unknown_subcommand"

    it "maps PathNotFound to yass.path.not_found" $
      errorCodeText PathNotFound `shouldBe` "yass.path.not_found"

    it "maps YamlNotUtf8 to yass.yaml.not_utf8" $
      errorCodeText YamlNotUtf8 `shouldBe` "yass.yaml.not_utf8"

    it "maps PreambleHasSpecKey to yass.preamble.has_spec_key" $
      errorCodeText PreambleHasSpecKey `shouldBe` "yass.preamble.has_spec_key"

    it "maps SpecNoName to yass.spec.no_name" $
      errorCodeText SpecNoName `shouldBe` "yass.spec.no_name"

    it "maps RefMalformed to yass.ref.malformed" $
      errorCodeText RefMalformed `shouldBe` "yass.ref.malformed"

    it "maps QueryNoMatch to yass.query.no_match" $
      errorCodeText QueryNoMatch `shouldBe` "yass.query.no_match"

    it "maps InternalUncaught to yass.internal.uncaught" $
      errorCodeText InternalUncaught `shouldBe` "yass.internal.uncaught"

    it "uses only valid characters [a-z0-9._] in code strings" $ do
      let allCodes = [minBound .. maxBound] :: [ErrorCode]
          validChars = Set.fromList $ ['a'..'z'] ++ ['0'..'9'] ++ ['.', '_']
      mapM_ (\c ->
        let txt = errorCodeText c
        in all (\ch -> Set.member ch validChars) (T.unpack txt) `shouldBe` True
        ) allCodes

  describe "exitCodeForError" $ do
    it "returns 0 for ExitSuccess" $
      exitCodeForError ExitSuccess `shouldBe` 0

    it "returns 130 for ExitSigint" $
      exitCodeForError ExitSigint `shouldBe` 130

    it "returns 143 for ExitSigterm" $
      exitCodeForError ExitSigterm `shouldBe` 143

    it "returns 2 for argv errors" $ do
      exitCodeForError ArgvUnknownSubcommand `shouldBe` 2
      exitCodeForError ArgvNoSubcommand `shouldBe` 2
      exitCodeForError ArgvUnknownFlag `shouldBe` 2
      exitCodeForError ArgvEmptyArgument `shouldBe` 2
      exitCodeForError ArgvShortFlag `shouldBe` 2
      exitCodeForError ArgvCaseMismatch `shouldBe` 2
      exitCodeForError ArgvAbbreviation `shouldBe` 2
      exitCodeForError ArgvMissingPositional `shouldBe` 2
      exitCodeForError ArgvStdinDash `shouldBe` 2

    it "returns 2 for path errors" $ do
      exitCodeForError PathNotFound `shouldBe` 2
      exitCodeForError PathBadExtension `shouldBe` 2
      exitCodeForError PathUnreadable `shouldBe` 2
      exitCodeForError PathInvalidType `shouldBe` 2
      exitCodeForError PathColonInPath `shouldBe` 2

    it "returns 2 for glob/discover/findroot errors" $ do
      exitCodeForError GlobNoMatch `shouldBe` 2
      exitCodeForError DiscoverNoFiles `shouldBe` 2
      exitCodeForError FindrootNoMarker `shouldBe` 2

    it "returns 2 for query scope/name errors" $ do
      exitCodeForError QueryNameMissing `shouldBe` 2
      exitCodeForError QueryScopeNotFound `shouldBe` 2
      exitCodeForError QueryScopeEmpty `shouldBe` 2

    it "returns 1 for processing errors" $ do
      exitCodeForError YamlNotUtf8 `shouldBe` 1
      exitCodeForError YamlHasBom `shouldBe` 1
      exitCodeForError YamlMalformed `shouldBe` 1
      exitCodeForError PreambleHasSpecKey `shouldBe` 1
      exitCodeForError SpecNoName `shouldBe` 1
      exitCodeForError RefMalformed `shouldBe` 1
      exitCodeForError QueryNoMatch `shouldBe` 1
      exitCodeForError InternalUncaught `shouldBe` 1

    it "only uses exit codes 0, 1, 2, 130, or 143" $ do
      let allCodes = [minBound .. maxBound] :: [ErrorCode]
          validExits = Set.fromList [0, 1, 2, 130, 143] :: Set.Set Int
      mapM_ (\c ->
        Set.member (exitCodeForError c) validExits `shouldBe` True
        ) allCodes

  describe "slotKeywords" $ do
    it "contains INPUT, RETURN, ERROR, SIDE-EFFECT, INVARIANT" $ do
      Set.member "INPUT" slotKeywords `shouldBe` True
      Set.member "RETURN" slotKeywords `shouldBe` True
      Set.member "ERROR" slotKeywords `shouldBe` True
      Set.member "SIDE-EFFECT" slotKeywords `shouldBe` True
      Set.member "INVARIANT" slotKeywords `shouldBe` True

    it "contains exactly 5 keywords" $
      Set.size slotKeywords `shouldBe` 5

  describe "normativityKeywords" $ do
    it "contains MUST, MUST-NOT, SHOULD, SHOULD-NOT, MAY" $ do
      Set.member "MUST" normativityKeywords `shouldBe` True
      Set.member "MUST-NOT" normativityKeywords `shouldBe` True
      Set.member "SHOULD" normativityKeywords `shouldBe` True
      Set.member "SHOULD-NOT" normativityKeywords `shouldBe` True
      Set.member "MAY" normativityKeywords `shouldBe` True

    it "contains exactly 5 keywords" $
      Set.size normativityKeywords `shouldBe` 5

  describe "isReservedKeyword" $ do
    it "matches case-insensitively" $ do
      isReservedKeyword "input" `shouldBe` True
      isReservedKeyword "Input" `shouldBe` True
      isReservedKeyword "INPUT" `shouldBe` True
      isReservedKeyword "must" `shouldBe` True
      isReservedKeyword "Must" `shouldBe` True
      isReservedKeyword "must-not" `shouldBe` True
      isReservedKeyword "MUST-NOT" `shouldBe` True

    it "rejects non-keywords" $ do
      isReservedKeyword "HELLO" `shouldBe` False
      isReservedKeyword "spec" `shouldBe` False
      isReservedKeyword "" `shouldBe` False

  describe "isRelationKeyword" $ do
    it "recognizes CONFORMS, USES, SEE" $ do
      isRelationKeyword "CONFORMS" `shouldBe` True
      isRelationKeyword "USES" `shouldBe` True
      isRelationKeyword "SEE" `shouldBe` True

    it "rejects other strings" $ do
      isRelationKeyword "conforms" `shouldBe` False
      isRelationKeyword "WHEN" `shouldBe` False

  describe "isGuardKeyword" $ do
    it "recognizes WHEN" $
      isGuardKeyword "WHEN" `shouldBe` True

    it "rejects non-WHEN" $ do
      isGuardKeyword "when" `shouldBe` False
      isGuardKeyword "IF" `shouldBe` False
