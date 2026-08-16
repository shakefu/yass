// Tests for M1 — diagnostics foundation.
//
// Specs covered:
//   spec/cli.errors.yass.yaml :: cli.errors
//   spec/cli.yass.yaml       :: cli.ExitCode
//   spec/cli.yass.yaml       :: cli.ErrorLine
//
// Test names cite spec :: slot :: obligation so a failure points back to the
// source obligation.

#include "doctest.h"

#include <set>
#include <string>
#include <vector>

#include "diag.hpp"

using namespace yass::diag;

namespace {

// One row of the cli.errors RETURN table: the enum value, the exact token
// string, the exact exit code, and (where the message is FIXED with no runtime
// substitution) the exact message prose from the spec. fixed_message is empty
// for codes whose message carries a substitution or carries no user prose.
struct Expect {
    ErrorCode code;
    const char* token;
    int exit;
    const char* fixed_message;  // "" when not applicable / has substitution
};

// Mirrors cli.errors RETURN, in spec order.
const std::vector<Expect>& expectations() {
    static const std::vector<Expect> kE = {
        // yass.exit.* — meaning is in the token; carry no user prose.
        {ErrorCode::exit_success, "yass.exit.success", 0, ""},
        {ErrorCode::exit_processing, "yass.exit.processing", 1, ""},
        {ErrorCode::exit_usage, "yass.exit.usage", 2, ""},
        {ErrorCode::exit_sigint, "yass.exit.sigint", 130, ""},
        {ErrorCode::exit_sigterm, "yass.exit.sigterm", 143, ""},

        // yass.argv.*
        {ErrorCode::argv_unknown_subcommand, "yass.argv.unknown_subcommand", 2, ""},
        {ErrorCode::argv_no_subcommand, "yass.argv.no_subcommand", 2, "no subcommand given"},
        {ErrorCode::argv_unknown_flag, "yass.argv.unknown_flag", 2, ""},
        {ErrorCode::argv_empty_argument, "yass.argv.empty_argument", 2, "empty argument"},
        {ErrorCode::argv_short_flag, "yass.argv.short_flag", 2, ""},
        {ErrorCode::argv_case_mismatch, "yass.argv.case_mismatch", 2, ""},
        {ErrorCode::argv_abbreviation, "yass.argv.abbreviation", 2, ""},
        {ErrorCode::argv_missing_positional, "yass.argv.missing_positional", 2, ""},
        {ErrorCode::argv_stdin_dash, "yass.argv.stdin_dash", 2,
         "stdin marker `-` is not supported; pass a file path"},

        // yass.path.*
        {ErrorCode::path_not_found, "yass.path.not_found", 2, ""},
        {ErrorCode::path_bad_extension, "yass.path.bad_extension", 2, ""},
        {ErrorCode::path_unreadable, "yass.path.unreadable", 2, ""},
        {ErrorCode::path_invalid_type, "yass.path.invalid_type", 2, ""},
        {ErrorCode::path_colon_in_path, "yass.path.colon_in_path", 2, ""},

        // yass.glob.*
        {ErrorCode::glob_no_match, "yass.glob.no_match", 2, ""},

        // yass.discover.*
        {ErrorCode::discover_no_files, "yass.discover.no_files", 2, "no .yass.yaml files found"},
        // discover.dir_unreadable is "non-fatal during recursion" — the spec
        // assigns no standalone exit; exit_for is not asserted here.
        {ErrorCode::discover_dir_unreadable, "yass.discover.dir_unreadable", -1, ""},

        // yass.findroot.*
        {ErrorCode::findroot_no_marker, "yass.findroot.no_marker", 2, "no project root marker found"},

        // yass.yaml.*
        {ErrorCode::yaml_not_utf8, "yass.yaml.not_utf8", 1, "file is not valid UTF-8"},
        {ErrorCode::yaml_has_bom, "yass.yaml.has_bom", 1, "file begins with a UTF-8 BOM"},
        {ErrorCode::yaml_malformed, "yass.yaml.malformed", 1, "YAML well-formedness error"},
        {ErrorCode::yaml_empty_file, "yass.yaml.empty_file", 1, "empty file"},
        {ErrorCode::yaml_duplicate_key, "yass.yaml.duplicate_key", 1, ""},
        {ErrorCode::yaml_anchor_or_alias, "yass.yaml.anchor_or_alias", 1,
         "YAML anchors, aliases, and explicit tags are not allowed"},
        {ErrorCode::yaml_empty_stream, "yass.yaml.empty_stream", 1, "YAML stream contains no documents"},

        // yass.preamble.*
        {ErrorCode::preamble_has_spec_key, "yass.preamble.has_spec_key", 1,
         "first document must be a Preamble, not a Spec"},
        {ErrorCode::preamble_missing, "yass.preamble.missing", 1, "missing Preamble"},
        {ErrorCode::preamble_misplaced, "yass.preamble.misplaced", 1, "Preamble must be the first document"},
        {ErrorCode::preamble_duplicate, "yass.preamble.duplicate", 1, "more than one Preamble in file"},
        {ErrorCode::preamble_missing_description, "yass.preamble.missing_description", 1,
         "Preamble missing description"},
        {ErrorCode::preamble_missing_version, "yass.preamble.missing_version", 1, "Preamble missing version"},
        {ErrorCode::preamble_unknown_version, "yass.preamble.unknown_version", 1, ""},
        {ErrorCode::preamble_bad_related, "yass.preamble.bad_related", 1,
         "Preamble related must be a sequence of strings"},

        // yass.spec.*
        {ErrorCode::spec_no_name, "yass.spec.no_name", 1, "spec document missing spec key"},
        {ErrorCode::spec_name_not_string, "yass.spec.name_not_string", 1, "spec name must be a string"},
        {ErrorCode::spec_name_empty, "yass.spec.name_empty", 1, "spec name is empty"},
        {ErrorCode::spec_name_bad_chars, "yass.spec.name_bad_chars", 1, ""},
        {ErrorCode::spec_name_bad_form, "yass.spec.name_bad_form", 1, ""},
        {ErrorCode::spec_name_reserved, "yass.spec.name_reserved", 1, ""},
        {ErrorCode::spec_unknown_key, "yass.spec.unknown_key", 1, ""},
        {ErrorCode::spec_duplicate_name, "yass.spec.duplicate_name", 1, ""},

        // yass.slot.*
        {ErrorCode::slot_value_not_list, "yass.slot.value_not_list", 1, ""},

        // yass.obligation.*
        {ErrorCode::obligation_bad_value_shape, "yass.obligation.bad_value_shape", 1,
         "obligation value must be a quoted scalar"},
        {ErrorCode::obligation_missing_normativity_or_ref, "yass.obligation.missing_normativity_or_ref", 1,
         "obligation must carry a Normativity keyword or a Reference"},
        {ErrorCode::obligation_guard_without_normativity, "yass.obligation.guard_without_normativity", 1,
         "WHEN guard requires a Normativity keyword"},
        {ErrorCode::obligation_duplicate_reference, "yass.obligation.duplicate_reference", 1, ""},
        {ErrorCode::obligation_duplicate_normativity, "yass.obligation.duplicate_normativity", 1,
         "duplicate Normativity keyword in obligation"},

        // yass.normativity.*
        {ErrorCode::normativity_unknown, "yass.normativity.unknown", 1, ""},

        // yass.reference.*
        {ErrorCode::reference_unknown_relation, "yass.reference.unknown_relation", 1, ""},

        // yass.ref.*
        {ErrorCode::ref_malformed, "yass.ref.malformed", 1, ""},
        {ErrorCode::ref_unknown_slot, "yass.ref.unknown_slot", 1, ""},
        {ErrorCode::ref_slot_not_declared, "yass.ref.slot_not_declared", 1, ""},
        {ErrorCode::ref_spec_not_found_same_file, "yass.ref.spec_not_found_same_file", 1, ""},
        {ErrorCode::ref_file_not_found, "yass.ref.file_not_found", 1, ""},
        {ErrorCode::ref_file_not_parseable, "yass.ref.file_not_parseable", 1, ""},
        {ErrorCode::ref_spec_not_found_other_file, "yass.ref.spec_not_found_other_file", 1, ""},

        // yass.query.*
        {ErrorCode::query_name_missing, "yass.query.name_missing", 2, "missing spec name"},
        {ErrorCode::query_name_blank, "yass.query.name_blank", 2, "spec name is blank or contains whitespace"},
        {ErrorCode::query_no_match, "yass.query.no_match", 1, ""},
        {ErrorCode::query_conforms_unresolved, "yass.query.conforms_unresolved", 1, ""},
        {ErrorCode::query_conforms_no_slot, "yass.query.conforms_no_slot", 1, ""},
        {ErrorCode::query_scope_not_found, "yass.query.scope_not_found", 2, ""},
        {ErrorCode::query_scope_empty, "yass.query.scope_empty", 2, ""},

        // yass.internal.*
        {ErrorCode::internal_uncaught, "yass.internal.uncaught", 1, ""},
    };
    return kE;
}

}  // namespace

// ===========================================================================
// cli.errors :: RETURN — token mapping for EVERY code.
// ===========================================================================
TEST_CASE("cli.errors::RETURN every ErrorCode maps to its exact stable token") {
    for (const Expect& e : expectations()) {
        CHECK(std::string(token(e.code)) == std::string(e.token));
    }
}

// ===========================================================================
// cli.errors :: RETURN — exit code per code (cross-checks cli.ExitCode mapping).
// ===========================================================================
TEST_CASE("cli.errors::RETURN every ErrorCode maps to its exact spec exit code") {
    for (const Expect& e : expectations()) {
        if (e.exit < 0) {
            continue;  // discover.dir_unreadable: non-fatal, no standalone exit
        }
        CHECK_MESSAGE(exit_for(e.code) == e.exit, "code: " << e.token);
    }
}

// ===========================================================================
// cli.errors :: INVARIANT — names live under yass.<area>.<error> and use only
// the published code charset.
//
// SPEC ISSUE: cli.errors INPUT and INVARIANT both state the charset is
// "[a-z0-9.]", but EVERY multi-word code published in cli.errors RETURN uses
// an underscore (e.g. yass.argv.unknown_flag, yass.query.scope_not_found). The
// published codes are authoritative (they are the stable wire tokens), so the
// real allowed charset is [a-z0-9._]. This test pins the actual codes and the
// charset they actually use.
// ===========================================================================
TEST_CASE("cli.errors::INVARIANT every token is yass-namespaced and uses only [a-z0-9._]") {
    for (const Expect& e : expectations()) {
        std::string t(token(e.code));
        CHECK_MESSAGE(t.rfind("yass.", 0) == 0, "token not yass-namespaced: " << t);
        for (char c : t) {
            bool ok = (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '_';
            CHECK_MESSAGE(ok, "token has disallowed char: " << t);
        }
    }
}

// ===========================================================================
// cli.errors :: INVARIANT — codes are distinct (no reused token).
// ===========================================================================
TEST_CASE("cli.errors::INVARIANT every ErrorCode has a distinct token") {
    std::set<std::string> seen;
    for (const Expect& e : expectations()) {
        std::string t(token(e.code));
        CHECK_MESSAGE(seen.insert(t).second, "duplicate token: " << t);
    }
}

// ===========================================================================
// cli.ExitCode :: RETURN — the named exit-code constants.
// ===========================================================================
TEST_CASE("cli.ExitCode::RETURN exit-code constants have their specified values") {
    CHECK(ExitCode::SUCCESS == 0);     // exit 0 on success
    CHECK(ExitCode::PROCESSING == 1);  // validation/processing rule violated
    CHECK(ExitCode::USAGE == 2);       // argv-parse or file-input failure
    CHECK(ExitCode::ON_SIGINT == 130);   // received SIGINT
    CHECK(ExitCode::ON_SIGTERM == 143);  // received SIGTERM
}

// ===========================================================================
// cli.ExitCode :: RETURN (MUST-NOT) — only 0/1/2/130/143 are permitted.
// ===========================================================================
TEST_CASE("cli.ExitCode::RETURN MUST-NOT only 0/1/2/130/143 are valid exit codes") {
    CHECK(is_valid_exit_code(0));
    CHECK(is_valid_exit_code(1));
    CHECK(is_valid_exit_code(2));
    CHECK(is_valid_exit_code(130));
    CHECK(is_valid_exit_code(143));
    // A representative set of forbidden codes.
    CHECK_FALSE(is_valid_exit_code(3));
    CHECK_FALSE(is_valid_exit_code(127));
    CHECK_FALSE(is_valid_exit_code(255));
    CHECK_FALSE(is_valid_exit_code(-1));
}

TEST_CASE("cli.ExitCode::RETURN MUST-NOT every code's exit is one of the permitted values") {
    for (const Expect& e : expectations()) {
        CHECK_MESSAGE(is_valid_exit_code(exit_for(e.code)), "code: " << e.token);
    }
}

// ===========================================================================
// cli.errors :: RETURN — canonical FIXED messages match the spec byte-for-byte.
// ===========================================================================
TEST_CASE("cli.errors::RETURN fixed-message codes produce the exact spec prose") {
    for (const Expect& e : expectations()) {
        if (e.fixed_message[0] == '\0') {
            continue;  // skip substitution / no-prose codes
        }
        CHECK_MESSAGE(canonical_message(e.code) == std::string(e.fixed_message),
                      "code: " << e.token);
    }
}

// ===========================================================================
// cli.errors :: RETURN — substituted messages match the spec template exactly.
// A representative subset of codes whose message carries a `<...>` placeholder.
// ===========================================================================
TEST_CASE("cli.errors::RETURN substituted-message codes interpolate the spec template") {
    CHECK(canonical_message(ErrorCode::argv_unknown_subcommand, "frobnicate") ==
          "unknown subcommand: frobnicate");
    CHECK(canonical_message(ErrorCode::argv_unknown_flag, "--nope") == "unknown flag: --nope");
    CHECK(canonical_message(ErrorCode::argv_short_flag, "-x") ==
          "short-form flags are not supported in v1: -x");
    CHECK(canonical_message(ErrorCode::argv_case_mismatch, "Validate") ==
          "subcommand or flag case mismatch: Validate");
    CHECK(canonical_message(ErrorCode::argv_abbreviation, "val") ==
          "abbreviations are not supported: val");
    CHECK(canonical_message(ErrorCode::argv_missing_positional, "name") ==
          "missing required argument: name");
    CHECK(canonical_message(ErrorCode::path_not_found, "a/b.yass.yaml") ==
          "path does not exist: a/b.yass.yaml");
    CHECK(canonical_message(ErrorCode::path_bad_extension, "x.txt") ==
          "expected a .yass.yaml file: x.txt");
    CHECK(canonical_message(ErrorCode::path_colon_in_path, "a:b") ==
          "path contains an unsupported colon character: a:b");
    CHECK(canonical_message(ErrorCode::glob_no_match, "**/*.yass.yaml") ==
          "no files matched pattern: **/*.yass.yaml");
    CHECK(canonical_message(ErrorCode::yaml_duplicate_key, "spec") ==
          "duplicate mapping key: spec");
    CHECK(canonical_message(ErrorCode::preamble_unknown_version, "v2") ==
          "unsupported Preamble version: v2");
    CHECK(canonical_message(ErrorCode::spec_name_bad_chars, "a b") ==
          "spec name contains disallowed characters: a b");
    CHECK(canonical_message(ErrorCode::spec_name_reserved, "MUST") ==
          "spec name collides with a reserved keyword: MUST");
    CHECK(canonical_message(ErrorCode::spec_unknown_key, "bogus") == "unknown spec key: bogus");
    CHECK(canonical_message(ErrorCode::slot_value_not_list, "INPUT") ==
          "slot value must be a list: INPUT");
    CHECK(canonical_message(ErrorCode::normativity_unknown, "SHALL") ==
          "unknown Normativity keyword: SHALL");
    CHECK(canonical_message(ErrorCode::reference_unknown_relation, "RELATES") ==
          "unknown Reference relation: RELATES");
    CHECK(canonical_message(ErrorCode::ref_malformed, "x@") == "malformed ref target: x@");
    CHECK(canonical_message(ErrorCode::query_no_match, "pkg.Sym") == "no spec matches: pkg.Sym");
    CHECK(canonical_message(ErrorCode::query_conforms_no_slot, "a@B") ==
          "CONFORMS ref must address a slot in v1: a@B");
    CHECK(canonical_message(ErrorCode::internal_uncaught, "boom") == "internal error: boom");
    CHECK(canonical_message(ErrorCode::discover_dir_unreadable, "d") == "cannot read directory: d");
}

// A no-substitution code ignores any arg passed.
TEST_CASE("cli.errors::RETURN fixed-message code ignores a substitution arg") {
    CHECK(canonical_message(ErrorCode::argv_no_subcommand, "ignored") == "no subcommand given");
}

// ===========================================================================
// cli.ErrorLine :: RETURN — format WITH a known source line.
// ===========================================================================
TEST_CASE("cli.ErrorLine::RETURN MUST format with line as <file>:<line>: [<code>] <message>") {
    Diagnostic d;
    d.file = "spec/cli.yass.yaml";
    d.line = 42;
    d.code = ErrorCode::yaml_duplicate_key;
    d.message = "duplicate mapping key: spec";
    CHECK(format_error_line(d) ==
          "spec/cli.yass.yaml:42: [yass.yaml.duplicate_key] duplicate mapping key: spec");
}

// ===========================================================================
// cli.ErrorLine :: RETURN — format WITHOUT a known source line.
// ===========================================================================
TEST_CASE("cli.ErrorLine::RETURN MUST format without line as <file>: [<code>] <message>") {
    Diagnostic d;
    d.file = "foo.yass.yaml";
    d.line = std::nullopt;
    d.code = ErrorCode::yaml_empty_file;
    d.message = "empty file";
    CHECK(format_error_line(d) == "foo.yass.yaml: [yass.yaml.empty_file] empty file");
}

// ===========================================================================
// cli.ErrorLine :: RETURN (WHEN no input file) — emit literal "yass" as <file>.
// ===========================================================================
TEST_CASE("cli.ErrorLine::RETURN WHEN no input file emits literal yass as <file>") {
    Diagnostic d;
    d.file = "";  // no associated input file
    d.line = std::nullopt;
    d.code = ErrorCode::argv_no_subcommand;
    d.message = "no subcommand given";
    CHECK(format_error_line(d) == "yass: [yass.argv.no_subcommand] no subcommand given");
}

// Even with no file, a line value still renders between file and code.
TEST_CASE("cli.ErrorLine::RETURN no-file with line uses yass as <file>") {
    Diagnostic d;
    d.file = "";
    d.line = 7;
    d.code = ErrorCode::internal_uncaught;
    d.message = "internal error: boom";
    CHECK(format_error_line(d) == "yass:7: [yass.internal.uncaught] internal error: boom");
}

// ===========================================================================
// cli.ErrorLine :: RETURN (MUST replace newline) — newline -> single space.
// ===========================================================================
TEST_CASE("cli.ErrorLine::RETURN MUST replace any newline in <message> with a single space") {
    Diagnostic d;
    d.file = "a.yass.yaml";
    d.line = std::nullopt;
    d.code = ErrorCode::yaml_malformed;
    d.message = "line one\nline two\nline three";
    CHECK(format_error_line(d) ==
          "a.yass.yaml: [yass.yaml.malformed] line one line two line three");
}

// ===========================================================================
// cli.ErrorLine :: RETURN (MUST-NOT ANSI) — output is plain (no escapes).
// ===========================================================================
TEST_CASE("cli.ErrorLine::RETURN MUST-NOT include ANSI escape codes") {
    Diagnostic d;
    d.file = "a.yass.yaml";
    d.line = 1;
    d.code = ErrorCode::yaml_empty_file;
    d.message = "empty file";
    std::string line = format_error_line(d);
    CHECK(line.find('\x1b') == std::string::npos);
    CHECK(line.find('\n') == std::string::npos);  // single line, no trailing LF
}

// ===========================================================================
// cli.ErrorLine :: RETURN — path form (relativize_path).
// ===========================================================================

TEST_CASE("cli.ErrorLine::RETURN MUST emit basename alone when file is directly inside cwd") {
    CHECK(relativize_path("/home/u/proj/foo.yass.yaml", "/home/u/proj") == "foo.yass.yaml");
    // Relative input resolved against cwd, still directly inside.
    CHECK(relativize_path("foo.yass.yaml", "/home/u/proj") == "foo.yass.yaml");
}

TEST_CASE("cli.ErrorLine::RETURN MUST emit a relative path WITHOUT leading ./ when nested under cwd") {
    std::string r = relativize_path("/home/u/proj/spec/cli.yass.yaml", "/home/u/proj");
    CHECK(r == "spec/cli.yass.yaml");
    CHECK(r.rfind("./", 0) != 0);  // no leading "./"
    CHECK(relativize_path("/home/u/proj/a/b/c.yass.yaml", "/home/u/proj") == "a/b/c.yass.yaml");
}

TEST_CASE("cli.ErrorLine::RETURN WHEN file is not under cwd MUST emit an absolute path") {
    // Sibling directory: NOT under cwd (must not be rendered with ../).
    CHECK(relativize_path("/home/u/other/x.yass.yaml", "/home/u/proj") ==
          "/home/u/other/x.yass.yaml");
    // Parent directory file: not under cwd.
    CHECK(relativize_path("/home/u/x.yass.yaml", "/home/u/proj") == "/home/u/x.yass.yaml");
    // Completely unrelated absolute path outside cwd stays absolute.
    CHECK(relativize_path("/etc/passwd.yass.yaml", "/home/u/proj") == "/etc/passwd.yass.yaml");
}

TEST_CASE("cli.ErrorLine::RETURN prefix that is not a path-component boundary is NOT under cwd") {
    // "/home/u/project2" shares the textual prefix "/home/u/proj" but is a
    // different directory; it must be treated as NOT under cwd.
    CHECK(relativize_path("/home/u/project2/x.yass.yaml", "/home/u/proj") ==
          "/home/u/project2/x.yass.yaml");
}

TEST_CASE("cli.ErrorLine::RETURN MUST emit forward slashes as the path separator") {
    std::string r = relativize_path("/home/u/proj/a/b/c.yass.yaml", "/home/u/proj");
    CHECK(r.find('\\') == std::string::npos);
    CHECK(r == "a/b/c.yass.yaml");
}

TEST_CASE("cli.ErrorLine::RETURN MUST-NOT resolve symlinks; lexical normalization only") {
    // A "." / ".." segment is normalized lexically (no filesystem access). The
    // path need not exist; we are testing pure lexical behavior.
    CHECK(relativize_path("/home/u/proj/./spec/x.yass.yaml", "/home/u/proj") == "spec/x.yass.yaml");
    CHECK(relativize_path("/home/u/proj/spec/../spec/x.yass.yaml", "/home/u/proj") ==
          "spec/x.yass.yaml");
    // cwd with a trailing slash is handled the same as without.
    CHECK(relativize_path("/home/u/proj/foo.yass.yaml", "/home/u/proj/") == "foo.yass.yaml");
}

TEST_CASE("cli.ErrorLine::RETURN already-absolute path outside cwd is emitted as-is") {
    CHECK(relativize_path("/var/data/x.yass.yaml", "/home/u/proj") == "/var/data/x.yass.yaml");
}
