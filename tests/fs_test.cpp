// M4 — filesystem discovery primitives tests.
//
// Each test name cites the spec/slot/obligation it exercises so a failure points
// back to the normative source. Spec basis: spec/cli.shared.yass.yaml
//   - cli.FindProjectRoot   (INPUT / RETURN / ERROR / INVARIANT)
//   - cli.DiscoverSpecFiles (INPUT / RETURN / ERROR / INVARIANT)
//   - cli.ExpandGlob        (INPUT / RETURN / ERROR / INVARIANT)
// plus spec/cli.errors.yass.yaml :: cli.errors RETURN (codes) and
// spec/cli.yass.yaml :: cli.ErrorLine (path-form rules).
//
// Trees are built with the RAII tests/support/tmptree.hpp helper. All paths
// handed to the code under test are absolute (the helper's root), and `cwd` is
// set to the tree root so relativize_path produces basenames / relative forms we
// can assert on directly.

#include "doctest.h"

#include <sys/stat.h>

#include <algorithm>
#include <filesystem>
#include <string>
#include <vector>

#include "diag.hpp"
#include "fs.hpp"
#include "support/tmptree.hpp"

using namespace yass::fs;
using yass::diag::ErrorCode;
using yass::test::TmpTree;
namespace stdfs = std::filesystem;

namespace {

std::string cwd_of(const TmpTree& t) { return t.root().generic_string(); }

bool has_value_code(const std::optional<yass::diag::Diagnostic>& d, ErrorCode c) {
    return d.has_value() && d->code == c;
}

}  // namespace

// ===========================================================================
// cli.FindProjectRoot
// ===========================================================================

// INPUT MAY: accept a starting directory path. RETURN MUST: return absolute path.
TEST_CASE("FindProjectRoot INPUT/RETURN: accepts a starting directory and returns an absolute root") {
    TmpTree t;
    t.write(".git/HEAD", "ref: refs/heads/main\n");
    t.mkdir("a/b/c");
    auto r = find_project_root((t.root() / "a/b/c").generic_string());
    REQUIRE(r.ok());
    CHECK(stdfs::path(r.root).is_absolute());
    CHECK(stdfs::path(r.root).lexically_normal() == t.root().lexically_normal());
}

// RETURN MUST: search upward inclusive; inspect the starting directory itself
// before its parent. A .git in the START directory wins.
TEST_CASE("FindProjectRoot RETURN: inspects the starting directory itself before its parent") {
    TmpTree t;
    // .git at BOTH the start dir and an ancestor; the deepest (start) must win.
    t.write(".git/HEAD", "x");
    t.write("a/.git/HEAD", "x");
    auto r = find_project_root((t.root() / "a").generic_string());
    REQUIRE(r.ok());
    CHECK(stdfs::path(r.root).lexically_normal() == (t.root() / "a").lexically_normal());
}

// RETURN WHEN any ancestor up to/including fs root contains .git MUST: return the
// DEEPEST such ancestor and stop.
TEST_CASE("FindProjectRoot RETURN: returns the deepest ancestor containing a .git entry") {
    TmpTree t;
    t.write(".git/HEAD", "x");      // shallow
    t.write("a/b/.git/HEAD", "x");  // deeper
    t.mkdir("a/b/c/d");
    auto r = find_project_root((t.root() / "a/b/c/d").generic_string());
    REQUIRE(r.ok());
    CHECK(stdfs::path(r.root).lexically_normal() == (t.root() / "a/b").lexically_normal());
}

// RETURN: a .git ENTRY may be a FILE (submodule worktree), not just a directory.
TEST_CASE("FindProjectRoot RETURN: a .git entry that is a file is honored as a marker") {
    TmpTree t;
    t.write(".git", "gitdir: ../.git/modules/x\n");
    t.mkdir("sub");
    auto r = find_project_root((t.root() / "sub").generic_string());
    REQUIRE(r.ok());
    CHECK(stdfs::path(r.root).lexically_normal() == t.root().lexically_normal());
}

// RETURN WHEN no .git anywhere MUST: restart and find the DEEPEST ancestor
// containing any .yass.yaml file.
TEST_CASE("FindProjectRoot RETURN: falls back to the deepest ancestor with a .yass.yaml file when no .git") {
    TmpTree t;
    t.write("a/marker.yass.yaml", "description: x\nversion: v1\n");  // deeper marker
    t.write("root.yass.yaml", "description: x\nversion: v1\n");       // shallow marker
    t.mkdir("a/b/c");
    auto r = find_project_root((t.root() / "a/b/c").generic_string());
    REQUIRE(r.ok());
    CHECK(stdfs::path(r.root).lexically_normal() == (t.root() / "a").lexically_normal());
}

// INVARIANT MUST-NOT: honor .yass.yaml markers when ANY .git exists on the path.
// A deep .yass.yaml must lose to a shallow .git ancestor.
TEST_CASE("FindProjectRoot INVARIANT: .git precedence over a deeper .yass.yaml marker") {
    TmpTree t;
    t.write(".git/HEAD", "x");                                       // shallow .git
    t.write("a/b/deep.yass.yaml", "description: x\nversion: v1\n");  // deeper .yass.yaml
    t.mkdir("a/b/c");
    auto r = find_project_root((t.root() / "a/b/c").generic_string());
    REQUIRE(r.ok());
    // .git wins even though the .yass.yaml ancestor is deeper.
    CHECK(stdfs::path(r.root).lexically_normal() == t.root().lexically_normal());
}

// ERROR MUST: error yass.findroot.no_marker when no marker found up to fs root.
TEST_CASE("FindProjectRoot ERROR: yass.findroot.no_marker when no .git or .yass.yaml anywhere") {
    TmpTree t;
    t.mkdir("a/b/c");
    // No .git and no *.yass.yaml anywhere under the tree; searching from a leaf
    // walks up to the filesystem root without finding a marker. (The tmp tree's
    // ancestors are real fs dirs with no markers either.)
    auto r = find_project_root((t.root() / "a/b/c").generic_string());
    REQUIRE_FALSE(r.ok());
    CHECK(has_value_code(r.error, ErrorCode::findroot_no_marker));
    CHECK(r.error->file.empty());  // no associated input file -> "yass"
}

// INVARIANT MUST-NOT: descend into child directories during the search. A
// .yass.yaml only in a CHILD of the start dir is NOT a marker for the start.
TEST_CASE("FindProjectRoot INVARIANT: does not descend into children (child-only marker is not found)") {
    TmpTree t;
    // marker is in a CHILD of the start dir, not at/above it.
    t.write("start/child/only.yass.yaml", "description: x\nversion: v1\n");
    auto r = find_project_root((t.root() / "start").generic_string());
    // Walking UP from start finds no marker (child is below). Up to fs root: none.
    REQUIRE_FALSE(r.ok());
    CHECK(has_value_code(r.error, ErrorCode::findroot_no_marker));
}

// ===========================================================================
// cli.DiscoverSpecFiles — file vs dir vs default classification
// ===========================================================================

// RETURN WHEN input is a file path MUST: return that single file path.
TEST_CASE("DiscoverSpecFiles RETURN: a provided file path returns that single file") {
    TmpTree t;
    t.write("one.yass.yaml", "description: x\nversion: v1\n");
    auto r = discover_spec_files((t.root() / "one.yass.yaml").generic_string(), cwd_of(t));
    REQUIRE(r.ok());
    REQUIRE(r.files.size() == 1);
    CHECK(r.files[0] == "one.yass.yaml");  // basename: directly inside cwd
}

// RETURN WHEN input is a directory path MUST: recursively search for *.yass.yaml.
TEST_CASE("DiscoverSpecFiles RETURN: a provided directory path is searched recursively") {
    TmpTree t;
    t.write("top.yass.yaml", "x");
    t.write("a/mid.yass.yaml", "x");
    t.write("a/b/deep.yass.yaml", "x");
    t.write("a/notspec.yaml", "x");  // plain .yaml -> not matched
    auto r = discover_spec_files(t.root().generic_string(), cwd_of(t));
    REQUIRE(r.ok());
    std::vector<std::string> expect = {"a/b/deep.yass.yaml", "a/mid.yass.yaml", "top.yass.yaml"};
    CHECK(r.files == expect);
}

// INPUT WHEN no path is provided MUST: search from the project root (USES
// FindProjectRoot). Make the tree root a project root via .git, cwd = root.
TEST_CASE("DiscoverSpecFiles INPUT: no path searches from the project root") {
    TmpTree t;
    t.write(".git/HEAD", "x");
    t.write("p.yass.yaml", "x");
    t.write("d/q.yass.yaml", "x");
    auto r = discover_spec_files({}, cwd_of(t));
    REQUIRE(r.ok());
    std::vector<std::string> expect = {"d/q.yass.yaml", "p.yass.yaml"};
    CHECK(r.files == expect);
}

// ===========================================================================
// cli.DiscoverSpecFiles — INVARIANT suffix / hidden matching rules
// ===========================================================================

// INVARIANT MUST-NOT: descend into directories whose name begins with '.'.
// INVARIANT MUST-NOT: match files whose basename begins with '.'.
TEST_CASE("DiscoverSpecFiles INVARIANT: skips dotted directories and dotted basenames") {
    TmpTree t;
    t.write("good.yass.yaml", "x");
    t.write(".hiddendir/inside.yass.yaml", "x");  // dotted dir -> not descended
    t.write("sub/.hidden.yass.yaml", "x");        // dotted basename -> not matched
    auto r = discover_spec_files(t.root().generic_string(), cwd_of(t));
    REQUIRE(r.ok());
    std::vector<std::string> expect = {"good.yass.yaml"};
    CHECK(r.files == expect);
}

// INVARIANT MUST: require a non-empty basename before .yass.yaml; the bare
// ".yass.yaml" MUST NOT match during recursion.
TEST_CASE("DiscoverSpecFiles INVARIANT: bare \".yass.yaml\" is not matched in recursion") {
    TmpTree t;
    t.write(".yass.yaml", "x");      // bare -> NOT matched (also dotted basename)
    t.write("real.yass.yaml", "x");  // matched
    auto r = discover_spec_files(t.root().generic_string(), cwd_of(t));
    REQUIRE(r.ok());
    std::vector<std::string> expect = {"real.yass.yaml"};
    CHECK(r.files == expect);
}

// INVARIANT MUST-NOT: match files ending only in .yaml without the .yass prefix.
TEST_CASE("DiscoverSpecFiles INVARIANT: plain .yaml without .yass prefix is not matched") {
    TmpTree t;
    t.write("config.yaml", "x");        // not matched
    t.write("thing.yass.yaml", "x");    // matched
    t.write("trap.yass.yaml.bak", "x"); // suffix not at end -> not matched
    auto r = discover_spec_files(t.root().generic_string(), cwd_of(t));
    REQUIRE(r.ok());
    std::vector<std::string> expect = {"thing.yass.yaml"};
    CHECK(r.files == expect);
}

// INVARIANT MUST: case-sensitive byte suffix comparison on all platforms.
TEST_CASE("DiscoverSpecFiles INVARIANT: suffix match is case-sensitive (.YASS.YAML not matched)") {
    TmpTree t;
    t.write("upper.YASS.YAML", "x");  // wrong case -> not matched
    t.write("lower.yass.yaml", "x");  // matched
    auto r = discover_spec_files(t.root().generic_string(), cwd_of(t));
    REQUIRE(r.ok());
    std::vector<std::string> expect = {"lower.yass.yaml"};
    CHECK(r.files == expect);
}

// is_spec_basename helper directly: the spec's matching predicate edge cases.
TEST_CASE("DiscoverSpecFiles INVARIANT: is_spec_basename predicate edge cases") {
    CHECK(is_spec_basename("a.yass.yaml"));
    CHECK(is_spec_basename("x-y_z.yass.yaml"));
    CHECK_FALSE(is_spec_basename(".yass.yaml"));       // bare / dotted
    CHECK_FALSE(is_spec_basename(".x.yass.yaml"));     // dotted basename
    CHECK_FALSE(is_spec_basename("a.yaml"));           // no .yass
    CHECK_FALSE(is_spec_basename("a.yass.yaml.bak"));  // suffix not at end
    CHECK_FALSE(is_spec_basename("A.YASS.YAML"));      // case
    CHECK_FALSE(is_spec_basename(""));                 // empty
}

// ===========================================================================
// cli.DiscoverSpecFiles — sorting (RETURN sort rules)
// ===========================================================================

// RETURN MUST: sort by Unicode code-point order on NFC-normalized UTF-8 path.
// MUST-NOT: case-fold (uppercase sorts before lowercase by code point).
TEST_CASE("DiscoverSpecFiles RETURN: results sorted by code-point order, no case-fold") {
    TmpTree t;
    // Code-point order: digits (0x3x) < uppercase (0x4x/0x5x) < lowercase (0x6x).
    t.write("banana.yass.yaml", "x");
    t.write("Apple.yass.yaml", "x");
    t.write("Zebra.yass.yaml", "x");
    t.write("9nine.yass.yaml", "x");
    auto r = discover_spec_files(t.root().generic_string(), cwd_of(t));
    REQUIRE(r.ok());
    std::vector<std::string> expect = {"9nine.yass.yaml", "Apple.yass.yaml", "Zebra.yass.yaml",
                                       "banana.yass.yaml"};
    CHECK(r.files == expect);
}

// ===========================================================================
// cli.DiscoverSpecFiles — symlink behavior
// ===========================================================================

// INVARIANT MUST-NOT: follow symbolic links during recursive traversal.
// WHEN a symlink is encountered during recursion MUST: treat that entry as absent.
TEST_CASE("DiscoverSpecFiles INVARIANT: symlinks encountered during recursion are treated as absent") {
    TmpTree t;
    t.write("real.yass.yaml", "x");
    t.write("target/inside.yass.yaml", "x");
    // A symlink FILE that points at a real .yass.yaml -> absent (not matched).
    t.symlink("link.yass.yaml", t.root() / "real.yass.yaml");
    // A symlink DIR that points at a dir containing a .yass.yaml -> not followed.
    t.symlink("linkdir", t.root() / "target");
    auto r = discover_spec_files(t.root().generic_string(), cwd_of(t));
    REQUIRE(r.ok());
    // Only the two REAL files; neither symlink contributes.
    std::vector<std::string> expect = {"real.yass.yaml", "target/inside.yass.yaml"};
    CHECK(r.files == expect);
}

// WHEN a provided directory-path argument is a symlink MUST: treat the symlink
// path itself as the directory for traversal (follow it as a top-level arg).
TEST_CASE("DiscoverSpecFiles INPUT: a provided directory-symlink argument is traversed") {
    TmpTree t;
    t.write("realdir/a.yass.yaml", "x");
    t.symlink("dirlink", t.root() / "realdir");
    auto r = discover_spec_files((t.root() / "dirlink").generic_string(), cwd_of(t));
    REQUIRE(r.ok());
    REQUIRE(r.files.size() == 1);
    // Reported via the symlink path (no realpath), relative to cwd.
    CHECK(r.files[0] == "dirlink/a.yass.yaml");
}

// WHEN a provided file-path argument is a symlink MUST: use the symlink path for
// reporting (content read from target elsewhere).
TEST_CASE("DiscoverSpecFiles INPUT: a provided file-symlink argument reports the symlink path") {
    TmpTree t;
    t.write("real.yass.yaml", "description: x\nversion: v1\n");
    t.symlink("alias.yass.yaml", t.root() / "real.yass.yaml");
    auto r = discover_spec_files((t.root() / "alias.yass.yaml").generic_string(), cwd_of(t));
    REQUIRE(r.ok());
    REQUIRE(r.files.size() == 1);
    CHECK(r.files[0] == "alias.yass.yaml");  // symlink path, NOT realpath
}

// ===========================================================================
// cli.DiscoverSpecFiles — ERROR obligations
// ===========================================================================

// ERROR MUST: yass.path.not_found when a provided file path does not exist.
TEST_CASE("DiscoverSpecFiles ERROR: yass.path.not_found for a missing file path") {
    TmpTree t;
    auto r = discover_spec_files((t.root() / "missing.yass.yaml").generic_string(), cwd_of(t));
    REQUIRE_FALSE(r.ok());
    CHECK(has_value_code(r.error, ErrorCode::path_not_found));
}

// ERROR MUST: yass.path.not_found when a provided directory path does not exist.
TEST_CASE("DiscoverSpecFiles ERROR: yass.path.not_found for a missing directory path") {
    TmpTree t;
    auto r = discover_spec_files((t.root() / "no/such/dir").generic_string(), cwd_of(t));
    REQUIRE_FALSE(r.ok());
    CHECK(has_value_code(r.error, ErrorCode::path_not_found));
}

// ERROR WHEN input is a file path MUST: yass.path.bad_extension when no .yass.yaml.
TEST_CASE("DiscoverSpecFiles ERROR: yass.path.bad_extension for a file lacking the suffix") {
    TmpTree t;
    t.write("notes.txt", "x");
    auto r = discover_spec_files((t.root() / "notes.txt").generic_string(), cwd_of(t));
    REQUIRE_FALSE(r.ok());
    CHECK(has_value_code(r.error, ErrorCode::path_bad_extension));
}

// ERROR: a provided file whose basename begins with '.' but DOES end in
// .yass.yaml is accepted as a file arg (the dotted-basename rule applies only to
// recursive discovery, per the reference). bad_extension must NOT fire.
TEST_CASE("DiscoverSpecFiles ERROR: a provided \".yass.yaml\" file arg is accepted (suffix present)") {
    TmpTree t;
    t.write(".yass.yaml", "description: x\nversion: v1\n");
    auto r = discover_spec_files((t.root() / ".yass.yaml").generic_string(), cwd_of(t));
    REQUIRE(r.ok());
    REQUIRE(r.files.size() == 1);
}

// ERROR WHEN a path resolves to neither a regular file nor a directory MUST:
// treat as a filesystem error (yass.path.invalid_type).
TEST_CASE("DiscoverSpecFiles ERROR: yass.path.invalid_type for a non-file/non-dir entry (FIFO)") {
    TmpTree t;
    stdfs::path fifo = t.root() / "pipe";
    int rc = ::mkfifo(fifo.c_str(), 0644);
    if (rc != 0) {
        WARN_MESSAGE(false, "mkfifo unavailable; skipping invalid_type assertion");
        return;
    }
    auto r = discover_spec_files(fifo.generic_string(), cwd_of(t));
    REQUIRE_FALSE(r.ok());
    CHECK(has_value_code(r.error, ErrorCode::path_invalid_type));
}

// ERROR WHEN a directory cannot be listed during recursion MUST: emit one
// yass.discover.dir_unreadable ErrorLine and CONTINUE traversing siblings
// (non-fatal). MUST-NOT abort discovery on per-directory errors below a top arg.
TEST_CASE("DiscoverSpecFiles ERROR: unreadable subdirectory yields a non-fatal dir_unreadable warning and continues siblings") {
    TmpTree t;
    t.write("readable/ok.yass.yaml", "x");
    stdfs::path locked = t.mkdir("locked");
    t.write("locked/hidden.yass.yaml", "x");
    // Remove read/execute perms so the directory cannot be listed.
    stdfs::permissions(locked, stdfs::perms::none, stdfs::perm_options::replace);
    auto r = discover_spec_files(t.root().generic_string(), cwd_of(t));
    // Restore perms so the RAII cleanup can remove the tree.
    stdfs::permissions(locked, stdfs::perms::owner_all, stdfs::perm_options::replace);

    REQUIRE(r.ok());  // non-fatal: discovery still succeeds
    // The readable sibling is still discovered.
    bool found_ok = std::find(r.files.begin(), r.files.end(), "readable/ok.yass.yaml") !=
                    r.files.end();
    CHECK(found_ok);
    // Exactly one dir_unreadable warning for the locked directory.
    bool warned = false;
    for (const auto& w : r.warnings) {
        if (w.code == ErrorCode::discover_dir_unreadable) {
            warned = true;
        }
    }
    CHECK(warned);
}

// ===========================================================================
// cli.DiscoverSpecFiles — path-form RETURN obligations (cli.ErrorLine reuse)
// ===========================================================================

// RETURN MUST: emit basename alone when directly inside cwd; relative without
// leading "./" when nested under cwd.
TEST_CASE("DiscoverSpecFiles RETURN: path forms (basename in cwd, relative without ./ when nested)") {
    TmpTree t;
    t.write("flat.yass.yaml", "x");
    t.write("nested/deep.yass.yaml", "x");
    auto r = discover_spec_files(t.root().generic_string(), cwd_of(t));
    REQUIRE(r.ok());
    // flat is directly in cwd -> basename; nested -> relative WITHOUT ./.
    std::vector<std::string> expect = {"flat.yass.yaml", "nested/deep.yass.yaml"};
    CHECK(r.files == expect);
    for (const auto& f : r.files) {
        CHECK(f.rfind("./", 0) != 0);  // no leading ./
    }
}

// RETURN WHEN a discovered file is not under the cwd MUST: return it as an
// absolute path. Discover from the tree while cwd is a SIBLING outside the tree.
TEST_CASE("DiscoverSpecFiles RETURN: files not under cwd are returned absolute") {
    TmpTree t;       // the search tree
    TmpTree cwddir;  // an unrelated cwd, not an ancestor of t
    t.write("outside.yass.yaml", "x");
    auto r = discover_spec_files(t.root().generic_string(), cwd_of(cwddir));
    REQUIRE(r.ok());
    REQUIRE(r.files.size() == 1);
    CHECK(stdfs::path(r.files[0]).is_absolute());
    CHECK(r.files[0] == (t.root() / "outside.yass.yaml").generic_string());
}

// ===========================================================================
// cli.ExpandGlob
// ===========================================================================

// RETURN WHEN no glob metacharacters MUST: return the single literal path
// unchanged (even if it does not exist on disk).
TEST_CASE("ExpandGlob RETURN: a literal path with no metacharacters is returned unchanged") {
    TmpTree t;
    auto r = expand_glob("some/plain/path.yass.yaml", cwd_of(t));
    REQUIRE(r.ok());
    REQUIRE(r.matches.size() == 1);
    CHECK(r.matches[0] == "some/plain/path.yass.yaml");
}

// has_glob_metacharacters helper: only *, ?, [ are metacharacters.
TEST_CASE("ExpandGlob INPUT: metacharacter detection (*, ?, [ only)") {
    CHECK(has_glob_metacharacters("a*"));
    CHECK(has_glob_metacharacters("a?"));
    CHECK(has_glob_metacharacters("a[bc]"));
    CHECK_FALSE(has_glob_metacharacters("plain.yass.yaml"));
    CHECK_FALSE(has_glob_metacharacters("a/b/c"));
    CHECK_FALSE(has_glob_metacharacters("{a,b}"));  // braces are NOT metacharacters
}

// RETURN WHEN metacharacters MUST: '*' matches any run of non-'/' characters
// (does NOT cross directory separators).
TEST_CASE("ExpandGlob RETURN: '*' matches within a single path segment and not across '/'") {
    TmpTree t;
    t.write("a.yass.yaml", "x");
    t.write("b.yass.yaml", "x");
    t.write("sub/c.yass.yaml", "x");  // in a subdir -> NOT matched by top '*'
    auto r = expand_glob("*.yass.yaml", cwd_of(t));
    REQUIRE(r.ok());
    std::vector<std::string> expect = {"a.yass.yaml", "b.yass.yaml"};
    CHECK(r.matches == expect);
}

// RETURN: '?' matches exactly one non-'/' character.
TEST_CASE("ExpandGlob RETURN: '?' matches exactly one character") {
    TmpTree t;
    t.write("a.txt", "x");
    t.write("ab.txt", "x");  // two chars before .txt -> not matched by '?.txt'
    auto r = expand_glob("?.txt", cwd_of(t));
    REQUIRE(r.ok());
    std::vector<std::string> expect = {"a.txt"};
    CHECK(r.matches == expect);
}

// RETURN: '[...]' is a POSIX bracket expression (set + range + negation).
TEST_CASE("ExpandGlob RETURN: '[...]' bracket expression matches a set, a range, and negation") {
    TmpTree t;
    t.write("a.txt", "x");
    t.write("b.txt", "x");
    t.write("c.txt", "x");
    t.write("9.txt", "x");
    // Set: [ab]
    {
        auto r = expand_glob("[ab].txt", cwd_of(t));
        REQUIRE(r.ok());
        std::vector<std::string> expect = {"a.txt", "b.txt"};
        CHECK(r.matches == expect);
    }
    // Range: [a-c]
    {
        auto r = expand_glob("[a-c].txt", cwd_of(t));
        REQUIRE(r.ok());
        std::vector<std::string> expect = {"a.txt", "b.txt", "c.txt"};
        CHECK(r.matches == expect);
    }
    // Negation: [!9] (anything but '9')
    {
        auto r = expand_glob("[!9].txt", cwd_of(t));
        REQUIRE(r.ok());
        std::vector<std::string> expect = {"a.txt", "b.txt", "c.txt"};
        CHECK(r.matches == expect);
    }
}

// RETURN: '**' matches zero or more path segments and MUST be its own segment.
TEST_CASE("ExpandGlob RETURN: '**' matches zero or more path segments as its own segment") {
    TmpTree t;
    t.write("top.yass.yaml", "x");        // zero intermediate segments
    t.write("a/mid.yass.yaml", "x");      // one segment
    t.write("a/b/deep.yass.yaml", "x");   // two segments
    auto r = expand_glob("**/*.yass.yaml", cwd_of(t));
    REQUIRE(r.ok());
    std::vector<std::string> expect = {"a/b/deep.yass.yaml", "a/mid.yass.yaml", "top.yass.yaml"};
    CHECK(r.matches == expect);
}

// glob_match helper directly: segment-level semantics including ** zero-match and
// '*' not crossing '/'.
TEST_CASE("ExpandGlob RETURN: glob_match segment semantics (helper)") {
    CHECK(glob_match("*.yaml", "a.yaml"));
    CHECK_FALSE(glob_match("*.yaml", "sub/a.yaml"));    // '*' does not cross '/'
    CHECK(glob_match("**/*.yaml", "a.yaml"));           // ** matches zero segments
    CHECK(glob_match("**/*.yaml", "x/y/a.yaml"));       // ** matches two segments
    CHECK(glob_match("a/**/b.txt", "a/b.txt"));         // ** zero between a and b
    CHECK(glob_match("a/**/b.txt", "a/x/y/b.txt"));     // ** multiple between
    CHECK(glob_match("?", "a"));
    CHECK_FALSE(glob_match("?", "ab"));
    CHECK(glob_match("[a-c]z", "bz"));
    CHECK_FALSE(glob_match("[a-c]z", "dz"));
}

// INVARIANT MUST: match case-sensitively on all platforms.
//
// NOTE: the default macOS tmp volume is case-INSENSITIVE, so two filenames that
// differ only by case cannot coexist. We therefore prove the matcher's
// case-sensitivity against a SINGLE real file: an uppercase pattern segment must
// NOT match a lowercase filename (and the lowercase pattern must match it). This
// exercises the byte-exact comparison without relying on the volume's casing.
TEST_CASE("ExpandGlob INVARIANT: matching is case-sensitive") {
    TmpTree t;
    t.write("lower.txt", "x");
    // Lowercase pattern matches the lowercase file.
    {
        auto r = expand_glob("lower.*", cwd_of(t));
        REQUIRE(r.ok());
        std::vector<std::string> expect = {"lower.txt"};
        CHECK(r.matches == expect);
    }
    // Uppercase pattern does NOT match the lowercase file -> zero matches.
    {
        auto r = expand_glob("LOWER.*", cwd_of(t));
        REQUIRE_FALSE(r.ok());
        CHECK(has_value_code(r.error, ErrorCode::glob_no_match));
    }
}

// INVARIANT MUST-NOT: match hidden files or descend into hidden directories.
TEST_CASE("ExpandGlob INVARIANT: hidden files are not matched and hidden directories are not descended") {
    TmpTree t;
    t.write("visible.yass.yaml", "x");
    t.write(".hidden.yass.yaml", "x");         // hidden file -> not matched
    t.write(".hiddendir/inside.yass.yaml", "x");  // hidden dir -> not descended
    {
        auto r = expand_glob("*.yass.yaml", cwd_of(t));
        REQUIRE(r.ok());
        std::vector<std::string> expect = {"visible.yass.yaml"};
        CHECK(r.matches == expect);
    }
    {
        // ** must not descend hidden dirs either.
        auto r = expand_glob("**/*.yass.yaml", cwd_of(t));
        REQUIRE(r.ok());
        std::vector<std::string> expect = {"visible.yass.yaml"};
        CHECK(r.matches == expect);
    }
}

// INVARIANT MUST-NOT: follow symbolic links.
TEST_CASE("ExpandGlob INVARIANT: symbolic links are not followed") {
    TmpTree t;
    t.write("real.txt", "x");
    t.symlink("link.txt", t.root() / "real.txt");
    auto r = expand_glob("*.txt", cwd_of(t));
    REQUIRE(r.ok());
    std::vector<std::string> expect = {"real.txt"};  // the symlink is skipped
    CHECK(r.matches == expect);
}

// INVARIANT MUST-NOT: perform brace expansion. A literal "{a,b}" with no glob
// metacharacters is returned UNCHANGED (a literal passthrough, not expanded).
TEST_CASE("ExpandGlob INVARIANT: no brace expansion (literal passthrough)") {
    TmpTree t;
    t.write("a.txt", "x");
    t.write("b.txt", "x");
    auto r = expand_glob("{a,b}.txt", cwd_of(t));
    REQUIRE(r.ok());
    REQUIRE(r.matches.size() == 1);
    CHECK(r.matches[0] == "{a,b}.txt");  // returned literally, NOT {a,b} expanded
}

// RETURN MUST: results sorted by NFC code-point order.
TEST_CASE("ExpandGlob RETURN: matches sorted by code-point order") {
    TmpTree t;
    t.write("Banana.txt", "x");
    t.write("apple.txt", "x");
    t.write("Cherry.txt", "x");
    auto r = expand_glob("*.txt", cwd_of(t));
    REQUIRE(r.ok());
    // Code-point: 'B'(0x42) < 'C'(0x43) < 'a'(0x61).
    std::vector<std::string> expect = {"Banana.txt", "Cherry.txt", "apple.txt"};
    CHECK(r.matches == expect);
}

// ERROR WHEN a glob matches zero files MUST: yass.glob.no_match and exit 2.
TEST_CASE("ExpandGlob ERROR: yass.glob.no_match (exit 2) when the pattern matches zero files") {
    TmpTree t;
    t.write("a.txt", "x");
    auto r = expand_glob("nope-*.zzz", cwd_of(t));
    REQUIRE_FALSE(r.ok());
    CHECK(has_value_code(r.error, ErrorCode::glob_no_match));
    CHECK(yass::diag::exit_for(r.error->code) == 2);
    CHECK(r.error->file.empty());  // no associated input file -> "yass"
}
