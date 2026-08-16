// M6 — the `validate` subcommand tests.
//
// Spec basis (spec/cli.validate.yass.yaml :: cli.validate INPUT / RETURN /
// ERROR / SIDE-EFFECT / INVARIANT), plus spec/cli.yass.yaml :: cli.ExitCode /
// cli.ErrorLine and spec/cli.shared.yass.yaml (glob / discover / findroot).
//
// Conformance policy (tech lead): stdout bytes AND the process exit code MUST
// match the reference `yass validate ...` byte-for-byte (differential, via
// tests/support/diff.hpp). stderr follows the SPEC: the cli.errors message PROSE
// (diag::canonical_message), the cli.ErrorLine format, and the spec ordering,
// with error CODE / LINE / EXIT still matching the reference. So each case below
// asserts stdout+exit against the reference and asserts stderr structure
// (file/line/[code]) against the spec, cross-checking the [code] against the
// reference where the reference's prose agrees.
//
// Known spec/reference divergence (documented, NOT differential-tested for
// stdout): the bare `-` token. The reference treats `-` as a stdin marker
// (yass.argv.stdin_dash, no summary line) whereas cli.validate INPUT mandates
// treating `-` as a LITERAL path. We follow the SPEC: `-` is a literal path ->
// yass.path.not_found, exit 2, with the summary line. Exit (2) still matches.

#include "doctest.h"

#include <filesystem>
#include <fstream>
#include <iterator>
#include <sstream>
#include <string>
#include <vector>

#include "diag.hpp"
#include "validate.hpp"
#include "support/diff.hpp"
#include "support/tmptree.hpp"

namespace {

namespace fs = std::filesystem;
using yass::test::CwdGuard;
using yass::test::expect_stdout_exit_match;
using yass::test::TmpTree;

// A minimal valid Preamble (description + version v1).
constexpr const char* kPre = "---\ndescription: a test\nversion: v1\n---\n";

// Build the in-process runner that wraps run_validate(args, out, err).
yass::test::InProcRunner runner_for(const std::vector<std::string>& args) {
    return [args](std::ostream& out, std::ostream& err) {
        return yass::validate::run_validate(args, out, err);
    };
}

// The reference args for `validate` = {"validate"} + args.
std::vector<std::string> ref_args(const std::vector<std::string>& args) {
    std::vector<std::string> v;
    v.push_back("validate");
    v.insert(v.end(), args.begin(), args.end());
    return v;
}

// Run our run_validate(args) in-process under the current cwd; capture results.
struct Captured {
    int exit = 0;
    std::string out;
    std::string err;
};
Captured run_local(const std::vector<std::string>& args) {
    std::ostringstream out, err;
    Captured c;
    c.exit = yass::validate::run_validate(args, out, err);
    c.out = out.str();
    c.err = err.str();
    return c;
}

// True iff `err` contains the cli.ErrorLine token `[<code>]`.
bool has_code(const std::string& err, const char* code) {
    return err.find(std::string("[") + code + "]") != std::string::npos;
}

}  // namespace

// ===========================================================================
// SIDE-EFFECT / RETURN — the summary line + a clean directory (0 errors).
// ===========================================================================
TEST_CASE("validate: clean directory discovers and reports 0 errors") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("a.yass.yaml", std::string(kPre) + "spec: alpha\nINPUT:\n- MUST: do a thing\n");
    tree.write("b.yass.yaml", std::string(kPre) + "spec: beta\n");
    CwdGuard cd(tree.root());

    // No args -> discover from project root.
    auto o = expect_stdout_exit_match(ref_args({}), runner_for({}));
    auto c = run_local({});
    // (SIDE-EFFECT) summary line, last bytes; (RETURN) exit 0; no errors emitted.
    CHECK(c.out == "checked 2 files, found 0 errors\n");
    CHECK(c.exit == 0);
    CHECK(c.err.empty());
    if (o.ref_available) CHECK(o.ref_err.empty());
}

TEST_CASE("validate: directory argument is discovered recursively") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("specs/a.yass.yaml", std::string(kPre) + "spec: alpha\n");
    tree.write("specs/nested/b.yass.yaml", std::string(kPre) + "spec: beta\n");
    CwdGuard cd(tree.root());

    expect_stdout_exit_match(ref_args({"specs"}), runner_for({"specs"}));
    auto c = run_local({"specs"});
    CHECK(c.out == "checked 2 files, found 0 errors\n");
    CHECK(c.exit == 0);
}

// ===========================================================================
// RETURN — single file argument; mixed valid/invalid; error ordering.
// ===========================================================================
TEST_CASE("validate: single valid file argument") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("only.yass.yaml", std::string(kPre) + "spec: only.one\nINPUT:\n- MUST: x\n");
    CwdGuard cd(tree.root());

    expect_stdout_exit_match(ref_args({"only.yass.yaml"}), runner_for({"only.yass.yaml"}));
    auto c = run_local({"only.yass.yaml"});
    CHECK(c.out == "checked 1 files, found 0 errors\n");
    CHECK(c.exit == 0);
}

TEST_CASE("validate: single invalid file -> one error, exit 1, summary last") {
    TmpTree tree;
    tree.mkdir(".git");
    // A bad spec name (contains a space) -> yass.spec.name_bad_chars at line 5.
    tree.write("bad.yass.yaml", std::string(kPre) + "spec: \"foo bar\"\nINPUT:\n- MUST: x\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"bad.yass.yaml"}), runner_for({"bad.yass.yaml"}));
    auto c = run_local({"bad.yass.yaml"});
    CHECK(c.out == "checked 1 files, found 1 errors\n");
    CHECK(c.exit == 1);
    // (SIDE-EFFECT) errors precede the summary on their respective streams; the
    // ErrorLine carries the file, line, and [code] per the spec.
    CHECK(c.err == "bad.yass.yaml:5: [yass.spec.name_bad_chars] "
                   "spec name contains disallowed characters: foo bar\n");
    if (o.ref_available) {
        // The reference's [code] for this case agrees with the spec.
        CHECK(has_code(o.ref_err, "yass.spec.name_bad_chars"));
    }
}

TEST_CASE("validate: mixed valid + invalid files; errors in (file,line) order, no interleave") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("good.yass.yaml", std::string(kPre) + "spec: good.one\nINPUT:\n- MUST: x\n");
    // Two errors in one file at different lines, from different checks:
    // unknown_key (line 6) then a same-file ref miss (line 9).
    tree.write("bad.yass.yaml",
               std::string(kPre) +
                   "spec: bad.one\nBADKEY:\n- MUST: x\nINPUT:\n- MUST: y\n  USES: no.such.spec\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({}), runner_for({}));
    auto c = run_local({});
    // good + bad discovered; 2 errors total; exit 1.
    CHECK(c.out == "checked 2 files, found 2 errors\n");
    CHECK(c.exit == 1);
    // (RETURN) within bad.yass.yaml, errors ascend by line: unknown_key at the
    // BADKEY line (6), then the same-file ref miss at the USES line (10).
    std::string expected =
        "bad.yass.yaml:6: [yass.spec.unknown_key] unknown spec key: BADKEY\n"
        "bad.yass.yaml:10: [yass.ref.spec_not_found_same_file] spec not found in file: no.such.spec\n";
    CHECK(c.err == expected);
    if (o.ref_available) {
        CHECK(has_code(o.ref_err, "yass.spec.unknown_key"));
        CHECK(has_code(o.ref_err, "yass.ref.spec_not_found_same_file"));
    }
}

// ===========================================================================
// ERROR — bad_extension / not_found / glob.no_match / no_files / colon.
// ===========================================================================
TEST_CASE("validate: file argument without .yass.yaml suffix -> bad_extension exit 2") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("notes.txt", "hello\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"notes.txt"}), runner_for({"notes.txt"}));
    auto c = run_local({"notes.txt"});
    // (SIDE-EFFECT) the summary is emitted even on a usage failure.
    CHECK(c.out == "checked 0 files, found 0 errors\n");
    CHECK(c.exit == 2);
    CHECK(c.err ==
          "notes.txt: [yass.path.bad_extension] expected a .yass.yaml file: notes.txt\n");
    if (o.ref_available) CHECK(has_code(o.ref_err, "yass.path.bad_extension"));
}

TEST_CASE("validate: missing path -> not_found exit 2") {
    TmpTree tree;
    tree.mkdir(".git");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"missing.yass.yaml"}),
                                      runner_for({"missing.yass.yaml"}));
    auto c = run_local({"missing.yass.yaml"});
    CHECK(c.out == "checked 0 files, found 0 errors\n");
    CHECK(c.exit == 2);
    CHECK(c.err ==
          "missing.yass.yaml: [yass.path.not_found] path does not exist: missing.yass.yaml\n");
    if (o.ref_available) CHECK(has_code(o.ref_err, "yass.path.not_found"));
}

TEST_CASE("validate: colon-in-path -> colon_in_path exit 2 (before findroot)") {
    TmpTree tree;
    tree.mkdir(".git");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"foo:bar.yass.yaml"}),
                                      runner_for({"foo:bar.yass.yaml"}));
    auto c = run_local({"foo:bar.yass.yaml"});
    CHECK(c.out == "checked 0 files, found 0 errors\n");
    CHECK(c.exit == 2);
    // (INPUT) colon rejection; ErrorLine file token is "yass" (no input file).
    CHECK(c.err ==
          "yass: [yass.path.colon_in_path] "
          "path contains an unsupported colon character: foo:bar.yass.yaml\n");
    if (o.ref_available) CHECK(has_code(o.ref_err, "yass.path.colon_in_path"));
}

TEST_CASE("validate: glob with no match -> glob.no_match exit 2") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("a.yass.yaml", std::string(kPre) + "spec: alpha\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"*.nope.yaml"}), runner_for({"*.nope.yaml"}));
    auto c = run_local({"*.nope.yaml"});
    CHECK(c.out == "checked 0 files, found 0 errors\n");
    CHECK(c.exit == 2);
    CHECK(c.err ==
          "*.nope.yaml: [yass.glob.no_match] no files matched pattern: *.nope.yaml\n");
    if (o.ref_available) CHECK(has_code(o.ref_err, "yass.glob.no_match"));
}

TEST_CASE("validate: project root but no spec files -> discover.no_files exit 2") {
    TmpTree tree;
    tree.mkdir(".git");  // marker, but no .yass.yaml anywhere.
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({}), runner_for({}));
    auto c = run_local({});
    CHECK(c.out == "checked 0 files, found 0 errors\n");
    CHECK(c.exit == 2);
    CHECK(c.err == "yass: [yass.discover.no_files] no .yass.yaml files found\n");
    if (o.ref_available) CHECK(has_code(o.ref_err, "yass.discover.no_files"));
}

TEST_CASE("validate: glob-expanded non-spec files are skipped silently") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("notes.txt", "hi\n");
    tree.write("a.yass.yaml", std::string(kPre) + "spec: alpha\n");
    CwdGuard cd(tree.root());

    // `*` matches both files; the non-spec one is skipped (ERROR: skip silently),
    // leaving exactly one validated file.
    auto o = expect_stdout_exit_match(ref_args({"*"}), runner_for({"*"}));
    auto c = run_local({"*"});
    CHECK(c.out == "checked 1 files, found 0 errors\n");
    CHECK(c.exit == 0);
    CHECK(c.err.empty());  // no bad_extension for the skipped non-spec match.
}

TEST_CASE("validate: glob matching only non-spec files -> no_files exit 2") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("notes.txt", "hi\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"*.txt"}), runner_for({"*.txt"}));
    auto c = run_local({"*.txt"});
    CHECK(c.out == "checked 0 files, found 0 errors\n");
    CHECK(c.exit == 2);
    CHECK(has_code(c.err, "yass.discover.no_files"));
}

// ===========================================================================
// INVARIANT — findroot failure: emit one ErrorLine to stderr, exit 2, BEFORE
// checking any file; MUST-NOT silently fall back to cwd.
// ===========================================================================
TEST_CASE("validate: no project root marker -> findroot.no_marker exit 2 before any file") {
    TmpTree tree;  // NO .git, NO *.yass.yaml -> no marker anywhere upward.
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({}), runner_for({}));
    auto c = run_local({});
    CHECK(c.out == "checked 0 files, found 0 errors\n");
    CHECK(c.exit == 2);
    CHECK(has_code(c.err, "yass.findroot.no_marker"));
    // (INVARIANT) `<cwd>:` is the absolute process cwd, per cli.validate. We use
    // current_path() (the process-cwd model). Cross-check the [code] only against
    // the reference, since the reference's <cwd> may differ under symlinked
    // tmpdirs ($PWD vs realpath).
    std::error_code ec;
    std::string cwd = fs::current_path(ec).generic_string();
    CHECK(c.err == cwd + ": [yass.findroot.no_marker] no project root marker found\n");
    if (o.ref_available) CHECK(has_code(o.ref_err, "yass.findroot.no_marker"));
}

// ===========================================================================
// INPUT — dedup by lexically-normalized absolute path; never process twice.
// ===========================================================================
TEST_CASE("validate: same file via two spellings is deduplicated to one") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("dup.yass.yaml", std::string(kPre) + "spec: dup.one\nINPUT:\n- MUST: x\n");
    CwdGuard cd(tree.root());

    // ./dup.yass.yaml and dup.yass.yaml normalize to the same absolute path.
    auto o = expect_stdout_exit_match(ref_args({"./dup.yass.yaml", "dup.yass.yaml"}),
                                      runner_for({"./dup.yass.yaml", "dup.yass.yaml"}));
    auto c = run_local({"./dup.yass.yaml", "dup.yass.yaml"});
    CHECK(c.out == "checked 1 files, found 0 errors\n");  // counted once, not twice.
    CHECK(c.exit == 0);
}

TEST_CASE("validate: explicit file then glob dedups, preserving first occurrence") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("alpha.yass.yaml", std::string(kPre) + "spec: alpha\n");
    tree.write("zeta.yass.yaml", std::string(kPre) + "spec: zeta\n");
    CwdGuard cd(tree.root());

    // zeta explicit, then `*.yass.yaml` (alpha, zeta): zeta seen first; the glob's
    // zeta is a dup. N = 2 (zeta, alpha).
    auto o = expect_stdout_exit_match(ref_args({"zeta.yass.yaml", "*.yass.yaml"}),
                                      runner_for({"zeta.yass.yaml", "*.yass.yaml"}));
    auto c = run_local({"zeta.yass.yaml", "*.yass.yaml"});
    CHECK(c.out == "checked 2 files, found 0 errors\n");
    CHECK(c.exit == 0);
}

// ===========================================================================
// CheckYAML — empty file, malformed YAML (line attributed), counts as one error.
// ===========================================================================
TEST_CASE("validate: empty file -> yaml.empty_file, counts as one error, exit 1") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("empty.yass.yaml", "");  // zero bytes.
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"empty.yass.yaml"}),
                                      runner_for({"empty.yass.yaml"}));
    auto c = run_local({"empty.yass.yaml"});
    // (RETURN) empty counts as exactly one CheckYAML error in M; file counts in N.
    CHECK(c.out == "checked 1 files, found 1 errors\n");
    CHECK(c.exit == 1);
    CHECK(c.err == "empty.yass.yaml: [yass.yaml.empty_file] empty file\n");
    if (o.ref_available) CHECK(has_code(o.ref_err, "yass.yaml.empty_file"));
}

TEST_CASE("validate: malformed YAML attributes a line and matches the reference") {
    TmpTree tree;
    tree.mkdir(".git");
    // A tab-indented sequence item: ryml AND the reference agree on the line (7).
    tree.write("m.yass.yaml",
               "---\ndescription: d\nversion: v1\n---\nspec: ccc\nINPUT:\n\t- MUST: x\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"m.yass.yaml"}), runner_for({"m.yass.yaml"}));
    auto c = run_local({"m.yass.yaml"});
    CHECK(c.out == "checked 1 files, found 1 errors\n");
    CHECK(c.exit == 1);
    // (CheckYAML) malformed attributes to line 7; one error for the file.
    CHECK(c.err == "m.yass.yaml:7: [yass.yaml.malformed] YAML well-formedness error\n");
    if (o.ref_available) {
        // Both stdout+exit (asserted above by expect_stdout_exit_match) and the
        // malformed line agree with the reference.
        CHECK(has_code(o.ref_err, "yass.yaml.malformed"));
        CHECK(o.ref_err.find("m.yass.yaml:7:") != std::string::npos);
    }
}

TEST_CASE("validate: malformed YAML skips structural checks (one error only)") {
    TmpTree tree;
    tree.mkdir(".git");
    // Malformed YAML that would ALSO have a bad spec name if it parsed; the bad
    // name must NOT be reported because CheckYAML failure skips the rest.
    tree.write("m2.yass.yaml",
               "---\ndescription: d\nversion: v1\n---\nspec: \"foo bar\"\nINPUT:\n\t- MUST: x\n");
    CwdGuard cd(tree.root());

    auto c = run_local({"m2.yass.yaml"});
    CHECK(c.out == "checked 1 files, found 1 errors\n");
    CHECK(c.exit == 1);
    CHECK(has_code(c.err, "yass.yaml.malformed"));
    // (RETURN: WHEN YAML parsing fails MUST skip CheckSpec) -> no name_bad_chars.
    CHECK_FALSE(has_code(c.err, "yass.spec.name_bad_chars"));
}

// ===========================================================================
// RETURN — continue after a per-file failure; report all errors across files.
// ===========================================================================
TEST_CASE("validate: continues after one file fails; reports all errors; M counts lines") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("aaa.yass.yaml", "");  // empty -> 1 error
    tree.write("bbb.yass.yaml",
               std::string(kPre) + "spec: \"bad name!\"\n");  // name_bad_chars -> 1 error
    tree.write("ccc.yass.yaml",
               std::string(kPre) + "spec: ccc\nINPUT:\n- MUST: ok\n");  // clean
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({}), runner_for({}));
    auto c = run_local({});
    // 3 files discovered, 2 errors found, exit 1.
    CHECK(c.out == "checked 3 files, found 2 errors\n");
    CHECK(c.exit == 1);
    CHECK(has_code(c.err, "yass.yaml.empty_file"));
    CHECK(has_code(c.err, "yass.spec.name_bad_chars"));
    // (MUST-NOT interleave) aaa's error precedes bbb's error (sort order).
    CHECK(c.err.find("aaa.yass.yaml") < c.err.find("bbb.yass.yaml"));
}

// ===========================================================================
// SIDE-EFFECT — MUST-NOT modify any input file.
// ===========================================================================
TEST_CASE("validate: does not modify input files") {
    TmpTree tree;
    tree.mkdir(".git");
    std::string content = std::string(kPre) + "spec: \"bad name!\"\n";
    fs::path file = tree.write("x.yass.yaml", content);
    CwdGuard cd(tree.root());

    auto before = fs::last_write_time(file);
    auto c = run_local({"x.yass.yaml"});
    CHECK(c.exit == 1);
    // (SIDE-EFFECT MUST-NOT modify) content and mtime unchanged.
    std::ifstream is(file, std::ios::binary);
    std::string after((std::istreambuf_iterator<char>(is)), std::istreambuf_iterator<char>());
    CHECK(after == content);
    CHECK(fs::last_write_time(file) == before);
}

// ===========================================================================
// cli.DiscoverSpecFiles ERROR (CONFORMS cli.ErrorLine) — `validate .` MUST emit
// the dir_unreadable warning for an unreadable subdirectory, continue checking
// the readable siblings, and NOT count the warning toward the M error total.
// Previously the warning was silently dropped.
// ===========================================================================
TEST_CASE("validate: an unreadable subdir emits dir_unreadable but is not counted in M") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("top.yass.yaml", std::string(kPre) + "spec: top.Thing\nINPUT:\n- MUST: x\n");
    fs::path locked = tree.mkdir("locked");
    tree.write("locked/hidden.yass.yaml", std::string(kPre) + "spec: locked.Thing\nINPUT:\n- MUST: x\n");
    fs::permissions(locked, fs::perms::none, fs::perm_options::replace);
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"."}), runner_for({"."}));
    auto c = run_local({"."});

    fs::permissions(locked, fs::perms::owner_all, fs::perm_options::replace);

    // The readable top.yass.yaml is checked and clean: M = 0 (the dir_unreadable
    // warning does NOT count), exit 0.
    CHECK(c.out == "checked 1 files, found 0 errors\n");
    CHECK(c.exit == 0);
    // The warning is on stderr, attributed to the locked directory.
    CHECK(has_code(c.err, "yass.discover.dir_unreadable"));
    CHECK(c.err.find("locked") != std::string::npos);
    if (o.ref_available) {
        CHECK(c.out == o.ref_out);
        CHECK(c.err == o.ref_err);
        CHECK(c.exit == o.ref_exit);
    }
}

// ===========================================================================
// INPUT — `-` is a literal path, NOT a stdin marker (spec wins over reference).
// ===========================================================================
TEST_CASE("validate: bare '-' is a literal path (spec), yielding not_found exit 2") {
    TmpTree tree;
    tree.mkdir(".git");
    CwdGuard cd(tree.root());

    // Per cli.validate INPUT, `-` MUST be a literal path. It does not exist ->
    // yass.path.not_found, exit 2, WITH the summary line. (The reference diverges:
    // it emits yass.argv.stdin_dash with no summary; we follow the SPEC. Exit 2
    // still matches the reference, but stdout differs, so this is asserted
    // directly rather than via the differential helper.)
    auto c = run_local({"-"});
    CHECK(c.out == "checked 0 files, found 0 errors\n");
    CHECK(c.exit == 2);
    CHECK(has_code(c.err, "yass.path.not_found"));
    CHECK_FALSE(has_code(c.err, "yass.argv.stdin_dash"));
}

// ===========================================================================
// End-to-end oracle — our run_validate over the REAL repo `spec` dir matches
// `yass validate spec` (checked 6 files, found 0 errors, exit 0).
// ===========================================================================
TEST_CASE("validate: real repo spec directory matches the reference end-to-end") {
    // Locate the repo root by walking up from this test's build dir is fragile;
    // instead derive it from YASS_BIN (build/yass) -> repo root is its parent's
    // parent. Fall back to a few candidates.
    fs::path repo;
    {
        fs::path bin = fs::path(YASS_BIN);            // <repo>/build/yass
        fs::path cand = bin.parent_path().parent_path();  // <repo>
        std::error_code ec;
        if (fs::is_directory(cand / "spec", ec)) repo = cand;
    }
    if (repo.empty()) {
        // Skip if we cannot locate the repo (keeps the suite robust off-tree).
        WARN("could not locate repo root; skipping real-spec oracle");
        return;
    }
    CwdGuard cd(repo);

    auto o = expect_stdout_exit_match(ref_args({"spec"}), runner_for({"spec"}));
    auto c = run_local({"spec"});
    CHECK(c.out == "checked 6 files, found 0 errors\n");
    CHECK(c.exit == 0);
    CHECK(c.err.empty());
    if (o.ref_available) {
        CHECK(o.ref_out == "checked 6 files, found 0 errors\n");
        CHECK(o.ref_exit == 0);
    }
}
