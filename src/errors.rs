// Machine-stable error code constants for the yass CLI.
// Every ErrorLine carries one of these codes in [<code>].
// Codes are case-sensitive, stable across releases.

// ---------------------------------------------------------------------------
// Exit codes
// ---------------------------------------------------------------------------

/// Run completed without error.
pub const EXIT_SUCCESS: i32 = 0;
/// A validation or processing rule was violated.
pub const EXIT_PROCESSING: i32 = 1;
/// An argv-parse or file-input failure.
pub const EXIT_USAGE: i32 = 2;
/// Process received SIGINT.
pub const EXIT_SIGINT: i32 = 130;
/// Process received SIGTERM.
pub const EXIT_SIGTERM: i32 = 143;

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.exit.*
// ---------------------------------------------------------------------------

pub const EXIT_SUCCESS_CODE: &str = "yass.exit.success";
pub const EXIT_PROCESSING_CODE: &str = "yass.exit.processing";
pub const EXIT_USAGE_CODE: &str = "yass.exit.usage";
pub const EXIT_SIGINT_CODE: &str = "yass.exit.sigint";
pub const EXIT_SIGTERM_CODE: &str = "yass.exit.sigterm";

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.argv.*
// ---------------------------------------------------------------------------

pub const ARGV_UNKNOWN_SUBCOMMAND: &str = "yass.argv.unknown_subcommand";
pub const ARGV_NO_SUBCOMMAND: &str = "yass.argv.no_subcommand";
pub const ARGV_UNKNOWN_FLAG: &str = "yass.argv.unknown_flag";
pub const ARGV_EMPTY_ARGUMENT: &str = "yass.argv.empty_argument";
pub const ARGV_SHORT_FLAG: &str = "yass.argv.short_flag";
pub const ARGV_CASE_MISMATCH: &str = "yass.argv.case_mismatch";
pub const ARGV_ABBREVIATION: &str = "yass.argv.abbreviation";
pub const ARGV_MISSING_POSITIONAL: &str = "yass.argv.missing_positional";
pub const ARGV_STDIN_DASH: &str = "yass.argv.stdin_dash";
// Not in spec but used by existing argv.rs code:
pub const ARGV_TOO_MANY_ARGUMENTS: &str = "yass.argv.too_many_arguments";

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.path.*
// ---------------------------------------------------------------------------

pub const PATH_NOT_FOUND: &str = "yass.path.not_found";
pub const PATH_BAD_EXTENSION: &str = "yass.path.bad_extension";
pub const PATH_UNREADABLE: &str = "yass.path.unreadable";
pub const PATH_INVALID_TYPE: &str = "yass.path.invalid_type";
pub const PATH_COLON_IN_PATH: &str = "yass.path.colon_in_path";

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.glob.*
// ---------------------------------------------------------------------------

pub const GLOB_NO_MATCH: &str = "yass.glob.no_match";

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.discover.*
// ---------------------------------------------------------------------------

pub const DISCOVER_NO_FILES: &str = "yass.discover.no_files";
pub const DISCOVER_DIR_UNREADABLE: &str = "yass.discover.dir_unreadable";

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.findroot.*
// ---------------------------------------------------------------------------

pub const FINDROOT_NO_MARKER: &str = "yass.findroot.no_marker";

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.yaml.*
// ---------------------------------------------------------------------------

pub const YAML_NOT_UTF8: &str = "yass.yaml.not_utf8";
pub const YAML_HAS_BOM: &str = "yass.yaml.has_bom";
pub const YAML_MALFORMED: &str = "yass.yaml.malformed";
pub const YAML_EMPTY_FILE: &str = "yass.yaml.empty_file";
pub const YAML_DUPLICATE_KEY: &str = "yass.yaml.duplicate_key";
pub const YAML_ANCHOR_OR_ALIAS: &str = "yass.yaml.anchor_or_alias";
pub const YAML_EMPTY_STREAM: &str = "yass.yaml.empty_stream";

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.preamble.*
// ---------------------------------------------------------------------------

pub const PREAMBLE_HAS_SPEC_KEY: &str = "yass.preamble.has_spec_key";
pub const PREAMBLE_MISSING: &str = "yass.preamble.missing";
pub const PREAMBLE_MISPLACED: &str = "yass.preamble.misplaced";
pub const PREAMBLE_DUPLICATE: &str = "yass.preamble.duplicate";
pub const PREAMBLE_MISSING_DESCRIPTION: &str = "yass.preamble.missing_description";
pub const PREAMBLE_MISSING_VERSION: &str = "yass.preamble.missing_version";
pub const PREAMBLE_UNKNOWN_VERSION: &str = "yass.preamble.unknown_version";
pub const PREAMBLE_BAD_RELATED: &str = "yass.preamble.bad_related";

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.spec.*
// ---------------------------------------------------------------------------

pub const SPEC_NO_NAME: &str = "yass.spec.no_name";
pub const SPEC_NAME_NOT_STRING: &str = "yass.spec.name_not_string";
pub const SPEC_NAME_EMPTY: &str = "yass.spec.name_empty";
pub const SPEC_NAME_BAD_CHARS: &str = "yass.spec.name_bad_chars";
pub const SPEC_NAME_BAD_FORM: &str = "yass.spec.name_bad_form";
pub const SPEC_NAME_RESERVED: &str = "yass.spec.name_reserved";
pub const SPEC_UNKNOWN_KEY: &str = "yass.spec.unknown_key";
pub const SPEC_DUPLICATE_NAME: &str = "yass.spec.duplicate_name";

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.slot.*
// ---------------------------------------------------------------------------

pub const SLOT_VALUE_NOT_LIST: &str = "yass.slot.value_not_list";

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.obligation.*
// ---------------------------------------------------------------------------

pub const OBLIGATION_BAD_VALUE_SHAPE: &str = "yass.obligation.bad_value_shape";
pub const OBLIGATION_MISSING_NORMATIVITY_OR_REF: &str =
    "yass.obligation.missing_normativity_or_ref";
pub const OBLIGATION_GUARD_WITHOUT_NORMATIVITY: &str =
    "yass.obligation.guard_without_normativity";
pub const OBLIGATION_DUPLICATE_REFERENCE: &str = "yass.obligation.duplicate_reference";
pub const OBLIGATION_DUPLICATE_NORMATIVITY: &str = "yass.obligation.duplicate_normativity";

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.normativity.*
// ---------------------------------------------------------------------------

pub const NORMATIVITY_UNKNOWN: &str = "yass.normativity.unknown";

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.reference.*
// ---------------------------------------------------------------------------

pub const REFERENCE_UNKNOWN_RELATION: &str = "yass.reference.unknown_relation";

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.ref.*
// ---------------------------------------------------------------------------

pub const REF_MALFORMED: &str = "yass.ref.malformed";
pub const REF_UNKNOWN_SLOT: &str = "yass.ref.unknown_slot";
pub const REF_SLOT_NOT_DECLARED: &str = "yass.ref.slot_not_declared";
pub const REF_SPEC_NOT_FOUND_SAME_FILE: &str = "yass.ref.spec_not_found_same_file";
pub const REF_FILE_NOT_FOUND: &str = "yass.ref.file_not_found";
pub const REF_FILE_NOT_PARSEABLE: &str = "yass.ref.file_not_parseable";
pub const REF_SPEC_NOT_FOUND_OTHER_FILE: &str = "yass.ref.spec_not_found_other_file";

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.query.*
// ---------------------------------------------------------------------------

pub const QUERY_NAME_MISSING: &str = "yass.query.name_missing";
pub const QUERY_NAME_BLANK: &str = "yass.query.name_blank";
pub const QUERY_NO_MATCH: &str = "yass.query.no_match";
pub const QUERY_CONFORMS_UNRESOLVED: &str = "yass.query.conforms_unresolved";
pub const QUERY_CONFORMS_NO_SLOT: &str = "yass.query.conforms_no_slot";
pub const QUERY_SCOPE_NOT_FOUND: &str = "yass.query.scope_not_found";
pub const QUERY_SCOPE_EMPTY: &str = "yass.query.scope_empty";

// ---------------------------------------------------------------------------
// Error code string constants  --  yass.internal.*
// ---------------------------------------------------------------------------

pub const INTERNAL_UNCAUGHT: &str = "yass.internal.uncaught";

// ---------------------------------------------------------------------------
// Complete list of all spec-defined error codes (for exhaustiveness tests)
// ---------------------------------------------------------------------------

pub const ALL_CODES: &[&str] = &[
    EXIT_SUCCESS_CODE,
    EXIT_PROCESSING_CODE,
    EXIT_USAGE_CODE,
    EXIT_SIGINT_CODE,
    EXIT_SIGTERM_CODE,
    ARGV_UNKNOWN_SUBCOMMAND,
    ARGV_NO_SUBCOMMAND,
    ARGV_UNKNOWN_FLAG,
    ARGV_EMPTY_ARGUMENT,
    ARGV_SHORT_FLAG,
    ARGV_CASE_MISMATCH,
    ARGV_ABBREVIATION,
    ARGV_MISSING_POSITIONAL,
    ARGV_STDIN_DASH,
    PATH_NOT_FOUND,
    PATH_BAD_EXTENSION,
    PATH_UNREADABLE,
    PATH_INVALID_TYPE,
    PATH_COLON_IN_PATH,
    GLOB_NO_MATCH,
    DISCOVER_NO_FILES,
    DISCOVER_DIR_UNREADABLE,
    FINDROOT_NO_MARKER,
    YAML_NOT_UTF8,
    YAML_HAS_BOM,
    YAML_MALFORMED,
    YAML_EMPTY_FILE,
    YAML_DUPLICATE_KEY,
    YAML_ANCHOR_OR_ALIAS,
    YAML_EMPTY_STREAM,
    PREAMBLE_HAS_SPEC_KEY,
    PREAMBLE_MISSING,
    PREAMBLE_MISPLACED,
    PREAMBLE_DUPLICATE,
    PREAMBLE_MISSING_DESCRIPTION,
    PREAMBLE_MISSING_VERSION,
    PREAMBLE_UNKNOWN_VERSION,
    PREAMBLE_BAD_RELATED,
    SPEC_NO_NAME,
    SPEC_NAME_NOT_STRING,
    SPEC_NAME_EMPTY,
    SPEC_NAME_BAD_CHARS,
    SPEC_NAME_BAD_FORM,
    SPEC_NAME_RESERVED,
    SPEC_UNKNOWN_KEY,
    SPEC_DUPLICATE_NAME,
    SLOT_VALUE_NOT_LIST,
    OBLIGATION_BAD_VALUE_SHAPE,
    OBLIGATION_MISSING_NORMATIVITY_OR_REF,
    OBLIGATION_GUARD_WITHOUT_NORMATIVITY,
    OBLIGATION_DUPLICATE_REFERENCE,
    OBLIGATION_DUPLICATE_NORMATIVITY,
    NORMATIVITY_UNKNOWN,
    REFERENCE_UNKNOWN_RELATION,
    REF_MALFORMED,
    REF_UNKNOWN_SLOT,
    REF_SLOT_NOT_DECLARED,
    REF_SPEC_NOT_FOUND_SAME_FILE,
    REF_FILE_NOT_FOUND,
    REF_FILE_NOT_PARSEABLE,
    REF_SPEC_NOT_FOUND_OTHER_FILE,
    QUERY_NAME_MISSING,
    QUERY_NAME_BLANK,
    QUERY_NO_MATCH,
    QUERY_CONFORMS_UNRESOLVED,
    QUERY_CONFORMS_NO_SLOT,
    QUERY_SCOPE_NOT_FOUND,
    QUERY_SCOPE_EMPTY,
    INTERNAL_UNCAUGHT,
];

// ---------------------------------------------------------------------------
// ErrorCode struct  --  pairs a code with a rendered message
// ---------------------------------------------------------------------------

/// A concrete error instance pairing a machine-stable code with a
/// human-readable message.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ErrorCode {
    pub code: &'static str,
    pub message: String,
}

impl std::fmt::Display for ErrorCode {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "[{}] {}", self.code, self.message)
    }
}

// ---------------------------------------------------------------------------
// Message constructor functions  --  one per error code
// ---------------------------------------------------------------------------

// -- exit -------------------------------------------------------------------

pub fn exit_success() -> ErrorCode {
    ErrorCode {
        code: EXIT_SUCCESS_CODE,
        message: "success".into(),
    }
}

pub fn exit_processing() -> ErrorCode {
    ErrorCode {
        code: EXIT_PROCESSING_CODE,
        message: "processing error".into(),
    }
}

pub fn exit_usage() -> ErrorCode {
    ErrorCode {
        code: EXIT_USAGE_CODE,
        message: "usage error".into(),
    }
}

pub fn exit_sigint() -> ErrorCode {
    ErrorCode {
        code: EXIT_SIGINT_CODE,
        message: "received SIGINT".into(),
    }
}

pub fn exit_sigterm() -> ErrorCode {
    ErrorCode {
        code: EXIT_SIGTERM_CODE,
        message: "received SIGTERM".into(),
    }
}

// -- argv -------------------------------------------------------------------

pub fn argv_unknown_subcommand(arg: &str) -> ErrorCode {
    ErrorCode {
        code: ARGV_UNKNOWN_SUBCOMMAND,
        message: format!("unknown subcommand: {arg}"),
    }
}

pub fn argv_no_subcommand() -> ErrorCode {
    ErrorCode {
        code: ARGV_NO_SUBCOMMAND,
        message: "no subcommand given".into(),
    }
}

pub fn argv_unknown_flag(flag: &str) -> ErrorCode {
    ErrorCode {
        code: ARGV_UNKNOWN_FLAG,
        message: format!("unknown flag: {flag}"),
    }
}

pub fn argv_empty_argument() -> ErrorCode {
    ErrorCode {
        code: ARGV_EMPTY_ARGUMENT,
        message: "empty argument".into(),
    }
}

pub fn argv_short_flag(flag: &str) -> ErrorCode {
    ErrorCode {
        code: ARGV_SHORT_FLAG,
        message: format!("short-form flags are not supported in v1: {flag}"),
    }
}

pub fn argv_case_mismatch(token: &str) -> ErrorCode {
    ErrorCode {
        code: ARGV_CASE_MISMATCH,
        message: format!("subcommand or flag case mismatch: {token}"),
    }
}

pub fn argv_abbreviation(token: &str) -> ErrorCode {
    ErrorCode {
        code: ARGV_ABBREVIATION,
        message: format!("abbreviations are not supported: {token}"),
    }
}

pub fn argv_missing_positional(name: &str) -> ErrorCode {
    ErrorCode {
        code: ARGV_MISSING_POSITIONAL,
        message: format!("missing required argument: {name}"),
    }
}

pub fn argv_stdin_dash() -> ErrorCode {
    ErrorCode {
        code: ARGV_STDIN_DASH,
        message: "stdin marker `-` is not supported; pass a file path".into(),
    }
}

// -- path -------------------------------------------------------------------

pub fn path_not_found(path: &str) -> ErrorCode {
    ErrorCode {
        code: PATH_NOT_FOUND,
        message: format!("path does not exist: {path}"),
    }
}

pub fn path_bad_extension(path: &str) -> ErrorCode {
    ErrorCode {
        code: PATH_BAD_EXTENSION,
        message: format!("expected a .yass.yaml file: {path}"),
    }
}

pub fn path_unreadable(path: &str) -> ErrorCode {
    ErrorCode {
        code: PATH_UNREADABLE,
        message: format!("cannot read path: {path}"),
    }
}

pub fn path_invalid_type(path: &str) -> ErrorCode {
    ErrorCode {
        code: PATH_INVALID_TYPE,
        message: format!("path is neither a file nor a directory: {path}"),
    }
}

pub fn path_colon_in_path(path: &str) -> ErrorCode {
    ErrorCode {
        code: PATH_COLON_IN_PATH,
        message: format!("path contains an unsupported colon character: {path}"),
    }
}

// -- glob -------------------------------------------------------------------

pub fn glob_no_match(pattern: &str) -> ErrorCode {
    ErrorCode {
        code: GLOB_NO_MATCH,
        message: format!("no files matched pattern: {pattern}"),
    }
}

// -- discover ---------------------------------------------------------------

pub fn discover_no_files() -> ErrorCode {
    ErrorCode {
        code: DISCOVER_NO_FILES,
        message: "no .yass.yaml files found".into(),
    }
}

pub fn discover_dir_unreadable(path: &str) -> ErrorCode {
    ErrorCode {
        code: DISCOVER_DIR_UNREADABLE,
        message: format!("cannot read directory: {path}"),
    }
}

// -- findroot ---------------------------------------------------------------

pub fn findroot_no_marker() -> ErrorCode {
    ErrorCode {
        code: FINDROOT_NO_MARKER,
        message: "no project root marker found".into(),
    }
}

// -- yaml -------------------------------------------------------------------

pub fn yaml_not_utf8() -> ErrorCode {
    ErrorCode {
        code: YAML_NOT_UTF8,
        message: "file is not valid UTF-8".into(),
    }
}

pub fn yaml_has_bom() -> ErrorCode {
    ErrorCode {
        code: YAML_HAS_BOM,
        message: "file begins with a UTF-8 BOM".into(),
    }
}

pub fn yaml_malformed() -> ErrorCode {
    ErrorCode {
        code: YAML_MALFORMED,
        message: "YAML well-formedness error".into(),
    }
}

pub fn yaml_empty_file() -> ErrorCode {
    ErrorCode {
        code: YAML_EMPTY_FILE,
        message: "empty file".into(),
    }
}

pub fn yaml_duplicate_key(key: &str) -> ErrorCode {
    ErrorCode {
        code: YAML_DUPLICATE_KEY,
        message: format!("duplicate mapping key: {key}"),
    }
}

pub fn yaml_anchor_or_alias() -> ErrorCode {
    ErrorCode {
        code: YAML_ANCHOR_OR_ALIAS,
        message: "YAML anchors, aliases, and explicit tags are not allowed".into(),
    }
}

pub fn yaml_empty_stream() -> ErrorCode {
    ErrorCode {
        code: YAML_EMPTY_STREAM,
        message: "YAML stream contains no documents".into(),
    }
}

// -- preamble ---------------------------------------------------------------

pub fn preamble_has_spec_key() -> ErrorCode {
    ErrorCode {
        code: PREAMBLE_HAS_SPEC_KEY,
        message: "first document must be a Preamble, not a Spec".into(),
    }
}

pub fn preamble_missing() -> ErrorCode {
    ErrorCode {
        code: PREAMBLE_MISSING,
        message: "missing Preamble".into(),
    }
}

pub fn preamble_misplaced() -> ErrorCode {
    ErrorCode {
        code: PREAMBLE_MISPLACED,
        message: "Preamble must be the first document".into(),
    }
}

pub fn preamble_duplicate() -> ErrorCode {
    ErrorCode {
        code: PREAMBLE_DUPLICATE,
        message: "more than one Preamble in file".into(),
    }
}

pub fn preamble_missing_description() -> ErrorCode {
    ErrorCode {
        code: PREAMBLE_MISSING_DESCRIPTION,
        message: "Preamble missing description".into(),
    }
}

pub fn preamble_missing_version() -> ErrorCode {
    ErrorCode {
        code: PREAMBLE_MISSING_VERSION,
        message: "Preamble missing version".into(),
    }
}

pub fn preamble_unknown_version(version: &str) -> ErrorCode {
    ErrorCode {
        code: PREAMBLE_UNKNOWN_VERSION,
        message: format!("unsupported Preamble version: {version}"),
    }
}

pub fn preamble_bad_related() -> ErrorCode {
    ErrorCode {
        code: PREAMBLE_BAD_RELATED,
        message: "Preamble related must be a sequence of strings".into(),
    }
}

// -- spec -------------------------------------------------------------------

pub fn spec_no_name() -> ErrorCode {
    ErrorCode {
        code: SPEC_NO_NAME,
        message: "spec document missing spec key".into(),
    }
}

pub fn spec_name_not_string() -> ErrorCode {
    ErrorCode {
        code: SPEC_NAME_NOT_STRING,
        message: "spec name must be a string".into(),
    }
}

pub fn spec_name_empty() -> ErrorCode {
    ErrorCode {
        code: SPEC_NAME_EMPTY,
        message: "spec name is empty".into(),
    }
}

pub fn spec_name_bad_chars(name: &str) -> ErrorCode {
    ErrorCode {
        code: SPEC_NAME_BAD_CHARS,
        message: format!("spec name contains disallowed characters: {name}"),
    }
}

pub fn spec_name_bad_form(name: &str) -> ErrorCode {
    ErrorCode {
        code: SPEC_NAME_BAD_FORM,
        message: format!("spec name is malformed: {name}"),
    }
}

pub fn spec_name_reserved(name: &str) -> ErrorCode {
    ErrorCode {
        code: SPEC_NAME_RESERVED,
        message: format!("spec name collides with a reserved keyword: {name}"),
    }
}

pub fn spec_unknown_key(key: &str) -> ErrorCode {
    ErrorCode {
        code: SPEC_UNKNOWN_KEY,
        message: format!("unknown spec key: {key}"),
    }
}

pub fn spec_duplicate_name(name: &str) -> ErrorCode {
    ErrorCode {
        code: SPEC_DUPLICATE_NAME,
        message: format!("duplicate spec name in file: {name}"),
    }
}

// -- slot -------------------------------------------------------------------

pub fn slot_value_not_list(slot: &str) -> ErrorCode {
    ErrorCode {
        code: SLOT_VALUE_NOT_LIST,
        message: format!("slot value must be a list: {slot}"),
    }
}

// -- obligation -------------------------------------------------------------

pub fn obligation_bad_value_shape() -> ErrorCode {
    ErrorCode {
        code: OBLIGATION_BAD_VALUE_SHAPE,
        message: "obligation value must be a quoted scalar".into(),
    }
}

pub fn obligation_missing_normativity_or_ref() -> ErrorCode {
    ErrorCode {
        code: OBLIGATION_MISSING_NORMATIVITY_OR_REF,
        message: "obligation must carry a Normativity keyword or a Reference".into(),
    }
}

pub fn obligation_guard_without_normativity() -> ErrorCode {
    ErrorCode {
        code: OBLIGATION_GUARD_WITHOUT_NORMATIVITY,
        message: "WHEN guard requires a Normativity keyword".into(),
    }
}

pub fn obligation_duplicate_reference(relation: &str) -> ErrorCode {
    ErrorCode {
        code: OBLIGATION_DUPLICATE_REFERENCE,
        message: format!("duplicate Reference relation in obligation: {relation}"),
    }
}

pub fn obligation_duplicate_normativity() -> ErrorCode {
    ErrorCode {
        code: OBLIGATION_DUPLICATE_NORMATIVITY,
        message: "duplicate Normativity keyword in obligation".into(),
    }
}

// -- normativity ------------------------------------------------------------

pub fn normativity_unknown(keyword: &str) -> ErrorCode {
    ErrorCode {
        code: NORMATIVITY_UNKNOWN,
        message: format!("unknown Normativity keyword: {keyword}"),
    }
}

// -- reference --------------------------------------------------------------

pub fn reference_unknown_relation(relation: &str) -> ErrorCode {
    ErrorCode {
        code: REFERENCE_UNKNOWN_RELATION,
        message: format!("unknown Reference relation: {relation}"),
    }
}

// -- ref --------------------------------------------------------------------

pub fn ref_malformed(target: &str) -> ErrorCode {
    ErrorCode {
        code: REF_MALFORMED,
        message: format!("malformed ref target: {target}"),
    }
}

pub fn ref_unknown_slot(slot: &str) -> ErrorCode {
    ErrorCode {
        code: REF_UNKNOWN_SLOT,
        message: format!("unknown slot in ref target: {slot}"),
    }
}

pub fn ref_slot_not_declared(target: &str) -> ErrorCode {
    ErrorCode {
        code: REF_SLOT_NOT_DECLARED,
        message: format!("referenced spec does not declare slot: {target}"),
    }
}

pub fn ref_spec_not_found_same_file(target: &str) -> ErrorCode {
    ErrorCode {
        code: REF_SPEC_NOT_FOUND_SAME_FILE,
        message: format!("spec not found in file: {target}"),
    }
}

pub fn ref_file_not_found(target: &str) -> ErrorCode {
    ErrorCode {
        code: REF_FILE_NOT_FOUND,
        message: format!("referenced file not found: {target}"),
    }
}

pub fn ref_file_not_parseable(target: &str) -> ErrorCode {
    ErrorCode {
        code: REF_FILE_NOT_PARSEABLE,
        message: format!("referenced file not parseable: {target}"),
    }
}

pub fn ref_spec_not_found_other_file(target: &str) -> ErrorCode {
    ErrorCode {
        code: REF_SPEC_NOT_FOUND_OTHER_FILE,
        message: format!("spec not found in referenced file: {target}"),
    }
}

// -- query ------------------------------------------------------------------

pub fn query_name_missing() -> ErrorCode {
    ErrorCode {
        code: QUERY_NAME_MISSING,
        message: "missing spec name".into(),
    }
}

pub fn query_name_blank() -> ErrorCode {
    ErrorCode {
        code: QUERY_NAME_BLANK,
        message: "spec name is blank or contains whitespace".into(),
    }
}

pub fn query_no_match(name: &str) -> ErrorCode {
    ErrorCode {
        code: QUERY_NO_MATCH,
        message: format!("no spec matches: {name}"),
    }
}

pub fn query_conforms_unresolved(target: &str) -> ErrorCode {
    ErrorCode {
        code: QUERY_CONFORMS_UNRESOLVED,
        message: format!("unresolvable CONFORMS ref: {target}"),
    }
}

pub fn query_conforms_no_slot(target: &str) -> ErrorCode {
    ErrorCode {
        code: QUERY_CONFORMS_NO_SLOT,
        message: format!("CONFORMS ref must address a slot in v1: {target}"),
    }
}

pub fn query_scope_not_found(path: &str) -> ErrorCode {
    ErrorCode {
        code: QUERY_SCOPE_NOT_FOUND,
        message: format!("scope path does not exist: {path}"),
    }
}

pub fn query_scope_empty(path: &str) -> ErrorCode {
    ErrorCode {
        code: QUERY_SCOPE_EMPTY,
        message: format!("no .yass.yaml files found in scope: {path}"),
    }
}

// -- internal ---------------------------------------------------------------

pub fn internal_uncaught(message: &str) -> ErrorCode {
    ErrorCode {
        code: INTERNAL_UNCAUGHT,
        message: format!("internal error: {message}"),
    }
}

// ---------------------------------------------------------------------------
// message_for  --  returns the message template for a given error code
// ---------------------------------------------------------------------------

/// Returns a static message template string for the given error code.
/// Parameterized messages use `<placeholder>` tokens.
pub fn message_for(code: &str) -> &'static str {
    match code {
        EXIT_SUCCESS_CODE => "success",
        EXIT_PROCESSING_CODE => "processing error",
        EXIT_USAGE_CODE => "usage error",
        EXIT_SIGINT_CODE => "received SIGINT",
        EXIT_SIGTERM_CODE => "received SIGTERM",

        ARGV_UNKNOWN_SUBCOMMAND => "unknown subcommand: <arg>",
        ARGV_NO_SUBCOMMAND => "no subcommand given",
        ARGV_UNKNOWN_FLAG => "unknown flag: <flag>",
        ARGV_EMPTY_ARGUMENT => "empty argument",
        ARGV_SHORT_FLAG => "short-form flags are not supported in v1: <flag>",
        ARGV_CASE_MISMATCH => "subcommand or flag case mismatch: <token>",
        ARGV_ABBREVIATION => "abbreviations are not supported: <token>",
        ARGV_MISSING_POSITIONAL => "missing required argument: <name>",
        ARGV_STDIN_DASH => "stdin marker `-` is not supported; pass a file path",

        PATH_NOT_FOUND => "path does not exist: <path>",
        PATH_BAD_EXTENSION => "expected a .yass.yaml file: <path>",
        PATH_UNREADABLE => "cannot read path: <path>",
        PATH_INVALID_TYPE => "path is neither a file nor a directory: <path>",
        PATH_COLON_IN_PATH => "path contains an unsupported colon character: <path>",

        GLOB_NO_MATCH => "no files matched pattern: <pattern>",

        DISCOVER_NO_FILES => "no .yass.yaml files found",
        DISCOVER_DIR_UNREADABLE => "cannot read directory: <path>",

        FINDROOT_NO_MARKER => "no project root marker found",

        YAML_NOT_UTF8 => "file is not valid UTF-8",
        YAML_HAS_BOM => "file begins with a UTF-8 BOM",
        YAML_MALFORMED => "YAML well-formedness error",
        YAML_EMPTY_FILE => "empty file",
        YAML_DUPLICATE_KEY => "duplicate mapping key: <key>",
        YAML_ANCHOR_OR_ALIAS => "YAML anchors, aliases, and explicit tags are not allowed",
        YAML_EMPTY_STREAM => "YAML stream contains no documents",

        PREAMBLE_HAS_SPEC_KEY => "first document must be a Preamble, not a Spec",
        PREAMBLE_MISSING => "missing Preamble",
        PREAMBLE_MISPLACED => "Preamble must be the first document",
        PREAMBLE_DUPLICATE => "more than one Preamble in file",
        PREAMBLE_MISSING_DESCRIPTION => "Preamble missing description",
        PREAMBLE_MISSING_VERSION => "Preamble missing version",
        PREAMBLE_UNKNOWN_VERSION => "unsupported Preamble version: <version>",
        PREAMBLE_BAD_RELATED => "Preamble related must be a sequence of strings",

        SPEC_NO_NAME => "spec document missing spec key",
        SPEC_NAME_NOT_STRING => "spec name must be a string",
        SPEC_NAME_EMPTY => "spec name is empty",
        SPEC_NAME_BAD_CHARS => "spec name contains disallowed characters: <name>",
        SPEC_NAME_BAD_FORM => "spec name is malformed: <name>",
        SPEC_NAME_RESERVED => "spec name collides with a reserved keyword: <name>",
        SPEC_UNKNOWN_KEY => "unknown spec key: <key>",
        SPEC_DUPLICATE_NAME => "duplicate spec name in file: <name>",

        SLOT_VALUE_NOT_LIST => "slot value must be a list: <slot>",

        OBLIGATION_BAD_VALUE_SHAPE => "obligation value must be a quoted scalar",
        OBLIGATION_MISSING_NORMATIVITY_OR_REF => {
            "obligation must carry a Normativity keyword or a Reference"
        }
        OBLIGATION_GUARD_WITHOUT_NORMATIVITY => "WHEN guard requires a Normativity keyword",
        OBLIGATION_DUPLICATE_REFERENCE => {
            "duplicate Reference relation in obligation: <relation>"
        }
        OBLIGATION_DUPLICATE_NORMATIVITY => "duplicate Normativity keyword in obligation",

        NORMATIVITY_UNKNOWN => "unknown Normativity keyword: <keyword>",

        REFERENCE_UNKNOWN_RELATION => "unknown Reference relation: <relation>",

        REF_MALFORMED => "malformed ref target: <target>",
        REF_UNKNOWN_SLOT => "unknown slot in ref target: <slot>",
        REF_SLOT_NOT_DECLARED => "referenced spec does not declare slot: <target>",
        REF_SPEC_NOT_FOUND_SAME_FILE => "spec not found in file: <target>",
        REF_FILE_NOT_FOUND => "referenced file not found: <target>",
        REF_FILE_NOT_PARSEABLE => "referenced file not parseable: <target>",
        REF_SPEC_NOT_FOUND_OTHER_FILE => "spec not found in referenced file: <target>",

        QUERY_NAME_MISSING => "missing spec name",
        QUERY_NAME_BLANK => "spec name is blank or contains whitespace",
        QUERY_NO_MATCH => "no spec matches: <name>",
        QUERY_CONFORMS_UNRESOLVED => "unresolvable CONFORMS ref: <target>",
        QUERY_CONFORMS_NO_SLOT => "CONFORMS ref must address a slot in v1: <target>",
        QUERY_SCOPE_NOT_FOUND => "scope path does not exist: <path>",
        QUERY_SCOPE_EMPTY => "no .yass.yaml files found in scope: <path>",

        INTERNAL_UNCAUGHT => "internal error: <message>",

        _ => "unknown error code",
    }
}

// ---------------------------------------------------------------------------
// exit_code_for  --  maps an error code string to a process exit code
// ---------------------------------------------------------------------------

/// Returns the process exit code for the given error code string.
pub fn exit_code_for(code: &str) -> i32 {
    match code {
        // exit.* -- explicit exit codes
        EXIT_SUCCESS_CODE => EXIT_SUCCESS,
        EXIT_PROCESSING_CODE => EXIT_PROCESSING,
        EXIT_USAGE_CODE => EXIT_USAGE,
        EXIT_SIGINT_CODE => EXIT_SIGINT,
        EXIT_SIGTERM_CODE => EXIT_SIGTERM,

        // argv.* -- all usage (exit 2)
        ARGV_UNKNOWN_SUBCOMMAND
        | ARGV_NO_SUBCOMMAND
        | ARGV_UNKNOWN_FLAG
        | ARGV_EMPTY_ARGUMENT
        | ARGV_SHORT_FLAG
        | ARGV_CASE_MISMATCH
        | ARGV_ABBREVIATION
        | ARGV_MISSING_POSITIONAL
        | ARGV_STDIN_DASH
        | ARGV_TOO_MANY_ARGUMENTS => EXIT_USAGE,

        // path.* -- all usage (exit 2)
        PATH_NOT_FOUND | PATH_BAD_EXTENSION | PATH_UNREADABLE | PATH_INVALID_TYPE
        | PATH_COLON_IN_PATH => EXIT_USAGE,

        // glob.* -- usage (exit 2)
        GLOB_NO_MATCH => EXIT_USAGE,

        // discover.* -- usage (exit 2)
        DISCOVER_NO_FILES | DISCOVER_DIR_UNREADABLE => EXIT_USAGE,

        // findroot.* -- usage (exit 2)
        FINDROOT_NO_MARKER => EXIT_USAGE,

        // query.* -- some are usage (exit 2), rest are processing (exit 1)
        QUERY_NAME_MISSING | QUERY_NAME_BLANK | QUERY_SCOPE_NOT_FOUND | QUERY_SCOPE_EMPTY => {
            EXIT_USAGE
        }

        // Everything else is processing (exit 1):
        // yaml.*, preamble.*, spec.*, slot.*, obligation.*, normativity.*,
        // reference.*, ref.*, query.no_match, query.conforms_*,
        // internal.uncaught
        _ => EXIT_PROCESSING,
    }
}

// ---------------------------------------------------------------------------
// Keyword constants and helpers (used by validate)
// ---------------------------------------------------------------------------

/// Known YASS slot keywords.
pub const SLOT_KEYWORDS: &[&str] = &["INPUT", "RETURN", "ERROR", "SIDE-EFFECT", "INVARIANT"];

/// Known YASS normativity keywords.
pub const NORMATIVITY_KEYWORDS: &[&str] = &["MUST", "MUST-NOT", "SHOULD", "SHOULD-NOT", "MAY"];

/// Known YASS reference relation keywords.
pub const REFERENCE_KEYWORDS: &[&str] = &["CONFORMS", "USES", "SEE"];

/// Known YASS guard keyword.
pub const GUARD_KEYWORD: &str = "WHEN";

/// All structural keywords (slots + normativity) for reserved-name checking.
pub fn is_reserved_keyword(name: &str) -> bool {
    let upper = name.to_uppercase();
    SLOT_KEYWORDS.contains(&upper.as_str())
        || NORMATIVITY_KEYWORDS.contains(&upper.as_str())
        || upper == GUARD_KEYWORD
        || REFERENCE_KEYWORDS.contains(&upper.as_str())
}

// ---------------------------------------------------------------------------
// CliError  --  rich error type used throughout the CLI
// ---------------------------------------------------------------------------

/// CLI-level error.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CliError {
    /// File path associated with the error (None -> use "yass").
    pub file: Option<String>,
    /// 1-based line number in the source file.
    pub line: Option<usize>,
    /// Dotted error code.
    pub code: &'static str,
    /// Human-readable error message.
    pub message: String,
    /// Process exit code.
    pub exit_code: i32,
}

impl CliError {
    pub fn new(code: &'static str, message: impl Into<String>) -> Self {
        Self {
            file: None,
            line: None,
            code,
            message: message.into(),
            exit_code: exit_code_for(code),
        }
    }

    pub fn with_file(mut self, file: impl Into<String>) -> Self {
        self.file = Some(file.into());
        self
    }

    pub fn with_line(mut self, line: usize) -> Self {
        self.line = Some(line);
        self
    }

    pub fn with_exit_code(mut self, code: i32) -> Self {
        self.exit_code = code;
        self
    }
}

impl std::fmt::Display for CliError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let file = self.file.as_deref().unwrap_or("yass");
        let msg = self.message.replace('\n', " ").replace('\r', " ");
        if let Some(line) = self.line {
            write!(f, "{file}:{line}: [{code}] {msg}", code = self.code)
        } else {
            write!(f, "{file}: [{code}] {msg}", code = self.code)
        }
    }
}

impl std::error::Error for CliError {}

// ===========================================================================
// Tests
// ===========================================================================

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashSet;

    // =======================================================================
    // Constant string value tests  --  verify every code has the right value
    // =======================================================================

    #[test]
    fn test_exit_code_string_constants() {
        assert_eq!(EXIT_SUCCESS_CODE, "yass.exit.success");
        assert_eq!(EXIT_PROCESSING_CODE, "yass.exit.processing");
        assert_eq!(EXIT_USAGE_CODE, "yass.exit.usage");
        assert_eq!(EXIT_SIGINT_CODE, "yass.exit.sigint");
        assert_eq!(EXIT_SIGTERM_CODE, "yass.exit.sigterm");
    }

    #[test]
    fn test_argv_code_constants() {
        assert_eq!(ARGV_UNKNOWN_SUBCOMMAND, "yass.argv.unknown_subcommand");
        assert_eq!(ARGV_NO_SUBCOMMAND, "yass.argv.no_subcommand");
        assert_eq!(ARGV_UNKNOWN_FLAG, "yass.argv.unknown_flag");
        assert_eq!(ARGV_EMPTY_ARGUMENT, "yass.argv.empty_argument");
        assert_eq!(ARGV_SHORT_FLAG, "yass.argv.short_flag");
        assert_eq!(ARGV_CASE_MISMATCH, "yass.argv.case_mismatch");
        assert_eq!(ARGV_ABBREVIATION, "yass.argv.abbreviation");
        assert_eq!(ARGV_MISSING_POSITIONAL, "yass.argv.missing_positional");
        assert_eq!(ARGV_STDIN_DASH, "yass.argv.stdin_dash");
    }

    #[test]
    fn test_path_code_constants() {
        assert_eq!(PATH_NOT_FOUND, "yass.path.not_found");
        assert_eq!(PATH_BAD_EXTENSION, "yass.path.bad_extension");
        assert_eq!(PATH_UNREADABLE, "yass.path.unreadable");
        assert_eq!(PATH_INVALID_TYPE, "yass.path.invalid_type");
        assert_eq!(PATH_COLON_IN_PATH, "yass.path.colon_in_path");
    }

    #[test]
    fn test_glob_code_constants() {
        assert_eq!(GLOB_NO_MATCH, "yass.glob.no_match");
    }

    #[test]
    fn test_discover_code_constants() {
        assert_eq!(DISCOVER_NO_FILES, "yass.discover.no_files");
        assert_eq!(DISCOVER_DIR_UNREADABLE, "yass.discover.dir_unreadable");
    }

    #[test]
    fn test_findroot_code_constants() {
        assert_eq!(FINDROOT_NO_MARKER, "yass.findroot.no_marker");
    }

    #[test]
    fn test_yaml_code_constants() {
        assert_eq!(YAML_NOT_UTF8, "yass.yaml.not_utf8");
        assert_eq!(YAML_HAS_BOM, "yass.yaml.has_bom");
        assert_eq!(YAML_MALFORMED, "yass.yaml.malformed");
        assert_eq!(YAML_EMPTY_FILE, "yass.yaml.empty_file");
        assert_eq!(YAML_DUPLICATE_KEY, "yass.yaml.duplicate_key");
        assert_eq!(YAML_ANCHOR_OR_ALIAS, "yass.yaml.anchor_or_alias");
        assert_eq!(YAML_EMPTY_STREAM, "yass.yaml.empty_stream");
    }

    #[test]
    fn test_preamble_code_constants() {
        assert_eq!(PREAMBLE_HAS_SPEC_KEY, "yass.preamble.has_spec_key");
        assert_eq!(PREAMBLE_MISSING, "yass.preamble.missing");
        assert_eq!(PREAMBLE_MISPLACED, "yass.preamble.misplaced");
        assert_eq!(PREAMBLE_DUPLICATE, "yass.preamble.duplicate");
        assert_eq!(
            PREAMBLE_MISSING_DESCRIPTION,
            "yass.preamble.missing_description"
        );
        assert_eq!(PREAMBLE_MISSING_VERSION, "yass.preamble.missing_version");
        assert_eq!(PREAMBLE_UNKNOWN_VERSION, "yass.preamble.unknown_version");
        assert_eq!(PREAMBLE_BAD_RELATED, "yass.preamble.bad_related");
    }

    #[test]
    fn test_spec_code_constants() {
        assert_eq!(SPEC_NO_NAME, "yass.spec.no_name");
        assert_eq!(SPEC_NAME_NOT_STRING, "yass.spec.name_not_string");
        assert_eq!(SPEC_NAME_EMPTY, "yass.spec.name_empty");
        assert_eq!(SPEC_NAME_BAD_CHARS, "yass.spec.name_bad_chars");
        assert_eq!(SPEC_NAME_BAD_FORM, "yass.spec.name_bad_form");
        assert_eq!(SPEC_NAME_RESERVED, "yass.spec.name_reserved");
        assert_eq!(SPEC_UNKNOWN_KEY, "yass.spec.unknown_key");
        assert_eq!(SPEC_DUPLICATE_NAME, "yass.spec.duplicate_name");
    }

    #[test]
    fn test_slot_code_constants() {
        assert_eq!(SLOT_VALUE_NOT_LIST, "yass.slot.value_not_list");
    }

    #[test]
    fn test_obligation_code_constants() {
        assert_eq!(
            OBLIGATION_BAD_VALUE_SHAPE,
            "yass.obligation.bad_value_shape"
        );
        assert_eq!(
            OBLIGATION_MISSING_NORMATIVITY_OR_REF,
            "yass.obligation.missing_normativity_or_ref"
        );
        assert_eq!(
            OBLIGATION_GUARD_WITHOUT_NORMATIVITY,
            "yass.obligation.guard_without_normativity"
        );
        assert_eq!(
            OBLIGATION_DUPLICATE_REFERENCE,
            "yass.obligation.duplicate_reference"
        );
        assert_eq!(
            OBLIGATION_DUPLICATE_NORMATIVITY,
            "yass.obligation.duplicate_normativity"
        );
    }

    #[test]
    fn test_normativity_code_constants() {
        assert_eq!(NORMATIVITY_UNKNOWN, "yass.normativity.unknown");
    }

    #[test]
    fn test_reference_code_constants() {
        assert_eq!(REFERENCE_UNKNOWN_RELATION, "yass.reference.unknown_relation");
    }

    #[test]
    fn test_ref_code_constants() {
        assert_eq!(REF_MALFORMED, "yass.ref.malformed");
        assert_eq!(REF_UNKNOWN_SLOT, "yass.ref.unknown_slot");
        assert_eq!(REF_SLOT_NOT_DECLARED, "yass.ref.slot_not_declared");
        assert_eq!(
            REF_SPEC_NOT_FOUND_SAME_FILE,
            "yass.ref.spec_not_found_same_file"
        );
        assert_eq!(REF_FILE_NOT_FOUND, "yass.ref.file_not_found");
        assert_eq!(REF_FILE_NOT_PARSEABLE, "yass.ref.file_not_parseable");
        assert_eq!(
            REF_SPEC_NOT_FOUND_OTHER_FILE,
            "yass.ref.spec_not_found_other_file"
        );
    }

    #[test]
    fn test_query_code_constants() {
        assert_eq!(QUERY_NAME_MISSING, "yass.query.name_missing");
        assert_eq!(QUERY_NAME_BLANK, "yass.query.name_blank");
        assert_eq!(QUERY_NO_MATCH, "yass.query.no_match");
        assert_eq!(QUERY_CONFORMS_UNRESOLVED, "yass.query.conforms_unresolved");
        assert_eq!(QUERY_CONFORMS_NO_SLOT, "yass.query.conforms_no_slot");
        assert_eq!(QUERY_SCOPE_NOT_FOUND, "yass.query.scope_not_found");
        assert_eq!(QUERY_SCOPE_EMPTY, "yass.query.scope_empty");
    }

    #[test]
    fn test_internal_code_constants() {
        assert_eq!(INTERNAL_UNCAUGHT, "yass.internal.uncaught");
    }

    // =======================================================================
    // Exit code numeric value tests
    // =======================================================================

    #[test]
    fn test_exit_code_values() {
        assert_eq!(EXIT_SUCCESS, 0);
        assert_eq!(EXIT_PROCESSING, 1);
        assert_eq!(EXIT_USAGE, 2);
        assert_eq!(EXIT_SIGINT, 130);
        assert_eq!(EXIT_SIGTERM, 143);
    }

    // =======================================================================
    // Code format invariants
    // =======================================================================

    #[test]
    fn test_all_codes_match_allowed_chars() {
        // The spec says [a-z0-9.] but all actual codes use underscores as
        // the intra-segment separator, so we allow [a-z0-9._].
        for code in ALL_CODES {
            assert!(!code.is_empty(), "code must not be empty");
            for c in code.chars() {
                assert!(
                    c.is_ascii_lowercase() || c.is_ascii_digit() || c == '.' || c == '_',
                    "code {code:?} contains invalid char '{c}'"
                );
            }
        }
    }

    #[test]
    fn test_all_codes_start_with_yass_prefix() {
        for code in ALL_CODES {
            assert!(
                code.starts_with("yass."),
                "code {code:?} does not start with 'yass.'"
            );
        }
    }

    #[test]
    fn test_all_codes_have_three_segments() {
        for code in ALL_CODES {
            let segments: Vec<&str> = code.split('.').collect();
            assert_eq!(
                segments.len(),
                3,
                "code {code:?} does not have exactly 3 dot-separated segments: {segments:?}"
            );
        }
    }

    #[test]
    fn test_no_duplicate_codes() {
        let mut seen = HashSet::new();
        for code in ALL_CODES {
            assert!(seen.insert(code), "duplicate code in ALL_CODES: {code:?}");
        }
    }

    #[test]
    fn test_all_codes_count() {
        // 5 exit + 9 argv + 5 path + 1 glob + 2 discover + 1 findroot
        // + 7 yaml + 8 preamble + 8 spec + 1 slot + 5 obligation
        // + 1 normativity + 1 reference + 7 ref + 7 query + 1 internal = 69
        assert_eq!(ALL_CODES.len(), 69, "expected 69 error codes in ALL_CODES");
    }

    // =======================================================================
    // exit_code_for tests  --  every category
    // =======================================================================

    #[test]
    fn test_exit_code_for_exit_codes() {
        assert_eq!(exit_code_for(EXIT_SUCCESS_CODE), 0);
        assert_eq!(exit_code_for(EXIT_PROCESSING_CODE), 1);
        assert_eq!(exit_code_for(EXIT_USAGE_CODE), 2);
        assert_eq!(exit_code_for(EXIT_SIGINT_CODE), 130);
        assert_eq!(exit_code_for(EXIT_SIGTERM_CODE), 143);
    }

    #[test]
    fn test_exit_code_for_argv_codes() {
        let codes = [
            ARGV_UNKNOWN_SUBCOMMAND,
            ARGV_NO_SUBCOMMAND,
            ARGV_UNKNOWN_FLAG,
            ARGV_EMPTY_ARGUMENT,
            ARGV_SHORT_FLAG,
            ARGV_CASE_MISMATCH,
            ARGV_ABBREVIATION,
            ARGV_MISSING_POSITIONAL,
            ARGV_STDIN_DASH,
        ];
        for code in &codes {
            assert_eq!(
                exit_code_for(code),
                EXIT_USAGE,
                "exit_code_for({code:?})"
            );
        }
    }

    #[test]
    fn test_exit_code_for_path_codes() {
        let codes = [
            PATH_NOT_FOUND,
            PATH_BAD_EXTENSION,
            PATH_UNREADABLE,
            PATH_INVALID_TYPE,
            PATH_COLON_IN_PATH,
        ];
        for code in &codes {
            assert_eq!(
                exit_code_for(code),
                EXIT_USAGE,
                "exit_code_for({code:?})"
            );
        }
    }

    #[test]
    fn test_exit_code_for_glob_codes() {
        assert_eq!(exit_code_for(GLOB_NO_MATCH), EXIT_USAGE);
    }

    #[test]
    fn test_exit_code_for_discover_codes() {
        assert_eq!(exit_code_for(DISCOVER_NO_FILES), EXIT_USAGE);
        assert_eq!(exit_code_for(DISCOVER_DIR_UNREADABLE), EXIT_USAGE);
    }

    #[test]
    fn test_exit_code_for_findroot_codes() {
        assert_eq!(exit_code_for(FINDROOT_NO_MARKER), EXIT_USAGE);
    }

    #[test]
    fn test_exit_code_for_yaml_codes() {
        let codes = [
            YAML_NOT_UTF8,
            YAML_HAS_BOM,
            YAML_MALFORMED,
            YAML_EMPTY_FILE,
            YAML_DUPLICATE_KEY,
            YAML_ANCHOR_OR_ALIAS,
            YAML_EMPTY_STREAM,
        ];
        for code in &codes {
            assert_eq!(
                exit_code_for(code),
                EXIT_PROCESSING,
                "exit_code_for({code:?})"
            );
        }
    }

    #[test]
    fn test_exit_code_for_preamble_codes() {
        let codes = [
            PREAMBLE_HAS_SPEC_KEY,
            PREAMBLE_MISSING,
            PREAMBLE_MISPLACED,
            PREAMBLE_DUPLICATE,
            PREAMBLE_MISSING_DESCRIPTION,
            PREAMBLE_MISSING_VERSION,
            PREAMBLE_UNKNOWN_VERSION,
            PREAMBLE_BAD_RELATED,
        ];
        for code in &codes {
            assert_eq!(
                exit_code_for(code),
                EXIT_PROCESSING,
                "exit_code_for({code:?})"
            );
        }
    }

    #[test]
    fn test_exit_code_for_spec_codes() {
        let codes = [
            SPEC_NO_NAME,
            SPEC_NAME_NOT_STRING,
            SPEC_NAME_EMPTY,
            SPEC_NAME_BAD_CHARS,
            SPEC_NAME_BAD_FORM,
            SPEC_NAME_RESERVED,
            SPEC_UNKNOWN_KEY,
            SPEC_DUPLICATE_NAME,
        ];
        for code in &codes {
            assert_eq!(
                exit_code_for(code),
                EXIT_PROCESSING,
                "exit_code_for({code:?})"
            );
        }
    }

    #[test]
    fn test_exit_code_for_slot_codes() {
        assert_eq!(exit_code_for(SLOT_VALUE_NOT_LIST), EXIT_PROCESSING);
    }

    #[test]
    fn test_exit_code_for_obligation_codes() {
        let codes = [
            OBLIGATION_BAD_VALUE_SHAPE,
            OBLIGATION_MISSING_NORMATIVITY_OR_REF,
            OBLIGATION_GUARD_WITHOUT_NORMATIVITY,
            OBLIGATION_DUPLICATE_REFERENCE,
            OBLIGATION_DUPLICATE_NORMATIVITY,
        ];
        for code in &codes {
            assert_eq!(
                exit_code_for(code),
                EXIT_PROCESSING,
                "exit_code_for({code:?})"
            );
        }
    }

    #[test]
    fn test_exit_code_for_normativity_codes() {
        assert_eq!(exit_code_for(NORMATIVITY_UNKNOWN), EXIT_PROCESSING);
    }

    #[test]
    fn test_exit_code_for_reference_codes() {
        assert_eq!(exit_code_for(REFERENCE_UNKNOWN_RELATION), EXIT_PROCESSING);
    }

    #[test]
    fn test_exit_code_for_ref_codes() {
        let codes = [
            REF_MALFORMED,
            REF_UNKNOWN_SLOT,
            REF_SLOT_NOT_DECLARED,
            REF_SPEC_NOT_FOUND_SAME_FILE,
            REF_FILE_NOT_FOUND,
            REF_FILE_NOT_PARSEABLE,
            REF_SPEC_NOT_FOUND_OTHER_FILE,
        ];
        for code in &codes {
            assert_eq!(
                exit_code_for(code),
                EXIT_PROCESSING,
                "exit_code_for({code:?})"
            );
        }
    }

    #[test]
    fn test_exit_code_for_query_codes() {
        // Usage (exit 2)
        assert_eq!(exit_code_for(QUERY_NAME_MISSING), EXIT_USAGE);
        assert_eq!(exit_code_for(QUERY_NAME_BLANK), EXIT_USAGE);
        assert_eq!(exit_code_for(QUERY_SCOPE_NOT_FOUND), EXIT_USAGE);
        assert_eq!(exit_code_for(QUERY_SCOPE_EMPTY), EXIT_USAGE);

        // Processing (exit 1)
        assert_eq!(exit_code_for(QUERY_NO_MATCH), EXIT_PROCESSING);
        assert_eq!(exit_code_for(QUERY_CONFORMS_UNRESOLVED), EXIT_PROCESSING);
        assert_eq!(exit_code_for(QUERY_CONFORMS_NO_SLOT), EXIT_PROCESSING);
    }

    #[test]
    fn test_exit_code_for_internal_codes() {
        assert_eq!(exit_code_for(INTERNAL_UNCAUGHT), EXIT_PROCESSING);
    }

    #[test]
    fn test_exit_code_for_unknown_code_defaults_to_processing() {
        assert_eq!(exit_code_for("yass.unknown.whatever"), EXIT_PROCESSING);
    }

    #[test]
    fn test_exit_code_for_all_codes_returns_valid_exit() {
        let valid = [
            EXIT_SUCCESS,
            EXIT_PROCESSING,
            EXIT_USAGE,
            EXIT_SIGINT,
            EXIT_SIGTERM,
        ];
        for code in ALL_CODES {
            let ec = exit_code_for(code);
            assert!(
                valid.contains(&ec),
                "exit_code_for({code:?}) returned {ec}, which is not in {{0,1,2,130,143}}"
            );
        }
    }

    // =======================================================================
    // message_for tests
    // =======================================================================

    #[test]
    fn test_message_for_all_codes_returns_non_fallback() {
        for code in ALL_CODES {
            let msg = message_for(code);
            assert!(!msg.is_empty(), "message_for({code:?}) is empty");
            assert_ne!(
                msg, "unknown error code",
                "message_for({code:?}) returned fallback"
            );
        }
    }

    #[test]
    fn test_message_for_unknown_code() {
        assert_eq!(message_for("yass.bogus.code"), "unknown error code");
    }

    #[test]
    fn test_message_for_specific_templates() {
        assert_eq!(message_for(EXIT_SUCCESS_CODE), "success");
        assert_eq!(
            message_for(ARGV_UNKNOWN_SUBCOMMAND),
            "unknown subcommand: <arg>"
        );
        assert_eq!(message_for(ARGV_NO_SUBCOMMAND), "no subcommand given");
        assert_eq!(
            message_for(ARGV_STDIN_DASH),
            "stdin marker `-` is not supported; pass a file path"
        );
        assert_eq!(
            message_for(PATH_NOT_FOUND),
            "path does not exist: <path>"
        );
        assert_eq!(
            message_for(YAML_DUPLICATE_KEY),
            "duplicate mapping key: <key>"
        );
        assert_eq!(
            message_for(PREAMBLE_UNKNOWN_VERSION),
            "unsupported Preamble version: <version>"
        );
        assert_eq!(
            message_for(SPEC_NAME_BAD_CHARS),
            "spec name contains disallowed characters: <name>"
        );
        assert_eq!(
            message_for(SLOT_VALUE_NOT_LIST),
            "slot value must be a list: <slot>"
        );
        assert_eq!(
            message_for(OBLIGATION_DUPLICATE_REFERENCE),
            "duplicate Reference relation in obligation: <relation>"
        );
        assert_eq!(
            message_for(NORMATIVITY_UNKNOWN),
            "unknown Normativity keyword: <keyword>"
        );
        assert_eq!(
            message_for(REFERENCE_UNKNOWN_RELATION),
            "unknown Reference relation: <relation>"
        );
        assert_eq!(
            message_for(REF_MALFORMED),
            "malformed ref target: <target>"
        );
        assert_eq!(message_for(QUERY_NO_MATCH), "no spec matches: <name>");
        assert_eq!(
            message_for(QUERY_CONFORMS_NO_SLOT),
            "CONFORMS ref must address a slot in v1: <target>"
        );
        assert_eq!(
            message_for(INTERNAL_UNCAUGHT),
            "internal error: <message>"
        );
    }

    // =======================================================================
    // Constructor / message template tests  --  verify exact messages
    // =======================================================================

    #[test]
    fn test_ctor_exit_success() {
        let e = exit_success();
        assert_eq!(e.code, EXIT_SUCCESS_CODE);
        assert_eq!(e.message, "success");
    }

    #[test]
    fn test_ctor_exit_processing() {
        let e = exit_processing();
        assert_eq!(e.code, EXIT_PROCESSING_CODE);
        assert_eq!(e.message, "processing error");
    }

    #[test]
    fn test_ctor_exit_usage() {
        let e = exit_usage();
        assert_eq!(e.code, EXIT_USAGE_CODE);
        assert_eq!(e.message, "usage error");
    }

    #[test]
    fn test_ctor_exit_sigint() {
        let e = exit_sigint();
        assert_eq!(e.code, EXIT_SIGINT_CODE);
        assert_eq!(e.message, "received SIGINT");
    }

    #[test]
    fn test_ctor_exit_sigterm() {
        let e = exit_sigterm();
        assert_eq!(e.code, EXIT_SIGTERM_CODE);
        assert_eq!(e.message, "received SIGTERM");
    }

    #[test]
    fn test_ctor_argv_unknown_subcommand() {
        let e = argv_unknown_subcommand("frobnicate");
        assert_eq!(e.code, ARGV_UNKNOWN_SUBCOMMAND);
        assert_eq!(e.message, "unknown subcommand: frobnicate");
    }

    #[test]
    fn test_ctor_argv_no_subcommand() {
        let e = argv_no_subcommand();
        assert_eq!(e.code, ARGV_NO_SUBCOMMAND);
        assert_eq!(e.message, "no subcommand given");
    }

    #[test]
    fn test_ctor_argv_unknown_flag() {
        let e = argv_unknown_flag("--verbose");
        assert_eq!(e.code, ARGV_UNKNOWN_FLAG);
        assert_eq!(e.message, "unknown flag: --verbose");
    }

    #[test]
    fn test_ctor_argv_empty_argument() {
        let e = argv_empty_argument();
        assert_eq!(e.code, ARGV_EMPTY_ARGUMENT);
        assert_eq!(e.message, "empty argument");
    }

    #[test]
    fn test_ctor_argv_short_flag() {
        let e = argv_short_flag("-v");
        assert_eq!(e.code, ARGV_SHORT_FLAG);
        assert_eq!(e.message, "short-form flags are not supported in v1: -v");
    }

    #[test]
    fn test_ctor_argv_case_mismatch() {
        let e = argv_case_mismatch("Validate");
        assert_eq!(e.code, ARGV_CASE_MISMATCH);
        assert_eq!(e.message, "subcommand or flag case mismatch: Validate");
    }

    #[test]
    fn test_ctor_argv_abbreviation() {
        let e = argv_abbreviation("val");
        assert_eq!(e.code, ARGV_ABBREVIATION);
        assert_eq!(e.message, "abbreviations are not supported: val");
    }

    #[test]
    fn test_ctor_argv_missing_positional() {
        let e = argv_missing_positional("spec-name");
        assert_eq!(e.code, ARGV_MISSING_POSITIONAL);
        assert_eq!(e.message, "missing required argument: spec-name");
    }

    #[test]
    fn test_ctor_argv_stdin_dash() {
        let e = argv_stdin_dash();
        assert_eq!(e.code, ARGV_STDIN_DASH);
        assert_eq!(
            e.message,
            "stdin marker `-` is not supported; pass a file path"
        );
    }

    #[test]
    fn test_ctor_path_not_found() {
        let e = path_not_found("/tmp/missing.yass.yaml");
        assert_eq!(e.code, PATH_NOT_FOUND);
        assert_eq!(e.message, "path does not exist: /tmp/missing.yass.yaml");
    }

    #[test]
    fn test_ctor_path_bad_extension() {
        let e = path_bad_extension("foo.txt");
        assert_eq!(e.code, PATH_BAD_EXTENSION);
        assert_eq!(e.message, "expected a .yass.yaml file: foo.txt");
    }

    #[test]
    fn test_ctor_path_unreadable() {
        let e = path_unreadable("/secret/file.yass.yaml");
        assert_eq!(e.code, PATH_UNREADABLE);
        assert_eq!(e.message, "cannot read path: /secret/file.yass.yaml");
    }

    #[test]
    fn test_ctor_path_invalid_type() {
        let e = path_invalid_type("/dev/null");
        assert_eq!(e.code, PATH_INVALID_TYPE);
        assert_eq!(
            e.message,
            "path is neither a file nor a directory: /dev/null"
        );
    }

    #[test]
    fn test_ctor_path_colon_in_path() {
        let e = path_colon_in_path("foo:bar.yass.yaml");
        assert_eq!(e.code, PATH_COLON_IN_PATH);
        assert_eq!(
            e.message,
            "path contains an unsupported colon character: foo:bar.yass.yaml"
        );
    }

    #[test]
    fn test_ctor_glob_no_match() {
        let e = glob_no_match("**/*.nope");
        assert_eq!(e.code, GLOB_NO_MATCH);
        assert_eq!(e.message, "no files matched pattern: **/*.nope");
    }

    #[test]
    fn test_ctor_discover_no_files() {
        let e = discover_no_files();
        assert_eq!(e.code, DISCOVER_NO_FILES);
        assert_eq!(e.message, "no .yass.yaml files found");
    }

    #[test]
    fn test_ctor_discover_dir_unreadable() {
        let e = discover_dir_unreadable("/locked/dir");
        assert_eq!(e.code, DISCOVER_DIR_UNREADABLE);
        assert_eq!(e.message, "cannot read directory: /locked/dir");
    }

    #[test]
    fn test_ctor_findroot_no_marker() {
        let e = findroot_no_marker();
        assert_eq!(e.code, FINDROOT_NO_MARKER);
        assert_eq!(e.message, "no project root marker found");
    }

    #[test]
    fn test_ctor_yaml_not_utf8() {
        let e = yaml_not_utf8();
        assert_eq!(e.code, YAML_NOT_UTF8);
        assert_eq!(e.message, "file is not valid UTF-8");
    }

    #[test]
    fn test_ctor_yaml_has_bom() {
        let e = yaml_has_bom();
        assert_eq!(e.code, YAML_HAS_BOM);
        assert_eq!(e.message, "file begins with a UTF-8 BOM");
    }

    #[test]
    fn test_ctor_yaml_malformed() {
        let e = yaml_malformed();
        assert_eq!(e.code, YAML_MALFORMED);
        assert_eq!(e.message, "YAML well-formedness error");
    }

    #[test]
    fn test_ctor_yaml_empty_file() {
        let e = yaml_empty_file();
        assert_eq!(e.code, YAML_EMPTY_FILE);
        assert_eq!(e.message, "empty file");
    }

    #[test]
    fn test_ctor_yaml_duplicate_key() {
        let e = yaml_duplicate_key("spec");
        assert_eq!(e.code, YAML_DUPLICATE_KEY);
        assert_eq!(e.message, "duplicate mapping key: spec");
    }

    #[test]
    fn test_ctor_yaml_anchor_or_alias() {
        let e = yaml_anchor_or_alias();
        assert_eq!(e.code, YAML_ANCHOR_OR_ALIAS);
        assert_eq!(
            e.message,
            "YAML anchors, aliases, and explicit tags are not allowed"
        );
    }

    #[test]
    fn test_ctor_yaml_empty_stream() {
        let e = yaml_empty_stream();
        assert_eq!(e.code, YAML_EMPTY_STREAM);
        assert_eq!(e.message, "YAML stream contains no documents");
    }

    #[test]
    fn test_ctor_preamble_has_spec_key() {
        let e = preamble_has_spec_key();
        assert_eq!(e.code, PREAMBLE_HAS_SPEC_KEY);
        assert_eq!(
            e.message,
            "first document must be a Preamble, not a Spec"
        );
    }

    #[test]
    fn test_ctor_preamble_missing() {
        let e = preamble_missing();
        assert_eq!(e.code, PREAMBLE_MISSING);
        assert_eq!(e.message, "missing Preamble");
    }

    #[test]
    fn test_ctor_preamble_misplaced() {
        let e = preamble_misplaced();
        assert_eq!(e.code, PREAMBLE_MISPLACED);
        assert_eq!(e.message, "Preamble must be the first document");
    }

    #[test]
    fn test_ctor_preamble_duplicate() {
        let e = preamble_duplicate();
        assert_eq!(e.code, PREAMBLE_DUPLICATE);
        assert_eq!(e.message, "more than one Preamble in file");
    }

    #[test]
    fn test_ctor_preamble_missing_description() {
        let e = preamble_missing_description();
        assert_eq!(e.code, PREAMBLE_MISSING_DESCRIPTION);
        assert_eq!(e.message, "Preamble missing description");
    }

    #[test]
    fn test_ctor_preamble_missing_version() {
        let e = preamble_missing_version();
        assert_eq!(e.code, PREAMBLE_MISSING_VERSION);
        assert_eq!(e.message, "Preamble missing version");
    }

    #[test]
    fn test_ctor_preamble_unknown_version() {
        let e = preamble_unknown_version("v2");
        assert_eq!(e.code, PREAMBLE_UNKNOWN_VERSION);
        assert_eq!(e.message, "unsupported Preamble version: v2");
    }

    #[test]
    fn test_ctor_preamble_bad_related() {
        let e = preamble_bad_related();
        assert_eq!(e.code, PREAMBLE_BAD_RELATED);
        assert_eq!(
            e.message,
            "Preamble related must be a sequence of strings"
        );
    }

    #[test]
    fn test_ctor_spec_no_name() {
        let e = spec_no_name();
        assert_eq!(e.code, SPEC_NO_NAME);
        assert_eq!(e.message, "spec document missing spec key");
    }

    #[test]
    fn test_ctor_spec_name_not_string() {
        let e = spec_name_not_string();
        assert_eq!(e.code, SPEC_NAME_NOT_STRING);
        assert_eq!(e.message, "spec name must be a string");
    }

    #[test]
    fn test_ctor_spec_name_empty() {
        let e = spec_name_empty();
        assert_eq!(e.code, SPEC_NAME_EMPTY);
        assert_eq!(e.message, "spec name is empty");
    }

    #[test]
    fn test_ctor_spec_name_bad_chars() {
        let e = spec_name_bad_chars("foo bar!");
        assert_eq!(e.code, SPEC_NAME_BAD_CHARS);
        assert_eq!(
            e.message,
            "spec name contains disallowed characters: foo bar!"
        );
    }

    #[test]
    fn test_ctor_spec_name_bad_form() {
        let e = spec_name_bad_form(".leading.dot");
        assert_eq!(e.code, SPEC_NAME_BAD_FORM);
        assert_eq!(e.message, "spec name is malformed: .leading.dot");
    }

    #[test]
    fn test_ctor_spec_name_reserved() {
        let e = spec_name_reserved("MUST");
        assert_eq!(e.code, SPEC_NAME_RESERVED);
        assert_eq!(
            e.message,
            "spec name collides with a reserved keyword: MUST"
        );
    }

    #[test]
    fn test_ctor_spec_unknown_key() {
        let e = spec_unknown_key("FOOBAR");
        assert_eq!(e.code, SPEC_UNKNOWN_KEY);
        assert_eq!(e.message, "unknown spec key: FOOBAR");
    }

    #[test]
    fn test_ctor_spec_duplicate_name() {
        let e = spec_duplicate_name("cli.errors");
        assert_eq!(e.code, SPEC_DUPLICATE_NAME);
        assert_eq!(e.message, "duplicate spec name in file: cli.errors");
    }

    #[test]
    fn test_ctor_slot_value_not_list() {
        let e = slot_value_not_list("INPUT");
        assert_eq!(e.code, SLOT_VALUE_NOT_LIST);
        assert_eq!(e.message, "slot value must be a list: INPUT");
    }

    #[test]
    fn test_ctor_obligation_bad_value_shape() {
        let e = obligation_bad_value_shape();
        assert_eq!(e.code, OBLIGATION_BAD_VALUE_SHAPE);
        assert_eq!(e.message, "obligation value must be a quoted scalar");
    }

    #[test]
    fn test_ctor_obligation_missing_normativity_or_ref() {
        let e = obligation_missing_normativity_or_ref();
        assert_eq!(e.code, OBLIGATION_MISSING_NORMATIVITY_OR_REF);
        assert_eq!(
            e.message,
            "obligation must carry a Normativity keyword or a Reference"
        );
    }

    #[test]
    fn test_ctor_obligation_guard_without_normativity() {
        let e = obligation_guard_without_normativity();
        assert_eq!(e.code, OBLIGATION_GUARD_WITHOUT_NORMATIVITY);
        assert_eq!(e.message, "WHEN guard requires a Normativity keyword");
    }

    #[test]
    fn test_ctor_obligation_duplicate_reference() {
        let e = obligation_duplicate_reference("CONFORMS");
        assert_eq!(e.code, OBLIGATION_DUPLICATE_REFERENCE);
        assert_eq!(
            e.message,
            "duplicate Reference relation in obligation: CONFORMS"
        );
    }

    #[test]
    fn test_ctor_obligation_duplicate_normativity() {
        let e = obligation_duplicate_normativity();
        assert_eq!(e.code, OBLIGATION_DUPLICATE_NORMATIVITY);
        assert_eq!(e.message, "duplicate Normativity keyword in obligation");
    }

    #[test]
    fn test_ctor_normativity_unknown() {
        let e = normativity_unknown("ALWAYS");
        assert_eq!(e.code, NORMATIVITY_UNKNOWN);
        assert_eq!(e.message, "unknown Normativity keyword: ALWAYS");
    }

    #[test]
    fn test_ctor_reference_unknown_relation() {
        let e = reference_unknown_relation("INHERITS");
        assert_eq!(e.code, REFERENCE_UNKNOWN_RELATION);
        assert_eq!(e.message, "unknown Reference relation: INHERITS");
    }

    #[test]
    fn test_ctor_ref_malformed() {
        let e = ref_malformed("@@@");
        assert_eq!(e.code, REF_MALFORMED);
        assert_eq!(e.message, "malformed ref target: @@@");
    }

    #[test]
    fn test_ctor_ref_unknown_slot() {
        let e = ref_unknown_slot("FOOBAR");
        assert_eq!(e.code, REF_UNKNOWN_SLOT);
        assert_eq!(e.message, "unknown slot in ref target: FOOBAR");
    }

    #[test]
    fn test_ctor_ref_slot_not_declared() {
        let e = ref_slot_not_declared("spec@cli.foo::INPUT");
        assert_eq!(e.code, REF_SLOT_NOT_DECLARED);
        assert_eq!(
            e.message,
            "referenced spec does not declare slot: spec@cli.foo::INPUT"
        );
    }

    #[test]
    fn test_ctor_ref_spec_not_found_same_file() {
        let e = ref_spec_not_found_same_file("cli.missing");
        assert_eq!(e.code, REF_SPEC_NOT_FOUND_SAME_FILE);
        assert_eq!(e.message, "spec not found in file: cli.missing");
    }

    #[test]
    fn test_ctor_ref_file_not_found() {
        let e = ref_file_not_found("./missing.yass.yaml@cli.foo");
        assert_eq!(e.code, REF_FILE_NOT_FOUND);
        assert_eq!(
            e.message,
            "referenced file not found: ./missing.yass.yaml@cli.foo"
        );
    }

    #[test]
    fn test_ctor_ref_file_not_parseable() {
        let e = ref_file_not_parseable("./bad.yass.yaml@cli.foo");
        assert_eq!(e.code, REF_FILE_NOT_PARSEABLE);
        assert_eq!(
            e.message,
            "referenced file not parseable: ./bad.yass.yaml@cli.foo"
        );
    }

    #[test]
    fn test_ctor_ref_spec_not_found_other_file() {
        let e = ref_spec_not_found_other_file("./other.yass.yaml@cli.missing");
        assert_eq!(e.code, REF_SPEC_NOT_FOUND_OTHER_FILE);
        assert_eq!(
            e.message,
            "spec not found in referenced file: ./other.yass.yaml@cli.missing"
        );
    }

    #[test]
    fn test_ctor_query_name_missing() {
        let e = query_name_missing();
        assert_eq!(e.code, QUERY_NAME_MISSING);
        assert_eq!(e.message, "missing spec name");
    }

    #[test]
    fn test_ctor_query_name_blank() {
        let e = query_name_blank();
        assert_eq!(e.code, QUERY_NAME_BLANK);
        assert_eq!(e.message, "spec name is blank or contains whitespace");
    }

    #[test]
    fn test_ctor_query_no_match() {
        let e = query_no_match("cli.nope");
        assert_eq!(e.code, QUERY_NO_MATCH);
        assert_eq!(e.message, "no spec matches: cli.nope");
    }

    #[test]
    fn test_ctor_query_conforms_unresolved() {
        let e = query_conforms_unresolved("./missing.yass.yaml@cli.foo::INPUT");
        assert_eq!(e.code, QUERY_CONFORMS_UNRESOLVED);
        assert_eq!(
            e.message,
            "unresolvable CONFORMS ref: ./missing.yass.yaml@cli.foo::INPUT"
        );
    }

    #[test]
    fn test_ctor_query_conforms_no_slot() {
        let e = query_conforms_no_slot("cli.foo");
        assert_eq!(e.code, QUERY_CONFORMS_NO_SLOT);
        assert_eq!(
            e.message,
            "CONFORMS ref must address a slot in v1: cli.foo"
        );
    }

    #[test]
    fn test_ctor_query_scope_not_found() {
        let e = query_scope_not_found("/tmp/missing");
        assert_eq!(e.code, QUERY_SCOPE_NOT_FOUND);
        assert_eq!(e.message, "scope path does not exist: /tmp/missing");
    }

    #[test]
    fn test_ctor_query_scope_empty() {
        let e = query_scope_empty("/tmp/empty");
        assert_eq!(e.code, QUERY_SCOPE_EMPTY);
        assert_eq!(
            e.message,
            "no .yass.yaml files found in scope: /tmp/empty"
        );
    }

    #[test]
    fn test_ctor_internal_uncaught() {
        let e = internal_uncaught("something broke");
        assert_eq!(e.code, INTERNAL_UNCAUGHT);
        assert_eq!(e.message, "internal error: something broke");
    }

    // =======================================================================
    // ErrorCode Display trait
    // =======================================================================

    #[test]
    fn test_error_code_display_with_arg() {
        let e = argv_unknown_flag("--debug");
        assert_eq!(
            format!("{e}"),
            "[yass.argv.unknown_flag] unknown flag: --debug"
        );
    }

    #[test]
    fn test_error_code_display_no_arg() {
        let e = preamble_missing();
        assert_eq!(format!("{e}"), "[yass.preamble.missing] missing Preamble");
    }

    // =======================================================================
    // ErrorCode equality and clone
    // =======================================================================

    #[test]
    fn test_error_code_equality() {
        assert_eq!(argv_no_subcommand(), argv_no_subcommand());
    }

    #[test]
    fn test_error_code_inequality() {
        assert_ne!(argv_no_subcommand(), argv_empty_argument());
    }

    #[test]
    fn test_error_code_clone() {
        let e = internal_uncaught("boom");
        let cloned = e.clone();
        assert_eq!(e, cloned);
    }

    // =======================================================================
    // Keyword / reserved-name helpers
    // =======================================================================

    #[test]
    fn test_reserved_keywords() {
        assert!(is_reserved_keyword("INPUT"));
        assert!(is_reserved_keyword("input"));
        assert!(is_reserved_keyword("MUST"));
        assert!(is_reserved_keyword("must-not"));
        assert!(is_reserved_keyword("WHEN"));
        assert!(is_reserved_keyword("CONFORMS"));
        assert!(is_reserved_keyword("uses"));
        assert!(!is_reserved_keyword("MySpec"));
        assert!(!is_reserved_keyword("cli.validate"));
    }

    // =======================================================================
    // CliError tests
    // =======================================================================

    #[test]
    fn test_cli_error_display_with_file_and_line() {
        let e = CliError::new(YAML_MALFORMED, "YAML well-formedness error")
            .with_file("foo.yass.yaml")
            .with_line(5);
        assert_eq!(
            e.to_string(),
            "foo.yass.yaml:5: [yass.yaml.malformed] YAML well-formedness error"
        );
    }

    #[test]
    fn test_cli_error_display_without_line() {
        let e = CliError::new(YAML_MALFORMED, "YAML well-formedness error")
            .with_file("foo.yass.yaml");
        assert_eq!(
            e.to_string(),
            "foo.yass.yaml: [yass.yaml.malformed] YAML well-formedness error"
        );
    }

    #[test]
    fn test_cli_error_display_no_file() {
        let e = CliError::new(ARGV_NO_SUBCOMMAND, "no subcommand given");
        assert_eq!(
            e.to_string(),
            "yass: [yass.argv.no_subcommand] no subcommand given"
        );
    }

    #[test]
    fn test_cli_error_newline_in_message() {
        let e = CliError::new(INTERNAL_UNCAUGHT, "line1\nline2\rline3");
        assert_eq!(
            e.to_string(),
            "yass: [yass.internal.uncaught] line1 line2 line3"
        );
    }

    #[test]
    fn test_cli_error_exit_code_derived_from_code() {
        let e = CliError::new(ARGV_UNKNOWN_FLAG, "unknown flag: --foo");
        assert_eq!(e.exit_code, EXIT_USAGE);

        let e = CliError::new(YAML_MALFORMED, "bad yaml");
        assert_eq!(e.exit_code, EXIT_PROCESSING);
    }

    #[test]
    fn test_cli_error_with_exit_code_override() {
        let e = CliError::new(INTERNAL_UNCAUGHT, "boom").with_exit_code(42);
        assert_eq!(e.exit_code, 42);
    }
}
