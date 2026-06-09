#pragma once

// Differential test harness for the validate (and later) subcommands.
//
// Conformance policy (tech lead): our stdout bytes AND process exit code MUST
// match the reference `yass` byte-for-byte; stderr follows the SPEC and is
// asserted separately (codes / lines / message PROSE). These helpers run the
// reference binary as a subprocess and an in-process runner under the SAME
// process cwd, then expose both results so a test can CHECK stdout+exit against
// the reference and assert stderr structure against the spec.
//
// doctest is single-threaded, so the RAII cwd guard (chdir on construct, restore
// on destruct) is safe: only one test touches the process cwd at a time.

#include "doctest.h"

#include <unistd.h>

#include <filesystem>
#include <functional>
#include <optional>
#include <sstream>
#include <string>
#include <vector>

#include "support/proc.hpp"

namespace yass::test {

// --------------------------------------------------------------------------
// RAII current-working-directory guard.
// --------------------------------------------------------------------------
// chdir() into `dir` on construction; restore the previous cwd on destruction.
// Single-threaded doctest makes this safe. Uses ::chdir directly so the change
// affects std::filesystem::current_path() (the process-cwd model the code under
// test relies on).
class CwdGuard {
   public:
    explicit CwdGuard(const std::filesystem::path& dir) {
        std::error_code ec;
        prev_ = std::filesystem::current_path(ec);
        REQUIRE(::chdir(dir.string().c_str()) == 0);
    }
    ~CwdGuard() {
        if (!prev_.empty()) {
            (void)::chdir(prev_.string().c_str());
        }
    }
    CwdGuard(const CwdGuard&) = delete;
    CwdGuard& operator=(const CwdGuard&) = delete;

   private:
    std::filesystem::path prev_;
};

// --------------------------------------------------------------------------
// Reference-binary discovery.
// --------------------------------------------------------------------------
// YASS_REF_BIN is resolved ONCE at CMake configure time to an ABSOLUTE path
// (see CMakeLists.txt: YASS_REF env override, else /tmp/yass if it runs, else
// find_program(yass) on PATH) and baked in as a compile definition. We do NOT
// search PATH or probe hardcoded fallbacks at runtime here: the oracle is
// pinned for reproducibility. Returns the baked path if it is still executable,
// otherwise empty — in which case differential checks SKIP gracefully (the
// in-process SPEC assertions still run; the oracle is a confirmation, not a
// dependency).
inline std::string find_ref_bin() {
    namespace fs = std::filesystem;
    std::string configured = YASS_REF_BIN;
    if (configured.empty()) return {};
    std::error_code ec;
    fs::path p(configured);
    if (fs::is_regular_file(p, ec) && ::access(p.c_str(), X_OK) == 0) {
        return p.string();
    }
    return {};
}

// Signature of an in-process subcommand runner: write to `out`/`err`, return the
// process exit code. (e.g. a lambda wrapping run_validate(args, out, err).)
using InProcRunner = std::function<int(std::ostream&, std::ostream&)>;

// The paired outcome of a differential run.
struct DiffOutcome {
    bool ref_available = false;
    // Reference subprocess result (valid when ref_available).
    int ref_exit = 0;
    std::string ref_out;
    std::string ref_err;
    // In-process runner result (always populated).
    int our_exit = 0;
    std::string our_out;
    std::string our_err;
};

// Run the reference `yass <ref_args...>` as a subprocess in the current process
// cwd, and the in-process `runner` under that same cwd, capturing both stdout,
// stderr, and exit code. Does NOT assert anything; the caller decides which
// fields to compare. When the reference binary is unavailable, only the
// in-process fields are populated and ref_available is false.
inline DiffOutcome diff_run(const std::vector<std::string>& ref_args,
                            const InProcRunner& runner) {
    DiffOutcome o;

    std::ostringstream out;
    std::ostringstream err;
    o.our_exit = runner(out, err);
    o.our_out = out.str();
    o.our_err = err.str();

    std::string ref = find_ref_bin();
    if (!ref.empty()) {
        std::vector<std::string> argv;
        argv.reserve(ref_args.size() + 1);
        argv.push_back(ref);
        for (const auto& a : ref_args) argv.push_back(a);
        // cwd = "" inherits the parent (test process) cwd, which the test set via
        // CwdGuard — exactly the process-cwd model the in-process runner uses.
        ProcResult r = run(argv, "");
        o.ref_available = true;
        o.ref_exit = r.exit_code;
        o.ref_out = r.out;
        o.ref_err = r.err;
    }
    return o;
}

// Assert our stdout == reference stdout (byte-for-byte) and our exit == reference
// exit. `ref_args` are the args passed to the reference (e.g. {"validate","."}).
// When the reference is unavailable the check is skipped and the outcome is
// returned for any further SPEC-based stderr assertions the caller wants.
inline DiffOutcome expect_stdout_exit_match(const std::vector<std::string>& ref_args,
                                            const InProcRunner& runner) {
    DiffOutcome o = diff_run(ref_args, runner);
    if (o.ref_available) {
        CHECK(o.our_out == o.ref_out);
        CHECK(o.our_exit == o.ref_exit);
    }
    return o;
}

}  // namespace yass::test
