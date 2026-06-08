{-# LANGUAGE OverloadedStrings #-}

module Yass.Query.OutputProfileSpec (spec) where

import qualified Data.Text as T
import Test.Hspec
import Yass.Parser (RawDocument(..), RawValue(..))
import Yass.Query.OutputProfile

spec :: Spec
spec = describe "OutputProfile" $ do

  -- ── needsQuoting ─────────────────────────────────────────────────────

  describe "needsQuoting" $ do
    it "quotes empty string" $ do
      needsQuoting "" `shouldBe` True

    it "does not quote plain text" $ do
      needsQuoting "hello world" `shouldBe` False

    it "quotes strings containing colon-space" $ do
      needsQuoting "key: value" `shouldBe` True

    it "quotes strings starting with special chars" $ do
      needsQuoting "?query" `shouldBe` True
      needsQuoting "*glob" `shouldBe` True
      needsQuoting "&anchor" `shouldBe` True
      needsQuoting "!tag" `shouldBe` True
      needsQuoting "|block" `shouldBe` True
      needsQuoting ">fold" `shouldBe` True
      needsQuoting "%directive" `shouldBe` True
      needsQuoting "@mention" `shouldBe` True

    it "quotes strings starting with dash" $ do
      needsQuoting "-dash" `shouldBe` True

    it "quotes strings with leading space" $ do
      needsQuoting " leading" `shouldBe` True

    it "quotes strings with trailing space" $ do
      needsQuoting "trailing " `shouldBe` True

    it "quotes YAML type token: true" $ do
      needsQuoting "true" `shouldBe` True

    it "quotes YAML type token: false" $ do
      needsQuoting "false" `shouldBe` True

    it "quotes YAML type token: null" $ do
      needsQuoting "null" `shouldBe` True

    it "quotes YAML type token: yes" $ do
      needsQuoting "yes" `shouldBe` True

    it "quotes YAML type token: no" $ do
      needsQuoting "no" `shouldBe` True

    it "quotes YAML type token: on" $ do
      needsQuoting "on" `shouldBe` True

    it "quotes YAML type token: off" $ do
      needsQuoting "off" `shouldBe` True

    it "quotes tilde (null alias)" $ do
      needsQuoting "~" `shouldBe` True

    it "quotes case-insensitive type tokens" $ do
      needsQuoting "True" `shouldBe` True
      needsQuoting "FALSE" `shouldBe` True
      needsQuoting "Null" `shouldBe` True
      needsQuoting "YES" `shouldBe` True

    it "quotes numeric strings" $ do
      needsQuoting "42" `shouldBe` True
      needsQuoting "3.14" `shouldBe` True

    it "quotes negative numeric strings" $ do
      needsQuoting "-1" `shouldBe` True

    it "quotes .inf and .nan tokens" $ do
      needsQuoting ".inf" `shouldBe` True
      needsQuoting "-.inf" `shouldBe` True
      needsQuoting ".nan" `shouldBe` True

    it "does not quote hyphenated words" $ do
      needsQuoting "must-not" `shouldBe` False

    it "does not quote dotted names" $ do
      needsQuoting "Auth.Login" `shouldBe` False

    it "does not quote underscore names" $ do
      needsQuoting "my_spec" `shouldBe` False

  -- ── emitYamlValue ────────────────────────────────────────────────────

  describe "emitYamlValue" $ do
    it "emits plain string unquoted" $ do
      emitYamlValue (RVString "hello") `shouldBe` "hello"

    it "emits string needing quotes with double quotes" $ do
      emitYamlValue (RVString "key: value") `shouldBe` "\"key: value\""

    it "emits true boolean" $ do
      emitYamlValue (RVBool True) `shouldBe` "true"

    it "emits false boolean" $ do
      emitYamlValue (RVBool False) `shouldBe` "false"

    it "emits integer number without decimal" $ do
      emitYamlValue (RVNumber 42.0) `shouldBe` "42"

    it "emits floating number with decimal" $ do
      emitYamlValue (RVNumber 3.14) `shouldBe` "3.14"

    it "emits null" $ do
      emitYamlValue RVNull `shouldBe` "null"

    it "emits positive infinity" $ do
      emitYamlValue (RVNumber (1/0)) `shouldBe` ".inf"

    it "emits negative infinity" $ do
      emitYamlValue (RVNumber (-1/0)) `shouldBe` "-.inf"

    it "emits NaN" $ do
      emitYamlValue (RVNumber (0/0)) `shouldBe` ".nan"

    it "emits inline list" $ do
      emitYamlValue (RVList [RVString "a", RVString "b"] 1) `shouldBe` "[a, b]"

    it "emits inline mapping" $ do
      emitYamlValue (RVMapping [("k", RVString "v", 1)] 1) `shouldBe` "{k: v}"

    it "escapes double quotes in quoted strings" $ do
      emitYamlValue (RVString "say: \"hello\"") `shouldBe` "\"say: \\\"hello\\\"\""

    it "escapes newlines in quoted strings" $ do
      emitYamlValue (RVString "line1: \nline2") `shouldBe` "\"line1: \\nline2\""

    it "escapes backslashes in quoted strings" $ do
      emitYamlValue (RVString " a\\b") `shouldBe` "\" a\\\\b\""

  -- ── emitFragment ─────────────────────────────────────────────────────

  describe "emitFragment" $ do
    it "starts with document separator ---" $ do
      let doc = RawDocument 1 [("spec", RVString "Test", 1)]
      T.isPrefixOf "---\n" (emitFragment doc) `shouldBe` True

    it "ends with exactly one trailing LF" $ do
      let doc = RawDocument 1 [("spec", RVString "Test", 1)]
          result = emitFragment doc
      T.isSuffixOf "\n" result `shouldBe` True
      T.isSuffixOf "\n\n" result `shouldBe` False

    it "emits simple key-value pair" $ do
      let doc = RawDocument 1 [("spec", RVString "Test", 1)]
      emitFragment doc `shouldBe` "---\nspec: Test\n"

    it "emits multiple entries" $ do
      let doc = RawDocument 1
            [ ("spec", RVString "Test", 1)
            , ("INPUT", RVList [RVMapping [("MUST", RVString "exist", 3)] 3] 2, 2)
            ]
          result = emitFragment doc
      T.isInfixOf "spec: Test" result `shouldBe` True
      T.isInfixOf "INPUT:" result `shouldBe` True
      T.isInfixOf "- MUST: exist" result `shouldBe` True

    it "uses 2-space indentation for nested structures" $ do
      -- Top-level key with a list; items at indent 1 get (indent-1)*2=0 spaces for "-"
      -- and remaining entries get indent*2=2 spaces
      let doc = RawDocument 1
            [ ("INPUT", RVList
                [ RVMapping
                    [ ("MUST", RVString "be valid", 3)
                    , ("WHEN", RVString "called", 4)
                    ] 3
                ] 2, 2)
            ]
          result = emitFragment doc
      T.isInfixOf "- MUST: be valid" result `shouldBe` True
      T.isInfixOf "  WHEN: called" result `shouldBe` True

    it "emits list items with \"- \" prefix" $ do
      let doc = RawDocument 1
            [ ("items", RVList [RVString "alpha", RVString "beta"] 2, 2)
            ]
          result = emitFragment doc
      T.isInfixOf "- alpha" result `shouldBe` True
      T.isInfixOf "- beta" result `shouldBe` True

    it "emits empty mapping list item as {}" $ do
      let doc = RawDocument 1
            [ ("items", RVList [RVMapping [] 2] 2, 2)
            ]
          result = emitFragment doc
      T.isInfixOf "- {}" result `shouldBe` True

    it "quotes values that need quoting in emitted fragment" $ do
      let doc = RawDocument 1 [("note", RVString "key: value", 1)]
          result = emitFragment doc
      T.isInfixOf "\"key: value\"" result `shouldBe` True

    it "emits nested mapping entries" $ do
      let doc = RawDocument 1
            [ ("outer", RVMapping
                [ ("inner", RVString "deep", 3)
                ] 2, 2)
            ]
          result = emitFragment doc
      T.isInfixOf "  inner: deep" result `shouldBe` True
