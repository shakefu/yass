package errors

// Exit-code error codes
const (
	CodeExitSuccess    = "yass.exit.success"
	CodeExitProcessing = "yass.exit.processing"
	CodeExitUsage      = "yass.exit.usage"
	CodeExitSigInt     = "yass.exit.sigint"
	CodeExitSigTerm    = "yass.exit.sigterm"
)

// Argv error codes
const (
	CodeArgvUnknownSubcommand = "yass.argv.unknown_subcommand"
	CodeArgvNoSubcommand      = "yass.argv.no_subcommand"
	CodeArgvUnknownFlag       = "yass.argv.unknown_flag"
	CodeArgvEmptyArgument     = "yass.argv.empty_argument"
	CodeArgvShortFlag         = "yass.argv.short_flag"
	CodeArgvCaseMismatch      = "yass.argv.case_mismatch"
	CodeArgvAbbreviation      = "yass.argv.abbreviation"
	CodeArgvMissingPositional = "yass.argv.missing_positional"
	CodeArgvStdinDash         = "yass.argv.stdin_dash"
)

// Path error codes
const (
	CodePathNotFound    = "yass.path.not_found"
	CodePathBadExtension = "yass.path.bad_extension"
	CodePathUnreadable  = "yass.path.unreadable"
	CodePathInvalidType = "yass.path.invalid_type"
	CodePathColonInPath = "yass.path.colon_in_path"
)

// Glob error codes
const (
	CodeGlobNoMatch = "yass.glob.no_match"
)

// Discovery error codes
const (
	CodeDiscoverNoFiles      = "yass.discover.no_files"
	CodeDiscoverDirUnreadable = "yass.discover.dir_unreadable"
)

// FindRoot error codes
const (
	CodeFindRootNoMarker = "yass.findroot.no_marker"
)

// YAML parsing error codes
const (
	CodeYamlNotUTF8      = "yass.yaml.not_utf8"
	CodeYamlHasBOM       = "yass.yaml.has_bom"
	CodeYamlMalformed    = "yass.yaml.malformed"
	CodeYamlEmptyFile    = "yass.yaml.empty_file"
	CodeYamlDuplicateKey = "yass.yaml.duplicate_key"
	CodeYamlAnchorOrAlias = "yass.yaml.anchor_or_alias"
	CodeYamlEmptyStream  = "yass.yaml.empty_stream"
)

// Preamble error codes
const (
	CodePreambleHasSpecKey          = "yass.preamble.has_spec_key"
	CodePreambleMissing             = "yass.preamble.missing"
	CodePreambleMisplaced           = "yass.preamble.misplaced"
	CodePreambleDuplicate           = "yass.preamble.duplicate"
	CodePreambleMissingDescription  = "yass.preamble.missing_description"
	CodePreambleMissingVersion      = "yass.preamble.missing_version"
	CodePreambleUnknownVersion      = "yass.preamble.unknown_version"
	CodePreambleBadRelated          = "yass.preamble.bad_related"
)

// Spec error codes
const (
	CodeSpecNoName       = "yass.spec.no_name"
	CodeSpecNameNotString = "yass.spec.name_not_string"
	CodeSpecNameEmpty    = "yass.spec.name_empty"
	CodeSpecNameBadChars = "yass.spec.name_bad_chars"
	CodeSpecNameBadForm  = "yass.spec.name_bad_form"
	CodeSpecNameReserved = "yass.spec.name_reserved"
	CodeSpecUnknownKey   = "yass.spec.unknown_key"
	CodeSpecDuplicateName = "yass.spec.duplicate_name"
)

// Slot error codes
const (
	CodeSlotValueNotList = "yass.slot.value_not_list"
)

// Obligation error codes
const (
	CodeObligationBadValueShape          = "yass.obligation.bad_value_shape"
	CodeObligationMissingNormativityOrRef = "yass.obligation.missing_normativity_or_ref"
	CodeObligationGuardWithoutNormativity = "yass.obligation.guard_without_normativity"
	CodeObligationDuplicateReference      = "yass.obligation.duplicate_reference"
	CodeObligationDuplicateNormativity    = "yass.obligation.duplicate_normativity"
)

// Normativity error codes
const (
	CodeNormativityUnknown = "yass.normativity.unknown"
)

// Reference error codes
const (
	CodeReferenceUnknownRelation = "yass.reference.unknown_relation"
)

// Ref error codes
const (
	CodeRefMalformed          = "yass.ref.malformed"
	CodeRefUnknownSlot        = "yass.ref.unknown_slot"
	CodeRefSlotNotDeclared    = "yass.ref.slot_not_declared"
	CodeRefSpecNotFoundSameFile  = "yass.ref.spec_not_found_same_file"
	CodeRefFileNotFound       = "yass.ref.file_not_found"
	CodeRefFileNotParseable   = "yass.ref.file_not_parseable"
	CodeRefSpecNotFoundOtherFile = "yass.ref.spec_not_found_other_file"
)

// Query error codes
const (
	CodeQueryNameMissing       = "yass.query.name_missing"
	CodeQueryNameBlank         = "yass.query.name_blank"
	CodeQueryNoMatch           = "yass.query.no_match"
	CodeQueryConformsUnresolved = "yass.query.conforms_unresolved"
	CodeQueryConformsNoSlot    = "yass.query.conforms_no_slot"
	CodeQueryScopeNotFound     = "yass.query.scope_not_found"
	CodeQueryScopeEmpty        = "yass.query.scope_empty"
)

// Internal error codes
const (
	CodeInternalUncaught = "yass.internal.uncaught"
)

// Messages maps each error code to its canonical message template.
var Messages = map[string]string{
	// Exit codes
	CodeExitSuccess:    "success",
	CodeExitProcessing: "processing error",
	CodeExitUsage:      "usage error",
	CodeExitSigInt:     "interrupted",
	CodeExitSigTerm:    "terminated",

	// Argv errors
	CodeArgvUnknownSubcommand: "unknown subcommand: %s",
	CodeArgvNoSubcommand:      "no subcommand given",
	CodeArgvUnknownFlag:       "unknown flag: %s",
	CodeArgvEmptyArgument:     "empty argument",
	CodeArgvShortFlag:         "short-form flags are not supported in v1: %s",
	CodeArgvCaseMismatch:      "subcommand or flag case mismatch: %s",
	CodeArgvAbbreviation:      "abbreviations are not supported: %s",
	CodeArgvMissingPositional: "missing required argument: %s",
	CodeArgvStdinDash:         "stdin marker `-` is not supported; pass a file path",

	// Path errors
	CodePathNotFound:    "path does not exist: %s",
	CodePathBadExtension: "expected a .yass.yaml file: %s",
	CodePathUnreadable:  "cannot read path: %s",
	CodePathInvalidType: "path is neither a file nor a directory: %s",
	CodePathColonInPath: "path contains an unsupported colon character: %s",

	// Glob errors
	CodeGlobNoMatch: "no files matched pattern: %s",

	// Discovery errors
	CodeDiscoverNoFiles:      "no .yass.yaml files found",
	CodeDiscoverDirUnreadable: "cannot read directory: %s",

	// FindRoot errors
	CodeFindRootNoMarker: "no project root marker found",

	// YAML errors
	CodeYamlNotUTF8:      "file is not valid UTF-8",
	CodeYamlHasBOM:       "file begins with a UTF-8 BOM",
	CodeYamlMalformed:    "YAML well-formedness error",
	CodeYamlEmptyFile:    "empty file",
	CodeYamlDuplicateKey: "duplicate mapping key: %s",
	CodeYamlAnchorOrAlias: "YAML anchors, aliases, and explicit tags are not allowed",
	CodeYamlEmptyStream:  "YAML stream contains no documents",

	// Preamble errors
	CodePreambleHasSpecKey:          "first document must be a Preamble, not a Spec",
	CodePreambleMissing:             "missing Preamble",
	CodePreambleMisplaced:           "Preamble must be the first document",
	CodePreambleDuplicate:           "more than one Preamble in file",
	CodePreambleMissingDescription:  "Preamble missing description",
	CodePreambleMissingVersion:      "Preamble missing version",
	CodePreambleUnknownVersion:      "unsupported Preamble version: %s",
	CodePreambleBadRelated:          "Preamble related must be a sequence of strings",

	// Spec errors
	CodeSpecNoName:       "spec document missing spec key",
	CodeSpecNameNotString: "spec name must be a string",
	CodeSpecNameEmpty:    "spec name is empty",
	CodeSpecNameBadChars: "spec name contains disallowed characters: %s",
	CodeSpecNameBadForm:  "spec name is malformed: %s",
	CodeSpecNameReserved: "spec name collides with a reserved keyword: %s",
	CodeSpecUnknownKey:   "unknown spec key: %s",
	CodeSpecDuplicateName: "duplicate spec name in file: %s",

	// Slot errors
	CodeSlotValueNotList: "slot value must be a list: %s",

	// Obligation errors
	CodeObligationBadValueShape:          "obligation value must be a quoted scalar",
	CodeObligationMissingNormativityOrRef: "obligation must carry a Normativity keyword or a Reference",
	CodeObligationGuardWithoutNormativity: "WHEN guard requires a Normativity keyword",
	CodeObligationDuplicateReference:      "duplicate Reference relation in obligation: %s",
	CodeObligationDuplicateNormativity:    "duplicate Normativity keyword in obligation",

	// Normativity errors
	CodeNormativityUnknown: "unknown Normativity keyword: %s",

	// Reference errors
	CodeReferenceUnknownRelation: "unknown Reference relation: %s",

	// Ref errors
	CodeRefMalformed:          "malformed ref target: %s",
	CodeRefUnknownSlot:        "unknown slot in ref target: %s",
	CodeRefSlotNotDeclared:    "referenced spec does not declare slot: %s",
	CodeRefSpecNotFoundSameFile:  "spec not found in file: %s",
	CodeRefFileNotFound:       "referenced file not found: %s",
	CodeRefFileNotParseable:   "referenced file not parseable: %s",
	CodeRefSpecNotFoundOtherFile: "spec not found in referenced file: %s",

	// Query errors
	CodeQueryNameMissing:       "missing spec name",
	CodeQueryNameBlank:         "spec name is blank or contains whitespace",
	CodeQueryNoMatch:           "no spec matches: %s",
	CodeQueryConformsUnresolved: "unresolvable CONFORMS ref: %s",
	CodeQueryConformsNoSlot:    "CONFORMS ref must address a slot in v1: %s",
	CodeQueryScopeNotFound:     "scope path does not exist: %s",
	CodeQueryScopeEmpty:        "no .yass.yaml files found in scope: %s",

	// Internal errors
	CodeInternalUncaught: "internal error: %s",
}
