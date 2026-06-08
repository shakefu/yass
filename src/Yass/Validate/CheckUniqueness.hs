{-# LANGUAGE OverloadedStrings #-}

module Yass.Validate.CheckUniqueness (checkUniqueness) where

import Data.Text (Text)
import qualified Data.Text as T
import qualified Data.Map.Strict as Map

import Yass.Types (YassError(..), ErrorCode(..))
import Yass.Parser (RawDocument(..), RawValue(..))

-- | Check spec name uniqueness within a file.
--
-- Returns one 'YassError' per duplicate-after-first occurrence of a spec name.
-- The error points at the line of each subsequent occurrence.
--
-- Parameters:
--   cwd      - current working directory (for error context)
--   filePath - path to the file being validated
--   docs     - all parsed 'RawDocument' values from the file (including preamble)
checkUniqueness :: FilePath -> FilePath -> [RawDocument] -> [YassError]
checkUniqueness _cwd filePath docs =
  let specDocs = extractSpecNames docs
      -- Build up a map of name -> [line], preserving document order
      nameLines :: Map.Map Text [Int]
      nameLines = foldl (\acc (name, line) ->
        Map.insertWith (\new old -> old ++ new) name [line] acc
        ) Map.empty specDocs
      -- For each name that appears more than once, produce an error for each
      -- occurrence after the first, in order of appearance
      errors = concatMap (\(name, lines_) ->
        case lines_ of
          (_:dups) -> map (\ln -> YassError
            { yeFile    = Just filePath
            , yeLine    = Just ln
            , yeCode    = SpecDuplicateName
            , yeMessage = "duplicate spec name in file: " <> name
            }) dups
          _ -> []
        ) (Map.toList nameLines)
  in -- Sort errors by line number to preserve document order
     sortByLine errors

-- | Extract (specName, line) pairs from documents that have a "spec" key
-- with a string value. Skips the preamble (first doc) and any docs without
-- a valid string spec name (those are handled by CheckSpec).
extractSpecNames :: [RawDocument] -> [(Text, Int)]
extractSpecNames [] = []
extractSpecNames (_:rest) = concatMap extractFromDoc rest
  where
    extractFromDoc doc =
      case lookup3 "spec" (rawDocEntries doc) of
        Just (RVString name, line) | not (T.null name) -> [(name, line)]
        _ -> []

-- | Lookup a key in a list of (key, value, line) triples
lookup3 :: Text -> [(Text, RawValue, Int)] -> Maybe (RawValue, Int)
lookup3 _ [] = Nothing
lookup3 k ((k', v, ln):rest)
  | k == k'   = Just (v, ln)
  | otherwise  = lookup3 k rest

-- | Sort errors by line number
sortByLine :: [YassError] -> [YassError]
sortByLine = sortBy (\a b -> compare (yeLine a) (yeLine b))
  where
    sortBy _ [] = []
    sortBy cmp (x:xs) =
      let lesser  = filter (\y -> cmp y x /= GT) xs
          greater = filter (\y -> cmp y x == GT) xs
      in sortBy cmp lesser ++ [x] ++ sortBy cmp greater
