module Main (main) where

import Test.Hspec

import qualified Yass.TypesSpec
import qualified Yass.ErrorLineSpec
import qualified Yass.ParserSpec
import qualified Yass.ProjectRootSpec
import qualified Yass.DiscoverSpec
import qualified Yass.GlobSpec
import qualified Yass.Validate.CheckYAMLSpec
import qualified Yass.Validate.CheckPreambleSpec
import qualified Yass.Validate.CheckSpecSpec
import qualified Yass.Validate.CheckUniquenessSpec
import qualified Yass.Validate.CheckRefsSpec
import qualified Yass.ValidateSpec
import qualified Yass.ListSpec
import qualified Yass.Query.NameLookupSpec
import qualified Yass.Query.ExtractFragmentSpec
import qualified Yass.Query.InlineConformsSpec
import qualified Yass.Query.OutputProfileSpec
import qualified Yass.QuerySpec
import qualified Yass.CLISpec
import qualified Yass.IntegrationSpec

main :: IO ()
main = hspec $ do
  describe "Yass.Types" Yass.TypesSpec.spec
  describe "Yass.ErrorLine" Yass.ErrorLineSpec.spec
  describe "Yass.Parser" Yass.ParserSpec.spec
  describe "Yass.ProjectRoot" Yass.ProjectRootSpec.spec
  describe "Yass.Discover" Yass.DiscoverSpec.spec
  describe "Yass.Glob" Yass.GlobSpec.spec
  describe "Yass.Validate.CheckYAML" Yass.Validate.CheckYAMLSpec.spec
  describe "Yass.Validate.CheckPreamble" Yass.Validate.CheckPreambleSpec.spec
  describe "Yass.Validate.CheckSpec" Yass.Validate.CheckSpecSpec.spec
  describe "Yass.Validate.CheckUniqueness" Yass.Validate.CheckUniquenessSpec.spec
  describe "Yass.Validate.CheckRefs" Yass.Validate.CheckRefsSpec.spec
  describe "Yass.Validate" Yass.ValidateSpec.spec
  describe "Yass.List" Yass.ListSpec.spec
  describe "Yass.Query.NameLookup" Yass.Query.NameLookupSpec.spec
  describe "Yass.Query.ExtractFragment" Yass.Query.ExtractFragmentSpec.spec
  describe "Yass.Query.InlineConforms" Yass.Query.InlineConformsSpec.spec
  describe "Yass.Query.OutputProfile" Yass.Query.OutputProfileSpec.spec
  describe "Yass.Query" Yass.QuerySpec.spec
  describe "Yass.CLI" Yass.CLISpec.spec
  describe "Integration" Yass.IntegrationSpec.spec
