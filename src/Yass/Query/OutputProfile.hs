{-# LANGUAGE OverloadedStrings #-}

module Yass.Query.OutputProfile
  ( emitFragment
  , emitYamlValue
  , needsQuoting
  ) where

import Data.Char (isDigit)
import Data.List (sortBy)
import Data.Ord (comparing)
import Data.Text (Text)
import qualified Data.Text as T
import Yass.Parser (RawDocument(..), RawValue(..))

-- | Emit a spec fragment as YAML per the output profile.
-- Starts with "---", uses 2-space indentation, emits trailing LF.
-- Provenance is a list of (refTarget, Int) pairs indicating which list items
-- at a given index should have a provenance comment.
emitFragment :: RawDocument -> Text
emitFragment doc =
  let entries = rawDocEntries doc
      body = T.intercalate "\n" $ concatMap (emitEntry 0) entries
  in "---\n" <> body <> "\n"

-- | Emit a single key-value entry at a given indentation level
emitEntry :: Int -> (Text, RawValue, Int) -> [Text]
emitEntry indent (key, val, _) =
  let ind = T.replicate indent "  "
  in case val of
    RVList items _ ->
      (ind <> key <> ":") : concatMap (emitListItem (indent + 1)) (zip [0..] items)
    RVMapping entries _ ->
      (ind <> key <> ":") : concatMap (emitEntry (indent + 1)) entries
    _ ->
      [ind <> key <> ": " <> emitYamlValue val]

-- | Emit a list item.
-- The indent parameter is the indentation level of the list content.
-- The "- " prefix replaces the first 2 chars of indentation.
emitListItem :: Int -> (Int, RawValue) -> [Text]
emitListItem _indent (_, RVComment txt) =
  -- Provenance comments are emitted at column zero
  ["# " <> txt]
emitListItem indent (_, val) =
  let ind = T.replicate indent "  "
  in case val of
    RVMapping entries _ ->
      -- Sort obligation entries: Normativity first, WHEN second, References last
      let sorted = sortObligationKeys entries
      in case sorted of
        [] -> [ind <> "- {}"]
        ((k, v, _ln):rest) ->
          let firstLine = case v of
                RVList items _ ->
                  (ind <> "- " <> k <> ":") : concatMap (emitListItem (indent + 1)) (zip [0..] items)
                RVMapping subEntries _ ->
                  (ind <> "- " <> k <> ":") : concatMap (emitEntry (indent + 1)) subEntries
                _ -> [ind <> "- " <> k <> ": " <> emitYamlValue v]
              restLines = concatMap (emitEntry (indent + 1)) rest
          in firstLine ++ restLines
    _ ->
      [ind <> "- " <> emitYamlValue val]

-- | Sort obligation mapping keys per spec:
-- Normativity keywords first, then WHEN guard, then Reference relations.
-- Keys that don't match any category keep their relative order at the end.
-- Uses a stable sort (sortBy uses merge sort) so relative order within
-- each group is preserved.
sortObligationKeys :: [(Text, RawValue, Int)] -> [(Text, RawValue, Int)]
sortObligationKeys = sortBy (comparing keyGroup)
  where
    keyGroup :: (Text, RawValue, Int) -> Int
    keyGroup (k, _, _)
      | k `elem` ["MUST", "MUST-NOT", "SHOULD", "SHOULD-NOT", "MAY"] = 0
      | k == "WHEN" = 1
      | k `elem` ["CONFORMS", "USES", "SEE"] = 2
      | otherwise = 3

-- | Emit a scalar YAML value.
-- Plain scalars unquoted by default. Double-quote when needed.
emitYamlValue :: RawValue -> Text
emitYamlValue (RVString t)
  | needsQuoting t = "\"" <> escapeYaml t <> "\""
  | otherwise = t
emitYamlValue (RVBool True) = "true"
emitYamlValue (RVBool False) = "false"
emitYamlValue (RVNumber n)
  | isInfinite n && n > 0 = ".inf"
  | isInfinite n = "-.inf"
  | isNaN n = ".nan"
  | n == fromIntegral (round n :: Integer) = T.pack (show (round n :: Integer))
  | otherwise = T.pack (show n)
emitYamlValue RVNull = "null"
emitYamlValue (RVList items _) =
  "[" <> T.intercalate ", " (map emitYamlValue items) <> "]"
emitYamlValue (RVMapping entries _) =
  "{" <> T.intercalate ", " (map (\(k,v,_) -> k <> ": " <> emitYamlValue v) entries) <> "}"
emitYamlValue (RVComment txt) = "# " <> txt

-- | Check if a scalar text needs double-quoting per the output profile.
needsQuoting :: Text -> Bool
needsQuoting t
  | T.null t = True
  | T.isInfixOf ": " t = True
  | T.head t `elem` ("?-*&!|>%@" :: String) = True
  | not (T.null t) && (T.head t == ' ' || T.last t == ' ') = True
  | T.any (\c -> c == '\n' || c == '\r' || c == '\t') t = True
  | T.any (== '#') t = True
  | isYamlTypeToken t = True
  | otherwise = False

-- | Check if text matches a YAML 1.2 core-schema type token
isYamlTypeToken :: Text -> Bool
isYamlTypeToken t = T.toLower t `elem`
  ["true", "false", "null", "yes", "no", "on", "off"
  , ".inf", "-.inf", ".nan", "~"]
  || looksNumeric t

-- | Check if text looks like a numeric literal
looksNumeric :: Text -> Bool
looksNumeric t
  | T.null t = False
  | otherwise =
    let s = T.unpack t
    in case s of
         ('-':rest) -> all isDigit rest && not (null rest)
         ('+':rest) -> all isDigit rest && not (null rest)
         _ -> case reads s :: [(Double, String)] of
                [(_, "")] -> True
                _ -> False

-- | Escape special characters for YAML double-quoted strings
escapeYaml :: Text -> Text
escapeYaml = T.concatMap escapeChar
  where
    escapeChar '"'  = "\\\""
    escapeChar '\\' = "\\\\"
    escapeChar '\n' = "\\n"
    escapeChar '\r' = "\\r"
    escapeChar '\t' = "\\t"
    escapeChar c    = T.singleton c
