// M6 — the `validate` subcommand (implementation).
//
// See validate.hpp for the spec basis and conformance policy. This file is the
// orchestrator: it resolves the positional path arguments into a deduplicated,
// ordered file set (cli.validate INPUT), runs the M5 structural checks per file
// in the mandated order (cli.validate RETURN), emits the error lines and the
// stdout summary (cli.validate SIDE-EFFECT), and computes the process exit code
// (cli.ExitCode). All I/O goes through the supplied streams; std::exit is never
// called.

#include "validate.hpp"

#include <algorithm>
#include <filesystem>
#include <optional>
#include <ostream>
#include <set>
#include <string>
#include <vector>

#include "check.hpp"
#include "diag.hpp"
#include "fs.hpp"
#include "model.hpp"
#include "textio.hpp"
#include "yaml.hpp"

namespace yass::validate {

namespace sfs = std::filesystem;  // std::filesystem
namespace pfs = yass::fs;         // project filesystem module (src/fs.hpp)
using diag::Diagnostic;
using diag::ErrorCode;

namespace {

// A resolved input file: the cli.ErrorLine display path (relativized against the
// process cwd) and the lexically-normalized absolute path used for reading, for
// dedup, and as the CheckRefs base path.
struct InputFile {
    std::string display;  // cli.ErrorLine <file> form
    std::string abs;      // lexically-normalized absolute path
};

// Lexically absolutize+normalize `p` against `cwd` (no realpath / no symlink
// resolution), mirroring diag::relativize_path's lexical model so the dedup key
// is consistent with the discovery output.
std::string lexical_abs(std::string_view p, const sfs::path& cwd) {
    sfs::path path{std::string(p)};
    if (path.is_relative()) path = cwd / path;
    return path.lexically_normal().generic_string();
}

Diagnostic make_diag(std::string_view file, std::optional<int> line, ErrorCode code,
                     std::string_view arg = {}) {
    Diagnostic d;
    d.file.assign(file);
    d.line = line;
    d.code = code;
    d.message = diag::canonical_message(code, arg);
    return d;
}

// Emit one error line + '\n' to `err`, count it toward M.
void emit(std::ostream& err, const Diagnostic& d, int& m) {
    err << diag::format_error_line(d) << '\n';
    ++m;
}

// Write the stdout summary line (the LAST bytes on any stream). When no files
// were checked the spec mandates the exact `checked 0 files, found 0 errors`.
void write_summary(std::ostream& out, int n, int m) {
    out << "checked " << n << " files, found " << m << " errors\n";
}

// The terminal result of input resolution.
struct ResolveResult {
    std::vector<InputFile> files;     // ordered, deduplicated
    std::optional<Diagnostic> error;  // a fatal argv / file-input failure
    std::string project_root;         // computed exactly once when ok()
    // Non-fatal per-directory discovery warnings (yass.discover.dir_unreadable),
    // accumulated across the default search and any directory arguments. Emitted
    // to stderr; they do NOT count toward the M error total.
    std::vector<Diagnostic> warnings;
    bool ok() const { return !error.has_value(); }
};

// Resolve the positional `args` into the ordered, deduplicated file set, per
// cli.validate INPUT. Errors (colon / not_found / bad_extension / unreadable /
// glob.no_match / no_files / findroot) are returned as a single fatal diagnostic
// (exit 2) and stop resolution. The project root is computed exactly ONCE.
ResolveResult resolve_inputs(const std::vector<std::string>& args, const sfs::path& cwd) {
    ResolveResult res;
    const std::string cwd_str = cwd.generic_string();

    // (INPUT) Reject any positional argument containing a literal ':' BEFORE any
    // other work — the reference rejects colon ahead of findroot.
    for (const std::string& a : args) {
        if (a.find(':') != std::string::npos) {
            res.error = make_diag("", std::nullopt, ErrorCode::path_colon_in_path, a);
            return res;
        }
    }

    // (INVARIANT) Establish the project root exactly once via cli.FindProjectRoot
    // using the cwd as the starting point. On failure: one ErrorLine
    // `<cwd>: [yass.findroot.no_marker] ...` and exit 2 BEFORE checking any file.
    pfs::FindRootResult root = pfs::find_project_root(cwd_str);
    if (!root.ok()) {
        Diagnostic d = *root.error;
        d.file = cwd_str;  // attribute to the absolute cwd, per the reference.
        res.error = std::move(d);
        return res;
    }
    res.project_root = root.root;

    // Dedup by lexically-normalized absolute path; first occurrence wins, order
    // preserved (the reference processes arguments in argument order, sorting the
    // discovery/glob results WITHIN each argument).
    std::set<std::string> seen;
    auto add_file = [&](const std::string& display, const std::string& abs) {
        if (seen.insert(abs).second) {
            res.files.push_back(InputFile{display, abs});
        }
    };

    if (args.empty()) {
        // (INPUT) No file paths -> discover from the project root.
        pfs::DiscoverResult dr = pfs::discover_spec_files({}, cwd_str);
        if (!dr.ok()) {
            res.error = dr.error;
            return res;
        }
        for (const Diagnostic& w : dr.warnings) res.warnings.push_back(w);
        for (const std::string& f : dr.files) add_file(f, lexical_abs(f, cwd));
    } else {
        for (const std::string& arg : args) {
            if (pfs::has_glob_metacharacters(arg)) {
                // (INPUT) Resolve glob expansion BEFORE the file-vs-directory
                // distinction. A glob-expanded path is processed ONLY when it is
                // an existing .yass.yaml file; anything else (non-spec file, a
                // directory) is skipped silently (ERROR).
                pfs::GlobResult gr = pfs::expand_glob(arg, cwd_str);
                if (!gr.ok()) {
                    // (cli.ErrorLine) attribute the glob.no_match error's <file> to
                    // the pattern, matching the reference (the fs module leaves it
                    // empty, which would render as the bare "yass" token).
                    Diagnostic d = *gr.error;
                    if (d.file.empty()) d.file = arg;
                    res.error = std::move(d);
                    return res;
                }
                for (const std::string& m : gr.matches) {
                    std::string abs = lexical_abs(m, cwd);
                    std::error_code ec;
                    if (!sfs::is_regular_file(abs, ec)) continue;
                    sfs::path base = sfs::path(abs).filename();
                    if (!pfs::is_spec_basename(base.string())) continue;
                    add_file(m, abs);
                }
                continue;
            }

            // A literal (non-glob) argument: classify by on-disk type.
            std::string abs = lexical_abs(arg, cwd);
            std::error_code ec;
            sfs::file_status st = sfs::status(abs, ec);
            if (ec || !sfs::exists(st)) {
                // not_found (a path that does not exist).
                res.error = make_diag(diag::relativize_path(arg, cwd_str), std::nullopt,
                                      ErrorCode::path_not_found, arg);
                return res;
            }
            if (sfs::is_directory(st)) {
                // (INPUT) Existing directory -> recursively discover .yass.yaml.
                pfs::DiscoverResult dr = pfs::discover_spec_files(arg, cwd_str);
                if (!dr.ok()) {
                    res.error = dr.error;
                    return res;
                }
                for (const Diagnostic& w : dr.warnings) res.warnings.push_back(w);
                for (const std::string& f : dr.files) add_file(f, lexical_abs(f, cwd));
                continue;
            }
            if (sfs::is_regular_file(st)) {
                // (INPUT) Existing file -> validate directly; bad_extension when
                // it lacks the `.yass.yaml` suffix.
                sfs::path base = sfs::path(abs).filename();
                if (!pfs::is_spec_basename(base.string())) {
                    res.error = make_diag(diag::relativize_path(arg, cwd_str), std::nullopt,
                                          ErrorCode::path_bad_extension, arg);
                    return res;
                }
                add_file(diag::relativize_path(arg, cwd_str), abs);
                continue;
            }
            // Neither a regular file nor a directory that we could read.
            res.error = make_diag(diag::relativize_path(arg, cwd_str), std::nullopt,
                                  ErrorCode::path_unreadable, arg);
            return res;
        }
    }

    // (ERROR) No .yass.yaml files found after expansion and discovery.
    if (res.files.empty()) {
        res.error = make_diag("", std::nullopt, ErrorCode::discover_no_files);
        return res;
    }
    return res;
}

// Run the full check pipeline on ONE file and append its error lines (already
// stable-sorted by line) to `err`, counting them toward `m`. Per cli.validate
// RETURN: CheckYAML first; on YAML failure skip the structural checks and count
// the single CheckYAML error.
void check_one_file(const InputFile& file, const std::string& project_root, std::ostream& err,
                    int& m) {
    // Read the file's raw bytes. A read failure here is treated as a
    // well-formedness (CheckYAML) error per cli.validate (torn-read -> CheckYAML)
    // — but discovery already classified the path, so this is the rare race case.
    textio::ReadResult rr = textio::read_file_bytes(file.abs);
    if (!rr.ok()) {
        // Surface as a CheckYAML-style malformed error (one error for the file).
        emit(err, make_diag(file.display, std::nullopt, ErrorCode::yaml_malformed), m);
        return;
    }

    yaml::CheckYamlResult yr = yaml::check_yaml(file.display, rr.bytes);
    if (!yr.ok) {
        // (RETURN) A CheckYAML failure counts as exactly ONE error for the file;
        // CheckPreamble/CheckSpec/CheckUniqueness/CheckRefs are skipped.
        emit(err, *yr.error, m);
        return;
    }

    model::Model model = model::extract(*yr.stream, rr.bytes);

    std::vector<Diagnostic> diags;
    if (auto pre = check::check_preamble(file.display, *yr.stream, rr.bytes)) {
        diags.push_back(std::move(*pre));
    }
    {
        auto sp = check::check_specs(file.display, model);
        diags.insert(diags.end(), sp.begin(), sp.end());
    }
    {
        auto uq = check::check_uniqueness(file.display, model);
        diags.insert(diags.end(), uq.begin(), uq.end());
    }
    {
        // CheckRefs uses the project root + the file's own absolute path as base.
        auto rf = check::check_refs(file.display, model, file.abs, project_root);
        diags.insert(diags.end(), rf.begin(), rf.end());
    }

    // (RETURN) Emit error lines in (file, line, column) ascending order within
    // each file. Stable-sort by line preserves the within-check emission order
    // (which already matches the reference's intra-line column order); diagnostics
    // without a line sort first.
    std::stable_sort(diags.begin(), diags.end(), [](const Diagnostic& a, const Diagnostic& b) {
        int la = a.line.value_or(0);
        int lb = b.line.value_or(0);
        return la < lb;
    });

    for (const Diagnostic& d : diags) emit(err, d, m);
}

}  // namespace

int run_validate(const std::vector<std::string>& args, std::ostream& out, std::ostream& err) {
    std::error_code ec;
    sfs::path cwd = sfs::current_path(ec);
    if (ec) cwd = sfs::path(".");
    cwd = cwd.lexically_normal();

    ResolveResult resolved = resolve_inputs(args, cwd);
    if (!resolved.ok()) {
        // A fatal argv / file-input failure: emit the one error line, then the
        // summary (`checked 0 files, found 0 errors`) as the final bytes, exit 2.
        err << diag::format_error_line(*resolved.error) << '\n';
        write_summary(out, /*n=*/0, /*m=*/0);
        return diag::ExitCode::USAGE;
    }

    // (ERROR / cli.DiscoverSpecFiles) Emit any non-fatal per-directory warnings
    // (yass.discover.dir_unreadable) to stderr BEFORE the per-file errors,
    // matching the reference ordering. These do NOT count toward M.
    for (const Diagnostic& w : resolved.warnings) {
        err << diag::format_error_line(w) << '\n';
    }

    // N = number of files discovery returned (every file in the resolved set;
    // referenced-only files are NOT counted because they never enter this set).
    const int n = static_cast<int>(resolved.files.size());
    int m = 0;  // M = number of error lines emitted to stderr.

    // (INVARIANT) The project root was computed exactly once during resolution.
    const std::string& root_str = resolved.project_root;

    // (RETURN) Process files sequentially; (MUST-NOT) never interleave files.
    for (const InputFile& file : resolved.files) {
        check_one_file(file, root_str, err, m);
    }

    // (SIDE-EFFECT) Flush all stderr error lines BEFORE the stdout summary, and
    // write the summary as the final byte sequence on any stream.
    err.flush();
    write_summary(out, n, m);

    // (RETURN) exit 1 when any validation error was found; otherwise 0.
    return m > 0 ? diag::ExitCode::PROCESSING : diag::ExitCode::SUCCESS;
}

}  // namespace yass::validate
