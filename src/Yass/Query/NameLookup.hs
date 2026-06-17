{-# LANGUAGE OverloadedStrings #-}

module Yass.Query.NameLookup
  ( lookupName
  , matchesName
  , MatchResult(..)
  ) where

import Data.Text (Text)
import qualified Data.Text as T
import Yass.Parser (RawDocument(..), RawValue(..))

-- | Result of a name lookup
data MatchResult
  = NoMatch
  | SingleMatch !FilePath !Text ![RawDocument]  -- ^ file, spec name, all docs in file
  | MultiMatch ![(FilePath, Text)]              -- ^ multiple (file, specName) matches
  deriving (Show, Eq)

-- | Look up a spec by name across multiple files.
-- Matches full spec name (case-sensitive) OR any dot-aligned trailing suffix.
lookupName :: Text -> [(FilePath, [RawDocument])] -> MatchResult
lookupName query filesDocs =
  let matches = concatMap (findInFile query) filesDocs
  in case matches of
       []  -> NoMatch
       [(fp, name, docs)] -> SingleMatch fp name docs
       ms  -> MultiMatch [(fp, name) | (fp, name, _) <- ms]

-- | Find matching specs in a single file
findInFile :: Text -> (FilePath, [RawDocument]) -> [(FilePath, Text, [RawDocument])]
findInFile query (fp, docs) =
  case docs of
    [] -> []
    (_:specDocs) ->
      let names = extractSpecNames specDocs
          matched = filter (matchesName query) names
      in [(fp, name, docs) | name <- matched]

-- | Extract spec names from spec documents
extractSpecNames :: [RawDocument] -> [Text]
extractSpecNames = concatMap getSpecName
  where
    getSpecName doc = case lookup3 "spec" (rawDocEntries doc) of
      Just (RVString name, _) | not (T.null name) -> [name]
      _ -> []

-- | Check if a query matches a spec name.
-- Matches:
-- - Exact full match (case-sensitive)
-- - Dot-aligned trailing suffix: query equals spec name with zero or more
--   leading dot-separated components removed.
-- Does NOT match partial substrings that aren't a full trailing component.
matchesName :: Text -> Text -> Bool
matchesName query specName
  | query == specName = True  -- exact match
  | otherwise =
      -- Check if query matches a trailing suffix after a dot
      let suffixes = dotSuffixes specName
      in query `elem` suffixes

-- | Generate all dot-aligned trailing suffixes of a name.
-- For "a.b.c" returns ["b.c", "c"]
-- Does NOT include the full name itself.
dotSuffixes :: Text -> [Text]
dotSuffixes name =
  let parts = T.splitOn "." name
      tails' = tail' (map (T.intercalate ".") (suffixesOf parts))
  in tails'
  where
    suffixesOf [] = []
    suffixesOf xs = xs : suffixesOf (tail xs)
    tail' [] = []
    tail' (_:xs) = xs

-- | Lookup helper
lookup3 :: Text -> [(Text, RawValue, Int)] -> Maybe (RawValue, Int)
lookup3 _ [] = Nothing
lookup3 k ((k', v, ln):rest)
  | k == k'   = Just (v, ln)
  | otherwise  = lookup3 k rest
