#include "diag.hpp"

#include <array>
#include <filesystem>
#include <string>

namespace yass::diag {

namespace {

// Per-code static table. `message_template` carries the FIXED-message prose
// for codes with no runtime substitution, or a template where "{}" marks the
// single substitution point for codes that take an argument. canonical_message
// fills the "{}".
struct Entry {
    ErrorCode code;
    std::string_view token;
    int exit;
    std::string_view message_template;  // "{}" = substitution point
};

// Order is irrelevant for lookup (we index linearly keyed on `code`), but the
// table mirrors cli.errors RETURN top-to-bottom for auditability.
constexpr std::array<Entry, 69> kTable = {{
    // yass.exit.* — meaning is in the token; messages are not user-facing
    // prose, so they are left empty.
    {ErrorCode::exit_success, "yass.exit.success", ExitCode::SUCCESS, ""},
    {ErrorCode::exit_processing, "yass.exit.processing", ExitCode::PROCESSING, ""},
    {ErrorCode::exit_usage, "yass.exit.usage", ExitCode::USAGE, ""},
    {ErrorCode::exit_sigint, "yass.exit.sigint", ExitCode::ON_SIGINT, ""},
    {ErrorCode::exit_sigterm, "yass.exit.sigterm", ExitCode::ON_SIGTERM, ""},

    // yass.argv.*
    {ErrorCode::argv_unknown_subcommand, "yass.argv.unknown_subcommand", ExitCode::USAGE,
     "unknown subcommand: {}"},
    {ErrorCode::argv_no_subcommand, "yass.argv.no_subcommand", ExitCode::USAGE,
     "no subcommand given"},
    {ErrorCode::argv_unknown_flag, "yass.argv.unknown_flag", ExitCode::USAGE,
     "unknown flag: {}"},
    {ErrorCode::argv_empty_argument, "yass.argv.empty_argument", ExitCode::USAGE,
     "empty argument"},
    {ErrorCode::argv_short_flag, "yass.argv.short_flag", ExitCode::USAGE,
     "short-form flags are not supported in v1: {}"},
    {ErrorCode::argv_case_mismatch, "yass.argv.case_mismatch", ExitCode::USAGE,
     "subcommand or flag case mismatch: {}"},
    {ErrorCode::argv_abbreviation, "yass.argv.abbreviation", ExitCode::USAGE,
     "abbreviations are not supported: {}"},
    {ErrorCode::argv_missing_positional, "yass.argv.missing_positional", ExitCode::USAGE,
     "missing required argument: {}"},
    {ErrorCode::argv_stdin_dash, "yass.argv.stdin_dash", ExitCode::USAGE,
     "stdin marker `-` is not supported; pass a file path"},

    // yass.path.*
    {ErrorCode::path_not_found, "yass.path.not_found", ExitCode::USAGE,
     "path does not exist: {}"},
    {ErrorCode::path_bad_extension, "yass.path.bad_extension", ExitCode::USAGE,
     "expected a .yass.yaml file: {}"},
    {ErrorCode::path_unreadable, "yass.path.unreadable", ExitCode::USAGE,
     "cannot read path: {}"},
    {ErrorCode::path_invalid_type, "yass.path.invalid_type", ExitCode::USAGE,
     "path is neither a file nor a directory: {}"},
    {ErrorCode::path_colon_in_path, "yass.path.colon_in_path", ExitCode::USAGE,
     "path contains an unsupported colon character: {}"},

    // yass.glob.*
    {ErrorCode::glob_no_match, "yass.glob.no_match", ExitCode::USAGE,
     "no files matched pattern: {}"},

    // yass.discover.*
    {ErrorCode::discover_no_files, "yass.discover.no_files", ExitCode::USAGE,
     "no .yass.yaml files found"},
    // Non-fatal during recursion; no standalone exit code in cli.errors. We map
    // it to PROCESSING as a defined fallback (never the sole exit determinant).
    {ErrorCode::discover_dir_unreadable, "yass.discover.dir_unreadable", ExitCode::PROCESSING,
     "cannot read directory: {}"},

    // yass.findroot.*
    {ErrorCode::findroot_no_marker, "yass.findroot.no_marker", ExitCode::USAGE,
     "no project root marker found"},

    // yass.yaml.*
    {ErrorCode::yaml_not_utf8, "yass.yaml.not_utf8", ExitCode::PROCESSING,
     "file is not valid UTF-8"},
    {ErrorCode::yaml_has_bom, "yass.yaml.has_bom", ExitCode::PROCESSING,
     "file begins with a UTF-8 BOM"},
    {ErrorCode::yaml_malformed, "yass.yaml.malformed", ExitCode::PROCESSING,
     "YAML well-formedness error"},
    {ErrorCode::yaml_empty_file, "yass.yaml.empty_file", ExitCode::PROCESSING,
     "empty file"},
    {ErrorCode::yaml_duplicate_key, "yass.yaml.duplicate_key", ExitCode::PROCESSING,
     "duplicate mapping key: {}"},
    {ErrorCode::yaml_anchor_or_alias, "yass.yaml.anchor_or_alias", ExitCode::PROCESSING,
     "YAML anchors, aliases, and explicit tags are not allowed"},
    {ErrorCode::yaml_empty_stream, "yass.yaml.empty_stream", ExitCode::PROCESSING,
     "YAML stream contains no documents"},

    // yass.preamble.*
    {ErrorCode::preamble_has_spec_key, "yass.preamble.has_spec_key", ExitCode::PROCESSING,
     "first document must be a Preamble, not a Spec"},
    {ErrorCode::preamble_missing, "yass.preamble.missing", ExitCode::PROCESSING,
     "missing Preamble"},
    {ErrorCode::preamble_misplaced, "yass.preamble.misplaced", ExitCode::PROCESSING,
     "Preamble must be the first document"},
    {ErrorCode::preamble_duplicate, "yass.preamble.duplicate", ExitCode::PROCESSING,
     "more than one Preamble in file"},
    {ErrorCode::preamble_missing_description, "yass.preamble.missing_description", ExitCode::PROCESSING,
     "Preamble missing description"},
    {ErrorCode::preamble_missing_version, "yass.preamble.missing_version", ExitCode::PROCESSING,
     "Preamble missing version"},
    {ErrorCode::preamble_unknown_version, "yass.preamble.unknown_version", ExitCode::PROCESSING,
     "unsupported Preamble version: {}"},
    {ErrorCode::preamble_bad_related, "yass.preamble.bad_related", ExitCode::PROCESSING,
     "Preamble related must be a sequence of strings"},

    // yass.spec.*
    {ErrorCode::spec_no_name, "yass.spec.no_name", ExitCode::PROCESSING,
     "spec document missing spec key"},
    {ErrorCode::spec_name_not_string, "yass.spec.name_not_string", ExitCode::PROCESSING,
     "spec name must be a string"},
    {ErrorCode::spec_name_empty, "yass.spec.name_empty", ExitCode::PROCESSING,
     "spec name is empty"},
    {ErrorCode::spec_name_bad_chars, "yass.spec.name_bad_chars", ExitCode::PROCESSING,
     "spec name contains disallowed characters: {}"},
    {ErrorCode::spec_name_bad_form, "yass.spec.name_bad_form", ExitCode::PROCESSING,
     "spec name is malformed: {}"},
    {ErrorCode::spec_name_reserved, "yass.spec.name_reserved", ExitCode::PROCESSING,
     "spec name collides with a reserved keyword: {}"},
    {ErrorCode::spec_unknown_key, "yass.spec.unknown_key", ExitCode::PROCESSING,
     "unknown spec key: {}"},
    {ErrorCode::spec_duplicate_name, "yass.spec.duplicate_name", ExitCode::PROCESSING,
     "duplicate spec name in file: {}"},

    // yass.slot.*
    {ErrorCode::slot_value_not_list, "yass.slot.value_not_list", ExitCode::PROCESSING,
     "slot value must be a list: {}"},

    // yass.obligation.*
    {ErrorCode::obligation_bad_value_shape, "yass.obligation.bad_value_shape", ExitCode::PROCESSING,
     "obligation value must be a quoted scalar"},
    {ErrorCode::obligation_missing_normativity_or_ref, "yass.obligation.missing_normativity_or_ref",
     ExitCode::PROCESSING, "obligation must carry a Normativity keyword or a Reference"},
    {ErrorCode::obligation_guard_without_normativity, "yass.obligation.guard_without_normativity",
     ExitCode::PROCESSING, "WHEN guard requires a Normativity keyword"},
    {ErrorCode::obligation_duplicate_reference, "yass.obligation.duplicate_reference",
     ExitCode::PROCESSING, "duplicate Reference relation in obligation: {}"},
    {ErrorCode::obligation_duplicate_normativity, "yass.obligation.duplicate_normativity",
     ExitCode::PROCESSING, "duplicate Normativity keyword in obligation"},

    // yass.normativity.*
    {ErrorCode::normativity_unknown, "yass.normativity.unknown", ExitCode::PROCESSING,
     "unknown Normativity keyword: {}"},

    // yass.reference.*
    {ErrorCode::reference_unknown_relation, "yass.reference.unknown_relation", ExitCode::PROCESSING,
     "unknown Reference relation: {}"},

    // yass.ref.*
    {ErrorCode::ref_malformed, "yass.ref.malformed", ExitCode::PROCESSING,
     "malformed ref target: {}"},
    {ErrorCode::ref_unknown_slot, "yass.ref.unknown_slot", ExitCode::PROCESSING,
     "unknown slot in ref target: {}"},
    {ErrorCode::ref_slot_not_declared, "yass.ref.slot_not_declared", ExitCode::PROCESSING,
     "referenced spec does not declare slot: {}"},
    {ErrorCode::ref_spec_not_found_same_file, "yass.ref.spec_not_found_same_file", ExitCode::PROCESSING,
     "spec not found in file: {}"},
    {ErrorCode::ref_file_not_found, "yass.ref.file_not_found", ExitCode::PROCESSING,
     "referenced file not found: {}"},
    {ErrorCode::ref_file_not_parseable, "yass.ref.file_not_parseable", ExitCode::PROCESSING,
     "referenced file not parseable: {}"},
    {ErrorCode::ref_spec_not_found_other_file, "yass.ref.spec_not_found_other_file", ExitCode::PROCESSING,
     "spec not found in referenced file: {}"},

    // yass.query.*
    {ErrorCode::query_name_missing, "yass.query.name_missing", ExitCode::USAGE,
     "missing spec name"},
    {ErrorCode::query_name_blank, "yass.query.name_blank", ExitCode::USAGE,
     "spec name is blank or contains whitespace"},
    {ErrorCode::query_no_match, "yass.query.no_match", ExitCode::PROCESSING,
     "no spec matches: {}"},
    {ErrorCode::query_conforms_unresolved, "yass.query.conforms_unresolved", ExitCode::PROCESSING,
     "unresolvable CONFORMS ref: {}"},
    {ErrorCode::query_conforms_no_slot, "yass.query.conforms_no_slot", ExitCode::PROCESSING,
     "CONFORMS ref must address a slot in v1: {}"},
    {ErrorCode::query_scope_not_found, "yass.query.scope_not_found", ExitCode::USAGE,
     "scope path does not exist: {}"},
    {ErrorCode::query_scope_empty, "yass.query.scope_empty", ExitCode::USAGE,
     "no .yass.yaml files found in scope: {}"},

    // yass.internal.*
    {ErrorCode::internal_uncaught, "yass.internal.uncaught", ExitCode::PROCESSING,
     "internal error: {}"},
}};

const Entry& lookup(ErrorCode code) {
    for (const Entry& e : kTable) {
        if (e.code == code) {
            return e;
        }
    }
    // Unreachable: every enumerator has a table row. Fall back to the first row
    // rather than UB if the table ever drifts.
    return kTable[0];
}

// Lexically normalize and absolutize `p` against `base`, WITHOUT touching the
// filesystem (no realpath / no symlink resolution).
std::filesystem::path lexical_absolute(std::filesystem::path p,
                                       const std::filesystem::path& base) {
    if (p.is_relative()) {
        p = base / p;
    }
    return p.lexically_normal();
}

}  // namespace

bool is_valid_exit_code(int code) {
    return code == ExitCode::SUCCESS || code == ExitCode::PROCESSING ||
           code == ExitCode::USAGE || code == ExitCode::ON_SIGINT ||
           code == ExitCode::ON_SIGTERM;
}

std::string_view token(ErrorCode code) { return lookup(code).token; }

int exit_for(ErrorCode code) { return lookup(code).exit; }

std::string canonical_message(ErrorCode code, std::string_view arg) {
    std::string_view tmpl = lookup(code).message_template;
    auto pos = tmpl.find("{}");
    if (pos == std::string_view::npos) {
        return std::string(tmpl);
    }
    std::string out;
    out.reserve(tmpl.size() + arg.size());
    out.append(tmpl.substr(0, pos));
    out.append(arg);
    out.append(tmpl.substr(pos + 2));
    return out;
}

std::string format_error_line(const Diagnostic& diag) {
    // <file>: "yass" when none/empty.
    std::string file = diag.file.empty() ? std::string("yass") : diag.file;

    // Replace any newline in the message with a single ASCII space.
    std::string message = diag.message;
    for (char& c : message) {
        if (c == '\n') {
            c = ' ';
        }
    }

    std::string out;
    out += file;
    if (diag.line.has_value()) {
        out += ':';
        out += std::to_string(*diag.line);
    }
    out += ": [";
    out += token(diag.code);
    out += "] ";
    out += message;
    return out;
}

std::string relativize_path(std::string_view input_path, std::string_view cwd) {
    namespace fs = std::filesystem;

    fs::path input{std::string(input_path)};
    fs::path base{std::string(cwd)};

    fs::path abs_input = lexical_absolute(input, base);
    fs::path abs_cwd = base.lexically_normal();

    // Compute the lexical relative path of abs_input with respect to abs_cwd.
    // We do this by hand (not lexically_relative, which would emit "../" for
    // siblings) so we can decide "under cwd" strictly.
    auto cwd_begin = abs_cwd.begin();
    auto cwd_end = abs_cwd.end();
    // A trailing empty element appears for paths ending in '/'; drop it.
    auto last_real = [](const fs::path& p) {
        auto it = p.end();
        if (it != p.begin()) {
            --it;
            if (it->empty()) {
                return it;  // points at trailing empty component
            }
            ++it;
        }
        return it;
    };
    fs::path::iterator cwd_stop = last_real(abs_cwd);

    auto in_it = abs_input.begin();
    auto in_end = abs_input.end();
    auto cwd_it = cwd_begin;

    bool under = true;
    for (; cwd_it != cwd_stop && cwd_it != cwd_end; ++cwd_it, ++in_it) {
        if (in_it == in_end || *in_it != *cwd_it) {
            under = false;
            break;
        }
    }

    auto to_forward = [](fs::path p) {
        std::string s = p.generic_string();  // generic_string uses '/'
        return s;
    };

    if (!under) {
        // Not under cwd: emit the absolute path (forward slashes).
        return to_forward(abs_input);
    }

    // `in_it` now points at the first component after the cwd prefix.
    // Collect the remaining components as the relative path.
    fs::path rel;
    int component_count = 0;
    for (auto it = in_it; it != in_end; ++it) {
        if (it->empty()) {
            continue;  // skip trailing empty component
        }
        rel /= *it;
        ++component_count;
    }

    if (component_count == 0) {
        // Input IS the cwd itself; emit it as an absolute path (degenerate, the
        // CLI rejects directories-as-files upstream, but stay well-defined).
        return to_forward(abs_input);
    }

    if (component_count == 1) {
        // Directly inside cwd: basename alone.
        return to_forward(rel);
    }

    // Nested under cwd: relative path WITHOUT a leading "./".
    return to_forward(rel);
}

}  // namespace yass::diag
