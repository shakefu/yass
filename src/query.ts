/**
 * query subcommand — retrieve a spec by name and emit it as a YAML fragment
 * to stdout, resolving CONFORMS refs by inlining with provenance comments.
 */

import { existsSync, statSync, readFileSync } from "node:fs";
import { resolve, isAbsolute, dirname, join, basename } from "node:path";
import {
  ErrorCode,
  exitCodeFor,
  messageFor,
  SLOT_KEYWORDS,
  NORMATIVITY_KEYWORDS,
  REFERENCE_KEYWORDS,
  GUARD_KEYWORD,
} from "./errors.ts";
import { formatErrorLine, formatFilePath } from "./error-line.ts";
import { findProjectRoot } from "./find-root.ts";
import { discoverSpecFiles } from "./discover.ts";
import type { DiscoverResult, DiscoverError } from "./discover.ts";
import { parseYamlFile, parseYamlContent } from "./yaml-parser.ts";
import type { ParseSuccess, ParseError, YamlDocument } from "./yaml-parser.ts";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface SpecMatch {
  file: string; // absolute path
  specName: string;
  docIndex: number; // 0-based index among ALL documents (including preamble)
  description: string; // preamble description for disambiguation
}

interface Obligation {
  [key: string]: unknown;
}

// ---------------------------------------------------------------------------
// YAML 1.2 core-schema type tokens that require quoting
// ---------------------------------------------------------------------------

const YAML_TYPE_TOKENS = new Set([
  "true",
  "false",
  "null",
  "yes",
  "no",
  "on",
  "off",
  "True",
  "False",
  "Null",
  "Yes",
  "No",
  "On",
  "Off",
  "TRUE",
  "FALSE",
  "NULL",
  "YES",
  "NO",
  "ON",
  "OFF",
  "~",
]);

/** Characters that require quoting when they appear at the start of a scalar. */
const LEADING_SPECIAL = new Set(["?", "-", "*", "&", "!", "|", ">", "%", "@"]);

// ---------------------------------------------------------------------------
// Scalar quoting
// ---------------------------------------------------------------------------

/**
 * Returns true when a scalar must be double-quoted per OutputProfile.
 */
function needsQuoting(value: string): boolean {
  if (value.length === 0) return true;

  // Leading or trailing whitespace
  if (value !== value.trim()) return true;

  // Contains ": " (colon-space)
  if (value.includes(": ")) return true;

  // Leading special character
  if (LEADING_SPECIAL.has(value[0]!)) return true;

  // YAML 1.2 core-schema type tokens
  if (YAML_TYPE_TOKENS.has(value)) return true;

  // Numeric literals: integers, floats, octal, hex, inf, nan, signed variants
  if (isNumericLiteral(value)) return true;

  return false;
}

function isNumericLiteral(s: string): boolean {
  // Integer (decimal, optional sign)
  if (/^[-+]?[0-9]+$/.test(s)) return true;
  // Float (optional sign, with decimal point or exponent)
  if (/^[-+]?(\.[0-9]+|[0-9]+(\.[0-9]*)?)([eE][-+]?[0-9]+)?$/.test(s) && /[.eE]/.test(s)) return true;
  // Octal
  if (/^0o[0-7]+$/i.test(s)) return true;
  // Hex
  if (/^0x[0-9a-fA-F]+$/i.test(s)) return true;
  // Infinity / NaN
  if (/^[-+]?\.(inf|Inf|INF)$/.test(s)) return true;
  if (/^\.(nan|NaN|NAN)$/.test(s)) return true;
  return false;
}

/**
 * Format a scalar value for YAML output. Double-quote when needed.
 */
function formatScalar(value: unknown): string {
  if (value === null || value === undefined) return '""';
  const s = String(value);
  if (needsQuoting(s)) {
    // Escape backslashes and double quotes inside the value
    const escaped = s.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
    return `"${escaped}"`;
  }
  return s;
}

// ---------------------------------------------------------------------------
// YAML fragment emitter
// ---------------------------------------------------------------------------

/**
 * Emit a spec document as a YAML fragment string conforming to OutputProfile.
 * The spec data is the parsed document data (a mapping), and obligations
 * may carry provenance comments.
 */
function emitFragment(specData: Record<string, unknown>, provenanceMap: Map<object, string>): string {
  const lines: string[] = ["---"];

  // Emit keys in document order (preserved from source)
  for (const [key, value] of Object.entries(specData)) {
    if (key === "spec") {
      lines.push(`spec: ${formatScalar(value)}`);
    } else if (isSlotKey(key)) {
      lines.push(`${key}:`);
      if (Array.isArray(value)) {
        for (const item of value) {
          emitObligation(item as Record<string, unknown>, lines, provenanceMap);
        }
      }
    } else {
      // Unknown key — emit as-is
      lines.push(`${key}: ${formatScalar(value)}`);
    }
  }

  return lines.join("\n") + "\n";
}

function isSlotKey(key: string): boolean {
  return (SLOT_KEYWORDS as readonly string[]).includes(key);
}

/**
 * Emit a single obligation list item.
 * Key order: Normativity, WHEN, References.
 */
function emitObligation(
  obligation: Record<string, unknown>,
  lines: string[],
  provenanceMap: Map<object, string>,
): void {
  // Check for provenance comment
  const provenance = provenanceMap.get(obligation);
  if (provenance !== undefined) {
    lines.push(provenance);
  }

  const normativityKeys = (NORMATIVITY_KEYWORDS as readonly string[]).filter(
    (k) => k in obligation,
  );
  const hasWhen = GUARD_KEYWORD in obligation;
  const refKeys = (REFERENCE_KEYWORDS as readonly string[]).filter(
    (k) => k in obligation,
  );

  // Determine the order: normativity first, then WHEN, then refs
  const orderedKeys: string[] = [...normativityKeys];
  if (hasWhen) orderedKeys.push(GUARD_KEYWORD);
  orderedKeys.push(...refKeys);

  // If no recognized keys, emit whatever we have
  if (orderedKeys.length === 0) {
    for (const [key, value] of Object.entries(obligation)) {
      orderedKeys.push(key);
    }
  }

  let first = true;
  for (const key of orderedKeys) {
    const value = obligation[key];
    const prefix = first ? "- " : "  ";
    first = false;
    lines.push(`${prefix}${key}: ${formatScalar(value)}`);
  }
}

// ---------------------------------------------------------------------------
// Name matching
// ---------------------------------------------------------------------------

/**
 * Match a query name against a spec name.
 * - Exact match (case-sensitive byte comparison)
 * - Dot-aligned trailing suffix: query "Bar" matches "Foo.Bar"
 * - No partial substring matches
 */
function nameMatches(specName: string, query: string): boolean {
  if (specName === query) return true;

  // Dot-aligned trailing suffix: specName must end with "." + query
  if (specName.endsWith("." + query)) return true;

  return false;
}

// ---------------------------------------------------------------------------
// CONFORMS inlining
// ---------------------------------------------------------------------------

interface InlineError {
  code: string;
  message: string;
  file?: string;
}

/**
 * Parse a CONFORMS ref target.
 * Returns { filePart, specName, slotName } or null if malformed.
 *
 * Formats:
 *   SpecName::SLOT
 *   ./path@SpecName::SLOT
 *   ../path@SpecName::SLOT
 */
function parseConformsRef(
  target: string,
): { filePart: string | null; specName: string; slotName: string } | null {
  // Must have ::SLOT suffix
  const slotSepIdx = target.indexOf("::");
  if (slotSepIdx === -1) return null;

  const beforeSlot = target.slice(0, slotSepIdx);
  const slotName = target.slice(slotSepIdx + 2);

  if (!slotName) return null;

  // Check for file@ prefix
  const atIdx = beforeSlot.indexOf("@");
  if (atIdx !== -1) {
    const filePart = beforeSlot.slice(0, atIdx);
    const specName = beforeSlot.slice(atIdx + 1);
    return { filePart, specName, slotName };
  }

  return { filePart: null, specName: beforeSlot, slotName };
}

/**
 * Resolve a CONFORMS ref file path.
 * - Project-root-anchored refs (no ./ or ../) resolve from project root
 * - Relative refs (./ or ../) resolve from the matched spec's directory
 */
function resolveRefFile(
  filePart: string,
  specDir: string,
  projectRoot: string,
): string {
  if (filePart.startsWith("./") || filePart.startsWith("../")) {
    // Relative to spec's containing directory
    return resolve(specDir, filePart);
  }
  // Project-root-anchored
  return resolve(projectRoot, filePart);
}

/**
 * Find a spec file for a reference. The filePart may not have the .yass.yaml
 * extension, so we try appending it.
 */
function findRefFile(resolvedPath: string): string | null {
  // Try exact path
  if (existsSync(resolvedPath)) return resolvedPath;

  // Try appending .yass.yaml
  const withExt = resolvedPath + ".yass.yaml";
  if (existsSync(withExt)) return withExt;

  return null;
}

/**
 * Find a spec's obligations for a given slot in a parsed file.
 */
function findSpecSlotInFile(
  filePath: string,
  specName: string,
  slotName: string,
): { obligations: Record<string, unknown>[] } | { error: InlineError } {
  const parseResult = parseYamlFile(filePath);
  if (!parseResult.ok) {
    return {
      error: {
        code: ErrorCode.QUERY_CONFORMS_UNRESOLVED,
        message: messageFor(ErrorCode.QUERY_CONFORMS_UNRESOLVED, {
          target: `${specName}::${slotName}`,
        }),
        file: filePath,
      },
    };
  }

  // Find the spec document
  for (const doc of parseResult.documents) {
    if (doc.data.spec === specName) {
      const slotValue = doc.data[slotName];
      if (slotValue === undefined) {
        // Slot not declared in the spec
        return {
          error: {
            code: ErrorCode.QUERY_CONFORMS_UNRESOLVED,
            message: messageFor(ErrorCode.QUERY_CONFORMS_UNRESOLVED, {
              target: `${specName}::${slotName}`,
            }),
            file: filePath,
          },
        };
      }
      if (!Array.isArray(slotValue)) {
        return {
          error: {
            code: ErrorCode.QUERY_CONFORMS_UNRESOLVED,
            message: messageFor(ErrorCode.QUERY_CONFORMS_UNRESOLVED, {
              target: `${specName}::${slotName}`,
            }),
            file: filePath,
          },
        };
      }
      return { obligations: slotValue as Record<string, unknown>[] };
    }
  }

  // Spec not found in file
  return {
    error: {
      code: ErrorCode.QUERY_CONFORMS_UNRESOLVED,
      message: messageFor(ErrorCode.QUERY_CONFORMS_UNRESOLVED, {
        target: `${specName}::${slotName}`,
      }),
      file: filePath,
    },
  };
}

/**
 * Perform CONFORMS inlining on a spec document.
 * Returns the modified spec data and provenance map, or errors.
 */
function inlineConforms(
  specData: Record<string, unknown>,
  specFile: string,
  projectRoot: string,
): {
  data: Record<string, unknown>;
  provenanceMap: Map<object, string>;
  errors: InlineError[];
} {
  const specDir = dirname(specFile);
  const errors: InlineError[] = [];
  const provenanceMap = new Map<object, string>();
  const result: Record<string, unknown> = {};

  for (const [key, value] of Object.entries(specData)) {
    if (!isSlotKey(key) || !Array.isArray(value)) {
      result[key] = value;
      continue;
    }

    const newObligations: Record<string, unknown>[] = [];

    for (const obligation of value as Record<string, unknown>[]) {
      const conformsRef = obligation.CONFORMS;
      if (conformsRef === undefined || typeof conformsRef !== "string") {
        // No CONFORMS ref — keep as-is
        newObligations.push(obligation);
        continue;
      }

      // Check if it has ::SLOT suffix
      if (!conformsRef.includes("::")) {
        errors.push({
          code: ErrorCode.QUERY_CONFORMS_NO_SLOT,
          message: messageFor(ErrorCode.QUERY_CONFORMS_NO_SLOT, {
            target: conformsRef,
          }),
        });
        continue;
      }

      // Parse the ref target
      const parsed = parseConformsRef(conformsRef);
      if (!parsed) {
        errors.push({
          code: ErrorCode.QUERY_CONFORMS_UNRESOLVED,
          message: messageFor(ErrorCode.QUERY_CONFORMS_UNRESOLVED, {
            target: conformsRef,
          }),
        });
        continue;
      }

      // Resolve the file
      let targetFile: string;
      if (parsed.filePart !== null) {
        const resolvedPath = resolveRefFile(parsed.filePart, specDir, projectRoot);
        const found = findRefFile(resolvedPath);
        if (!found) {
          errors.push({
            code: ErrorCode.QUERY_CONFORMS_UNRESOLVED,
            message: messageFor(ErrorCode.QUERY_CONFORMS_UNRESOLVED, {
              target: conformsRef,
            }),
          });
          continue;
        }
        targetFile = found;
      } else {
        // Same file reference
        targetFile = specFile;
      }

      // Find the slot in the target file
      const slotResult = findSpecSlotInFile(
        targetFile,
        parsed.specName,
        parsed.slotName,
      );
      if ("error" in slotResult) {
        errors.push(slotResult.error);
        continue;
      }

      // Determine if this is a reference-only or normative obligation
      const hasNormativity = (NORMATIVITY_KEYWORDS as readonly string[]).some(
        (k) => k in obligation,
      );
      const carrierWhen =
        obligation[GUARD_KEYWORD] !== undefined
          ? String(obligation[GUARD_KEYWORD])
          : null;

      // Build non-CONFORMS parts of carrier
      const carrierWithoutConforms: Record<string, unknown> = {};
      for (const [ok, ov] of Object.entries(obligation)) {
        if (ok !== "CONFORMS") {
          carrierWithoutConforms[ok] = ov;
        }
      }

      if (hasNormativity) {
        // Normative obligation with CONFORMS:
        // Keep the normative obligation (without CONFORMS), then append inlined
        newObligations.push(carrierWithoutConforms);
      }
      // If reference-only: replace entirely with inlined obligations

      // Inline the referenced obligations
      for (const refObligation of slotResult.obligations) {
        const inlined: Record<string, unknown> = {};

        // Copy all keys from the referenced obligation, building proper order
        // Normativity first, then WHEN, then refs
        const refObl = refObligation as Record<string, unknown>;

        // Collect normativity keys
        for (const nk of NORMATIVITY_KEYWORDS) {
          if (nk in refObl) {
            inlined[nk] = refObl[nk];
          }
        }

        // Handle WHEN guard combination
        const innerWhen =
          refObl[GUARD_KEYWORD] !== undefined
            ? String(refObl[GUARD_KEYWORD])
            : null;

        if (carrierWhen !== null && innerWhen !== null) {
          // Combine: outer + " and " + inner
          inlined[GUARD_KEYWORD] = carrierWhen + " and " + innerWhen;
        } else if (carrierWhen !== null) {
          inlined[GUARD_KEYWORD] = carrierWhen;
        } else if (innerWhen !== null) {
          inlined[GUARD_KEYWORD] = innerWhen;
        }

        // Copy reference keys (except we don't re-inline CONFORMS from inlined obligations)
        for (const rk of REFERENCE_KEYWORDS) {
          if (rk in refObl) {
            inlined[rk] = refObl[rk];
          }
        }

        // Copy any remaining keys not yet handled
        for (const [rk, rv] of Object.entries(refObl)) {
          if (!(rk in inlined)) {
            inlined[rk] = rv;
          }
        }

        // Set provenance comment
        provenanceMap.set(inlined, `# CONFORMS: ${conformsRef}`);

        newObligations.push(inlined);
      }
    }

    result[key] = newObligations;
  }

  return { data: result, provenanceMap, errors };
}

// ---------------------------------------------------------------------------
// Disambiguation row formatting (cli.list format)
// ---------------------------------------------------------------------------

function formatDisambiguationRow(
  file: string,
  specName: string,
  description: string,
  cwd: string,
): string {
  const formattedPath = formatFilePath(file, cwd);
  // Replace tabs in path with spaces
  const safePath = formattedPath.replace(/\t/g, " ");
  return `${safePath}\t${specName}\t${description}`;
}

/**
 * Normalize a preamble description: collapse all whitespace runs to a single
 * space, then trim.
 */
function normalizeDescription(desc: unknown): string {
  if (desc === null || desc === undefined || typeof desc !== "string") return "";
  return desc.replace(/\s+/g, " ").trim();
}

// ---------------------------------------------------------------------------
// Main entry point
// ---------------------------------------------------------------------------

export function queryCommand(
  args: string[],
  cwd: string,
  stdout: { write(s: string): void },
  stderr: { write(s: string): void },
  isTTY: boolean,
): number {
  // --- Parse arguments ---
  const specNameArg = args[0];
  const scopeArg = args[1];

  // Check for missing spec name
  if (specNameArg === undefined) {
    stderr.write(
      formatErrorLine(
        {
          code: ErrorCode.QUERY_NAME_MISSING,
          message: messageFor(ErrorCode.QUERY_NAME_MISSING),
        },
        cwd,
      ) + "\n",
    );
    return exitCodeFor(ErrorCode.QUERY_NAME_MISSING);
  }

  // Check for blank spec name
  if (specNameArg === "") {
    stderr.write(
      formatErrorLine(
        {
          code: ErrorCode.QUERY_NAME_BLANK,
          message: messageFor(ErrorCode.QUERY_NAME_BLANK),
        },
        cwd,
      ) + "\n",
    );
    return exitCodeFor(ErrorCode.QUERY_NAME_BLANK);
  }

  // Check for whitespace in spec name — treat as no-match (not blank error)
  const hasWhitespace = /\s/.test(specNameArg);

  // --- Validate scope path ---
  // Check for colon in scope path
  if (scopeArg !== undefined && scopeArg.includes(":")) {
    stderr.write(
      formatErrorLine(
        {
          code: ErrorCode.PATH_COLON_IN_PATH,
          message: messageFor(ErrorCode.PATH_COLON_IN_PATH, {
            path: scopeArg,
          }),
        },
        cwd,
      ) + "\n",
    );
    return exitCodeFor(ErrorCode.PATH_COLON_IN_PATH);
  }

  // --- Find project root ---
  const rootResult = findProjectRoot(cwd);
  if ("code" in rootResult) {
    stderr.write(
      formatErrorLine(
        {
          code: rootResult.code,
          message: rootResult.message,
        },
        cwd,
      ) + "\n",
    );
    return exitCodeFor(rootResult.code);
  }
  const projectRoot = rootResult.root;

  // --- Validate scope ---
  if (scopeArg !== undefined) {
    const scopePath = isAbsolute(scopeArg) ? scopeArg : resolve(cwd, scopeArg);

    // Check existence
    let scopeStat;
    try {
      scopeStat = statSync(scopePath, { throwIfNoEntry: false });
    } catch {
      scopeStat = null;
    }

    if (!scopeStat) {
      stderr.write(
        formatErrorLine(
          {
            code: ErrorCode.QUERY_SCOPE_NOT_FOUND,
            message: messageFor(ErrorCode.QUERY_SCOPE_NOT_FOUND, {
              path: scopeArg,
            }),
          },
          cwd,
        ) + "\n",
      );
      return exitCodeFor(ErrorCode.QUERY_SCOPE_NOT_FOUND);
    }

    // Check for .yass.yaml files in scope
    const discoverResult = discoverSpecFiles(scopeArg, projectRoot, cwd);
    if ("code" in discoverResult) {
      // Discovery error
      stderr.write(
        formatErrorLine(
          {
            code: ErrorCode.QUERY_SCOPE_NOT_FOUND,
            message: messageFor(ErrorCode.QUERY_SCOPE_NOT_FOUND, {
              path: scopeArg,
            }),
          },
          cwd,
        ) + "\n",
      );
      return exitCodeFor(ErrorCode.QUERY_SCOPE_NOT_FOUND);
    }

    if (discoverResult.files.length === 0) {
      stderr.write(
        formatErrorLine(
          {
            code: ErrorCode.QUERY_SCOPE_EMPTY,
            message: messageFor(ErrorCode.QUERY_SCOPE_EMPTY, {
              path: scopeArg,
            }),
          },
          cwd,
        ) + "\n",
      );
      return exitCodeFor(ErrorCode.QUERY_SCOPE_EMPTY);
    }
  }

  // --- Whitespace in spec name: immediate no-match ---
  if (hasWhitespace) {
    stderr.write(
      formatErrorLine(
        {
          code: ErrorCode.QUERY_NO_MATCH,
          message: messageFor(ErrorCode.QUERY_NO_MATCH, { name: specNameArg }),
        },
        cwd,
      ) + "\n",
    );
    return exitCodeFor(ErrorCode.QUERY_NO_MATCH);
  }

  // --- Discover spec files ---
  const discoverResult = discoverSpecFiles(scopeArg, projectRoot, cwd);
  if ("code" in discoverResult) {
    // Fatal discover error
    stderr.write(
      formatErrorLine(
        {
          code: discoverResult.code,
          message: discoverResult.message,
          file: "file" in discoverResult ? (discoverResult as any).file : undefined,
        },
        cwd,
      ) + "\n",
    );
    return exitCodeFor(discoverResult.code);
  }

  // --- Name lookup across all discovered files ---
  const matches: SpecMatch[] = [];

  for (const relFile of discoverResult.files) {
    const absFile = isAbsolute(relFile) ? relFile : resolve(cwd, relFile);
    const parseResult = parseYamlFile(absFile);
    if (!parseResult.ok) continue; // Skip unparseable files

    // Extract preamble description (first document without 'spec' key)
    let description = "";
    for (const doc of parseResult.documents) {
      if (!("spec" in doc.data) && "description" in doc.data) {
        description = normalizeDescription(doc.data.description);
        break;
      }
    }

    // Find matching specs
    for (let i = 0; i < parseResult.documents.length; i++) {
      const doc = parseResult.documents[i]!;
      if (typeof doc.data.spec !== "string") continue;

      if (nameMatches(doc.data.spec, specNameArg)) {
        matches.push({
          file: absFile,
          specName: doc.data.spec,
          docIndex: i,
          description,
        });
      }
    }
  }

  // --- Dispatch on match count ---
  if (matches.length === 0) {
    stderr.write(
      formatErrorLine(
        {
          code: ErrorCode.QUERY_NO_MATCH,
          message: messageFor(ErrorCode.QUERY_NO_MATCH, { name: specNameArg }),
        },
        cwd,
      ) + "\n",
    );
    return exitCodeFor(ErrorCode.QUERY_NO_MATCH);
  }

  if (matches.length > 1) {
    // Multi-match: emit disambiguation rows in list format
    for (const match of matches) {
      const row = formatDisambiguationRow(
        match.file,
        match.specName,
        match.description,
        cwd,
      );
      stdout.write(row + "\n");
    }
    return 0;
  }

  // --- Single match: extract and emit YAML fragment ---
  const match = matches[0]!;
  const parseResult = parseYamlFile(match.file);
  if (!parseResult.ok) {
    stderr.write(
      formatErrorLine(
        {
          code: parseResult.code,
          message: parseResult.message,
          file: match.file,
          line: parseResult.line,
        },
        cwd,
      ) + "\n",
    );
    return exitCodeFor(parseResult.code);
  }

  // Find the matching document
  const matchDoc = parseResult.documents[match.docIndex]!;

  // Perform CONFORMS inlining
  const inlineResult = inlineConforms(matchDoc.data, match.file, projectRoot);

  if (inlineResult.errors.length > 0) {
    // Emit all errors, do NOT emit fragment
    for (const err of inlineResult.errors) {
      stderr.write(
        formatErrorLine(
          {
            code: err.code,
            message: err.message,
            file: err.file,
          },
          cwd,
        ) + "\n",
      );
    }
    return exitCodeFor(inlineResult.errors[0]!.code);
  }

  // Emit the fragment
  const fragment = emitFragment(inlineResult.data, inlineResult.provenanceMap);
  stdout.write(fragment);
  return 0;
}
