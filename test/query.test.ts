import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import {
  mkdirSync,
  writeFileSync,
  rmSync,
} from "node:fs";
import { join, resolve } from "node:path";
import { tmpdir } from "node:os";
import { queryCommand } from "../src/query.ts";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeTmpDir(): string {
  const dir = join(
    tmpdir(),
    `yass-query-test-${Date.now()}-${Math.random().toString(36).slice(2)}`,
  );
  mkdirSync(dir, { recursive: true });
  return dir;
}

function touch(path: string, content: string = ""): void {
  const parent = resolve(path, "..");
  mkdirSync(parent, { recursive: true });
  writeFileSync(path, content);
}

interface CapturedOutput {
  stdout: string;
  stderr: string;
}

function run(
  args: string[],
  cwd: string,
  isTTY: boolean = false,
): { exitCode: number } & CapturedOutput {
  let stdoutBuf = "";
  let stderrBuf = "";
  const stdout = { write(s: string) { stdoutBuf += s; } };
  const stderr = { write(s: string) { stderrBuf += s; } };
  const exitCode = queryCommand(args, cwd, stdout, stderr, isTTY);
  return { exitCode, stdout: stdoutBuf, stderr: stderrBuf };
}

// ---------------------------------------------------------------------------
// Fixture content
// ---------------------------------------------------------------------------

const PREAMBLE = `---
description: Test spec file.
version: v1
`;

const SIMPLE_SPEC = `${PREAMBLE}---
spec: SimpleSpec
INPUT:
- MUST: accept a string
RETURN:
- MUST: return a boolean
ERROR:
- WHEN: the input is empty
  MUST: error with invalid input
`;

const MULTI_SPEC = `${PREAMBLE}---
spec: FirstSpec
INPUT:
- MUST: accept input
RETURN:
- MUST: return output
---
spec: SecondSpec
INPUT:
- MUST: accept different input
ERROR:
- MUST: error on failure
---
spec: ThirdSpec
INVARIANT:
- MUST: always hold true
`;

const NAMESPACED_SPEC = `${PREAMBLE}---
spec: cli.query.NameLookup
INPUT:
- MUST: accept a spec name string
RETURN:
- MUST: match the full spec name
`;

const CONFORMS_SPEC = `${PREAMBLE}---
spec: BaseSpec
RETURN:
- MUST: return a string
- MUST-NOT: return null
INVARIANT:
- MUST: be idempotent
---
spec: DerivedSpec
INPUT:
- MUST: accept input
RETURN:
- CONFORMS: BaseSpec::RETURN
- MUST: also return a number
ERROR:
- MUST: error on invalid input
`;

const CONFORMS_NORMATIVE = `${PREAMBLE}---
spec: BaseSpec
RETURN:
- MUST: return a string
- MUST-NOT: return null
---
spec: NormativeDerived
RETURN:
- MUST: do something special
  CONFORMS: BaseSpec::RETURN
`;

const CONFORMS_WITH_WHEN = `${PREAMBLE}---
spec: BaseSpec
RETURN:
- MUST: return a string
- WHEN: input is empty
  MUST: return empty string
---
spec: GuardedDerived
RETURN:
- WHEN: the mode is strict
  CONFORMS: BaseSpec::RETURN
`;

const CONFORMS_WHEN_COMBO = `${PREAMBLE}---
spec: BaseSpec
RETURN:
- WHEN: input is valid
  MUST: return a string
---
spec: ComboDerived
RETURN:
- WHEN: the mode is strict
  CONFORMS: BaseSpec::RETURN
`;

const CONFORMS_NO_SLOT = `${PREAMBLE}---
spec: BadRef
RETURN:
- CONFORMS: SomeSpec
`;

const CONFORMS_UNRESOLVED = `${PREAMBLE}---
spec: UnresolvedRef
RETURN:
- CONFORMS: NonExistent::RETURN
`;

const CONFORMS_WITH_USES = `${PREAMBLE}---
spec: BaseSpec
RETURN:
- MUST: return a string
---
spec: WithUses
RETURN:
- MUST: accept input
  USES: BaseSpec
  CONFORMS: BaseSpec::RETURN
`;

const SPEC_WITH_QUOTING_NEEDS = `${PREAMBLE}---
spec: QuotingSpec
INPUT:
- MUST: "accept a value: with colon-space"
RETURN:
- MUST: return true
`;

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("queryCommand", () => {
  let tmp: string;

  beforeEach(() => {
    tmp = makeTmpDir();
    // Create a .git directory so findProjectRoot works
    mkdirSync(join(tmp, ".git"), { recursive: true });
  });

  afterEach(() => {
    rmSync(tmp, { recursive: true, force: true });
  });

  // =========================================================================
  // Error cases: missing/blank name
  // =========================================================================

  describe("missing spec name", () => {
    test("errors with yass.query.name_missing when no args given", () => {
      const result = run([], tmp);
      expect(result.exitCode).toBe(2);
      expect(result.stderr).toContain("yass.query.name_missing");
      expect(result.stdout).toBe("");
    });
  });

  describe("blank spec name", () => {
    test("errors with yass.query.name_blank when name is empty string", () => {
      const result = run([""], tmp);
      expect(result.exitCode).toBe(2);
      expect(result.stderr).toContain("yass.query.name_blank");
      expect(result.stdout).toBe("");
    });
  });

  describe("whitespace in spec name", () => {
    test("treats whitespace name as no-match, not blank error", () => {
      touch(join(tmp, "spec.yass.yaml"), SIMPLE_SPEC);
      const result = run(["Simple Spec"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stderr).toContain("yass.query.no_match");
      expect(result.stderr).not.toContain("yass.query.name_blank");
    });

    test("whitespace-only name treated as no-match", () => {
      touch(join(tmp, "spec.yass.yaml"), SIMPLE_SPEC);
      const result = run(["  "], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stderr).toContain("yass.query.no_match");
    });
  });

  // =========================================================================
  // Scope validation
  // =========================================================================

  describe("scope validation", () => {
    test("errors with scope_not_found when scope does not exist", () => {
      const result = run(["SomeSpec", join(tmp, "nonexistent")], tmp);
      expect(result.exitCode).toBe(2);
      expect(result.stderr).toContain("yass.query.scope_not_found");
    });

    test("errors with scope_empty when scope has no .yass.yaml files", () => {
      const emptyDir = join(tmp, "empty");
      mkdirSync(emptyDir, { recursive: true });
      const result = run(["SomeSpec", emptyDir], tmp);
      expect(result.exitCode).toBe(2);
      expect(result.stderr).toContain("yass.query.scope_empty");
    });

    test("scope error takes priority over name-not-found", () => {
      const result = run(["NonExistent", join(tmp, "nonexistent")], tmp);
      expect(result.exitCode).toBe(2);
      expect(result.stderr).toContain("yass.query.scope_not_found");
      expect(result.stderr).not.toContain("yass.query.no_match");
    });

    test("rejects scope paths containing colon", () => {
      const result = run(["SomeSpec", "path:with:colon"], tmp);
      expect(result.exitCode).toBe(2);
      expect(result.stderr).toContain("yass.path.colon_in_path");
    });

    test("validates scope before name lookup", () => {
      // Scope is invalid directory, name could match but we shouldn't get there
      const result = run(["SimpleSpec", join(tmp, "nope")], tmp);
      expect(result.exitCode).toBe(2);
      expect(result.stderr).toContain("yass.query.scope_not_found");
    });
  });

  // =========================================================================
  // Name lookup: full match, suffix match, no partial match
  // =========================================================================

  describe("name lookup", () => {
    test("full match returns the spec", () => {
      touch(join(tmp, "spec.yass.yaml"), SIMPLE_SPEC);
      const result = run(["SimpleSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("spec: SimpleSpec");
    });

    test("dot-aligned trailing suffix matches", () => {
      touch(join(tmp, "spec.yass.yaml"), NAMESPACED_SPEC);
      const result = run(["NameLookup"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("spec: cli.query.NameLookup");
    });

    test("multi-component suffix matches", () => {
      touch(join(tmp, "spec.yass.yaml"), NAMESPACED_SPEC);
      const result = run(["query.NameLookup"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("spec: cli.query.NameLookup");
    });

    test("partial substring does NOT match", () => {
      touch(join(tmp, "spec.yass.yaml"), NAMESPACED_SPEC);
      const result = run(["ameLookup"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stderr).toContain("yass.query.no_match");
    });

    test("case-sensitive: wrong case does NOT match", () => {
      touch(join(tmp, "spec.yass.yaml"), SIMPLE_SPEC);
      const result = run(["simplespec"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stderr).toContain("yass.query.no_match");
    });

    test("no match emits error to stderr", () => {
      touch(join(tmp, "spec.yass.yaml"), SIMPLE_SPEC);
      const result = run(["NonExistentSpec"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stderr).toContain("yass.query.no_match");
      expect(result.stdout).toBe("");
    });

    test("does not match partial dot-prefix", () => {
      // "cli" should not match "cli.query.NameLookup" because "cli" is not
      // a trailing suffix — it's a leading prefix
      touch(join(tmp, "spec.yass.yaml"), NAMESPACED_SPEC);
      const result = run(["cli"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stderr).toContain("yass.query.no_match");
    });
  });

  // =========================================================================
  // Single match: fragment emission
  // =========================================================================

  describe("single match fragment", () => {
    test("starts with ---", () => {
      touch(join(tmp, "spec.yass.yaml"), SIMPLE_SPEC);
      const result = run(["SimpleSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout.startsWith("---\n")).toBe(true);
    });

    test("does NOT end with ...", () => {
      touch(join(tmp, "spec.yass.yaml"), SIMPLE_SPEC);
      const result = run(["SimpleSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).not.toContain("...");
    });

    test("ends with exactly one trailing LF", () => {
      touch(join(tmp, "spec.yass.yaml"), SIMPLE_SPEC);
      const result = run(["SimpleSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout.endsWith("\n")).toBe(true);
      expect(result.stdout.endsWith("\n\n")).toBe(false);
    });

    test("does NOT include preamble", () => {
      touch(join(tmp, "spec.yass.yaml"), SIMPLE_SPEC);
      const result = run(["SimpleSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).not.toContain("description:");
      expect(result.stdout).not.toContain("version:");
    });

    test("does NOT include other specs from same file", () => {
      touch(join(tmp, "spec.yass.yaml"), MULTI_SPEC);
      const result = run(["FirstSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("spec: FirstSpec");
      expect(result.stdout).not.toContain("SecondSpec");
      expect(result.stdout).not.toContain("ThirdSpec");
    });

    test("includes all slots and obligations", () => {
      touch(join(tmp, "spec.yass.yaml"), SIMPLE_SPEC);
      const result = run(["SimpleSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("INPUT:");
      expect(result.stdout).toContain("RETURN:");
      expect(result.stdout).toContain("ERROR:");
      expect(result.stdout).toContain("accept a string");
      expect(result.stdout).toContain("return a boolean");
    });

    test("uses 2-space indentation", () => {
      touch(join(tmp, "spec.yass.yaml"), SIMPLE_SPEC);
      const result = run(["SimpleSpec"], tmp);
      expect(result.exitCode).toBe(0);
      const lines = result.stdout.split("\n");
      // In the ERROR slot, the obligation has WHEN + MUST.
      // Key order is: Normativity (MUST) first, then WHEN.
      // First key gets "- " prefix, continuation gets "  " prefix.
      const mustLine = lines.find((l) => l.includes("MUST: error"));
      expect(mustLine).toBeDefined();
      expect(mustLine!.startsWith("- ")).toBe(true);
      const whenLine = lines.find((l) => l.includes("WHEN:"));
      expect(whenLine).toBeDefined();
      expect(whenLine!.startsWith("  ")).toBe(true);
    });

    test("emits list items with dash-space prefix", () => {
      touch(join(tmp, "spec.yass.yaml"), SIMPLE_SPEC);
      const result = run(["SimpleSpec"], tmp);
      expect(result.exitCode).toBe(0);
      const lines = result.stdout.split("\n");
      const listItemLines = lines.filter((l) => l.startsWith("- "));
      expect(listItemLines.length).toBeGreaterThan(0);
    });
  });

  // =========================================================================
  // Multi-match: disambiguation
  // =========================================================================

  describe("multi-match disambiguation", () => {
    test("emits tab-separated rows when multiple specs match", () => {
      // Two files each with a spec named "Spec" via suffix matching
      const file1Content = `${PREAMBLE}---
spec: Alpha.Spec
INPUT:
- MUST: accept input
`;
      const file2Content = `---
description: Other file.
version: v1
---
spec: Beta.Spec
INPUT:
- MUST: accept input
`;
      touch(join(tmp, "alpha.yass.yaml"), file1Content);
      touch(join(tmp, "beta.yass.yaml"), file2Content);

      const result = run(["Spec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stderr).toBe("");

      const lines = result.stdout.trim().split("\n");
      expect(lines.length).toBe(2);
      // Each line should have tab-separated fields
      for (const line of lines) {
        const fields = line.split("\t");
        expect(fields.length).toBe(3);
      }
    });

    test("does NOT emit YAML fragment on multi-match", () => {
      const file1Content = `${PREAMBLE}---
spec: Alpha.Spec
INPUT:
- MUST: accept input
`;
      const file2Content = `---
description: Other file.
version: v1
---
spec: Beta.Spec
INPUT:
- MUST: accept input
`;
      touch(join(tmp, "alpha.yass.yaml"), file1Content);
      touch(join(tmp, "beta.yass.yaml"), file2Content);

      const result = run(["Spec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).not.toContain("---");
    });

    test("does NOT write to stderr on multi-match", () => {
      const file1Content = `${PREAMBLE}---
spec: Alpha.Spec
INPUT:
- MUST: accept input
`;
      const file2Content = `---
description: Other file.
version: v1
---
spec: Beta.Spec
INPUT:
- MUST: accept input
`;
      touch(join(tmp, "alpha.yass.yaml"), file1Content);
      touch(join(tmp, "beta.yass.yaml"), file2Content);

      const result = run(["Spec"], tmp);
      expect(result.stderr).toBe("");
    });

    test("rows sorted by file path lex order, specs in document order", () => {
      const fileA = `---
description: Alpha file.
version: v1
---
spec: Alpha.Spec
INPUT:
- MUST: accept input
`;
      const fileB = `---
description: Beta file.
version: v1
---
spec: Beta.Spec
INPUT:
- MUST: accept input
`;
      touch(join(tmp, "b.yass.yaml"), fileB);
      touch(join(tmp, "a.yass.yaml"), fileA);

      const result = run(["Spec"], tmp);
      expect(result.exitCode).toBe(0);
      const lines = result.stdout.trim().split("\n");
      expect(lines.length).toBe(2);
      // a.yass.yaml should come before b.yass.yaml
      expect(lines[0]!).toContain("a.yass.yaml");
      expect(lines[1]!).toContain("b.yass.yaml");
    });

    test("does NOT truncate descriptions on multi-match", () => {
      const longDesc =
        "A very long description that would normally be truncated if we were in list mode with TTY truncation enabled but should not be truncated in query disambiguation mode";
      const content = `---
description: ${longDesc}
version: v1
---
spec: Alpha.Spec
INPUT:
- MUST: accept input
`;
      const content2 = `---
description: Short.
version: v1
---
spec: Beta.Spec
INPUT:
- MUST: accept input
`;
      touch(join(tmp, "a.yass.yaml"), content);
      touch(join(tmp, "b.yass.yaml"), content2);

      const result = run(["Spec"], tmp, true); // TTY=true
      expect(result.exitCode).toBe(0);
      const lines = result.stdout.trim().split("\n");
      const firstFields = lines[0]!.split("\t");
      expect(firstFields[2]).toBe(longDesc);
    });
  });

  // =========================================================================
  // CONFORMS inlining: reference-only
  // =========================================================================

  describe("CONFORMS inlining: reference-only", () => {
    test("replaces reference-only CONFORMS with inlined obligations", () => {
      touch(join(tmp, "spec.yass.yaml"), CONFORMS_SPEC);
      const result = run(["DerivedSpec"], tmp);
      expect(result.exitCode).toBe(0);
      // The inlined obligations from BaseSpec::RETURN should appear
      expect(result.stdout).toContain("return a string");
      expect(result.stdout).toContain("return null");
      // The CONFORMS ref key should be stripped from obligation mappings
      // (provenance comments like "# CONFORMS:" are expected)
      const lines = result.stdout.split("\n");
      const conformsKeyLines = lines.filter(
        (l) => !l.startsWith("#") && l.includes("CONFORMS:"),
      );
      expect(conformsKeyLines.length).toBe(0);
    });

    test("preserves normativity of inlined obligations", () => {
      touch(join(tmp, "spec.yass.yaml"), CONFORMS_SPEC);
      const result = run(["DerivedSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("MUST: return a string");
      expect(result.stdout).toContain("MUST-NOT: return null");
    });

    test("emits provenance comment before each inlined obligation", () => {
      touch(join(tmp, "spec.yass.yaml"), CONFORMS_SPEC);
      const result = run(["DerivedSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("# CONFORMS: BaseSpec::RETURN");
    });

    test("provenance comment at column zero", () => {
      touch(join(tmp, "spec.yass.yaml"), CONFORMS_SPEC);
      const result = run(["DerivedSpec"], tmp);
      expect(result.exitCode).toBe(0);
      const lines = result.stdout.split("\n");
      const provenanceLines = lines.filter((l) =>
        l.startsWith("# CONFORMS:"),
      );
      expect(provenanceLines.length).toBeGreaterThan(0);
      for (const pl of provenanceLines) {
        expect(pl[0]).toBe("#"); // starts at column zero
      }
    });
  });

  // =========================================================================
  // CONFORMS inlining: normative with CONFORMS
  // =========================================================================

  describe("CONFORMS inlining: normative with CONFORMS", () => {
    test("keeps normative obligation and appends inlined after it", () => {
      touch(join(tmp, "spec.yass.yaml"), CONFORMS_NORMATIVE);
      const result = run(["NormativeDerived"], tmp);
      expect(result.exitCode).toBe(0);
      // The original normative obligation should be kept
      expect(result.stdout).toContain("MUST: do something special");
      // Inlined obligations should follow
      expect(result.stdout).toContain("MUST: return a string");
      expect(result.stdout).toContain("MUST-NOT: return null");
      // CONFORMS key should be stripped from carrier (provenance comments ok)
      const lines = result.stdout.split("\n");
      const conformsKeyLines = lines.filter(
        (l) => !l.startsWith("#") && l.includes("CONFORMS:"),
      );
      expect(conformsKeyLines.length).toBe(0);
    });

    test("normative carrier appears before inlined obligations", () => {
      touch(join(tmp, "spec.yass.yaml"), CONFORMS_NORMATIVE);
      const result = run(["NormativeDerived"], tmp);
      expect(result.exitCode).toBe(0);
      const stdout = result.stdout;
      const carrierIdx = stdout.indexOf("do something special");
      const inlinedIdx = stdout.indexOf("return a string");
      expect(carrierIdx).toBeLessThan(inlinedIdx);
    });
  });

  // =========================================================================
  // WHEN guard combination
  // =========================================================================

  describe("WHEN guard combination", () => {
    test("carrier WHEN guard is applied to inlined obligations without WHEN", () => {
      touch(join(tmp, "spec.yass.yaml"), CONFORMS_WITH_WHEN);
      const result = run(["GuardedDerived"], tmp);
      expect(result.exitCode).toBe(0);
      // First inlined obligation (no inner WHEN): gets carrier WHEN
      expect(result.stdout).toContain("the mode is strict");
    });

    test("combined WHEN: outer + ' and ' + inner", () => {
      touch(join(tmp, "spec.yass.yaml"), CONFORMS_WHEN_COMBO);
      const result = run(["ComboDerived"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain(
        "the mode is strict and input is valid",
      );
    });

    test("combined WHEN has no parentheses and no case change", () => {
      touch(join(tmp, "spec.yass.yaml"), CONFORMS_WHEN_COMBO);
      const result = run(["ComboDerived"], tmp);
      expect(result.exitCode).toBe(0);
      // Should not have parentheses around the combined guard
      expect(result.stdout).not.toContain("(the mode");
      expect(result.stdout).not.toContain("valid)");
    });
  });

  // =========================================================================
  // CONFORMS error cases
  // =========================================================================

  describe("CONFORMS errors", () => {
    test("conforms_no_slot when CONFORMS ref lacks ::SLOT suffix", () => {
      touch(join(tmp, "spec.yass.yaml"), CONFORMS_NO_SLOT);
      const result = run(["BadRef"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stderr).toContain("yass.query.conforms_no_slot");
      expect(result.stdout).toBe("");
    });

    test("conforms_unresolved when ref cannot be resolved", () => {
      touch(join(tmp, "spec.yass.yaml"), CONFORMS_UNRESOLVED);
      const result = run(["UnresolvedRef"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stderr).toContain("yass.query.conforms_unresolved");
      expect(result.stdout).toBe("");
    });

    test("does NOT emit partial fragment on CONFORMS failure", () => {
      touch(join(tmp, "spec.yass.yaml"), CONFORMS_UNRESOLVED);
      const result = run(["UnresolvedRef"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stdout).toBe("");
    });
  });

  // =========================================================================
  // Non-CONFORMS refs (USES, SEE)
  // =========================================================================

  describe("USES and SEE refs", () => {
    test("USES is NOT inlined, kept on carrier after CONFORMS stripping", () => {
      touch(join(tmp, "spec.yass.yaml"), CONFORMS_WITH_USES);
      const result = run(["WithUses"], tmp);
      expect(result.exitCode).toBe(0);
      // USES should remain on the carrier
      expect(result.stdout).toContain("USES: BaseSpec");
      // CONFORMS key should be stripped (provenance comments ok)
      const lines = result.stdout.split("\n");
      const conformsKeyLines = lines.filter(
        (l) => !l.startsWith("#") && l.includes("CONFORMS:"),
      );
      expect(conformsKeyLines.length).toBe(0);
    });
  });

  // =========================================================================
  // OutputProfile: quoting rules
  // =========================================================================

  describe("OutputProfile quoting", () => {
    test("plain scalars unquoted by default", () => {
      touch(join(tmp, "spec.yass.yaml"), SIMPLE_SPEC);
      const result = run(["SimpleSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("MUST: accept a string");
      // "accept a string" should NOT be quoted
      expect(result.stdout).not.toContain('MUST: "accept a string"');
    });

    test("double-quotes when scalar contains colon-space", () => {
      touch(join(tmp, "spec.yass.yaml"), SPEC_WITH_QUOTING_NEEDS);
      const result = run(["QuotingSpec"], tmp);
      expect(result.exitCode).toBe(0);
      // The scalar that contains ": " should be quoted
      expect(result.stdout).toContain('"accept a value: with colon-space"');
    });

    test("double-quotes yaml type tokens", () => {
      const content = `${PREAMBLE}---
spec: BoolSpec
INPUT:
- MUST: true
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["BoolSpec"], tmp);
      expect(result.exitCode).toBe(0);
      // "true" is a YAML core-schema token and must be quoted
      // The yaml parser will have parsed "true" to boolean true,
      // and we must re-emit it quoted
      expect(result.stdout).toContain('MUST: "true"');
    });

    test("double-quotes scalars with leading special characters", () => {
      const content = `${PREAMBLE}---
spec: SpecialSpec
INPUT:
- MUST: "*something"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["SpecialSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('"*something"');
    });
  });

  // =========================================================================
  // Scope with file path
  // =========================================================================

  describe("scope with file path", () => {
    test("narrows search to a specific file", () => {
      touch(join(tmp, "a.yass.yaml"), SIMPLE_SPEC);
      const otherSpec = `${PREAMBLE}---
spec: OtherSpec
INPUT:
- MUST: do something
`;
      touch(join(tmp, "b.yass.yaml"), otherSpec);

      // Search for SimpleSpec in a.yass.yaml only
      const result = run(["SimpleSpec", join(tmp, "a.yass.yaml")], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("spec: SimpleSpec");
    });

    test("narrows search to a specific directory", () => {
      const subDir = join(tmp, "sub");
      touch(join(subDir, "spec.yass.yaml"), SIMPLE_SPEC);
      const otherSpec = `${PREAMBLE}---
spec: OtherSpec
INPUT:
- MUST: do something
`;
      touch(join(tmp, "other.yass.yaml"), otherSpec);

      // Search within sub/ directory only
      const result = run(["SimpleSpec", subDir], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("spec: SimpleSpec");
    });
  });

  // =========================================================================
  // Default scope: project root
  // =========================================================================

  describe("default scope", () => {
    test("searches from project root when no scope given", () => {
      const subDir = join(tmp, "sub");
      touch(join(subDir, "spec.yass.yaml"), SIMPLE_SPEC);

      // Even from a subdirectory, should find files at project root level
      const result = run(["SimpleSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("spec: SimpleSpec");
    });
  });

  // =========================================================================
  // Cross-file CONFORMS resolution
  // =========================================================================

  describe("cross-file CONFORMS resolution", () => {
    test("resolves CONFORMS ref to spec in another file (same-file ref)", () => {
      // This tests the common case where the ref spec is in the same file
      touch(join(tmp, "spec.yass.yaml"), CONFORMS_SPEC);
      const result = run(["DerivedSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("return a string");
    });
  });

  // =========================================================================
  // Obligation key order
  // =========================================================================

  describe("obligation key order", () => {
    test("normativity keyword comes before WHEN", () => {
      const content = `${PREAMBLE}---
spec: OrderSpec
ERROR:
- WHEN: the input is empty
  MUST: error with invalid input
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["OrderSpec"], tmp);
      expect(result.exitCode).toBe(0);
      const lines = result.stdout.split("\n");
      // Find the MUST and WHEN lines
      const mustLine = lines.findIndex((l) => l.includes("MUST: error"));
      const whenLine = lines.findIndex((l) => l.includes("WHEN:"));
      // In the same obligation, MUST should come before WHEN
      // But actually the spec says: "Normativity, then WHEN, then References"
      // So we check obligation key order within a single list item
      // The first key in the list item starts with "- "
      const firstKeyLine = lines.find(
        (l) =>
          l.startsWith("- ") &&
          (l.includes("MUST:") || l.includes("WHEN:")),
      );
      // When the source has WHEN first, our emitter reorders to MUST first
      // Actually, let's just check the output ordering
      expect(firstKeyLine).toContain("MUST:");
    });
  });

  // =========================================================================
  // Fragment: no extra content
  // =========================================================================

  describe("fragment isolation", () => {
    test("only one --- at the start", () => {
      touch(join(tmp, "spec.yass.yaml"), MULTI_SPEC);
      const result = run(["SecondSpec"], tmp);
      expect(result.exitCode).toBe(0);
      const dashes = result.stdout.match(/^---$/gm);
      expect(dashes).toHaveLength(1);
    });
  });

  // =========================================================================
  // Zero match when no files found
  // =========================================================================

  describe("zero match scenarios", () => {
    test("no .yass.yaml files found: exits 1 with no_match", () => {
      // No spec files in the project
      const result = run(["SomeSpec"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stderr).toContain("yass.query.no_match");
    });
  });

  // =========================================================================
  // UTF-8 / LF
  // =========================================================================

  describe("output encoding", () => {
    test("output uses LF line endings", () => {
      touch(join(tmp, "spec.yass.yaml"), SIMPLE_SPEC);
      const result = run(["SimpleSpec"], tmp);
      expect(result.exitCode).toBe(0);
      // No CR characters
      expect(result.stdout).not.toContain("\r");
    });
  });

  // =========================================================================
  // YAML emitter: unknown keys (line 151)
  // =========================================================================

  describe("emitFragment unknown keys", () => {
    test("unknown top-level key is emitted as key: value", () => {
      // A spec with a key that is not "spec" and not a slot keyword
      const content = `${PREAMBLE}---
spec: CustomKeySpec
flavor: vanilla
INPUT:
- MUST: accept input
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["CustomKeySpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("flavor: vanilla");
    });
  });

  // =========================================================================
  // YAML emitter: obligation with no recognized keys (lines 194-196)
  // =========================================================================

  describe("emitObligation no recognized keys", () => {
    test("obligation with unknown keys still emits them", () => {
      // Create a spec where an obligation has no normativity, WHEN, or ref keys
      // This is technically invalid per the spec, but the emitter should handle it.
      // We can achieve this by having a YAML file where the obligation has only
      // custom keys - but the validator normally rejects these.
      // The easiest path: create a base spec with unusual keys and CONFORMS inline them.
      // Actually, since inlineConforms builds obligation objects, the simplest
      // approach is to test via a spec that has obligations with non-standard keys
      // that the parser preserves.

      // The yaml parser preserves all keys. Let's create a spec with a non-standard
      // obligation key only (no MUST/SHOULD/etc, no WHEN, no CONFORMS/USES/SEE).
      // The query command doesn't validate - it just emits.
      const content = `${PREAMBLE}---
spec: WeirdSpec
INPUT:
- custom: some value
  another: thing
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["WeirdSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("custom: some value");
      expect(result.stdout).toContain("another: thing");
    });
  });

  // =========================================================================
  // YAML emitter: additional quoting edge cases
  // =========================================================================

  describe("OutputProfile quoting edge cases", () => {
    test("empty scalar value is double-quoted", () => {
      const content = `${PREAMBLE}---
spec: EmptyValSpec
INPUT:
- MUST: ""
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["EmptyValSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('MUST: ""');
    });

    test("scalars with leading/trailing whitespace are quoted", () => {
      const content = `${PREAMBLE}---
spec: WhitespaceSpec
INPUT:
- MUST: "  padded  "
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["WhitespaceSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('"  padded  "');
    });

    test("numeric integer scalar is quoted", () => {
      const content = `${PREAMBLE}---
spec: NumSpec
INPUT:
- MUST: "42"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["NumSpec"], tmp);
      expect(result.exitCode).toBe(0);
      // The YAML parser will parse "42" as number 42, and the emitter must quote it
      expect(result.stdout).toContain('MUST: "42"');
    });

    test("null scalar is quoted", () => {
      const content = `${PREAMBLE}---
spec: NullSpec
INPUT:
- MUST: "null"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["NullSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('MUST: "null"');
    });

    test("scalars with leading dash are quoted", () => {
      const content = `${PREAMBLE}---
spec: DashSpec
INPUT:
- MUST: "-something"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["DashSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('"-something"');
    });

    test("scalars with leading question mark are quoted", () => {
      const content = `${PREAMBLE}---
spec: QMarkSpec
INPUT:
- MUST: "?maybe"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["QMarkSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('"?maybe"');
    });

    test("hex numeric literal is quoted", () => {
      const content = `${PREAMBLE}---
spec: HexSpec
INPUT:
- MUST: "0xFF"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["HexSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('"0xFF"');
    });

    test("octal numeric literal is quoted", () => {
      const content = `${PREAMBLE}---
spec: OctalSpec
INPUT:
- MUST: "0o77"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["OctalSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('"0o77"');
    });

    test("infinity literal is quoted", () => {
      const content = `${PREAMBLE}---
spec: InfSpec
INPUT:
- MUST: ".inf"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["InfSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('".inf"');
    });

    test("NaN literal is quoted", () => {
      const content = `${PREAMBLE}---
spec: NanSpec
INPUT:
- MUST: ".nan"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["NanSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('".nan"');
    });

    test("float literal is quoted", () => {
      const content = `${PREAMBLE}---
spec: FloatSpec
INPUT:
- MUST: "3.14"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["FloatSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('"3.14"');
    });

    test("tilde is quoted", () => {
      const content = `${PREAMBLE}---
spec: TildeSpec
INPUT:
- MUST: "~"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["TildeSpec"], tmp);
      expect(result.exitCode).toBe(0);
      // ~ is a YAML type token (null alias)
      expect(result.stdout).toContain('"~"');
    });

    test("leading ampersand is quoted", () => {
      const content = `${PREAMBLE}---
spec: AmpSpec
INPUT:
- MUST: "&anchor"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["AmpSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('"&anchor"');
    });

    test("leading exclamation mark is quoted", () => {
      const content = `${PREAMBLE}---
spec: BangSpec
INPUT:
- MUST: "!tag"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["BangSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('"!tag"');
    });

    test("leading pipe is quoted", () => {
      const content = `${PREAMBLE}---
spec: PipeSpec
INPUT:
- MUST: "|literal"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["PipeSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('"|literal"');
    });

    test("leading greater-than is quoted", () => {
      const content = `${PREAMBLE}---
spec: GtSpec
INPUT:
- MUST: ">folded"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["GtSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('">folded"');
    });

    test("leading percent is quoted", () => {
      const content = `${PREAMBLE}---
spec: PctSpec
INPUT:
- MUST: "%directive"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["PctSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('"%directive"');
    });

    test("leading at sign is quoted", () => {
      const content = `${PREAMBLE}---
spec: AtSpec
INPUT:
- MUST: "@mention"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["AtSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain('"@mention"');
    });
  });

  // =========================================================================
  // Cross-file CONFORMS: file@Spec::SLOT format (lines 261-263, 274-285,
  // 291-298, 426-437)
  // =========================================================================

  describe("cross-file CONFORMS resolution", () => {
    test("resolves file@Spec::SLOT ref with explicit .yass.yaml extension", () => {
      const baseContent = `${PREAMBLE}---
spec: BaseSpec
RETURN:
- MUST: return a string
`;
      const derivedContent = `${PREAMBLE}---
spec: DerivedSpec
RETURN:
- CONFORMS: ./base.yass.yaml@BaseSpec::RETURN
`;
      touch(join(tmp, "base.yass.yaml"), baseContent);
      touch(join(tmp, "derived.yass.yaml"), derivedContent);
      const result = run(["DerivedSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("MUST: return a string");
      expect(result.stdout).toContain("# CONFORMS: ./base.yass.yaml@BaseSpec::RETURN");
    });

    test("resolves file@Spec::SLOT ref with auto-appended .yass.yaml extension", () => {
      const baseContent = `${PREAMBLE}---
spec: BaseSpec
RETURN:
- MUST: return a string
`;
      const derivedContent = `${PREAMBLE}---
spec: DerivedSpec
RETURN:
- CONFORMS: ./base@BaseSpec::RETURN
`;
      touch(join(tmp, "base.yass.yaml"), baseContent);
      touch(join(tmp, "derived.yass.yaml"), derivedContent);
      const result = run(["DerivedSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("MUST: return a string");
    });

    test("resolves ../ relative file ref from spec directory", () => {
      const baseContent = `${PREAMBLE}---
spec: BaseSpec
RETURN:
- MUST: return a string
`;
      const derivedContent = `${PREAMBLE}---
spec: DerivedSpec
RETURN:
- CONFORMS: ../base.yass.yaml@BaseSpec::RETURN
`;
      mkdirSync(join(tmp, "sub"), { recursive: true });
      touch(join(tmp, "base.yass.yaml"), baseContent);
      touch(join(tmp, "sub", "derived.yass.yaml"), derivedContent);
      const result = run(["DerivedSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("MUST: return a string");
    });

    test("resolves project-root-anchored file ref (no ./ prefix)", () => {
      const baseContent = `${PREAMBLE}---
spec: BaseSpec
RETURN:
- MUST: return a string
`;
      const derivedContent = `${PREAMBLE}---
spec: DerivedSpec
RETURN:
- CONFORMS: base.yass.yaml@BaseSpec::RETURN
`;
      touch(join(tmp, "base.yass.yaml"), baseContent);
      mkdirSync(join(tmp, "sub"), { recursive: true });
      touch(join(tmp, "sub", "derived.yass.yaml"), derivedContent);
      const result = run(["DerivedSpec"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("MUST: return a string");
    });

    test("conforms_unresolved when cross-file ref points to missing file", () => {
      const derivedContent = `${PREAMBLE}---
spec: DerivedSpec
RETURN:
- CONFORMS: ./nonexistent.yass.yaml@BaseSpec::RETURN
`;
      touch(join(tmp, "derived.yass.yaml"), derivedContent);
      const result = run(["DerivedSpec"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stderr).toContain("yass.query.conforms_unresolved");
      expect(result.stdout).toBe("");
    });

    test("conforms_unresolved when cross-file spec name not found in target file", () => {
      const baseContent = `${PREAMBLE}---
spec: OtherSpec
RETURN:
- MUST: return something
`;
      const derivedContent = `${PREAMBLE}---
spec: DerivedSpec
RETURN:
- CONFORMS: ./base.yass.yaml@NonExistent::RETURN
`;
      touch(join(tmp, "base.yass.yaml"), baseContent);
      touch(join(tmp, "derived.yass.yaml"), derivedContent);
      const result = run(["DerivedSpec"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stderr).toContain("yass.query.conforms_unresolved");
    });

    test("conforms_unresolved when cross-file spec has no matching slot", () => {
      const baseContent = `${PREAMBLE}---
spec: BaseSpec
INPUT:
- MUST: accept input
`;
      const derivedContent = `${PREAMBLE}---
spec: DerivedSpec
RETURN:
- CONFORMS: ./base.yass.yaml@BaseSpec::RETURN
`;
      touch(join(tmp, "base.yass.yaml"), baseContent);
      touch(join(tmp, "derived.yass.yaml"), derivedContent);
      const result = run(["DerivedSpec"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stderr).toContain("yass.query.conforms_unresolved");
    });

    test("conforms_unresolved when cross-file slot value is not an array", () => {
      const baseContent = `${PREAMBLE}---
spec: BaseSpec
RETURN: not-a-list
`;
      const derivedContent = `${PREAMBLE}---
spec: DerivedSpec
RETURN:
- CONFORMS: ./base.yass.yaml@BaseSpec::RETURN
`;
      touch(join(tmp, "base.yass.yaml"), baseContent);
      touch(join(tmp, "derived.yass.yaml"), derivedContent);
      const result = run(["DerivedSpec"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stderr).toContain("yass.query.conforms_unresolved");
    });

    test("conforms_unresolved when cross-file ref target is unparseable YAML", () => {
      const badContent = `not: valid: yaml: [[[`;
      const derivedContent = `${PREAMBLE}---
spec: DerivedSpec
RETURN:
- CONFORMS: ./bad.yass.yaml@BaseSpec::RETURN
`;
      touch(join(tmp, "bad.yass.yaml"), badContent);
      touch(join(tmp, "derived.yass.yaml"), derivedContent);
      const result = run(["DerivedSpec"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stderr).toContain("yass.query.conforms_unresolved");
    });
  });

  // =========================================================================
  // CONFORMS: malformed ref (lines 414-420)
  // =========================================================================

  describe("CONFORMS malformed ref", () => {
    test("conforms_unresolved when ref has :: but empty slot name", () => {
      const content = `${PREAMBLE}---
spec: MalformedRef
RETURN:
- CONFORMS: "SomeSpec::"
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["MalformedRef"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stderr).toContain("yass.query.conforms_unresolved");
      expect(result.stdout).toBe("");
    });
  });

  // =========================================================================
  // CONFORMS inlining: inner WHEN only (line 505)
  // =========================================================================

  describe("CONFORMS inlining: inner WHEN only", () => {
    test("inner WHEN is preserved when carrier has no WHEN", () => {
      const content = `${PREAMBLE}---
spec: BaseSpec
RETURN:
- WHEN: the input is valid
  MUST: return a string
---
spec: NoGuardDerived
RETURN:
- CONFORMS: BaseSpec::RETURN
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["NoGuardDerived"], tmp);
      expect(result.exitCode).toBe(0);
      // The inner WHEN should be preserved as-is (no combination)
      expect(result.stdout).toContain("WHEN: the input is valid");
      expect(result.stdout).toContain("MUST: return a string");
    });
  });

  // =========================================================================
  // CONFORMS inlining: reference keys on inlined obligations (line 511)
  // =========================================================================

  describe("CONFORMS inlining: reference keys preserved", () => {
    test("USES key on inlined obligation is preserved", () => {
      const content = `${PREAMBLE}---
spec: BaseSpec
RETURN:
- MUST: return a string
  USES: SomeHelper
---
spec: RefDerived
RETURN:
- CONFORMS: BaseSpec::RETURN
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["RefDerived"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("USES: SomeHelper");
      expect(result.stdout).toContain("MUST: return a string");
    });

    test("SEE key on inlined obligation is preserved", () => {
      const content = `${PREAMBLE}---
spec: BaseSpec
RETURN:
- MUST: return a string
  SEE: SomeOtherSpec
---
spec: SeeDerived
RETURN:
- CONFORMS: BaseSpec::RETURN
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["SeeDerived"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("SEE: SomeOtherSpec");
    });
  });

  // =========================================================================
  // CONFORMS inlining: remaining keys (line 518)
  // =========================================================================

  describe("CONFORMS inlining: remaining keys on inlined obligations", () => {
    test("inlining succeeds when base obligation has non-standard keys alongside recognized ones", () => {
      // This exercises the remaining-keys loop (lines 516-519) in inlineConforms.
      // custom_key is not normativity, WHEN, or a reference keyword, so it goes
      // through the remaining-keys loop. The emitter only outputs orderedKeys
      // (normativity, WHEN, refs), so custom_key won't appear in stdout, but
      // the code path is exercised.
      const content = `${PREAMBLE}---
spec: BaseSpec
RETURN:
- MUST: return a string
  custom_key: custom_value
---
spec: RemainingKeysDerived
RETURN:
- CONFORMS: BaseSpec::RETURN
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["RemainingKeysDerived"], tmp);
      expect(result.exitCode).toBe(0);
      expect(result.stdout).toContain("MUST: return a string");
      expect(result.stdout).toContain("# CONFORMS: BaseSpec::RETURN");
    });
  });

  // =========================================================================
  // Scope validation: discover error (lines 670-681)
  // =========================================================================

  describe("scope validation discover errors", () => {
    test("scope_not_found when discover returns an error for scope", () => {
      // Create a file that exists but is not a valid .yass.yaml
      // (a file with bad extension as scope should trigger discover error)
      const badFile = join(tmp, "notaspec.txt");
      touch(badFile, "hello");
      const result = run(["SomeSpec", badFile], tmp);
      expect(result.exitCode).toBe(2);
      expect(result.stderr).toContain("yass.query.scope_not_found");
    });
  });

  // =========================================================================
  // Multi-match disambiguation: description normalization (line 745)
  // =========================================================================

  describe("multi-match disambiguation description handling", () => {
    test("includes description from preamble in disambiguation rows", () => {
      const file1 = `---
description: First spec file description.
version: v1
---
spec: Alpha.Spec
INPUT:
- MUST: accept input
`;
      const file2 = `---
description: Second spec file description.
version: v1
---
spec: Beta.Spec
INPUT:
- MUST: accept input
`;
      touch(join(tmp, "alpha.yass.yaml"), file1);
      touch(join(tmp, "beta.yass.yaml"), file2);

      const result = run(["Spec"], tmp);
      expect(result.exitCode).toBe(0);
      const lines = result.stdout.trim().split("\n");
      expect(lines.length).toBe(2);
      // Check descriptions are in the third tab-separated field
      const fields0 = lines[0]!.split("\t");
      const fields1 = lines[1]!.split("\t");
      expect(fields0[2]).toBeDefined();
      expect(fields1[2]).toBeDefined();
      // One should have "First..." and the other "Second..."
      const descriptions = [fields0[2], fields1[2]].sort();
      expect(descriptions[0]).toBe("First spec file description.");
      expect(descriptions[1]).toBe("Second spec file description.");
    });

    test("empty description when preamble has no description key", () => {
      // A file where the preamble (first doc without spec) has no description
      const file1 = `---
version: v1
---
spec: Alpha.Spec
INPUT:
- MUST: accept input
`;
      const file2 = `---
description: Has description.
version: v1
---
spec: Beta.Spec
INPUT:
- MUST: accept input
`;
      touch(join(tmp, "alpha.yass.yaml"), file1);
      touch(join(tmp, "beta.yass.yaml"), file2);

      const result = run(["Spec"], tmp);
      expect(result.exitCode).toBe(0);
      const lines = result.stdout.trim().split("\n");
      expect(lines.length).toBe(2);
      // The file without description should have empty third field
      const alphaLine = lines.find((l) => l.includes("alpha.yass.yaml"));
      expect(alphaLine).toBeDefined();
      const fields = alphaLine!.split("\t");
      expect(fields[2]).toBe("");
    });

    test("multiline description is normalized to single line", () => {
      const file1 = `---
description: "First line\n  second line\n  third line"
version: v1
---
spec: Alpha.Spec
INPUT:
- MUST: accept input
`;
      const file2 = `---
description: Normal desc.
version: v1
---
spec: Beta.Spec
INPUT:
- MUST: accept input
`;
      touch(join(tmp, "alpha.yass.yaml"), file1);
      touch(join(tmp, "beta.yass.yaml"), file2);

      const result = run(["Spec"], tmp);
      expect(result.exitCode).toBe(0);
      const lines = result.stdout.trim().split("\n");
      const alphaLine = lines.find((l) => l.includes("alpha.yass.yaml"));
      expect(alphaLine).toBeDefined();
      const fields = alphaLine!.split("\t");
      // All whitespace runs should be collapsed to single space
      expect(fields[2]).not.toContain("\n");
      expect(fields[2]).toBe("First line second line third line");
    });
  });

  // =========================================================================
  // CONFORMS with WHEN: carrier + inner WHEN combination (complete coverage)
  // =========================================================================

  describe("CONFORMS WHEN guard combination edge cases", () => {
    test("carrier WHEN applied to multiple inlined obligations, some with inner WHEN", () => {
      const content = `${PREAMBLE}---
spec: BaseSpec
RETURN:
- MUST: return a string
- WHEN: input is valid
  MUST: return validated string
---
spec: MixedGuardDerived
RETURN:
- WHEN: the mode is strict
  CONFORMS: BaseSpec::RETURN
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["MixedGuardDerived"], tmp);
      expect(result.exitCode).toBe(0);
      // First obligation: carrier WHEN only (no inner WHEN)
      expect(result.stdout).toContain("WHEN: the mode is strict");
      // Second obligation: combined WHEN
      expect(result.stdout).toContain("WHEN: the mode is strict and input is valid");
    });
  });

  // =========================================================================
  // CONFORMS: normative with WHEN and CONFORMS (combined coverage)
  // =========================================================================

  describe("CONFORMS normative with WHEN guard", () => {
    test("normative carrier with WHEN: keeps carrier then appends inlined with combined guards", () => {
      const content = `${PREAMBLE}---
spec: BaseSpec
RETURN:
- WHEN: data is ready
  MUST: return data
---
spec: NormGuardDerived
RETURN:
- MUST: do normative thing
  WHEN: in strict mode
  CONFORMS: BaseSpec::RETURN
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["NormGuardDerived"], tmp);
      expect(result.exitCode).toBe(0);
      // Carrier obligation is kept (without CONFORMS key)
      expect(result.stdout).toContain("MUST: do normative thing");
      // Inlined obligation gets combined WHEN
      expect(result.stdout).toContain("WHEN: in strict mode and data is ready");
      // The original normative should appear before inlined
      const stdout = result.stdout;
      const normIdx = stdout.indexOf("do normative thing");
      const inlinedIdx = stdout.indexOf("return data");
      expect(normIdx).toBeLessThan(inlinedIdx);
    });
  });

  // =========================================================================
  // Multiple CONFORMS errors in same spec
  // =========================================================================

  describe("multiple CONFORMS errors", () => {
    test("emits all CONFORMS errors to stderr", () => {
      const content = `${PREAMBLE}---
spec: MultiErrorSpec
RETURN:
- CONFORMS: NonExistent1::RETURN
- CONFORMS: NonExistent2::INPUT
`;
      touch(join(tmp, "spec.yass.yaml"), content);
      const result = run(["MultiErrorSpec"], tmp);
      expect(result.exitCode).toBe(1);
      expect(result.stdout).toBe("");
      // Both errors should be reported
      expect(result.stderr).toContain("NonExistent1::RETURN");
      expect(result.stderr).toContain("NonExistent2::INPUT");
    });
  });

  // =========================================================================
  // No project root marker (lines 626-635)
  // =========================================================================

  describe("no project root", () => {
    test("errors with findroot_no_marker when no .git directory exists", () => {
      // Create a tmpdir WITHOUT .git to trigger findProjectRoot failure
      const noRootDir = join(
        tmpdir(),
        `yass-noroot-${Date.now()}-${Math.random().toString(36).slice(2)}`,
      );
      mkdirSync(noRootDir, { recursive: true });
      try {
        const result = run(["SomeSpec"], noRootDir);
        expect(result.exitCode).toBe(2);
        expect(result.stderr).toContain("yass.findroot.no_marker");
        expect(result.stdout).toBe("");
      } finally {
        rmSync(noRootDir, { recursive: true, force: true });
      }
    });
  });
});
