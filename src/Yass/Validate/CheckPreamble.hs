{-# LANGUAGE OverloadedStrings #-}

module Yass.Validate.CheckPreamble (checkPreamble) where

import Data.Text (Text)
import qualified Data.Text as T
import Yass.Types (YassError(..), ErrorCode(..))
import Yass.Parser (RawDocument(..), RawValue(..))

-- | Check preamble structure of parsed YAML documents.
--
-- Returns at most one error per file, checked in priority order:
-- has_spec_key, empty_stream, missing, duplicate, misplaced,
-- missing_description, missing_version, unknown_version, bad_related
checkPreamble :: FilePath -> FilePath -> [RawDocument] -> [YassError]
checkPreamble _cwd fp docs = case docs of
  -- 1. Empty stream
  [] -> [mkError fp Nothing YamlEmptyStream "YAML stream contains no documents"]

  (first:rest)
    -- 2. First doc has a "spec" key -> has_spec_key
    --    This also means the preamble is missing, but has_spec_key takes priority
    | hasKey "spec" first ->
        [mkError fp (Just (rawDocLine first)) PreambleHasSpecKey
          "first document must be a Preamble, not a Spec"]

    -- First doc does NOT have "spec" key, so it is the preamble candidate.
    -- Check for structural issues with non-first docs before validating
    -- preamble contents.
    | otherwise ->
        let -- Non-first docs without a "spec" key are extra preambles
            extraPreambles = filter (not . hasKey "spec") rest
        in case extraPreambles of
           (extra:_) ->
             -- Any non-first doc without "spec" key means 2+ preambles → duplicate
             -- (Priority 4: duplicate before 5: misplaced)
             [mkError fp (Just (rawDocLine extra)) PreambleDuplicate
               "more than one Preamble in file"]
           [] ->
             -- No extra preambles. Validate preamble contents.
             validatePreambleContents fp first

-- | Validate the contents of a preamble document.
-- Returns at most one error, in priority order:
-- missing_description, missing_version, unknown_version, bad_related
validatePreambleContents :: FilePath -> RawDocument -> [YassError]
validatePreambleContents fp doc
  | not (hasKey "description" doc) =
      [mkError fp (Just (rawDocLine doc)) PreambleMissingDescription
        "Preamble missing description"]
  | not (hasKey "version" doc) =
      [mkError fp (Just (rawDocLine doc)) PreambleMissingVersion
        "Preamble missing version"]
  | otherwise =
      case getKeyValue "version" doc of
        Just (RVString "v1") ->
          -- Version is valid, now check related
          checkRelated fp doc
        Just (RVString v) ->
          [mkError fp (Just (rawDocLine doc)) PreambleUnknownVersion
            ("unsupported Preamble version: " <> v)]
        Just RVNull ->
          [mkError fp (Just (rawDocLine doc)) PreambleUnknownVersion
            "unsupported Preamble version: null"]
        Just (RVBool b) ->
          [mkError fp (Just (rawDocLine doc)) PreambleUnknownVersion
            ("unsupported Preamble version: " <> if b then "true" else "false")]
        Just (RVNumber n) ->
          [mkError fp (Just (rawDocLine doc)) PreambleUnknownVersion
            ("unsupported Preamble version: " <> T.pack (show n))]
        Just (RVList _ _) ->
          [mkError fp (Just (rawDocLine doc)) PreambleUnknownVersion
            "unsupported Preamble version: [list]"]
        Just (RVMapping _ _) ->
          [mkError fp (Just (rawDocLine doc)) PreambleUnknownVersion
            "unsupported Preamble version: {mapping}"]
        Nothing ->
          -- Should not happen since we checked hasKey above
          []

-- | Check the "related" key if present. Must be a list of strings.
checkRelated :: FilePath -> RawDocument -> [YassError]
checkRelated fp doc =
  case getKeyValue "related" doc of
    Nothing -> []  -- "related" is optional
    Just (RVList items _) ->
      if all isRVString items
        then []
        else [mkError fp (Just (rawDocLine doc)) PreambleBadRelated
               "Preamble related must be a sequence of strings"]
    Just _ ->
      [mkError fp (Just (rawDocLine doc)) PreambleBadRelated
        "Preamble related must be a sequence of strings"]

-- | Check if a RawValue is a string
isRVString :: RawValue -> Bool
isRVString (RVString _) = True
isRVString _            = False

-- | Check if a RawDocument has a given key
hasKey :: Text -> RawDocument -> Bool
hasKey key doc = any (\(k, _, _) -> k == key) (rawDocEntries doc)

-- | Get the value for a key from a RawDocument
getKeyValue :: Text -> RawDocument -> Maybe RawValue
getKeyValue key doc =
  case filter (\(k, _, _) -> k == key) (rawDocEntries doc) of
    ((_, v, _):_) -> Just v
    []            -> Nothing

-- | Helper to construct a YassError
mkError :: FilePath -> Maybe Int -> ErrorCode -> Text -> YassError
mkError fp line code msg = YassError
  { yeFile    = Just fp
  , yeLine    = line
  , yeCode    = code
  , yeMessage = msg
  }
