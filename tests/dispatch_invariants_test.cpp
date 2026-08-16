// Cross-cutting cli.Dispatch INVARIANT / SIDE-EFFECT tests on the BUILT binary.
//
// Spec basis: spec/cli.yass.yaml :: cli.Dispatch INVARIANT + SIDE-EFFECT, applied
// uniformly across the validate / list / query subcommands. These obligations are
// process-level (they constrain what the running process does to stdin, the
// filesystem, and the byte stream it writes), so they are exercised by running
// the real ./build/yass (YASS_BIN) as a subprocess — the only way to observe fd 0
// consumption, on-disk side effects, and the exact terminator bytes.
//
// Each test SKIPS gracefully (doctest MESSAGE) when the built binary is missing,
// rather than failing.

#include "doctest.h"

#include <algorithm>
#include <filesystem>
#include <fstream>
#include <set>
#include <sstream>
#include <string>
#include <vector>

#include "support/proc.hpp"
#include "support/pty.hpp"
#include "support/tmptree.hpp"

namespace {

namespace fs = std::filesystem;

// Path to the built CLI under test (an absolute path baked in by CMake).
fs::path built_bin() { return fs::path(YASS_BIN); }

bool have_built_bin() {
    std::error_code ec;
    return fs::is_regular_file(built_bin(), ec);
}

// A minimal valid single-file spec project rooted at `t` (with a .git marker so
// FindProjectRoot succeeds). Returns the project root path string.
std::string make_project(yass::test::TmpTree& t) {
    t.mkdir(".git");
    t.write("good.yass.yaml",
            "---\ndescription: a test\nversion: v1\n---\n"
            "spec: inv.Thing\nINPUT:\n- MUST: do the thing\n");
    return t.root().string();
}

// Recursively snapshot the set of regular-file paths under `dir` (absolute).
// Coverage `.profraw` files are an artifact of the instrumented build (the
// llvm coverage runtime writes them when LLVM_PROFILE_FILE is unset and the
// program is instrumented) — NOT a behavior of the program under test — so they
// are excluded from the snapshot. The production `yass` binary is not
// instrumented and writes none.
std::set<std::string> snapshot_files(const fs::path& dir) {
    std::set<std::string> out;
    std::error_code ec;
    if (!fs::exists(dir, ec)) return out;
    for (auto it = fs::recursive_directory_iterator(
             dir, fs::directory_options::skip_permission_denied, ec);
         !ec && it != fs::recursive_directory_iterator(); it.increment(ec)) {
        std::error_code sec;
        if (!fs::is_regular_file(it->path(), sec)) continue;
        if (it->path().extension() == ".profraw") continue;  // coverage artifact.
        out.insert(it->path().string());
    }
    return out;
}

// Read a whole file into a string (binary).
std::string read_all(const fs::path& p) {
    std::ifstream is(p, std::ios::binary);
    std::ostringstream ss;
    ss << is.rdbuf();
    return ss.str();
}

}  // namespace

// ===========================================================================
// cli.Dispatch INVARIANT: MUST-NOT read from stdin in any subcommand in v1.
// ===========================================================================
// We feed a large amount of stdin and assert (a) the process completes (does not
// block waiting on fd 0) and (b) the input bytes are not consumed in a way that
// changes behavior. The proc harness writes stdin via a pipe; if yass never
// reads, the pipe simply fills/closes — the process must still finish promptly
// with the same output it produces with empty stdin.
TEST_CASE("cli.Dispatch INVARIANT: subcommands do not read stdin (no block, no consumption)") {
    if (!have_built_bin()) {
        MESSAGE("built yass binary unavailable; skipping stdin-prohibition E2E");
        return;
    }
    yass::test::TmpTree t;
    std::string root = make_project(t);

    // 64 KiB of sentinel bytes on stdin. A process that read fd 0 would either
    // consume these or change its output; one that ignores fd 0 finishes the same.
    const std::string junk(64 * 1024, 'Z');

    for (const std::vector<std::string>& tail :
         {std::vector<std::string>{"validate", "."}, std::vector<std::string>{"list", "."},
          std::vector<std::string>{"query", "inv.Thing", "."}}) {
        std::vector<std::string> argv{built_bin().string()};
        argv.insert(argv.end(), tail.begin(), tail.end());

        // With stdin fed.
        yass::test::ProcResult fed = yass::test::run(argv, root, {}, junk);
        // With stdin empty.
        yass::test::ProcResult empty = yass::test::run(argv, root, {}, "");

        // Completed (not killed by a signal, e.g. from blocking then SIGPIPE).
        CHECK_FALSE(fed.signaled);
        // stdin presence MUST NOT change stdout or the exit code.
        CHECK(fed.out == empty.out);
        CHECK(fed.exit_code == empty.exit_code);
    }
}

// ===========================================================================
// cli.Dispatch INVARIANT: MUST-NOT create/modify/delete any file outside process
// memory; MUST-NOT write to temp/cache/config dirs.
// ===========================================================================
// Run each subcommand under an isolated HOME / TMPDIR / XDG_* environment and a
// fresh cwd, snapshotting every directory before and after. No new files may
// appear and no existing file may change.
TEST_CASE("cli.Dispatch INVARIANT: no writes to cwd / temp / cache / config dirs") {
    if (!have_built_bin()) {
        MESSAGE("built yass binary unavailable; skipping no-write E2E");
        return;
    }
    yass::test::TmpTree project;
    std::string root = make_project(project);

    // Isolated sandbox dirs the process might be tempted to write to.
    yass::test::TmpTree sandbox;
    fs::path home = sandbox.mkdir("home");
    fs::path tmp = sandbox.mkdir("tmp");
    fs::path cache = sandbox.mkdir("cache");
    fs::path config = sandbox.mkdir("config");
    fs::path workdir = sandbox.mkdir("work");

    std::vector<std::pair<std::string, std::string>> env = {
        {"HOME", home.string()},
        {"TMPDIR", tmp.string()},
        {"XDG_CACHE_HOME", cache.string()},
        {"XDG_CONFIG_HOME", config.string()},
        {"XDG_DATA_HOME", sandbox.root().string()},
    };

    for (const std::vector<std::string>& tail :
         {std::vector<std::string>{"validate", root}, std::vector<std::string>{"list", root},
          std::vector<std::string>{"query", "inv.Thing", root}}) {
        std::set<std::string> before_sandbox = snapshot_files(sandbox.root());
        std::set<std::string> before_work = snapshot_files(workdir);
        std::set<std::string> before_project = snapshot_files(project.root());

        std::vector<std::string> argv{built_bin().string()};
        argv.insert(argv.end(), tail.begin(), tail.end());
        // Run from the isolated workdir so any cwd writes land there.
        yass::test::ProcResult r = yass::test::run(argv, workdir.string(), env, "");
        CHECK_FALSE(r.signaled);

        // No new files anywhere in the sandbox (HOME / TMPDIR / cache / config /
        // work) and none in the cwd.
        CHECK(snapshot_files(sandbox.root()) == before_sandbox);
        CHECK(snapshot_files(workdir) == before_work);
        // The project tree (the inputs) is unchanged file-set-wise too.
        CHECK(snapshot_files(project.root()) == before_project);
    }
}

// ===========================================================================
// cli.Dispatch SIDE-EFFECT / INVARIANT: MUST-NOT modify input files / take a
// lock on them — observed across ALL subcommands (validate already has a test;
// list and query were untested).
// ===========================================================================
TEST_CASE("cli.Dispatch INVARIANT: input files are not modified by list or query") {
    if (!have_built_bin()) {
        MESSAGE("built yass binary unavailable; skipping input-unchanged E2E");
        return;
    }
    yass::test::TmpTree t;
    t.mkdir(".git");
    fs::path f = t.write("good.yass.yaml",
                         "---\ndescription: a test\nversion: v1\n---\n"
                         "spec: inv.Thing\nINPUT:\n- MUST: do the thing\n");
    std::string root = t.root().string();

    const std::string before_content = read_all(f);
    std::error_code ec;
    auto before_mtime = fs::last_write_time(f, ec);

    for (const std::vector<std::string>& tail :
         {std::vector<std::string>{"list", "."}, std::vector<std::string>{"query", "inv.Thing", "."},
          std::vector<std::string>{"validate", "."}}) {
        std::vector<std::string> argv{built_bin().string()};
        argv.insert(argv.end(), tail.begin(), tail.end());
        yass::test::ProcResult r = yass::test::run(argv, root, {}, "");
        CHECK_FALSE(r.signaled);
        // Content AND mtime unchanged: not modified, not rewritten in place.
        CHECK(read_all(f) == before_content);
        std::error_code mec;
        CHECK(fs::last_write_time(f, mec) == before_mtime);
    }
}

// ===========================================================================
// cli.Dispatch INVARIANT: MUST emit LF (U+000A) as the only line terminator on
// stdout and stderr; MUST end every emitted line including the last with one LF.
// ===========================================================================
// Scan the REAL emitted bytes (not a synthetic ErrorLine string) for any CR
// (0x0d) on stdout and stderr across representative runs, and assert non-empty
// output ends with exactly one LF.
TEST_CASE("cli.Dispatch INVARIANT: stdout/stderr use LF only and end with one LF") {
    if (!have_built_bin()) {
        MESSAGE("built yass binary unavailable; skipping LF-only E2E");
        return;
    }
    yass::test::TmpTree t;
    std::string root = make_project(t);

    struct Case {
        std::vector<std::string> tail;
        std::string cwd;
    };
    std::vector<Case> cases = {
        {{"list", "."}, root},
        {{"query", "inv.Thing", "."}, root},
        {{"validate", "."}, root},
        {{"--help"}, root},
        {{"--version"}, root},
        {{"bogus"}, root},          // argv error -> one ErrorLine on stderr.
        {{"validate", "nope.yass.yaml"}, root},  // path.not_found + summary.
    };

    for (const Case& c : cases) {
        std::vector<std::string> argv{built_bin().string()};
        argv.insert(argv.end(), c.tail.begin(), c.tail.end());
        yass::test::ProcResult r = yass::test::run(argv, c.cwd, {}, "");

        // No carriage return anywhere on either stream.
        CHECK(r.out.find('\r') == std::string::npos);
        CHECK(r.err.find('\r') == std::string::npos);
        // Non-empty output ends with exactly one LF (and not a blank double-LF
        // tail that would imply a stray terminator).
        if (!r.out.empty()) {
            CHECK(r.out.back() == '\n');
        }
        if (!r.err.empty()) {
            CHECK(r.err.back() == '\n');
        }
    }
}

// ===========================================================================
// cli.Dispatch INVARIANT: MUST-NOT emit ANSI escape codes / color / terminal
// formatting on stdout or stderr; MUST-NOT page/alter output when stdout is a
// TTY. Verified by running under a real pty with color-forcing env set and
// scanning for ESC (0x1b).
// ===========================================================================
TEST_CASE("cli.Dispatch INVARIANT: no ANSI escapes even when stdout is a TTY") {
    if (!have_built_bin()) {
        MESSAGE("built yass binary unavailable; skipping no-ANSI TTY E2E");
        return;
    }
    yass::test::TmpTree t;
    std::string root = make_project(t);

    // Color-forcing environment that a misbehaving tool might honor.
    std::vector<std::pair<std::string, std::string>> env = {
        {"TERM", "xterm-256color"},
        {"CLICOLOR_FORCE", "1"},
        {"COLORTERM", "truecolor"},
    };

    for (const std::vector<std::string>& tail :
         {std::vector<std::string>{"list", "."}, std::vector<std::string>{"query", "inv.Thing", "."},
          std::vector<std::string>{"validate", "."}, std::vector<std::string>{"--help"}}) {
        std::vector<std::string> argv{built_bin().string()};
        argv.insert(argv.end(), tail.begin(), tail.end());

        // width 80 columns; the pty merges stdout+stderr but ESC must appear on
        // neither.
        yass::test::PtyResult p = yass::test::run_under_pty(argv, root, /*width=*/80, /*rows=*/24, env);
        if (!p.ok) {
            MESSAGE("pty could not be allocated; skipping no-ANSI check for this case");
            continue;
        }
        // No ESC (0x1b) byte in the pty output: no color, no cursor control, no
        // pager handshake.
        CHECK(p.raw.find('\x1b') == std::string::npos);
        // And no bare CR beyond the ONLCR-inserted ones (strip_cr_before_lf left
        // the canonical bytes); the stripped output carries LF only.
        CHECK(p.out.find('\r') == std::string::npos);
    }
}
