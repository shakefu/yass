{-# LANGUAGE OverloadedStrings #-}

module Yass.Query.ExtractFragment (extractFragment, findSpecDoc) where

import Data.Text (Text)
import qualified Data.Text as T
import Yass.Parser (RawDocument(..), RawValue(..))

-- | Extract a spec fragment -- find the spec document matching the given name.
-- Returns the matching RawDocument if found.
extractFragment :: Text -> [RawDocument] -> Maybe RawDocument
extractFragment name docs = case docs of
  [] -> Nothing
  (_:specDocs) -> findSpecDoc name specDocs

-- | Find a spec document by name in a list of spec documents.
findSpecDoc :: Text -> [RawDocument] -> Maybe RawDocument
findSpecDoc _ [] = Nothing
findSpecDoc name (doc:rest) =
  case lookup3 "spec" (rawDocEntries doc) of
    Just (RVString n, _) | n == name -> Just doc
    _ -> findSpecDoc name rest

-- | Lookup helper
lookup3 :: Text -> [(Text, RawValue, Int)] -> Maybe (RawValue, Int)
lookup3 _ [] = Nothing
lookup3 k ((k', v, ln):rest)
  | k == k'   = Just (v, ln)
  | otherwise  = lookup3 k rest
