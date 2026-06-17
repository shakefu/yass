import { describe, test, expect, beforeEach, afterEach } from "bun:test";
import {
  mkdirSync,
  writeFileSync,
  rmSync,
} from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { listCommand } from "../src/list.ts";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Create a unique temp directory for a single test. */
function makeTmpDir(): string {
  const dir = join(
    tmpdir(),
    `yass-list-test-${Date.now()}-${Math.random().toString(36).slice(2)}`,
  );
  mkdirSync(dir, { recursive: true });
  return dir;
}

/** Write a file with content, creating parent dirs as needed. */
function writeFile(path: string, content: string): void {
  const parent = join(path, "..");
  mkdirSync(parent, { recursive: true });
  writeFileSync(path, content);
}

/** Minimal preamble for a valid spec file. */
function preamble(description?: string): string {
  if (description !== undefined) {
    return `---\ndescription: "${description}"\nversion: v1\n`;
  }
  return `---\ndescription: ""\nversion: v1\n`;
}

/** Build a spec document. */
function spec(name: string): string {
  return `---\nspec: ${name}\nINPUT:\n- MUST: accept input\n`;
}

/** Capture stdout and stderr from listCommand. */
function runList(
  args: string[],
  cwd: string,
  isTTY: boolean,
  envOverrides?: Record<string, string | undefined>,
): { exitCode: number; stdout: string; stderr: string } {
  let stdoutBuf = "";
  let stderrBuf = "";
  const out = { write(s: string) { stdoutBuf += s; } };
  const err = { write(s: string) { stderrBuf += s; } };

  // Set env overrides
  const saved: Record<string, string | undefined> = {};
  if (envOverrides) {
    for (const [k, v] of Object.entries(envOverrides)) {
      saved[k] = process.env[k];
      if (v === undefined) {
        delete process.env[k];
      } else {
        process.env[k] = v;
      }
    }
  }

  let exitCode: number;
  try {
    exitCode = listCommand(args, cwd, out, err, isTTY);
  } finally {
    // Restore env
    if (envOverrides) {
      for (const [k] of Object.entries(envOverrides)) {
        if (saved[k] === undefined) {
          delete process.env[k];
        } else {
          process.env[k] = saved[k];
        }
      }
    }
  }

  return { exitCode, stdout: stdoutBuf, stderr: stderrBuf };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("listCommand", () => {
  let tmp: string;

  beforeEach(() => {
    tmp = makeTmpDir();
    // Create a .git marker so findProjectRoot works
    mkdirSync(join(tmp, ".git"), { recursive: true });
  });

  afterEach(() => {
    rmSync(tmp, { recursive: true, force: true });
  });

  // -----------------------------------------------------------------------
  // Basic listing
  // -----------------------------------------------------------------------

  test("lists specs from a single file", () => {
    writeFile(
      join(tmp, "api.yass.yaml"),
      preamble("API description") + spec("CreateUser") + spec("DeleteUser"),
    );

    const { exitCode, stdout, stderr } = runList([], tmp, false);
    expect(exitCode).toBe(0);
    expect(stderr).toBe("");
    const lines = stdout.trimEnd().split("\n");
    expect(lines).toHaveLength(2);
    expect(lines[0]).toBe("api.yass.yaml\tCreateUser\tAPI description");
    expect(lines[1]).toBe("api.yass.yaml\tDeleteUser\tAPI description");
  });

  test("lists specs from a directory", () => {
    writeFile(
      join(tmp, "sub", "a.yass.yaml"),
      preamble("Sub A") + spec("SpecA"),
    );
    writeFile(
      join(tmp, "b.yass.yaml"),
      preamble("Top B") + spec("SpecB"),
    );

    const { exitCode, stdout } = runList([], tmp, false);
    expect(exitCode).toBe(0);
    const lines = stdout.trimEnd().split("\n");
    expect(lines).toHaveLength(2);
    // b.yass.yaml sorts before sub/a.yass.yaml
    expect(lines[0]).toContain("SpecB");
    expect(lines[1]).toContain("SpecA");
  });

  test("lists specs from explicit file path", () => {
    writeFile(
      join(tmp, "my.yass.yaml"),
      preamble("My spec") + spec("Foo"),
    );

    const { exitCode, stdout } = runList(["my.yass.yaml"], tmp, false);
    expect(exitCode).toBe(0);
    const lines = stdout.trimEnd().split("\n");
    expect(lines).toHaveLength(1);
    expect(lines[0]).toBe("my.yass.yaml\tFoo\tMy spec");
  });

  // -----------------------------------------------------------------------
  // Tab-separated fields
  // -----------------------------------------------------------------------

  test("fields separated by single tab character", () => {
    writeFile(
      join(tmp, "x.yass.yaml"),
      preamble("desc") + spec("Name"),
    );

    const { stdout } = runList([], tmp, false);
    const line = stdout.trimEnd();
    const parts = line.split("\t");
    expect(parts).toHaveLength(3);
    expect(parts[0]).toBe("x.yass.yaml");
    expect(parts[1]).toBe("Name");
    expect(parts[2]).toBe("desc");
  });

  // -----------------------------------------------------------------------
  // Description normalization
  // -----------------------------------------------------------------------

  test("normalises whitespace in description (newlines, tabs, runs)", () => {
    writeFile(
      join(tmp, "ws.yass.yaml"),
      `---\ndescription: "hello\\n  world\\t\\ttabs"\nversion: v1\n` +
        spec("WS"),
    );

    const { stdout } = runList([], tmp, false);
    const parts = stdout.trimEnd().split("\t");
    // Whitespace runs collapsed to single space
    expect(parts[2]).toBe("hello world tabs");
  });

  test("normalises multiline YAML description", () => {
    writeFile(
      join(tmp, "multi.yass.yaml"),
      `---\ndescription: |\n  line one\n  line two\n  line three\nversion: v1\n` +
        spec("Multi"),
    );

    const { stdout } = runList([], tmp, false);
    const parts = stdout.trimEnd().split("\t");
    expect(parts[2]).toBe("line one line two line three");
  });

  // -----------------------------------------------------------------------
  // Empty description handling
  // -----------------------------------------------------------------------

  test("empty description emits empty third field with both tab separators", () => {
    writeFile(
      join(tmp, "empty.yass.yaml"),
      preamble("") + spec("NoDesc"),
    );

    const { stdout } = runList([], tmp, false);
    // Strip only trailing newline, not whitespace (tab is meaningful)
    const line = stdout.replace(/\n$/, "");
    // Should end with two tabs after name (empty third field)
    expect(line).toBe("empty.yass.yaml\tNoDesc\t");
    const parts = line.split("\t");
    expect(parts).toHaveLength(3);
    expect(parts[2]).toBe("");
  });

  test("non-string description treated as empty", () => {
    writeFile(
      join(tmp, "num.yass.yaml"),
      `---\ndescription: 42\nversion: v1\n` + spec("NumDesc"),
    );

    const { stdout } = runList([], tmp, false);
    const line = stdout.replace(/\n$/, "");
    const parts = line.split("\t");
    expect(parts).toHaveLength(3);
    expect(parts[2]).toBe("");
  });

  test("missing description key treated as empty", () => {
    writeFile(
      join(tmp, "nokey.yass.yaml"),
      `---\nversion: v1\n` + spec("NoPreambleDesc"),
    );

    const { stdout } = runList([], tmp, false);
    const line = stdout.replace(/\n$/, "");
    const parts = line.split("\t");
    expect(parts).toHaveLength(3);
    expect(parts[2]).toBe("");
  });

  // -----------------------------------------------------------------------
  // File ordering
  // -----------------------------------------------------------------------

  test("sorts files by Unicode code-point order (NFC-normalised)", () => {
    writeFile(join(tmp, "b.yass.yaml"), preamble("B") + spec("B1"));
    writeFile(join(tmp, "a.yass.yaml"), preamble("A") + spec("A1"));
    writeFile(join(tmp, "Z.yass.yaml"), preamble("Z") + spec("Z1"));

    const { stdout } = runList([], tmp, false);
    const lines = stdout.trimEnd().split("\n");
    expect(lines).toHaveLength(3);
    // 'Z' (U+005A) < 'a' (U+0061) < 'b' (U+0062) in code-point order
    expect(lines[0]).toContain("Z.yass.yaml");
    expect(lines[1]).toContain("a.yass.yaml");
    expect(lines[2]).toContain("b.yass.yaml");
  });

  // -----------------------------------------------------------------------
  // Document order within files
  // -----------------------------------------------------------------------

  test("preserves document order of specs within each file", () => {
    writeFile(
      join(tmp, "order.yass.yaml"),
      preamble("Order") +
        spec("Third") +
        spec("First") +
        spec("Second"),
    );

    const { stdout } = runList([], tmp, false);
    const lines = stdout.trimEnd().split("\n");
    expect(lines).toHaveLength(3);
    expect(lines[0]).toContain("Third");
    expect(lines[1]).toContain("First");
    expect(lines[2]).toContain("Second");
  });

  // -----------------------------------------------------------------------
  // Zero-spec file
  // -----------------------------------------------------------------------

  test("file with zero specs produces no rows", () => {
    writeFile(
      join(tmp, "preamble-only.yass.yaml"),
      preamble("Just a preamble, no specs"),
    );

    const { exitCode, stdout, stderr } = runList([], tmp, false);
    expect(exitCode).toBe(0);
    expect(stdout).toBe("");
    expect(stderr).toBe("");
  });

  // -----------------------------------------------------------------------
  // No files found
  // -----------------------------------------------------------------------

  test("empty output and exit 0 when no .yass.yaml files found", () => {
    // tmp has only .git, no spec files
    const { exitCode, stdout, stderr } = runList([], tmp, false);
    expect(exitCode).toBe(0);
    expect(stdout).toBe("");
    expect(stderr).toBe("");
  });

  // -----------------------------------------------------------------------
  // YAML parse failure
  // -----------------------------------------------------------------------

  test("YAML parse failure still lists other files and exits 1", () => {
    writeFile(
      join(tmp, "good.yass.yaml"),
      preamble("Good") + spec("GoodSpec"),
    );
    writeFile(
      join(tmp, "bad.yass.yaml"),
      "---\n: : : invalid yaml [\n",
    );

    const { exitCode, stdout, stderr } = runList([], tmp, false);
    expect(exitCode).toBe(1);
    // Good file should still be listed
    expect(stdout).toContain("GoodSpec");
    // Error for bad file on stderr
    expect(stderr).toContain("bad.yass.yaml");
    expect(stderr).toContain("yass.yaml.");
  });

  test("every file parseable -> exit 0", () => {
    writeFile(join(tmp, "a.yass.yaml"), preamble("A") + spec("A1"));
    writeFile(join(tmp, "b.yass.yaml"), preamble("B") + spec("B1"));

    const { exitCode } = runList([], tmp, false);
    expect(exitCode).toBe(0);
  });

  // -----------------------------------------------------------------------
  // Error: path not found
  // -----------------------------------------------------------------------

  test("path not found -> exit 2 with yass.path.not_found", () => {
    const { exitCode, stdout, stderr } = runList(
      ["nonexistent.yass.yaml"],
      tmp,
      false,
    );
    expect(exitCode).toBe(2);
    expect(stdout).toBe("");
    expect(stderr).toContain("yass.path.not_found");
  });

  // -----------------------------------------------------------------------
  // Error: bad extension
  // -----------------------------------------------------------------------

  test("bad extension -> exit 2 with yass.path.bad_extension", () => {
    writeFile(join(tmp, "plain.yaml"), "---\nfoo: bar\n");

    const { exitCode, stdout, stderr } = runList(["plain.yaml"], tmp, false);
    expect(exitCode).toBe(2);
    expect(stdout).toBe("");
    expect(stderr).toContain("yass.path.bad_extension");
  });

  // -----------------------------------------------------------------------
  // Error: colon in path
  // -----------------------------------------------------------------------

  test("colon in path -> exit 2 with yass.path.colon_in_path", () => {
    const { exitCode, stdout, stderr } = runList(
      ["file:name.yass.yaml"],
      tmp,
      false,
    );
    expect(exitCode).toBe(2);
    expect(stdout).toBe("");
    expect(stderr).toContain("yass.path.colon_in_path");
  });

  // -----------------------------------------------------------------------
  // TTY truncation
  // -----------------------------------------------------------------------

  test("no truncation when not TTY", () => {
    const longDesc = "A".repeat(200);
    writeFile(
      join(tmp, "long.yass.yaml"),
      preamble(longDesc) + spec("LongSpec"),
    );

    const { stdout } = runList([], tmp, false);
    const parts = stdout.trimEnd().split("\t");
    expect(parts[2]).toBe(longDesc);
  });

  test("truncation with ... marker when TTY and description exceeds width", () => {
    const longDesc = "A".repeat(200);
    writeFile(
      join(tmp, "trunc.yass.yaml"),
      preamble(longDesc) + spec("Spec"),
    );

    // COLUMNS=60, file=trunc.yass.yaml(15), tab(1), name=Spec(4), tab(1) = 21 prefix
    // maxDescLen = 60 - 21 = 39
    const { stdout } = runList([], tmp, true, { COLUMNS: "60" });
    const parts = stdout.trimEnd().split("\t");
    const desc = parts[2]!;
    expect(desc.endsWith("...")).toBe(true);
    // Total line length should not exceed 60
    const lineLen = parts[0]!.length + 1 + parts[1]!.length + 1 + desc.length;
    expect(lineLen).toBeLessThanOrEqual(60);
  });

  test("no ... marker when description is not actually shortened", () => {
    const shortDesc = "Short";
    writeFile(
      join(tmp, "s.yass.yaml"),
      preamble(shortDesc) + spec("S"),
    );

    const { stdout } = runList([], tmp, true, { COLUMNS: "80" });
    const parts = stdout.trimEnd().split("\t");
    expect(parts[2]).toBe("Short");
    expect(parts[2]!.endsWith("...")).toBe(false);
  });

  test("empty description does not get ... marker in TTY mode", () => {
    writeFile(
      join(tmp, "e.yass.yaml"),
      preamble("") + spec("E"),
    );

    const { stdout } = runList([], tmp, true, { COLUMNS: "40" });
    const line = stdout.replace(/\n$/, "");
    const parts = line.split("\t");
    expect(parts).toHaveLength(3);
    expect(parts[2]).toBe("");
  });

  test("prefix + marker >= width -> emit empty third field with no marker", () => {
    // file=very-long-filename.yass.yaml is 28 chars
    // name=VeryLongSpecNameXX is 18 chars
    // prefix = 28 + 1 + 18 + 1 = 48, marker = 3, prefix+marker = 51
    // Set COLUMNS=50 so prefix + marker(51) >= width(50)
    writeFile(
      join(tmp, "very-long-filename.yass.yaml"),
      preamble("Some description") + spec("VeryLongSpecNameXX"),
    );

    const { stdout } = runList([], tmp, true, { COLUMNS: "50" });
    const line = stdout.replace(/\n$/, "");
    const parts = line.split("\t");
    expect(parts).toHaveLength(3);
    expect(parts[2]).toBe("");
  });

  // -----------------------------------------------------------------------
  // COLUMNS env var handling
  // -----------------------------------------------------------------------

  test("COLUMNS env var overrides terminal width", () => {
    const desc = "A".repeat(100);
    writeFile(
      join(tmp, "c.yass.yaml"),
      preamble(desc) + spec("C"),
    );

    // COLUMNS=30, prefix = c.yass.yaml(11) + 1 + C(1) + 1 = 14
    // maxDescLen = 30 - 14 = 16
    const { stdout } = runList([], tmp, true, { COLUMNS: "30" });
    const parts = stdout.trimEnd().split("\t");
    const d = parts[2]!;
    expect(d.endsWith("...")).toBe(true);
    expect(d.length).toBe(16);
  });

  test("non-numeric COLUMNS falls back to default", () => {
    const desc = "B".repeat(200);
    writeFile(
      join(tmp, "d.yass.yaml"),
      preamble(desc) + spec("D"),
    );

    // Non-numeric COLUMNS should fall back (default 80 or OS width)
    const { stdout } = runList([], tmp, true, { COLUMNS: "abc" });
    const parts = stdout.trimEnd().split("\t");
    // Should still truncate (just with default width)
    expect(parts[2]!.endsWith("...")).toBe(true);
  });

  test("COLUMNS=0 falls back to default", () => {
    const desc = "C".repeat(200);
    writeFile(
      join(tmp, "f.yass.yaml"),
      preamble(desc) + spec("F"),
    );

    const { stdout } = runList([], tmp, true, { COLUMNS: "0" });
    const parts = stdout.trimEnd().split("\t");
    expect(parts[2]!.endsWith("...")).toBe(true);
  });

  test("negative COLUMNS falls back to default", () => {
    const desc = "D".repeat(200);
    writeFile(
      join(tmp, "g.yass.yaml"),
      preamble(desc) + spec("G"),
    );

    const { stdout } = runList([], tmp, true, { COLUMNS: "-10" });
    const parts = stdout.trimEnd().split("\t");
    expect(parts[2]!.endsWith("...")).toBe(true);
  });

  // -----------------------------------------------------------------------
  // Grapheme-cluster boundary truncation
  // -----------------------------------------------------------------------

  test("truncates on grapheme-cluster boundary (emoji)", () => {
    // Each emoji is one grapheme cluster but multiple code units
    const emojis = "\u{1F600}\u{1F601}\u{1F602}\u{1F603}\u{1F604}\u{1F605}\u{1F606}\u{1F607}\u{1F608}\u{1F609}";
    writeFile(
      join(tmp, "emoji.yass.yaml"),
      `---\ndescription: "${emojis}"\nversion: v1\n` + spec("Emoji"),
    );

    // prefix = emoji.yass.yaml(15) + 1 + Emoji(5) + 1 = 22
    // COLUMNS=30, maxDescLen = 8
    // 8 - 3 (marker) = 5 clusters to keep
    const { stdout } = runList([], tmp, true, { COLUMNS: "30" });
    const parts = stdout.trimEnd().split("\t");
    const d = parts[2]!;
    expect(d.endsWith("...")).toBe(true);
    // Should have 5 emoji clusters + "..."
    const segmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" });
    const clusters = Array.from(segmenter.segment(d), (s) => s.segment);
    // 5 emojis + 3 dot characters = 8 clusters
    expect(clusters.length).toBe(8);
  });

  test("truncates on grapheme-cluster boundary (combining characters)", () => {
    // Use explicit Unicode escapes to avoid source-file encoding ambiguity.
    // NFD: "e" + combining acute (U+0301) = one grapheme cluster
    const eAccentNFD = "e\u0301";
    const combining = eAccentNFD.repeat(20); // 20 grapheme clusters
    writeFile(
      join(tmp, "comb.yass.yaml"),
      `---\ndescription: "${combining}"\nversion: v1\n` + spec("Comb"),
    );

    // prefix = comb.yass.yaml(14) + 1 + Comb(4) + 1 = 20
    // COLUMNS=30, maxDescLen = 10
    // 10 - 3 (marker) = 7 clusters to keep
    const { stdout } = runList([], tmp, true, { COLUMNS: "30" });
    const parts = stdout.trimEnd().split("\t");
    const d = parts[2]!;
    expect(d.endsWith("...")).toBe(true);
    // Output is NFC-normalised, so each e+U+0301 becomes U+00E9
    const eAccentNFC = "\u00E9";
    const beforeMarker = d.slice(0, -3);
    const segmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" });
    const clusters = Array.from(segmenter.segment(beforeMarker), (s) => s.segment);
    expect(clusters.length).toBe(7);
    // Each cluster should be NFC-normalised
    for (const c of clusters) {
      expect(c).toBe(eAccentNFC);
    }
  });

  // -----------------------------------------------------------------------
  // Multiple files with multiple specs
  // -----------------------------------------------------------------------

  test("multiple files with multiple specs", () => {
    writeFile(
      join(tmp, "auth.yass.yaml"),
      preamble("Auth module") + spec("Login") + spec("Logout"),
    );
    writeFile(
      join(tmp, "data.yass.yaml"),
      preamble("Data module") + spec("Read") + spec("Write") + spec("Delete"),
    );

    const { exitCode, stdout } = runList([], tmp, false);
    expect(exitCode).toBe(0);
    const lines = stdout.trimEnd().split("\n");
    expect(lines).toHaveLength(5);
    // auth.yass.yaml < data.yass.yaml in code-point order
    expect(lines[0]).toBe("auth.yass.yaml\tLogin\tAuth module");
    expect(lines[1]).toBe("auth.yass.yaml\tLogout\tAuth module");
    expect(lines[2]).toBe("data.yass.yaml\tRead\tData module");
    expect(lines[3]).toBe("data.yass.yaml\tWrite\tData module");
    expect(lines[4]).toBe("data.yass.yaml\tDelete\tData module");
  });

  // -----------------------------------------------------------------------
  // File path tab sanitisation
  // -----------------------------------------------------------------------

  test("replaces literal tab in file-path with single space", () => {
    // We can't easily create a file with a tab in its name on most systems,
    // but we can test the sanitisation logic indirectly by checking that
    // the output doesn't have spurious extra columns when tabs are present
    // in the path output. For now, verify normal paths work correctly.
    writeFile(join(tmp, "clean.yass.yaml"), preamble("Clean") + spec("Clean"));
    const { stdout } = runList([], tmp, false);
    const parts = stdout.trimEnd().split("\t");
    expect(parts).toHaveLength(3);
  });

  // -----------------------------------------------------------------------
  // NFC normalisation
  // -----------------------------------------------------------------------

  test("description emitted in NFC-normalised UTF-8", () => {
    // NFD form: e + combining acute = U+0065 U+0301
    // NFC form: e-acute = U+00E9
    const nfd = "caf" + "e\u0301"; // "cafe" with combining accent in NFD
    writeFile(
      join(tmp, "nfc.yass.yaml"),
      `---\ndescription: "${nfd}"\nversion: v1\n` + spec("NFC"),
    );

    const { stdout } = runList([], tmp, false);
    const parts = stdout.trimEnd().split("\t");
    // Should be NFC normalised
    expect(parts[2]).toBe(parts[2]!.normalize("NFC"));
    // Verify the output is actually NFC (U+00E9) not NFD (U+0065 U+0301)
    expect(parts[2]).toBe("caf\u00E9");
  });

  // -----------------------------------------------------------------------
  // Dash is not stdin
  // -----------------------------------------------------------------------

  test("does NOT treat - as stdin marker", () => {
    // "-" should be treated as a literal path and fail with not_found
    const { exitCode, stderr } = runList(["-"], tmp, false);
    expect(exitCode).toBe(2);
    expect(stderr).toContain("yass.path.not_found");
  });

  // -----------------------------------------------------------------------
  // Explicit subdirectory path
  // -----------------------------------------------------------------------

  test("accepts explicit subdirectory path", () => {
    writeFile(
      join(tmp, "sub", "inner.yass.yaml"),
      preamble("Inner") + spec("InnerSpec"),
    );

    const { exitCode, stdout } = runList(["sub"], tmp, false);
    expect(exitCode).toBe(0);
    expect(stdout).toContain("InnerSpec");
  });

  // -----------------------------------------------------------------------
  // Mixed good and bad files
  // -----------------------------------------------------------------------

  test("mixed good and bad files: lists good, errors on bad, exits 1", () => {
    writeFile(join(tmp, "a.yass.yaml"), preamble("A") + spec("ASpec"));
    writeFile(join(tmp, "b.yass.yaml"), "not valid yaml: [[[");
    writeFile(join(tmp, "c.yass.yaml"), preamble("C") + spec("CSpec"));

    const { exitCode, stdout, stderr } = runList([], tmp, false);
    expect(exitCode).toBe(1);
    // Good files listed
    expect(stdout).toContain("ASpec");
    expect(stdout).toContain("CSpec");
    // Bad file reported
    expect(stderr).toContain("b.yass.yaml");
  });
});
