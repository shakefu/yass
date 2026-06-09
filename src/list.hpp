#pragma once

// M8 — the `list` subcommand.
//
// Spec basis (spec/cli.list.yass.yaml :: cli.list):
//   - INPUT       — optional positional file/dir path routed through
//                   cli.DiscoverSpecFiles for existence / extension / fs-error
//                   checks; a `:` in any positional rejects with
//                   yass.path.colon_in_path (exit 2); `-` is a literal path (the
//                   dispatcher intercepts the stdin marker earlier).
//   - RETURN      — one `file<TAB>name<TAB>description` row per Spec document,
//                   files sorted by NFC code-point order, specs in document
//                   order; description normalization (whitespace collapse, NFC);
//                   tab-in-path-field replaced by a space; optional grapheme-
//                   cluster truncation with an ASCII `...` marker when stdout is
//                   a terminal; exit 0/1 rules.
//   - ERROR       — not_found / bad_extension (exit 2) surfaced by discovery; a
//                   discovered file that fails YAML parse emits one ErrorLine
//                   yass.yaml.malformed and continues (exit 1 overall).
//   - SIDE-EFFECT — rows to stdout, error lines to stderr, no file modified.
//   - INVARIANT   — never parses / validates obligation content; never resolves
//                   refs.
//
// Related specs: spec/cli.yass.yaml :: cli.ErrorLine / cli.ExitCode,
// spec/cli.shared.yass.yaml :: cli.FindProjectRoot / cli.DiscoverSpecFiles (via
// src/fs.hpp).
//
// Conformance (tech-lead policy): stdout bytes AND the process exit code match
// the reference `yass list` byte-for-byte; stderr follows the SPEC (cli.errors
// canonical message via diag::canonical_message, cli.ErrorLine format), with the
// error CODE / LINE / EXIT still matching the reference.
//
// run_list performs all stdout/stderr I/O through the supplied streams and
// returns the process exit code; it never calls std::exit, so it is directly
// unit- and differential-testable in-process. It uses the PROCESS current
// working directory for every relative-path resolution, matching the reference's
// process-cwd model.

#include <iosfwd>
#include <string>
#include <vector>

namespace yass::list {

// Run the `list` subcommand. `args` are the tokens AFTER the `list` subcommand:
// at most one positional file-or-directory path (args[0]); none means discover
// from the project root.
//
// `tty_width` carries the terminal-width decision the dispatcher already made:
//   - tty_width == 0  -> stdout is NOT a terminal: emit full, untruncated rows.
//   - tty_width  > 0  -> stdout IS a terminal of that many columns: truncate the
//                        description on a grapheme-cluster boundary so each line
//                        fits the width (cli.list RETURN, the WHEN-stdout-is-a-
//                        terminal obligations).
// Keeping the width a parameter (rather than probing isatty/COLUMNS here) leaves
// run_list ostream-based and fully testable.
//
// Returns the process exit code: 0 when every discovered file parsed (even with
// zero rows), 1 when at least one discovered file failed to parse as YAML, 2 on
// an argv / file-input failure (colon-in-path, not_found, bad_extension, ...).
int run_list(const std::vector<std::string>& args, std::ostream& out, std::ostream& err,
             int tty_width);

}  // namespace yass::list
