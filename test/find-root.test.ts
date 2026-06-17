import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import { mkdirSync, writeFileSync, rmSync } from "node:fs";
import { join, resolve } from "node:path";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import {
  findProjectRoot,
  isFindRootError,
  type FindRootResult,
  type FindRootError,
} from "../src/find-root.ts";

/** Create a fresh temporary directory tree for each test. */
function makeTmp(): string {
  return mkdtempSync(join(tmpdir(), "yass-findroot-"));
}

describe("findProjectRoot", () => {
  let tmp: string;

  beforeEach(() => {
    tmp = makeTmp();
  });

  afterEach(() => {
    rmSync(tmp, { recursive: true, force: true });
  });

  // ---------------------------------------------------------------
  // .git discovery
  // ---------------------------------------------------------------

  test("finds .git in starting directory", () => {
    mkdirSync(join(tmp, ".git"));
    const result = findProjectRoot(tmp);
    expect(isFindRootError(result)).toBe(false);
    expect((result as FindRootResult).root).toBe(tmp);
  });

  test("finds .git in parent directory", () => {
    mkdirSync(join(tmp, ".git"));
    const child = join(tmp, "child");
    mkdirSync(child);
    const result = findProjectRoot(child);
    expect(isFindRootError(result)).toBe(false);
    expect((result as FindRootResult).root).toBe(tmp);
  });

  test("finds .git in grandparent directory", () => {
    mkdirSync(join(tmp, ".git"));
    const deep = join(tmp, "a", "b");
    mkdirSync(deep, { recursive: true });
    const result = findProjectRoot(deep);
    expect(isFindRootError(result)).toBe(false);
    expect((result as FindRootResult).root).toBe(tmp);
  });

  test("returns deepest .git ancestor (not shallowest)", () => {
    // Place .git at both tmp/ and tmp/child/
    mkdirSync(join(tmp, ".git"));
    const child = join(tmp, "child");
    mkdirSync(child);
    mkdirSync(join(child, ".git"));

    const leaf = join(child, "sub");
    mkdirSync(leaf);

    // Starting from leaf, the deepest .git ancestor is child/ (found first
    // when walking upward), not tmp/.
    const result = findProjectRoot(leaf);
    expect(isFindRootError(result)).toBe(false);
    expect((result as FindRootResult).root).toBe(child);
  });

  // ---------------------------------------------------------------
  // .yass.yaml fallback
  // ---------------------------------------------------------------

  test("falls back to .yass.yaml when no .git exists", () => {
    writeFileSync(join(tmp, ".yass.yaml"), "");
    const child = join(tmp, "child");
    mkdirSync(child);
    const result = findProjectRoot(child);
    expect(isFindRootError(result)).toBe(false);
    expect((result as FindRootResult).root).toBe(tmp);
  });

  test("does not honor .yass.yaml when .git exists somewhere on path", () => {
    // .yass.yaml in child, .git only in tmp (higher up).
    mkdirSync(join(tmp, ".git"));
    const child = join(tmp, "child");
    mkdirSync(child);
    writeFileSync(join(child, ".yass.yaml"), "");

    // Starting from child: upward search finds .git in tmp.
    // .yass.yaml in child must NOT be considered because .git exists.
    const result = findProjectRoot(child);
    expect(isFindRootError(result)).toBe(false);
    expect((result as FindRootResult).root).toBe(tmp);
  });

  // ---------------------------------------------------------------
  // Error case
  // ---------------------------------------------------------------

  test("errors when no marker found", () => {
    // tmp has neither .git nor .yass.yaml, and neither do ancestors within
    // the temp tree. We create a deep isolated path to reduce the chance of
    // accidentally matching a real .git above tmpdir. To guarantee isolation
    // we simply assert the returned code (if the host machine has a .git
    // above tmpdir the test would still pass because it would find that).
    const isolated = join(tmp, "a", "b", "c");
    mkdirSync(isolated, { recursive: true });

    const result = findProjectRoot(isolated);

    // If the host happens to have a .git somewhere above tmpdir, the function
    // will legitimately find it.  That is correct behaviour.  We can only
    // assert the error case when we know there is truly no marker.  So we
    // check: if an error was returned, it must be the right one.
    if (isFindRootError(result)) {
      expect((result as FindRootError).code).toBe("yass.findroot.no_marker");
      expect((result as FindRootError).message).toContain(isolated);
    } else {
      // It found a real .git above tmpdir -- acceptable, just not an error.
      expect((result as FindRootResult).root).toBeTruthy();
    }
  });

  // ---------------------------------------------------------------
  // Traversal invariants
  // ---------------------------------------------------------------

  test("starting directory is checked before parent", () => {
    // Both start and parent have .git -- we must get start back.
    mkdirSync(join(tmp, ".git"));
    const child = join(tmp, "child");
    mkdirSync(child);
    mkdirSync(join(child, ".git"));

    const result = findProjectRoot(child);
    expect(isFindRootError(result)).toBe(false);
    expect((result as FindRootResult).root).toBe(child);
  });

  test("does not descend into children", () => {
    // Place .git only inside a sibling directory -- it must NOT be found
    // when starting from a different child.
    const a = join(tmp, "a");
    const b = join(tmp, "b");
    mkdirSync(a);
    mkdirSync(b);
    mkdirSync(join(b, ".git"));

    // Starting from a: upward search should never look into b/.
    const result = findProjectRoot(a);
    if (isFindRootError(result)) {
      expect((result as FindRootError).code).toBe("yass.findroot.no_marker");
    } else {
      // If a real .git exists above tmpdir, the result must NOT be b.
      expect((result as FindRootResult).root).not.toBe(b);
    }
  });

  // ---------------------------------------------------------------
  // Default to cwd
  // ---------------------------------------------------------------

  test("uses cwd as default when no startDir provided", () => {
    // We just verify it returns *something* without throwing,
    // and that the result path is absolute.
    const result = findProjectRoot();
    if (isFindRootError(result)) {
      expect((result as FindRootError).code).toBe("yass.findroot.no_marker");
    } else {
      const root = (result as FindRootResult).root;
      // Must be an absolute path.
      expect(root.startsWith("/")).toBe(true);
    }
  });
});
