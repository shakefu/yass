// M7 — the `query` subcommand tests.
//
// Spec basis (spec/cli.query.yass.yaml :: cli.query / cli.query.NameLookup /
// cli.query.ExtractFragment / cli.query.InlineConforms / cli.query.OutputProfile),
// plus spec/cli.yass.yaml :: cli.ErrorLine / cli.ExitCode, spec/cli.list.yass.yaml
// (the disambiguation row format), and spec/cli.shared.yass.yaml (discover /
// findroot, via src/fs.hpp).
//
// Conformance policy (tech lead): stdout bytes AND the process exit code MUST
// match the reference `yass query ...` byte-for-byte (differential, via
// tests/support/diff.hpp). stderr follows the SPEC (cli.errors message PROSE via
// diag::canonical_message, cli.ErrorLine format, spec ordering) with the error
// CODE / LINE / EXIT still matching the reference where the reference agrees.
//
// The conformance backbone is the corpus sweep: every (file, name) the reference
// `list` reports is queried single-scope and asserted byte-for-byte against the
// reference. That deterministic single-match form covers the full emitter,
// quoting, key-reorder, and CONFORMS-inlining surface across the real corpus
// (incl. yass.yass.yaml's Keyword::INVARIANT reference-only inline and the cli.*
// CONFORMS/USES refs). The crafted tmptrees then pin multi-match dispatch and
// every error path.
//
// Known spec/reference divergence (documented, asserted on the SPEC side only):
// an EMPTY spec name. The reference's top-level dispatcher rejects an empty
// positional argument with yass.argv.empty_argument (exit 2) BEFORE the query
// handler runs, whereas cli.query.NameLookup mandates yass.query.name_blank when
// the spec name is empty. run_query receives the tokens AFTER `query`, so it
// follows the SPEC and emits name_blank. Both exit 2 with empty stdout, so the
// stdout+exit differential still holds; only the stderr [code] differs.

#include "doctest.h"

#include <algorithm>
#include <filesystem>
#include <sstream>
#include <string>
#include <vector>

#include "diag.hpp"
#include "emit.hpp"
#include "query.hpp"
#include "support/diff.hpp"
#include "support/proc.hpp"
#include "support/tmptree.hpp"

namespace {

namespace fs = std::filesystem;
using yass::test::CwdGuard;
using yass::test::DiffOutcome;
using yass::test::expect_stdout_exit_match;
using yass::test::find_ref_bin;
using yass::test::TmpTree;

// A minimal valid Preamble (description + version v1).
constexpr const char* kPre = "---\ndescription: a test\nversion: v1\n---\n";

// The reference args for `query` = {"query"} + args.
std::vector<std::string> ref_args(const std::vector<std::string>& args) {
    std::vector<std::string> v;
    v.push_back("query");
    v.insert(v.end(), args.begin(), args.end());
    return v;
}

yass::test::InProcRunner runner_for(const std::vector<std::string>& args) {
    return [args](std::ostream& out, std::ostream& err) {
        return yass::query::run_query(args, out, err);
    };
}

struct Captured {
    int exit = 0;
    std::string out;
    std::string err;
};
Captured run_local(const std::vector<std::string>& args) {
    std::ostringstream out, err;
    Captured c;
    c.exit = yass::query::run_query(args, out, err);
    c.out = out.str();
    c.err = err.str();
    return c;
}

bool has_code(const std::string& err, const char* code) {
    return err.find(std::string("[") + code + "]") != std::string::npos;
}

// One (file, name) pair from the reference `list` output.
struct ListRow {
    std::string file;
    std::string name;
};

// Run the reference `list` from `cwd` and parse the file<TAB>name<TAB>desc rows.
std::vector<ListRow> reference_list(const std::string& ref_bin, const fs::path& cwd) {
    yass::test::ProcResult r = yass::test::run({ref_bin, "list"}, cwd.string());
    std::vector<ListRow> rows;
    std::size_t pos = 0;
    const std::string& s = r.out;
    while (pos < s.size()) {
        std::size_t eol = s.find('\n', pos);
        if (eol == std::string::npos) eol = s.size();
        std::string line = s.substr(pos, eol - pos);
        pos = eol + 1;
        std::size_t t1 = line.find('\t');
        if (t1 == std::string::npos) continue;
        std::size_t t2 = line.find('\t', t1 + 1);
        if (t2 == std::string::npos) continue;
        rows.push_back(ListRow{line.substr(0, t1), line.substr(t1 + 1, t2 - t1 - 1)});
    }
    return rows;
}

}  // namespace

// ===========================================================================
// Conformance backbone — corpus sweep: every (file, name) single-scope query
// matches the reference byte-for-byte (stdout + exit). This is deterministic
// (file+name yields exactly one match) and exercises the whole emitter.
// ===========================================================================
TEST_CASE("query: every spec in the repo round-trips against the reference") {
    std::string ref = find_ref_bin();
    if (ref.empty()) {
        WARN_MESSAGE(false, "reference yass unavailable; skipping corpus sweep");
        return;
    }
    // The repo root is the test process's starting cwd (tests run from the repo).
    fs::path repo_root = fs::current_path();
    // Be robust if tests are launched from build/: walk up to the dir holding spec/.
    for (int i = 0; i < 5 && !fs::exists(repo_root / "spec"); ++i) {
        repo_root = repo_root.parent_path();
    }
    REQUIRE(fs::exists(repo_root / "spec"));

    CwdGuard cd(repo_root);
    std::vector<ListRow> rows = reference_list(ref, repo_root);
    REQUIRE(rows.size() > 10);  // sanity: the corpus is non-trivial.

    int checked = 0;
    for (const ListRow& row : rows) {
        // Single-scope form: deterministic exactly-one match per (file, name).
        expect_stdout_exit_match(ref_args({row.name, row.file}),
                                 runner_for({row.name, row.file}));
        ++checked;
    }
    CHECK(checked == static_cast<int>(rows.size()));
}

// A no-scope query for a unique trailing suffix also matches the reference
// (project-root search from a subdirectory).
TEST_CASE("query: no-scope trailing-suffix match from a subdirectory") {
    std::string ref = find_ref_bin();
    fs::path repo_root = fs::current_path();
    for (int i = 0; i < 5 && !fs::exists(repo_root / "spec"); ++i) {
        repo_root = repo_root.parent_path();
    }
    REQUIRE(fs::exists(repo_root / "spec"));
    CwdGuard cd(repo_root / "spec");

    // 'CheckYAML' is a trailing suffix of cli.validate.CheckYAML (unique).
    auto o = expect_stdout_exit_match(ref_args({"CheckYAML"}), runner_for({"CheckYAML"}));
    auto c = run_local({"CheckYAML"});
    CHECK(c.exit == 0);
    CHECK(c.out.rfind("---\nspec: cli.validate.CheckYAML\n", 0) == 0);
    if (o.ref_available) CHECK(c.out == o.ref_out);
}

// ===========================================================================
// cli.query.NameLookup — matching semantics.
// ===========================================================================
TEST_CASE("query: matches a full spec name and any dot-aligned trailing suffix") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("a.yass.yaml", std::string(kPre) + "spec: ns.deep.Thing\nINPUT:\n- MUST: x\n");
    CwdGuard cd(tree.root());

    // Full name.
    CHECK(run_local({"ns.deep.Thing"}).exit == 0);
    // Trailing suffixes (one or more leading components removed).
    CHECK(run_local({"deep.Thing"}).exit == 0);
    CHECK(run_local({"Thing"}).exit == 0);
    // A LEADING component is NOT a trailing suffix -> no match.
    CHECK(run_local({"ns"}).exit == 1);
    CHECK(run_local({"ns.deep"}).exit == 1);
    // A non-component substring is NOT a match.
    CHECK(run_local({"hing"}).exit == 1);
    CHECK(run_local({"ep.Thing"}).exit == 1);
    // Case-sensitive: no case folding.
    CHECK(run_local({"thing"}).exit == 1);
    CHECK(run_local({"NS.DEEP.THING"}).exit == 1);
}

TEST_CASE("query: matching matches the reference for full name and suffix") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("a.yass.yaml", std::string(kPre) + "spec: ns.deep.Thing\nINPUT:\n- MUST: x\n");
    CwdGuard cd(tree.root());
    expect_stdout_exit_match(ref_args({"ns.deep.Thing"}), runner_for({"ns.deep.Thing"}));
    expect_stdout_exit_match(ref_args({"deep.Thing"}), runner_for({"deep.Thing"}));
    expect_stdout_exit_match(ref_args({"Thing"}), runner_for({"Thing"}));
    expect_stdout_exit_match(ref_args({"thing"}), runner_for({"thing"}));
}

// ===========================================================================
// cli.query RETURN — multi-match disambiguation rows.
// ===========================================================================
TEST_CASE("query: more than one match emits cli.list rows to stdout, exit 0, no stderr") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("a.yass.yaml", std::string(kPre) + "spec: alpha.Common\nINPUT:\n- MUST: do a thing\n");
    tree.write("b.yass.yaml", std::string(kPre) + "spec: beta.Common\nINPUT:\n- MUST: do other\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"Common"}), runner_for({"Common"}));
    auto c = run_local({"Common"});
    // file<TAB>name<TAB>description rows in cli.list order (files code-point sorted).
    CHECK(c.out ==
          "a.yass.yaml\talpha.Common\ta test\n"
          "b.yass.yaml\tbeta.Common\ta test\n");
    CHECK(c.exit == 0);
    CHECK(c.err.empty());  // MUST-NOT write any line to stderr.
    if (o.ref_available) {
        CHECK(c.out == o.ref_out);
        CHECK(o.ref_err.empty());
    }
}

TEST_CASE("query: multi-match preserves description and never truncates") {
    TmpTree tree;
    tree.mkdir(".git");
    std::string longdesc =
        "---\ndescription: >\n  " + std::string(200, 'x') + "\nversion: v1\n---\n";
    tree.write("a.yass.yaml", longdesc + "spec: a.Wide\nINPUT:\n- MUST: x\n");
    tree.write("b.yass.yaml", longdesc + "spec: b.Wide\nINPUT:\n- MUST: y\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"Wide"}), runner_for({"Wide"}));
    auto c = run_local({"Wide"});
    // The 200-char description survives intact (MUST-NOT truncate regardless of TTY).
    CHECK(c.out.find(std::string(200, 'x')) != std::string::npos);
    CHECK(c.exit == 0);
    if (o.ref_available) CHECK(c.out == o.ref_out);
}

// ===========================================================================
// cli.query RETURN / NameLookup ERROR — zero matches.
// ===========================================================================
TEST_CASE("query: zero matches -> no stdout, one yass.query.no_match ErrorLine, exit 1") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("a.yass.yaml", std::string(kPre) + "spec: ns.Thing\nINPUT:\n- MUST: x\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"Nope"}), runner_for({"Nope"}));
    auto c = run_local({"Nope"});
    CHECK(c.out.empty());
    CHECK(c.exit == 1);
    CHECK(c.err == "yass: [yass.query.no_match] no spec matches: Nope\n");
    if (o.ref_available) {
        CHECK(o.ref_out.empty());
        CHECK(has_code(o.ref_err, "yass.query.no_match"));
    }
}

TEST_CASE("query: a whitespace-bearing name is a no-match, not a blank error") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("a.yass.yaml", std::string(kPre) + "spec: ns.Thing\nINPUT:\n- MUST: x\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"a b"}), runner_for({"a b"}));
    auto c = run_local({"a b"});
    CHECK(c.out.empty());
    CHECK(c.exit == 1);
    CHECK(has_code(c.err, "yass.query.no_match"));      // NOT name_blank.
    CHECK(!has_code(c.err, "yass.query.name_blank"));
    if (o.ref_available) CHECK(has_code(o.ref_err, "yass.query.no_match"));
}

// ===========================================================================
// cli.query ERROR / NameLookup ERROR — name_missing / name_blank.
// ===========================================================================
TEST_CASE("query: no spec name -> yass.query.name_missing, exit 2") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("a.yass.yaml", std::string(kPre) + "spec: ns.Thing\nINPUT:\n- MUST: x\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({}), runner_for({}));
    auto c = run_local({});
    CHECK(c.out.empty());
    CHECK(c.exit == 2);
    CHECK(c.err == "yass: [yass.query.name_missing] missing spec name\n");
    if (o.ref_available) {
        CHECK(o.ref_out.empty());
        CHECK(has_code(o.ref_err, "yass.query.name_missing"));
    }
}

TEST_CASE("query: empty spec name -> yass.query.name_blank (SPEC), exit 2 matches reference") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("a.yass.yaml", std::string(kPre) + "spec: ns.Thing\nINPUT:\n- MUST: x\n");
    CwdGuard cd(tree.root());

    // Differential: stdout (empty) + exit (2) match the reference; the reference's
    // dispatcher uses argv.empty_argument, but run_query (post-`query` tokens)
    // follows the SPEC and emits name_blank. Both exit 2 with empty stdout.
    auto o = expect_stdout_exit_match(ref_args({""}), runner_for({""}));
    auto c = run_local({""});
    CHECK(c.out.empty());
    CHECK(c.exit == 2);
    CHECK(c.err == "yass: [yass.query.name_blank] spec name is blank or contains whitespace\n");
    if (o.ref_available) {
        CHECK(o.ref_out.empty());
        CHECK(o.ref_exit == 2);
    }
}

// ===========================================================================
// cli.query INPUT / ERROR — scope validation (BEFORE name lookup).
// ===========================================================================
TEST_CASE("query: a scope containing ':' -> yass.path.colon_in_path, exit 2") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("a.yass.yaml", std::string(kPre) + "spec: ns.Thing\nINPUT:\n- MUST: x\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"Thing", "a:b"}), runner_for({"Thing", "a:b"}));
    auto c = run_local({"Thing", "a:b"});
    CHECK(c.out.empty());
    CHECK(c.exit == 2);
    CHECK(c.err ==
          "yass: [yass.path.colon_in_path] path contains an unsupported colon character: a:b\n");
    if (o.ref_available) CHECK(has_code(o.ref_err, "yass.path.colon_in_path"));
}

TEST_CASE("query: nonexistent scope -> yass.query.scope_not_found, exit 2") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("a.yass.yaml", std::string(kPre) + "spec: ns.Thing\nINPUT:\n- MUST: x\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"Thing", "nodir"}), runner_for({"Thing", "nodir"}));
    auto c = run_local({"Thing", "nodir"});
    CHECK(c.out.empty());
    CHECK(c.exit == 2);
    CHECK(c.err ==
          "nodir: [yass.query.scope_not_found] scope path does not exist: nodir\n");
    if (o.ref_available) CHECK(has_code(o.ref_err, "yass.query.scope_not_found"));
}

TEST_CASE("query: empty scope dir -> yass.query.scope_empty, exit 2") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("a.yass.yaml", std::string(kPre) + "spec: ns.Thing\nINPUT:\n- MUST: x\n");
    tree.mkdir("empty");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"Thing", "empty"}), runner_for({"Thing", "empty"}));
    auto c = run_local({"Thing", "empty"});
    CHECK(c.out.empty());
    CHECK(c.exit == 2);
    CHECK(c.err ==
          "empty: [yass.query.scope_empty] no .yass.yaml files found in scope: empty\n");
    if (o.ref_available) CHECK(has_code(o.ref_err, "yass.query.scope_empty"));
}

TEST_CASE("query: when scope is invalid the scope error is the only one emitted") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("a.yass.yaml", std::string(kPre) + "spec: ns.Thing\nINPUT:\n- MUST: x\n");
    CwdGuard cd(tree.root());

    // Scope invalid AND name would not match: emit ONLY the scope error.
    auto c = run_local({"Nope", "nodir"});
    CHECK(c.exit == 2);
    CHECK(has_code(c.err, "yass.query.scope_not_found"));
    CHECK(!has_code(c.err, "yass.query.no_match"));
    expect_stdout_exit_match(ref_args({"Nope", "nodir"}), runner_for({"Nope", "nodir"}));
}

TEST_CASE("query: a single-file scope searches only that file") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("a.yass.yaml", std::string(kPre) + "spec: a.Thing\nINPUT:\n- MUST: x\n");
    tree.write("b.yass.yaml", std::string(kPre) + "spec: b.Thing\nINPUT:\n- MUST: y\n");
    CwdGuard cd(tree.root());

    // b.Thing is not in a.yass.yaml -> no_match within that file scope.
    auto c = run_local({"b.Thing", "a.yass.yaml"});
    CHECK(c.exit == 1);
    CHECK(has_code(c.err, "yass.query.no_match"));
    expect_stdout_exit_match(ref_args({"b.Thing", "a.yass.yaml"}),
                             runner_for({"b.Thing", "a.yass.yaml"}));
    // a.Thing IS in a.yass.yaml -> single match.
    auto c2 = run_local({"a.Thing", "a.yass.yaml"});
    CHECK(c2.exit == 0);
    expect_stdout_exit_match(ref_args({"a.Thing", "a.yass.yaml"}),
                             runner_for({"a.Thing", "a.yass.yaml"}));
}

// ===========================================================================
// cli.query.InlineConforms — successful inlining (reference-only replace,
// normative-carrier append, WHEN-guard preservation + combination, provenance).
// ===========================================================================
TEST_CASE("query: CONFORMS inlining (replace, append, WHEN-combine, provenance)") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("x.yass.yaml",
               std::string(kPre) +
                   "spec: x.Target\n"
                   "INPUT:\n"
                   "- MUST: target one\n"
                   "- SHOULD: target two\n"
                   "  WHEN: the moon is full\n"
                   "---\n"
                   "spec: x.Carrier\n"
                   "INPUT:\n"
                   "- CONFORMS: x.Target::INPUT\n"
                   "- MUST: keep me\n"
                   "  WHEN: outer guard holds\n"
                   "  CONFORMS: x.Target::INPUT\n"
                   "  USES: x.Target\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"x.Carrier", "x.yass.yaml"}),
                                      runner_for({"x.Carrier", "x.yass.yaml"}));
    auto c = run_local({"x.Carrier", "x.yass.yaml"});
    CHECK(c.exit == 0);
    // Reference-only carrier REPLACED; normative carrier KEPT (CONFORMS stripped,
    // USES kept) with inlined obligations appended; outer WHEN preserved /
    // combined; provenance comments at column zero above each inlined obligation.
    const std::string expected =
        "---\n"
        "spec: x.Carrier\n"
        "INPUT:\n"
        "# CONFORMS: x.Target::INPUT\n"
        "- MUST: target one\n"
        "# CONFORMS: x.Target::INPUT\n"
        "- SHOULD: target two\n"
        "  WHEN: the moon is full\n"
        "- MUST: keep me\n"
        "  WHEN: outer guard holds\n"
        "  USES: x.Target\n"
        "# CONFORMS: x.Target::INPUT\n"
        "- MUST: target one\n"
        "  WHEN: outer guard holds\n"
        "# CONFORMS: x.Target::INPUT\n"
        "- SHOULD: target two\n"
        "  WHEN: outer guard holds and the moon is full\n";
    CHECK(c.out == expected);
    if (o.ref_available) CHECK(c.out == o.ref_out);
}

// cli.query.InlineConforms INVARIANT: "MUST-NOT recursively resolve CONFORMS
// refs found within inlined obligations"; RETURN: "preserve each inlined
// obligation as written (keep its Normativity, refs)". The reference keeps an
// inlined obligation's OWN CONFORMS ref VERBATIM (one level deep = do not
// recurse, but the inner ref text is retained as a relation). Earlier our
// read_obligation routed the inner CONFORMS into conforms_target and dropped it.
TEST_CASE("query: an inlined obligation's own CONFORMS is preserved (not recursed, not dropped)") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("c.yass.yaml",
               std::string(kPre) +
                   "spec: c.Carrier\n"
                   "INPUT:\n"
                   "- WHEN: outer guard\n"
                   "  CONFORMS: c.Deep::INPUT\n"
                   "---\n"
                   "spec: c.Deep\n"
                   "INPUT:\n"
                   "- MUST: deep obligation\n"
                   "  USES: c.Other::INPUT\n"
                   "  CONFORMS: c.Inner::INPUT\n"
                   "  SEE: c.Last::INPUT\n"
                   "---\nspec: c.Other\nINPUT:\n- MUST: o\n"
                   "---\nspec: c.Inner\nINPUT:\n- MUST: i\n"
                   "---\nspec: c.Last\nINPUT:\n- MUST: l\n");
    CwdGuard cd(tree.root());

    auto c = run_local({"c.Carrier", "c.yass.yaml"});
    CHECK(c.exit == 0);
    // The inlined obligation preserves ALL of its relations in source order,
    // including its OWN CONFORMS (NOT recursively resolved). The outer WHEN is
    // applied; the carrier's resolved CONFORMS becomes a provenance comment.
    // The carrier carries a WHEN guard (so it is NOT reference-only): it is KEPT
    // (its resolved CONFORMS stripped) and the inlined obligation is appended.
    const std::string expected =
        "---\n"
        "spec: c.Carrier\n"
        "INPUT:\n"
        "- WHEN: outer guard\n"
        "# CONFORMS: c.Deep::INPUT\n"
        "- MUST: deep obligation\n"
        "  WHEN: outer guard\n"
        "  USES: c.Other::INPUT\n"
        "  CONFORMS: c.Inner::INPUT\n"
        "  SEE: c.Last::INPUT\n";
    CHECK(c.out == expected);
    auto o = expect_stdout_exit_match(ref_args({"c.Carrier", "c.yass.yaml"}),
                                      runner_for({"c.Carrier", "c.yass.yaml"}));
    if (o.ref_available) CHECK(c.out == o.ref_out);
}

TEST_CASE("query: CONFORMS to a ./-relative file resolves against the matched spec dir") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("sub/dep.yass.yaml", std::string(kPre) + "spec: dep.Thing\nINPUT:\n- MUST: dep one\n");
    tree.write("sub/host.yass.yaml",
               std::string(kPre) + "spec: host.Spec\nINPUT:\n- CONFORMS: ./dep@dep.Thing::INPUT\n");

    {
        CwdGuard cd(tree.root());
        auto c = run_local({"host.Spec", "sub/host.yass.yaml"});
        CHECK(c.exit == 0);
        CHECK(c.out ==
              "---\nspec: host.Spec\nINPUT:\n# CONFORMS: ./dep@dep.Thing::INPUT\n- MUST: dep one\n");
        expect_stdout_exit_match(ref_args({"host.Spec", "sub/host.yass.yaml"}),
                                 runner_for({"host.Spec", "sub/host.yass.yaml"}));
    }
    {
        // Same result regardless of scope/cwd: resolve against the matched spec dir.
        CwdGuard cd(tree.root() / "sub");
        auto c = run_local({"host.Spec", "host.yass.yaml"});
        CHECK(c.exit == 0);
        expect_stdout_exit_match(ref_args({"host.Spec", "host.yass.yaml"}),
                                 runner_for({"host.Spec", "host.yass.yaml"}));
    }
}

// ===========================================================================
// cli.query.InlineConforms ERROR — conforms_no_slot / conforms_unresolved.
// ===========================================================================
TEST_CASE("query: a CONFORMS ref lacking ::SLOT -> yass.query.conforms_no_slot, exit 1") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("x.yass.yaml",
               std::string(kPre) +
                   "spec: x.Target\nINPUT:\n- MUST: one\n"
                   "---\nspec: x.Carrier\nINPUT:\n- CONFORMS: x.Target\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"x.Carrier", "x.yass.yaml"}),
                                      runner_for({"x.Carrier", "x.yass.yaml"}));
    auto c = run_local({"x.Carrier", "x.yass.yaml"});
    CHECK(c.out.empty());  // MUST-NOT emit a partial fragment.
    CHECK(c.exit == 1);
    CHECK(c.err ==
          "x.yass.yaml: [yass.query.conforms_no_slot] "
          "CONFORMS ref must address a slot in v1: x.Target\n");
    if (o.ref_available) {
        CHECK(o.ref_out.empty());
        CHECK(has_code(o.ref_err, "yass.query.conforms_no_slot"));
    }
}

TEST_CASE("query: a CONFORMS ref that fails to resolve -> yass.query.conforms_unresolved, exit 1") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("y.yass.yaml",
               std::string(kPre) + "spec: y.Carrier\nINPUT:\n- CONFORMS: y.Missing::INPUT\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"y.Carrier", "y.yass.yaml"}),
                                      runner_for({"y.Carrier", "y.yass.yaml"}));
    auto c = run_local({"y.Carrier", "y.yass.yaml"});
    CHECK(c.out.empty());
    CHECK(c.exit == 1);
    CHECK(c.err ==
          "y.yass.yaml: [yass.query.conforms_unresolved] unresolvable CONFORMS ref: y.Missing::INPUT\n");
    if (o.ref_available) {
        CHECK(o.ref_out.empty());
        CHECK(has_code(o.ref_err, "yass.query.conforms_unresolved"));
    }
}

TEST_CASE("query: CONFORMS to a declared spec but an undeclared slot -> conforms_unresolved") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("z.yass.yaml",
               std::string(kPre) +
                   "spec: z.Target\nINPUT:\n- MUST: one\n"
                   "---\nspec: z.Carrier\nRETURN:\n- CONFORMS: z.Target::ERROR\n");
    CwdGuard cd(tree.root());

    auto c = run_local({"z.Carrier", "z.yass.yaml"});
    CHECK(c.out.empty());
    CHECK(c.exit == 1);
    CHECK(has_code(c.err, "yass.query.conforms_unresolved"));
    expect_stdout_exit_match(ref_args({"z.Carrier", "z.yass.yaml"}),
                             runner_for({"z.Carrier", "z.yass.yaml"}));
}

// ===========================================================================
// cli.query.OutputProfile — scalar quoting predicate (direct unit tests).
// ===========================================================================
TEST_CASE("emit: needs_double_quote follows the OutputProfile rule") {
    using yass::emit::needs_double_quote;
    // Plain by default.
    CHECK(!needs_double_quote("hello world"));
    CHECK(!needs_double_quote("a-b-c"));            // internal dash is fine.
    CHECK(!needs_double_quote("be one of MUST, MAY"));  // commas / keywords in prose.
    CHECK(!needs_double_quote("match `^[A-Za-z]+$`"));  // backtick / regex in prose.
    CHECK(!needs_double_quote("a\\.b"));            // a plain literal backslash.

    // colon-space anywhere.
    CHECK(needs_double_quote("key: value"));
    CHECK(needs_double_quote("a `: ` token"));
    CHECK(!needs_double_quote("ratio 3:4"));        // colon WITHOUT a following space.

    // leading indicator characters.
    for (const char* s : {"- dash", "? q", "* star", "& amp", "! bang", "| pipe", "> fold",
                          "% pct", "@ at"}) {
        CHECK(needs_double_quote(s));
    }

    // leading / trailing whitespace; empty.
    CHECK(needs_double_quote(" leading"));
    CHECK(needs_double_quote("trailing "));
    CHECK(needs_double_quote(""));

    // core-schema tokens (case-insensitive) and yes/no/on/off.
    for (const char* s : {"true", "False", "NULL", "~", "yes", "No", "on", "OFF"}) {
        CHECK(needs_double_quote(s));
    }

    // cli.query.OutputProfile RETURN: the enumerated quoting triggers do NOT
    // include interior control bytes other than a line break, nor an interior
    // double-quote. A LINE BREAK (LF or CR) cannot live in a single-line plain
    // scalar and DOES force quoting; an interior TAB / FF / VT / ESC / DEL and an
    // interior `"` must NOT trigger quoting (confirmed byte-for-byte against the
    // reference). Earlier our predicate over-quoted all C0/DEL bytes and `"`.
    CHECK(needs_double_quote("a\nb"));    // LF forces quoting.
    CHECK(needs_double_quote("a\rb"));    // CR forces quoting.
    CHECK(!needs_double_quote("a\tb"));   // interior TAB: PLAIN.
    CHECK(!needs_double_quote("a\fb"));   // interior FF: PLAIN.
    CHECK(!needs_double_quote("a\vb"));   // interior VT: PLAIN.
    CHECK(!needs_double_quote("a\x1b" "b")); // interior ESC: PLAIN.
    CHECK(!needs_double_quote("a\x7f" "b")); // interior DEL: PLAIN.
    CHECK(!needs_double_quote("quote \" here"));  // interior double-quote: PLAIN.
    // numeric literals: int / float / hex / octal / exponent / inf-nan.
    for (const char* s : {"0", "42", "-7", "+3", "3.14", ".5", "6.", "1e9", "-2.5E-3",
                          "0x1F", "0o17", ".inf", "-.inf", ".nan"}) {
        CHECK(needs_double_quote(s));
    }
    // string look-alikes that are NOT core tokens stay plain.
    for (const char* s : {"v1", "12abc", "0x", "1.2.3", "truthy", "onward"}) {
        CHECK(!needs_double_quote(s));
    }
}

TEST_CASE("emit: emit_scalar wraps and escapes only when required") {
    using yass::emit::emit_scalar;
    CHECK(emit_scalar("plain text") == "plain text");
    CHECK(emit_scalar("key: value") == "\"key: value\"");
    // A decoded line break (LF) forces a quoted, escaped scalar.
    CHECK(emit_scalar("a\nb") == "\"a\\nb\"");
    // cli.query.OutputProfile: an interior TAB does NOT trigger quoting -> the
    // scalar stays PLAIN with the literal tab byte (matches the reference).
    CHECK(emit_scalar("tab\there") == "tab\there");
    // An interior double-quote does NOT trigger quoting on its own -> PLAIN.
    CHECK(emit_scalar("say \"hi\"") == "say \"hi\"");
    // But when the scalar is quoted for another reason (a line break), interior
    // bytes ARE escaped: a tab becomes \t and a quote becomes \".
    CHECK(emit_scalar("tab\tand\nbreak") == "\"tab\\tand\\nbreak\"");
    CHECK(emit_scalar("q\"and\nbreak") == "\"q\\\"and\\nbreak\"");
}

TEST_CASE("emit: emit_fragment renders the OutputProfile shape") {
    using namespace yass::emit;
    Fragment f;
    f.spec_name = "ns.Thing";
    SlotGroup g;
    g.key = "RETURN";
    // An obligation whose source order was WHEN-before-MUST: the emitter reorders
    // to Normativity-keyword, then WHEN, then references.
    Obligation ob;
    ob.has_norm = true;
    ob.norm_key = "MUST";
    ob.norm_value = "do the thing";
    ob.has_when = true;
    ob.when_value = "some guard";
    ob.refs.push_back(Ref{"USES", "Other"});
    g.obligations.push_back(ob);
    // An inlined obligation with a provenance comment at column zero.
    Obligation inl;
    inl.has_norm = true;
    inl.norm_key = "MUST-NOT";
    inl.norm_value = "leak";
    inl.provenance = "x@y::RETURN";
    g.obligations.push_back(inl);
    f.slots.push_back(g);

    const std::string expected =
        "---\n"
        "spec: ns.Thing\n"
        "RETURN:\n"
        "- MUST: do the thing\n"
        "  WHEN: some guard\n"
        "  USES: Other\n"
        "# CONFORMS: x@y::RETURN\n"
        "- MUST-NOT: leak\n";
    CHECK(emit_fragment(f) == expected);
    // Ends with exactly one trailing LF and never emits a `...` end marker.
    CHECK(emit_fragment(f).back() == '\n');
    CHECK(emit_fragment(f).find("...") == std::string::npos);
}

// ===========================================================================
// cli.query.OutputProfile RETURN — interior control bytes (other than a line
// break) and an interior double-quote MUST NOT trigger quoting; the scalar is
// emitted PLAIN. End-to-end differential against the reference on a spec that
// validates clean on both binaries. Previously our predicate over-quoted these.
// ===========================================================================
TEST_CASE("cli.query OutputProfile: interior tab / quote / control bytes stay plain (differential)") {
    TmpTree tree;
    tree.mkdir(".git");
    // Double-quoted YAML escapes decode to interior control bytes / a quote.
    tree.write("oq.yass.yaml",
               std::string(kPre) +
                   "spec: oq.Thing\n"
                   "INPUT:\n"
                   "- MUST: \"a\\ttab inside\"\n"
                   "- MUST: \"quote \\\" here\"\n"
                   "- MUST: \"esc\\ehere\"\n"
                   "- MUST: \"del\\x7fhere\"\n"
                   "- MUST: \"nl\\nhere\"\n");
    CwdGuard cd(tree.root());

    auto o = expect_stdout_exit_match(ref_args({"oq.Thing", "oq.yass.yaml"}),
                                      runner_for({"oq.Thing", "oq.yass.yaml"}));
    auto c = run_local({"oq.Thing", "oq.yass.yaml"});
    CHECK(c.exit == 0);
    // The interior TAB stays plain (literal 0x09, NOT a \t escape and NOT quoted).
    CHECK(c.out.find("- MUST: a\ttab inside\n") != std::string::npos);
    // The interior double-quote stays plain.
    CHECK(c.out.find("- MUST: quote \" here\n") != std::string::npos);
    // A line break DOES force quoting + escaping.
    CHECK(c.out.find("- MUST: \"nl\\nhere\"\n") != std::string::npos);
    if (o.ref_available) CHECK(c.out == o.ref_out);
}

// ===========================================================================
// cli.FindProjectRoot ERROR (cli.ErrorLine) — when no project root marker is
// found, `query` (no scope) MUST render the no_marker ErrorLine with the
// absolute cwd as the <file> prefix, NOT the bare "yass" token. Before the fix
// query surfaced the fs diagnostic verbatim (empty <file> -> "yass").
// ===========================================================================
TEST_CASE("cli.query ERROR: findroot.no_marker renders the absolute-cwd <file> prefix") {
    TmpTree tree;
    fs::path sub = tree.mkdir("sub");
    CwdGuard cd(sub);

    auto o = expect_stdout_exit_match(ref_args({"Foo"}), runner_for({"Foo"}));
    auto c = run_local({"Foo"});

    if (o.ref_available && has_code(c.err, "yass.findroot.no_marker")) {
        // Not the bare "yass" prefix; begins with an absolute path.
        CHECK(c.err.rfind("yass:", 0) != 0);
        CHECK(c.err.front() == '/');
        CHECK(c.err == o.ref_err);
        CHECK(c.exit == o.ref_exit);
        CHECK(c.out == o.ref_out);
    } else {
        MESSAGE("temp root has a project marker above it (or no reference); "
                "skipping query no_marker prefix assertion");
    }
}

// ===========================================================================
// cli.DiscoverSpecFiles / cli.query — UNLIKE `list` and `validate`, the
// reference `query` does NOT surface the non-fatal dir_unreadable warning during
// scope discovery (its stderr stays empty for an unreadable subdir). We match the
// reference byte-for-byte: the match still succeeds, stderr is empty.
// ===========================================================================
TEST_CASE("cli.query: an unreadable subdir does NOT emit a dir_unreadable warning (matches reference)") {
    TmpTree tree;
    tree.mkdir(".git");
    tree.write("top.yass.yaml", std::string(kPre) + "spec: q.Top\nINPUT:\n- MUST: t\n");
    fs::path locked = tree.mkdir("locked");
    tree.write("locked/hidden.yass.yaml", std::string(kPre) + "spec: q.Hidden\nINPUT:\n- MUST: h\n");
    fs::permissions(locked, fs::perms::none, fs::perm_options::replace);

    CwdGuard cd(tree.root());
    auto o = expect_stdout_exit_match(ref_args({"q.Top"}), runner_for({"q.Top"}));
    auto c = run_local({"q.Top"});

    fs::permissions(locked, fs::perms::owner_all, fs::perm_options::replace);

    // The match still succeeds (exit 0) and the fragment is emitted; query emits
    // NO discover warning (unlike list/validate), so stderr is empty.
    CHECK(c.exit == 0);
    CHECK(c.out.find("spec: q.Top") != std::string::npos);
    CHECK_FALSE(has_code(c.err, "yass.discover.dir_unreadable"));
    CHECK(c.err.empty());
    if (o.ref_available) {
        CHECK(c.out == o.ref_out);
        CHECK(c.err == o.ref_err);
        CHECK(c.exit == o.ref_exit);
    }
}
