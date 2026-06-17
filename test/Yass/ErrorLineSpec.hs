{-# LANGUAGE OverloadedStrings #-}

module Yass.ErrorLineSpec (spec) where

import Test.Hspec
import Yass.ErrorLine
import Yass.Types (ErrorCode(..))

spec :: Spec
spec = do
  describe "relativizePath" $ do
    describe "file under cwd" $ do
      it "returns relative path for file in subdirectory" $
        relativizePath "/home/user/project" "/home/user/project/src/Foo.hs"
          `shouldBe` "src/Foo.hs"

      it "returns relative path for deeply nested file" $
        relativizePath "/home/user/project" "/home/user/project/a/b/c/d.yaml"
          `shouldBe` "a/b/c/d.yaml"

    describe "file directly in cwd" $ do
      it "returns basename for file directly inside cwd" $
        relativizePath "/home/user/project" "/home/user/project/foo.yaml"
          `shouldBe` "foo.yaml"

      it "returns basename for dotfile directly inside cwd" $
        relativizePath "/home/user/project" "/home/user/project/.yass.yaml"
          `shouldBe` ".yass.yaml"

    describe "file not under cwd" $ do
      it "returns absolute path when file is not under cwd" $
        relativizePath "/home/user/project" "/other/place/bar.yaml"
          `shouldBe` "/other/place/bar.yaml"

      it "returns absolute path when cwd is a prefix of dir name but not a parent" $
        relativizePath "/home/user/proj" "/home/user/project/foo.yaml"
          `shouldBe` "/home/user/project/foo.yaml"

      it "returns absolute path for sibling directory" $
        relativizePath "/home/user/project-a" "/home/user/project-b/foo.yaml"
          `shouldBe` "/home/user/project-b/foo.yaml"

    describe "non-absolute inputs" $ do
      it "returns fp with forward slashes when fp is not absolute" $
        relativizePath "/home/user" "relative/path.yaml"
          `shouldBe` "relative/path.yaml"

      it "returns fp as-is when cwd is not absolute" $
        relativizePath "relative-cwd" "/some/file.yaml"
          `shouldBe` "/some/file.yaml"

    describe "no leading dot-slash" $ do
      it "strips leading ./ from a relative fp" $
        relativizePath "/home/user" "./relative/path.yaml"
          `shouldBe` "relative/path.yaml"

      it "strips leading ./ from relativized result" $
        -- This cannot happen with real paths since stripPrefix won't produce ./,
        -- but tests the stripping behavior for non-absolute paths
        relativizePath "/some/cwd" "./foo.yaml"
          `shouldBe` "foo.yaml"

    describe "forward slashes" $ do
      it "converts backslashes to forward slashes in relative result" $
        relativizePath "/home/user/project" "/home/user/project/src\\Foo.hs"
          `shouldBe` "src/Foo.hs"

      it "converts backslashes to forward slashes in absolute result" $
        relativizePath "/home/user" "/other\\path\\file.yaml"
          `shouldBe` "/other/path/file.yaml"

    describe "cwd with trailing separator" $ do
      it "handles cwd ending with /" $
        relativizePath "/home/user/project/" "/home/user/project/src/Foo.hs"
          `shouldBe` "src/Foo.hs"

    describe "yass token" $ do
      it "passes through 'yass' unchanged (not absolute)" $
        relativizePath "/home/user" "yass"
          `shouldBe` "yass"

  describe "formatErrorLine" $ do
    it "formats error line with file and line number" $
      formatErrorLine "/home/user" "/home/user/foo.yaml" 42 YamlMalformed "bad yaml"
        `shouldBe` "foo.yaml:42: [yass.yaml.malformed] bad yaml"

    it "formats error line with subdirectory path" $
      formatErrorLine "/home/user" "/home/user/specs/api.yass.yaml" 10 SpecNoName "missing name"
        `shouldBe` "specs/api.yass.yaml:10: [yass.spec.no_name] missing name"

    it "formats error line with absolute path when not under cwd" $
      formatErrorLine "/home/user" "/other/foo.yaml" 1 PathNotFound "file not found"
        `shouldBe` "/other/foo.yaml:1: [yass.path.not_found] file not found"

    it "replaces newlines in message with spaces" $
      formatErrorLine "/cwd" "/cwd/f.yaml" 1 YamlMalformed "line1\nline2\nline3"
        `shouldBe` "f.yaml:1: [yass.yaml.malformed] line1 line2 line3"

    it "replaces carriage returns in message with spaces" $
      formatErrorLine "/cwd" "/cwd/f.yaml" 1 YamlMalformed "line1\rline2"
        `shouldBe` "f.yaml:1: [yass.yaml.malformed] line1 line2"

    it "replaces CRLF in message with a single space" $
      formatErrorLine "/cwd" "/cwd/f.yaml" 1 YamlMalformed "line1\r\nline2"
        `shouldBe` "f.yaml:1: [yass.yaml.malformed] line1 line2"

    it "uses yass token when fp is 'yass'" $
      formatErrorLine "/cwd" "yass" 0 InternalUncaught "something broke"
        `shouldBe` "yass:0: [yass.internal.uncaught] something broke"

  describe "formatErrorLineNoLine" $ do
    it "formats error line without line number" $
      formatErrorLineNoLine "/home/user" "/home/user/foo.yaml" YamlMalformed "bad yaml"
        `shouldBe` "foo.yaml: [yass.yaml.malformed] bad yaml"

    it "formats error line with subdirectory path" $
      formatErrorLineNoLine "/home/user" "/home/user/specs/api.yass.yaml" SpecNoName "missing"
        `shouldBe` "specs/api.yass.yaml: [yass.spec.no_name] missing"

    it "formats error line with absolute path when not under cwd" $
      formatErrorLineNoLine "/home/user" "/other/foo.yaml" PathNotFound "not found"
        `shouldBe` "/other/foo.yaml: [yass.path.not_found] not found"

    it "replaces newlines in message with spaces" $
      formatErrorLineNoLine "/cwd" "/cwd/f.yaml" YamlMalformed "a\nb\nc"
        `shouldBe` "f.yaml: [yass.yaml.malformed] a b c"

    it "uses yass token when fp is 'yass'" $
      formatErrorLineNoLine "/cwd" "yass" InternalUncaught "oops"
        `shouldBe` "yass: [yass.internal.uncaught] oops"

    it "handles empty message" $
      formatErrorLineNoLine "/cwd" "/cwd/f.yaml" YamlMalformed ""
        `shouldBe` "f.yaml: [yass.yaml.malformed] "
