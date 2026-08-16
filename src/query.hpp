#pragma once

// M7 — the `query` subcommand.
//
// Spec basis (spec/cli.query.yass.yaml):
//   - cli.query                 — argument handling, scope validation order,
//                                 single/multi/zero-match dispatch, exit codes.
//   - cli.query.NameLookup      — case-sensitive full-name / dot-aligned
//                                 trailing-suffix matching; blank / whitespace
//                                 rules; scope-narrowed search.
//   - cli.query.ExtractFragment — re-parse the matched file and lift the single
//                                 matched spec document.
//   - cli.query.InlineConforms  — one-level CONFORMS inlining with provenance.
//   - cli.query.OutputProfile   — the byte-exact YAML emitter (src/emit.hpp).
//
// Related specs: spec/cli.list.yass.yaml (the disambiguation row format),
// spec/cli.yass.yaml :: cli.ErrorLine / cli.ExitCode, spec/cli.shared.yass.yaml
// :: cli.FindProjectRoot / cli.DiscoverSpecFiles (via src/fs.hpp).
//
// Conformance (tech-lead policy): stdout bytes AND the process exit code match
// the reference `yass query` byte-for-byte; stderr follows the SPEC (cli.errors
// canonical message via diag::canonical_message, cli.ErrorLine format), with the
// error CODE / LINE / EXIT still matching the reference.
//
// run_query performs all stdout/stderr I/O through the supplied streams and
// returns the process exit code; it never calls std::exit, so it is directly
// unit- and differential-testable in-process. It uses the PROCESS current
// working directory for every relative-path resolution, matching the reference's
// process-cwd model.

#include <iosfwd>
#include <string>
#include <vector>

namespace yass::query {

// Run the `query` subcommand. `args` are the tokens AFTER the `query`
// subcommand: args[0] is the required spec name, args[1] is an optional file or
// directory scope. On exactly one match the matched spec is emitted as a YAML
// fragment to `out` (CONFORMS inlined). On more than one match, cli.list-format
// disambiguation rows are written to `out`. On zero matches one ErrorLine is
// written to `err`. Returns the process exit code: 0 on a match (single or
// multi), 1 on no-match or a CONFORMS inlining failure, 2 on an argv / scope
// failure.
int run_query(const std::vector<std::string>& args, std::ostream& out, std::ostream& err);

}  // namespace yass::query
