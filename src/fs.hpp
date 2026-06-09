#pragma once

// M4 — filesystem discovery primitives.
//
// Implements three shared behaviors from spec/cli.shared.yass.yaml:
//   - cli.FindProjectRoot   -> find_project_root
//   - cli.DiscoverSpecFiles -> discover_spec_files
//   - cli.ExpandGlob        -> expand_glob
//
// Error codes (spec/cli.errors.yass.yaml :: cli.errors RETURN):
//   yass.findroot.no_marker           (find_project_root)
//   yass.path.not_found               (discover_spec_files, top-level arg)
//   yass.path.bad_extension           (discover_spec_files, file arg)
//   yass.path.unreadable + exit 2     (discover_spec_files, unreadable arg)
//   yass.path.invalid_type            (discover_spec_files, neither file nor dir)
//   yass.discover.dir_unreadable      (discover_spec_files, non-fatal per dir)
//   yass.glob.no_match + exit 2       (expand_glob)
//
// Output path forms route through diag::relativize_path (relative without ./
// under cwd, basename when directly in cwd, absolute when not under cwd, forward
// slashes, no realpath). Sort keys use textio::nfc + textio::codepoint_less.
//
// These functions perform NO direct I/O to stdout/stderr; they return values and
// diagnostics, leaving emission and process-exit to the calling subcommand.

#include <filesystem>
#include <optional>
#include <string>
#include <string_view>
#include <vector>

#include "diag.hpp"

namespace yass::fs {

// --------------------------------------------------------------------------
// cli.FindProjectRoot
// --------------------------------------------------------------------------
struct FindRootResult {
    // The absolute project-root path (lexically normalized, no realpath) when
    // ok() is true.
    std::string root;
    // Set when no marker was found: yass.findroot.no_marker (exit 2). Carries no
    // associated input file (Diagnostic.file empty -> emitted as "yass").
    std::optional<diag::Diagnostic> error;

    bool ok() const { return !error.has_value(); }
};

// Find the project root per cli.FindProjectRoot.
//
// Search upward INCLUSIVE from `start_dir` (the starting directory is inspected
// before its parent) toward the filesystem root:
//   1. If any ancestor up to and including the filesystem root contains a `.git`
//      entry, return the DEEPEST such ancestor and stop.
//   2. Otherwise restart from `start_dir` inclusive and return the DEEPEST
//      ancestor containing any `*.yass.yaml` file.
//   3. If neither marker is found, error yass.findroot.no_marker.
//
// `.yass.yaml` markers are NOT honored when ANY `.git` marker exists anywhere on
// the upward path. No horizontal traversal and no descent into children.
//
// The returned root is an absolute, lexically-normalized path. When `start_dir`
// is empty, the current working directory is used (cli.FindProjectRoot INPUT:
// WHEN no starting path is provided MUST default to the cwd).
FindRootResult find_project_root(std::string_view start_dir = {});

// --------------------------------------------------------------------------
// cli.DiscoverSpecFiles
// --------------------------------------------------------------------------
struct DiscoverResult {
    // Discovered spec-file paths, already in cli.ErrorLine path form relative to
    // `cwd`, sorted by NFC code-point order. Populated when ok() is true.
    std::vector<std::string> files;
    // A fatal top-level error (path.not_found / bad_extension / unreadable /
    // invalid_type, or findroot.no_marker from the default search). When set,
    // `files` is meaningless.
    std::optional<diag::Diagnostic> error;
    // Non-fatal ErrorLines accumulated during recursion (one
    // yass.discover.dir_unreadable per directory that could not be listed).
    // Present even when ok() is true.
    std::vector<diag::Diagnostic> warnings;

    bool ok() const { return !error.has_value(); }
};

// Discover `.yass.yaml` spec files per cli.DiscoverSpecFiles.
//
//   - `arg` empty / none  -> search from the project root (USES find_project_root
//                            with `cwd` as the starting point).
//   - `arg` a file path   -> return that single file (after stat-classification);
//                            error bad_extension if it lacks the `.yass.yaml`
//                            suffix; not_found if missing; unreadable if it cannot
//                            be accessed.
//   - `arg` a directory   -> recursively find files whose basename has a non-empty
//                            stem before the literal byte suffix `.yass.yaml`,
//                            skipping dotted dirs/files and treating symlinks
//                            encountered during recursion as absent.
//   - `arg` neither file nor directory -> invalid_type.
//
// All returned paths are run through diag::relativize_path against `cwd` and
// sorted by textio::nfc + textio::codepoint_less.
DiscoverResult discover_spec_files(std::string_view arg, std::string_view cwd);

// --------------------------------------------------------------------------
// cli.ExpandGlob
// --------------------------------------------------------------------------
struct GlobResult {
    // Matched paths in cli.ErrorLine path form relative to `cwd` (when the
    // pattern contained metacharacters), or the single literal `arg` unchanged
    // (when it did not). Sorted by NFC code-point order.
    std::vector<std::string> matches;
    // Set on zero matches for a metacharacter pattern: yass.glob.no_match
    // (exit 2). Carries the original pattern as the substitution arg.
    std::optional<diag::Diagnostic> error;

    bool ok() const { return !error.has_value(); }
};

// Expand `arg` per cli.ExpandGlob.
//
//   - No glob metacharacters -> return `arg` literally, unchanged (single
//     element, NO error even if it does not exist on disk).
//   - With metacharacters -> doublestar match against the filesystem rooted at
//     `cwd`: `*` = any run of non-`/`, `?` = one non-`/`, `[...]` = POSIX bracket
//     expression, `**` = zero or more path segments and MUST be its own segment
//     between `/`. Hidden files / hidden directories (name begins with `.`) are
//     never matched or descended; symlinks are not followed; case-sensitive; no
//     brace / tilde / env / realpath expansion. Zero matches -> yass.glob.no_match.
//
// Matches are emitted in diag::relativize_path form against `cwd` and sorted by
// textio::nfc + textio::codepoint_less.
GlobResult expand_glob(std::string_view arg, std::string_view cwd);

// --------------------------------------------------------------------------
// Helpers exposed for direct unit testing (spec-derived predicates).
// --------------------------------------------------------------------------
// True iff `basename` is a discoverable spec-file name: a non-empty stem
// followed by the literal byte suffix ".yass.yaml", basename NOT beginning with
// '.', case-sensitive. The bare name ".yass.yaml" and plain "*.yaml" do NOT
// match. (cli.DiscoverSpecFiles INVARIANT.)
bool is_spec_basename(std::string_view basename);

// True iff `arg` contains any glob metacharacter ('*', '?', '['). Used to decide
// literal passthrough vs. expansion. (cli.ExpandGlob RETURN.)
bool has_glob_metacharacters(std::string_view arg);

// Match a single doublestar glob `pattern` against a forward-slash `path`
// (relative, no leading slash). Implements `*`, `?`, `[...]`, and segment-level
// `**`. Case-sensitive, '/' never matched by '*'/'?'/bracket. (cli.ExpandGlob.)
bool glob_match(std::string_view pattern, std::string_view path);

}  // namespace yass::fs
