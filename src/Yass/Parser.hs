{-# LANGUAGE OverloadedStrings #-}
{-# LANGUAGE ScopedTypeVariables #-}

module Yass.Parser
  ( parseYassFile
  , parseYassDocuments
  , RawDocument(..)
  , RawValue(..)
  , ParseResult(..)
  ) where

import qualified Data.ByteString as BS
import Data.Conduit (runConduit, (.|))
import qualified Data.Conduit.List as CL
import Control.Monad.Trans.Resource (runResourceT)
import Data.Text (Text)
import qualified Data.Text as T
import qualified Data.Text.Encoding as TE
import qualified Data.Text.Encoding.Error as TEE (lenientDecode)
import qualified Data.Set as Set
import Text.Libyaml
  ( MarkedEvent(..)
  , Event(..)
  , Tag(..)
  , Style(..)
  , YamlMark(..)
  , YamlException(..)
  , decodeMarked
  )
import Control.Exception (try, SomeException)
import qualified Control.Exception as E

-- | A raw parsed YAML document (mapping of key -> value)
data RawDocument = RawDocument
  { rawDocLine    :: !Int
  , rawDocEntries :: ![(Text, RawValue, Int)]  -- ^ (key, value, line)
  } deriving (Show, Eq)

-- | Raw YAML values
data RawValue
  = RVString !Text
  | RVBool !Bool
  | RVNumber !Double
  | RVNull
  | RVList ![RawValue] !Int
  | RVMapping ![(Text, RawValue, Int)] !Int
  | RVComment !Text  -- ^ Provenance or annotation comment (emitted as "# <text>")
  deriving (Show, Eq)

-- | Result of parsing a yass file
data ParseResult
  = ParseOK ![RawDocument]
  | ParseError !Text !Int  -- ^ error message, line
  deriving (Show, Eq)

-- | Parse a yass file from disk, returning raw documents
parseYassFile :: FilePath -> IO ParseResult
parseYassFile fp = do
  bs <- BS.readFile fp
  parseYassBytes fp bs

-- | Parse yass documents from text content (for testing convenience)
parseYassDocuments :: FilePath -> Text -> ParseResult
parseYassDocuments = error "parseYassDocuments: use parseYassFile for IO-based parsing"

-- | Internal: parse from raw bytes
parseYassBytes :: FilePath -> BS.ByteString -> IO ParseResult
parseYassBytes fp bs = do
  -- Priority order per spec: not_utf8 > has_bom > empty_file > malformed
  -- Check for empty file first (it's valid UTF-8 vacuously)
  if BS.null bs
    then return $ ParseError "empty file" 0
    else do
      -- Check for valid UTF-8 (highest priority after empty)
      case validateUtf8 bs of
        Just err -> return $ ParseError err 1
        Nothing -> do
          -- Check for BOM (UTF-8 BOM: EF BB BF)
          let hasBom = BS.length bs >= 3
                       && BS.index bs 0 == 0xEF
                       && BS.index bs 1 == 0xBB
                       && BS.index bs 2 == 0xBF
          if hasBom
            then return $ ParseError "file begins with a UTF-8 BOM" 1
            else parseYamlEvents fp bs

-- | Validate that bytes are valid UTF-8 (pure check)
validateUtf8 :: BS.ByteString -> Maybe Text
validateUtf8 bs =
  if isValidUtf8 bs then Nothing else Just "file is not valid UTF-8"

-- | Check if ByteString is valid UTF-8 (pure check)
isValidUtf8 :: BS.ByteString -> Bool
isValidUtf8 bs = go 0
  where
    len = BS.length bs
    go i
      | i >= len = True
      | otherwise =
        let b = BS.index bs i
        in if b <= 0x7F then go (i + 1)                     -- ASCII
           else if b >= 0xC2 && b <= 0xDF then               -- 2-byte
             i + 1 < len && isCont (BS.index bs (i+1)) && go (i + 2)
           else if b >= 0xE0 && b <= 0xEF then               -- 3-byte
             i + 2 < len
             && isCont (BS.index bs (i+1))
             && isCont (BS.index bs (i+2))
             && not (b == 0xE0 && BS.index bs (i+1) < 0xA0)  -- overlong
             && not (b == 0xED && BS.index bs (i+1) > 0x9F)  -- surrogate
             && go (i + 3)
           else if b >= 0xF0 && b <= 0xF4 then               -- 4-byte
             i + 3 < len
             && isCont (BS.index bs (i+1))
             && isCont (BS.index bs (i+2))
             && isCont (BS.index bs (i+3))
             && not (b == 0xF0 && BS.index bs (i+1) < 0x90)  -- overlong
             && not (b == 0xF4 && BS.index bs (i+1) > 0x8F)  -- too large
             && go (i + 4)
           else False
    isCont b = b >= 0x80 && b <= 0xBF

-- | Parse YAML events from valid UTF-8 bytes
parseYamlEvents :: FilePath -> BS.ByteString -> IO ParseResult
parseYamlEvents _fp bs = do
  result <- try $ runResourceT $ runConduit $
    decodeMarked bs .| CL.consume
  case result of
    Left (e :: SomeException) ->
      case E.fromException e of
        Just (YamlParseException problem _context mark) ->
          return $ ParseError (T.pack problem) (yamlLine mark + 1)
        Just (YamlException msg) ->
          return $ ParseError (T.pack msg) 1
        Nothing ->
          return $ ParseError (T.pack (show e)) 1
    Right events -> processEvents events

-- | Process a list of MarkedEvents into RawDocuments
processEvents :: [MarkedEvent] -> IO ParseResult
processEvents events = do
  -- First pass: check for anchors, aliases, and tags
  case findForbidden events of
    Just (msg, line) -> return $ ParseError msg line
    Nothing -> do
      -- Parse documents from events
      let docs = parseDocuments events
      case docs of
        Left (msg, line) -> return $ ParseError msg line
        Right [] -> return $ ParseOK []
        Right ds -> return $ ParseOK ds

-- | Check for forbidden YAML features: anchors, aliases, non-standard tags
findForbidden :: [MarkedEvent] -> Maybe (Text, Int)
findForbidden [] = Nothing
findForbidden (me:rest) =
  case yamlEvent me of
    EventAlias _ ->
      Just ("anchor/alias not allowed", yamlLine (yamlStartMark me) + 1)
    EventScalar _ tag _ anchor ->
      case checkAnchor anchor (yamlStartMark me) of
        Just r  -> Just r
        Nothing -> case checkTag tag (yamlStartMark me) of
          Just r  -> Just r
          Nothing -> findForbidden rest
    EventSequenceStart tag _ anchor ->
      case checkAnchor anchor (yamlStartMark me) of
        Just r  -> Just r
        Nothing -> case checkTag tag (yamlStartMark me) of
          Just r  -> Just r
          Nothing -> findForbidden rest
    EventMappingStart tag _ anchor ->
      case checkAnchor anchor (yamlStartMark me) of
        Just r  -> Just r
        Nothing -> case checkTag tag (yamlStartMark me) of
          Just r  -> Just r
          Nothing -> findForbidden rest
    _ -> findForbidden rest

-- | Check if an anchor is present
checkAnchor :: Maybe String -> YamlMark -> Maybe (Text, Int)
checkAnchor (Just _) mark = Just ("anchor/alias not allowed", yamlLine mark + 1)
checkAnchor Nothing _ = Nothing

-- | Check if a tag is non-standard (UriTag)
checkTag :: Tag -> YamlMark -> Maybe (Text, Int)
checkTag (UriTag _) mark = Just ("custom tag not allowed", yamlLine mark + 1)
checkTag _ _ = Nothing

-- | Parse documents from a list of MarkedEvents
parseDocuments :: [MarkedEvent] -> Either (Text, Int) [RawDocument]
parseDocuments events = go events []
  where
    go [] acc = Right (reverse acc)
    go (me:rest) acc =
      case yamlEvent me of
        EventStreamStart -> go rest acc
        EventStreamEnd   -> Right (reverse acc)
        EventDocumentStart ->
          let docLine = yamlLine (yamlStartMark me) + 1
          in case parseDocBody rest of
               Left err -> Left err
               Right (doc, remaining) ->
                 go remaining (RawDocument docLine (rawDocEntries doc) : acc)
        -- Skip other events at top level
        _ -> go rest acc

-- | Parse document body (expects a mapping at top level, then EventDocumentEnd)
parseDocBody :: [MarkedEvent] -> Either (Text, Int) (RawDocument, [MarkedEvent])
parseDocBody [] = Left ("unexpected end of events in document", 0)
parseDocBody (me:rest) =
  case yamlEvent me of
    EventMappingStart _ _ _ ->
      let mapLine = yamlLine (yamlStartMark me) + 1
      in case parseMapping rest Set.empty of
           Left err -> Left err
           Right (entries, remaining) ->
             case skipDocEnd remaining of
               Left err -> Left err
               Right remaining' -> Right (RawDocument mapLine entries, remaining')
    EventDocumentEnd ->
      -- Empty document -- return empty mapping
      Right (RawDocument (yamlLine (yamlStartMark me) + 1) [], rest)
    EventScalar _ _ _ _ ->
      -- Scalar document -- treat as degenerate (should not happen in yass)
      -- But we need to handle this gracefully
      let line = yamlLine (yamlStartMark me) + 1
      in case skipDocEnd rest of
           Left err -> Left err
           Right remaining -> Right (RawDocument line [], remaining)
    _ ->
      Left ("expected mapping at document top level", yamlLine (yamlStartMark me) + 1)

-- | Skip EventDocumentEnd
skipDocEnd :: [MarkedEvent] -> Either (Text, Int) [MarkedEvent]
skipDocEnd [] = Right []
skipDocEnd (me:rest) =
  case yamlEvent me of
    EventDocumentEnd -> Right rest
    _ -> Right (me:rest)  -- be lenient

-- | Parse a YAML mapping (after MappingStart, until MappingEnd)
parseMapping :: [MarkedEvent] -> Set.Set Text -> Either (Text, Int) ([(Text, RawValue, Int)], [MarkedEvent])
parseMapping [] _ = Left ("unexpected end of events in mapping", 0)
parseMapping (me:rest) seen =
  case yamlEvent me of
    EventMappingEnd -> Right ([], rest)
    EventScalar keyBs _tag _style _anchor ->
      let keyText = decodeScalarText keyBs
          keyLine = yamlLine (yamlStartMark me) + 1
      in if Set.member keyText seen
         then Left ("duplicate key: " <> keyText, keyLine)
         else case parseValue rest of
                Left err -> Left err
                Right (val, remaining) ->
                  case parseMapping remaining (Set.insert keyText seen) of
                    Left err -> Left err
                    Right (entries, remaining') ->
                      Right ((keyText, val, keyLine) : entries, remaining')
    _ ->
      Left ("expected scalar key in mapping", yamlLine (yamlStartMark me) + 1)

-- | Parse a YAML value
parseValue :: [MarkedEvent] -> Either (Text, Int) (RawValue, [MarkedEvent])
parseValue [] = Left ("unexpected end of events for value", 0)
parseValue (me:rest) =
  case yamlEvent me of
    EventScalar valBs tag style _anchor ->
      Right (resolveScalar valBs tag style, rest)
    EventSequenceStart _ _ _ ->
      let seqLine = yamlLine (yamlStartMark me) + 1
      in case parseSequence rest of
           Left err -> Left err
           Right (items, remaining) -> Right (RVList items seqLine, remaining)
    EventMappingStart _ _ _ ->
      let mapLine = yamlLine (yamlStartMark me) + 1
      in case parseMapping rest Set.empty of
           Left err -> Left err
           Right (entries, remaining) -> Right (RVMapping entries mapLine, remaining)
    _ ->
      Left ("unexpected event for value", yamlLine (yamlStartMark me) + 1)

-- | Parse a YAML sequence (after SequenceStart, until SequenceEnd)
parseSequence :: [MarkedEvent] -> Either (Text, Int) ([RawValue], [MarkedEvent])
parseSequence [] = Left ("unexpected end of events in sequence", 0)
parseSequence (me:rest) =
  case yamlEvent me of
    EventSequenceEnd -> Right ([], rest)
    _ -> case parseValue (me:rest) of
           Left err -> Left err
           Right (val, remaining) ->
             case parseSequence remaining of
               Left err -> Left err
               Right (vals, remaining') -> Right (val : vals, remaining')

-- | Decode a ByteString scalar to Text
decodeScalarText :: BS.ByteString -> Text
decodeScalarText = TE.decodeUtf8With TEE.lenientDecode

-- | Resolve a scalar value according to YAML 1.2 core schema rules,
-- but treating yes/no/on/off as plain strings
resolveScalar :: BS.ByteString -> Tag -> Style -> RawValue
resolveScalar bs tag style =
  let txt = decodeScalarText bs
  in case tag of
    -- Explicit tags override everything
    StrTag   -> RVString txt
    BoolTag  -> resolveBool txt
    IntTag   -> resolveNumber txt
    FloatTag -> resolveNumber txt
    NullTag  -> RVNull
    -- Quoted strings are always strings
    _ | style == SingleQuoted || style == DoubleQuoted || style == Literal || style == Folded ->
          RVString txt
    -- Plain scalars with NoTag: use core schema resolution
    NoTag -> resolvePlain txt
    -- Anything else: treat as string
    _ -> RVString txt

-- | Resolve a plain (unquoted, no explicit tag) scalar
resolvePlain :: Text -> RawValue
resolvePlain txt
  -- Null
  | txt == ""     = RVNull
  | txt == "~"    = RVNull
  | txt == "null" = RVNull
  | txt == "Null" = RVNull
  | txt == "NULL" = RVNull
  -- Booleans (YAML 1.2 core schema: only true/false)
  | txt == "true"  = RVBool True
  | txt == "True"  = RVBool True
  | txt == "TRUE"  = RVBool True
  | txt == "false" = RVBool False
  | txt == "False" = RVBool False
  | txt == "FALSE" = RVBool False
  -- yes/no/on/off are treated as strings per yass spec
  -- Numbers
  | Just n <- tryParseNumber txt = RVNumber n
  -- Otherwise: string
  | otherwise = RVString txt

-- | Resolve a boolean from text (when BoolTag is explicit)
resolveBool :: Text -> RawValue
resolveBool txt
  | txt `elem` ["true", "True", "TRUE", "yes", "Yes", "YES", "on", "On", "ON"] = RVBool True
  | txt `elem` ["false", "False", "FALSE", "no", "No", "NO", "off", "Off", "OFF"] = RVBool False
  | otherwise = RVString txt  -- fallback

-- | Try to parse a number from text
tryParseNumber :: Text -> Maybe Double
tryParseNumber txt
  | T.null txt = Nothing
  | txt == ".inf"  || txt == ".Inf"  || txt == ".INF"  = Just (1/0)
  | txt == "-.inf" || txt == "-.Inf" || txt == "-.INF" = Just (-(1/0))
  | txt == ".nan"  || txt == ".NaN"  || txt == ".NAN"  = Just (0/0)
  | otherwise =
      let s = T.unpack txt
      in case reads s :: [(Double, String)] of
           [(n, "")] -> Just n
           _ ->
             -- Try integers with various bases
             case reads s :: [(Integer, String)] of
               [(n, "")] -> Just (fromIntegral n)
               _ -> tryHexOct s

-- | Try parsing hex (0x) and octal (0o) numbers
tryHexOct :: String -> Maybe Double
tryHexOct ('0':'x':rest) = case reads ("0x" ++ rest) :: [(Integer, String)] of
  [(n, "")] -> Just (fromIntegral n)
  _ -> Nothing
tryHexOct ('0':'o':rest) = case reads ("0o" ++ rest) :: [(Integer, String)] of
  [(n, "")] -> Just (fromIntegral n)
  _ -> Nothing
tryHexOct _ = Nothing

-- | Resolve a number from text
resolveNumber :: Text -> RawValue
resolveNumber txt =
  case tryParseNumber txt of
    Just n  -> RVNumber n
    Nothing -> RVString txt
