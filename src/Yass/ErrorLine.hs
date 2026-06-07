{-# LANGUAGE OverloadedStrings #-}

module Yass.ErrorLine
  ( formatErrorLine
  , formatErrorLineNoLine
  , relativizePath
  ) where

import Data.Text (Text)
import qualified Data.Text as T
import System.FilePath (isAbsolute, takeFileName)
import Data.List (stripPrefix)
import Yass.Types

-- | Format an error line with a known source line:
-- @<file>:<line>: [<code>] <message>@
formatErrorLine :: FilePath -> FilePath -> Int -> ErrorCode -> Text -> Text
formatErrorLine cwd fp line code msg =
  let relPath = relativizePath cwd fp
      sanitizedMsg = sanitize msg
  in T.pack relPath <> ":" <> T.pack (show line) <> ": [" <> errorCodeText code <> "] " <> sanitizedMsg

-- | Format an error line without a known source line:
-- @<file>: [<code>] <message>@
formatErrorLineNoLine :: FilePath -> FilePath -> ErrorCode -> Text -> Text
formatErrorLineNoLine cwd fp code msg =
  let relPath = relativizePath cwd fp
      sanitizedMsg = sanitize msg
  in T.pack relPath <> ": [" <> errorCodeText code <> "] " <> sanitizedMsg

-- | Replace any newline characters in the message with a single ASCII space.
-- Handle CRLF first so it becomes a single space, then lone CR/LF.
sanitize :: Text -> Text
sanitize t = T.replace "\r" " " $ T.replace "\n" " " $ T.replace "\r\n" " " t

-- | Relativize a path according to the spec rules:
-- - Relative when lexical absolute path begins with lexical absolute cwd followed by "/"
-- - Basename alone when directly inside cwd
-- - Absolute path otherwise
-- - Forward slashes on all platforms
-- - No leading "./"
-- - No symlink resolution
relativizePath :: FilePath -> FilePath -> FilePath
relativizePath cwd fp
  | not (isAbsolute fp) = stripDotSlash (forwardSlashes fp)
  | not (isAbsolute cwd) = forwardSlashes fp
  | otherwise =
      let cwdNorm = stripTrailingSep cwd
      in case stripPrefix (cwdNorm ++ "/") fp of
           Just rel
             | null rel  -> forwardSlashes (takeFileName fp)
             | otherwise -> stripDotSlash (forwardSlashes rel)
           Nothing -> forwardSlashes fp
  where
    forwardSlashes = map (\c -> if c == '\\' then '/' else c)
    stripTrailingSep s = case s of
      [] -> []
      _  -> if last s == '/' || last s == '\\' then init s else s
    stripDotSlash ('.':'/':rest) = rest
    stripDotSlash s = s
