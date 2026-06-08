/**
 * List subcommand — enumerate specs from .yass.yaml files.
 *
 * Output: one line per spec, tab-separated fields:
 *   <file-path>\t<spec-name>\t<description>
 *
 * - Description is whitespace-normalised and NFC-normalised.
 * - When stdout is a TTY, descriptions are truncated on grapheme-cluster
 *   boundaries with a "..." marker.
 */

import { resolve, isAbsolute } from "node:path";
import { ErrorCode, exitCodeFor, messageFor } from "./errors.ts";
import { formatErrorLine } from "./error-line.ts";
import { findProjectRoot } from "./find-root.ts";
import { discoverSpecFiles } from "./discover.ts";
import type { DiscoverResult, DiscoverError } from "./discover.ts";
import { parseYamlFile } from "./yaml-parser.ts";
import type { ParseSuccess, ParseError } from "./yaml-parser.ts";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/** Minimal writable interface accepted by listCommand. */
type Writer = { write(s: string): void };

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Normalise a preamble description value:
 * - Non-string / null / undefined -> empty string
 * - Collapse runs of whitespace (including newlines, tabs) to a single space
 * - Trim leading/trailing whitespace
 * - NFC-normalise
 */
function normaliseDescription(raw: unknown): string {
  if (typeof raw !== "string") return "";
  const collapsed = raw.replace(/\s+/g, " ").trim();
  return collapsed.normalize("NFC");
}

/**
 * Sanitise a file path for the output row: replace literal tab characters
 * with a single space.
 */
function sanitiseFilePath(p: string): string {
  return p.replace(/\t/g, " ");
}

/**
 * Determine the terminal width to use for TTY truncation.
 *
 * Priority:
 *  1. COLUMNS env var if numeric and > 0
 *  2. OS terminal width (stdout.columns in Node/Bun)
 *  3. Default 80
 */
function getTerminalWidth(): number {
  const envCols = process.env["COLUMNS"];
  if (envCols !== undefined && envCols !== "") {
    const n = Number(envCols);
    if (Number.isFinite(n) && n > 0) {
      return Math.floor(n);
    }
  }
  // Query OS terminal width
  if (
    typeof process.stdout === "object" &&
    process.stdout !== null &&
    typeof (process.stdout as NodeJS.WriteStream).columns === "number"
  ) {
    const cols = (process.stdout as NodeJS.WriteStream).columns;
    if (cols > 0) return cols;
  }
  return 80;
}

/**
 * Segment a string into grapheme clusters using Intl.Segmenter.
 */
function graphemeClusters(s: string): string[] {
  const segmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" });
  return Array.from(segmenter.segment(s), (seg) => seg.segment);
}

/**
 * Truncate a description to fit within `maxLen` visible characters,
 * breaking on grapheme-cluster boundaries. Uses "..." as truncation marker.
 *
 * Returns the (possibly truncated) description string.
 *
 * - Empty description -> return "" (no marker)
 * - If full description fits -> return it unchanged
 * - If maxLen < marker length -> return "" (no marker)
 */
function truncateDescription(desc: string, maxLen: number): string {
  if (desc === "") return "";
  // Segment into grapheme clusters for accurate measurement
  const clusters = graphemeClusters(desc);
  if (clusters.length <= maxLen) return desc;
  const marker = "...";
  const markerLen = marker.length; // 3
  if (maxLen < markerLen) return "";
  // Take clusters up to maxLen - markerLen
  const keep = maxLen - markerLen;
  return clusters.slice(0, keep).join("") + marker;
}

/**
 * Check whether a path contains a colon character.
 */
function hasColon(p: string): boolean {
  return p.includes(":");
}

// ---------------------------------------------------------------------------
// Main entry point
// ---------------------------------------------------------------------------

/**
 * Execute the `list` subcommand.
 *
 * @param args   Positional arguments (after subcommand name).
 * @param cwd    Current working directory.
 * @param stdout Writable stream for normal output.
 * @param stderr Writable stream for error output.
 * @param isTTY  Whether stdout is a TTY (controls truncation).
 * @returns      Process exit code (0 = success, 1 = processing error, 2 = usage error).
 */
export function listCommand(
  args: string[],
  cwd: string,
  stdout: Writer,
  stderr: Writer,
  isTTY: boolean,
): number {
  // ------------------------------------------------------------------
  // 1. Determine the path argument (at most one positional)
  // ------------------------------------------------------------------
  const pathArg: string | undefined = args[0];

  // Reject colon in path
  if (pathArg !== undefined && hasColon(pathArg)) {
    const code = ErrorCode.PATH_COLON_IN_PATH;
    const msg = messageFor(code, { path: pathArg });
    stderr.write(formatErrorLine({ code, message: msg }) + "\n");
    return exitCodeFor(code);
  }

  // Do NOT treat "-" as stdin marker (spec says so — just pass it through
  // to discover which will stat it and produce path.not_found).

  // ------------------------------------------------------------------
  // 2. Find project root (needed for discover default)
  // ------------------------------------------------------------------
  const rootResult = findProjectRoot(cwd);
  if ("code" in rootResult) {
    // If we have a path arg, we can still try discover with cwd as root.
    // But the spec says route ALL input through discover, so if no root
    // and no path, we just get an error from discover.
    // Actually, we need a projectRoot for discover. If no root found
    // and no explicit path, that's fine — discover will use projectRoot
    // which won't exist, but let's handle it gracefully.
    if (pathArg === undefined) {
      // No path arg and no project root — no files to list
      return 0;
    }
  }
  const projectRoot = "root" in rootResult ? rootResult.root : cwd;

  // ------------------------------------------------------------------
  // 3. Discover spec files
  // ------------------------------------------------------------------
  const discoverResult = discoverSpecFiles(pathArg, projectRoot, cwd);

  // Fatal discover error -> emit and exit
  if ("code" in discoverResult && !("files" in discoverResult)) {
    const err = discoverResult as DiscoverError;
    const code = err.code;
    const exit = exitCodeFor(code);
    stderr.write(
      formatErrorLine({ file: err.file, code, message: err.message }, cwd) +
        "\n",
    );
    return exit;
  }

  const discovered = discoverResult as DiscoverResult;

  // No files found -> exit 0 silently
  if (discovered.files.length === 0) {
    return 0;
  }

  // ------------------------------------------------------------------
  // 4. Parse each file and collect spec rows
  // ------------------------------------------------------------------
  interface SpecRow {
    filePath: string;
    name: string;
    description: string;
  }

  const rows: SpecRow[] = [];
  let hadParseFailure = false;

  // Files are already sorted by discover (Unicode code-point order on
  // NFC-normalised paths). We need to resolve them to absolute paths for
  // parsing, but emit the formatted (possibly relative) path.
  for (const filePath of discovered.files) {
    const absPath = isAbsolute(filePath) ? filePath : resolve(cwd, filePath);
    const parseResult = parseYamlFile(absPath);

    if (!parseResult.ok) {
      // YAML parse failure — report and continue
      hadParseFailure = true;
      const pe = parseResult as ParseError;
      stderr.write(
        formatErrorLine(
          { file: absPath, line: pe.line, code: pe.code, message: pe.message },
          cwd,
        ) + "\n",
      );
      continue;
    }

    const parsed = parseResult as ParseSuccess;
    const docs = parsed.documents;

    // First document is the preamble — extract its description for the
    // file-level description. Specs start from document index 1.
    if (docs.length === 0) continue;

    const preamble = docs[0]!;
    const fileDescription = normaliseDescription(preamble.data["description"]);

    // Walk spec documents (index 1+) in document order
    for (let i = 1; i < docs.length; i++) {
      const doc = docs[i]!;
      const specName = doc.data["spec"];
      if (typeof specName !== "string" || specName === "") continue;

      rows.push({
        filePath: sanitiseFilePath(filePath),
        name: specName,
        description: fileDescription,
      });
    }
  }

  // ------------------------------------------------------------------
  // 5. Emit output
  // ------------------------------------------------------------------
  const width = isTTY ? getTerminalWidth() : 0;

  for (const row of rows) {
    const { filePath, name, description } = row;

    if (!isTTY) {
      // No truncation
      stdout.write(`${filePath}\t${name}\t${description}\n`);
    } else {
      // TTY truncation
      // prefixLen = filePath + tab + name + tab
      const prefixLen = filePath.length + 1 + name.length + 1;
      const markerLen = 3; // "..."

      if (description === "") {
        // Empty description — emit with both tab separators, no marker
        stdout.write(`${filePath}\t${name}\t\n`);
      } else if (prefixLen + markerLen >= width) {
        // No room even for marker — emit empty third field, no marker
        stdout.write(`${filePath}\t${name}\t\n`);
      } else {
        const maxDescLen = width - prefixLen;
        const truncated = truncateDescription(description, maxDescLen);
        stdout.write(`${filePath}\t${name}\t${truncated}\n`);
      }
    }
  }

  // ------------------------------------------------------------------
  // 6. Return exit code
  // ------------------------------------------------------------------
  return hadParseFailure ? 1 : 0;
}
