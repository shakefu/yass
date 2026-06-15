const std = @import("std");
const Allocator = std.mem.Allocator;
const yaml = @import("yaml_parse.zig");
const err = @import("errors.zig");

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

pub const ListRow = struct {
    file: []const u8,
    spec_name: []const u8,
    description: []const u8,
};

pub const ListError = struct {
    file: []const u8,
    code: err.ErrorCode,
    message: []const u8,
};

pub const ListResult = struct {
    rows: []ListRow,
    errors: []ListError,
    has_parse_errors: bool,
};

// ---------------------------------------------------------------------------
// Core implementation
// ---------------------------------------------------------------------------

/// List specs discovered from the given files.
///
/// For each file, parse the YAML and extract spec documents. The first
/// document in a file is the preamble (a mapping without a "spec" key);
/// subsequent documents with a "spec" key produce rows. Document order
/// within each file is preserved; files are expected to arrive pre-sorted
/// (by Unicode code-point order).
///
/// `terminal_width`: if non-null, descriptions are truncated to fit the
/// terminal width using a "..." marker. If null, no truncation occurs.
pub fn listSpecs(
    allocator: Allocator,
    io: std.Io,
    files: []const []const u8,
    cwd: []const u8,
    terminal_width: ?u16,
) ListResult {
    _ = cwd;

    var rows: std.ArrayList(ListRow) = .empty;
    var errs: std.ArrayList(ListError) = .empty;
    var has_parse_errors = false;

    for (files) |file| {
        processFile(allocator, io, file, &rows, &errs, &has_parse_errors) catch {
            // OOM - record as internal error and continue
            errs.append(allocator, .{
                .file = allocator.dupe(u8, file) catch file,
                .code = .internal_uncaught,
                .message = allocator.dupe(u8, "out of memory") catch "out of memory",
            }) catch {};
            has_parse_errors = true;
            continue;
        };
    }

    // Apply TTY truncation if needed
    if (terminal_width) |width| {
        for (rows.items) |*row| {
            row.description = truncateDescription(
                allocator,
                row.file,
                row.spec_name,
                row.description,
                width,
            ) catch row.description;
        }
    }

    return .{
        .rows = rows.toOwnedSlice(allocator) catch &.{},
        .errors = errs.toOwnedSlice(allocator) catch &.{},
        .has_parse_errors = has_parse_errors,
    };
}

/// Process a single file: parse YAML, extract preamble description, and
/// emit rows for each spec document.
fn processFile(
    allocator: Allocator,
    io: std.Io,
    file: []const u8,
    rows: *std.ArrayList(ListRow),
    errs: *std.ArrayList(ListError),
    has_parse_errors: *bool,
) !void {
    // Read file content
    const content = std.Io.Dir.cwd().readFileAlloc(io, file, allocator, @enumFromInt(10 * 1024 * 1024)) catch |e| {
        const msg = switch (e) {
            error.FileNotFound => "file not found",
            else => "unable to read file",
        };
        try errs.append(allocator, .{
            .file = try allocator.dupe(u8, file),
            .code = .path_unreadable,
            .message = try allocator.dupe(u8, msg),
        });
        has_parse_errors.* = true;
        return;
    };
    defer allocator.free(content);

    // Parse YAML
    const parse_result = yaml.parseYaml(allocator, content) catch |e| {
        const code: err.ErrorCode = switch (e) {
            error.not_utf8 => .yaml_not_utf8,
            error.has_bom => .yaml_has_bom,
            error.empty_file => .yaml_empty_file,
            error.malformed => .yaml_malformed,
            error.duplicate_key => .yaml_duplicate_key,
            error.anchor_or_alias => .yaml_anchor_or_alias,
        };
        const msg = switch (e) {
            error.not_utf8 => "file is not valid UTF-8",
            error.has_bom => "file contains a BOM",
            error.empty_file => "file is empty",
            error.malformed => "YAML is malformed",
            error.duplicate_key => "duplicate key",
            error.anchor_or_alias => "anchors and aliases are not allowed",
        };
        try errs.append(allocator, .{
            .file = try allocator.dupe(u8, file),
            .code = code,
            .message = try allocator.dupe(u8, msg),
        });
        has_parse_errors.* = true;
        return;
    };

    defer yaml.freeParseResult(allocator, parse_result);

    if (parse_result.documents.len == 0) return;

    // Extract preamble description from first document
    var description: []const u8 = "";
    var first_doc_is_preamble = false;

    if (parse_result.documents[0].root) |root| {
        switch (root) {
            .mapping => |m| {
                if (!mappingHasKey(m, "spec")) {
                    first_doc_is_preamble = true;
                    description = extractDescription(allocator, m) catch "";
                }
            },
            else => {},
        }
    }

    // Process spec documents
    const start_idx: usize = if (first_doc_is_preamble) 1 else 0;
    for (parse_result.documents[start_idx..]) |doc| {
        const root = doc.root orelse continue;
        switch (root) {
            .mapping => |m| {
                const spec_name = extractSpecName(m) orelse continue;
                try rows.append(allocator, .{
                    .file = try allocator.dupe(u8, file),
                    .spec_name = try allocator.dupe(u8, spec_name),
                    .description = try allocator.dupe(u8, description),
                });
            },
            else => {},
        }
    }
}

// ---------------------------------------------------------------------------
// YAML extraction helpers
// ---------------------------------------------------------------------------

/// Check whether a mapping contains a key with the given name.
fn mappingHasKey(m: yaml.YamlMapping, key: []const u8) bool {
    for (m.entries) |entry| {
        if (std.mem.eql(u8, entry.key.value, key)) return true;
    }
    return false;
}

/// Extract the "description" value from a preamble mapping.
/// Returns the normalized description string, or empty string if absent
/// or non-string.
fn extractDescription(allocator: Allocator, m: yaml.YamlMapping) ![]const u8 {
    for (m.entries) |entry| {
        if (std.mem.eql(u8, entry.key.value, "description")) {
            switch (entry.value) {
                .scalar => |s| {
                    return try normalizeDescription(allocator, s.value);
                },
                else => return "",
            }
        }
    }
    return "";
}

/// Extract the "spec" value (name) from a spec document mapping.
fn extractSpecName(m: yaml.YamlMapping) ?[]const u8 {
    for (m.entries) |entry| {
        if (std.mem.eql(u8, entry.key.value, "spec")) {
            switch (entry.value) {
                .scalar => |s| return s.value,
                else => return null,
            }
        }
    }
    return null;
}

// ---------------------------------------------------------------------------
// Description normalization
// ---------------------------------------------------------------------------

/// Normalize a description string:
/// - Collapse runs of whitespace (spaces, tabs, newlines, CR) to a single space
/// - Trim leading and trailing whitespace
fn normalizeDescription(allocator: Allocator, raw: []const u8) ![]const u8 {
    if (raw.len == 0) return "";

    var result = try allocator.alloc(u8, raw.len);
    var out_len: usize = 0;
    var in_whitespace = false;

    for (raw) |ch| {
        if (ch == ' ' or ch == '\t' or ch == '\n' or ch == '\r') {
            if (!in_whitespace and out_len > 0) {
                // Only emit space if we've already emitted non-whitespace
                result[out_len] = ' ';
                out_len += 1;
            }
            in_whitespace = true;
        } else {
            in_whitespace = false;
            result[out_len] = ch;
            out_len += 1;
        }
    }

    // Trim trailing space
    if (out_len > 0 and result[out_len - 1] == ' ') {
        out_len -= 1;
    }

    if (out_len == 0) {
        allocator.free(result);
        return "";
    }

    // Shrink to fit
    if (out_len < result.len) {
        const shrunk = allocator.realloc(result, out_len) catch result;
        return shrunk[0..out_len];
    }
    return result[0..out_len];
}

// ---------------------------------------------------------------------------
// TTY truncation
// ---------------------------------------------------------------------------

/// Count the number of grapheme clusters in a string.
/// For ASCII text this equals byte length. For multi-byte UTF-8 codepoints,
/// we count codepoints as a simple approximation (true grapheme cluster
/// segmentation requires UAX #29, but for this CLI the codepoint count is
/// sufficient and correct for all ASCII content).
fn countGraphemes(s: []const u8) usize {
    var count: usize = 0;
    var i: usize = 0;
    while (i < s.len) {
        const byte = s[i];
        if (byte < 0x80) {
            i += 1;
        } else if (byte < 0xC0) {
            // Continuation byte (shouldn't appear at start, skip)
            i += 1;
            continue;
        } else if (byte < 0xE0) {
            i += 2;
        } else if (byte < 0xF0) {
            i += 3;
        } else {
            i += 4;
        }
        count += 1;
    }
    return count;
}

/// Find the byte offset at which the first `max_graphemes` grapheme clusters end.
fn graphemeByteBoundary(s: []const u8, max_graphemes: usize) usize {
    var count: usize = 0;
    var i: usize = 0;
    while (i < s.len and count < max_graphemes) {
        const byte = s[i];
        if (byte < 0x80) {
            i += 1;
        } else if (byte < 0xC0) {
            i += 1;
            continue;
        } else if (byte < 0xE0) {
            i += 2;
        } else if (byte < 0xF0) {
            i += 3;
        } else {
            i += 4;
        }
        count += 1;
    }
    return i;
}

/// Truncate description to fit within terminal width.
///
/// Line layout: <file>\t<spec_name>\t<description>
/// If the description must be truncated, append "..." marker.
fn truncateDescription(
    allocator: Allocator,
    file: []const u8,
    spec_name: []const u8,
    description: []const u8,
    width: u16,
) ![]const u8 {
    if (description.len == 0) return description;

    const file_graphemes = countGraphemes(file);
    const name_graphemes = countGraphemes(spec_name);
    const desc_graphemes = countGraphemes(description);

    // Overhead: file + tab + name + tab
    const overhead = file_graphemes + 1 + name_graphemes + 1;
    const marker_len: usize = 3; // "..."

    // If path + name + tabs + marker already >= width: emit empty description
    if (overhead + marker_len >= @as(usize, width)) {
        return "";
    }

    const available = @as(usize, width) - overhead;

    // If description fits, no truncation needed
    if (desc_graphemes <= available) {
        return description;
    }

    // Truncate: available - marker_len graphemes, then "..."
    const max_desc_graphemes = available - marker_len;
    const boundary = graphemeByteBoundary(description, max_desc_graphemes);
    const truncated = try std.fmt.allocPrint(allocator, "{s}...", .{description[0..boundary]});
    return truncated;
}

// ---------------------------------------------------------------------------
// Output formatting
// ---------------------------------------------------------------------------

/// Format a ListRow as a tab-separated line terminated by LF.
pub fn formatRow(allocator: Allocator, row: ListRow) ![]const u8 {
    return std.fmt.allocPrint(allocator, "{s}\t{s}\t{s}\n", .{
        row.file,
        row.spec_name,
        row.description,
    });
}

// ===========================================================================
// Tests
// ===========================================================================

const testing = std.testing;

// ---------------------------------------------------------------------------
// Helper: parse YAML from string and call listSpecs-like logic
// ---------------------------------------------------------------------------

fn freeYamlNode(allocator: Allocator, node: yaml.YamlNode) void {
    switch (node) {
        .scalar => |s| allocator.free(s.value),
        .mapping => |m| {
            for (m.entries) |entry| {
                allocator.free(entry.key.value);
                freeYamlNode(allocator, entry.value);
            }
            allocator.free(m.entries);
        },
        .sequence => |seq| {
            for (seq.items) |item| {
                freeYamlNode(allocator, item);
            }
            allocator.free(seq.items);
        },
    }
}

fn freeParseResult(allocator: Allocator, result: yaml.ParseResult) void {
    for (result.documents) |doc| {
        if (doc.root) |root| {
            freeYamlNode(allocator, root);
        }
    }
    allocator.free(result.documents);
}

fn testProcessContent(allocator: Allocator, file_name: []const u8, content: []const u8) !struct {
    rows: []ListRow,
    parse_failed: bool,
} {
    const parse_result = yaml.parseYaml(allocator, content) catch {
        return .{ .rows = &.{}, .parse_failed = true };
    };
    defer freeParseResult(allocator, parse_result);

    if (parse_result.documents.len == 0) {
        return .{ .rows = &.{}, .parse_failed = false };
    }

    var description: []const u8 = "";
    var desc_allocated = false;
    var first_doc_is_preamble = false;

    if (parse_result.documents[0].root) |root| {
        switch (root) {
            .mapping => |m| {
                if (!mappingHasKey(m, "spec")) {
                    first_doc_is_preamble = true;
                    description = extractDescription(allocator, m) catch "";
                    desc_allocated = description.len > 0;
                }
            },
            else => {},
        }
    }
    defer if (desc_allocated) allocator.free(description);

    var rows: std.ArrayList(ListRow) = .empty;
    const start_idx: usize = if (first_doc_is_preamble) 1 else 0;
    for (parse_result.documents[start_idx..]) |doc| {
        const root = doc.root orelse continue;
        switch (root) {
            .mapping => |m| {
                const spec_name = extractSpecName(m) orelse continue;
                try rows.append(allocator, .{
                    .file = try allocator.dupe(u8, file_name),
                    .spec_name = try allocator.dupe(u8, spec_name),
                    .description = try allocator.dupe(u8, description),
                });
            },
            else => {},
        }
    }

    return .{
        .rows = try rows.toOwnedSlice(allocator),
        .parse_failed = false,
    };
}

fn freeTestRows(allocator: Allocator, rows: []ListRow) void {
    for (rows) |row| {
        allocator.free(row.file);
        allocator.free(row.spec_name);
        if (row.description.len > 0) allocator.free(row.description);
    }
    allocator.free(rows);
}

// ---------------------------------------------------------------------------
// Test: parse a simple spec file and extract rows
// ---------------------------------------------------------------------------

test "parse a simple spec file and extract rows" {
    const allocator = testing.allocator;
    const content =
        \\---
        \\description: A test spec
        \\version: 1
        \\---
        \\spec: my-service
        \\obligations:
        \\  - MUST do stuff
        \\
    ;
    const result = try testProcessContent(allocator, "test.yass.yaml", content);
    defer freeTestRows(allocator, result.rows);

    try testing.expect(!result.parse_failed);
    try testing.expectEqual(@as(usize, 1), result.rows.len);
    try testing.expectEqualStrings("test.yass.yaml", result.rows[0].file);
    try testing.expectEqualStrings("my-service", result.rows[0].spec_name);
    try testing.expectEqualStrings("A test spec", result.rows[0].description);
}

// ---------------------------------------------------------------------------
// Test: multiple specs in one file
// ---------------------------------------------------------------------------

test "multiple specs in one file" {
    const allocator = testing.allocator;
    const content =
        \\---
        \\description: Multi spec file
        \\version: 1
        \\---
        \\spec: service-a
        \\obligations:
        \\  - MUST exist
        \\---
        \\spec: service-b
        \\obligations:
        \\  - MUST also exist
        \\
    ;
    const result = try testProcessContent(allocator, "multi.yass.yaml", content);
    defer freeTestRows(allocator, result.rows);

    try testing.expect(!result.parse_failed);
    try testing.expectEqual(@as(usize, 2), result.rows.len);
    try testing.expectEqualStrings("service-a", result.rows[0].spec_name);
    try testing.expectEqualStrings("service-b", result.rows[1].spec_name);
    // Both should have the same description from the preamble
    try testing.expectEqualStrings("Multi spec file", result.rows[0].description);
    try testing.expectEqualStrings("Multi spec file", result.rows[1].description);
}

// ---------------------------------------------------------------------------
// Test: empty description handling
// ---------------------------------------------------------------------------

test "empty description handling" {
    const allocator = testing.allocator;

    // Preamble without description key
    const content =
        \\---
        \\version: 1
        \\---
        \\spec: no-desc
        \\
    ;
    const result = try testProcessContent(allocator, "nodesc.yass.yaml", content);
    defer freeTestRows(allocator, result.rows);

    try testing.expect(!result.parse_failed);
    try testing.expectEqual(@as(usize, 1), result.rows.len);
    try testing.expectEqualStrings("", result.rows[0].description);
}

// ---------------------------------------------------------------------------
// Test: description normalization (whitespace collapsing)
// ---------------------------------------------------------------------------

test "description normalization whitespace collapsing" {
    const allocator = testing.allocator;

    // Test normalizeDescription directly
    const result1 = try normalizeDescription(allocator, "hello   world");
    defer if (result1.len > 0) allocator.free(result1);
    try testing.expectEqualStrings("hello world", result1);

    const result2 = try normalizeDescription(allocator, "  leading and trailing  ");
    defer if (result2.len > 0) allocator.free(result2);
    try testing.expectEqualStrings("leading and trailing", result2);

    const result3 = try normalizeDescription(allocator, "tabs\there\tand\tthere");
    defer if (result3.len > 0) allocator.free(result3);
    try testing.expectEqualStrings("tabs here and there", result3);

    const result4 = try normalizeDescription(allocator, "line\none\nline\ntwo");
    defer if (result4.len > 0) allocator.free(result4);
    try testing.expectEqualStrings("line one line two", result4);

    const result5 = try normalizeDescription(allocator, "mixed \t \n whitespace");
    defer if (result5.len > 0) allocator.free(result5);
    try testing.expectEqualStrings("mixed whitespace", result5);
}

// ---------------------------------------------------------------------------
// Test: TTY truncation with marker
// ---------------------------------------------------------------------------

test "TTY truncation with marker" {
    const allocator = testing.allocator;

    // file=10 chars, tab=1, name=5 chars, tab=1 => overhead=17
    // width=30, available=13, desc has 20 graphemes => truncate
    // max_desc = 13 - 3 = 10 graphemes
    const result = try truncateDescription(
        allocator,
        "myfile.yss", // 10 graphemes
        "svcab", // 5 graphemes
        "this is a long description!!", // 28 graphemes
        30,
    );
    defer if (result.len > 0) allocator.free(result);

    // Should be 10 graphemes + "..."
    try testing.expectEqualStrings("this is a ...", result);
}

// ---------------------------------------------------------------------------
// Test: no truncation when not TTY
// ---------------------------------------------------------------------------

test "no truncation when not TTY" {
    const allocator = testing.allocator;

    // When terminal_width is null, truncateDescription should not be called,
    // but we test the logic: if we pass a large width, no truncation occurs
    const result = try truncateDescription(
        allocator,
        "file.yaml",
        "svc",
        "a short description",
        200,
    );
    // Result should be the original string (no allocation)
    try testing.expectEqualStrings("a short description", result);
}

// ---------------------------------------------------------------------------
// Test: file with no specs emits no rows
// ---------------------------------------------------------------------------

test "file with no specs emits no rows" {
    const allocator = testing.allocator;

    // Only preamble, no spec documents
    const content =
        \\---
        \\description: Just a preamble
        \\version: 1
        \\
    ;
    const result = try testProcessContent(allocator, "preamble-only.yass.yaml", content);
    defer freeTestRows(allocator, result.rows);

    try testing.expect(!result.parse_failed);
    try testing.expectEqual(@as(usize, 0), result.rows.len);
}

// ---------------------------------------------------------------------------
// Test: parse error handling
// ---------------------------------------------------------------------------

test "parse error handling" {
    const allocator = testing.allocator;

    // Invalid YAML - BOM
    const bom_content = "\xEF\xBB\xBFspec: test\n";
    const result1 = try testProcessContent(allocator, "bom.yass.yaml", bom_content);
    defer freeTestRows(allocator, result1.rows);
    try testing.expect(result1.parse_failed);
    try testing.expectEqual(@as(usize, 0), result1.rows.len);

    // Empty file
    const result2 = try testProcessContent(allocator, "empty.yass.yaml", "");
    defer freeTestRows(allocator, result2.rows);
    try testing.expect(result2.parse_failed);
    try testing.expectEqual(@as(usize, 0), result2.rows.len);
}

// ---------------------------------------------------------------------------
// Test: normalizeDescription edge cases
// ---------------------------------------------------------------------------

test "normalizeDescription empty string" {
    const allocator = testing.allocator;
    const result = try normalizeDescription(allocator, "");
    try testing.expectEqualStrings("", result);
}

test "normalizeDescription all whitespace" {
    const allocator = testing.allocator;
    const result = try normalizeDescription(allocator, "   \t\n\r  ");
    try testing.expectEqualStrings("", result);
}

// ---------------------------------------------------------------------------
// Test: formatRow output
// ---------------------------------------------------------------------------

test "formatRow produces tab-separated LF-terminated line" {
    const allocator = testing.allocator;
    const row = ListRow{
        .file = "specs/auth.yass.yaml",
        .spec_name = "auth-service",
        .description = "Authentication service",
    };
    const line = try formatRow(allocator, row);
    defer allocator.free(line);

    try testing.expectEqualStrings("specs/auth.yass.yaml\tauth-service\tAuthentication service\n", line);
}

test "formatRow with empty description" {
    const allocator = testing.allocator;
    const row = ListRow{
        .file = "spec.yass.yaml",
        .spec_name = "my-spec",
        .description = "",
    };
    const line = try formatRow(allocator, row);
    defer allocator.free(line);

    try testing.expectEqualStrings("spec.yass.yaml\tmy-spec\t\n", line);
}

// ---------------------------------------------------------------------------
// Test: countGraphemes
// ---------------------------------------------------------------------------

test "countGraphemes ASCII" {
    try testing.expectEqual(@as(usize, 5), countGraphemes("hello"));
    try testing.expectEqual(@as(usize, 0), countGraphemes(""));
}

test "countGraphemes UTF-8 multibyte" {
    // "caf" + e-acute (U+00E9, 2 bytes) = 4 codepoints
    try testing.expectEqual(@as(usize, 4), countGraphemes("caf\xc3\xa9"));
    // 3-byte UTF-8 codepoint (checkmark U+2713)
    try testing.expectEqual(@as(usize, 1), countGraphemes("\xe2\x9c\x93"));
}

// ---------------------------------------------------------------------------
// Test: truncation edge cases
// ---------------------------------------------------------------------------

test "truncation when overhead exceeds width" {
    const allocator = testing.allocator;

    // file=20, tab=1, name=15, tab=1 => overhead=37, marker=3 => 40
    // width=35 => overhead+marker >= width => empty
    const result = try truncateDescription(
        allocator,
        "a-very-long-filename", // 20
        "also-long-svcna", // 15
        "some description",
        35,
    );
    try testing.expectEqualStrings("", result);
}

test "truncation empty description passes through" {
    const allocator = testing.allocator;

    const result = try truncateDescription(
        allocator,
        "file.yaml",
        "svc",
        "",
        80,
    );
    try testing.expectEqualStrings("", result);
}

// ---------------------------------------------------------------------------
// Test: description that is non-string (mapping/sequence)
// ---------------------------------------------------------------------------

test "non-string description yields empty" {
    const allocator = testing.allocator;

    // description is a mapping, not a scalar
    const content =
        \\---
        \\description:
        \\  key: value
        \\version: 1
        \\---
        \\spec: test-spec
        \\
    ;
    const result = try testProcessContent(allocator, "nonsring.yass.yaml", content);
    defer freeTestRows(allocator, result.rows);

    try testing.expect(!result.parse_failed);
    try testing.expectEqual(@as(usize, 1), result.rows.len);
    try testing.expectEqualStrings("", result.rows[0].description);
}

// ---------------------------------------------------------------------------
// Test: first document with "spec" key is not treated as preamble
// ---------------------------------------------------------------------------

test "first document with spec key is not preamble" {
    const allocator = testing.allocator;

    // No preamble - first doc has spec key
    const content =
        \\---
        \\spec: direct-spec
        \\obligations:
        \\  - MUST work
        \\
    ;
    const result = try testProcessContent(allocator, "nopreamble.yass.yaml", content);
    defer freeTestRows(allocator, result.rows);

    try testing.expect(!result.parse_failed);
    try testing.expectEqual(@as(usize, 1), result.rows.len);
    try testing.expectEqualStrings("direct-spec", result.rows[0].spec_name);
    try testing.expectEqualStrings("", result.rows[0].description);
}
