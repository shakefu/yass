import { describe, expect, test } from "bun:test";
import { formatErrorLine, formatFilePath } from "../src/error-line.ts";
import type { ErrorLineInput } from "../src/error-line.ts";

// ---------------------------------------------------------------------------
// formatFilePath
// ---------------------------------------------------------------------------

describe("formatFilePath", () => {
  test("returns relative path for file under cwd", () => {
    expect(formatFilePath("/home/user/project/src/foo.yass", "/home/user/project"))
      .toBe("src/foo.yass");
  });

  test("returns basename alone for file directly inside cwd", () => {
    expect(formatFilePath("/home/user/project/foo.yass", "/home/user/project"))
      .toBe("foo.yass");
  });

  test("returns absolute path for file outside cwd", () => {
    expect(formatFilePath("/other/dir/foo.yass", "/home/user/project"))
      .toBe("/other/dir/foo.yass");
  });

  test("does not emit leading './'", () => {
    const result = formatFilePath("/cwd/file.yass", "/cwd");
    expect(result).toBe("file.yass");
    expect(result.startsWith("./")).toBe(false);
  });

  test("does not emit leading './' for nested paths", () => {
    const result = formatFilePath("/cwd/a/b/c.yass", "/cwd");
    expect(result).toBe("a/b/c.yass");
    expect(result.startsWith("./")).toBe(false);
  });

  test("uses forward slashes for backslash input", () => {
    expect(formatFilePath("C:\\Users\\dev\\project\\src\\foo.yass", "C:\\Users\\dev\\project"))
      .toBe("src/foo.yass");
  });

  test("returns absolute with forward slashes when outside cwd (backslash input)", () => {
    expect(formatFilePath("C:\\Other\\foo.yass", "C:\\Users\\dev\\project"))
      .toBe("C:/Other/foo.yass");
  });

  test("treats cwd as strict prefix — no partial directory match", () => {
    // /home/user/project-extra is NOT under /home/user/project
    expect(formatFilePath("/home/user/project-extra/foo.yass", "/home/user/project"))
      .toBe("/home/user/project-extra/foo.yass");
  });

  test("handles cwd with trailing slash", () => {
    expect(formatFilePath("/cwd/foo.yass", "/cwd/"))
      .toBe("foo.yass");
  });

  test("handles deeply nested relative path", () => {
    expect(formatFilePath("/cwd/a/b/c/d/e.yass", "/cwd"))
      .toBe("a/b/c/d/e.yass");
  });

  test("file equal to cwd is returned as absolute (not under cwd/)", () => {
    expect(formatFilePath("/cwd", "/cwd"))
      .toBe("/cwd");
  });
});

// ---------------------------------------------------------------------------
// formatErrorLine
// ---------------------------------------------------------------------------

describe("formatErrorLine", () => {
  const cwd = "/home/user/project";

  test("file + line: standard format", () => {
    const input: ErrorLineInput = {
      file: "/home/user/project/src/foo.yass",
      line: 42,
      code: "E001",
      message: "unexpected token",
    };
    expect(formatErrorLine(input, cwd))
      .toBe("src/foo.yass:42: [E001] unexpected token");
  });

  test("file without line number", () => {
    const input: ErrorLineInput = {
      file: "/home/user/project/src/bar.yass",
      code: "E002",
      message: "file is empty",
    };
    expect(formatErrorLine(input, cwd))
      .toBe("src/bar.yass: [E002] file is empty");
  });

  test("no file — uses 'yass' token", () => {
    const input: ErrorLineInput = {
      code: "E099",
      message: "something went wrong",
    };
    expect(formatErrorLine(input, cwd))
      .toBe("yass: [E099] something went wrong");
  });

  test("empty string file — uses 'yass' token", () => {
    const input: ErrorLineInput = {
      file: "",
      code: "E099",
      message: "something went wrong",
    };
    expect(formatErrorLine(input, cwd))
      .toBe("yass: [E099] something went wrong");
  });

  test("no file with line number — still uses 'yass' with line", () => {
    const input: ErrorLineInput = {
      line: 5,
      code: "E010",
      message: "bad stuff",
    };
    expect(formatErrorLine(input, cwd))
      .toBe("yass:5: [E010] bad stuff");
  });

  test("file outside cwd — absolute path", () => {
    const input: ErrorLineInput = {
      file: "/other/dir/baz.yass",
      line: 1,
      code: "E003",
      message: "not found",
    };
    expect(formatErrorLine(input, cwd))
      .toBe("/other/dir/baz.yass:1: [E003] not found");
  });

  test("file directly inside cwd — basename only", () => {
    const input: ErrorLineInput = {
      file: "/home/user/project/root.yass",
      line: 10,
      code: "E004",
      message: "duplicate key",
    };
    expect(formatErrorLine(input, cwd))
      .toBe("root.yass:10: [E004] duplicate key");
  });

  // -----------------------------------------------------------------------
  // Newline replacement
  // -----------------------------------------------------------------------

  test("replaces LF in message with space", () => {
    const input: ErrorLineInput = {
      code: "E005",
      message: "line one\nline two",
    };
    expect(formatErrorLine(input, cwd))
      .toBe("yass: [E005] line one line two");
  });

  test("replaces CRLF in message with space", () => {
    const input: ErrorLineInput = {
      code: "E005",
      message: "line one\r\nline two",
    };
    expect(formatErrorLine(input, cwd))
      .toBe("yass: [E005] line one line two");
  });

  test("replaces multiple consecutive newlines with single space", () => {
    const input: ErrorLineInput = {
      code: "E005",
      message: "a\n\n\nb",
    };
    expect(formatErrorLine(input, cwd))
      .toBe("yass: [E005] a b");
  });

  test("replaces CR in message with space", () => {
    const input: ErrorLineInput = {
      code: "E005",
      message: "a\rb",
    };
    expect(formatErrorLine(input, cwd))
      .toBe("yass: [E005] a b");
  });

  // -----------------------------------------------------------------------
  // No trailing newline
  // -----------------------------------------------------------------------

  test("does not include trailing newline", () => {
    const result = formatErrorLine({ code: "E000", message: "m" }, cwd);
    expect(result.endsWith("\n")).toBe(false);
    expect(result.endsWith("\r")).toBe(false);
  });

  // -----------------------------------------------------------------------
  // No ANSI escape codes
  // -----------------------------------------------------------------------

  test("output contains no ANSI escape codes", () => {
    const input: ErrorLineInput = {
      file: "/home/user/project/x.yass",
      line: 1,
      code: "E006",
      message: "some error",
    };
    const result = formatErrorLine(input, cwd);
    // eslint-disable-next-line no-control-regex
    expect(result).not.toMatch(/\x1b\[/);
  });

  // -----------------------------------------------------------------------
  // Forward slashes
  // -----------------------------------------------------------------------

  test("uses forward slashes on backslash paths", () => {
    const input: ErrorLineInput = {
      file: "C:\\Users\\dev\\project\\src\\foo.yass",
      line: 3,
      code: "E007",
      message: "oops",
    };
    const result = formatErrorLine(input, "C:\\Users\\dev\\project");
    expect(result).toBe("src/foo.yass:3: [E007] oops");
    expect(result).not.toContain("\\");
  });

  // -----------------------------------------------------------------------
  // Edge cases
  // -----------------------------------------------------------------------

  test("line number 0 is still emitted (spec says 1-based but we format whatever is given)", () => {
    const input: ErrorLineInput = {
      file: "/home/user/project/f.yass",
      line: 0,
      code: "E008",
      message: "huh",
    };
    expect(formatErrorLine(input, cwd))
      .toBe("f.yass:0: [E008] huh");
  });

  test("very large line number", () => {
    const input: ErrorLineInput = {
      file: "/home/user/project/f.yass",
      line: 999999,
      code: "E008",
      message: "big",
    };
    expect(formatErrorLine(input, cwd))
      .toBe("f.yass:999999: [E008] big");
  });

  test("empty message", () => {
    const input: ErrorLineInput = {
      code: "E009",
      message: "",
    };
    expect(formatErrorLine(input, cwd))
      .toBe("yass: [E009] ");
  });

  test("message with only newlines collapses to single space", () => {
    const input: ErrorLineInput = {
      code: "E009",
      message: "\n\n",
    };
    expect(formatErrorLine(input, cwd))
      .toBe("yass: [E009]  ");
  });

  // -----------------------------------------------------------------------
  // cwd default
  // -----------------------------------------------------------------------

  test("uses process.cwd() when cwd argument is omitted", () => {
    const input: ErrorLineInput = {
      file: process.cwd() + "/test-file.yass",
      code: "E010",
      message: "fallback cwd",
    };
    const result = formatErrorLine(input);
    expect(result).toBe("test-file.yass: [E010] fallback cwd");
  });
});
