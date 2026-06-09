#include "fs.hpp"

#include <algorithm>
#include <cerrno>
#include <system_error>

#include "textio.hpp"

namespace yass::fs {

namespace fs_ns = std::filesystem;

namespace {

// Lexically absolutize + normalize `p` against `base`, WITHOUT touching the
// filesystem (mirrors diag.cpp's private lexical_absolute; cli.shared MUST-NOT
// resolve realpath).
fs_ns::path lexical_absolute(fs_ns::path p, const fs_ns::path& base) {
    if (p.is_relative()) {
        p = base / p;
    }
    return p.lexically_normal();
}

// The cwd as a lexically-normalized absolute path. When `cwd` is empty, falls
// back to the process current_path (cli.FindProjectRoot INPUT default).
fs_ns::path effective_cwd(std::string_view cwd) {
    if (!cwd.empty()) {
        return fs_ns::path(std::string(cwd)).lexically_normal();
    }
    std::error_code ec;
    fs_ns::path cur = fs_ns::current_path(ec);
    if (ec) {
        return fs_ns::path(".").lexically_normal();
    }
    return cur.lexically_normal();
}

// --------------------------------------------------------------------------
// Symlink-aware classification via symlink_status (lstat) and status (stat).
// cli.DiscoverSpecFiles: classify via the stat result. A provided directory
// symlink is treated AS the directory (follow). A provided file symlink uses
// the symlink path for reporting but reads the target (follow for content).
// A symlink encountered DURING recursion is treated as absent (do not follow).
// --------------------------------------------------------------------------

// True iff the top-level argument `abs` (after following symlinks) is a regular
// file. Uses status() which follows symlinks.
fs_ns::file_status stat_follow(const fs_ns::path& abs, std::error_code& ec) {
    return fs_ns::status(abs, ec);
}

}  // namespace

// ==========================================================================
// is_spec_basename — cli.DiscoverSpecFiles INVARIANT predicates.
// ==========================================================================
bool is_spec_basename(std::string_view basename) {
    static constexpr std::string_view kSuffix = ".yass.yaml";
    // MUST-NOT match basenames beginning with '.'.
    if (basename.empty() || basename.front() == '.') {
        return false;
    }
    // Case-sensitive byte suffix; MUST-NOT match plain ".yaml".
    if (basename.size() < kSuffix.size()) {
        return false;
    }
    std::string_view tail = basename.substr(basename.size() - kSuffix.size());
    if (tail != kSuffix) {
        return false;
    }
    // Require a NON-EMPTY stem before the suffix (the bare name ".yass.yaml"
    // already fails the leading-'.' test above, but a name equal in length to
    // the suffix has an empty stem regardless).
    return basename.size() > kSuffix.size();
}

// ==========================================================================
// cli.FindProjectRoot
// ==========================================================================
namespace {

// True iff directory `dir` contains an entry literally named ".git" (file or
// dir). cli.FindProjectRoot: a `.git` ENTRY (submodules use a .git FILE, so we
// do not require it to be a directory).
bool has_git_entry(const fs_ns::path& dir) {
    std::error_code ec;
    fs_ns::file_status st = fs_ns::symlink_status(dir / ".git", ec);
    if (ec) {
        return false;
    }
    return fs_ns::exists(st);
}

// True iff directory `dir` directly contains any file matching the literal
// suffix ".yass.yaml" (a *.yass.yaml file). No recursion, no descent.
bool has_yass_yaml(const fs_ns::path& dir) {
    std::error_code ec;
    fs_ns::directory_iterator it(dir, fs_ns::directory_options::none, ec);
    if (ec) {
        return false;
    }
    fs_ns::directory_iterator end;
    for (; it != end; it.increment(ec)) {
        if (ec) {
            return false;
        }
        const fs_ns::path& p = it->path();
        std::string name = p.filename().string();
        // For the FindProjectRoot marker the literal byte suffix is what matters
        // (any *.yass.yaml file, including a dotted name, is a marker here).
        static constexpr std::string_view kSuffix = ".yass.yaml";
        if (name.size() >= kSuffix.size() &&
            std::string_view(name).substr(name.size() - kSuffix.size()) == kSuffix) {
            std::error_code ec2;
            // The entry must be (or resolve to) a regular file marker. Use
            // status() to allow a symlinked marker; absence of a file is fine.
            if (fs_ns::is_regular_file(p, ec2)) {
                return true;
            }
        }
    }
    return false;
}

// Walk from `start` upward INCLUSIVE to the filesystem root, calling `pred` on
// each ancestor (start first). Returns the DEEPEST (i.e. first encountered going
// up) ancestor for which `pred` is true, or nullopt.
std::optional<fs_ns::path> deepest_ancestor(const fs_ns::path& start,
                                            bool (*pred)(const fs_ns::path&)) {
    fs_ns::path cur = start;
    for (;;) {
        if (pred(cur)) {
            return cur;
        }
        fs_ns::path parent = cur.parent_path();
        if (parent.empty() || parent == cur) {
            break;  // reached the filesystem root
        }
        cur = parent;
    }
    return std::nullopt;
}

}  // namespace

FindRootResult find_project_root(std::string_view start_dir) {
    FindRootResult result;

    fs_ns::path start = effective_cwd(start_dir);

    // Pass 1: deepest ancestor (inclusive) containing a `.git` entry. When ANY
    // `.git` exists on the upward path, it WINS over any `.yass.yaml` marker.
    if (auto git = deepest_ancestor(start, &has_git_entry)) {
        result.root = git->lexically_normal().generic_string();
        return result;
    }

    // Pass 2 (only when no `.git` anywhere): deepest ancestor (inclusive)
    // containing any `*.yass.yaml` file.
    if (auto yass = deepest_ancestor(start, &has_yass_yaml)) {
        result.root = yass->lexically_normal().generic_string();
        return result;
    }

    // Neither marker found after reaching the filesystem root.
    diag::Diagnostic d;
    d.file = {};  // no associated input file -> emitted as "yass"
    d.code = diag::ErrorCode::findroot_no_marker;
    d.message = diag::canonical_message(diag::ErrorCode::findroot_no_marker);
    result.error = std::move(d);
    return result;
}

// ==========================================================================
// cli.DiscoverSpecFiles
// ==========================================================================
namespace {

// Recursively collect spec files under `dir` into `out` (absolute paths), per
// the cli.DiscoverSpecFiles INVARIANTs:
//   - do NOT follow symlinks (symlink entries are treated as absent),
//   - do NOT descend into directories whose name begins with '.',
//   - do NOT match basenames beginning with '.',
//   - match the literal byte suffix ".yass.yaml" with a non-empty stem.
// A directory that cannot be listed appends one yass.discover.dir_unreadable
// warning (in cli.ErrorLine path form against `cwd`) and recursion CONTINUES
// with sibling directories (non-fatal).
void recurse_collect(const fs_ns::path& dir, const fs_ns::path& cwd,
                     std::vector<fs_ns::path>& out,
                     std::vector<diag::Diagnostic>& warnings) {
    std::error_code ec;
    // Use a non-following iterator. We classify each entry by symlink_status
    // (lstat) so symlinks are detected and skipped (treated as absent).
    fs_ns::directory_iterator it(dir, fs_ns::directory_options::none, ec);
    if (ec) {
        // cli.DiscoverSpecFiles ERROR (CONFORMS cli.ErrorLine): the <file> prefix
        // is the unreadable directory itself (relativized), matching the
        // reference `locked: [yass.discover.dir_unreadable] cannot read
        // directory: locked`. The message substitutes the same relativized path.
        std::string rel = diag::relativize_path(dir.generic_string(), cwd.generic_string());
        diag::Diagnostic d;
        d.file = rel;
        d.code = diag::ErrorCode::discover_dir_unreadable;
        d.message = diag::canonical_message(diag::ErrorCode::discover_dir_unreadable, rel);
        warnings.push_back(std::move(d));
        return;
    }

    // Collect entries first so a mid-iteration error on one entry does not abort
    // the rest; iterate defensively.
    fs_ns::directory_iterator end;
    std::vector<fs_ns::path> subdirs;
    for (; it != end; it.increment(ec)) {
        if (ec) {
            // An error advancing the iterator: treat as a listing failure for
            // this directory and stop (non-fatal, sibling dirs continue via the
            // caller's loop). Emit one warning with the directory as the <file>
            // prefix (CONFORMS cli.ErrorLine), matching the reference.
            std::string rel = diag::relativize_path(dir.generic_string(), cwd.generic_string());
            diag::Diagnostic d;
            d.file = rel;
            d.code = diag::ErrorCode::discover_dir_unreadable;
            d.message = diag::canonical_message(diag::ErrorCode::discover_dir_unreadable, rel);
            warnings.push_back(std::move(d));
            return;
        }
        const fs_ns::path entry = it->path();
        std::string name = entry.filename().string();

        std::error_code lec;
        fs_ns::file_status lst = fs_ns::symlink_status(entry, lec);
        if (lec) {
            continue;  // cannot stat the entry: treat as absent
        }
        // A symlink encountered during recursion is treated as ABSENT.
        if (fs_ns::is_symlink(lst)) {
            continue;
        }
        if (fs_ns::is_directory(lst)) {
            // MUST-NOT descend into directories whose name begins with '.'.
            if (!name.empty() && name.front() == '.') {
                continue;
            }
            subdirs.push_back(entry);
            continue;
        }
        if (fs_ns::is_regular_file(lst)) {
            if (is_spec_basename(name)) {
                out.push_back(entry);
            }
            continue;
        }
        // Any other type (fifo, socket, block/char device) is not a spec file
        // and not a directory to descend; ignore it during recursion.
    }

    // Descend into subdirectories after fully listing the current level. Sort is
    // applied once at the end on the full result set, so descent order is
    // irrelevant.
    for (const fs_ns::path& sub : subdirs) {
        recurse_collect(sub, cwd, out, warnings);
    }
}

// Build a fatal Diagnostic for a top-level path arg.
diag::Diagnostic top_level_error(diag::ErrorCode code, const fs_ns::path& reported,
                                 const fs_ns::path& cwd) {
    diag::Diagnostic d;
    std::string rel = diag::relativize_path(reported.generic_string(), cwd.generic_string());
    d.file = rel;
    d.code = code;
    d.message = diag::canonical_message(code, rel);
    return d;
}

}  // namespace

DiscoverResult discover_spec_files(std::string_view arg, std::string_view cwd) {
    DiscoverResult result;
    fs_ns::path base = effective_cwd(cwd);

    std::vector<fs_ns::path> collected;  // absolute, pre-relativize

    if (arg.empty()) {
        // No path provided: search from the project root.
        FindRootResult fr = find_project_root(base.generic_string());
        if (!fr.ok()) {
            result.error = fr.error;  // yass.findroot.no_marker (exit 2)
            return result;
        }
        recurse_collect(fs_ns::path(fr.root), base, collected, result.warnings);
    } else {
        // A path WAS provided. The argument is used verbatim for reporting (no
        // tilde/env expansion, no realpath). Absolutize lexically only for stat.
        fs_ns::path reported{std::string(arg)};
        fs_ns::path abs = lexical_absolute(reported, base);

        // Classify via stat (follows symlinks: a provided file/dir symlink is
        // followed for content/traversal). Distinguish not-found vs unreadable.
        std::error_code ec;
        fs_ns::file_status st = stat_follow(abs, ec);
        if (ec) {
            // status() failed. Missing -> not_found; otherwise unreadable.
            if (st.type() == fs_ns::file_type::not_found ||
                ec == std::errc::no_such_file_or_directory) {
                result.error = top_level_error(diag::ErrorCode::path_not_found, reported, base);
            } else {
                result.error = top_level_error(diag::ErrorCode::path_unreadable, reported, base);
            }
            return result;
        }

        fs_ns::file_type type = st.type();
        if (type == fs_ns::file_type::not_found) {
            result.error = top_level_error(diag::ErrorCode::path_not_found, reported, base);
            return result;
        }
        if (type == fs_ns::file_type::regular) {
            // A provided FILE path: require the literal ".yass.yaml" suffix
            // (case-sensitive). The basename '.'-prefix rule does NOT apply to an
            // explicitly-provided file (only to recursive discovery), so a bare
            // ".yass.yaml" file arg is accepted as long as it has the suffix.
            std::string name = reported.filename().string();
            static constexpr std::string_view kSuffix = ".yass.yaml";
            bool has_suffix = name.size() >= kSuffix.size() &&
                              std::string_view(name).substr(name.size() - kSuffix.size()) ==
                                  kSuffix;
            if (!has_suffix) {
                result.error =
                    top_level_error(diag::ErrorCode::path_bad_extension, reported, base);
                return result;
            }
            // Report the (symlink) path itself; content is read elsewhere from
            // the target. Here we only produce the reported path.
            collected.push_back(abs);
        } else if (type == fs_ns::file_type::directory) {
            // A provided DIRECTORY (possibly a symlink, already followed by
            // status): recursively search within it.
            recurse_collect(abs, base, collected, result.warnings);
        } else {
            // Neither a regular file nor a directory -> filesystem error.
            result.error = top_level_error(diag::ErrorCode::path_invalid_type, reported, base);
            return result;
        }
    }

    // Relativize + sort by NFC code-point order on the path string.
    std::vector<std::string> out;
    out.reserve(collected.size());
    for (const fs_ns::path& p : collected) {
        out.push_back(diag::relativize_path(p.generic_string(), base.generic_string()));
    }
    std::sort(out.begin(), out.end(), [](const std::string& a, const std::string& b) {
        return textio::codepoint_less(textio::nfc(a), textio::nfc(b));
    });
    result.files = std::move(out);
    return result;
}

// ==========================================================================
// cli.ExpandGlob
// ==========================================================================
bool has_glob_metacharacters(std::string_view arg) {
    for (char c : arg) {
        if (c == '*' || c == '?' || c == '[') {
            return true;
        }
    }
    return false;
}

namespace {

// Match one path SEGMENT (no '/') against one pattern segment containing `*`,
// `?`, and `[...]` (POSIX bracket). Implemented as a backtracking matcher.
// `*` matches any run of characters (the caller guarantees neither contains
// '/'). Case-sensitive.
bool segment_match(std::string_view pat, std::string_view str) {
    std::size_t pi = 0, si = 0;
    std::size_t star_pi = std::string_view::npos;  // last '*' position in pat
    std::size_t star_si = 0;                       // str position when '*' seen

    auto match_bracket = [](std::string_view p, std::size_t& bi, char c) -> bool {
        // p[bi] == '['. Parse [...]; on a malformed bracket (no closing ']'),
        // treat '[' literally. Returns whether `c` matches; advances `bi` past
        // the closing ']' on success.
        std::size_t j = bi + 1;
        bool negate = false;
        if (j < p.size() && (p[j] == '!' || p[j] == '^')) {
            negate = true;
            ++j;
        }
        // A ']' immediately after the (optional) negation is a literal member.
        bool matched = false;
        bool first = true;
        std::size_t start = j;
        bool closed = false;
        while (j < p.size()) {
            char pc = p[j];
            if (pc == ']' && !first) {
                closed = true;
                break;
            }
            first = false;
            // Range a-b (b present and not the closing bracket).
            if (j + 2 < p.size() && p[j + 1] == '-' && p[j + 2] != ']') {
                char lo = pc;
                char hi = p[j + 2];
                if (static_cast<unsigned char>(c) >= static_cast<unsigned char>(lo) &&
                    static_cast<unsigned char>(c) <= static_cast<unsigned char>(hi)) {
                    matched = true;
                }
                j += 3;
            } else {
                if (c == pc) {
                    matched = true;
                }
                ++j;
            }
        }
        if (!closed) {
            (void)start;
            return false;  // signal malformed; caller treats '[' literally
        }
        bi = j + 1;  // past ']'
        return negate ? !matched : matched;
    };

    while (si < str.size()) {
        if (pi < pat.size() && pat[pi] == '[') {
            std::size_t save = pi;
            bool ok = match_bracket(pat, pi, str[si]);
            if (pi == save) {
                // malformed bracket: treat '[' as a literal character
                if (pat[pi] == str[si]) {
                    ++pi;
                    ++si;
                    continue;
                }
                // fallthrough to star backtrack below
            } else {
                if (ok) {
                    ++si;
                    continue;
                }
                // bracket did not match: backtrack to last star if any
                if (star_pi != std::string_view::npos) {
                    pi = star_pi + 1;
                    si = ++star_si;
                    continue;
                }
                return false;
            }
        } else if (pi < pat.size() && (pat[pi] == '?' || pat[pi] == str[si])) {
            ++pi;
            ++si;
            continue;
        } else if (pi < pat.size() && pat[pi] == '*') {
            star_pi = pi;
            star_si = si;
            ++pi;
            continue;
        }
        // mismatch: backtrack to last '*'
        if (star_pi != std::string_view::npos) {
            pi = star_pi + 1;
            si = ++star_si;
            continue;
        }
        return false;
    }
    // Consume trailing '*' in the pattern.
    while (pi < pat.size() && pat[pi] == '*') {
        ++pi;
    }
    return pi == pat.size();
}

}  // namespace

bool glob_match(std::string_view pattern, std::string_view path) {
    // Split both into '/'-separated segments and match segment-by-segment, with
    // `**` consuming zero or more whole segments.
    std::vector<std::string_view> pseg;
    std::vector<std::string_view> sseg;
    auto split = [](std::string_view s, std::vector<std::string_view>& out) {
        std::size_t start = 0;
        for (std::size_t i = 0; i <= s.size(); ++i) {
            if (i == s.size() || s[i] == '/') {
                out.push_back(s.substr(start, i - start));
                start = i + 1;
            }
        }
    };
    split(pattern, pseg);
    split(path, sseg);

    // Recursive segment matcher with `**` support.
    // pi/si index into pseg/sseg.
    struct M {
        const std::vector<std::string_view>& p;
        const std::vector<std::string_view>& s;
        bool go(std::size_t pi, std::size_t si) const {
            if (pi == p.size()) {
                return si == s.size();
            }
            if (p[pi] == "**") {
                // `**` is its own segment matching zero or more segments. None of
                // the matched segments may be hidden (begin with '.') — hidden
                // dirs are never descended.
                // zero segments:
                if (go(pi + 1, si)) {
                    return true;
                }
                // one or more: consume s[si] (must not be hidden) then retry **.
                if (si < s.size() && !(s[si].size() > 0 && s[si].front() == '.')) {
                    return go(pi, si + 1);
                }
                return false;
            }
            if (si == s.size()) {
                return false;
            }
            // A hidden path segment can only be matched by a literal pattern
            // segment that itself begins with '.', never by a wildcard.
            bool s_hidden = !s[si].empty() && s[si].front() == '.';
            bool p_dot = !p[pi].empty() && p[pi].front() == '.';
            if (s_hidden && !p_dot) {
                return false;
            }
            if (!segment_match(p[pi], s[si])) {
                return false;
            }
            return go(pi + 1, si + 1);
        }
    } m{pseg, sseg};
    return m.go(0, 0);
}

namespace {

// Recursively walk the filesystem from `abs_base` collecting paths whose
// cwd-relative '/'-joined form matches `pattern`. Hidden dirs are never
// descended; hidden files never matched; symlinks never followed. `rel` is the
// path of `abs_base` relative to the glob anchor (empty at the top).
void glob_walk(const fs_ns::path& abs_dir, const std::string& rel,
               std::string_view pattern, std::vector<std::string>& out) {
    std::error_code ec;
    fs_ns::directory_iterator it(abs_dir, fs_ns::directory_options::none, ec);
    if (ec) {
        return;  // unlistable dir: silently skip (glob has no per-dir warning)
    }
    fs_ns::directory_iterator end;
    for (; it != end; it.increment(ec)) {
        if (ec) {
            return;
        }
        const fs_ns::path entry = it->path();
        std::string name = entry.filename().string();
        // MUST-NOT match hidden files / descend hidden directories.
        if (!name.empty() && name.front() == '.') {
            continue;
        }
        std::error_code lec;
        fs_ns::file_status lst = fs_ns::symlink_status(entry, lec);
        if (lec) {
            continue;
        }
        // MUST-NOT follow symlinks.
        if (fs_ns::is_symlink(lst)) {
            continue;
        }
        std::string child_rel = rel.empty() ? name : (rel + "/" + name);
        if (glob_match(pattern, child_rel)) {
            out.push_back(child_rel);
        }
        if (fs_ns::is_directory(lst)) {
            glob_walk(entry, child_rel, pattern, out);
        }
    }
}

}  // namespace

GlobResult expand_glob(std::string_view arg, std::string_view cwd) {
    GlobResult result;

    // No metacharacters: return the literal path unchanged (single element).
    if (!has_glob_metacharacters(arg)) {
        result.matches.push_back(std::string(arg));
        return result;
    }

    fs_ns::path base = effective_cwd(cwd);

    std::vector<std::string> rel_matches;
    glob_walk(base, std::string(), arg, rel_matches);

    if (rel_matches.empty()) {
        diag::Diagnostic d;
        d.file = {};  // no associated input file -> "yass"
        d.code = diag::ErrorCode::glob_no_match;
        d.message = diag::canonical_message(diag::ErrorCode::glob_no_match, arg);
        result.error = std::move(d);
        return result;
    }

    // Emit matches in cli.ErrorLine path form relative to cwd, sorted by NFC
    // code-point order.
    std::vector<std::string> out;
    out.reserve(rel_matches.size());
    for (const std::string& r : rel_matches) {
        fs_ns::path abs = base / r;
        out.push_back(diag::relativize_path(abs.generic_string(), base.generic_string()));
    }
    std::sort(out.begin(), out.end(), [](const std::string& a, const std::string& b) {
        return textio::codepoint_less(textio::nfc(a), textio::nfc(b));
    });
    result.matches = std::move(out);
    return result;
}

}  // namespace yass::fs
