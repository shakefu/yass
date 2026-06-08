/**
 * ExpandGlob — expand a single argument string using doublestar glob semantics.
 *
 * Metacharacters recognised: * ? [
 * Brace expansion is explicitly disabled.
 * Hidden files/directories (names starting with ".") are never matched.
 * Symbolic links are never followed (excluded from results entirely).
 * Results are sorted by Unicode code-point order on NFC-normalised UTF-8 paths.
 */

import { lstatSync } from "node:fs";
import { join } from "node:path";

// ── public types ────────────────────────────────────────────────────────────

export interface GlobResult {
  paths: string[];
}

export interface GlobError {
  code: string;
  message: string;
}

// ── helpers ─────────────────────────────────────────────────────────────────

/** The set of characters that make a string a glob pattern. */
const GLOB_META = /[*?[]/;

/**
 * Return `true` when `arg` contains at least one unescaped glob
 * metacharacter (`*`, `?`, or `[`).
 */
export function isGlobPattern(arg: string): boolean {
  return GLOB_META.test(arg);
}

/**
 * Escape brace characters so Bun.Glob treats them literally.
 * The spec forbids brace expansion ({a,b}).
 */
function escapeBraces(pattern: string): string {
  return pattern.replace(/([{}])/g, "\\$1");
}

// ── main entry point ────────────────────────────────────────────────────────

/**
 * Expand a single argument string.
 *
 * - When the argument contains no glob metacharacters the literal path is
 *   returned unchanged (no filesystem check).
 * - Otherwise the pattern is expanded with Bun.Glob using doublestar
 *   semantics.  Results are sorted by Unicode code-point order on
 *   NFC-normalised paths.
 *
 * @param pattern  The raw argument string (no prior shell expansion).
 * @param cwd      Working directory for relative patterns (defaults to
 *                 `process.cwd()`).
 */
export function expandGlob(
  pattern: string,
  cwd?: string,
): GlobResult | GlobError {
  // Literal path — no metacharacters, return as-is.
  if (!isGlobPattern(pattern)) {
    return { paths: [pattern] };
  }

  // Escape braces so Bun.Glob does not perform brace expansion.
  const safePattern = escapeBraces(pattern);

  const glob = new Bun.Glob(safePattern);

  const resolvedCwd = cwd ?? process.cwd();
  const paths: string[] = [];

  // Bun.Glob.scanSync returns an iterator of matched paths.
  // With followSymlinks: false Bun still *returns* symlink entries (it just
  // doesn't traverse symlinked directories).  The spec says "Do NOT follow
  // symbolic links", so we exclude symlink entries entirely via lstat.
  for (const entry of glob.scanSync({
    cwd: resolvedCwd,
    dot: false,
    followSymlinks: false,
    absolute: false,
    onlyFiles: false,
  })) {
    try {
      const stat = lstatSync(join(resolvedCwd, entry));
      if (stat.isSymbolicLink()) continue;
    } catch {
      // If we can't stat it, skip it.
      continue;
    }
    paths.push(entry);
  }

  if (paths.length === 0) {
    return {
      code: "yass.glob.no_match",
      message: `no files matched pattern: ${pattern}`,
    };
  }

  // Sort by Unicode code-point order on NFC-normalised paths.
  paths.sort((a, b) => {
    const na = a.normalize("NFC");
    const nb = b.normalize("NFC");
    return na < nb ? -1 : na > nb ? 1 : 0;
  });

  return { paths };
}

// ── type guards ─────────────────────────────────────────────────────────────

export function isGlobError(result: GlobResult | GlobError): result is GlobError {
  return "code" in result;
}

export function isGlobResult(result: GlobResult | GlobError): result is GlobResult {
  return "paths" in result;
}
