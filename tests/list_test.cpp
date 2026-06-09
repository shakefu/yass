// M8 — the `list` subcommand tests.
//
// Spec basis (spec/cli.list.yass.yaml :: cli.list INPUT / RETURN / ERROR /
// SIDE-EFFECT / INVARIANT), plus spec/cli.yass.yaml :: cli.ExitCode /
// cli.ErrorLine and spec/cli.shared.yass.yaml (discover / findroot).
//
// Conformance policy (tech lead): stdout bytes AND the process exit code MUST
// match the reference `yass list ...` byte-for-byte (differential).
//
//   1. NON-TTY (tty_width = 0): run_list(...) stdout+exit == `yass list ...`
//      stdout+exit, byte-for-byte, for several invocations (a dir, the default
//      root discovery, a single file, the self-defining yass.yass.yaml file).
//      The reference's stdout under a pipe is its non-TTY branch, so the
//      proc-based diff harness compares like-for-like.
//   2. Error cases: not_found / bad_extension / colon-in-path (exit 2), and a
//      directory containing a deliberately malformed .yass.yaml among valid ones
//      (exit 1, the malformed ErrorLine on stderr, the valid rows still listed).
//   3. TTY truncation: run the reference under a pty (tests/support/pty.hpp) with
//      COLUMNS=W, and assert run_list(args, ..., W) stdout == the reference's
//      pty stdout for several widths and for the empty-description / meets-or-
//      exceeds-width edge cases. When no pty can be allocated, the case SKIPS
//      gracefully via a doctest MESSAGE.
//
// `-` is NOT differential-tested: the dispatcher intercepts the stdin marker
// before list runs, so `yass list -` reports yass.argv.stdin_dash, whereas
// run_list (per cli.list INPUT) treats `-` as a literal path. We assert the
// literal-path handling directly instead.

#include "doctest.h"

#include <filesystem>
#include <sstream>
#include <string>
#include <vector>

#include "diag.hpp"
#include "list.hpp"
#include "support/diff.hpp"
#include "support/pty.hpp"
#include "support/tmptree.hpp"

namespace {

namespace fs = std::filesystem;
using yass::test::CwdGuard;
using yass::test::DiffOutcome;
using yass::test::expect_stdout_exit_match;
using yass::test::find_ref_bin;
using yass::test::PtyResult;
using yass::test::run_under_pty;
using yass::test::TmpTree;

// The repo root: tests run from build/, the source root is two levels up via the
// CMake-provided test binary, but the differential cases chdir explicitly. We
// resolve the repo root from this source file's known location at compile time
// is brittle; instead the non-TTY cases chdir into the repo root discovered by
// walking up from the current path until a `.git` dir is found.
fs::path repo_root() {
    fs::path p = fs::current_path();
    for (int i = 0; i < 12; ++i) {
        std::error_code ec;
        if (fs::exists(p / ".git", ec)) return p;
        if (p.parent_path() == p) break;
        p = p.parent_path();
    }
    return fs::current_path();
}

// Build the in-process runner wrapping run_list(args, out, err, tty_width).
yass::test::InProcRunner runner_for(const std::vector<std::string>& args, int tty_width) {
    return [args, tty_width](std::ostream& out, std::ostream& err) {
        return yass::list::run_list(args, out, err, tty_width);
    };
}

// The reference args for `list` = {"list"} + args.
std::vector<std::string> ref_args(const std::vector<std::string>& args) {
    std::vector<std::string> v;
    v.push_back("list");
    v.insert(v.end(), args.begin(), args.end());
    return v;
}

struct Captured {
    int exit = 0;
    std::string out;
    std::string err;
};
Captured run_local(const std::vector<std::string>& args, int tty_width) {
    std::ostringstream out, err;
    Captured c;
    c.exit = yass::list::run_list(args, out, err, tty_width);
    c.out = out.str();
    c.err = err.str();
    return c;
}

// A minimal valid Preamble + one spec, with a chosen description and spec name.
std::string spec_file(const std::string& description, const std::string& name) {
    std::string desc_field = description.empty() ? std::string("\"\"") : ("\"" + description + "\"");
    return "---\ndescription: " + desc_field + "\nversion: v1\n---\nspec: " + name +
           "\nINPUT:\n- MUST: x\n";
}

}  // namespace

// ===========================================================================
// 1. NON-TTY differential (tty_width == 0): byte-for-byte stdout + exit.
// ===========================================================================
TEST_CASE("cli.list NON-TTY stdout+exit match the reference (RETURN)") {
    CwdGuard guard(repo_root());

    SUBCASE("list a directory: `list spec`") {
        DiffOutcome o = expect_stdout_exit_match(ref_args({"spec"}), runner_for({"spec"}, 0));
        if (o.ref_available) {
            // Sanity: real rows were produced and tab-separated into three fields.
            CHECK(o.our_out.find('\t') != std::string::npos);
        }
    }

    SUBCASE("default discovery from the project root: `list`") {
        expect_stdout_exit_match(ref_args({}), runner_for({}, 0));
    }

    SUBCASE("a single spec file: `list spec/cli.list.yass.yaml`") {
        expect_stdout_exit_match(ref_args({"spec/cli.list.yass.yaml"}),
                                 runner_for({"spec/cli.list.yass.yaml"}, 0));
    }

    SUBCASE("the self-defining file: `list yass.yass.yaml`") {
        // Exercises a file whose FIRST document is itself a Spec (spec: Document),
        // so every document contributes a row sharing the file's description.
        expect_stdout_exit_match(ref_args({"yass.yass.yaml"}),
                                 runner_for({"yass.yass.yaml"}, 0));
    }
}

// ===========================================================================
// 2. Error cases (exit 2) and the malformed-in-a-dir case (exit 1).
// ===========================================================================
TEST_CASE("cli.list not_found / bad_extension / colon are exit 2 (ERROR / INPUT)") {
    TmpTree tree;
    CwdGuard guard(tree.root());

    SUBCASE("not_found: a path that does not exist -> exit 2") {
        DiffOutcome o = expect_stdout_exit_match(ref_args({"nope.yass.yaml"}),
                                                 runner_for({"nope.yass.yaml"}, 0));
        // (ERROR) yass.path.not_found, exit 2, no stdout rows.
        CHECK(run_local({"nope.yass.yaml"}, 0).exit == 2);
        CHECK(run_local({"nope.yass.yaml"}, 0).out.empty());
        Captured c = run_local({"nope.yass.yaml"}, 0);
        CHECK(c.err.find("[yass.path.not_found]") != std::string::npos);
        (void)o;
    }

    SUBCASE("bad_extension: an existing non-spec file -> exit 2") {
        tree.write("plain.txt", "hello\n");
        DiffOutcome o = expect_stdout_exit_match(ref_args({"plain.txt"}),
                                                 runner_for({"plain.txt"}, 0));
        Captured c = run_local({"plain.txt"}, 0);
        CHECK(c.exit == 2);
        CHECK(c.out.empty());
        CHECK(c.err.find("[yass.path.bad_extension]") != std::string::npos);
        (void)o;
    }

    SUBCASE("colon_in_path: any positional containing ':' -> exit 2") {
        DiffOutcome o = expect_stdout_exit_match(ref_args({"foo:bar"}),
                                                 runner_for({"foo:bar"}, 0));
        Captured c = run_local({"foo:bar"}, 0);
        CHECK(c.exit == 2);
        CHECK(c.out.empty());
        CHECK(c.err.find("[yass.path.colon_in_path]") != std::string::npos);
        (void)o;
    }
}

TEST_CASE("cli.list a malformed file among valid ones: exit 1, valid rows kept (ERROR)") {
    TmpTree tree;
    tree.write("good.yass.yaml", spec_file("Good one", "good.spec"));
    tree.write("zzz.yass.yaml", spec_file("Last", "zzz.spec"));
    // A deliberately malformed YAML file (an unterminated flow sequence).
    tree.write("bad.yass.yaml",
               "---\ndescription: Bad\nversion: v1\n---\nspec: bad\nINPUT:\n- MUST: [unclosed\n");

    CwdGuard guard(tree.root());

    DiffOutcome o = expect_stdout_exit_match(ref_args({"."}), runner_for({"."}, 0));

    Captured c = run_local({"."}, 0);
    // (ERROR) exit 1 after listing all parseable specs.
    CHECK(c.exit == 1);
    // The valid rows are still emitted, in file code-point order (good before zzz).
    CHECK(c.out.find("good.yass.yaml\tgood.spec\tGood one\n") != std::string::npos);
    CHECK(c.out.find("zzz.yass.yaml\tzzz.spec\tLast\n") != std::string::npos);
    // The malformed file is NOT listed on stdout.
    CHECK(c.out.find("bad.yass.yaml") == std::string::npos);
    // One ErrorLine with the malformed code on stderr.
    CHECK(c.err.find("[yass.yaml.malformed]") != std::string::npos);
    CHECK(c.err.find("bad.yass.yaml") != std::string::npos);

    if (o.ref_available) {
        // Differential: our stdout+exit already CHECKed equal to the reference by
        // expect_stdout_exit_match; the reference also exits 1 here.
        CHECK(o.ref_exit == 1);
    }
}

// ===========================================================================
// Direct in-process RETURN behaviors (no reference needed).
// ===========================================================================
TEST_CASE("cli.list empty-but-valid scope -> exit 0, no output (RETURN)") {
    TmpTree tree;
    tree.mkdir("empty");
    CwdGuard guard(tree.root());
    Captured c = run_local({"empty"}, 0);
    CHECK(c.exit == 0);
    CHECK(c.out.empty());
    CHECK(c.err.empty());
}

TEST_CASE("cli.list a file with zero Spec documents -> no rows, no error (RETURN)") {
    TmpTree tree;
    // Preamble only, no Spec document.
    tree.write("only.yass.yaml", "---\ndescription: just a preamble\nversion: v1\n");
    CwdGuard guard(tree.root());
    Captured c = run_local({"only.yass.yaml"}, 0);
    CHECK(c.exit == 0);
    CHECK(c.out.empty());
    CHECK(c.err.empty());
}

TEST_CASE("cli.list missing/empty/non-string description -> empty third field, both tabs (RETURN)") {
    TmpTree tree;
    // No description key.
    tree.write("a.yass.yaml", "---\nversion: v1\n---\nspec: a.spec\nINPUT:\n- MUST: x\n");
    // Empty string description.
    tree.write("b.yass.yaml", spec_file("", "b.spec"));
    // Non-string (sequence) description.
    tree.write("c.yass.yaml",
               "---\ndescription:\n  - x\n  - y\nversion: v1\n---\nspec: c.spec\nINPUT:\n- MUST: x\n");
    CwdGuard guard(tree.root());

    CHECK(run_local({"a.yass.yaml"}, 0).out == "a.yass.yaml\ta.spec\t\n");
    CHECK(run_local({"b.yass.yaml"}, 0).out == "b.yass.yaml\tb.spec\t\n");
    CHECK(run_local({"c.yass.yaml"}, 0).out == "c.yass.yaml\tc.spec\t\n");
}

TEST_CASE("cli.list description whitespace is collapsed and trimmed (RETURN)") {
    TmpTree tree;
    // A folded/literal block with internal multi-space and newlines.
    tree.write("w.yass.yaml",
               "---\ndescription: |\n  Hello   world\n  next line\nversion: v1\n---\nspec: "
               "w.spec\nINPUT:\n- MUST: x\n");
    CwdGuard guard(tree.root());
    CHECK(run_local({"w.yass.yaml"}, 0).out == "w.yass.yaml\tw.spec\tHello world next line\n");
}

TEST_CASE("cli.list document order is preserved within a file (RETURN)") {
    TmpTree tree;
    tree.write("multi.yass.yaml",
               "---\ndescription: D\nversion: v1\n---\nspec: first\nINPUT:\n- MUST: x\n---\nspec: "
               "second\nINPUT:\n- MUST: y\n");
    CwdGuard guard(tree.root());
    Captured c = run_local({"multi.yass.yaml"}, 0);
    CHECK(c.out == "multi.yass.yaml\tfirst\tD\nmulti.yass.yaml\tsecond\tD\n");
}

// ===========================================================================
// 3. TTY truncation differential (run the reference under a pty).
// ===========================================================================
namespace {

// Run the reference `yass list <dir>` under a pty of `width` columns (COLUMNS=W),
// in working directory `cwd`. Returns the pty result (ok=false => skip).
PtyResult ref_list_under_pty(const std::string& ref_bin, const std::string& cwd,
                             const std::vector<std::string>& list_args, int width) {
    std::vector<std::string> argv;
    argv.push_back(ref_bin);
    argv.push_back("list");
    argv.insert(argv.end(), list_args.begin(), list_args.end());
    std::vector<std::pair<std::string, std::string>> env = {{"COLUMNS", std::to_string(width)}};
    return run_under_pty(argv, cwd, width, /*rows=*/24, env);
}

}  // namespace

TEST_CASE("cli.list TTY truncation matches the reference under a pty (RETURN)") {
    std::string ref = find_ref_bin();
    if (ref.empty()) {
        MESSAGE("reference yass not available; skipping pty truncation differential");
        return;
    }

    // Build a controlled tree: one file with a known long description so the
    // arithmetic (prefix = file-graphemes + 2 tabs + name + desc) is predictable.
    TmpTree tree;
    tree.write("a.yass.yaml", spec_file("ABCDEFGHIJ", "x.y"));
    const std::string cwd = tree.root().string();

    // Several widths spanning: far below the prefix, mid-truncation, exact fit,
    // and far above (no truncation).
    for (int width : {10, 16, 19, 20, 21, 25, 26, 27, 200}) {
        PtyResult ref_r = ref_list_under_pty(ref, cwd, {"a.yass.yaml"}, width);
        if (!ref_r.ok) {
            MESSAGE("pty could not be allocated; skipping width " << width);
            continue;
        }
        CwdGuard guard(tree.root());
        Captured local = run_local({"a.yass.yaml"}, width);
        INFO("width=" << width);
        CHECK(local.out == ref_r.out);
        CHECK(local.exit == ref_r.exit_code);
    }
}

TEST_CASE("cli.list TTY non-ascii description truncates on a code-point boundary (RETURN)") {
    std::string ref = find_ref_bin();
    if (ref.empty()) {
        MESSAGE("reference yass not available; skipping pty non-ascii truncation differential");
        return;
    }
    TmpTree tree;
    // "café ☕ test": multi-byte codepoints (é, ☕) must never be split mid-byte.
    tree.write("e.yass.yaml", spec_file("caf\xC3\xA9 \xE2\x98\x95 test", "e"));
    const std::string cwd = tree.root().string();

    for (int width : {14, 18, 19, 20, 30, 100}) {
        PtyResult ref_r = ref_list_under_pty(ref, cwd, {"e.yass.yaml"}, width);
        if (!ref_r.ok) {
            MESSAGE("pty could not be allocated; skipping width " << width);
            continue;
        }
        CwdGuard guard(tree.root());
        Captured local = run_local({"e.yass.yaml"}, width);
        INFO("width=" << width);
        CHECK(local.out == ref_r.out);
        CHECK(local.exit == ref_r.exit_code);
    }
}

TEST_CASE("cli.list TTY empty description never appends a marker (RETURN)") {
    std::string ref = find_ref_bin();
    if (ref.empty()) {
        MESSAGE("reference yass not available; skipping pty empty-description differential");
        return;
    }
    TmpTree tree;
    tree.write("b.yass.yaml", spec_file("", "ee"));
    const std::string cwd = tree.root().string();

    for (int width : {5, 15, 16, 100}) {
        PtyResult ref_r = ref_list_under_pty(ref, cwd, {"b.yass.yaml"}, width);
        if (!ref_r.ok) {
            MESSAGE("pty could not be allocated; skipping width " << width);
            continue;
        }
        CwdGuard guard(tree.root());
        Captured local = run_local({"b.yass.yaml"}, width);
        INFO("width=" << width);
        CHECK(local.out == ref_r.out);
        // The third field is always empty, no "..." marker.
        CHECK(local.out.find("...") == std::string::npos);
        CHECK(local.exit == ref_r.exit_code);
    }
}

TEST_CASE("cli.list TTY meets-or-exceeds-width yields an empty third field, no marker (RETURN)") {
    std::string ref = find_ref_bin();
    if (ref.empty()) {
        MESSAGE("reference yass not available; skipping pty meets-or-exceeds differential");
        return;
    }
    TmpTree tree;
    tree.write("a.yass.yaml", spec_file("ABCDEFGHIJ", "x.y"));
    const std::string cwd = tree.root().string();

    // For a.yass.yaml (path 11 + 2 tabs + name 3 = prefix 16): prefix + 3-char
    // marker = 19, so widths <= 19 leave no room and must emit an empty third
    // field with no marker.
    for (int width : {17, 18, 19}) {
        PtyResult ref_r = ref_list_under_pty(ref, cwd, {"a.yass.yaml"}, width);
        if (!ref_r.ok) {
            MESSAGE("pty could not be allocated; skipping width " << width);
            continue;
        }
        CwdGuard guard(tree.root());
        Captured local = run_local({"a.yass.yaml"}, width);
        INFO("width=" << width);
        CHECK(local.out == "a.yass.yaml\tx.y\t\n");
        CHECK(local.out == ref_r.out);
        CHECK(local.exit == ref_r.exit_code);
    }
}

// ===========================================================================
// cli.DiscoverSpecFiles ERROR (CONFORMS cli.ErrorLine) — the discover warning
// (yass.discover.dir_unreadable) MUST be emitted by the `list` subcommand for an
// unreadable subdirectory, continuing with the readable siblings (exit 0). This
// was silently dropped before (no caller surfaced dr.warnings).
// ===========================================================================
TEST_CASE("cli.list ERROR: an unreadable subdir emits dir_unreadable and still lists siblings") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("top.yass.yaml", spec_file("desc", "top.Thing"));
    fs::path locked = tree.mkdir("locked");
    tree.write("locked/hidden.yass.yaml", spec_file("d", "locked.Thing"));
    fs::permissions(locked, fs::perms::none, fs::perm_options::replace);

    CwdGuard guard(tree.root());
    // Non-TTY (width 0): stdout+exit match the reference; stderr carries the
    // discover warning (CONFORMS cli.ErrorLine) with the directory as <file>.
    DiffOutcome o = expect_stdout_exit_match(ref_args({}), runner_for({}, 0));
    Captured c = run_local({}, 0);

    fs::permissions(locked, fs::perms::owner_all, fs::perm_options::replace);

    // The readable sibling is still listed; exit 0 (the warning is non-fatal).
    CHECK(c.out.find("top.Thing") != std::string::npos);
    CHECK(c.exit == 0);
    // The warning is emitted on stderr, attributed to the locked directory.
    CHECK(c.err.find("[yass.discover.dir_unreadable]") != std::string::npos);
    CHECK(c.err.find("locked") != std::string::npos);
    // stderr matches the reference byte-for-byte too (warning text + <file>).
    if (o.ref_available) {
        CHECK(c.out == o.ref_out);
        CHECK(c.err == o.ref_err);
        CHECK(c.exit == o.ref_exit);
    }
}

// ===========================================================================
// cli.FindProjectRoot ERROR (cli.ErrorLine) — when no project root marker is
// found, `list` MUST render the no_marker ErrorLine with the absolute cwd as the
// <file> prefix (NOT the bare "yass" token). Before the fix list surfaced the
// fs diagnostic verbatim (empty <file> -> "yass").
// ===========================================================================
TEST_CASE("cli.list ERROR: findroot.no_marker renders the absolute-cwd <file> prefix") {
    // A directory tree with NO .git and NO *.yass.yaml anywhere upward. TMPDIR on
    // a developer machine is normally clean of markers; if a marker happens to
    // exist above the temp root, the reference would not error either, so we
    // assert against the reference rather than a hardcoded expectation.
    TmpTree tree;
    fs::path sub = tree.mkdir("sub");

    CwdGuard guard(sub);
    DiffOutcome o = expect_stdout_exit_match(ref_args({}), runner_for({}, 0));
    Captured c = run_local({}, 0);

    if (o.ref_available && c.err.find("[yass.findroot.no_marker]") != std::string::npos) {
        // The error line must NOT begin with the bare "yass" token.
        CHECK(c.err.rfind("yass:", 0) != 0);
        // It begins with an absolute path (the cwd) followed by ": [".
        CHECK(c.err.front() == '/');
        // And matches the reference byte-for-byte (modulo the macOS /private
        // realpath prefix, which the reference shares with validate).
        CHECK(c.err == o.ref_err);
        CHECK(c.exit == o.ref_exit);
    } else {
        MESSAGE("temp root has a project marker above it (or no reference); "
                "skipping list no_marker prefix assertion");
    }
}
