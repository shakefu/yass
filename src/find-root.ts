import { existsSync, realpathSync } from "node:fs";
import { resolve, dirname, parse as parsePath } from "node:path";

export interface FindRootResult {
  root: string; // absolute path
}

export interface FindRootError {
  code: string; // "yass.findroot.no_marker"
  message: string;
}

/**
 * Search upward from `startDir` (default: cwd) for a project root marker.
 *
 * Strategy:
 *  1. Walk from startDir inclusive toward filesystem root looking for a `.git`
 *     entry.  If ANY ancestor contains `.git`, return the **deepest** such
 *     ancestor (i.e. the first one found walking upward) and stop.
 *  2. Only when the entire upward path contains NO `.git` at all, restart from
 *     startDir and walk upward looking for any `.yass.yaml` file, returning
 *     the deepest such ancestor.
 *  3. If neither marker is found, return an error.
 */
export function findProjectRoot(
  startDir?: string,
): FindRootResult | FindRootError {
  const start = resolve(startDir ?? process.cwd());

  // --- Phase 1: search for .git ---
  const gitRoot = searchUpward(start, (dir) =>
    existsSync(resolve(dir, ".git")),
  );
  if (gitRoot !== undefined) {
    return { root: gitRoot };
  }

  // --- Phase 2: fallback to .yass.yaml (only when NO .git anywhere) ---
  const yassRoot = searchUpward(start, (dir) =>
    existsSync(resolve(dir, ".yass.yaml")),
  );
  if (yassRoot !== undefined) {
    return { root: yassRoot };
  }

  // --- Phase 3: no marker found ---
  return {
    code: "yass.findroot.no_marker",
    message: `No project root marker (.git or .yass.yaml) found from ${start} to filesystem root`,
  };
}

/**
 * Walk from `start` toward the filesystem root (inclusive of `start`).
 * Return the first directory for which `predicate` returns true,
 * or `undefined` if the root is reached without a match.
 */
function searchUpward(
  start: string,
  predicate: (dir: string) => boolean,
): string | undefined {
  let current = start;
  for (;;) {
    if (predicate(current)) {
      return current;
    }
    const parent = dirname(current);
    if (parent === current) {
      // Reached filesystem root without a match.
      return undefined;
    }
    current = parent;
  }
}

/** Type-guard: narrow a result to FindRootError. */
export function isFindRootError(
  result: FindRootResult | FindRootError,
): result is FindRootError {
  return "code" in result;
}
