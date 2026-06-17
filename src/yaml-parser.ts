/**
 * YAML parser module — wraps the 'yaml' npm package (v2.x) to perform all
 * checks required by cli.validate.CheckYAML.
 *
 * Priority order (emit at most ONE error per file):
 *   1. yass.yaml.not_utf8
 *   2. yass.yaml.has_bom
 *   3. yass.yaml.empty_file
 *   4. yass.yaml.malformed
 *   5. yass.yaml.duplicate_key
 *   6. yass.yaml.anchor_or_alias
 */

import {
  type Document,
  LineCounter,
  isAlias,
  parseAllDocuments,
  visit,
} from "yaml";

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

export interface YamlDocument {
  data: Record<string, unknown>;
  /** 1-based line where this document starts in the file */
  line: number;
}

export interface ParseSuccess {
  ok: true;
  documents: YamlDocument[];
  /** Original file content as a string */
  rawContent: string;
}

export interface ParseError {
  ok: false;
  /** One of the yass.yaml.* error codes */
  code: string;
  message: string;
  /** 1-based line of the error, when applicable */
  line?: number;
}

export type ParseResult = ParseSuccess | ParseError;

// ---------------------------------------------------------------------------
// Error codes
// ---------------------------------------------------------------------------

const CODE = {
  NOT_UTF8: "yass.yaml.not_utf8",
  HAS_BOM: "yass.yaml.has_bom",
  EMPTY_FILE: "yass.yaml.empty_file",
  MALFORMED: "yass.yaml.malformed",
  DUPLICATE_KEY: "yass.yaml.duplicate_key",
  ANCHOR_OR_ALIAS: "yass.yaml.anchor_or_alias",
} as const;

// Default YAML tags that are allowed (part of the YAML 1.2 core schema).
const DEFAULT_TAG_PREFIX = "tag:yaml.org,2002:";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Return true if `buf` is valid UTF-8. */
function isValidUtf8(buf: Buffer): boolean {
  // TextDecoder with fatal:true throws on invalid sequences.
  try {
    new TextDecoder("utf-8", { fatal: true }).decode(buf);
    return true;
  } catch {
    return false;
  }
}

/** Return true if `buf` starts with the UTF-8 BOM (EF BB BF). */
function hasBom(buf: Buffer): boolean {
  return buf.length >= 3 && buf[0] === 0xef && buf[1] === 0xbb && buf[2] === 0xbf;
}

/**
 * Convert a character offset into a 1-based line number using LineCounter.
 */
function offsetToLine(lineCounter: LineCounter, offset: number): number {
  return lineCounter.linePos(offset).line;
}

// ---------------------------------------------------------------------------
// Core parse logic (operates on string content)
// ---------------------------------------------------------------------------

/**
 * Parse YAML content from a string. Runs all checks except not_utf8.
 *
 * Exported so callers can parse in-memory content directly (e.g. for
 * cross-file reference resolution or testing).
 */
export function parseYamlContent(content: string): ParseResult {
  // 3. empty_file — nothing to parse
  if (content.length === 0) {
    return { ok: false, code: CODE.EMPTY_FILE, message: "File is empty" };
  }

  // Parse with the yaml library
  const lineCounter = new LineCounter();
  const docs = parseAllDocuments(content, {
    uniqueKeys: true,
    version: "1.2",
    lineCounter,
  });

  // 4. malformed — any parse error that is NOT a duplicate-key error
  for (const doc of docs) {
    for (const err of doc.errors) {
      if (err.code !== "DUPLICATE_KEY") {
        const line =
          err.pos && err.pos[0] != null
            ? offsetToLine(lineCounter, err.pos[0])
            : undefined;
        return {
          ok: false,
          code: CODE.MALFORMED,
          message: `Malformed YAML: ${err.message}`,
          line,
        };
      }
    }
  }

  // 5. duplicate_key
  for (const doc of docs) {
    for (const err of doc.errors) {
      if (err.code === "DUPLICATE_KEY") {
        const line =
          err.pos && err.pos[0] != null
            ? offsetToLine(lineCounter, err.pos[0])
            : undefined;
        return {
          ok: false,
          code: CODE.DUPLICATE_KEY,
          message: `Duplicate mapping key`,
          line,
        };
      }
    }
  }

  // 6. anchor_or_alias — walk the AST of every document
  const anchorAliasResult = checkAnchorsAliasesTags(docs, lineCounter);
  if (anchorAliasResult) {
    return anchorAliasResult;
  }

  // All checks passed — build result
  const documents: YamlDocument[] = [];
  for (const doc of docs) {
    const startOffset = doc.range?.[0] ?? 0;
    const line = offsetToLine(lineCounter, startOffset);
    const json = doc.toJSON();
    // Coerce to Record<string, unknown>; non-mapping documents become { _: value }
    const data: Record<string, unknown> =
      json !== null && typeof json === "object" && !Array.isArray(json)
        ? (json as Record<string, unknown>)
        : { _: json };
    documents.push({ data, line });
  }

  return { ok: true, documents, rawContent: content };
}

// ---------------------------------------------------------------------------
// File-based entry point
// ---------------------------------------------------------------------------

/**
 * Read a file from disk and parse it, running all CheckYAML validations.
 */
export function parseYamlFile(filePath: string): ParseResult {
  // Read raw bytes synchronously
  const fs = require("fs") as typeof import("fs");
  const buf: Buffer = fs.readFileSync(filePath);

  // 1. not_utf8
  if (!isValidUtf8(buf)) {
    return {
      ok: false,
      code: CODE.NOT_UTF8,
      message: "File is not valid UTF-8",
    };
  }

  // 2. has_bom
  if (hasBom(buf)) {
    return {
      ok: false,
      code: CODE.HAS_BOM,
      message: "File begins with a UTF-8 BOM",
    };
  }

  // Decode to string and delegate remaining checks
  const content = buf.toString("utf-8");
  return parseYamlContent(content);
}

// ---------------------------------------------------------------------------
// Anchor / Alias / Tag checker
// ---------------------------------------------------------------------------

/**
 * Walk every document's AST looking for anchors, aliases, or non-default tags.
 * Returns a ParseError on first hit, or null if clean.
 */
function checkAnchorsAliasesTags(
  docs: Document.Parsed[],
  lineCounter: LineCounter,
): ParseError | null {
  for (const doc of docs) {
    let found: ParseError | null = null;

    visit(doc, {
      Alias(_key, node) {
        if (!found) {
          const line =
            node.range ? offsetToLine(lineCounter, node.range[0]) : undefined;
          found = {
            ok: false,
            code: CODE.ANCHOR_OR_ALIAS,
            message: `YAML aliases are not allowed`,
            line,
          };
        }
        return visit.BREAK;
      },
      Scalar(_key, node) {
        // Check for anchor
        if (!found && node.anchor) {
          const line =
            node.range ? offsetToLine(lineCounter, node.range[0]) : undefined;
          found = {
            ok: false,
            code: CODE.ANCHOR_OR_ALIAS,
            message: `YAML anchors are not allowed`,
            line,
          };
          return visit.BREAK;
        }
        // Check for non-default tag
        if (!found && node.tag && !node.tag.startsWith(DEFAULT_TAG_PREFIX)) {
          const line =
            node.range ? offsetToLine(lineCounter, node.range[0]) : undefined;
          found = {
            ok: false,
            code: CODE.ANCHOR_OR_ALIAS,
            message: `Non-default YAML tag "${node.tag}" is not allowed`,
            line,
          };
          return visit.BREAK;
        }
      },
      Map(_key, node) {
        if (!found && node.anchor) {
          const line =
            node.range ? offsetToLine(lineCounter, node.range[0]) : undefined;
          found = {
            ok: false,
            code: CODE.ANCHOR_OR_ALIAS,
            message: `YAML anchors are not allowed`,
            line,
          };
          return visit.BREAK;
        }
        if (!found && node.tag && !node.tag.startsWith(DEFAULT_TAG_PREFIX)) {
          const line =
            node.range ? offsetToLine(lineCounter, node.range[0]) : undefined;
          found = {
            ok: false,
            code: CODE.ANCHOR_OR_ALIAS,
            message: `Non-default YAML tag "${node.tag}" is not allowed`,
            line,
          };
          return visit.BREAK;
        }
      },
      Seq(_key, node) {
        if (!found && node.anchor) {
          const line =
            node.range ? offsetToLine(lineCounter, node.range[0]) : undefined;
          found = {
            ok: false,
            code: CODE.ANCHOR_OR_ALIAS,
            message: `YAML anchors are not allowed`,
            line,
          };
          return visit.BREAK;
        }
        if (!found && node.tag && !node.tag.startsWith(DEFAULT_TAG_PREFIX)) {
          const line =
            node.range ? offsetToLine(lineCounter, node.range[0]) : undefined;
          found = {
            ok: false,
            code: CODE.ANCHOR_OR_ALIAS,
            message: `Non-default YAML tag "${node.tag}" is not allowed`,
            line,
          };
          return visit.BREAK;
        }
      },
    });

    if (found) return found;
  }
  return null;
}
