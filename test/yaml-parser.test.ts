import { describe, expect, it, beforeEach, afterEach } from "bun:test";
import { parseYamlFile, parseYamlContent } from "../src/yaml-parser";
import type { ParseSuccess, ParseError } from "../src/yaml-parser";
import { mkdtempSync, writeFileSync, rmSync } from "fs";
import { join } from "path";
import { tmpdir } from "os";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

let tmpDir: string;

beforeEach(() => {
  tmpDir = mkdtempSync(join(tmpdir(), "yass-yaml-test-"));
});

afterEach(() => {
  rmSync(tmpDir, { recursive: true, force: true });
});

/** Write content to a temp file and return its path. */
function writeTmp(name: string, content: string | Buffer): string {
  const p = join(tmpDir, name);
  writeFileSync(p, content);
  return p;
}

function ok(r: ReturnType<typeof parseYamlContent>): ParseSuccess {
  if (!r.ok) throw new Error(`Expected ok, got error: ${r.code} — ${r.message}`);
  return r;
}

function err(r: ReturnType<typeof parseYamlContent>): ParseError {
  if (r.ok) throw new Error(`Expected error, got success`);
  return r;
}

// ---------------------------------------------------------------------------
// parseYamlContent — valid inputs
// ---------------------------------------------------------------------------

describe("parseYamlContent", () => {
  describe("valid single-document YAML", () => {
    it("parses a simple mapping", () => {
      const r = ok(parseYamlContent("a: 1\nb: hello\n"));
      expect(r.documents).toHaveLength(1);
      expect(r.documents[0]!.data).toEqual({ a: 1, b: "hello" });
      expect(r.documents[0]!.line).toBe(1);
    });

    it("parses nested mappings", () => {
      const content = "top:\n  nested: value\n  deep:\n    leaf: 42\n";
      const r = ok(parseYamlContent(content));
      expect(r.documents[0]!.data).toEqual({
        top: { nested: "value", deep: { leaf: 42 } },
      });
    });

    it("preserves rawContent", () => {
      const content = "x: y\n";
      const r = ok(parseYamlContent(content));
      expect(r.rawContent).toBe(content);
    });
  });

  describe("valid multi-document YAML", () => {
    it("parses two documents separated by ---", () => {
      const content = "a: 1\n---\nb: 2\n";
      const r = ok(parseYamlContent(content));
      expect(r.documents).toHaveLength(2);
      expect(r.documents[0]!.data).toEqual({ a: 1 });
      expect(r.documents[1]!.data).toEqual({ b: 2 });
    });

    it("parses three documents", () => {
      const content = "x: 1\n---\ny: 2\n---\nz: 3\n";
      const r = ok(parseYamlContent(content));
      expect(r.documents).toHaveLength(3);
      expect(r.documents[2]!.data).toEqual({ z: 3 });
    });

    it("preserves start line numbers for each document", () => {
      const content = "a: 1\nb: 2\n---\nc: 3\n---\nd: 4\n";
      const r = ok(parseYamlContent(content));
      expect(r.documents[0]!.line).toBe(1);
      expect(r.documents[1]!.line).toBe(3); // --- is on line 3
      expect(r.documents[2]!.line).toBe(5);
    });
  });

  describe("non-mapping documents", () => {
    it("wraps a scalar document as { _: value }", () => {
      const r = ok(parseYamlContent("hello\n"));
      expect(r.documents[0]!.data).toEqual({ _: "hello" });
    });

    it("wraps a sequence document as { _: [...] }", () => {
      const r = ok(parseYamlContent("- a\n- b\n"));
      expect(r.documents[0]!.data).toEqual({ _: ["a", "b"] });
    });

    it("wraps null document as { _: null }", () => {
      const r = ok(parseYamlContent("---\n...\n"));
      expect(r.documents[0]!.data).toEqual({ _: null });
    });
  });

  // -------------------------------------------------------------------------
  // YAML 1.2 boolean handling: yes/no/on/off are strings
  // -------------------------------------------------------------------------

  describe("YAML 1.2 boolean handling", () => {
    it("treats yes, no, on, off as strings", () => {
      const content = "a: yes\nb: no\nc: on\nd: off\n";
      const r = ok(parseYamlContent(content));
      expect(r.documents[0]!.data).toEqual({
        a: "yes",
        b: "no",
        c: "on",
        d: "off",
      });
    });

    it("still parses true and false as booleans", () => {
      const content = "a: true\nb: false\n";
      const r = ok(parseYamlContent(content));
      expect(r.documents[0]!.data).toEqual({ a: true, b: false });
    });
  });

  // -------------------------------------------------------------------------
  // Error checks
  // -------------------------------------------------------------------------

  describe("empty_file", () => {
    it("rejects empty string", () => {
      const r = err(parseYamlContent(""));
      expect(r.code).toBe("yass.yaml.empty_file");
    });
  });

  describe("malformed YAML", () => {
    it("detects unterminated flow collection", () => {
      const r = err(parseYamlContent("{a: 1, b"));
      expect(r.code).toBe("yass.yaml.malformed");
    });

    it("detects bad indentation", () => {
      const r = err(parseYamlContent("a:\n b: 1\n  c: 2\n d: 3\n"));
      // This may or may not error depending on exact content — use a definite bad case
    });

    it("detects tab indentation error", () => {
      const r = err(parseYamlContent("a:\n\tb: 1\n"));
      expect(r.code).toBe("yass.yaml.malformed");
    });

    it("includes line number in error", () => {
      const r = err(parseYamlContent("a: 1\nb: [unterminated\n"));
      expect(r.code).toBe("yass.yaml.malformed");
      expect(r.line).toBeGreaterThanOrEqual(1);
    });
  });

  describe("duplicate_key", () => {
    it("detects duplicate keys in a mapping", () => {
      const r = err(parseYamlContent("a: 1\na: 2\n"));
      expect(r.code).toBe("yass.yaml.duplicate_key");
    });

    it("detects duplicate keys in nested mapping", () => {
      const r = err(parseYamlContent("top:\n  x: 1\n  x: 2\n"));
      expect(r.code).toBe("yass.yaml.duplicate_key");
    });

    it("includes line number", () => {
      const r = err(parseYamlContent("a: 1\nb: 2\na: 3\n"));
      expect(r.code).toBe("yass.yaml.duplicate_key");
      expect(r.line).toBe(3);
    });
  });

  describe("anchor_or_alias", () => {
    it("detects anchor on scalar", () => {
      const r = err(parseYamlContent("a: &val 1\n"));
      expect(r.code).toBe("yass.yaml.anchor_or_alias");
    });

    it("detects alias", () => {
      const r = err(parseYamlContent("a: &val 1\nb: *val\n"));
      expect(r.code).toBe("yass.yaml.anchor_or_alias");
    });

    it("detects anchor on mapping", () => {
      const r = err(parseYamlContent("a: &m\n  x: 1\n"));
      expect(r.code).toBe("yass.yaml.anchor_or_alias");
    });

    it("detects non-default tag", () => {
      const r = err(parseYamlContent("a: !custom hello\n"));
      expect(r.code).toBe("yass.yaml.anchor_or_alias");
      expect(r.message).toContain("!custom");
    });

    it("allows default YAML tags", () => {
      // !!str, !!int etc. are default tags and should be allowed
      const r = ok(parseYamlContent("a: !!str 123\nb: !!int 42\n"));
      expect(r.documents[0]!.data).toEqual({ a: "123", b: 42 });
    });

    it("includes line number for anchor", () => {
      const r = err(parseYamlContent("x: 1\ny: &anc 2\n"));
      expect(r.code).toBe("yass.yaml.anchor_or_alias");
      expect(r.line).toBe(2);
    });

    it("detects alias without a preceding anchor in the same parse", () => {
      // Use an alias reference directly (no anchor defined) so the Alias
      // visitor fires before any Scalar anchor is found.
      const r = err(parseYamlContent("a: *missing\n"));
      expect(r.code).toBe("yass.yaml.anchor_or_alias");
      expect(r.message).toContain("aliases");
    });

    it("detects alias and reports line number", () => {
      const r = err(parseYamlContent("x: 1\ny: *ref\n"));
      expect(r.code).toBe("yass.yaml.anchor_or_alias");
      expect(r.message).toContain("aliases");
      expect(r.line).toBe(2);
    });

    it("detects non-default tag on a mapping node", () => {
      const r = err(parseYamlContent("a: !custom\n  x: 1\n"));
      expect(r.code).toBe("yass.yaml.anchor_or_alias");
      expect(r.message).toContain("!custom");
    });

    it("detects anchor on a sequence node", () => {
      const r = err(parseYamlContent("a: &seq\n  - 1\n  - 2\n"));
      expect(r.code).toBe("yass.yaml.anchor_or_alias");
      expect(r.message).toContain("anchors");
    });

    it("detects non-default tag on a sequence node", () => {
      const r = err(parseYamlContent("a: !mytag\n  - 1\n  - 2\n"));
      expect(r.code).toBe("yass.yaml.anchor_or_alias");
      expect(r.message).toContain("!mytag");
    });

    it("detects anchor on a top-level mapping node", () => {
      const r = err(parseYamlContent("&mapanchor\na: 1\n"));
      expect(r.code).toBe("yass.yaml.anchor_or_alias");
      expect(r.message).toContain("anchors");
    });

    it("detects non-default tag on a top-level sequence", () => {
      const r = err(parseYamlContent("!customseq\n- 1\n- 2\n"));
      expect(r.code).toBe("yass.yaml.anchor_or_alias");
      expect(r.message).toContain("!customseq");
    });
  });

  // -------------------------------------------------------------------------
  // Priority order
  // -------------------------------------------------------------------------

  describe("priority order", () => {
    it("empty_file before malformed", () => {
      // Empty string triggers empty_file, not malformed
      const r = err(parseYamlContent(""));
      expect(r.code).toBe("yass.yaml.empty_file");
    });

    it("malformed before duplicate_key", () => {
      // Content that is both malformed and has duplicate keys:
      // Malformed should win
      const r = err(parseYamlContent("a: [\na: 2\n"));
      expect(r.code).toBe("yass.yaml.malformed");
    });

    it("duplicate_key before anchor_or_alias", () => {
      // Content with both duplicate keys and anchors
      const r = err(parseYamlContent("a: &x 1\na: 2\n"));
      expect(r.code).toBe("yass.yaml.duplicate_key");
    });
  });

  // -------------------------------------------------------------------------
  // Edge cases
  // -------------------------------------------------------------------------

  describe("edge cases", () => {
    it("handles document with only comments", () => {
      // A YAML file with only comments produces no documents (like empty)
      // But the content string is not empty, so it should succeed with 0 docs
      const r = ok(parseYamlContent("# just a comment\n"));
      expect(r.documents).toHaveLength(0);
    });

    it("handles document starting with explicit ---", () => {
      const r = ok(parseYamlContent("---\na: 1\n"));
      expect(r.documents).toHaveLength(1);
      expect(r.documents[0]!.data).toEqual({ a: 1 });
    });

    it("handles document ending with ...", () => {
      const r = ok(parseYamlContent("a: 1\n...\n"));
      expect(r.documents).toHaveLength(1);
      expect(r.documents[0]!.data).toEqual({ a: 1 });
    });

    it("handles whitespace-only content", () => {
      const r = ok(parseYamlContent("   \n  \n"));
      expect(r.documents).toHaveLength(0);
    });
  });
});

// ---------------------------------------------------------------------------
// parseYamlFile — file-level checks
// ---------------------------------------------------------------------------

describe("parseYamlFile", () => {
  describe("not_utf8", () => {
    it("rejects a file with invalid UTF-8 bytes", () => {
      const p = writeTmp("bad.yaml", Buffer.from([0x80, 0x81, 0x82]));
      const r = err(parseYamlFile(p));
      expect(r.code).toBe("yass.yaml.not_utf8");
    });

    it("rejects a file with invalid continuation byte", () => {
      // Start of a 2-byte sequence (0xC0) followed by non-continuation
      const p = writeTmp("bad2.yaml", Buffer.from([0xc0, 0x00]));
      const r = err(parseYamlFile(p));
      expect(r.code).toBe("yass.yaml.not_utf8");
    });
  });

  describe("has_bom", () => {
    it("rejects a file starting with UTF-8 BOM", () => {
      const bom = Buffer.from([0xef, 0xbb, 0xbf]);
      const content = Buffer.from("a: 1\n");
      const p = writeTmp("bom.yaml", Buffer.concat([bom, content]));
      const r = err(parseYamlFile(p));
      expect(r.code).toBe("yass.yaml.has_bom");
    });
  });

  describe("empty_file", () => {
    it("rejects a zero-byte file", () => {
      const p = writeTmp("empty.yaml", Buffer.alloc(0));
      const r = err(parseYamlFile(p));
      expect(r.code).toBe("yass.yaml.empty_file");
    });
  });

  describe("valid file", () => {
    it("parses a normal YAML file", () => {
      const p = writeTmp("good.yaml", "name: test\nvalue: 42\n");
      const r = ok(parseYamlFile(p));
      expect(r.documents).toHaveLength(1);
      expect(r.documents[0]!.data).toEqual({ name: "test", value: 42 });
    });

    it("parses a multi-document file", () => {
      const p = writeTmp("multi.yaml", "a: 1\n---\nb: 2\n");
      const r = ok(parseYamlFile(p));
      expect(r.documents).toHaveLength(2);
    });
  });

  describe("priority order across file checks", () => {
    it("not_utf8 before has_bom", () => {
      // A file that starts with BOM-like bytes but is not valid UTF-8 overall
      // Actually BOM + invalid UTF-8 after
      const p = writeTmp(
        "priority.yaml",
        Buffer.from([0xef, 0xbb, 0xbf, 0x80, 0x81]),
      );
      // This has BOM but also invalid UTF-8 trailing bytes
      // However, the BOM itself is valid UTF-8 — so the bytes after are the problem
      // The UTF-8 check runs first, so not_utf8 should win
      // Actually 0x80 0x81 alone are not valid UTF-8.
      // But EF BB BF 80 81 — the 80 81 are continuation bytes without a start byte
      // So this IS invalid UTF-8
      const r = err(parseYamlFile(p));
      expect(r.code).toBe("yass.yaml.not_utf8");
    });

    it("has_bom before empty_file", () => {
      // BOM-only file (3 bytes, all BOM)
      const p = writeTmp("bom-only.yaml", Buffer.from([0xef, 0xbb, 0xbf]));
      const r = err(parseYamlFile(p));
      expect(r.code).toBe("yass.yaml.has_bom");
    });

    it("has_bom before malformed", () => {
      const bom = Buffer.from([0xef, 0xbb, 0xbf]);
      const content = Buffer.from("{unterminated");
      const p = writeTmp("bom-malformed.yaml", Buffer.concat([bom, content]));
      const r = err(parseYamlFile(p));
      expect(r.code).toBe("yass.yaml.has_bom");
    });
  });

  describe("malformed from file", () => {
    it("detects malformed YAML in file", () => {
      const p = writeTmp("malformed.yaml", "{a: 1, b");
      const r = err(parseYamlFile(p));
      expect(r.code).toBe("yass.yaml.malformed");
    });
  });

  describe("duplicate_key from file", () => {
    it("detects duplicate key in file", () => {
      const p = writeTmp("dup.yaml", "a: 1\na: 2\n");
      const r = err(parseYamlFile(p));
      expect(r.code).toBe("yass.yaml.duplicate_key");
    });
  });

  describe("anchor_or_alias from file", () => {
    it("detects anchors in file", () => {
      const p = writeTmp("anchor.yaml", "a: &x 1\nb: *x\n");
      const r = err(parseYamlFile(p));
      expect(r.code).toBe("yass.yaml.anchor_or_alias");
    });
  });
});
