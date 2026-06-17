#!/usr/bin/env bun
/**
 * yass CLI entry point — thin wrapper around dispatch.
 *
 * Reads the VERSION file, sets up signal handlers, calls dispatch(),
 * and exits with the returned code.
 */

import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { dispatch } from "./dispatch.ts";
import { ErrorCode, exitCodeFor, messageFor } from "./errors.ts";
import { formatErrorLine } from "./error-line.ts";

// ---------------------------------------------------------------------------
// Version
// ---------------------------------------------------------------------------

const VERSION_PATH = resolve(import.meta.dir, "..", "VERSION");

function readVersion(): string {
  try {
    return readFileSync(VERSION_PATH, "utf-8").trim();
  } catch {
    return "0.0.0";
  }
}

// ---------------------------------------------------------------------------
// Signal handlers
// ---------------------------------------------------------------------------

// SIGPIPE: exit cleanly (exit 0) when pipe reader closes.
process.on("SIGPIPE", () => {
  process.exit(0);
});

// SIGINT: exit 130 after flushing.
process.on("SIGINT", () => {
  process.exit(exitCodeFor(ErrorCode.EXIT_SIGINT));
});

// SIGTERM: exit 143 after flushing.
process.on("SIGTERM", () => {
  process.exit(exitCodeFor(ErrorCode.EXIT_SIGTERM));
});

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

try {
  const version = readVersion();
  const code = dispatch({
    argv: process.argv.slice(2),
    cwd: process.cwd(),
    stdout: process.stdout,
    stderr: process.stderr,
    isTTY: process.stdout.isTTY ?? false,
    version,
  });
  process.exit(code);
} catch (err: unknown) {
  // Uncaught internal error
  const message =
    err instanceof Error ? err.message : String(err);
  const line = formatErrorLine({
    code: ErrorCode.INTERNAL_UNCAUGHT,
    message: messageFor(ErrorCode.INTERNAL_UNCAUGHT, { message }),
  });
  process.stderr.write(line + "\n");
  process.exit(exitCodeFor(ErrorCode.INTERNAL_UNCAUGHT));
}
