/**
 * DiscoverSpecFiles — locate .yass.yaml specification files.
 *
 * Given an optional path (file or directory) and a project root, returns a
 * sorted list of discovered spec-file paths together with any non-fatal errors
 * encountered during recursive traversal.
 */

import { statSync, readdirSync } from "node:fs";
import { type Dirent } from "node:fs";
import { resolve, isAbsolute, join, basename } from "node:path";

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

export interface DiscoverResult {
  files: string[];
  errors: Array<{
    code: string;
    message: string;
    file: string;
  }>;
}

export interface DiscoverError {
  code: string;
  message: string;
  file?: string;
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const SPEC_SUFFIX = ".yass.yaml";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** True when `name` ends with `.yass.yaml` and has a non-empty prefix. */
function isSpecFile(name: string): boolean {
  if (!name.endsWith(SPEC_SUFFIX)) return false;
  const prefix = name.slice(0, -SPEC_SUFFIX.length);
  return prefix.length > 0;
}

/** True when `name` starts with ".". */
function isHidden(name: string): boolean {
  return name.startsWith(".");
}

/**
 * Format a path for output.
 *
 * - If the path starts with `cwd + "/"`, emit the relative portion
 *   (no leading "./").
 * - Otherwise emit the absolute path unchanged.
 *
 * Lexical comparison only — no realpath resolution.
 */
function formatPath(absPath: string, cwd: string): string {
  const prefix = cwd.endsWith("/") ? cwd : cwd + "/";
  if (absPath.startsWith(prefix)) {
    return absPath.slice(prefix.length);
  }
  return absPath;
}

/**
 * NFC-normalised Unicode code-point comparator.
 *
 * No locale-aware collation.  No case-folding.
 */
function codepointCompare(a: string, b: string): number {
  const na = a.normalize("NFC");
  const nb = b.normalize("NFC");
  if (na < nb) return -1;
  if (na > nb) return 1;
  return 0;
}

// ---------------------------------------------------------------------------
// Recursive directory walker
// ---------------------------------------------------------------------------

interface WalkAccumulator {
  files: string[];
  errors: Array<{ code: string; message: string; file: string }>;
}

/**
 * Recursively walk `dir`, collecting .yass.yaml files.
 *
 * - Does NOT follow symbolic links.
 * - Does NOT descend into hidden directories.
 * - Does NOT match hidden files.
 * - Records unreadable directories as non-fatal errors.
 */
function walk(dir: string, acc: WalkAccumulator): void {
  let entries: Dirent[];
  try {
    entries = readdirSync(dir, { withFileTypes: true }) as Dirent[];
  } catch {
    acc.errors.push({
      code: "yass.discover.dir_unreadable",
      message: `Cannot list directory: ${dir}`,
      file: dir,
    });
    return;
  }

  for (const entry of entries) {
    const name = entry.name;

    // Skip hidden entries entirely.
    if (isHidden(name)) continue;

    // Skip symbolic links encountered during traversal.
    if (entry.isSymbolicLink()) continue;

    const fullPath = join(dir, name);

    if (entry.isDirectory()) {
      walk(fullPath, acc);
    } else if (entry.isFile() && isSpecFile(name)) {
      acc.files.push(fullPath);
    }
  }
}

// ---------------------------------------------------------------------------
// Main entry point
// ---------------------------------------------------------------------------

export function discoverSpecFiles(
  path: string | undefined,
  projectRoot: string,
  cwd: string,
): DiscoverResult | DiscoverError {
  // Resolve the target path.  No tilde / env-var expansion.
  const target = path != null ? (isAbsolute(path) ? path : resolve(cwd, path)) : projectRoot;

  // ------------------------------------------------------------------
  // stat the target — lstat so we can detect symlinks
  // ------------------------------------------------------------------
  let stat: ReturnType<typeof statSync>;
  try {
    stat = statSync(target, { throwIfNoEntry: false })!;
    if (!stat) {
      return {
        code: "yass.path.not_found",
        message: `Path does not exist: ${target}`,
        file: target,
      };
    }
  } catch {
    return {
      code: "yass.path.unreadable",
      message: `Cannot access path: ${target}`,
      file: target,
    };
  }

  // ------------------------------------------------------------------
  // Classify path
  // ------------------------------------------------------------------

  // For symlinks provided as the direct argument, we need special handling:
  // use the symlink path for reporting but follow through for type detection.
  // For the top-level argument we *do* follow the symlink (via statSync which
  // follows by default).  Symlinks encountered during recursive traversal are
  // skipped (handled in walk()).

  if (stat.isFile()) {
    // ---- Single file path ----
    const name = basename(target);
    if (!name.endsWith(SPEC_SUFFIX) || name === SPEC_SUFFIX) {
      return {
        code: "yass.path.bad_extension",
        message: `File does not have ${SPEC_SUFFIX} suffix: ${target}`,
        file: target,
      };
    }
    // Spec says: symlink file arg -> use symlink path for reporting.
    // Since we resolved `target` from the caller-supplied path (without
    // realpath), we already have the symlink path, not the resolved target.
    return {
      files: [formatPath(target, cwd)],
      errors: [],
    };
  }

  if (stat.isDirectory()) {
    // ---- Directory path ----
    const acc: WalkAccumulator = { files: [], errors: [] };

    // Verify the directory itself is readable.
    try {
      readdirSync(target);
    } catch {
      return {
        code: "yass.path.unreadable",
        message: `Cannot read directory: ${target}`,
        file: target,
      };
    }

    walk(target, acc);

    // Format & sort paths.
    const formatted = acc.files.map((f) => formatPath(f, cwd));
    formatted.sort(codepointCompare);

    return {
      files: formatted,
      errors: acc.errors,
    };
  }

  // Neither regular file nor directory (e.g. block device, FIFO, …).
  return {
    code: "yass.path.unreadable",
    message: `Path is neither a regular file nor a directory: ${target}`,
    file: target,
  };
}
