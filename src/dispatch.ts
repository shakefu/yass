/**
 * CLI dispatch — parse top-level argv, route to subcommands.
 *
 * This module is the testable core of the CLI entry point. It accepts
 * injected I/O streams so tests can capture output without touching
 * process.stdout / process.stderr.
 */

import { ErrorCode, exitCodeFor, messageFor } from "./errors.ts";
import { formatErrorLine } from "./error-line.ts";
import { validateCommand } from "./validate.ts";
import { listCommand } from "./list.ts";
import { queryCommand } from "./query.ts";

// ---------------------------------------------------------------------------
// Public interface
// ---------------------------------------------------------------------------

export interface DispatchOptions {
  argv: string[];
  cwd: string;
  stdout: { write(s: string): void };
  stderr: { write(s: string): void };
  isTTY: boolean;
  version: string;
}

// ---------------------------------------------------------------------------
// Usage text
// ---------------------------------------------------------------------------

const USAGE = `Usage: yass <command> [options]

Commands:
  validate [paths...]   Validate .yass.yaml files
  query <name> [scope]  Query a spec by name
  list [path]           List discovered specs

Options:
  --help                Show this help message
  --version             Show version information
`;

// ---------------------------------------------------------------------------
// Known subcommands
// ---------------------------------------------------------------------------

const SUBCOMMANDS = ["validate", "query", "list"] as const;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Emit a single error line to stderr and return exit code 2.
 */
function usageError(
  stderr: { write(s: string): void },
  code: string,
  args: Record<string, string> = {},
): number {
  const msg = messageFor(code, args);
  const line = formatErrorLine({ code, message: msg });
  stderr.write(line + "\n");
  stderr.write(USAGE);
  return exitCodeFor(code);
}

/**
 * Check whether a token is an abbreviation of any known subcommand.
 * Returns true when the token is a proper prefix (length >= 1, length < full)
 * of at least one subcommand, and is NOT an exact match.
 */
function isAbbreviation(token: string): boolean {
  const lower = token.toLowerCase();
  for (const cmd of SUBCOMMANDS) {
    if (lower.length >= 1 && lower.length < cmd.length && cmd.startsWith(lower)) {
      return true;
    }
  }
  return false;
}

/**
 * Check whether a token is a case-mismatched form of a known subcommand.
 * Returns true when the lowered token exactly matches a subcommand but the
 * original token does not.
 */
function isCaseMismatch(token: string): boolean {
  const lower = token.toLowerCase();
  for (const cmd of SUBCOMMANDS) {
    if (lower === cmd && token !== cmd) {
      return true;
    }
  }
  return false;
}

/**
 * Known long flags in their canonical lowercase form.
 */
const KNOWN_FLAGS = ["--help", "--version"] as const;

/**
 * Check whether a long flag is a case-mismatched form of a known flag.
 * Returns true when the lowered flag exactly matches a known flag but the
 * original flag does not (e.g. "--Help", "--VERSION").
 */
function isFlagCaseMismatch(flag: string): boolean {
  const lower = flag.toLowerCase();
  for (const known of KNOWN_FLAGS) {
    if (lower === known && flag !== known) {
      return true;
    }
  }
  return false;
}

// ---------------------------------------------------------------------------
// Main dispatch
// ---------------------------------------------------------------------------

/**
 * Parse top-level argv, validate flags and subcommand, then route to the
 * appropriate handler. Returns a numeric exit code.
 */
export function dispatch(options: DispatchOptions): number {
  const { argv, cwd, stdout, stderr, isTTY, version } = options;

  // --- Scan for --help and --version anywhere in argv ---
  // --help takes priority over everything.
  // --version takes priority over errors but not --help.
  let hasHelp = false;
  let hasVersion = false;

  for (const arg of argv) {
    if (arg === "--help") hasHelp = true;
    if (arg === "--version") hasVersion = true;
  }

  if (hasHelp) {
    stdout.write(USAGE);
    return 0;
  }

  if (hasVersion) {
    stdout.write(`yass ${version}\n`);
    return 0;
  }

  // --- Walk argv to find the subcommand and validate flags ---
  // Flags (tokens starting with "-") are checked everywhere before "--".
  // The first non-flag, non-"--" token is the subcommand candidate.
  // Subsequent non-flag tokens are positional args for the subcommand.
  let subcommand: string | undefined;
  let doubleDashIndex = -1;

  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i]!;

    // End-of-options marker: stop flag checking.
    if (arg === "--") {
      doubleDashIndex = i;
      break;
    }

    // Empty argument
    if (arg === "") {
      return usageError(stderr, ErrorCode.ARGV_EMPTY_ARGUMENT);
    }

    // Bare "-" (stdin dash)
    if (arg === "-") {
      return usageError(stderr, ErrorCode.ARGV_STDIN_DASH);
    }

    // Long flag (not --help / --version, which were handled above)
    if (arg.startsWith("--")) {
      if (isFlagCaseMismatch(arg)) {
        return usageError(stderr, ErrorCode.ARGV_CASE_MISMATCH, { token: arg });
      }
      return usageError(stderr, ErrorCode.ARGV_UNKNOWN_FLAG, { flag: arg });
    }

    // Short flag
    if (arg.startsWith("-")) {
      return usageError(stderr, ErrorCode.ARGV_SHORT_FLAG, { flag: arg });
    }

    // Non-flag token: first one is the subcommand, rest are positional args.
    if (subcommand !== undefined) {
      // Positional arg for the subcommand — skip dispatch-level checks.
      continue;
    }

    // First non-flag token is the subcommand candidate.
    if ((SUBCOMMANDS as readonly string[]).includes(arg)) {
      subcommand = arg;
    } else if (isCaseMismatch(arg)) {
      return usageError(stderr, ErrorCode.ARGV_CASE_MISMATCH, { token: arg });
    } else if (isAbbreviation(arg)) {
      return usageError(stderr, ErrorCode.ARGV_ABBREVIATION, { token: arg });
    } else {
      return usageError(stderr, ErrorCode.ARGV_UNKNOWN_SUBCOMMAND, {
        arg,
      });
    }
  }

  // No subcommand found
  if (subcommand === undefined) {
    return usageError(stderr, ErrorCode.ARGV_NO_SUBCOMMAND);
  }

  // --- Build subcommand args ---
  // Collect positional args that follow the subcommand. The "--" token
  // (if present) is stripped; tokens after it are included verbatim.
  const subIdx = argv.indexOf(subcommand);
  const rawArgs = argv.slice(subIdx + 1);
  const subArgs = rawArgs.filter((a) => a !== "--");

  // --- Dispatch to subcommand ---
  switch (subcommand) {
    case "validate":
      return validateCommand(subArgs, cwd);

    case "list":
      return listCommand(subArgs, cwd, stdout, stderr, isTTY);

    case "query":
      return queryCommand(subArgs, cwd, stdout, stderr, isTTY);

    default:
      // Unreachable
      return usageError(stderr, ErrorCode.ARGV_UNKNOWN_SUBCOMMAND, {
        arg: subcommand,
      });
  }
}
