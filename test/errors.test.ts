import { describe, expect, it } from "bun:test";
import {
  ErrorCode,
  exitCodeFor,
  messageFor,
  SLOT_KEYWORDS,
  NORMATIVITY_KEYWORDS,
  REFERENCE_KEYWORDS,
  GUARD_KEYWORD,
  ALL_RESERVED_KEYWORDS,
} from "../src/errors.ts";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** All symbolic keys of ErrorCode. */
const allKeys = Object.keys(ErrorCode) as (keyof typeof ErrorCode)[];

/** All dotted string values of ErrorCode. */
const allCodes = Object.values(ErrorCode) as string[];

// ---------------------------------------------------------------------------
// Error code naming pattern
// ---------------------------------------------------------------------------

describe("ErrorCode naming pattern", () => {
  it("every code matches [a-z0-9._]+", () => {
    for (const code of allCodes) {
      expect(code).toMatch(/^[a-z0-9._]+$/);
    }
  });

  it("every code starts with yass.", () => {
    for (const code of allCodes) {
      expect(code.startsWith("yass.")).toBe(true);
    }
  });

  it("every code has exactly three dot-separated segments", () => {
    for (const code of allCodes) {
      const segments = code.split(".");
      expect(segments).toHaveLength(3);
    }
  });

  it("no duplicate codes", () => {
    const unique = new Set(allCodes);
    expect(unique.size).toBe(allCodes.length);
  });
});

// ---------------------------------------------------------------------------
// Completeness: every spec error code is defined
// ---------------------------------------------------------------------------

describe("ErrorCode completeness", () => {
  // Exit sentinels
  const exitCodes = [
    "yass.exit.success",
    "yass.exit.processing",
    "yass.exit.usage",
    "yass.exit.sigint",
    "yass.exit.sigterm",
  ];

  // Argv
  const argvCodes = [
    "yass.argv.unknown_subcommand",
    "yass.argv.no_subcommand",
    "yass.argv.unknown_flag",
    "yass.argv.empty_argument",
    "yass.argv.short_flag",
    "yass.argv.case_mismatch",
    "yass.argv.abbreviation",
    "yass.argv.missing_positional",
    "yass.argv.stdin_dash",
  ];

  // Path
  const pathCodes = [
    "yass.path.not_found",
    "yass.path.bad_extension",
    "yass.path.unreadable",
    "yass.path.invalid_type",
    "yass.path.colon_in_path",
  ];

  // Glob
  const globCodes = ["yass.glob.no_match"];

  // Discover
  const discoverCodes = [
    "yass.discover.no_files",
    "yass.discover.dir_unreadable",
  ];

  // Findroot
  const findrootCodes = ["yass.findroot.no_marker"];

  // YAML
  const yamlCodes = [
    "yass.yaml.not_utf8",
    "yass.yaml.has_bom",
    "yass.yaml.malformed",
    "yass.yaml.empty_file",
    "yass.yaml.duplicate_key",
    "yass.yaml.anchor_or_alias",
    "yass.yaml.empty_stream",
  ];

  // Preamble
  const preambleCodes = [
    "yass.preamble.has_spec_key",
    "yass.preamble.missing",
    "yass.preamble.misplaced",
    "yass.preamble.duplicate",
    "yass.preamble.missing_description",
    "yass.preamble.missing_version",
    "yass.preamble.unknown_version",
    "yass.preamble.bad_related",
  ];

  // Spec
  const specCodes = [
    "yass.spec.no_name",
    "yass.spec.name_not_string",
    "yass.spec.name_empty",
    "yass.spec.name_bad_chars",
    "yass.spec.name_bad_form",
    "yass.spec.name_reserved",
    "yass.spec.unknown_key",
    "yass.spec.duplicate_name",
  ];

  // Slot
  const slotCodes = ["yass.slot.value_not_list"];

  // Obligation
  const obligationCodes = [
    "yass.obligation.bad_value_shape",
    "yass.obligation.missing_normativity_or_ref",
    "yass.obligation.guard_without_normativity",
    "yass.obligation.duplicate_reference",
    "yass.obligation.duplicate_normativity",
  ];

  // Normativity
  const normativityCodes = ["yass.normativity.unknown"];

  // Reference
  const referenceCodes = ["yass.reference.unknown_relation"];

  // Ref
  const refCodes = [
    "yass.ref.malformed",
    "yass.ref.unknown_slot",
    "yass.ref.slot_not_declared",
    "yass.ref.spec_not_found_same_file",
    "yass.ref.file_not_found",
    "yass.ref.file_not_parseable",
    "yass.ref.spec_not_found_other_file",
  ];

  // Query
  const queryCodes = [
    "yass.query.name_missing",
    "yass.query.name_blank",
    "yass.query.no_match",
    "yass.query.conforms_unresolved",
    "yass.query.conforms_no_slot",
    "yass.query.scope_not_found",
    "yass.query.scope_empty",
  ];

  // Internal
  const internalCodes = ["yass.internal.uncaught"];

  const ALL_EXPECTED_CODES = [
    ...exitCodes,
    ...argvCodes,
    ...pathCodes,
    ...globCodes,
    ...discoverCodes,
    ...findrootCodes,
    ...yamlCodes,
    ...preambleCodes,
    ...specCodes,
    ...slotCodes,
    ...obligationCodes,
    ...normativityCodes,
    ...referenceCodes,
    ...refCodes,
    ...queryCodes,
    ...internalCodes,
  ];

  it("contains every expected code", () => {
    for (const code of ALL_EXPECTED_CODES) {
      expect(allCodes).toContain(code);
    }
  });

  it("does not contain extra unexpected codes", () => {
    for (const code of allCodes) {
      expect(ALL_EXPECTED_CODES).toContain(code);
    }
  });

  it("has the expected total count", () => {
    expect(allCodes.length).toBe(ALL_EXPECTED_CODES.length);
  });
});

// ---------------------------------------------------------------------------
// exitCodeFor: correct exit code mapping
// ---------------------------------------------------------------------------

describe("exitCodeFor", () => {
  // Exit sentinels
  it.each([
    ["yass.exit.success", 0],
    ["yass.exit.processing", 1],
    ["yass.exit.usage", 2],
    ["yass.exit.sigint", 130],
    ["yass.exit.sigterm", 143],
  ] as const)("%s -> %d", (code, expected) => {
    expect(exitCodeFor(code)).toBe(expected);
  });

  // Argv errors -> exit 2
  it("all argv errors exit 2", () => {
    const argvCodes = allCodes.filter((c) => c.startsWith("yass.argv."));
    expect(argvCodes.length).toBeGreaterThan(0);
    for (const code of argvCodes) {
      expect(exitCodeFor(code)).toBe(2);
    }
  });

  // Path errors -> exit 2
  it("all path errors exit 2", () => {
    const pathCodes = allCodes.filter((c) => c.startsWith("yass.path."));
    expect(pathCodes.length).toBeGreaterThan(0);
    for (const code of pathCodes) {
      expect(exitCodeFor(code)).toBe(2);
    }
  });

  // Glob -> exit 2
  it("glob.no_match exits 2", () => {
    expect(exitCodeFor("yass.glob.no_match")).toBe(2);
  });

  // Discover
  it("discover.no_files exits 2", () => {
    expect(exitCodeFor("yass.discover.no_files")).toBe(2);
  });

  it("discover.dir_unreadable exits 1 (non-fatal during recursion)", () => {
    expect(exitCodeFor("yass.discover.dir_unreadable")).toBe(1);
  });

  // Findroot -> exit 2
  it("findroot.no_marker exits 2", () => {
    expect(exitCodeFor("yass.findroot.no_marker")).toBe(2);
  });

  // YAML errors -> exit 1
  it("all yaml errors exit 1", () => {
    const yamlCodes = allCodes.filter((c) => c.startsWith("yass.yaml."));
    expect(yamlCodes.length).toBeGreaterThan(0);
    for (const code of yamlCodes) {
      expect(exitCodeFor(code)).toBe(1);
    }
  });

  // Preamble errors -> exit 1
  it("all preamble errors exit 1", () => {
    const preambleCodes = allCodes.filter((c) =>
      c.startsWith("yass.preamble."),
    );
    expect(preambleCodes.length).toBeGreaterThan(0);
    for (const code of preambleCodes) {
      expect(exitCodeFor(code)).toBe(1);
    }
  });

  // Spec errors -> exit 1
  it("all spec errors exit 1", () => {
    const specCodes = allCodes.filter((c) => c.startsWith("yass.spec."));
    expect(specCodes.length).toBeGreaterThan(0);
    for (const code of specCodes) {
      expect(exitCodeFor(code)).toBe(1);
    }
  });

  // Slot -> exit 1
  it("slot.value_not_list exits 1", () => {
    expect(exitCodeFor("yass.slot.value_not_list")).toBe(1);
  });

  // Obligation errors -> exit 1
  it("all obligation errors exit 1", () => {
    const obligationCodes = allCodes.filter((c) =>
      c.startsWith("yass.obligation."),
    );
    expect(obligationCodes.length).toBeGreaterThan(0);
    for (const code of obligationCodes) {
      expect(exitCodeFor(code)).toBe(1);
    }
  });

  // Normativity -> exit 1
  it("normativity.unknown exits 1", () => {
    expect(exitCodeFor("yass.normativity.unknown")).toBe(1);
  });

  // Reference -> exit 1
  it("reference.unknown_relation exits 1", () => {
    expect(exitCodeFor("yass.reference.unknown_relation")).toBe(1);
  });

  // Ref errors -> exit 1
  it("all ref errors exit 1", () => {
    const refCodes = allCodes.filter((c) => c.startsWith("yass.ref."));
    expect(refCodes.length).toBeGreaterThan(0);
    for (const code of refCodes) {
      expect(exitCodeFor(code)).toBe(1);
    }
  });

  // Query errors (mixed exit codes)
  it.each([
    ["yass.query.name_missing", 2],
    ["yass.query.name_blank", 2],
    ["yass.query.no_match", 1],
    ["yass.query.conforms_unresolved", 1],
    ["yass.query.conforms_no_slot", 1],
    ["yass.query.scope_not_found", 2],
    ["yass.query.scope_empty", 2],
  ] as const)("query: %s -> %d", (code, expected) => {
    expect(exitCodeFor(code)).toBe(expected);
  });

  // Internal -> exit 1
  it("internal.uncaught exits 1", () => {
    expect(exitCodeFor("yass.internal.uncaught")).toBe(1);
  });

  // Unknown code
  it("throws for unknown code", () => {
    expect(() => exitCodeFor("yass.nope.not_real")).toThrow(
      "unknown error code",
    );
  });
});

// ---------------------------------------------------------------------------
// messageFor: template formatting
// ---------------------------------------------------------------------------

describe("messageFor", () => {
  // --- Templates with no placeholders ---
  it.each([
    ["yass.argv.no_subcommand", "no subcommand given"],
    ["yass.argv.empty_argument", "empty argument"],
    [
      "yass.argv.stdin_dash",
      "stdin marker `-` is not supported; pass a file path",
    ],
    ["yass.discover.no_files", "no .yass.yaml files found"],
    ["yass.findroot.no_marker", "no project root marker found"],
    ["yass.yaml.not_utf8", "file is not valid UTF-8"],
    ["yass.yaml.has_bom", "file begins with a UTF-8 BOM"],
    ["yass.yaml.malformed", "YAML well-formedness error"],
    ["yass.yaml.empty_file", "empty file"],
    [
      "yass.yaml.anchor_or_alias",
      "YAML anchors, aliases, and explicit tags are not allowed",
    ],
    ["yass.yaml.empty_stream", "YAML stream contains no documents"],
    [
      "yass.preamble.has_spec_key",
      "first document must be a Preamble, not a Spec",
    ],
    ["yass.preamble.missing", "missing Preamble"],
    ["yass.preamble.misplaced", "Preamble must be the first document"],
    ["yass.preamble.duplicate", "more than one Preamble in file"],
    ["yass.preamble.missing_description", "Preamble missing description"],
    ["yass.preamble.missing_version", "Preamble missing version"],
    [
      "yass.preamble.bad_related",
      "Preamble related must be a sequence of strings",
    ],
    ["yass.spec.no_name", "spec document missing spec key"],
    ["yass.spec.name_not_string", "spec name must be a string"],
    ["yass.spec.name_empty", "spec name is empty"],
    [
      "yass.obligation.bad_value_shape",
      "obligation value must be a quoted scalar",
    ],
    [
      "yass.obligation.missing_normativity_or_ref",
      "obligation must carry a Normativity keyword or a Reference",
    ],
    [
      "yass.obligation.guard_without_normativity",
      "WHEN guard requires a Normativity keyword",
    ],
    [
      "yass.obligation.duplicate_normativity",
      "duplicate Normativity keyword in obligation",
    ],
    ["yass.query.name_missing", "missing spec name"],
    [
      "yass.query.name_blank",
      "spec name is blank or contains whitespace",
    ],
  ] as const)("no-arg template: %s", (code, expected) => {
    expect(messageFor(code)).toBe(expected);
  });

  // --- Templates with placeholders ---
  it("argv.unknown_subcommand substitutes <arg>", () => {
    expect(messageFor("yass.argv.unknown_subcommand", { arg: "frobnicate" }))
      .toBe("unknown subcommand: frobnicate");
  });

  it("argv.unknown_flag substitutes <flag>", () => {
    expect(messageFor("yass.argv.unknown_flag", { flag: "--nope" }))
      .toBe("unknown flag: --nope");
  });

  it("argv.short_flag substitutes <flag>", () => {
    expect(messageFor("yass.argv.short_flag", { flag: "-v" }))
      .toBe("short-form flags are not supported in v1: -v");
  });

  it("argv.case_mismatch substitutes <token>", () => {
    expect(messageFor("yass.argv.case_mismatch", { token: "Validate" }))
      .toBe("subcommand or flag case mismatch: Validate");
  });

  it("argv.abbreviation substitutes <token>", () => {
    expect(messageFor("yass.argv.abbreviation", { token: "val" }))
      .toBe("abbreviations are not supported: val");
  });

  it("argv.missing_positional substitutes <name>", () => {
    expect(messageFor("yass.argv.missing_positional", { name: "PATH" }))
      .toBe("missing required argument: PATH");
  });

  it("path.not_found substitutes <path>", () => {
    expect(messageFor("yass.path.not_found", { path: "/tmp/missing" }))
      .toBe("path does not exist: /tmp/missing");
  });

  it("path.bad_extension substitutes <path>", () => {
    expect(messageFor("yass.path.bad_extension", { path: "foo.yaml" }))
      .toBe("expected a .yass.yaml file: foo.yaml");
  });

  it("path.unreadable substitutes <path>", () => {
    expect(messageFor("yass.path.unreadable", { path: "/secret" }))
      .toBe("cannot read path: /secret");
  });

  it("path.invalid_type substitutes <path>", () => {
    expect(messageFor("yass.path.invalid_type", { path: "/dev/null" }))
      .toBe("path is neither a file nor a directory: /dev/null");
  });

  it("path.colon_in_path substitutes <path>", () => {
    expect(messageFor("yass.path.colon_in_path", { path: "C:\\foo" }))
      .toBe("path contains an unsupported colon character: C:\\foo");
  });

  it("glob.no_match substitutes <pattern>", () => {
    expect(messageFor("yass.glob.no_match", { pattern: "*.txt" }))
      .toBe("no files matched pattern: *.txt");
  });

  it("discover.dir_unreadable substitutes <path>", () => {
    expect(messageFor("yass.discover.dir_unreadable", { path: "/nope" }))
      .toBe("cannot read directory: /nope");
  });

  it("yaml.duplicate_key substitutes <key>", () => {
    expect(messageFor("yass.yaml.duplicate_key", { key: "spec" }))
      .toBe("duplicate mapping key: spec");
  });

  it("preamble.unknown_version substitutes <version>", () => {
    expect(
      messageFor("yass.preamble.unknown_version", { version: "99.0" }),
    ).toBe("unsupported Preamble version: 99.0");
  });

  it("spec.name_bad_chars substitutes <name>", () => {
    expect(messageFor("yass.spec.name_bad_chars", { name: "foo bar" }))
      .toBe("spec name contains disallowed characters: foo bar");
  });

  it("spec.name_bad_form substitutes <name>", () => {
    expect(messageFor("yass.spec.name_bad_form", { name: "..foo" }))
      .toBe("spec name is malformed: ..foo");
  });

  it("spec.name_reserved substitutes <name>", () => {
    expect(messageFor("yass.spec.name_reserved", { name: "MUST" }))
      .toBe("spec name collides with a reserved keyword: MUST");
  });

  it("spec.unknown_key substitutes <key>", () => {
    expect(messageFor("yass.spec.unknown_key", { key: "bogus" }))
      .toBe("unknown spec key: bogus");
  });

  it("spec.duplicate_name substitutes <name>", () => {
    expect(messageFor("yass.spec.duplicate_name", { name: "Foo" }))
      .toBe("duplicate spec name in file: Foo");
  });

  it("slot.value_not_list substitutes <slot>", () => {
    expect(messageFor("yass.slot.value_not_list", { slot: "INPUT" }))
      .toBe("slot value must be a list: INPUT");
  });

  it("obligation.duplicate_reference substitutes <relation>", () => {
    expect(
      messageFor("yass.obligation.duplicate_reference", {
        relation: "CONFORMS",
      }),
    ).toBe("duplicate Reference relation in obligation: CONFORMS");
  });

  it("normativity.unknown substitutes <keyword>", () => {
    expect(messageFor("yass.normativity.unknown", { keyword: "MIGHT" }))
      .toBe("unknown Normativity keyword: MIGHT");
  });

  it("reference.unknown_relation substitutes <relation>", () => {
    expect(
      messageFor("yass.reference.unknown_relation", { relation: "LINKS" }),
    ).toBe("unknown Reference relation: LINKS");
  });

  it("ref.malformed substitutes <target>", () => {
    expect(messageFor("yass.ref.malformed", { target: ":::bad" }))
      .toBe("malformed ref target: :::bad");
  });

  it("ref.unknown_slot substitutes <slot>", () => {
    expect(messageFor("yass.ref.unknown_slot", { slot: "BOGUS" }))
      .toBe("unknown slot in ref target: BOGUS");
  });

  it("ref.slot_not_declared substitutes <target>", () => {
    expect(
      messageFor("yass.ref.slot_not_declared", { target: "Foo#INPUT" }),
    ).toBe("referenced spec does not declare slot: Foo#INPUT");
  });

  it("ref.spec_not_found_same_file substitutes <target>", () => {
    expect(
      messageFor("yass.ref.spec_not_found_same_file", { target: "Bar" }),
    ).toBe("spec not found in file: Bar");
  });

  it("ref.file_not_found substitutes <target>", () => {
    expect(
      messageFor("yass.ref.file_not_found", { target: "missing.yass.yaml" }),
    ).toBe("referenced file not found: missing.yass.yaml");
  });

  it("ref.file_not_parseable substitutes <target>", () => {
    expect(
      messageFor("yass.ref.file_not_parseable", { target: "bad.yass.yaml" }),
    ).toBe("referenced file not parseable: bad.yass.yaml");
  });

  it("ref.spec_not_found_other_file substitutes <target>", () => {
    expect(
      messageFor("yass.ref.spec_not_found_other_file", {
        target: "other.yass.yaml:Baz",
      }),
    ).toBe("spec not found in referenced file: other.yass.yaml:Baz");
  });

  it("query.no_match substitutes <name>", () => {
    expect(messageFor("yass.query.no_match", { name: "DoesNotExist" }))
      .toBe("no spec matches: DoesNotExist");
  });

  it("query.conforms_unresolved substitutes <target>", () => {
    expect(
      messageFor("yass.query.conforms_unresolved", { target: "Foo#INPUT" }),
    ).toBe("unresolvable CONFORMS ref: Foo#INPUT");
  });

  it("query.conforms_no_slot substitutes <target>", () => {
    expect(
      messageFor("yass.query.conforms_no_slot", { target: "Foo" }),
    ).toBe("CONFORMS ref must address a slot in v1: Foo");
  });

  it("query.scope_not_found substitutes <path>", () => {
    expect(messageFor("yass.query.scope_not_found", { path: "/gone" }))
      .toBe("scope path does not exist: /gone");
  });

  it("query.scope_empty substitutes <path>", () => {
    expect(messageFor("yass.query.scope_empty", { path: "/empty" }))
      .toBe("no .yass.yaml files found in scope: /empty");
  });

  it("internal.uncaught substitutes <message>", () => {
    expect(
      messageFor("yass.internal.uncaught", { message: "something broke" }),
    ).toBe("internal error: something broke");
  });

  // --- Edge cases ---
  it("leaves unfilled placeholders intact", () => {
    expect(messageFor("yass.argv.unknown_subcommand", {})).toBe(
      "unknown subcommand: <arg>",
    );
  });

  it("ignores extra args not in the template", () => {
    expect(
      messageFor("yass.argv.no_subcommand", { extra: "ignored" }),
    ).toBe("no subcommand given");
  });

  it("throws for unknown code", () => {
    expect(() => messageFor("yass.nope.bogus")).toThrow("unknown error code");
  });
});

// ---------------------------------------------------------------------------
// Keyword sets
// ---------------------------------------------------------------------------

describe("SLOT_KEYWORDS", () => {
  it("contains exactly the 5 slot keywords", () => {
    expect(SLOT_KEYWORDS).toEqual([
      "INPUT",
      "RETURN",
      "ERROR",
      "SIDE-EFFECT",
      "INVARIANT",
    ]);
  });

  it("is an array", () => {
    expect(Array.isArray(SLOT_KEYWORDS)).toBe(true);
  });
});

describe("NORMATIVITY_KEYWORDS", () => {
  it("contains exactly the 5 normativity keywords", () => {
    expect(NORMATIVITY_KEYWORDS).toEqual([
      "MUST",
      "MUST-NOT",
      "SHOULD",
      "SHOULD-NOT",
      "MAY",
    ]);
  });

  it("is an array", () => {
    expect(Array.isArray(NORMATIVITY_KEYWORDS)).toBe(true);
  });
});

describe("REFERENCE_KEYWORDS", () => {
  it("contains exactly the 3 reference keywords", () => {
    expect(REFERENCE_KEYWORDS).toEqual(["CONFORMS", "USES", "SEE"]);
  });

  it("is an array", () => {
    expect(Array.isArray(REFERENCE_KEYWORDS)).toBe(true);
  });
});

describe("GUARD_KEYWORD", () => {
  it("is WHEN", () => {
    expect(GUARD_KEYWORD).toBe("WHEN");
  });
});

describe("ALL_RESERVED_KEYWORDS", () => {
  it("is the union of SLOT_KEYWORDS and NORMATIVITY_KEYWORDS", () => {
    const expected = [...SLOT_KEYWORDS, ...NORMATIVITY_KEYWORDS];
    expect([...ALL_RESERVED_KEYWORDS]).toEqual(expected);
  });

  it("contains 10 entries", () => {
    expect(ALL_RESERVED_KEYWORDS).toHaveLength(10);
  });

  it("does not include REFERENCE_KEYWORDS", () => {
    for (const kw of REFERENCE_KEYWORDS) {
      expect(ALL_RESERVED_KEYWORDS).not.toContain(kw);
    }
  });

  it("does not include GUARD_KEYWORD", () => {
    expect(ALL_RESERVED_KEYWORDS).not.toContain(GUARD_KEYWORD);
  });
});

// ---------------------------------------------------------------------------
// Every error code has both an exit code and a message template
// ---------------------------------------------------------------------------

describe("every ErrorCode has an exit code and message template", () => {
  for (const key of allKeys) {
    const code = ErrorCode[key];

    it(`${code} has a numeric exit code`, () => {
      const exit = exitCodeFor(code);
      expect(typeof exit).toBe("number");
      expect(Number.isInteger(exit)).toBe(true);
    });

    it(`${code} has a message template`, () => {
      const msg = messageFor(code);
      expect(typeof msg).toBe("string");
      expect(msg.length).toBeGreaterThan(0);
    });
  }
});
