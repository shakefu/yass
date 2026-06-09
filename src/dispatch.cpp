#include "dispatch.hpp"

#include <array>
#include <csignal>
#include <exception>
#include <optional>
#include <ostream>
#include <string>
#include <string_view>
#include <vector>

#include "diag.hpp"
#include "list.hpp"
#include "query.hpp"
#include "validate.hpp"

namespace yass::dispatch {

// --------------------------------------------------------------------------
// Top-level usage text — captured byte-for-byte from the reference `yass
// --help`. The reference reuses one fixed usage block; we reproduce it exactly
// so the differential happy-path (stdout + exit) holds.
// --------------------------------------------------------------------------
const char* const kUsageText =
    "Usage: yass <command> [arguments]\n"
    "\n"
    "Commands:\n"
    "  validate    Validate .yass.yaml files\n"
    "  list        List specs in .yass.yaml files\n"
    "  query       Query a spec by name\n"
    "\n"
    "Flags:\n"
    "  --help      Show this help\n"
    "  --version   Show version\n";

namespace {

// ASCII-only lowercase. Argv tokens are matched against the fixed ASCII
// keywords validate/query/list/--help/--version; an ASCII fold is correct and
// avoids locale dependence. Non-ASCII bytes pass through unchanged.
std::string ascii_lower(std::string_view s) {
    std::string out;
    out.reserve(s.size());
    for (char c : s) {
        if (c >= 'A' && c <= 'Z') c = static_cast<char>(c - 'A' + 'a');
        out.push_back(c);
    }
    return out;
}

// True iff `prefix` is a proper prefix of `whole` (strictly shorter, and `whole`
// begins with it).
bool is_proper_prefix(std::string_view prefix, std::string_view whole) {
    return prefix.size() < whole.size() && whole.substr(0, prefix.size()) == prefix;
}

// Emit one ErrorLine (no source file: <file> renders as the literal "yass") to
// `err` and return the spec-assigned exit code for `code`.
int emit_argv_error(std::ostream& err, diag::ErrorCode code, std::string_view arg) {
    diag::Diagnostic d;
    d.file = "";  // empty => the literal token "yass" per cli.ErrorLine
    d.code = code;
    d.message = diag::canonical_message(code, arg);
    err << diag::format_error_line(d) << '\n';
    return diag::exit_for(code);
}

// Canonical subcommands and global flags.
constexpr std::array<std::string_view, 3> kSubcommands = {"validate", "query", "list"};
constexpr std::array<std::string_view, 2> kFlags = {"--help", "--version"};

// Classify a flag-shaped token (it begins with '-', and is neither "-" nor
// "--"). Returns the argv error code for it. The reference rules, established
// empirically against the oracle:
//   - single dash (does not start with "--")        -> short_flag
//   - "--X" whose ASCII-lower equals a global flag   -> case_mismatch
//   - "--X" whose ASCII-lower is a proper prefix of a
//     global flag (and longer than "--")             -> abbreviation
//   - otherwise                                       -> unknown_flag
diag::ErrorCode classify_flag(std::string_view tok) {
    if (tok.size() < 2 || tok[1] != '-') {
        // Single-dash form: "-h", "-v", "-help", ...
        return diag::ErrorCode::argv_short_flag;
    }
    std::string lc = ascii_lower(tok);
    for (std::string_view f : kFlags) {
        if (lc == f) return diag::ErrorCode::argv_case_mismatch;
    }
    if (lc.size() > 2) {  // longer than the bare "--"
        for (std::string_view f : kFlags) {
            if (is_proper_prefix(lc, f)) return diag::ErrorCode::argv_abbreviation;
        }
    }
    return diag::ErrorCode::argv_unknown_flag;
}

// Classify the subcommand token (the first positional). Returns nullopt when it
// is an exact canonical subcommand (caller dispatches); otherwise the argv
// error code. Established empirically:
//   - exact canonical                                  -> dispatch (nullopt)
//   - ASCII-lower equals a canonical (wrong case)       -> case_mismatch
//   - ASCII-lower is a proper prefix of a canonical     -> abbreviation
//   - otherwise                                          -> unknown_subcommand
std::optional<diag::ErrorCode> classify_subcommand(std::string_view tok) {
    for (std::string_view s : kSubcommands) {
        if (tok == s) return std::nullopt;
    }
    std::string lc = ascii_lower(tok);
    for (std::string_view s : kSubcommands) {
        if (lc == s) return diag::ErrorCode::argv_case_mismatch;
    }
    for (std::string_view s : kSubcommands) {
        if (is_proper_prefix(lc, s)) return diag::ErrorCode::argv_abbreviation;
    }
    return diag::ErrorCode::argv_unknown_subcommand;
}

}  // namespace

int run_dispatch(const std::vector<std::string>& argv_tail, std::ostream& out, std::ostream& err,
                 int list_tty_width) {
    // --------------------------------------------------------------------
    // Phase 1 — global --help / --version scan.
    //
    // --help and --version are recognized ANYWHERE in argv, regardless of the
    // `--` end-of-options marker and regardless of any argv error that would
    // otherwise fire. --help wins over --version when both appear. Only the
    // EXACT lowercase spellings count here; wrong-case / abbreviated spellings
    // fall through to the argv-error classification below.
    // --------------------------------------------------------------------
    bool saw_help = false;
    bool saw_version = false;
    for (const std::string& tok : argv_tail) {
        if (tok == "--help") saw_help = true;
        else if (tok == "--version") saw_version = true;
    }
    if (saw_help) {
        out << kUsageText;
        return diag::ExitCode::SUCCESS;
    }
    if (saw_version) {
        out << "yass " << YASS_VERSION << '\n';
        return diag::ExitCode::SUCCESS;
    }

    // --------------------------------------------------------------------
    // Locate the single global `--` end-of-options marker (the FIRST "--"
    // token). Tokens at and after it are no longer flag-classified; the marker
    // itself is consumed and not passed to any handler. Empty-string and bare
    // "-" tokens remain argv errors even past the marker.
    // --------------------------------------------------------------------
    std::optional<std::size_t> dd_idx;
    for (std::size_t i = 0; i < argv_tail.size(); ++i) {
        if (argv_tail[i] == "--") {
            dd_idx = i;
            break;
        }
    }
    auto past_marker = [&](std::size_t i) { return dd_idx.has_value() && i > *dd_idx; };

    // --------------------------------------------------------------------
    // Phase 2 — token-level argv errors, left to right; the FIRST offending
    // token wins. These take precedence over the subcommand classification
    // (e.g. `bogus -h` reports short_flag, not unknown_subcommand).
    //   - empty string            -> empty_argument (even past the marker)
    //   - bare "-"                -> stdin_dash      (even past the marker)
    //   - flag-shaped, pre-marker -> classify_flag (short/case/abbrev/unknown)
    // --------------------------------------------------------------------
    for (std::size_t i = 0; i < argv_tail.size(); ++i) {
        if (dd_idx && i == *dd_idx) continue;  // the marker is not classified
        const std::string& tok = argv_tail[i];
        if (tok.empty()) {
            return emit_argv_error(err, diag::ErrorCode::argv_empty_argument, tok);
        }
        if (tok == "-") {
            return emit_argv_error(err, diag::ErrorCode::argv_stdin_dash, tok);
        }
        if (!past_marker(i) && tok[0] == '-' && tok != "--") {
            return emit_argv_error(err, classify_flag(tok), tok);
        }
    }

    // --------------------------------------------------------------------
    // Phase 3 — subcommand selection. Positionals are all tokens except the
    // consumed `--` marker. The first positional is the subcommand; the rest
    // are the handler's arguments (verbatim, marker already removed).
    // --------------------------------------------------------------------
    std::vector<std::string> positionals;
    positionals.reserve(argv_tail.size());
    for (std::size_t i = 0; i < argv_tail.size(); ++i) {
        if (dd_idx && i == *dd_idx) continue;
        positionals.push_back(argv_tail[i]);
    }

    if (positionals.empty()) {
        return emit_argv_error(err, diag::ErrorCode::argv_no_subcommand, {});
    }

    const std::string& sub = positionals.front();
    std::vector<std::string> handler_args(positionals.begin() + 1, positionals.end());

    std::optional<diag::ErrorCode> sub_err = classify_subcommand(sub);
    if (sub_err) {
        return emit_argv_error(err, *sub_err, sub);
    }

    // --------------------------------------------------------------------
    // Phase 4 — dispatch. Any uncaught exception from a handler becomes one
    // yass.internal.uncaught ErrorLine and exit 1 (cli.Dispatch INVARIANT).
    // --------------------------------------------------------------------
    try {
        if (sub == "validate") return yass::validate::run_validate(handler_args, out, err);
        if (sub == "query") return yass::query::run_query(handler_args, out, err);
        // sub == "list"
        return yass::list::run_list(handler_args, out, err, list_tty_width);
    } catch (const std::exception& e) {
        return emit_argv_error(err, diag::ErrorCode::internal_uncaught, e.what());
    } catch (...) {
        return emit_argv_error(err, diag::ErrorCode::internal_uncaught, "unknown error");
    }
}

int signal_exit_code(int sig) {
    switch (sig) {
        case SIGINT:
            return diag::ExitCode::ON_SIGINT;  // 130
        case SIGTERM:
            return diag::ExitCode::ON_SIGTERM;  // 143
        default:
            return diag::ExitCode::SUCCESS;
    }
}

}  // namespace yass::dispatch
