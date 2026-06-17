/**
 * ErrorLine formatter — every CLI error line follows a strict format.
 *
 * With a known source line:   `<file>:<line>: [<code>] <message>`
 * Without a known source line: `<file>: [<code>] <message>`
 *
 * When there is no associated file the literal token "yass" is used.
 */

export interface ErrorLineInput {
  /** Absolute file path, or undefined for no-file errors. */
  file?: string;
  /** 1-based line number, or undefined when unknown. */
  line?: number;
  /** Error code (e.g. from cli.errors). */
  code: string;
  /** Human-readable message. */
  message: string;
}

/**
 * Format an absolute file path relative to `cwd` per the spec rules:
 *
 * - Emit relative when the path starts with cwd + "/"
 * - Emit the basename alone when the file is directly inside cwd
 * - Emit absolute when the file is NOT under cwd
 * - Use forward slashes on all platforms
 * - Do NOT resolve symbolic links
 * - No leading "./" on relative paths
 */
export function formatFilePath(absolutePath: string, cwd: string): string {
  // Normalize separators to forward slashes (for Windows compat).
  const normPath = absolutePath.replaceAll("\\", "/");
  const normCwd = cwd.replaceAll("\\", "/");

  // The path must start with cwd + "/" to be considered "under" cwd.
  const prefix = normCwd.endsWith("/") ? normCwd : normCwd + "/";

  if (!normPath.startsWith(prefix)) {
    return normPath;
  }

  // Strip the cwd prefix to get the relative portion.
  const relative = normPath.slice(prefix.length);

  // The relative part should never start with "./" — it won't given how we
  // slice, but guard defensively.
  return relative;
}

/**
 * Format a single error line string (without trailing newline).
 *
 * @param input  The error details.
 * @param cwd    The current working directory (defaults to process.cwd()).
 */
export function formatErrorLine(
  input: ErrorLineInput,
  cwd?: string,
): string {
  const resolvedCwd = cwd ?? process.cwd();

  // Determine the file token.
  let fileToken: string;
  if (input.file === undefined || input.file === "") {
    fileToken = "yass";
  } else {
    fileToken = formatFilePath(input.file, resolvedCwd);
  }

  // Sanitize the message: replace any newline characters with a single space.
  const sanitizedMessage = input.message.replace(/[\r\n]+/g, " ");

  // Build the line.
  if (input.line !== undefined) {
    return `${fileToken}:${input.line}: [${input.code}] ${sanitizedMessage}`;
  }
  return `${fileToken}: [${input.code}] ${sanitizedMessage}`;
}
