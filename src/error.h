/*
 * error.h - Error line formatting per cli.ErrorLine spec
 */
#ifndef YASS_ERROR_H
#define YASS_ERROR_H

#include <stddef.h>

/*
 * Error codes per cli.errors spec.
 * Each code is a machine-stable token.
 */

/* Exit status codes */
#define EC_EXIT_SUCCESS              "yass.exit.success"
#define EC_EXIT_PROCESSING           "yass.exit.processing"
#define EC_EXIT_USAGE                "yass.exit.usage"
#define EC_EXIT_SIGINT               "yass.exit.sigint"
#define EC_EXIT_SIGTERM              "yass.exit.sigterm"

/* Argv errors */
#define EC_ARGV_UNKNOWN_SUBCOMMAND   "yass.argv.unknown_subcommand"
#define EC_ARGV_NO_SUBCOMMAND        "yass.argv.no_subcommand"
#define EC_ARGV_UNKNOWN_FLAG         "yass.argv.unknown_flag"
#define EC_ARGV_EMPTY_ARGUMENT       "yass.argv.empty_argument"
#define EC_ARGV_SHORT_FLAG           "yass.argv.short_flag"
#define EC_ARGV_CASE_MISMATCH        "yass.argv.case_mismatch"
#define EC_ARGV_ABBREVIATION         "yass.argv.abbreviation"
#define EC_ARGV_MISSING_POSITIONAL   "yass.argv.missing_positional"
#define EC_ARGV_STDIN_DASH           "yass.argv.stdin_dash"

/* Path errors */
#define EC_PATH_NOT_FOUND            "yass.path.not_found"
#define EC_PATH_BAD_EXTENSION        "yass.path.bad_extension"
#define EC_PATH_UNREADABLE           "yass.path.unreadable"
#define EC_PATH_INVALID_TYPE         "yass.path.invalid_type"
#define EC_PATH_COLON_IN_PATH        "yass.path.colon_in_path"

/* Glob errors */
#define EC_GLOB_NO_MATCH             "yass.glob.no_match"

/* Discovery errors */
#define EC_DISCOVER_NO_FILES         "yass.discover.no_files"
#define EC_DISCOVER_DIR_UNREADABLE   "yass.discover.dir_unreadable"

/* FindProjectRoot errors */
#define EC_FINDROOT_NO_MARKER        "yass.findroot.no_marker"

/* YAML errors */
#define EC_YAML_NOT_UTF8             "yass.yaml.not_utf8"
#define EC_YAML_HAS_BOM             "yass.yaml.has_bom"
#define EC_YAML_MALFORMED            "yass.yaml.malformed"
#define EC_YAML_EMPTY_FILE           "yass.yaml.empty_file"
#define EC_YAML_DUPLICATE_KEY        "yass.yaml.duplicate_key"
#define EC_YAML_ANCHOR_OR_ALIAS      "yass.yaml.anchor_or_alias"
#define EC_YAML_EMPTY_STREAM         "yass.yaml.empty_stream"

/* Preamble errors */
#define EC_PREAMBLE_HAS_SPEC_KEY     "yass.preamble.has_spec_key"
#define EC_PREAMBLE_MISSING          "yass.preamble.missing"
#define EC_PREAMBLE_MISPLACED        "yass.preamble.misplaced"
#define EC_PREAMBLE_DUPLICATE        "yass.preamble.duplicate"
#define EC_PREAMBLE_MISSING_DESC     "yass.preamble.missing_description"
#define EC_PREAMBLE_MISSING_VERSION  "yass.preamble.missing_version"
#define EC_PREAMBLE_UNKNOWN_VERSION  "yass.preamble.unknown_version"
#define EC_PREAMBLE_BAD_RELATED      "yass.preamble.bad_related"

/* Spec errors */
#define EC_SPEC_NO_NAME              "yass.spec.no_name"
#define EC_SPEC_NAME_NOT_STRING      "yass.spec.name_not_string"
#define EC_SPEC_NAME_EMPTY           "yass.spec.name_empty"
#define EC_SPEC_NAME_BAD_CHARS       "yass.spec.name_bad_chars"
#define EC_SPEC_NAME_BAD_FORM        "yass.spec.name_bad_form"
#define EC_SPEC_NAME_RESERVED        "yass.spec.name_reserved"
#define EC_SPEC_UNKNOWN_KEY          "yass.spec.unknown_key"
#define EC_SPEC_DUPLICATE_NAME       "yass.spec.duplicate_name"

/* Slot errors */
#define EC_SLOT_VALUE_NOT_LIST       "yass.slot.value_not_list"

/* Obligation errors */
#define EC_OBLIGATION_BAD_VALUE      "yass.obligation.bad_value_shape"
#define EC_OBLIGATION_MISSING_NORM   "yass.obligation.missing_normativity_or_ref"
#define EC_OBLIGATION_GUARD_NO_NORM  "yass.obligation.guard_without_normativity"
#define EC_OBLIGATION_DUP_REF        "yass.obligation.duplicate_reference"
#define EC_OBLIGATION_DUP_NORM       "yass.obligation.duplicate_normativity"

/* Normativity errors */
#define EC_NORMATIVITY_UNKNOWN       "yass.normativity.unknown"

/* Reference errors */
#define EC_REFERENCE_UNKNOWN         "yass.reference.unknown_relation"

/* Ref target errors */
#define EC_REF_MALFORMED             "yass.ref.malformed"
#define EC_REF_UNKNOWN_SLOT          "yass.ref.unknown_slot"
#define EC_REF_SLOT_NOT_DECLARED     "yass.ref.slot_not_declared"
#define EC_REF_SPEC_NOT_FOUND_SAME   "yass.ref.spec_not_found_same_file"
#define EC_REF_FILE_NOT_FOUND        "yass.ref.file_not_found"
#define EC_REF_FILE_NOT_PARSEABLE    "yass.ref.file_not_parseable"
#define EC_REF_SPEC_NOT_FOUND_OTHER  "yass.ref.spec_not_found_other_file"

/* Query errors */
#define EC_QUERY_NAME_MISSING        "yass.query.name_missing"
#define EC_QUERY_NAME_BLANK          "yass.query.name_blank"
#define EC_QUERY_NO_MATCH            "yass.query.no_match"
#define EC_QUERY_CONFORMS_UNRESOLVED "yass.query.conforms_unresolved"
#define EC_QUERY_CONFORMS_NO_SLOT    "yass.query.conforms_no_slot"
#define EC_QUERY_SCOPE_NOT_FOUND     "yass.query.scope_not_found"
#define EC_QUERY_SCOPE_EMPTY         "yass.query.scope_empty"

/* Internal errors */
#define EC_INTERNAL_UNCAUGHT         "yass.internal.uncaught"

/*
 * Emit an error line to stderr.
 * Format: <file>:<line>: [<code>] <message>
 * or:     <file>: [<code>] <message> when line is 0
 *
 * file: path relative to cwd when under cwd, absolute otherwise.
 *       "yass" when no file is associated.
 * line: 1-based line number, or 0 for no line.
 * code: machine-stable error code from cli.errors
 * message: human-readable message (newlines replaced with spaces)
 */
void error_emit(const char *file, int line, const char *code, const char *message);

/*
 * Format a file path for error output per cli.ErrorLine spec:
 * - relative to cwd when under cwd (no leading ./)
 * - basename alone when directly in cwd
 * - absolute when not under cwd
 * - "yass" when file is NULL
 * Returns a malloc'd string. Caller must free.
 */
char *error_format_path(const char *file);

/*
 * Replace newlines in a message with spaces.
 * Returns a malloc'd string. Caller must free.
 */
char *error_sanitize_message(const char *message);

/* Accumulate error count */
typedef struct {
    int count;
} error_counter_t;

void error_counter_init(error_counter_t *ec);
void error_count(error_counter_t *ec);

#endif /* YASS_ERROR_H */
