/**
 * Error codes, exit codes, and message templates for the yass CLI.
 *
 * Every error code follows the namespace pattern yass.<area>.<error>
 * using only [a-z0-9.] characters.
 */

// ---------------------------------------------------------------------------
// Recognized keyword sets
// ---------------------------------------------------------------------------

/** Slot keywords used in spec obligation parsing. */
export const SLOT_KEYWORDS = [
  "INPUT",
  "RETURN",
  "ERROR",
  "SIDE-EFFECT",
  "INVARIANT",
] as const;

/** Normativity keywords (RFC 2119 style). */
export const NORMATIVITY_KEYWORDS = [
  "MUST",
  "MUST-NOT",
  "SHOULD",
  "SHOULD-NOT",
  "MAY",
] as const;

/** Reference relation keywords. */
export const REFERENCE_KEYWORDS = ["CONFORMS", "USES", "SEE"] as const;

/** Guard keyword for conditional obligations. */
export const GUARD_KEYWORD = "WHEN" as const;

/**
 * Union of slot + normativity keywords.
 * Spec names must not collide with any of these.
 */
export const ALL_RESERVED_KEYWORDS: readonly string[] = [
  ...SLOT_KEYWORDS,
  ...NORMATIVITY_KEYWORDS,
];

// ---------------------------------------------------------------------------
// Error codes
// ---------------------------------------------------------------------------

/**
 * Every error code recognised by the CLI.
 *
 * Symbolic keys are UPPER_SNAKE derived from the dotted code, e.g.
 * `yass.argv.unknown_subcommand` -> `ARGV_UNKNOWN_SUBCOMMAND`.
 */
export const ErrorCode = {
  // Exit sentinels
  EXIT_SUCCESS: "yass.exit.success",
  EXIT_PROCESSING: "yass.exit.processing",
  EXIT_USAGE: "yass.exit.usage",
  EXIT_SIGINT: "yass.exit.sigint",
  EXIT_SIGTERM: "yass.exit.sigterm",

  // Argv
  ARGV_UNKNOWN_SUBCOMMAND: "yass.argv.unknown_subcommand",
  ARGV_NO_SUBCOMMAND: "yass.argv.no_subcommand",
  ARGV_UNKNOWN_FLAG: "yass.argv.unknown_flag",
  ARGV_EMPTY_ARGUMENT: "yass.argv.empty_argument",
  ARGV_SHORT_FLAG: "yass.argv.short_flag",
  ARGV_CASE_MISMATCH: "yass.argv.case_mismatch",
  ARGV_ABBREVIATION: "yass.argv.abbreviation",
  ARGV_MISSING_POSITIONAL: "yass.argv.missing_positional",
  ARGV_STDIN_DASH: "yass.argv.stdin_dash",

  // Path
  PATH_NOT_FOUND: "yass.path.not_found",
  PATH_BAD_EXTENSION: "yass.path.bad_extension",
  PATH_UNREADABLE: "yass.path.unreadable",
  PATH_INVALID_TYPE: "yass.path.invalid_type",
  PATH_COLON_IN_PATH: "yass.path.colon_in_path",

  // Glob
  GLOB_NO_MATCH: "yass.glob.no_match",

  // Discover
  DISCOVER_NO_FILES: "yass.discover.no_files",
  DISCOVER_DIR_UNREADABLE: "yass.discover.dir_unreadable",

  // Findroot
  FINDROOT_NO_MARKER: "yass.findroot.no_marker",

  // YAML
  YAML_NOT_UTF8: "yass.yaml.not_utf8",
  YAML_HAS_BOM: "yass.yaml.has_bom",
  YAML_MALFORMED: "yass.yaml.malformed",
  YAML_EMPTY_FILE: "yass.yaml.empty_file",
  YAML_DUPLICATE_KEY: "yass.yaml.duplicate_key",
  YAML_ANCHOR_OR_ALIAS: "yass.yaml.anchor_or_alias",
  YAML_EMPTY_STREAM: "yass.yaml.empty_stream",

  // Preamble
  PREAMBLE_HAS_SPEC_KEY: "yass.preamble.has_spec_key",
  PREAMBLE_MISSING: "yass.preamble.missing",
  PREAMBLE_MISPLACED: "yass.preamble.misplaced",
  PREAMBLE_DUPLICATE: "yass.preamble.duplicate",
  PREAMBLE_MISSING_DESCRIPTION: "yass.preamble.missing_description",
  PREAMBLE_MISSING_VERSION: "yass.preamble.missing_version",
  PREAMBLE_UNKNOWN_VERSION: "yass.preamble.unknown_version",
  PREAMBLE_BAD_RELATED: "yass.preamble.bad_related",

  // Spec
  SPEC_NO_NAME: "yass.spec.no_name",
  SPEC_NAME_NOT_STRING: "yass.spec.name_not_string",
  SPEC_NAME_EMPTY: "yass.spec.name_empty",
  SPEC_NAME_BAD_CHARS: "yass.spec.name_bad_chars",
  SPEC_NAME_BAD_FORM: "yass.spec.name_bad_form",
  SPEC_NAME_RESERVED: "yass.spec.name_reserved",
  SPEC_UNKNOWN_KEY: "yass.spec.unknown_key",
  SPEC_DUPLICATE_NAME: "yass.spec.duplicate_name",

  // Slot
  SLOT_VALUE_NOT_LIST: "yass.slot.value_not_list",

  // Obligation
  OBLIGATION_BAD_VALUE_SHAPE: "yass.obligation.bad_value_shape",
  OBLIGATION_MISSING_NORMATIVITY_OR_REF:
    "yass.obligation.missing_normativity_or_ref",
  OBLIGATION_GUARD_WITHOUT_NORMATIVITY:
    "yass.obligation.guard_without_normativity",
  OBLIGATION_DUPLICATE_REFERENCE: "yass.obligation.duplicate_reference",
  OBLIGATION_DUPLICATE_NORMATIVITY: "yass.obligation.duplicate_normativity",

  // Normativity
  NORMATIVITY_UNKNOWN: "yass.normativity.unknown",

  // Reference
  REFERENCE_UNKNOWN_RELATION: "yass.reference.unknown_relation",

  // Ref
  REF_MALFORMED: "yass.ref.malformed",
  REF_UNKNOWN_SLOT: "yass.ref.unknown_slot",
  REF_SLOT_NOT_DECLARED: "yass.ref.slot_not_declared",
  REF_SPEC_NOT_FOUND_SAME_FILE: "yass.ref.spec_not_found_same_file",
  REF_FILE_NOT_FOUND: "yass.ref.file_not_found",
  REF_FILE_NOT_PARSEABLE: "yass.ref.file_not_parseable",
  REF_SPEC_NOT_FOUND_OTHER_FILE: "yass.ref.spec_not_found_other_file",

  // Query
  QUERY_NAME_MISSING: "yass.query.name_missing",
  QUERY_NAME_BLANK: "yass.query.name_blank",
  QUERY_NO_MATCH: "yass.query.no_match",
  QUERY_CONFORMS_UNRESOLVED: "yass.query.conforms_unresolved",
  QUERY_CONFORMS_NO_SLOT: "yass.query.conforms_no_slot",
  QUERY_SCOPE_NOT_FOUND: "yass.query.scope_not_found",
  QUERY_SCOPE_EMPTY: "yass.query.scope_empty",

  // Internal
  INTERNAL_UNCAUGHT: "yass.internal.uncaught",
} as const;

/** The union type of all error-code string values. */
export type ErrorCodeValue = (typeof ErrorCode)[keyof typeof ErrorCode];

// ---------------------------------------------------------------------------
// Exit-code mapping
// ---------------------------------------------------------------------------

/** Maps every error code to its numeric process exit code. */
const EXIT_CODES: Record<ErrorCodeValue, number> = {
  // Exit sentinels
  [ErrorCode.EXIT_SUCCESS]: 0,
  [ErrorCode.EXIT_PROCESSING]: 1,
  [ErrorCode.EXIT_USAGE]: 2,
  [ErrorCode.EXIT_SIGINT]: 130,
  [ErrorCode.EXIT_SIGTERM]: 143,

  // Argv (all exit 2)
  [ErrorCode.ARGV_UNKNOWN_SUBCOMMAND]: 2,
  [ErrorCode.ARGV_NO_SUBCOMMAND]: 2,
  [ErrorCode.ARGV_UNKNOWN_FLAG]: 2,
  [ErrorCode.ARGV_EMPTY_ARGUMENT]: 2,
  [ErrorCode.ARGV_SHORT_FLAG]: 2,
  [ErrorCode.ARGV_CASE_MISMATCH]: 2,
  [ErrorCode.ARGV_ABBREVIATION]: 2,
  [ErrorCode.ARGV_MISSING_POSITIONAL]: 2,
  [ErrorCode.ARGV_STDIN_DASH]: 2,

  // Path (all exit 2)
  [ErrorCode.PATH_NOT_FOUND]: 2,
  [ErrorCode.PATH_BAD_EXTENSION]: 2,
  [ErrorCode.PATH_UNREADABLE]: 2,
  [ErrorCode.PATH_INVALID_TYPE]: 2,
  [ErrorCode.PATH_COLON_IN_PATH]: 2,

  // Glob (exit 2)
  [ErrorCode.GLOB_NO_MATCH]: 2,

  // Discover
  [ErrorCode.DISCOVER_NO_FILES]: 2,
  [ErrorCode.DISCOVER_DIR_UNREADABLE]: 1,

  // Findroot (exit 2)
  [ErrorCode.FINDROOT_NO_MARKER]: 2,

  // YAML (all exit 1)
  [ErrorCode.YAML_NOT_UTF8]: 1,
  [ErrorCode.YAML_HAS_BOM]: 1,
  [ErrorCode.YAML_MALFORMED]: 1,
  [ErrorCode.YAML_EMPTY_FILE]: 1,
  [ErrorCode.YAML_DUPLICATE_KEY]: 1,
  [ErrorCode.YAML_ANCHOR_OR_ALIAS]: 1,
  [ErrorCode.YAML_EMPTY_STREAM]: 1,

  // Preamble (all exit 1)
  [ErrorCode.PREAMBLE_HAS_SPEC_KEY]: 1,
  [ErrorCode.PREAMBLE_MISSING]: 1,
  [ErrorCode.PREAMBLE_MISPLACED]: 1,
  [ErrorCode.PREAMBLE_DUPLICATE]: 1,
  [ErrorCode.PREAMBLE_MISSING_DESCRIPTION]: 1,
  [ErrorCode.PREAMBLE_MISSING_VERSION]: 1,
  [ErrorCode.PREAMBLE_UNKNOWN_VERSION]: 1,
  [ErrorCode.PREAMBLE_BAD_RELATED]: 1,

  // Spec (all exit 1)
  [ErrorCode.SPEC_NO_NAME]: 1,
  [ErrorCode.SPEC_NAME_NOT_STRING]: 1,
  [ErrorCode.SPEC_NAME_EMPTY]: 1,
  [ErrorCode.SPEC_NAME_BAD_CHARS]: 1,
  [ErrorCode.SPEC_NAME_BAD_FORM]: 1,
  [ErrorCode.SPEC_NAME_RESERVED]: 1,
  [ErrorCode.SPEC_UNKNOWN_KEY]: 1,
  [ErrorCode.SPEC_DUPLICATE_NAME]: 1,

  // Slot (exit 1)
  [ErrorCode.SLOT_VALUE_NOT_LIST]: 1,

  // Obligation (all exit 1)
  [ErrorCode.OBLIGATION_BAD_VALUE_SHAPE]: 1,
  [ErrorCode.OBLIGATION_MISSING_NORMATIVITY_OR_REF]: 1,
  [ErrorCode.OBLIGATION_GUARD_WITHOUT_NORMATIVITY]: 1,
  [ErrorCode.OBLIGATION_DUPLICATE_REFERENCE]: 1,
  [ErrorCode.OBLIGATION_DUPLICATE_NORMATIVITY]: 1,

  // Normativity (exit 1)
  [ErrorCode.NORMATIVITY_UNKNOWN]: 1,

  // Reference (exit 1)
  [ErrorCode.REFERENCE_UNKNOWN_RELATION]: 1,

  // Ref (all exit 1)
  [ErrorCode.REF_MALFORMED]: 1,
  [ErrorCode.REF_UNKNOWN_SLOT]: 1,
  [ErrorCode.REF_SLOT_NOT_DECLARED]: 1,
  [ErrorCode.REF_SPEC_NOT_FOUND_SAME_FILE]: 1,
  [ErrorCode.REF_FILE_NOT_FOUND]: 1,
  [ErrorCode.REF_FILE_NOT_PARSEABLE]: 1,
  [ErrorCode.REF_SPEC_NOT_FOUND_OTHER_FILE]: 1,

  // Query
  [ErrorCode.QUERY_NAME_MISSING]: 2,
  [ErrorCode.QUERY_NAME_BLANK]: 2,
  [ErrorCode.QUERY_NO_MATCH]: 1,
  [ErrorCode.QUERY_CONFORMS_UNRESOLVED]: 1,
  [ErrorCode.QUERY_CONFORMS_NO_SLOT]: 1,
  [ErrorCode.QUERY_SCOPE_NOT_FOUND]: 2,
  [ErrorCode.QUERY_SCOPE_EMPTY]: 2,

  // Internal (exit 1)
  [ErrorCode.INTERNAL_UNCAUGHT]: 1,
};

// ---------------------------------------------------------------------------
// Message templates
// ---------------------------------------------------------------------------

/**
 * Message templates keyed by error code.
 *
 * Placeholders use `<name>` syntax and are replaced by the corresponding
 * key from the `args` record passed to `messageFor`.
 */
const MESSAGE_TEMPLATES: Record<ErrorCodeValue, string> = {
  // Exit sentinels (no user-facing messages, but included for completeness)
  [ErrorCode.EXIT_SUCCESS]: "success",
  [ErrorCode.EXIT_PROCESSING]: "processing error",
  [ErrorCode.EXIT_USAGE]: "usage error",
  [ErrorCode.EXIT_SIGINT]: "interrupted",
  [ErrorCode.EXIT_SIGTERM]: "terminated",

  // Argv
  [ErrorCode.ARGV_UNKNOWN_SUBCOMMAND]: "unknown subcommand: <arg>",
  [ErrorCode.ARGV_NO_SUBCOMMAND]: "no subcommand given",
  [ErrorCode.ARGV_UNKNOWN_FLAG]: "unknown flag: <flag>",
  [ErrorCode.ARGV_EMPTY_ARGUMENT]: "empty argument",
  [ErrorCode.ARGV_SHORT_FLAG]:
    "short-form flags are not supported in v1: <flag>",
  [ErrorCode.ARGV_CASE_MISMATCH]:
    "subcommand or flag case mismatch: <token>",
  [ErrorCode.ARGV_ABBREVIATION]: "abbreviations are not supported: <token>",
  [ErrorCode.ARGV_MISSING_POSITIONAL]: "missing required argument: <name>",
  [ErrorCode.ARGV_STDIN_DASH]:
    "stdin marker `-` is not supported; pass a file path",

  // Path
  [ErrorCode.PATH_NOT_FOUND]: "path does not exist: <path>",
  [ErrorCode.PATH_BAD_EXTENSION]: "expected a .yass.yaml file: <path>",
  [ErrorCode.PATH_UNREADABLE]: "cannot read path: <path>",
  [ErrorCode.PATH_INVALID_TYPE]:
    "path is neither a file nor a directory: <path>",
  [ErrorCode.PATH_COLON_IN_PATH]:
    "path contains an unsupported colon character: <path>",

  // Glob
  [ErrorCode.GLOB_NO_MATCH]: "no files matched pattern: <pattern>",

  // Discover
  [ErrorCode.DISCOVER_NO_FILES]: "no .yass.yaml files found",
  [ErrorCode.DISCOVER_DIR_UNREADABLE]: "cannot read directory: <path>",

  // Findroot
  [ErrorCode.FINDROOT_NO_MARKER]: "no project root marker found",

  // YAML
  [ErrorCode.YAML_NOT_UTF8]: "file is not valid UTF-8",
  [ErrorCode.YAML_HAS_BOM]: "file begins with a UTF-8 BOM",
  [ErrorCode.YAML_MALFORMED]: "YAML well-formedness error",
  [ErrorCode.YAML_EMPTY_FILE]: "empty file",
  [ErrorCode.YAML_DUPLICATE_KEY]: "duplicate mapping key: <key>",
  [ErrorCode.YAML_ANCHOR_OR_ALIAS]:
    "YAML anchors, aliases, and explicit tags are not allowed",
  [ErrorCode.YAML_EMPTY_STREAM]: "YAML stream contains no documents",

  // Preamble
  [ErrorCode.PREAMBLE_HAS_SPEC_KEY]:
    "first document must be a Preamble, not a Spec",
  [ErrorCode.PREAMBLE_MISSING]: "missing Preamble",
  [ErrorCode.PREAMBLE_MISPLACED]: "Preamble must be the first document",
  [ErrorCode.PREAMBLE_DUPLICATE]: "more than one Preamble in file",
  [ErrorCode.PREAMBLE_MISSING_DESCRIPTION]: "Preamble missing description",
  [ErrorCode.PREAMBLE_MISSING_VERSION]: "Preamble missing version",
  [ErrorCode.PREAMBLE_UNKNOWN_VERSION]:
    "unsupported Preamble version: <version>",
  [ErrorCode.PREAMBLE_BAD_RELATED]:
    "Preamble related must be a sequence of strings",

  // Spec
  [ErrorCode.SPEC_NO_NAME]: "spec document missing spec key",
  [ErrorCode.SPEC_NAME_NOT_STRING]: "spec name must be a string",
  [ErrorCode.SPEC_NAME_EMPTY]: "spec name is empty",
  [ErrorCode.SPEC_NAME_BAD_CHARS]:
    "spec name contains disallowed characters: <name>",
  [ErrorCode.SPEC_NAME_BAD_FORM]: "spec name is malformed: <name>",
  [ErrorCode.SPEC_NAME_RESERVED]:
    "spec name collides with a reserved keyword: <name>",
  [ErrorCode.SPEC_UNKNOWN_KEY]: "unknown spec key: <key>",
  [ErrorCode.SPEC_DUPLICATE_NAME]: "duplicate spec name in file: <name>",

  // Slot
  [ErrorCode.SLOT_VALUE_NOT_LIST]: "slot value must be a list: <slot>",

  // Obligation
  [ErrorCode.OBLIGATION_BAD_VALUE_SHAPE]:
    "obligation value must be a quoted scalar",
  [ErrorCode.OBLIGATION_MISSING_NORMATIVITY_OR_REF]:
    "obligation must carry a Normativity keyword or a Reference",
  [ErrorCode.OBLIGATION_GUARD_WITHOUT_NORMATIVITY]:
    "WHEN guard requires a Normativity keyword",
  [ErrorCode.OBLIGATION_DUPLICATE_REFERENCE]:
    "duplicate Reference relation in obligation: <relation>",
  [ErrorCode.OBLIGATION_DUPLICATE_NORMATIVITY]:
    "duplicate Normativity keyword in obligation",

  // Normativity
  [ErrorCode.NORMATIVITY_UNKNOWN]: "unknown Normativity keyword: <keyword>",

  // Reference
  [ErrorCode.REFERENCE_UNKNOWN_RELATION]:
    "unknown Reference relation: <relation>",

  // Ref
  [ErrorCode.REF_MALFORMED]: "malformed ref target: <target>",
  [ErrorCode.REF_UNKNOWN_SLOT]: "unknown slot in ref target: <slot>",
  [ErrorCode.REF_SLOT_NOT_DECLARED]:
    "referenced spec does not declare slot: <target>",
  [ErrorCode.REF_SPEC_NOT_FOUND_SAME_FILE]:
    "spec not found in file: <target>",
  [ErrorCode.REF_FILE_NOT_FOUND]: "referenced file not found: <target>",
  [ErrorCode.REF_FILE_NOT_PARSEABLE]:
    "referenced file not parseable: <target>",
  [ErrorCode.REF_SPEC_NOT_FOUND_OTHER_FILE]:
    "spec not found in referenced file: <target>",

  // Query
  [ErrorCode.QUERY_NAME_MISSING]: "missing spec name",
  [ErrorCode.QUERY_NAME_BLANK]:
    "spec name is blank or contains whitespace",
  [ErrorCode.QUERY_NO_MATCH]: "no spec matches: <name>",
  [ErrorCode.QUERY_CONFORMS_UNRESOLVED]:
    "unresolvable CONFORMS ref: <target>",
  [ErrorCode.QUERY_CONFORMS_NO_SLOT]:
    "CONFORMS ref must address a slot in v1: <target>",
  [ErrorCode.QUERY_SCOPE_NOT_FOUND]:
    "scope path does not exist: <path>",
  [ErrorCode.QUERY_SCOPE_EMPTY]:
    "no .yass.yaml files found in scope: <path>",

  // Internal
  [ErrorCode.INTERNAL_UNCAUGHT]: "internal error: <message>",
};

// ---------------------------------------------------------------------------
// Public helpers
// ---------------------------------------------------------------------------

/**
 * Return the numeric process exit code for a given error code string.
 *
 * Throws if the code is not recognised.
 */
export function exitCodeFor(code: string): number {
  const exit = EXIT_CODES[code as ErrorCodeValue];
  if (exit === undefined) {
    throw new Error(`unknown error code: ${code}`);
  }
  return exit;
}

/**
 * Format the message template for the given error code, substituting
 * `<key>` placeholders with values from `args`.
 *
 * Throws if the code is not recognised.
 */
export function messageFor(
  code: string,
  args: Record<string, string> = {},
): string {
  const template = MESSAGE_TEMPLATES[code as ErrorCodeValue];
  if (template === undefined) {
    throw new Error(`unknown error code: ${code}`);
  }
  return template.replace(/<([a-z_]+)>/g, (_match, key: string) => {
    const value = args[key];
    if (value === undefined) {
      return `<${key}>`;
    }
    return value;
  });
}
