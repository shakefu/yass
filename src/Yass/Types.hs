{-# LANGUAGE OverloadedStrings #-}
{-# LANGUAGE DeriveGeneric #-}

module Yass.Types
  ( -- * Core types
    YassFile(..)
  , Preamble(..)
  , Spec(..)
  , SlotKey(..)
  , SlotKey(..)
  , Obligation(..)
  , Normativity(..)
  , Guard(..)
  , Reference(..)
  , Relation(..)
  , RefTarget(..)
    -- * Error types
  , YassError(..)
  , ErrorCode(..)
  , errorCodeText
  , exitCodeForError
    -- * Keyword sets
  , slotKeywords
  , normativityKeywords
  , allReservedKeywords
  , isSlotKeyword
  , isNormativityKeyword
  , isReservedKeyword
  , isRelationKeyword
  , isGuardKeyword
    -- * Spec name validation
  , specNameRegex
  , validSpecNameChars
  ) where

import Data.Text (Text)
import qualified Data.Text as T
import GHC.Generics (Generic)
import Data.Set (Set)
import qualified Data.Set as Set

-- | A parsed yass file
data YassFile = YassFile
  { yassFilePath    :: !FilePath
  , yassFilePreamble :: !Preamble
  , yassFileSpecs   :: ![Spec]
  } deriving (Show, Eq, Generic)

-- | The preamble (first document)
data Preamble = Preamble
  { preambleDescription :: !Text
  , preambleVersion     :: !Text
  , preambleRelated     :: !(Maybe [Text])
  } deriving (Show, Eq, Generic)

-- | A spec document
data Spec = Spec
  { specName  :: !Text
  , specSlots :: ![(SlotKey, [Obligation])]
  , specLine  :: !Int  -- ^ 1-based line number in source
  } deriving (Show, Eq, Generic)

-- | Slot keys
data SlotKey
  = SlotINPUT
  | SlotRETURN
  | SlotERROR
  | SlotSIDEEFFECT
  | SlotINVARIANT
  deriving (Show, Eq, Ord, Enum, Bounded, Generic)

-- | An obligation within a slot
data Obligation = Obligation
  { obligationNormativity :: !(Maybe Normativity)
  , obligationGuard       :: !(Maybe Guard)
  , obligationRefs        :: ![Reference]
  , obligationLine        :: !Int  -- ^ 1-based line number
  } deriving (Show, Eq, Generic)

-- | Normativity keywords (RFC 2119)
data Normativity
  = MUST !Text
  | MUST_NOT !Text
  | SHOULD !Text
  | SHOULD_NOT !Text
  | MAY !Text
  deriving (Show, Eq, Generic)

-- | A WHEN guard
data Guard = Guard
  { guardProse :: !Text
  } deriving (Show, Eq, Generic)

-- | A reference relation
data Reference = Reference
  { refRelation :: !Relation
  , refTarget   :: !RefTarget
  , refLine     :: !Int
  } deriving (Show, Eq, Generic)

-- | Reference relation types
data Relation
  = CONFORMS
  | USES
  | SEE
  deriving (Show, Eq, Ord, Enum, Bounded, Generic)

-- | A parsed reference target
data RefTarget = RefTarget
  { refTargetRaw      :: !Text      -- ^ Original text as written
  , refTargetPath     :: !(Maybe Text) -- ^ File path component (before @)
  , refTargetSpecName :: !Text      -- ^ Spec name component
  , refTargetSlot     :: !(Maybe SlotKey) -- ^ Optional slot (after ::)
  } deriving (Show, Eq, Generic)

-- | Error codes
data ErrorCode
  -- Exit codes
  = ExitSuccess
  | ExitProcessing
  | ExitUsage
  | ExitSigint
  | ExitSigterm
  -- Argv errors
  | ArgvUnknownSubcommand
  | ArgvNoSubcommand
  | ArgvUnknownFlag
  | ArgvEmptyArgument
  | ArgvShortFlag
  | ArgvCaseMismatch
  | ArgvAbbreviation
  | ArgvMissingPositional
  | ArgvStdinDash
  -- Path errors
  | PathNotFound
  | PathBadExtension
  | PathUnreadable
  | PathInvalidType
  | PathColonInPath
  -- Glob errors
  | GlobNoMatch
  -- Discover errors
  | DiscoverNoFiles
  | DiscoverDirUnreadable
  -- FindRoot errors
  | FindrootNoMarker
  -- YAML errors
  | YamlNotUtf8
  | YamlHasBom
  | YamlMalformed
  | YamlEmptyFile
  | YamlDuplicateKey
  | YamlAnchorOrAlias
  | YamlEmptyStream
  -- Preamble errors
  | PreambleHasSpecKey
  | PreambleMissing
  | PreambleMisplaced
  | PreambleDuplicate
  | PreambleMissingDescription
  | PreambleMissingVersion
  | PreambleUnknownVersion
  | PreambleBadRelated
  -- Spec errors
  | SpecNoName
  | SpecNameNotString
  | SpecNameEmpty
  | SpecNameBadChars
  | SpecNameBadForm
  | SpecNameReserved
  | SpecUnknownKey
  | SpecDuplicateName
  -- Slot errors
  | SlotValueNotList
  -- Obligation errors
  | ObligationBadValueShape
  | ObligationMissingNormativityOrRef
  | ObligationGuardWithoutNormativity
  | ObligationDuplicateReference
  | ObligationDuplicateNormativity
  -- Normativity errors
  | NormativityUnknown
  -- Reference errors
  | ReferenceUnknownRelation
  -- Ref errors
  | RefMalformed
  | RefUnknownSlot
  | RefSlotNotDeclared
  | RefSpecNotFoundSameFile
  | RefFileNotFound
  | RefFileNotParseable
  | RefSpecNotFoundOtherFile
  -- Query errors
  | QueryNameMissing
  | QueryNameBlank
  | QueryNoMatch
  | QueryConformsUnresolved
  | QueryConformsNoSlot
  | QueryScopeNotFound
  | QueryScopeEmpty
  -- Internal errors
  | InternalUncaught
  deriving (Show, Eq, Ord, Enum, Bounded, Generic)

-- | A yass error with location information
data YassError = YassError
  { yeFile    :: !(Maybe FilePath)
  , yeLine    :: !(Maybe Int)
  , yeCode    :: !ErrorCode
  , yeMessage :: !Text
  } deriving (Show, Eq, Generic)

-- | Convert an error code to its text representation
errorCodeText :: ErrorCode -> Text
errorCodeText ExitSuccess              = "yass.exit.success"
errorCodeText ExitProcessing           = "yass.exit.processing"
errorCodeText ExitUsage                = "yass.exit.usage"
errorCodeText ExitSigint               = "yass.exit.sigint"
errorCodeText ExitSigterm              = "yass.exit.sigterm"
errorCodeText ArgvUnknownSubcommand    = "yass.argv.unknown_subcommand"
errorCodeText ArgvNoSubcommand         = "yass.argv.no_subcommand"
errorCodeText ArgvUnknownFlag          = "yass.argv.unknown_flag"
errorCodeText ArgvEmptyArgument        = "yass.argv.empty_argument"
errorCodeText ArgvShortFlag            = "yass.argv.short_flag"
errorCodeText ArgvCaseMismatch         = "yass.argv.case_mismatch"
errorCodeText ArgvAbbreviation         = "yass.argv.abbreviation"
errorCodeText ArgvMissingPositional    = "yass.argv.missing_positional"
errorCodeText ArgvStdinDash            = "yass.argv.stdin_dash"
errorCodeText PathNotFound             = "yass.path.not_found"
errorCodeText PathBadExtension         = "yass.path.bad_extension"
errorCodeText PathUnreadable           = "yass.path.unreadable"
errorCodeText PathInvalidType          = "yass.path.invalid_type"
errorCodeText PathColonInPath          = "yass.path.colon_in_path"
errorCodeText GlobNoMatch              = "yass.glob.no_match"
errorCodeText DiscoverNoFiles          = "yass.discover.no_files"
errorCodeText DiscoverDirUnreadable    = "yass.discover.dir_unreadable"
errorCodeText FindrootNoMarker         = "yass.findroot.no_marker"
errorCodeText YamlNotUtf8              = "yass.yaml.not_utf8"
errorCodeText YamlHasBom               = "yass.yaml.has_bom"
errorCodeText YamlMalformed            = "yass.yaml.malformed"
errorCodeText YamlEmptyFile            = "yass.yaml.empty_file"
errorCodeText YamlDuplicateKey         = "yass.yaml.duplicate_key"
errorCodeText YamlAnchorOrAlias        = "yass.yaml.anchor_or_alias"
errorCodeText YamlEmptyStream          = "yass.yaml.empty_stream"
errorCodeText PreambleHasSpecKey       = "yass.preamble.has_spec_key"
errorCodeText PreambleMissing          = "yass.preamble.missing"
errorCodeText PreambleMisplaced        = "yass.preamble.misplaced"
errorCodeText PreambleDuplicate        = "yass.preamble.duplicate"
errorCodeText PreambleMissingDescription = "yass.preamble.missing_description"
errorCodeText PreambleMissingVersion   = "yass.preamble.missing_version"
errorCodeText PreambleUnknownVersion   = "yass.preamble.unknown_version"
errorCodeText PreambleBadRelated       = "yass.preamble.bad_related"
errorCodeText SpecNoName               = "yass.spec.no_name"
errorCodeText SpecNameNotString        = "yass.spec.name_not_string"
errorCodeText SpecNameEmpty            = "yass.spec.name_empty"
errorCodeText SpecNameBadChars         = "yass.spec.name_bad_chars"
errorCodeText SpecNameBadForm          = "yass.spec.name_bad_form"
errorCodeText SpecNameReserved         = "yass.spec.name_reserved"
errorCodeText SpecUnknownKey           = "yass.spec.unknown_key"
errorCodeText SpecDuplicateName        = "yass.spec.duplicate_name"
errorCodeText SlotValueNotList         = "yass.slot.value_not_list"
errorCodeText ObligationBadValueShape  = "yass.obligation.bad_value_shape"
errorCodeText ObligationMissingNormativityOrRef = "yass.obligation.missing_normativity_or_ref"
errorCodeText ObligationGuardWithoutNormativity = "yass.obligation.guard_without_normativity"
errorCodeText ObligationDuplicateReference = "yass.obligation.duplicate_reference"
errorCodeText ObligationDuplicateNormativity = "yass.obligation.duplicate_normativity"
errorCodeText NormativityUnknown       = "yass.normativity.unknown"
errorCodeText ReferenceUnknownRelation = "yass.reference.unknown_relation"
errorCodeText RefMalformed             = "yass.ref.malformed"
errorCodeText RefUnknownSlot           = "yass.ref.unknown_slot"
errorCodeText RefSlotNotDeclared       = "yass.ref.slot_not_declared"
errorCodeText RefSpecNotFoundSameFile  = "yass.ref.spec_not_found_same_file"
errorCodeText RefFileNotFound          = "yass.ref.file_not_found"
errorCodeText RefFileNotParseable      = "yass.ref.file_not_parseable"
errorCodeText RefSpecNotFoundOtherFile = "yass.ref.spec_not_found_other_file"
errorCodeText QueryNameMissing         = "yass.query.name_missing"
errorCodeText QueryNameBlank           = "yass.query.name_blank"
errorCodeText QueryNoMatch             = "yass.query.no_match"
errorCodeText QueryConformsUnresolved  = "yass.query.conforms_unresolved"
errorCodeText QueryConformsNoSlot      = "yass.query.conforms_no_slot"
errorCodeText QueryScopeNotFound       = "yass.query.scope_not_found"
errorCodeText QueryScopeEmpty          = "yass.query.scope_empty"
errorCodeText InternalUncaught         = "yass.internal.uncaught"

-- | Get the exit code for an error code
exitCodeForError :: ErrorCode -> Int
exitCodeForError ExitSuccess           = 0
exitCodeForError ExitSigint            = 130
exitCodeForError ExitSigterm           = 143
exitCodeForError ExitUsage             = 2
exitCodeForError ArgvUnknownSubcommand = 2
exitCodeForError ArgvNoSubcommand      = 2
exitCodeForError ArgvUnknownFlag       = 2
exitCodeForError ArgvEmptyArgument     = 2
exitCodeForError ArgvShortFlag         = 2
exitCodeForError ArgvCaseMismatch      = 2
exitCodeForError ArgvAbbreviation      = 2
exitCodeForError ArgvMissingPositional = 2
exitCodeForError ArgvStdinDash         = 2
exitCodeForError PathNotFound          = 2
exitCodeForError PathBadExtension      = 2
exitCodeForError PathUnreadable        = 2
exitCodeForError PathInvalidType       = 2
exitCodeForError PathColonInPath       = 2
exitCodeForError GlobNoMatch           = 2
exitCodeForError DiscoverNoFiles       = 2
exitCodeForError DiscoverDirUnreadable = 2  -- non-fatal during recursion
exitCodeForError FindrootNoMarker      = 2
exitCodeForError QueryNameMissing      = 2
exitCodeForError QueryNameBlank        = 2
exitCodeForError QueryScopeNotFound    = 2
exitCodeForError QueryScopeEmpty       = 2
exitCodeForError _                     = 1  -- processing errors

-- | All slot keywords (uppercase)
slotKeywords :: Set Text
slotKeywords = Set.fromList
  ["INPUT", "RETURN", "ERROR", "SIDE-EFFECT", "INVARIANT"]

-- | All normativity keywords (uppercase)
normativityKeywords :: Set Text
normativityKeywords = Set.fromList
  ["MUST", "MUST-NOT", "SHOULD", "SHOULD-NOT", "MAY"]

-- | All reserved keywords (union of slot + normativity)
allReservedKeywords :: Set Text
allReservedKeywords = Set.union slotKeywords normativityKeywords

-- | Check if a text is a slot keyword
isSlotKeyword :: Text -> Bool
isSlotKeyword = (`Set.member` slotKeywords)

-- | Check if a text is a normativity keyword
isNormativityKeyword :: Text -> Bool
isNormativityKeyword = (`Set.member` normativityKeywords)

-- | Check if a text case-insensitively matches a reserved keyword
isReservedKeyword :: Text -> Bool
isReservedKeyword t = T.toUpper t `Set.member` allReservedKeywords

-- | Check if a text is a relation keyword
isRelationKeyword :: Text -> Bool
isRelationKeyword t = t `elem` ["CONFORMS", "USES", "SEE"]

-- | Check if a text is the guard keyword
isGuardKeyword :: Text -> Bool
isGuardKeyword t = t == "WHEN"

-- | Regex pattern for valid spec names
specNameRegex :: Text
specNameRegex = "^[A-Za-z0-9_-]+(\\.[A-Za-z0-9_-]+)*$"

-- | Valid characters for spec names
validSpecNameChars :: Set Char
validSpecNameChars = Set.fromList $
  ['A'..'Z'] ++ ['a'..'z'] ++ ['0'..'9'] ++ ['.', '_', '-']
