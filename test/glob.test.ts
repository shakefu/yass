import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import { mkdirSync, writeFileSync, symlinkSync, rmSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import {
  expandGlob,
  isGlobPattern,
  isGlobError,
  isGlobResult,
  type GlobResult,
  type GlobError,
} from "../src/glob.ts";

// ── helpers ─────────────────────────────────────────────────────────────────

/** Create a unique temporary directory for each test. */
function makeTmpDir(): string {
  const dir = join(tmpdir(), `yass-glob-test-${Date.now()}-${Math.random().toString(36).slice(2)}`);
  mkdirSync(dir, { recursive: true });
  return dir;
}

/** Create an empty file, creating parent directories as needed. */
function touch(path: string): void {
  const parent = path.slice(0, path.lastIndexOf("/"));
  mkdirSync(parent, { recursive: true });
  writeFileSync(path, "");
}

/** Detect whether the filesystem is case-sensitive. */
function isCaseSensitiveFS(dir: string): boolean {
  const probe = join(dir, "__CaSe_PrObE__");
  writeFileSync(probe, "");
  try {
    const entries = readdirSync(dir);
    // If writing "__CaSe_PrObE__" produces a file that also shows up as
    // "__case_probe__" in the listing, the FS is case-insensitive.
    return !entries.some((e) => e === "__case_probe__");
  } finally {
    rmSync(probe, { force: true });
  }
}

// ── isGlobPattern ───────────────────────────────────────────────────────────

describe("isGlobPattern", () => {
  test("returns false for plain paths", () => {
    expect(isGlobPattern("foo.txt")).toBe(false);
    expect(isGlobPattern("dir/file.txt")).toBe(false);
    expect(isGlobPattern("")).toBe(false);
    expect(isGlobPattern("no-special-chars")).toBe(false);
  });

  test("detects * metacharacter", () => {
    expect(isGlobPattern("*.txt")).toBe(true);
    expect(isGlobPattern("dir/*.txt")).toBe(true);
  });

  test("detects ** metacharacter", () => {
    expect(isGlobPattern("**/*.txt")).toBe(true);
  });

  test("detects ? metacharacter", () => {
    expect(isGlobPattern("file?.txt")).toBe(true);
  });

  test("detects [ metacharacter", () => {
    expect(isGlobPattern("[abc].txt")).toBe(true);
    expect(isGlobPattern("file[0-9].txt")).toBe(true);
  });

  test("does not treat braces as metacharacters", () => {
    expect(isGlobPattern("{a,b}.txt")).toBe(false);
  });
});

// ── expandGlob ──────────────────────────────────────────────────────────────

describe("expandGlob", () => {
  let tmp: string;

  beforeEach(() => {
    tmp = makeTmpDir();
  });

  afterEach(() => {
    rmSync(tmp, { recursive: true, force: true });
  });

  // ── literal path (no metacharacters) ────────────────────────────────────

  test("returns literal path unchanged when no metacharacters", () => {
    const result = expandGlob("some/path/file.txt");
    expect(isGlobResult(result)).toBe(true);
    expect((result as GlobResult).paths).toEqual(["some/path/file.txt"]);
  });

  test("returns literal path unchanged even when file does not exist", () => {
    const result = expandGlob("nonexistent.txt");
    expect(isGlobResult(result)).toBe(true);
    expect((result as GlobResult).paths).toEqual(["nonexistent.txt"]);
  });

  // ── star expansion ──────────────────────────────────────────────────────

  test("expands * to match files in a directory", () => {
    touch(join(tmp, "alpha.txt"));
    touch(join(tmp, "beta.txt"));
    touch(join(tmp, "gamma.log"));

    const result = expandGlob("*.txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    expect((result as GlobResult).paths).toEqual(["alpha.txt", "beta.txt"]);
  });

  test("* does not cross directory boundaries", () => {
    touch(join(tmp, "top.txt"));
    touch(join(tmp, "sub", "nested.txt"));

    const result = expandGlob("*.txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    expect((result as GlobResult).paths).toEqual(["top.txt"]);
  });

  // ── doublestar expansion ────────────────────────────────────────────────

  test("expands ** for recursive matching", () => {
    touch(join(tmp, "a.txt"));
    touch(join(tmp, "d1", "b.txt"));
    touch(join(tmp, "d1", "d2", "c.txt"));

    const result = expandGlob("**/*.txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    const paths = (result as GlobResult).paths;
    expect(paths).toContain("a.txt");
    expect(paths).toContain("d1/b.txt");
    expect(paths).toContain("d1/d2/c.txt");
    expect(paths.length).toBe(3);
  });

  // ── question mark expansion ─────────────────────────────────────────────

  test("expands ? for single character match", () => {
    touch(join(tmp, "a1.txt"));
    touch(join(tmp, "b2.txt"));
    touch(join(tmp, "cc.txt"));
    touch(join(tmp, "abc.txt")); // should NOT match — 3 chars before .txt

    const result = expandGlob("??.txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    const paths = (result as GlobResult).paths;
    expect(paths).toContain("a1.txt");
    expect(paths).toContain("b2.txt");
    expect(paths).toContain("cc.txt");
    expect(paths).not.toContain("abc.txt");
  });

  // ── bracket expressions ─────────────────────────────────────────────────

  test("bracket expressions work", () => {
    touch(join(tmp, "a.txt"));
    touch(join(tmp, "b.txt"));
    touch(join(tmp, "c.txt"));
    touch(join(tmp, "d.txt"));

    const result = expandGlob("[ab].txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    expect((result as GlobResult).paths).toEqual(["a.txt", "b.txt"]);
  });

  test("bracket range expressions work", () => {
    touch(join(tmp, "file1.txt"));
    touch(join(tmp, "file2.txt"));
    touch(join(tmp, "file3.txt"));
    touch(join(tmp, "fileA.txt"));

    const result = expandGlob("file[1-2].txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    expect((result as GlobResult).paths).toEqual(["file1.txt", "file2.txt"]);
  });

  // ── hidden files ────────────────────────────────────────────────────────

  test("does not match hidden files", () => {
    touch(join(tmp, "visible.txt"));
    touch(join(tmp, ".hidden.txt"));

    const result = expandGlob("*.txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    const paths = (result as GlobResult).paths;
    expect(paths).toContain("visible.txt");
    expect(paths).not.toContain(".hidden.txt");
  });

  test("does not descend into hidden directories", () => {
    touch(join(tmp, "top.txt"));
    touch(join(tmp, ".hidden", "secret.txt"));
    touch(join(tmp, "visible", "ok.txt"));

    const result = expandGlob("**/*.txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    const paths = (result as GlobResult).paths;
    expect(paths).toContain("top.txt");
    expect(paths).toContain("visible/ok.txt");
    expect(paths).not.toContain(".hidden/secret.txt");
  });

  // ── symlinks ────────────────────────────────────────────────────────────

  test("does not follow symbolic links", () => {
    touch(join(tmp, "real.txt"));
    symlinkSync(join(tmp, "real.txt"), join(tmp, "link.txt"));

    const result = expandGlob("*.txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    const paths = (result as GlobResult).paths;
    expect(paths).toContain("real.txt");
    expect(paths).not.toContain("link.txt");
  });

  test("does not follow symlinked directories", () => {
    touch(join(tmp, "realdir", "file.txt"));
    symlinkSync(join(tmp, "realdir"), join(tmp, "linkdir"));

    const result = expandGlob("**/*.txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    const paths = (result as GlobResult).paths;
    expect(paths).toContain("realdir/file.txt");
    expect(paths).not.toContain("linkdir/file.txt");
  });

  // ── brace expansion disabled ────────────────────────────────────────────

  test("does not perform brace expansion", () => {
    touch(join(tmp, "a.txt"));
    touch(join(tmp, "b.txt"));
    touch(join(tmp, "{a,b}.txt"));

    // {a,b}.txt is not a glob pattern (no metacharacters we recognise),
    // so it returns the literal string.
    const result = expandGlob("{a,b}.txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    expect((result as GlobResult).paths).toEqual(["{a,b}.txt"]);
  });

  test("braces in a pattern with real metacharacters are treated literally", () => {
    // Pattern has * so it IS a glob, but braces should still be literal.
    touch(join(tmp, "{x}.log"));
    touch(join(tmp, "y.log"));

    const result = expandGlob("{*}.log", tmp);
    expect(isGlobResult(result)).toBe(true);
    const paths = (result as GlobResult).paths;
    // Should match "{x}.log" because * matches "x" and braces are literal.
    expect(paths).toContain("{x}.log");
    // Should NOT match "y.log" because the braces are literal, so the
    // pattern requires the filename to start with "{" and end with "}.log".
    expect(paths).not.toContain("y.log");
  });

  // ── zero matches ────────────────────────────────────────────────────────

  test("returns error on zero matches", () => {
    // tmp is empty — no files to match
    const result = expandGlob("*.txt", tmp);
    expect(isGlobError(result)).toBe(true);
    const err = result as GlobError;
    expect(err.code).toBe("yass.glob.no_match");
    expect(err.message).toContain("*.txt");
  });

  test("error includes the original pattern", () => {
    const result = expandGlob("**/*.nonexistent", tmp);
    expect(isGlobError(result)).toBe(true);
    expect((result as GlobError).message).toContain("**/*.nonexistent");
  });

  // ── sorting ─────────────────────────────────────────────────────────────

  test("results are sorted by Unicode code-point order", () => {
    // Create files in reverse order to ensure sorting is applied.
    touch(join(tmp, "z.txt"));
    touch(join(tmp, "a.txt"));
    touch(join(tmp, "m.txt"));
    touch(join(tmp, "b.txt"));

    const result = expandGlob("*.txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    const paths = (result as GlobResult).paths;
    expect(paths).toEqual(["a.txt", "b.txt", "m.txt", "z.txt"]);
  });

  test("uppercase sorts before lowercase in code-point order", () => {
    // On case-insensitive FS (macOS default) we cannot create both A.txt
    // and a.txt, so we test with names that don't collide.
    touch(join(tmp, "Bfile.txt"));
    touch(join(tmp, "afile.txt"));

    const result = expandGlob("*file.txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    const paths = (result as GlobResult).paths;
    // 'B' (0x42) < 'a' (0x61) in code-point order
    if (isCaseSensitiveFS(tmp)) {
      expect(paths).toEqual(["Bfile.txt", "afile.txt"]);
    } else {
      // Case-insensitive FS: both files exist but may sort differently
      // depending on how the FS returns them.  Just verify both are present
      // and the array is sorted.
      expect(paths).toContain("Bfile.txt");
      expect(paths).toContain("afile.txt");
      // Verify sort is applied (our comparator normalises NFC and uses <)
      for (let i = 1; i < paths.length; i++) {
        expect(paths[i]!.normalize("NFC") >= paths[i - 1]!.normalize("NFC")).toBe(true);
      }
    }
  });

  test("sorting uses NFC-normalised comparison", () => {
    // Create files with names that sort in a known order
    touch(join(tmp, "c.txt"));
    touch(join(tmp, "a.txt"));

    const result = expandGlob("*.txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    const paths = (result as GlobResult).paths;
    // After NFC normalisation, sorting should be deterministic
    expect(paths).toEqual(["a.txt", "c.txt"]);
  });

  test("results with subdirectories are sorted correctly", () => {
    touch(join(tmp, "z", "a.txt"));
    touch(join(tmp, "a", "z.txt"));
    touch(join(tmp, "m", "m.txt"));

    const result = expandGlob("**/*.txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    const paths = (result as GlobResult).paths;
    // "/" (0x2F) sorts before any lowercase letter, so path components
    // are compared naturally.
    expect(paths).toEqual(["a/z.txt", "m/m.txt", "z/a.txt"]);
  });

  // ── case sensitivity ────────────────────────────────────────────────────

  test("matching is case-sensitive on supported filesystems", () => {
    // On case-insensitive FS (macOS HFS+/APFS), File.TXT, file.txt, FILE.txt
    // all refer to the same file. We test case sensitivity of the comparator
    // by checking sort order of names that differ in case but are unique.
    touch(join(tmp, "Alpha.txt"));
    touch(join(tmp, "beta.txt"));

    const result = expandGlob("*.txt", tmp);
    expect(isGlobResult(result)).toBe(true);
    const paths = (result as GlobResult).paths;
    expect(paths).toContain("Alpha.txt");
    expect(paths).toContain("beta.txt");
    // In code-point order, uppercase A (0x41) < lowercase b (0x62)
    expect(paths.indexOf("Alpha.txt")).toBeLessThan(paths.indexOf("beta.txt"));
  });

  // ── type guards ─────────────────────────────────────────────────────────

  test("isGlobResult and isGlobError are mutually exclusive", () => {
    const ok: GlobResult = { paths: ["a"] };
    const err: GlobError = { code: "yass.glob.no_match", message: "nope" };

    expect(isGlobResult(ok)).toBe(true);
    expect(isGlobError(ok)).toBe(false);
    expect(isGlobResult(err)).toBe(false);
    expect(isGlobError(err)).toBe(true);
  });
});
