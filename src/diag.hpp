#pragma once

// M1 — diagnostics foundation.
//
// Implements the machine-stable error-code table (spec/cli.errors.yass.yaml
// :: cli.errors), the exit-code contract (spec/cli.yass.yaml :: cli.ExitCode),
// and the error-line formatter + path-form rules (spec/cli.yass.yaml ::
// cli.ErrorLine).
//
// Every code listed in cli.errors RETURN has a corresponding ErrorCode enum
// value. The Entry table pins, per code, the stable token string and the exit
// code the spec assigns. Message PROSE that contains runtime substitutions is
// NOT stored here; callers build the final message string (canonical_message
// helps) and hand it to format_error_line.

#include <optional>
#include <string>
#include <string_view>

namespace yass::diag {

// --------------------------------------------------------------------------
// Exit codes (spec/cli.yass.yaml :: cli.ExitCode).
// Only 0, 1, 2, 130, 143 are permitted in v1.
// --------------------------------------------------------------------------
// NOTE: members are intentionally NOT named SIGINT/SIGTERM: those identifiers
// are object-like macros from <csignal> (pulled in transitively) and would be
// textually substituted, breaking compilation. We use the value-carrying names
// ON_SIGINT / ON_SIGTERM instead.
namespace ExitCode {
inline constexpr int SUCCESS = 0;       // exit 0 on success
inline constexpr int PROCESSING = 1;    // validation or processing rule violated
inline constexpr int USAGE = 2;         // argv-parse or file-input failure
inline constexpr int ON_SIGINT = 130;   // process received SIGINT
inline constexpr int ON_SIGTERM = 143;  // process received SIGTERM
}  // namespace ExitCode

// True iff `code` is one of the only exit codes permitted by cli.ExitCode.
bool is_valid_exit_code(int code);

// --------------------------------------------------------------------------
// ErrorCode — one value per code in cli.errors RETURN.
// --------------------------------------------------------------------------
enum class ErrorCode {
    // yass.exit.*
    exit_success,
    exit_processing,
    exit_usage,
    exit_sigint,
    exit_sigterm,

    // yass.argv.*
    argv_unknown_subcommand,
    argv_no_subcommand,
    argv_unknown_flag,
    argv_empty_argument,
    argv_short_flag,
    argv_case_mismatch,
    argv_abbreviation,
    argv_missing_positional,
    argv_stdin_dash,

    // yass.path.*
    path_not_found,
    path_bad_extension,
    path_unreadable,
    path_invalid_type,
    path_colon_in_path,

    // yass.glob.*
    glob_no_match,

    // yass.discover.*
    discover_no_files,
    discover_dir_unreadable,

    // yass.findroot.*
    findroot_no_marker,

    // yass.yaml.*
    yaml_not_utf8,
    yaml_has_bom,
    yaml_malformed,
    yaml_empty_file,
    yaml_duplicate_key,
    yaml_anchor_or_alias,
    yaml_empty_stream,

    // yass.preamble.*
    preamble_has_spec_key,
    preamble_missing,
    preamble_misplaced,
    preamble_duplicate,
    preamble_missing_description,
    preamble_missing_version,
    preamble_unknown_version,
    preamble_bad_related,

    // yass.spec.*
    spec_no_name,
    spec_name_not_string,
    spec_name_empty,
    spec_name_bad_chars,
    spec_name_bad_form,
    spec_name_reserved,
    spec_unknown_key,
    spec_duplicate_name,

    // yass.slot.*
    slot_value_not_list,

    // yass.obligation.*
    obligation_bad_value_shape,
    obligation_missing_normativity_or_ref,
    obligation_guard_without_normativity,
    obligation_duplicate_reference,
    obligation_duplicate_normativity,

    // yass.normativity.*
    normativity_unknown,

    // yass.reference.*
    reference_unknown_relation,

    // yass.ref.*
    ref_malformed,
    ref_unknown_slot,
    ref_slot_not_declared,
    ref_spec_not_found_same_file,
    ref_file_not_found,
    ref_file_not_parseable,
    ref_spec_not_found_other_file,

    // yass.query.*
    query_name_missing,
    query_name_blank,
    query_no_match,
    query_conforms_unresolved,
    query_conforms_no_slot,
    query_scope_not_found,
    query_scope_empty,

    // yass.internal.*
    internal_uncaught,
};

// Stable token string for `code`, e.g. ErrorCode::argv_unknown_flag ->
// "yass.argv.unknown_flag". (cli.errors INPUT: codes live under yass.<area>.<error>.)
std::string_view token(ErrorCode code);

// The exit code cli.errors assigns to `code`.
//
// Note: yass.discover.dir_unreadable is documented as "non-fatal during
// recursion" and carries no standalone exit code; exit_for returns
// ExitCode::PROCESSING for it as a defined-but-unused fallback. It is never
// the sole determinant of process exit.
int exit_for(ErrorCode code);

// --------------------------------------------------------------------------
// Diagnostic + error-line formatting (cli.ErrorLine).
// --------------------------------------------------------------------------
struct Diagnostic {
    // The path the file was reached by (NOT realpath), already in the form
    // cli.ErrorLine requires (see relativize_path). Empty/none => the literal
    // token "yass" is emitted as <file>.
    std::string file;
    // 1-based line of the offending YAML node, when known.
    std::optional<int> line;
    ErrorCode code;
    // Final, fully-substituted human-readable message.
    std::string message;
};

// Format ONE error line per cli.ErrorLine RETURN, WITHOUT a trailing newline:
//   with line:    "<file>:<line>: [<token>] <message>"
//   without line: "<file>: [<token>] <message>"
// When file is empty, the literal token "yass" is used as <file>. Any newline
// in <message> is replaced with a single ASCII space. No ANSI / formatting.
std::string format_error_line(const Diagnostic& diag);

// --------------------------------------------------------------------------
// Path form for <file> (cli.ErrorLine).
// --------------------------------------------------------------------------
// Compute the <file> string for `input_path` relative to `cwd`, by LEXICAL
// rules only (no symlink/realpath resolution):
//   - if the lexical-absolute input begins with lexical-absolute cwd + "/":
//     emit the path relative to cwd WITHOUT a leading "./";
//   - if the file is directly inside cwd: emit the basename alone;
//   - otherwise: emit the absolute path.
// Forward slashes are used as the separator on all platforms.
std::string relativize_path(std::string_view input_path, std::string_view cwd);

// --------------------------------------------------------------------------
// Canonical message text (cli.errors RETURN).
// --------------------------------------------------------------------------
// Build the exact message string cli.errors states for `code`, substituting
// `arg` into the single `<...>` placeholder where the message has one. For
// codes whose message is fixed (no placeholder), `arg` is ignored. Matches the
// spec's message PROSE byte-for-byte.
std::string canonical_message(ErrorCode code, std::string_view arg = {});

}  // namespace yass::diag
