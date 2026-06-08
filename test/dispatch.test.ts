import { describe, expect, it, mock, beforeEach } from "bun:test";
import { dispatch, type DispatchOptions } from "../src/dispatch.ts";
import { ErrorCode } from "../src/errors.ts";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Capture writes to a fake stream. */
function makeWriter(): { write(s: string): void; output: string } {
  const w = {
    output: "",
    write(s: string) {
      w.output += s;
    },
  };
  return w;
}

/** Build a DispatchOptions with sensible defaults. */
function opts(argv: string[], overrides: Partial<DispatchOptions> = {}): DispatchOptions {
  return {
    argv,
    cwd: process.cwd(),
    stdout: makeWriter(),
    stderr: makeWriter(),
    isTTY: false,
    version: "0.0.3",
    ...overrides,
  };
}

/** Extract captured stdout text. */
function stdoutOf(o: DispatchOptions): string {
  return (o.stdout as ReturnType<typeof makeWriter>).output;
}

/** Extract captured stderr text. */
function stderrOf(o: DispatchOptions): string {
  return (o.stderr as ReturnType<typeof makeWriter>).output;
}

// ---------------------------------------------------------------------------
// Usage text fragment (for matching)
// ---------------------------------------------------------------------------

const USAGE_FRAGMENT = "Usage: yass <command> [options]";

// ---------------------------------------------------------------------------
// No arguments
// ---------------------------------------------------------------------------

describe("dispatch: no arguments", () => {
  it("exits 2 and prints usage to stderr", () => {
    const o = opts([]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain(USAGE_FRAGMENT);
    expect(stderrOf(o)).toContain("yass.argv.no_subcommand");
    expect(stdoutOf(o)).toBe("");
  });
});

// ---------------------------------------------------------------------------
// --help
// ---------------------------------------------------------------------------

describe("dispatch: --help", () => {
  it("exits 0 and prints usage to stdout", () => {
    const o = opts(["--help"]);
    const code = dispatch(o);
    expect(code).toBe(0);
    expect(stdoutOf(o)).toContain(USAGE_FRAGMENT);
    expect(stderrOf(o)).toBe("");
  });

  it("--help takes priority over --version", () => {
    const o = opts(["--version", "--help"]);
    const code = dispatch(o);
    expect(code).toBe(0);
    expect(stdoutOf(o)).toContain(USAGE_FRAGMENT);
    expect(stdoutOf(o)).not.toContain("yass 0.0.3");
  });

  it("--help takes priority over unknown subcommand", () => {
    const o = opts(["bogus", "--help"]);
    const code = dispatch(o);
    expect(code).toBe(0);
    expect(stdoutOf(o)).toContain(USAGE_FRAGMENT);
  });

  it("--help anywhere in argv is recognized", () => {
    const o = opts(["validate", "some-path", "--help"]);
    const code = dispatch(o);
    expect(code).toBe(0);
    expect(stdoutOf(o)).toContain(USAGE_FRAGMENT);
  });

  it("--help after -- is still recognized (scanned globally)", () => {
    const o = opts(["--", "--help"]);
    const code = dispatch(o);
    expect(code).toBe(0);
    expect(stdoutOf(o)).toContain(USAGE_FRAGMENT);
  });
});

// ---------------------------------------------------------------------------
// --version
// ---------------------------------------------------------------------------

describe("dispatch: --version", () => {
  it("exits 0 and prints version to stdout", () => {
    const o = opts(["--version"]);
    const code = dispatch(o);
    expect(code).toBe(0);
    expect(stdoutOf(o)).toBe("yass 0.0.3\n");
    expect(stderrOf(o)).toBe("");
  });

  it("uses the version from options", () => {
    const o = opts(["--version"], { version: "1.2.3" });
    const code = dispatch(o);
    expect(code).toBe(0);
    expect(stdoutOf(o)).toBe("yass 1.2.3\n");
  });

  it("--version anywhere in argv is recognized", () => {
    const o = opts(["validate", "--version"]);
    const code = dispatch(o);
    expect(code).toBe(0);
    expect(stdoutOf(o)).toBe("yass 0.0.3\n");
  });
});

// ---------------------------------------------------------------------------
// Unknown subcommand
// ---------------------------------------------------------------------------

describe("dispatch: unknown subcommand", () => {
  it("exits 2 for a completely unknown word", () => {
    const o = opts(["frobnicate"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.unknown_subcommand");
    expect(stderrOf(o)).toContain("frobnicate");
    expect(stderrOf(o)).toContain(USAGE_FRAGMENT);
  });

  it("exits 2 for a numeric token", () => {
    const o = opts(["123"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.unknown_subcommand");
  });
});

// ---------------------------------------------------------------------------
// Short flag
// ---------------------------------------------------------------------------

describe("dispatch: short flag", () => {
  it("-h exits 2 with short_flag error", () => {
    const o = opts(["-h"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.short_flag");
    expect(stderrOf(o)).toContain("-h");
    expect(stderrOf(o)).toContain(USAGE_FRAGMENT);
  });

  it("-v exits 2 with short_flag error", () => {
    const o = opts(["-v"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.short_flag");
    expect(stderrOf(o)).toContain("-v");
  });

  it("-abc exits 2 with short_flag error", () => {
    const o = opts(["-abc"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.short_flag");
  });
});

// ---------------------------------------------------------------------------
// Case mismatch
// ---------------------------------------------------------------------------

describe("dispatch: case mismatch", () => {
  it("Validate exits 2 with case_mismatch error", () => {
    const o = opts(["Validate"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.case_mismatch");
    expect(stderrOf(o)).toContain("Validate");
    expect(stderrOf(o)).toContain(USAGE_FRAGMENT);
  });

  it("VALIDATE exits 2 with case_mismatch error", () => {
    const o = opts(["VALIDATE"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.case_mismatch");
  });

  it("Query exits 2 with case_mismatch error", () => {
    const o = opts(["Query"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.case_mismatch");
  });

  it("LIST exits 2 with case_mismatch error", () => {
    const o = opts(["LIST"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.case_mismatch");
  });

  it("--Help exits 2 with case_mismatch error", () => {
    const o = opts(["--Help"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.case_mismatch");
    expect(stderrOf(o)).toContain("--Help");
    expect(stderrOf(o)).toContain(USAGE_FRAGMENT);
  });

  it("--VERSION exits 2 with case_mismatch error", () => {
    const o = opts(["--VERSION"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.case_mismatch");
    expect(stderrOf(o)).toContain("--VERSION");
    expect(stderrOf(o)).toContain(USAGE_FRAGMENT);
  });

  it("--HELP exits 2 with case_mismatch error", () => {
    const o = opts(["--HELP"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.case_mismatch");
    expect(stderrOf(o)).toContain("--HELP");
    expect(stderrOf(o)).toContain(USAGE_FRAGMENT);
  });
});

// ---------------------------------------------------------------------------
// Abbreviation
// ---------------------------------------------------------------------------

describe("dispatch: abbreviation", () => {
  it("val exits 2 with abbreviation error", () => {
    const o = opts(["val"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.abbreviation");
    expect(stderrOf(o)).toContain("val");
    expect(stderrOf(o)).toContain(USAGE_FRAGMENT);
  });

  it("v exits 2 with abbreviation error", () => {
    const o = opts(["v"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.abbreviation");
  });

  it("q exits 2 with abbreviation error", () => {
    const o = opts(["q"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.abbreviation");
  });

  it("li exits 2 with abbreviation error", () => {
    const o = opts(["li"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.abbreviation");
  });

  it("lis exits 2 with abbreviation error", () => {
    const o = opts(["lis"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.abbreviation");
  });

  it("validat exits 2 with abbreviation error", () => {
    const o = opts(["validat"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.abbreviation");
  });

  it("quer exits 2 with abbreviation error", () => {
    const o = opts(["quer"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.abbreviation");
  });
});

// ---------------------------------------------------------------------------
// Empty argument
// ---------------------------------------------------------------------------

describe("dispatch: empty argument", () => {
  it("empty string exits 2 with empty_argument error", () => {
    const o = opts([""]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.empty_argument");
    expect(stderrOf(o)).toContain(USAGE_FRAGMENT);
  });
});

// ---------------------------------------------------------------------------
// Bare "-"
// ---------------------------------------------------------------------------

describe("dispatch: bare dash", () => {
  it("bare '-' exits 2 with stdin_dash error", () => {
    const o = opts(["-"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.stdin_dash");
    expect(stderrOf(o)).toContain(USAGE_FRAGMENT);
  });
});

// ---------------------------------------------------------------------------
// Unknown flag
// ---------------------------------------------------------------------------

describe("dispatch: unknown flag", () => {
  it("--verbose exits 2 with unknown_flag error", () => {
    const o = opts(["--verbose"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.unknown_flag");
    expect(stderrOf(o)).toContain("--verbose");
    expect(stderrOf(o)).toContain(USAGE_FRAGMENT);
  });

  it("--format exits 2 with unknown_flag error", () => {
    const o = opts(["--format"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.unknown_flag");
    expect(stderrOf(o)).toContain("--format");
  });

  it("unknown flag before subcommand exits 2", () => {
    const o = opts(["--foo", "validate"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.unknown_flag");
    expect(stderrOf(o)).toContain("--foo");
  });
});

// ---------------------------------------------------------------------------
// "--" end-of-options handling
// ---------------------------------------------------------------------------

describe("dispatch: -- end-of-options", () => {
  it("-- with no subcommand exits 2 with no_subcommand", () => {
    const o = opts(["--"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.no_subcommand");
  });
});

// ---------------------------------------------------------------------------
// Error line format
// ---------------------------------------------------------------------------

describe("dispatch: error line format", () => {
  it("error lines use 'yass:' prefix (no file)", () => {
    const o = opts([]);
    dispatch(o);
    // Error line should start with "yass: [yass.argv.no_subcommand]"
    const errLines = stderrOf(o).split("\n").filter(Boolean);
    const errorLine = errLines[0]!;
    expect(errorLine).toMatch(/^yass: \[yass\.argv\.no_subcommand\]/);
  });

  it("error lines contain the error code in brackets", () => {
    const o = opts(["--verbose"]);
    dispatch(o);
    const errLines = stderrOf(o).split("\n").filter(Boolean);
    const errorLine = errLines[0]!;
    expect(errorLine).toContain("[yass.argv.unknown_flag]");
  });

  it("error lines end with LF", () => {
    const o = opts(["bogus"]);
    dispatch(o);
    const raw = stderrOf(o);
    // The first chunk (before usage) should end with \n
    const firstLine = raw.split("\n")[0];
    expect(firstLine).toBeDefined();
    // The overall output ends with \n (usage text ends with \n)
    expect(raw.endsWith("\n")).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// Valid subcommand dispatching
// ---------------------------------------------------------------------------

describe("dispatch: valid subcommand routing", () => {
  // These tests verify that the dispatch function routes to the correct
  // subcommand. Since the actual subcommand implementations read from the
  // filesystem, we test that dispatch returns the exit code from the
  // subcommand (which will be a non-zero code since there's no project root
  // in a temp directory).

  it("validate subcommand is dispatched (returns subcommand exit code)", () => {
    const o = opts(["validate"], { cwd: "/tmp/nonexistent-yass-test" });
    const code = dispatch(o);
    // validate will fail because no project root marker, exit 2
    expect(code).toBe(2);
    // stderr should contain the validate subcommand's error, not dispatch error
    expect(stderrOf(o)).not.toContain("yass.argv");
  });

  it("list subcommand is dispatched", () => {
    const o = opts(["list"], { cwd: "/tmp/nonexistent-yass-test" });
    const code = dispatch(o);
    // list with no project root returns 0 (silent no-files)
    expect(typeof code).toBe("number");
    // Should NOT contain argv errors
    expect(stderrOf(o)).not.toContain("yass.argv");
  });

  it("query subcommand is dispatched", () => {
    const o = opts(["query", "SomeName"], { cwd: "/tmp/nonexistent-yass-test" });
    const code = dispatch(o);
    // query will fail because no project root marker, exit 2
    expect(code).toBe(2);
    // stderr should contain the query subcommand's error, not dispatch error
    expect(stderrOf(o)).not.toContain("yass.argv");
  });

  it("subcommand args are passed through", () => {
    // query with no name arg should emit query.name_missing
    const o = opts(["query"], { cwd: "/tmp/nonexistent-yass-test" });
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.query.name_missing");
  });

  it("unknown flags after subcommand are rejected per spec", () => {
    // Per spec: "any flag other than --help or --version is given to the
    // top-level command or any subcommand" -> exit 2
    const o = opts(["validate", "--weird-flag"], { cwd: "/tmp/nonexistent-yass-test" });
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.unknown_flag");
  });
});

// ---------------------------------------------------------------------------
// Priority / edge cases
// ---------------------------------------------------------------------------

describe("dispatch: priority and edge cases", () => {
  it("--help with --version: help wins", () => {
    const o = opts(["--help", "--version"]);
    const code = dispatch(o);
    expect(code).toBe(0);
    expect(stdoutOf(o)).toContain(USAGE_FRAGMENT);
    expect(stdoutOf(o)).not.toContain("yass 0.0.3");
  });

  it("--version with unknown flag: version wins", () => {
    const o = opts(["--version", "--bogus"]);
    const code = dispatch(o);
    expect(code).toBe(0);
    expect(stdoutOf(o)).toBe("yass 0.0.3\n");
  });

  it("--help with unknown subcommand: help wins", () => {
    const o = opts(["notreal", "--help"]);
    const code = dispatch(o);
    expect(code).toBe(0);
    expect(stdoutOf(o)).toContain(USAGE_FRAGMENT);
  });

  it("--help with short flag: help wins", () => {
    const o = opts(["-x", "--help"]);
    const code = dispatch(o);
    expect(code).toBe(0);
    expect(stdoutOf(o)).toContain(USAGE_FRAGMENT);
  });

  it("--version with short flag: version wins", () => {
    const o = opts(["-x", "--version"]);
    const code = dispatch(o);
    expect(code).toBe(0);
    expect(stdoutOf(o)).toBe("yass 0.0.3\n");
  });

  it("--help with empty arg: help wins", () => {
    const o = opts(["", "--help"]);
    const code = dispatch(o);
    expect(code).toBe(0);
    expect(stdoutOf(o)).toContain(USAGE_FRAGMENT);
  });

  it("case-mismatched abbreviation is detected as abbreviation", () => {
    // "Val" is not exactly "validate" lowered, but "val" (lowered) is a prefix
    // of "validate". Case mismatch check runs first on lowered == cmd, but
    // "val" != any cmd. Then abbreviation check: "val" is prefix of "validate".
    // Actually "Val" lowered is "val", which is a prefix but not equal to any cmd.
    // So it's an abbreviation.
    const o = opts(["Val"]);
    const code = dispatch(o);
    expect(code).toBe(2);
    expect(stderrOf(o)).toContain("yass.argv.abbreviation");
  });

  it("usage text lists all three commands", () => {
    const o = opts(["--help"]);
    dispatch(o);
    const usage = stdoutOf(o);
    expect(usage).toContain("validate");
    expect(usage).toContain("query");
    expect(usage).toContain("list");
    expect(usage).toContain("--help");
    expect(usage).toContain("--version");
  });

  it("all error exits are code 2", () => {
    const errorCases: string[][] = [
      [],
      ["bogus"],
      ["-h"],
      ["Validate"],
      ["val"],
      [""],
      ["-"],
      ["--bogus"],
    ];
    for (const args of errorCases) {
      const o = opts(args);
      const code = dispatch(o);
      expect(code).toBe(2);
    }
  });

  it("no ANSI escape codes in stdout on --help", () => {
    const o = opts(["--help"]);
    dispatch(o);
    // eslint-disable-next-line no-control-regex
    expect(stdoutOf(o)).not.toMatch(/\x1B\[/);
  });

  it("no ANSI escape codes in stderr on error", () => {
    const o = opts(["bogus"]);
    dispatch(o);
    // eslint-disable-next-line no-control-regex
    expect(stderrOf(o)).not.toMatch(/\x1B\[/);
  });
});

// ---------------------------------------------------------------------------
// Correct error codes used
// ---------------------------------------------------------------------------

describe("dispatch: correct error codes", () => {
  it("no subcommand -> yass.argv.no_subcommand", () => {
    const o = opts([]);
    dispatch(o);
    expect(stderrOf(o)).toContain(ErrorCode.ARGV_NO_SUBCOMMAND);
  });

  it("unknown subcommand -> yass.argv.unknown_subcommand", () => {
    const o = opts(["foo"]);
    dispatch(o);
    expect(stderrOf(o)).toContain(ErrorCode.ARGV_UNKNOWN_SUBCOMMAND);
  });

  it("unknown flag -> yass.argv.unknown_flag", () => {
    const o = opts(["--bar"]);
    dispatch(o);
    expect(stderrOf(o)).toContain(ErrorCode.ARGV_UNKNOWN_FLAG);
  });

  it("short flag -> yass.argv.short_flag", () => {
    const o = opts(["-x"]);
    dispatch(o);
    expect(stderrOf(o)).toContain(ErrorCode.ARGV_SHORT_FLAG);
  });

  it("case mismatch -> yass.argv.case_mismatch", () => {
    const o = opts(["List"]);
    dispatch(o);
    expect(stderrOf(o)).toContain(ErrorCode.ARGV_CASE_MISMATCH);
  });

  it("abbreviation -> yass.argv.abbreviation", () => {
    const o = opts(["que"]);
    dispatch(o);
    expect(stderrOf(o)).toContain(ErrorCode.ARGV_ABBREVIATION);
  });

  it("empty argument -> yass.argv.empty_argument", () => {
    const o = opts([""]);
    dispatch(o);
    expect(stderrOf(o)).toContain(ErrorCode.ARGV_EMPTY_ARGUMENT);
  });

  it("stdin dash -> yass.argv.stdin_dash", () => {
    const o = opts(["-"]);
    dispatch(o);
    expect(stderrOf(o)).toContain(ErrorCode.ARGV_STDIN_DASH);
  });
});
