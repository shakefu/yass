#pragma once

// M6 — the `validate` subcommand.
//
// Spec basis (spec/cli.validate.yass.yaml :: cli.validate):
//   - INPUT       — positional path handling: glob expansion, file-vs-dir
//                   classification, dedup by lexically-normalized absolute path,
//                   `-` as a literal path, `:`-in-path rejection.
//   - RETURN      — per-file check pipeline (CheckYAML -> CheckPreamble ->
//                   CheckSpec -> CheckUniqueness -> CheckRefs), error ordering,
//                   exit-code rules.
//   - ERROR       — no_files / not_found / bad_extension / unreadable /
//                   glob-expanded-non-spec-skip.
//   - SIDE-EFFECT — stderr error lines + the stdout summary line written LAST.
//   - INVARIANT   — project root computed exactly once via cli.FindProjectRoot;
//                   only MUST/MUST-NOT obligations are enforced as errors.
//
// Related specs:
//   - spec/cli.yass.yaml      :: cli.ExitCode (0/1/2 here), cli.ErrorLine.
//   - spec/cli.shared.yass.yaml :: cli.FindProjectRoot / cli.DiscoverSpecFiles /
//                                   cli.ExpandGlob (via src/fs.hpp).
//
// Conformance (decided by the tech lead): stdout bytes AND the process exit code
// match the reference `yass validate` byte-for-byte; stderr follows the SPEC
// (cli.errors message PROSE via diag::canonical_message, cli.ErrorLine format,
// spec ordering), with error CODE / LINE / EXIT still matching the reference.
//
// run_validate performs all stdout/stderr I/O through the supplied streams and
// returns the process exit code; it never calls std::exit, so it is directly
// unit- and differential-testable in-process. It uses the PROCESS current
// working directory (std::filesystem::current_path()) for every relative-path
// resolution, matching the reference's process-cwd model.

#include <iosfwd>
#include <string>
#include <vector>

namespace yass::validate {

// Run the `validate` subcommand. `args` are the tokens AFTER the `validate`
// subcommand (i.e. the positional path arguments; flags are handled by the
// dispatcher in a later module). Validation error lines are written to `err`
// (each = diag::format_error_line(...) + '\n'); the summary line
// `checked <N> files, found <M> errors\n` is written to `out` as the final
// bytes on any stream. Returns the process exit code: 0 on success, 1 when any
// validation error was found, 2 on an argv / file-input failure.
int run_validate(const std::vector<std::string>& args, std::ostream& out,
                 std::ostream& err);

}  // namespace yass::validate
