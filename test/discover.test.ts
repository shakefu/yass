import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import { mkdirSync, writeFileSync, symlinkSync, chmodSync, rmSync } from "node:fs";
import { join, resolve } from "node:path";
import { tmpdir } from "node:os";
import { discoverSpecFiles } from "../src/discover.ts";
import type { DiscoverResult, DiscoverError } from "../src/discover.ts";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function isResult(v: DiscoverResult | DiscoverError): v is DiscoverResult {
  return "files" in v;
}

function isError(v: DiscoverResult | DiscoverError): v is DiscoverError {
  return "code" in v && !("files" in v);
}

/** Create a unique temp directory for a single test. */
function makeTmpDir(): string {
  const dir = join(tmpdir(), `yass-test-${Date.now()}-${Math.random().toString(36).slice(2)}`);
  mkdirSync(dir, { recursive: true });
  return dir;
}

/** Convenience: write an empty file, creating parent dirs as needed. */
function touch(path: string): void {
  const parent = resolve(path, "..");
  mkdirSync(parent, { recursive: true });
  writeFileSync(path, "");
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("discoverSpecFiles", () => {
  let tmp: string;

  beforeEach(() => {
    tmp = makeTmpDir();
  });

  afterEach(() => {
    rmSync(tmp, { recursive: true, force: true });
  });

  // -----------------------------------------------------------------------
  // Single file input
  // -----------------------------------------------------------------------

  test("returns single file when given a file path", () => {
    const file = join(tmp, "spec.yass.yaml");
    touch(file);

    const result = discoverSpecFiles(file, tmp, tmp);
    expect(isResult(result)).toBe(true);
    if (!isResult(result)) return;
    expect(result.files).toEqual(["spec.yass.yaml"]);
    expect(result.errors).toEqual([]);
  });

  // -----------------------------------------------------------------------
  // Recursive directory discovery
  // -----------------------------------------------------------------------

  test("recursively discovers .yass.yaml files in directory", () => {
    touch(join(tmp, "a.yass.yaml"));
    touch(join(tmp, "sub", "b.yass.yaml"));
    touch(join(tmp, "sub", "deep", "c.yass.yaml"));

    const result = discoverSpecFiles(tmp, tmp, tmp);
    expect(isResult(result)).toBe(true);
    if (!isResult(result)) return;
    expect(result.files).toEqual([
      "a.yass.yaml",
      join("sub", "b.yass.yaml"),
      join("sub", "deep", "c.yass.yaml"),
    ]);
  });

  // -----------------------------------------------------------------------
  // Hidden directory skipping
  // -----------------------------------------------------------------------

  test("skips hidden directories (names starting with .)", () => {
    touch(join(tmp, ".hidden", "spec.yass.yaml"));
    touch(join(tmp, "visible", "spec.yass.yaml"));

    const result = discoverSpecFiles(tmp, tmp, tmp);
    expect(isResult(result)).toBe(true);
    if (!isResult(result)) return;
    expect(result.files).toEqual([join("visible", "spec.yass.yaml")]);
  });

  // -----------------------------------------------------------------------
  // Hidden file skipping
  // -----------------------------------------------------------------------

  test("skips hidden files", () => {
    touch(join(tmp, ".hidden.yass.yaml"));
    touch(join(tmp, "visible.yass.yaml"));

    const result = discoverSpecFiles(tmp, tmp, tmp);
    expect(isResult(result)).toBe(true);
    if (!isResult(result)) return;
    expect(result.files).toEqual(["visible.yass.yaml"]);
  });

  // -----------------------------------------------------------------------
  // Bare .yass.yaml
  // -----------------------------------------------------------------------

  test("skips .yass.yaml (bare name without prefix)", () => {
    // The file IS hidden (starts with "."), so the walk skips it.
    // Additionally, even if it were not hidden, the name ".yass.yaml" has
    // an empty prefix before the suffix, which must not match.
    touch(join(tmp, ".yass.yaml"));
    touch(join(tmp, "real.yass.yaml"));

    const result = discoverSpecFiles(tmp, tmp, tmp);
    expect(isResult(result)).toBe(true);
    if (!isResult(result)) return;
    expect(result.files).toEqual(["real.yass.yaml"]);
  });

  // -----------------------------------------------------------------------
  // .yaml without .yass prefix
  // -----------------------------------------------------------------------

  test("skips .yaml files without .yass prefix", () => {
    touch(join(tmp, "plain.yaml"));
    touch(join(tmp, "spec.yass.yaml"));

    const result = discoverSpecFiles(tmp, tmp, tmp);
    expect(isResult(result)).toBe(true);
    if (!isResult(result)) return;
    expect(result.files).toEqual(["spec.yass.yaml"]);
  });

  // -----------------------------------------------------------------------
  // Unicode code-point sort order
  // -----------------------------------------------------------------------

  test("sorts by Unicode code-point order", () => {
    // 'Z' (U+005A) < 'a' (U+0061) in code-point order.
    touch(join(tmp, "a.yass.yaml"));
    touch(join(tmp, "Z.yass.yaml"));
    touch(join(tmp, "b.yass.yaml"));

    const result = discoverSpecFiles(tmp, tmp, tmp);
    expect(isResult(result)).toBe(true);
    if (!isResult(result)) return;
    expect(result.files).toEqual(["Z.yass.yaml", "a.yass.yaml", "b.yass.yaml"]);
  });

  // -----------------------------------------------------------------------
  // Relative path emission
  // -----------------------------------------------------------------------

  test("returns relative paths when under cwd", () => {
    touch(join(tmp, "sub", "spec.yass.yaml"));

    const result = discoverSpecFiles(tmp, tmp, tmp);
    expect(isResult(result)).toBe(true);
    if (!isResult(result)) return;
    expect(result.files).toEqual([join("sub", "spec.yass.yaml")]);
    // Must NOT start with "./"
    for (const f of result.files) {
      expect(f.startsWith("./")).toBe(false);
    }
  });

  test("returns basename alone when file is directly inside cwd", () => {
    const file = join(tmp, "spec.yass.yaml");
    touch(file);

    const result = discoverSpecFiles(file, tmp, tmp);
    expect(isResult(result)).toBe(true);
    if (!isResult(result)) return;
    expect(result.files).toEqual(["spec.yass.yaml"]);
  });

  // -----------------------------------------------------------------------
  // Absolute path emission
  // -----------------------------------------------------------------------

  test("returns absolute paths when not under cwd", () => {
    const otherDir = makeTmpDir();
    try {
      touch(join(otherDir, "spec.yass.yaml"));

      // cwd is tmp but file is in otherDir
      const result = discoverSpecFiles(otherDir, otherDir, tmp);
      expect(isResult(result)).toBe(true);
      if (!isResult(result)) return;
      expect(result.files.length).toBe(1);
      expect(result.files[0]!.startsWith("/")).toBe(true);
    } finally {
      rmSync(otherDir, { recursive: true, force: true });
    }
  });

  // -----------------------------------------------------------------------
  // Error: non-existent path
  // -----------------------------------------------------------------------

  test("error on non-existent path", () => {
    const result = discoverSpecFiles(join(tmp, "nope"), tmp, tmp);
    expect(isError(result)).toBe(true);
    if (!isError(result)) return;
    expect(result.code).toBe("yass.path.not_found");
  });

  // -----------------------------------------------------------------------
  // Error: bad extension
  // -----------------------------------------------------------------------

  test("error on bad extension", () => {
    const file = join(tmp, "spec.yaml");
    touch(file);

    const result = discoverSpecFiles(file, tmp, tmp);
    expect(isError(result)).toBe(true);
    if (!isError(result)) return;
    expect(result.code).toBe("yass.path.bad_extension");
  });

  test("error on bare .yass.yaml as direct file argument", () => {
    const file = join(tmp, ".yass.yaml");
    touch(file);

    const result = discoverSpecFiles(file, tmp, tmp);
    expect(isError(result)).toBe(true);
    if (!isError(result)) return;
    expect(result.code).toBe("yass.path.bad_extension");
  });

  // -----------------------------------------------------------------------
  // Unreadable directories
  // -----------------------------------------------------------------------

  test("handles unreadable directories gracefully (continues)", () => {
    touch(join(tmp, "good.yass.yaml"));
    const badDir = join(tmp, "noaccess");
    mkdirSync(badDir, { recursive: true });
    touch(join(badDir, "hidden.yass.yaml"));
    chmodSync(badDir, 0o000);

    try {
      const result = discoverSpecFiles(tmp, tmp, tmp);
      expect(isResult(result)).toBe(true);
      if (!isResult(result)) return;
      expect(result.files).toEqual(["good.yass.yaml"]);
      expect(result.errors.length).toBe(1);
      expect(result.errors[0]!.code).toBe("yass.discover.dir_unreadable");
    } finally {
      // Restore permissions so cleanup works.
      chmodSync(badDir, 0o755);
    }
  });

  test("top-level unreadable directory returns fatal error", () => {
    const badDir = join(tmp, "noaccess");
    mkdirSync(badDir, { recursive: true });
    chmodSync(badDir, 0o000);

    try {
      const result = discoverSpecFiles(badDir, tmp, tmp);
      expect(isError(result)).toBe(true);
      if (!isError(result)) return;
      expect(result.code).toBe("yass.path.unreadable");
    } finally {
      chmodSync(badDir, 0o755);
    }
  });

  // -----------------------------------------------------------------------
  // Symlinks during traversal
  // -----------------------------------------------------------------------

  test("does not follow symlinks during traversal", () => {
    const realDir = join(tmp, "real");
    mkdirSync(realDir, { recursive: true });
    touch(join(realDir, "spec.yass.yaml"));

    // Create a symlink inside the search root pointing to realDir.
    const linkDir = join(tmp, "linked");
    symlinkSync(realDir, linkDir);

    // Also put a direct file so we get some results.
    touch(join(tmp, "direct.yass.yaml"));

    const result = discoverSpecFiles(tmp, tmp, tmp);
    expect(isResult(result)).toBe(true);
    if (!isResult(result)) return;

    // The file under the symlinked directory should NOT appear,
    // but the one under realDir should, plus the direct file.
    expect(result.files).toEqual([
      "direct.yass.yaml",
      join("real", "spec.yass.yaml"),
    ]);
  });

  test("symlink file encountered during traversal is skipped", () => {
    touch(join(tmp, "target.yass.yaml"));
    symlinkSync(join(tmp, "target.yass.yaml"), join(tmp, "link.yass.yaml"));

    const result = discoverSpecFiles(tmp, tmp, tmp);
    expect(isResult(result)).toBe(true);
    if (!isResult(result)) return;
    // Only the real file, not the symlink.
    expect(result.files).toEqual(["target.yass.yaml"]);
  });

  // -----------------------------------------------------------------------
  // Symlink as direct file argument
  // -----------------------------------------------------------------------

  test("treats symlink file args specially (uses symlink path for reporting)", () => {
    const realFile = join(tmp, "real.yass.yaml");
    touch(realFile);

    const linkFile = join(tmp, "link.yass.yaml");
    symlinkSync(realFile, linkFile);

    // Pass the symlink path directly.
    const result = discoverSpecFiles(linkFile, tmp, tmp);
    expect(isResult(result)).toBe(true);
    if (!isResult(result)) return;
    // Should report the symlink path, not the target.
    expect(result.files).toEqual(["link.yass.yaml"]);
  });

  // -----------------------------------------------------------------------
  // Symlink as directory argument
  // -----------------------------------------------------------------------

  test("treats symlink directory arg as directory for traversal", () => {
    const realDir = join(tmp, "real");
    mkdirSync(realDir, { recursive: true });
    touch(join(realDir, "spec.yass.yaml"));

    const linkDir = join(tmp, "linked");
    symlinkSync(realDir, linkDir);

    // Pass symlink directory as the top-level argument — should traverse.
    const result = discoverSpecFiles(linkDir, tmp, tmp);
    expect(isResult(result)).toBe(true);
    if (!isResult(result)) return;
    expect(result.files).toEqual([join("linked", "spec.yass.yaml")]);
  });

  // -----------------------------------------------------------------------
  // Default to project root
  // -----------------------------------------------------------------------

  test("defaults to project root when no path provided", () => {
    touch(join(tmp, "spec.yass.yaml"));

    const result = discoverSpecFiles(undefined, tmp, tmp);
    expect(isResult(result)).toBe(true);
    if (!isResult(result)) return;
    expect(result.files).toEqual(["spec.yass.yaml"]);
  });

  // -----------------------------------------------------------------------
  // Edge: path neither file nor directory
  // -----------------------------------------------------------------------

  test("returns error for path that is neither file nor directory", () => {
    // We can test with /dev/null which is a character device.
    const result = discoverSpecFiles("/dev/null", tmp, tmp);
    expect(isError(result)).toBe(true);
    if (!isError(result)) return;
    expect(result.code).toBe("yass.path.unreadable");
  });

  // -----------------------------------------------------------------------
  // No leading "./" in relative paths
  // -----------------------------------------------------------------------

  test("emits relative paths without leading ./", () => {
    touch(join(tmp, "spec.yass.yaml"));
    touch(join(tmp, "sub", "other.yass.yaml"));

    const result = discoverSpecFiles(tmp, tmp, tmp);
    expect(isResult(result)).toBe(true);
    if (!isResult(result)) return;
    for (const f of result.files) {
      expect(f.startsWith("./")).toBe(false);
    }
  });
});
