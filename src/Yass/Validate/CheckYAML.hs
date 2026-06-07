{-# LANGUAGE OverloadedStrings #-}

module Yass.Validate.CheckYAML (checkYAML) where

import Data.Text (Text)
import qualified Data.Text as T

import Yass.Types (YassError(..), ErrorCode(..))
import Yass.Parser (parseYassFile, ParseResult(..), RawDocument)

-- | Check YAML well-formedness of a file.
-- Takes cwd (for error path display) and the absolute file path.
-- Returns Left with a single YassError on failure, or Right with parsed documents.
-- Emits at most one error per file, in priority order:
-- not_utf8, has_bom, empty_file, malformed, duplicate_key, anchor_or_alias
checkYAML :: FilePath -> FilePath -> IO (Either YassError [RawDocument])
checkYAML _cwd fp = do
  result <- parseYassFile fp
  case result of
    ParseOK docs -> return (Right docs)
    ParseError msg line ->
      let (code, specMsg) = classifyError msg
          err = YassError
                  { yeFile    = Just fp
                  , yeLine    = if line > 0 then Just line else Nothing
                  , yeCode    = code
                  , yeMessage = specMsg
                  }
      in return (Left err)

-- | Classify a ParseError message into the appropriate ErrorCode and spec message.
-- Priority order: not_utf8, has_bom, empty_file, malformed, duplicate_key, anchor_or_alias
classifyError :: Text -> (ErrorCode, Text)
classifyError msg
  | "BOM"           `T.isInfixOf` msg = (YamlHasBom, "file begins with a UTF-8 BOM")
  | "UTF-8"         `T.isInfixOf` msg = (YamlNotUtf8, "file is not valid UTF-8")
  | "empty file"    `T.isInfixOf` msg = (YamlEmptyFile, "empty file")
  | "duplicate key" `T.isInfixOf` msg =
      let key = extractDupKey msg
      in (YamlDuplicateKey, "duplicate mapping key: " <> key)
  | "anchor"        `T.isInfixOf` msg = (YamlAnchorOrAlias, "YAML anchors, aliases, and explicit tags are not allowed")
  | "alias"         `T.isInfixOf` msg = (YamlAnchorOrAlias, "YAML anchors, aliases, and explicit tags are not allowed")
  | "tag"           `T.isInfixOf` msg = (YamlAnchorOrAlias, "YAML anchors, aliases, and explicit tags are not allowed")
  | otherwise                         = (YamlMalformed, "YAML well-formedness error")

-- | Extract the duplicate key name from a parser error message like "duplicate key: foo"
extractDupKey :: Text -> Text
extractDupKey msg =
  case T.stripPrefix "duplicate key: " msg of
    Just key -> key
    Nothing -> T.drop 2 $ snd $ T.breakOn ": " msg
