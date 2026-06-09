#pragma once

// M9 — top-level dispatch.
//
// Spec basis (spec/cli.yass.yaml):
//   - cli.Dispatch  — global flag recognition (--help/--version anywhere),
//                     subcommand selection (validate/query/list), the argv
//                     error table, and the `--` end-of-options marker.
//   - cli.ExitCode  — only 0/1/2/130/143 may be returned.
//   - cli.ErrorLine — every stderr line is diag::format_error_line(...) + '\n'.
//   - cli.errors    — the yass.argv.* / yass.internal.* codes and messages.
//
// Conformance (tech-lead policy): stdout bytes AND the process exit code match
// the reference `yass` byte-for-byte. For argv errors the reference also emits a
// single ErrorLine to stderr and NOTHING else (no usage block), so dispatch
// matches that: the cli.Dispatch ERROR obligations say "print usage to stderr"
// but the reference oracle emits only the ErrorLine — see spec_issues in the M9
// report. stderr otherwise follows the SPEC (canonical_message via diag, the
// cli.ErrorLine format), with the error CODE / exit still matching the oracle.
//
// run_dispatch performs all I/O through the supplied streams and returns the
// process exit code; it never calls std::exit, so it is directly unit- and
// differential-testable in-process. The `--help` and `--version` global flags,
// the argv error classification, and `--` handling are all resolved here before
// any subcommand handler is invoked. Uncaught exceptions thrown by a handler are
// converted to one yass.internal.uncaught ErrorLine and exit 1.

#include <iosfwd>
#include <string>
#include <vector>

namespace yass::dispatch {

// The exact top-level usage text the reference emits for `yass --help`, WITHOUT
// a trailing newline beyond the final one already present. Reproduced
// byte-for-byte from the reference binary so the differential happy-path holds.
extern const char* const kUsageText;

// Run the top-level CLI. `argv_tail` is everything after argv[0] (the program
// name): the global flags, the subcommand, and the subcommand's own arguments.
//
// `list_tty_width` is forwarded to run_list (0 = stdout is not a terminal; >0 =
// terminal column count); main computes it. It is ignored for every other path.
//
// Returns the process exit code (0/1/2; signals are handled in main, not here):
//   - --help anywhere   -> print kUsageText to `out`, return 0.
//   - --version anywhere-> print "yass <version>\n" to `out`, return 0.
//   - a recognized subcommand -> dispatch to run_validate / run_query / run_list
//     with the post-subcommand args (the single global `--` marker removed),
//     returning that handler's exit code; an uncaught exception becomes one
//     yass.internal.uncaught ErrorLine on `err` and return 1.
//   - any argv error -> one ErrorLine on `err`, return 2.
int run_dispatch(const std::vector<std::string>& argv_tail, std::ostream& out, std::ostream& err,
                 int list_tty_width);

// Map a received signal number to the cli.ExitCode the process exits with:
//   SIGINT  -> 130 (cli.ExitCode ON_SIGINT)
//   SIGTERM -> 143 (cli.ExitCode ON_SIGTERM)
//   anything else -> 0
// Defined here (not in main.cpp's anonymous namespace) so the 130/143 mapping
// can be asserted directly without delivering a real signal. main installs the
// SIGINT/SIGTERM handlers around this; SIGPIPE exits 0 separately.
int signal_exit_code(int sig);

}  // namespace yass::dispatch
