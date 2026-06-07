/**
 * Validate subcommand — validates .yass.yaml spec files.
 *
 * Runs a multi-phase validation pipeline on each file:
 *   1. CheckYAML (handled by parseYamlFile)
 *   2. CheckPreamble
 *   3. CheckSpec (per non-first document)
 *   4. CheckUniqueness
 *   5. CheckRefs
 *
 * Emits error lines to stderr and a summary line to stdout.
 * Exit codes: 0 = success, 1 = validation errors, 2 = usage/input errors.
 */

import { resolve, dirname, basename, isAbsolute, join } from "node:path";
import { statSync } from "node:fs";
import { formatErrorLine } from "./error-line.ts";
import {
  ErrorCode,
  messageFor,
  SLOT_KEYWORDS,
  NORMATIVITY_KEYWORDS,
  REFERENCE_KEYWORDS,
  GUARD_KEYWORD,
  ALL_RESERVED_KEYWORDS,
} from "./errors.ts";
import { findProjectRoot, isFindRootError } from "./find-root.ts";
import {
  discoverSpecFiles,
  type DiscoverResult,
  type DiscoverError,
} from "./discover.ts";
import { expandGlob, isGlobPattern, isGlobError } from "./glob.ts";
import { parseYamlFile, parseYamlContent } from "./yaml-parser.ts";
import type { ParseResult, ParseSuccess, YamlDocument } from "./yaml-parser.ts";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/** Collected error ready for output. */
interface ValidationError {
  file: string;
  line?: number;
  code: string;
  message: string;
}

/** Per-file parse cache for cross-file ref resolution. */
interface ParsedFileInfo {
  result: ParseResult;
  /** Spec names declared in the file (only set when parse succeeded). */
  specNames: string[];
  /** Map of spec name -> set of declared slot keywords. */
  specSlots: Map<string, Set<string>>;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const SPEC_SUFFIX = ".yass.yaml";

/** Normalize a path for deduplication: resolve to absolute, NFC-normalize. */
function normalizePath(p: string, cwd: string): string {
  const abs = isAbsolute(p) ? p : resolve(cwd, p);
  return abs.normalize("NFC");
}

/** Check if a string is a valid slot keyword (upper-case). */
function isSlotKeyword(key: string): boolean {
  return (SLOT_KEYWORDS as readonly string[]).includes(key);
}

/** Check if a key is "spec" (the spec name key). */
function isSpecKey(key: string): boolean {
  return key === "spec";
}

/** All valid keys in a spec document: spec + slot keywords. */
function isValidSpecKey(key: string): boolean {
  return isSpecKey(key) || isSlotKeyword(key);
}

/** Name validation: characters must match [A-Za-z0-9._-]. */
const NAME_CHARS_RE = /^[A-Za-z0-9._-]+$/;

/** Name validation: well-formed dotted name. */
const NAME_FORM_RE = /^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$/;

/** Ref target grammar. */
const REF_GRAMMAR_RE =
  /^([A-Za-z0-9._/-]+@)?[A-Za-z0-9._-]+(::([A-Z][A-Z-]*))?$/;

// ---------------------------------------------------------------------------
// Preamble keys
// ---------------------------------------------------------------------------

/** Keys that are valid in a Preamble document. */
const PREAMBLE_KEYS = new Set([
  "description",
  "version",
  "related",
]);

/** Detect whether a document looks like a preamble (no "spec" key). */
function isPreambleDoc(doc: YamlDocument): boolean {
  return !("spec" in doc.data);
}

/** Detect whether a document has a "spec" key. */
function isSpecDoc(doc: YamlDocument): boolean {
  return "spec" in doc.data;
}

// ---------------------------------------------------------------------------
// CheckPreamble
// ---------------------------------------------------------------------------

/**
 * Validate the preamble (first document).
 * Emits AT MOST ONE error per file, following priority order.
 */
function checkPreamble(
  documents: YamlDocument[],
  file: string,
): ValidationError | null {
  // Empty stream (0 documents)
  if (documents.length === 0) {
    return {
      file,
      code: ErrorCode.YAML_EMPTY_STREAM,
      message: messageFor(ErrorCode.YAML_EMPTY_STREAM),
    };
  }

  const first = documents[0]!;

  // has_spec_key: first document has "spec" key -> it's a spec, not a preamble
  if (isSpecDoc(first)) {
    return {
      file,
      line: first.line,
      code: ErrorCode.PREAMBLE_HAS_SPEC_KEY,
      message: messageFor(ErrorCode.PREAMBLE_HAS_SPEC_KEY),
    };
  }

  // At this point, first doc does NOT have "spec" key, so it's potentially a
  // preamble. But we also need to check: is first doc actually a preamble?
  // A preamble is a document without "spec" key. If it does have "spec", that's
  // has_spec_key (handled above).

  // Check for misplaced preambles (non-first docs that look like preambles but
  // are not spec docs). Actually, the spec says "misplaced" = a preamble that is
  // NOT the first doc. We handle that differently: we check for additional
  // preamble-like docs later.

  // Check for duplicate preamble: any later doc without "spec"
  for (let i = 1; i < documents.length; i++) {
    const doc = documents[i]!;
    if (isPreambleDoc(doc)) {
      return {
        file,
        line: doc.line,
        code: ErrorCode.PREAMBLE_DUPLICATE,
        message: messageFor(ErrorCode.PREAMBLE_DUPLICATE),
      };
    }
  }

  // missing_description
  if (
    !("description" in first.data) ||
    first.data.description === null ||
    first.data.description === undefined ||
    first.data.description === ""
  ) {
    return {
      file,
      line: first.line,
      code: ErrorCode.PREAMBLE_MISSING_DESCRIPTION,
      message: messageFor(ErrorCode.PREAMBLE_MISSING_DESCRIPTION),
    };
  }

  // missing_version
  if (
    !("version" in first.data) ||
    first.data.version === null ||
    first.data.version === undefined ||
    first.data.version === ""
  ) {
    return {
      file,
      line: first.line,
      code: ErrorCode.PREAMBLE_MISSING_VERSION,
      message: messageFor(ErrorCode.PREAMBLE_MISSING_VERSION),
    };
  }

  // unknown_version
  if (String(first.data.version) !== "v1") {
    return {
      file,
      line: first.line,
      code: ErrorCode.PREAMBLE_UNKNOWN_VERSION,
      message: messageFor(ErrorCode.PREAMBLE_UNKNOWN_VERSION, {
        version: String(first.data.version),
      }),
    };
  }

  // bad_related: if present, must be a sequence of strings
  if ("related" in first.data) {
    const related = first.data.related;
    if (!Array.isArray(related)) {
      return {
        file,
        line: first.line,
        code: ErrorCode.PREAMBLE_BAD_RELATED,
        message: messageFor(ErrorCode.PREAMBLE_BAD_RELATED),
      };
    }
    for (const item of related) {
      if (typeof item !== "string") {
        return {
          file,
          line: first.line,
          code: ErrorCode.PREAMBLE_BAD_RELATED,
          message: messageFor(ErrorCode.PREAMBLE_BAD_RELATED),
        };
      }
    }
  }

  return null;
}

// ---------------------------------------------------------------------------
// CheckSpec
// ---------------------------------------------------------------------------

/**
 * Validate a single spec document (non-first document).
 * Returns all errors found for this document.
 */
function checkSpec(
  doc: YamlDocument,
  file: string,
): ValidationError[] {
  const errors: ValidationError[] = [];

  // --- Name checks (priority order, emit at most one name error) ---

  // no_name: missing "spec" key
  if (!("spec" in doc.data)) {
    errors.push({
      file,
      line: doc.line,
      code: ErrorCode.SPEC_NO_NAME,
      message: messageFor(ErrorCode.SPEC_NO_NAME),
    });
    // Can't validate further without a name; but we still check other keys
    checkSpecKeysAndSlots(doc, file, errors, true);
    return errors;
  }

  const specVal = doc.data.spec;

  // name_not_string
  if (typeof specVal !== "string") {
    errors.push({
      file,
      line: doc.line,
      code: ErrorCode.SPEC_NAME_NOT_STRING,
      message: messageFor(ErrorCode.SPEC_NAME_NOT_STRING),
    });
    checkSpecKeysAndSlots(doc, file, errors, false);
    return errors;
  }

  // name_empty
  if (specVal === "") {
    errors.push({
      file,
      line: doc.line,
      code: ErrorCode.SPEC_NAME_EMPTY,
      message: messageFor(ErrorCode.SPEC_NAME_EMPTY),
    });
    checkSpecKeysAndSlots(doc, file, errors, false);
    return errors;
  }

  // name_bad_chars
  if (!NAME_CHARS_RE.test(specVal)) {
    errors.push({
      file,
      line: doc.line,
      code: ErrorCode.SPEC_NAME_BAD_CHARS,
      message: messageFor(ErrorCode.SPEC_NAME_BAD_CHARS, { name: specVal }),
    });
    checkSpecKeysAndSlots(doc, file, errors, false);
    return errors;
  }

  // name_bad_form
  if (!NAME_FORM_RE.test(specVal)) {
    errors.push({
      file,
      line: doc.line,
      code: ErrorCode.SPEC_NAME_BAD_FORM,
      message: messageFor(ErrorCode.SPEC_NAME_BAD_FORM, { name: specVal }),
    });
    checkSpecKeysAndSlots(doc, file, errors, false);
    return errors;
  }

  // name_reserved
  const upperName = specVal.toUpperCase();
  if (ALL_RESERVED_KEYWORDS.some((kw) => kw.toUpperCase() === upperName)) {
    errors.push({
      file,
      line: doc.line,
      code: ErrorCode.SPEC_NAME_RESERVED,
      message: messageFor(ErrorCode.SPEC_NAME_RESERVED, { name: specVal }),
    });
    // Continue checking keys/slots despite reserved name
  }

  // --- Key + slot checks ---
  checkSpecKeysAndSlots(doc, file, errors, false);

  return errors;
}

/**
 * Check for unknown keys and validate slot values + obligations.
 * @param skipSpecKey - if true, don't treat missing "spec" as unknown_key
 */
function checkSpecKeysAndSlots(
  doc: YamlDocument,
  file: string,
  errors: ValidationError[],
  skipSpecKey: boolean,
): void {
  for (const key of Object.keys(doc.data)) {
    if (key === "spec") continue;

    if (!isSlotKeyword(key)) {
      errors.push({
        file,
        line: doc.line,
        code: ErrorCode.SPEC_UNKNOWN_KEY,
        message: messageFor(ErrorCode.SPEC_UNKNOWN_KEY, { key }),
      });
      continue;
    }

    // Slot value must be a list
    const val = doc.data[key];
    if (!Array.isArray(val)) {
      errors.push({
        file,
        line: doc.line,
        code: ErrorCode.SLOT_VALUE_NOT_LIST,
        message: messageFor(ErrorCode.SLOT_VALUE_NOT_LIST, { slot: key }),
      });
      continue;
    }

    // Check each obligation in the slot
    for (const obligation of val) {
      checkObligation(obligation, doc, file, errors);
    }
  }
}

/**
 * Validate a single obligation value.
 * An obligation should be a mapping (object) with normativity/reference/guard
 * keywords as keys and string values.
 */
function checkObligation(
  obligation: unknown,
  doc: YamlDocument,
  file: string,
  errors: ValidationError[],
): void {
  // bad_value_shape: obligation must be a plain object (mapping)
  // null, array, scalar -> bad shape
  if (
    obligation === null ||
    obligation === undefined ||
    typeof obligation !== "object" ||
    Array.isArray(obligation)
  ) {
    errors.push({
      file,
      line: doc.line,
      code: ErrorCode.OBLIGATION_BAD_VALUE_SHAPE,
      message: messageFor(ErrorCode.OBLIGATION_BAD_VALUE_SHAPE),
    });
    return;
  }

  const ob = obligation as Record<string, unknown>;
  const keys = Object.keys(ob);

  // Track what we find
  let hasNormativity = false;
  let hasReference = false;
  let hasGuard = false;
  const seenNormativity = new Set<string>();
  const seenReferences = new Set<string>();

  for (const key of keys) {
    const val = ob[key];

    // Check if key is a normativity keyword
    if ((NORMATIVITY_KEYWORDS as readonly string[]).includes(key)) {
      // Value must be a string (quoted scalar)
      if (typeof val !== "string") {
        errors.push({
          file,
          line: doc.line,
          code: ErrorCode.OBLIGATION_BAD_VALUE_SHAPE,
          message: messageFor(ErrorCode.OBLIGATION_BAD_VALUE_SHAPE),
        });
        continue;
      }
      if (seenNormativity.size > 0) {
        errors.push({
          file,
          line: doc.line,
          code: ErrorCode.OBLIGATION_DUPLICATE_NORMATIVITY,
          message: messageFor(ErrorCode.OBLIGATION_DUPLICATE_NORMATIVITY),
        });
      }
      seenNormativity.add(key);
      hasNormativity = true;
      continue;
    }

    // Check if key is a reference relation
    if ((REFERENCE_KEYWORDS as readonly string[]).includes(key)) {
      if (typeof val !== "string") {
        errors.push({
          file,
          line: doc.line,
          code: ErrorCode.OBLIGATION_BAD_VALUE_SHAPE,
          message: messageFor(ErrorCode.OBLIGATION_BAD_VALUE_SHAPE),
        });
        continue;
      }
      if (seenReferences.has(key)) {
        errors.push({
          file,
          line: doc.line,
          code: ErrorCode.OBLIGATION_DUPLICATE_REFERENCE,
          message: messageFor(ErrorCode.OBLIGATION_DUPLICATE_REFERENCE, {
            relation: key,
          }),
        });
      } else {
        seenReferences.add(key);
      }
      hasReference = true;
      continue;
    }

    // Check if key is the guard keyword
    if (key === GUARD_KEYWORD) {
      if (typeof val !== "string") {
        errors.push({
          file,
          line: doc.line,
          code: ErrorCode.OBLIGATION_BAD_VALUE_SHAPE,
          message: messageFor(ErrorCode.OBLIGATION_BAD_VALUE_SHAPE),
        });
        continue;
      }
      hasGuard = true;
      continue;
    }

    // Unknown key in obligation context: check if it looks like a
    // normativity keyword (case-insensitive but wrong case)
    const upperKey = key.toUpperCase();
    if (
      (NORMATIVITY_KEYWORDS as readonly string[]).some(
        (kw) => kw === upperKey,
      )
    ) {
      errors.push({
        file,
        line: doc.line,
        code: ErrorCode.NORMATIVITY_UNKNOWN,
        message: messageFor(ErrorCode.NORMATIVITY_UNKNOWN, { keyword: key }),
      });
      continue;
    }

    if (
      (REFERENCE_KEYWORDS as readonly string[]).some((kw) => kw === upperKey)
    ) {
      errors.push({
        file,
        line: doc.line,
        code: ErrorCode.REFERENCE_UNKNOWN_RELATION,
        message: messageFor(ErrorCode.REFERENCE_UNKNOWN_RELATION, {
          relation: key,
        }),
      });
      continue;
    }

    // Truly unknown keyword in obligation
    errors.push({
      file,
      line: doc.line,
      code: ErrorCode.NORMATIVITY_UNKNOWN,
      message: messageFor(ErrorCode.NORMATIVITY_UNKNOWN, { keyword: key }),
    });
  }

  // missing_normativity_or_ref
  if (!hasNormativity && !hasReference) {
    errors.push({
      file,
      line: doc.line,
      code: ErrorCode.OBLIGATION_MISSING_NORMATIVITY_OR_REF,
      message: messageFor(ErrorCode.OBLIGATION_MISSING_NORMATIVITY_OR_REF),
    });
  }

  // guard_without_normativity
  if (hasGuard && !hasNormativity) {
    errors.push({
      file,
      line: doc.line,
      code: ErrorCode.OBLIGATION_GUARD_WITHOUT_NORMATIVITY,
      message: messageFor(ErrorCode.OBLIGATION_GUARD_WITHOUT_NORMATIVITY),
    });
  }
}

// ---------------------------------------------------------------------------
// CheckUniqueness
// ---------------------------------------------------------------------------

/**
 * Check for duplicate spec names within a file.
 * Returns errors for each duplicate occurrence (not the first).
 */
function checkUniqueness(
  documents: YamlDocument[],
  file: string,
): ValidationError[] {
  const errors: ValidationError[] = [];
  const seen = new Map<string, number>(); // name -> first-occurrence doc index

  for (let i = 1; i < documents.length; i++) {
    const doc = documents[i]!;
    const specVal = doc.data.spec;
    if (typeof specVal !== "string" || specVal === "") continue;

    if (seen.has(specVal)) {
      errors.push({
        file,
        line: doc.line,
        code: ErrorCode.SPEC_DUPLICATE_NAME,
        message: messageFor(ErrorCode.SPEC_DUPLICATE_NAME, { name: specVal }),
      });
    } else {
      seen.set(specVal, i);
    }
  }

  return errors;
}

// ---------------------------------------------------------------------------
// CheckRefs
// ---------------------------------------------------------------------------

/**
 * Parse a reference target string into its components.
 */
interface RefTarget {
  filePath?: string; // path before @, if present
  specName: string; // spec name portion
  slot?: string; // slot after ::, if present
}

function parseRefTarget(target: string): RefTarget | null {
  const match = REF_GRAMMAR_RE.exec(target);
  if (!match) return null;

  let filePath: string | undefined;
  if (match[1]) {
    // Remove trailing @
    filePath = match[1].slice(0, -1);
  }

  // Extract spec name: everything between (optional file@) and (optional ::SLOT)
  let rest = target;
  if (filePath !== undefined) {
    rest = rest.slice(filePath.length + 1);
  }
  const slotIdx = rest.indexOf("::");
  let specName: string;
  let slot: string | undefined;
  if (slotIdx >= 0) {
    specName = rest.slice(0, slotIdx);
    slot = rest.slice(slotIdx + 2);
  } else {
    specName = rest;
  }

  return { filePath, specName, slot };
}

/**
 * Collect all reference targets from the spec documents in a file.
 * Returns an array of { target, docLine } tuples.
 */
function collectRefs(
  documents: YamlDocument[],
): Array<{ target: string; docLine: number }> {
  const refs: Array<{ target: string; docLine: number }> = [];

  for (let i = 1; i < documents.length; i++) {
    const doc = documents[i]!;
    for (const key of Object.keys(doc.data)) {
      if (!isSlotKeyword(key)) continue;
      const val = doc.data[key];
      if (!Array.isArray(val)) continue;

      for (const obligation of val) {
        if (
          obligation === null ||
          obligation === undefined ||
          typeof obligation !== "object" ||
          Array.isArray(obligation)
        ) {
          continue;
        }
        const ob = obligation as Record<string, unknown>;
        for (const obKey of Object.keys(ob)) {
          if ((REFERENCE_KEYWORDS as readonly string[]).includes(obKey)) {
            const target = ob[obKey];
            if (typeof target === "string") {
              refs.push({ target, docLine: doc.line });
            }
          }
        }
      }
    }
  }

  return refs;
}

/**
 * Validate references in a file.
 * Resolves same-file refs, cross-file refs, and slot refs.
 */
function checkRefs(
  documents: YamlDocument[],
  file: string,
  projectRoot: string,
  parsedFileCache: Map<string, ParsedFileInfo>,
): ValidationError[] {
  const errors: ValidationError[] = [];
  const refs = collectRefs(documents);
  if (refs.length === 0) return errors;

  // Build set of spec names in this file
  const localSpecs = new Map<string, Set<string>>(); // specName -> declared slots
  for (let i = 1; i < documents.length; i++) {
    const doc = documents[i]!;
    const specVal = doc.data.spec;
    if (typeof specVal !== "string" || specVal === "") continue;
    const slots = new Set<string>();
    for (const key of Object.keys(doc.data)) {
      if (isSlotKeyword(key)) {
        slots.add(key);
      }
    }
    // Only record first occurrence
    if (!localSpecs.has(specVal)) {
      localSpecs.set(specVal, slots);
    }
  }

  // Track which cross-file paths we've already emitted file_not_found /
  // file_not_parseable for: at most one per (referencing-file, referenced-file)
  const fileErrorEmitted = new Set<string>();

  for (const { target, docLine } of refs) {
    // Check grammar
    const parsed = parseRefTarget(target);
    if (!parsed) {
      errors.push({
        file,
        line: docLine,
        code: ErrorCode.REF_MALFORMED,
        message: messageFor(ErrorCode.REF_MALFORMED, { target }),
      });
      continue;
    }

    // Check slot name if present
    if (parsed.slot !== undefined) {
      if (!isSlotKeyword(parsed.slot)) {
        errors.push({
          file,
          line: docLine,
          code: ErrorCode.REF_UNKNOWN_SLOT,
          message: messageFor(ErrorCode.REF_UNKNOWN_SLOT, { slot: parsed.slot }),
        });
        continue;
      }
    }

    if (parsed.filePath === undefined) {
      // Same-file ref
      const specSlots = localSpecs.get(parsed.specName);
      if (specSlots === undefined) {
        errors.push({
          file,
          line: docLine,
          code: ErrorCode.REF_SPEC_NOT_FOUND_SAME_FILE,
          message: messageFor(ErrorCode.REF_SPEC_NOT_FOUND_SAME_FILE, {
            target,
          }),
        });
        continue;
      }

      // Check slot declared
      if (parsed.slot !== undefined && !specSlots.has(parsed.slot)) {
        errors.push({
          file,
          line: docLine,
          code: ErrorCode.REF_SLOT_NOT_DECLARED,
          message: messageFor(ErrorCode.REF_SLOT_NOT_DECLARED, { target }),
        });
      }
    } else {
      // Cross-file ref
      const refFilePath = resolveRefFilePath(
        parsed.filePath,
        file,
        projectRoot,
      );

      // Check if we already emitted a file-level error for this path
      if (fileErrorEmitted.has(refFilePath)) {
        continue;
      }

      // Try to load the referenced file
      const refInfo = loadRefFile(refFilePath, parsedFileCache);

      if (refInfo === null) {
        // File not found
        errors.push({
          file,
          line: docLine,
          code: ErrorCode.REF_FILE_NOT_FOUND,
          message: messageFor(ErrorCode.REF_FILE_NOT_FOUND, {
            target: parsed.filePath,
          }),
        });
        fileErrorEmitted.add(refFilePath);
        continue;
      }

      if (refInfo.result.ok === false) {
        // File not parseable
        errors.push({
          file,
          line: docLine,
          code: ErrorCode.REF_FILE_NOT_PARSEABLE,
          message: messageFor(ErrorCode.REF_FILE_NOT_PARSEABLE, {
            target: parsed.filePath,
          }),
        });
        fileErrorEmitted.add(refFilePath);
        continue;
      }

      // File parsed OK, check spec name exists
      if (!refInfo.specNames.includes(parsed.specName)) {
        errors.push({
          file,
          line: docLine,
          code: ErrorCode.REF_SPEC_NOT_FOUND_OTHER_FILE,
          message: messageFor(ErrorCode.REF_SPEC_NOT_FOUND_OTHER_FILE, {
            target,
          }),
        });
        continue;
      }

      // Check slot declared
      if (parsed.slot !== undefined) {
        const specSlots = refInfo.specSlots.get(parsed.specName);
        if (!specSlots || !specSlots.has(parsed.slot)) {
          errors.push({
            file,
            line: docLine,
            code: ErrorCode.REF_SLOT_NOT_DECLARED,
            message: messageFor(ErrorCode.REF_SLOT_NOT_DECLARED, { target }),
          });
        }
      }
    }
  }

  return errors;
}

/**
 * Resolve a cross-file ref path to an absolute filesystem path.
 *
 * - "./path" or "../path" -> relative to referencing file's directory
 * - "path" (no leading dot) -> relative to project root
 * - Append ".yass.yaml"
 */
function resolveRefFilePath(
  refPath: string,
  referencingFile: string,
  projectRoot: string,
): string {
  const withExt = refPath + SPEC_SUFFIX;
  if (refPath.startsWith("./") || refPath.startsWith("../")) {
    // Relative to referencing file's directory
    return resolve(dirname(referencingFile), withExt);
  }
  // Relative to project root
  return resolve(projectRoot, withExt);
}

/**
 * Load and cache a referenced file for ref resolution.
 * Returns null if the file doesn't exist.
 */
function loadRefFile(
  absPath: string,
  cache: Map<string, ParsedFileInfo>,
): ParsedFileInfo | null {
  if (cache.has(absPath)) {
    return cache.get(absPath)!;
  }

  // Check if file exists
  try {
    const stat = statSync(absPath, { throwIfNoEntry: false });
    if (!stat || !stat.isFile()) {
      return null;
    }
  } catch {
    return null;
  }

  const result = parseYamlFile(absPath);
  const info: ParsedFileInfo = {
    result,
    specNames: [],
    specSlots: new Map(),
  };

  if (result.ok) {
    for (let i = 1; i < result.documents.length; i++) {
      const doc = result.documents[i]!;
      const specVal = doc.data.spec;
      if (typeof specVal === "string" && specVal !== "") {
        info.specNames.push(specVal);
        const slots = new Set<string>();
        for (const key of Object.keys(doc.data)) {
          if (isSlotKeyword(key)) {
            slots.add(key);
          }
        }
        if (!info.specSlots.has(specVal)) {
          info.specSlots.set(specVal, slots);
        }
      }
    }
  }

  cache.set(absPath, info);
  return info;
}

// ---------------------------------------------------------------------------
// Input processing
// ---------------------------------------------------------------------------

/**
 * Resolve input arguments to a list of files to validate.
 * Returns { files, errors, exitCode } where exitCode is set on fatal errors.
 */
function resolveInputFiles(
  args: string[],
  cwd: string,
  projectRoot: string,
): {
  files: string[];
  errors: ValidationError[];
  exitCode?: number;
} {
  const errors: ValidationError[] = [];

  // No arguments: discover from project root
  if (args.length === 0) {
    const result = discoverSpecFiles(undefined, projectRoot, cwd);
    if ("code" in result && !("files" in result)) {
      const discErr = result as DiscoverError;
      errors.push({
        file: discErr.file ?? "",
        code: discErr.code,
        message: discErr.message,
      });
      return { files: [], errors, exitCode: 2 };
    }
    const discResult = result as DiscoverResult;
    // Convert relative paths to absolute for internal use
    const files = discResult.files.map((f) => normalizePath(f, cwd));
    for (const e of discResult.errors) {
      errors.push({
        file: e.file,
        code: e.code,
        message: e.message,
      });
    }
    return { files, errors };
  }

  // Process each argument
  const allFiles: string[] = [];

  for (const arg of args) {
    // Check for colon in path
    if (arg.includes(":")) {
      errors.push({
        file: "",
        code: ErrorCode.PATH_COLON_IN_PATH,
        message: messageFor(ErrorCode.PATH_COLON_IN_PATH, { path: arg }),
      });
      return { files: [], errors, exitCode: 2 };
    }

    // Expand globs first
    if (isGlobPattern(arg)) {
      const globResult = expandGlob(arg, cwd);
      if (isGlobError(globResult)) {
        errors.push({
          file: "",
          code: globResult.code,
          message: globResult.message,
        });
        return { files: [], errors, exitCode: 2 };
      }

      // Filter to only .yass.yaml files from glob results (skip non-.yass.yaml silently)
      for (const p of globResult.paths) {
        const absPath = normalizePath(p, cwd);
        // Check if it's a directory or a file
        try {
          const stat = statSync(absPath, { throwIfNoEntry: false });
          if (!stat) continue;
          if (stat.isDirectory()) {
            // Discover in directory
            const discResult = discoverSpecFiles(absPath, projectRoot, cwd);
            if ("code" in discResult && !("files" in discResult)) {
              const discErr = discResult as DiscoverError;
              errors.push({
                file: discErr.file ?? "",
                code: discErr.code,
                message: discErr.message,
              });
              continue;
            }
            const dr = discResult as DiscoverResult;
            for (const f of dr.files) {
              allFiles.push(normalizePath(f, cwd));
            }
            for (const e of dr.errors) {
              errors.push({ file: e.file, code: e.code, message: e.message });
            }
          } else if (stat.isFile()) {
            // Only include .yass.yaml files from glob expansion
            if (basename(absPath).endsWith(SPEC_SUFFIX)) {
              allFiles.push(absPath);
            }
            // Skip non-.yass.yaml silently
          }
        } catch {
          continue;
        }
      }
      continue;
    }

    // Non-glob argument: resolve path
    const absPath = normalizePath(arg, cwd);

    try {
      const stat = statSync(absPath, { throwIfNoEntry: false });
      if (!stat) {
        errors.push({
          file: "",
          code: ErrorCode.PATH_NOT_FOUND,
          message: messageFor(ErrorCode.PATH_NOT_FOUND, { path: arg }),
        });
        return { files: [], errors, exitCode: 2 };
      }

      if (stat.isDirectory()) {
        // Discover in directory
        const discResult = discoverSpecFiles(absPath, projectRoot, cwd);
        if ("code" in discResult && !("files" in discResult)) {
          const discErr = discResult as DiscoverError;
          errors.push({
            file: discErr.file ?? "",
            code: discErr.code,
            message: discErr.message,
          });
          return { files: [], errors, exitCode: 2 };
        }
        const dr = discResult as DiscoverResult;
        for (const f of dr.files) {
          allFiles.push(normalizePath(f, cwd));
        }
        for (const e of dr.errors) {
          errors.push({ file: e.file, code: e.code, message: e.message });
        }
      } else if (stat.isFile()) {
        // Validate extension
        if (!basename(absPath).endsWith(SPEC_SUFFIX)) {
          errors.push({
            file: "",
            code: ErrorCode.PATH_BAD_EXTENSION,
            message: messageFor(ErrorCode.PATH_BAD_EXTENSION, { path: arg }),
          });
          return { files: [], errors, exitCode: 2 };
        }
        allFiles.push(absPath);
      } else {
        errors.push({
          file: "",
          code: ErrorCode.PATH_INVALID_TYPE,
          message: messageFor(ErrorCode.PATH_INVALID_TYPE, { path: arg }),
        });
        return { files: [], errors, exitCode: 2 };
      }
    } catch {
      errors.push({
        file: "",
        code: ErrorCode.PATH_NOT_FOUND,
        message: messageFor(ErrorCode.PATH_NOT_FOUND, { path: arg }),
      });
      return { files: [], errors, exitCode: 2 };
    }
  }

  // Deduplicate by normalized absolute path
  const seen = new Set<string>();
  const deduped: string[] = [];
  for (const f of allFiles) {
    if (!seen.has(f)) {
      seen.add(f);
      deduped.push(f);
    }
  }

  // Sort by normalized path
  deduped.sort((a, b) => {
    const na = a.normalize("NFC");
    const nb = b.normalize("NFC");
    return na < nb ? -1 : na > nb ? 1 : 0;
  });

  return { files: deduped, errors };
}

// ---------------------------------------------------------------------------
// Main validate pipeline
// ---------------------------------------------------------------------------

/**
 * Run the full validation pipeline on a single file.
 * Returns all validation errors found.
 */
function validateFile(
  absPath: string,
  projectRoot: string,
  cwd: string,
  parsedFileCache: Map<string, ParsedFileInfo>,
): ValidationError[] {
  const errors: ValidationError[] = [];

  // Phase 1: CheckYAML
  const parseResult = parseYamlFile(absPath);
  if (!parseResult.ok) {
    errors.push({
      file: absPath,
      line: parseResult.line,
      code: parseResult.code,
      message: parseResult.message,
    });
    return errors; // Skip remaining checks on parse failure
  }

  const { documents } = parseResult;

  // Phase 2: CheckPreamble
  const preambleError = checkPreamble(documents, absPath);
  if (preambleError) {
    errors.push(preambleError);
  }

  // Phase 3: CheckSpec (for each non-first document)
  for (let i = 1; i < documents.length; i++) {
    const doc = documents[i]!;
    const specErrors = checkSpec(doc, absPath);
    errors.push(...specErrors);
  }

  // Phase 4: CheckUniqueness
  const uniqueErrors = checkUniqueness(documents, absPath);
  errors.push(...uniqueErrors);

  // Phase 5: CheckRefs
  const refErrors = checkRefs(documents, absPath, projectRoot, parsedFileCache);
  errors.push(...refErrors);

  return errors;
}

// ---------------------------------------------------------------------------
// Command entry point
// ---------------------------------------------------------------------------

/**
 * The main validate handler -- called by CLI dispatch.
 * Returns exit code: 0 = success, 1 = validation errors, 2 = usage/input errors.
 */
export function validateCommand(args: string[], cwd: string): number {
  // Compute project root EXACTLY ONCE at startup
  const rootResult = findProjectRoot(cwd);
  if (isFindRootError(rootResult)) {
    // Emit error line and exit 2 before checking files
    const line = formatErrorLine(
      {
        code: rootResult.code,
        message: rootResult.message,
      },
      cwd,
    );
    process.stderr.write(line + "\n");
    return 2;
  }

  const projectRoot = rootResult.root;

  // Resolve input files
  const { files, errors: inputErrors, exitCode } = resolveInputFiles(
    args,
    cwd,
    projectRoot,
  );

  // If there's a fatal input error, emit it and return
  if (exitCode !== undefined) {
    for (const err of inputErrors) {
      const line = formatErrorLine(
        {
          file: err.file || undefined,
          line: err.line,
          code: err.code,
          message: err.message,
        },
        cwd,
      );
      process.stderr.write(line + "\n");
    }
    return exitCode;
  }

  // Emit non-fatal input errors (e.g. unreadable directories)
  let totalErrors = 0;
  for (const err of inputErrors) {
    const line = formatErrorLine(
      {
        file: err.file || undefined,
        line: err.line,
        code: err.code,
        message: err.message,
      },
      cwd,
    );
    process.stderr.write(line + "\n");
    totalErrors++;
  }

  // Process files sequentially
  const parsedFileCache = new Map<string, ParsedFileInfo>();
  const fileCount = files.length;

  for (const absPath of files) {
    const fileErrors = validateFile(absPath, projectRoot, cwd, parsedFileCache);

    // Sort errors within file by (line, code)
    fileErrors.sort((a, b) => {
      const lineA = a.line ?? 0;
      const lineB = b.line ?? 0;
      if (lineA !== lineB) return lineA - lineB;
      return a.code < b.code ? -1 : a.code > b.code ? 1 : 0;
    });

    for (const err of fileErrors) {
      const line = formatErrorLine(
        {
          file: err.file || undefined,
          line: err.line,
          code: err.code,
          message: err.message,
        },
        cwd,
      );
      process.stderr.write(line + "\n");
      totalErrors++;
    }
  }

  // Flush stderr before stdout summary
  // (In practice, synchronous writes complete immediately in Bun)

  // Summary line
  const summary = `checked ${fileCount} file${fileCount === 1 ? "" : "s"}, found ${totalErrors} error${totalErrors === 1 ? "" : "s"}`;
  process.stdout.write(summary + "\n");

  return totalErrors > 0 ? 1 : 0;
}
