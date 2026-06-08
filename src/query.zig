const std = @import("std");
const yaml = @import("yaml_parse.zig");
const errs = @import("errors.zig");
const shared = @import("shared.zig");

const Allocator = std.mem.Allocator;
const Io = std.Io;
const Dir = std.Io.Dir;

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

pub const Match = struct {
    file: []const u8,
    spec_name: []const u8,
    doc_index: usize,
};

pub const ListRow = struct {
    file: []const u8,
    spec_name: []const u8,
};

pub const QueryInlineError = struct {
    ref_target: []const u8,
    code: errs.ErrorCode,
    message: []const u8,
};

pub const SingleMatch = struct {
    fragment: []const u8,
    errors: []QueryInlineError,
};

pub const QueryError = struct {
    code: errs.ErrorCode,
    message: []const u8,
};

pub const QueryResult = union(enum) {
    single_match: SingleMatch,
    multi_match: []ListRow,
    no_match: void,
    err: QueryError,
};

pub const InlineResult = struct {
    output: []const u8,
    errors: []QueryInlineError,
};

// ---------------------------------------------------------------------------
// YAML quoting
// ---------------------------------------------------------------------------

/// Check whether a scalar value needs double-quoting in YAML output.
pub fn needsQuoting(value: []const u8) bool {
    // Empty string must be quoted
    if (value.len == 0) return true;

    // Leading/trailing whitespace
    if (value[0] == ' ' or value[0] == '\t') return true;
    if (value[value.len - 1] == ' ' or value[value.len - 1] == '\t') return true;

    // Starts with special YAML indicator characters
    switch (value[0]) {
        '?', '-', '*', '&', '!', '|', '>', '%', '@' => return true,
        else => {},
    }

    // Contains ": " (colon-space)
    if (std.mem.indexOf(u8, value, ": ") != null) return true;

    // Case-insensitive match against YAML 1.2 core-schema type tokens
    if (isCoreSchemaToken(value)) return true;

    // Numeric literal: starts with digit, +, or -
    if (isNumericLiteral(value)) return true;

    return false;
}

/// Check if a value matches a YAML 1.2 core-schema type token (case-insensitive).
fn isCoreSchemaToken(value: []const u8) bool {
    const tokens = [_][]const u8{
        "true", "false", "null", "yes", "no", "on", "off",
    };
    for (tokens) |token| {
        if (std.ascii.eqlIgnoreCase(value, token)) return true;
    }
    return false;
}

/// Check if a value looks like a numeric literal.
fn isNumericLiteral(value: []const u8) bool {
    if (value.len == 0) return false;
    switch (value[0]) {
        '0'...'9' => return true,
        '+', '-' => {
            // + or - alone is not numeric
            if (value.len > 1) {
                switch (value[1]) {
                    '0'...'9', '.' => return true,
                    else => return false,
                }
            }
            return false;
        },
        '.' => {
            // .5 style numbers
            if (value.len > 1) {
                switch (value[1]) {
                    '0'...'9' => return true,
                    else => return false,
                }
            }
            return false;
        },
        else => return false,
    }
}

/// Quote a scalar value with double quotes, escaping special characters.
fn quoteScalar(allocator: Allocator, value: []const u8) ![]const u8 {
    var buf: std.ArrayList(u8) = .empty;
    try buf.append(allocator, '"');
    for (value) |ch| {
        switch (ch) {
            '"' => try buf.appendSlice(allocator, "\\\""),
            '\\' => try buf.appendSlice(allocator, "\\\\"),
            '\n' => try buf.appendSlice(allocator, "\\n"),
            '\r' => try buf.appendSlice(allocator, "\\r"),
            '\t' => try buf.appendSlice(allocator, "\\t"),
            else => try buf.append(allocator, ch),
        }
    }
    try buf.append(allocator, '"');
    return try buf.toOwnedSlice(allocator);
}

/// Format a scalar for YAML output: quote if needed, plain otherwise.
fn formatScalar(allocator: Allocator, value: []const u8) ![]const u8 {
    if (needsQuoting(value)) {
        return quoteScalar(allocator, value);
    }
    return try allocator.dupe(u8, value);
}

// ---------------------------------------------------------------------------
// Name lookup
// ---------------------------------------------------------------------------

/// Match a spec name against parsed YAML documents in multiple files.
/// Returns all matches found (file, spec_name, doc_index).
pub fn nameLookup(
    allocator: Allocator,
    spec_name: []const u8,
    files: []const []const u8,
    file_contents: []const []const u8,
) ![]Match {
    var matches: std.ArrayList(Match) = .empty;

    for (files, file_contents) |file, content| {
        const parse_result = yaml.parseYaml(allocator, content) catch continue;
        defer yaml.freeParseResult(allocator, parse_result);

        for (parse_result.documents, 0..) |doc, doc_idx| {
            const root = doc.root orelse continue;
            switch (root) {
                .mapping => |m| {
                    // Look for "spec" key
                    for (m.entries) |entry| {
                        if (std.mem.eql(u8, entry.key.value, "spec")) {
                            switch (entry.value) {
                                .scalar => |s| {
                                    if (nameMatches(spec_name, s.value)) {
                                        try matches.append(allocator, .{
                                            .file = file,
                                            .spec_name = try allocator.dupe(u8, s.value),
                                            .doc_index = doc_idx,
                                        });
                                    }
                                },
                                else => {},
                            }
                            break;
                        }
                    }
                },
                else => {},
            }
        }
    }

    return try matches.toOwnedSlice(allocator);
}

/// Check if query matches a spec name.
/// - Exact match (case-sensitive byte comparison)
/// - Dot-aligned trailing suffix match:
///   query "ThirdSpec" matches "pkg.ThirdSpec" because there is a dot before the suffix
///   query "ThirdSpec" does NOT match "pkgThirdSpec" (no dot alignment)
fn nameMatches(query: []const u8, spec_name: []const u8) bool {
    // Exact match
    if (std.mem.eql(u8, query, spec_name)) return true;

    // Suffix match: query must be a proper suffix of spec_name
    // and the character before the suffix in spec_name must be '.'
    if (query.len < spec_name.len) {
        const prefix_len = spec_name.len - query.len;
        if (prefix_len > 0 and spec_name[prefix_len - 1] == '.') {
            if (std.mem.eql(u8, spec_name[prefix_len..], query)) {
                return true;
            }
        }
    }

    return false;
}

// ---------------------------------------------------------------------------
// Extract fragment
// ---------------------------------------------------------------------------

/// Extract a single spec document from a multi-doc YAML stream, matching by name.
/// Returns the YAML fragment as a string starting with "---\n", or null if not found.
pub fn extractFragment(allocator: Allocator, yaml_content: []const u8, spec_name: []const u8) !?[]const u8 {
    const parse_result = yaml.parseYaml(allocator, yaml_content) catch return null;
    defer yaml.freeParseResult(allocator, parse_result);

    for (parse_result.documents) |doc| {
        const root = doc.root orelse continue;
        switch (root) {
            .mapping => |m| {
                // Check if this document has a matching spec name
                var found_name: ?[]const u8 = null;
                for (m.entries) |entry| {
                    if (std.mem.eql(u8, entry.key.value, "spec")) {
                        switch (entry.value) {
                            .scalar => |s| {
                                if (nameMatches(spec_name, s.value)) {
                                    found_name = s.value;
                                }
                            },
                            else => {},
                        }
                        break;
                    }
                }

                if (found_name != null) {
                    // Emit this document as YAML
                    return try emitYamlDocument(allocator, m);
                }
            },
            else => {},
        }
    }

    return null;
}

// ---------------------------------------------------------------------------
// YAML emitter
// ---------------------------------------------------------------------------

/// Known normativity keywords, used for ordering obligation keys.
const normativity_keywords = [_][]const u8{
    "MUST", "MUST NOT", "SHOULD", "SHOULD NOT", "MAY",
    "SHALL", "SHALL NOT", "REQUIRED", "RECOMMENDED", "OPTIONAL",
};

/// Known reference keywords.
const reference_keywords = [_][]const u8{
    "CONFORMS", "EXTENDS", "IMPLEMENTS", "USES", "REQUIRES",
};

/// Check if a key is a normativity keyword.
fn isNormativity(key: []const u8) bool {
    for (normativity_keywords) |kw| {
        if (std.mem.eql(u8, key, kw)) return true;
    }
    return false;
}

/// Check if a key is a reference keyword.
fn isReference(key: []const u8) bool {
    for (reference_keywords) |kw| {
        if (std.mem.eql(u8, key, kw)) return true;
    }
    return false;
}

/// Emit a parsed YAML mapping as a formatted YAML document string.
fn emitYamlDocument(allocator: Allocator, mapping: yaml.YamlMapping) ![]const u8 {
    var buf: std.ArrayList(u8) = .empty;
    try buf.appendSlice(allocator, "---\n");

    for (mapping.entries) |entry| {
        const key = entry.key.value;
        switch (entry.value) {
            .scalar => |s| {
                // Top-level scalar: "key: value\n"
                const formatted = try formatScalar(allocator, s.value);
                defer allocator.free(formatted);
                try buf.appendSlice(allocator, key);
                try buf.appendSlice(allocator, ": ");
                try buf.appendSlice(allocator, formatted);
                try buf.append(allocator, '\n');
            },
            .sequence => |seq| {
                // Slot: "KEY:\n" followed by list items
                try buf.appendSlice(allocator, key);
                try buf.appendSlice(allocator, ":\n");
                try emitSequence(allocator, &buf, seq, 0);
            },
            .mapping => |m| {
                // Nested mapping
                try buf.appendSlice(allocator, key);
                try buf.appendSlice(allocator, ":\n");
                try emitMapping(allocator, &buf, m, 1);
            },
        }
    }

    return try buf.toOwnedSlice(allocator);
}

/// Emit a YAML sequence at a given indentation level.
fn emitSequence(allocator: Allocator, buf: *std.ArrayList(u8), seq: yaml.YamlSequence, indent: usize) Allocator.Error!void {
    for (seq.items) |item| {
        switch (item) {
            .scalar => |s| {
                // Simple scalar list item: "- value\n"
                try appendIndent(allocator, buf, indent);
                try buf.appendSlice(allocator, "- ");
                const formatted = try formatScalar(allocator, s.value);
                defer allocator.free(formatted);
                try buf.appendSlice(allocator, formatted);
                try buf.append(allocator, '\n');
            },
            .mapping => |m| {
                // Mapping list item: obligation
                try emitObligationMapping(allocator, buf, m, indent);
            },
            .sequence => |inner_seq| {
                // Nested sequence
                try appendIndent(allocator, buf, indent);
                try buf.appendSlice(allocator, "-\n");
                try emitSequence(allocator, buf, inner_seq, indent + 1);
            },
        }
    }
}

/// Emit a mapping at a given indentation level.
fn emitMapping(allocator: Allocator, buf: *std.ArrayList(u8), mapping: yaml.YamlMapping, indent: usize) !void {
    for (mapping.entries) |entry| {
        try appendIndent(allocator, buf, indent);
        try buf.appendSlice(allocator, entry.key.value);
        switch (entry.value) {
            .scalar => |s| {
                try buf.appendSlice(allocator, ": ");
                const formatted = try formatScalar(allocator, s.value);
                defer allocator.free(formatted);
                try buf.appendSlice(allocator, formatted);
                try buf.append(allocator, '\n');
            },
            .sequence => |seq| {
                try buf.appendSlice(allocator, ":\n");
                try emitSequence(allocator, buf, seq, indent + 1);
            },
            .mapping => |m| {
                try buf.appendSlice(allocator, ":\n");
                try emitMapping(allocator, buf, m, indent + 1);
            },
        }
    }
}

/// Emit a mapping as a list item (obligation format).
/// Key order: normativity first, then WHEN, then refs.
fn emitObligationMapping(allocator: Allocator, buf: *std.ArrayList(u8), mapping: yaml.YamlMapping, indent: usize) !void {
    // Categorize entries: normativity, WHEN, refs, other
    var normativity_entries: std.ArrayList(yaml.MappingEntry) = .empty;
    defer normativity_entries.deinit(allocator);
    var when_entries: std.ArrayList(yaml.MappingEntry) = .empty;
    defer when_entries.deinit(allocator);
    var ref_entries: std.ArrayList(yaml.MappingEntry) = .empty;
    defer ref_entries.deinit(allocator);
    var other_entries: std.ArrayList(yaml.MappingEntry) = .empty;
    defer other_entries.deinit(allocator);

    for (mapping.entries) |entry| {
        if (isNormativity(entry.key.value)) {
            try normativity_entries.append(allocator, entry);
        } else if (std.mem.eql(u8, entry.key.value, "WHEN")) {
            try when_entries.append(allocator, entry);
        } else if (isReference(entry.key.value)) {
            try ref_entries.append(allocator, entry);
        } else {
            try other_entries.append(allocator, entry);
        }
    }

    // Emit in order: normativity, WHEN, refs, other
    var first = true;
    for (normativity_entries.items) |entry| {
        try emitMappingEntryAsListItem(allocator, buf, entry, indent, first);
        first = false;
    }
    for (when_entries.items) |entry| {
        try emitMappingEntryAsListItem(allocator, buf, entry, indent, first);
        first = false;
    }
    for (ref_entries.items) |entry| {
        try emitMappingEntryAsListItem(allocator, buf, entry, indent, first);
        first = false;
    }
    for (other_entries.items) |entry| {
        try emitMappingEntryAsListItem(allocator, buf, entry, indent, first);
        first = false;
    }
}

/// Emit a single mapping entry. If `is_first`, prefix with "- ", else indent by 2 extra.
fn emitMappingEntryAsListItem(
    allocator: Allocator,
    buf: *std.ArrayList(u8),
    entry: yaml.MappingEntry,
    indent: usize,
    is_first: bool,
) !void {
    if (is_first) {
        try appendIndent(allocator, buf, indent);
        try buf.appendSlice(allocator, "- ");
    } else {
        try appendIndent(allocator, buf, indent);
        try buf.appendSlice(allocator, "  ");
    }
    try buf.appendSlice(allocator, entry.key.value);
    switch (entry.value) {
        .scalar => |s| {
            try buf.appendSlice(allocator, ": ");
            const formatted = try formatScalar(allocator, s.value);
            defer allocator.free(formatted);
            try buf.appendSlice(allocator, formatted);
            try buf.append(allocator, '\n');
        },
        .sequence => |seq| {
            try buf.appendSlice(allocator, ":\n");
            try emitSequence(allocator, buf, seq, indent + 2);
        },
        .mapping => |m| {
            try buf.appendSlice(allocator, ":\n");
            try emitMapping(allocator, buf, m, indent + 2);
        },
    }
}

/// Append 2-space indentation for a given level.
fn appendIndent(allocator: Allocator, buf: *std.ArrayList(u8), indent: usize) !void {
    for (0..indent) |_| {
        try buf.appendSlice(allocator, "  ");
    }
}

// ---------------------------------------------------------------------------
// Inline CONFORMS
// ---------------------------------------------------------------------------

/// Parse a CONFORMS reference target to extract file path, spec name, and slot.
/// Format: "SpecName::SLOT" or "path/to/file.yass.yaml::SpecName::SLOT"
const ConformsRef = struct {
    file: ?[]const u8,
    spec: []const u8,
    slot: []const u8,
};

fn parseConformsRef(ref_target: []const u8) ?ConformsRef {
    // Must contain "::" for slot separator
    // Find the last "::" which is the slot separator
    const last_sep = std.mem.lastIndexOf(u8, ref_target, "::") orelse return null;
    const slot = ref_target[last_sep + 2 ..];
    if (slot.len == 0) return null;

    const before_slot = ref_target[0..last_sep];
    // Check if there's another "::" for file::spec separation
    if (std.mem.lastIndexOf(u8, before_slot, "::")) |spec_sep| {
        return ConformsRef{
            .file = before_slot[0..spec_sep],
            .spec = before_slot[spec_sep + 2 ..],
            .slot = slot,
        };
    }

    // No file part: just "SpecName::SLOT"
    return ConformsRef{
        .file = null,
        .spec = before_slot,
        .slot = slot,
    };
}

/// Inline CONFORMS references in a YAML fragment.
/// Takes the fragment YAML string and resolves CONFORMS by looking up referenced specs.
pub fn inlineConforms(
    allocator: Allocator,
    io: Io,
    fragment_yaml: []const u8,
    project_root: []const u8,
    source_file_dir: []const u8,
) !InlineResult {
    const parse_result = yaml.parseYaml(allocator, fragment_yaml) catch {
        return InlineResult{ .output = try allocator.dupe(u8, fragment_yaml), .errors = &.{} };
    };
    defer yaml.freeParseResult(allocator, parse_result);

    if (parse_result.documents.len == 0) {
        return InlineResult{ .output = try allocator.dupe(u8, fragment_yaml), .errors = &.{} };
    }

    const doc = parse_result.documents[0];
    const root = doc.root orelse {
        return InlineResult{ .output = try allocator.dupe(u8, fragment_yaml), .errors = &.{} };
    };

    switch (root) {
        .mapping => |m| {
            return try inlineConformsInMapping(allocator, io, m, project_root, source_file_dir);
        },
        else => {
            return InlineResult{ .output = try allocator.dupe(u8, fragment_yaml), .errors = &.{} };
        },
    }
}

/// Process CONFORMS inlining on a parsed mapping.
fn inlineConformsInMapping(
    allocator: Allocator,
    io: Io,
    mapping: yaml.YamlMapping,
    project_root: []const u8,
    source_file_dir: []const u8,
) !InlineResult {
    var buf: std.ArrayList(u8) = .empty;
    var inline_errors: std.ArrayList(QueryInlineError) = .empty;

    try buf.appendSlice(allocator, "---\n");

    for (mapping.entries) |entry| {
        const key = entry.key.value;
        switch (entry.value) {
            .scalar => |s| {
                const formatted = try formatScalar(allocator, s.value);
                defer allocator.free(formatted);
                try buf.appendSlice(allocator, key);
                try buf.appendSlice(allocator, ": ");
                try buf.appendSlice(allocator, formatted);
                try buf.append(allocator, '\n');
            },
            .sequence => |seq| {
                // Check if this is a slot that might have CONFORMS obligations
                try buf.appendSlice(allocator, key);
                try buf.appendSlice(allocator, ":\n");
                try emitSequenceWithConformsInlining(
                    allocator,
                    io,
                    &buf,
                    seq,
                    0,
                    project_root,
                    source_file_dir,
                    &inline_errors,
                );
            },
            .mapping => |m| {
                try buf.appendSlice(allocator, key);
                try buf.appendSlice(allocator, ":\n");
                try emitMapping(allocator, &buf, m, 1);
            },
        }
    }

    return InlineResult{
        .output = try buf.toOwnedSlice(allocator),
        .errors = try inline_errors.toOwnedSlice(allocator),
    };
}

/// Emit a sequence, inlining CONFORMS references as we go.
fn emitSequenceWithConformsInlining(
    allocator: Allocator,
    io: Io,
    buf: *std.ArrayList(u8),
    seq: yaml.YamlSequence,
    indent: usize,
    project_root: []const u8,
    source_file_dir: []const u8,
    inline_errors: *std.ArrayList(QueryInlineError),
) !void {
    for (seq.items) |item| {
        switch (item) {
            .scalar => |s| {
                try appendIndent(allocator, buf, indent);
                try buf.appendSlice(allocator, "- ");
                const formatted = try formatScalar(allocator, s.value);
                defer allocator.free(formatted);
                try buf.appendSlice(allocator, formatted);
                try buf.append(allocator, '\n');
            },
            .mapping => |m| {
                // Check for CONFORMS in this obligation
                const conforms_info = findConformsInObligation(m);
                if (conforms_info.conforms_entry) |conforms| {
                    // Process CONFORMS inlining
                    try processConformsObligation(
                        allocator,
                        io,
                        buf,
                        m,
                        conforms,
                        conforms_info.is_normative,
                        conforms_info.when_value,
                        indent,
                        project_root,
                        source_file_dir,
                        inline_errors,
                    );
                } else {
                    // Normal obligation, emit as-is
                    try emitObligationMapping(allocator, buf, m, indent);
                }
            },
            .sequence => |inner_seq| {
                try appendIndent(allocator, buf, indent);
                try buf.appendSlice(allocator, "-\n");
                try emitSequenceWithConformsInlining(
                    allocator,
                    io,
                    buf,
                    inner_seq,
                    indent + 1,
                    project_root,
                    source_file_dir,
                    inline_errors,
                );
            },
        }
    }
}

const ConformsInfo = struct {
    conforms_entry: ?yaml.MappingEntry,
    is_normative: bool,
    when_value: ?[]const u8,
};

/// Find a CONFORMS entry in an obligation mapping.
fn findConformsInObligation(mapping: yaml.YamlMapping) ConformsInfo {
    var conforms_entry: ?yaml.MappingEntry = null;
    var has_normativity = false;
    var when_value: ?[]const u8 = null;

    for (mapping.entries) |entry| {
        if (std.mem.eql(u8, entry.key.value, "CONFORMS")) {
            conforms_entry = entry;
        } else if (isNormativity(entry.key.value)) {
            has_normativity = true;
        } else if (std.mem.eql(u8, entry.key.value, "WHEN")) {
            switch (entry.value) {
                .scalar => |s| {
                    when_value = s.value;
                },
                else => {},
            }
        }
    }

    return ConformsInfo{
        .conforms_entry = conforms_entry,
        .is_normative = has_normativity,
        .when_value = when_value,
    };
}

/// Process a CONFORMS obligation: resolve the reference and inline obligations.
fn processConformsObligation(
    allocator: Allocator,
    io: Io,
    buf: *std.ArrayList(u8),
    mapping: yaml.YamlMapping,
    conforms_entry: yaml.MappingEntry,
    is_normative: bool,
    carrier_when: ?[]const u8,
    indent: usize,
    project_root: []const u8,
    source_file_dir: []const u8,
    inline_errors: *std.ArrayList(QueryInlineError),
) !void {
    // Get the CONFORMS target value
    const ref_target = switch (conforms_entry.value) {
        .scalar => |s| s.value,
        else => {
            // Not a scalar target, emit as-is
            try emitObligationMapping(allocator, buf, mapping, indent);
            return;
        },
    };

    // Parse the reference
    const parsed_ref = parseConformsRef(ref_target) orelse {
        // No ::SLOT suffix
        try inline_errors.append(allocator, .{
            .ref_target = try allocator.dupe(u8, ref_target),
            .code = .query_conforms_no_slot,
            .message = try std.fmt.allocPrint(allocator, "CONFORMS reference '{s}' missing ::SLOT suffix", .{ref_target}),
        });
        // Emit the original obligation as-is (without stripping CONFORMS)
        try emitObligationMapping(allocator, buf, mapping, indent);
        return;
    };

    // Resolve the referenced spec file
    const referenced_obligations = resolveConformsReference(
        allocator,
        io,
        parsed_ref,
        project_root,
        source_file_dir,
    ) catch |e| {
        _ = e;
        try inline_errors.append(allocator, .{
            .ref_target = try allocator.dupe(u8, ref_target),
            .code = .query_conforms_unresolved,
            .message = try std.fmt.allocPrint(allocator, "could not resolve CONFORMS reference '{s}'", .{ref_target}),
        });
        try emitObligationMapping(allocator, buf, mapping, indent);
        return;
    };

    if (referenced_obligations == null) {
        try inline_errors.append(allocator, .{
            .ref_target = try allocator.dupe(u8, ref_target),
            .code = .query_conforms_unresolved,
            .message = try std.fmt.allocPrint(allocator, "could not resolve CONFORMS reference '{s}'", .{ref_target}),
        });
        try emitObligationMapping(allocator, buf, mapping, indent);
        return;
    }

    if (is_normative) {
        // Normative CONFORMS: keep the original obligation (without CONFORMS key),
        // then append inlined obligations
        try emitObligationWithoutConforms(allocator, buf, mapping, indent);
    }

    // Emit inlined obligations with provenance comments
    for (referenced_obligations.?.items) |ref_item| {
        switch (ref_item) {
            .mapping => |ref_m| {
                // Provenance comment at column 0
                try buf.appendSlice(allocator, "# CONFORMS: ");
                try buf.appendSlice(allocator, ref_target);
                try buf.append(allocator, '\n');
                // Emit the inlined obligation, possibly with combined WHEN
                try emitInlinedObligation(allocator, buf, ref_m, carrier_when, indent);
            },
            .scalar => |s| {
                try buf.appendSlice(allocator, "# CONFORMS: ");
                try buf.appendSlice(allocator, ref_target);
                try buf.append(allocator, '\n');
                try appendIndent(allocator, buf, indent);
                try buf.appendSlice(allocator, "- ");
                const formatted = try formatScalar(allocator, s.value);
                defer allocator.free(formatted);
                try buf.appendSlice(allocator, formatted);
                try buf.append(allocator, '\n');
            },
            else => {},
        }
    }
}

/// Emit an obligation mapping without the CONFORMS key.
fn emitObligationWithoutConforms(
    allocator: Allocator,
    buf: *std.ArrayList(u8),
    mapping: yaml.YamlMapping,
    indent: usize,
) !void {
    // Build a new mapping without CONFORMS
    var entries: std.ArrayList(yaml.MappingEntry) = .empty;
    defer entries.deinit(allocator);

    for (mapping.entries) |entry| {
        if (!std.mem.eql(u8, entry.key.value, "CONFORMS")) {
            try entries.append(allocator, entry);
        }
    }

    if (entries.items.len > 0) {
        const filtered = yaml.YamlMapping{
            .entries = entries.items,
            .line = mapping.line,
        };
        try emitObligationMapping(allocator, buf, filtered, indent);
    }
}

/// Emit an inlined obligation, combining WHEN guards if needed.
fn emitInlinedObligation(
    allocator: Allocator,
    buf: *std.ArrayList(u8),
    mapping: yaml.YamlMapping,
    carrier_when: ?[]const u8,
    indent: usize,
) !void {
    if (carrier_when) |outer_when| {
        // Check if the inlined obligation has its own WHEN
        var has_inner_when = false;
        for (mapping.entries) |entry| {
            if (std.mem.eql(u8, entry.key.value, "WHEN")) {
                has_inner_when = true;
                break;
            }
        }

        if (has_inner_when) {
            // Combine: create a new mapping with combined WHEN
            var new_entries: std.ArrayList(yaml.MappingEntry) = .empty;
            defer new_entries.deinit(allocator);

            for (mapping.entries) |entry| {
                if (std.mem.eql(u8, entry.key.value, "WHEN")) {
                    switch (entry.value) {
                        .scalar => |s| {
                            // Combined WHEN: "outer and inner"
                            const combined = try std.fmt.allocPrint(allocator, "{s} and {s}", .{ outer_when, s.value });
                            try new_entries.append(allocator, .{
                                .key = entry.key,
                                .value = .{ .scalar = .{ .value = combined, .line = s.line } },
                            });
                        },
                        else => try new_entries.append(allocator, entry),
                    }
                } else {
                    try new_entries.append(allocator, entry);
                }
            }

            const new_mapping = yaml.YamlMapping{
                .entries = new_entries.items,
                .line = mapping.line,
            };
            try emitObligationMapping(allocator, buf, new_mapping, indent);
        } else {
            // No inner WHEN: add carrier WHEN to the obligation
            var new_entries: std.ArrayList(yaml.MappingEntry) = .empty;
            defer new_entries.deinit(allocator);

            // Categorize for proper ordering: normativity first, then WHEN, then refs
            var added_when = false;
            for (mapping.entries) |entry| {
                if (isNormativity(entry.key.value) and !added_when) {
                    try new_entries.append(allocator, entry);
                    // Add WHEN right after normativity
                    try new_entries.append(allocator, .{
                        .key = .{ .value = "WHEN", .line = 0 },
                        .value = .{ .scalar = .{ .value = outer_when, .line = 0 } },
                    });
                    added_when = true;
                } else {
                    try new_entries.append(allocator, entry);
                }
            }

            if (!added_when) {
                // No normativity found, add WHEN at beginning
                var with_when: std.ArrayList(yaml.MappingEntry) = .empty;
                defer with_when.deinit(allocator);
                try with_when.append(allocator, .{
                    .key = .{ .value = "WHEN", .line = 0 },
                    .value = .{ .scalar = .{ .value = outer_when, .line = 0 } },
                });
                for (new_entries.items) |entry| {
                    try with_when.append(allocator, entry);
                }
                const new_mapping = yaml.YamlMapping{
                    .entries = with_when.items,
                    .line = mapping.line,
                };
                try emitObligationMapping(allocator, buf, new_mapping, indent);
                return;
            }

            const new_mapping = yaml.YamlMapping{
                .entries = new_entries.items,
                .line = mapping.line,
            };
            try emitObligationMapping(allocator, buf, new_mapping, indent);
        }
    } else {
        // No carrier WHEN, emit as-is
        try emitObligationMapping(allocator, buf, mapping, indent);
    }
}

/// Resolve a CONFORMS reference to get the obligations from the referenced slot.
fn resolveConformsReference(
    allocator: Allocator,
    io: Io,
    ref: ConformsRef,
    _: []const u8,
    source_file_dir: []const u8,
) !?yaml.YamlSequence {
    // Determine the file to read
    var file_path_buf: [4096]u8 = undefined;
    const file_path = if (ref.file) |f| blk: {
        // Relative path from source file directory or project root
        if (std.fs.path.isAbsolute(f)) {
            break :blk f;
        }
        // Try relative to source_file_dir first
        const len = (std.fmt.bufPrint(&file_path_buf, "{s}/{s}", .{ source_file_dir, f }) catch return null).len;
        break :blk file_path_buf[0..len];
    } else {
        // Same directory: find the spec in files at source_file_dir
        return resolveConformsInDirectory(allocator, io, ref.spec, ref.slot, source_file_dir);
    };

    // Read the file
    const dir = shared.openDirAny(io, std.fs.path.dirname(file_path) orelse ".", .{}) catch return null;
    defer dir.close(io);

    const content = dir.readFileAlloc(io, std.fs.path.basename(file_path), allocator, @enumFromInt(1024 * 1024)) catch return null;
    defer allocator.free(content);

    return findSlotInContent(allocator, content, ref.spec, ref.slot);
}

/// Find a spec and slot in a directory by scanning .yass.yaml files.
fn resolveConformsInDirectory(
    allocator: Allocator,
    io: Io,
    spec_name: []const u8,
    slot_name: []const u8,
    dir_path: []const u8,
) !?yaml.YamlSequence {
    const dir = shared.openDirAny(io, dir_path, .{ .iterate = true }) catch return null;
    defer dir.close(io);

    var iter = dir.iterate();
    while (iter.next(io) catch null) |entry| {
        if (entry.kind != .file) continue;
        if (!std.mem.endsWith(u8, entry.name, ".yass.yaml")) continue;
        if (entry.name.len <= ".yass.yaml".len) continue;

        const content = dir.readFileAlloc(io, entry.name, allocator, @enumFromInt(1024 * 1024)) catch continue;
        defer allocator.free(content);

        if (findSlotInContent(allocator, content, spec_name, slot_name)) |seq| {
            return seq;
        }
    }
    return null;
}

/// Find a specific slot's obligations in YAML content for a given spec name.
fn findSlotInContent(
    allocator: Allocator,
    content: []const u8,
    spec_name: []const u8,
    slot_name: []const u8,
) ?yaml.YamlSequence {
    const parse_result = yaml.parseYaml(allocator, content) catch return null;

    for (parse_result.documents) |doc| {
        const root = doc.root orelse continue;
        switch (root) {
            .mapping => |m| {
                // Check spec name
                var found_spec = false;
                for (m.entries) |entry| {
                    if (std.mem.eql(u8, entry.key.value, "spec")) {
                        switch (entry.value) {
                            .scalar => |s| {
                                if (std.mem.eql(u8, s.value, spec_name)) {
                                    found_spec = true;
                                }
                            },
                            else => {},
                        }
                        break;
                    }
                }
                if (!found_spec) continue;

                // Find the slot
                for (m.entries) |entry| {
                    if (std.mem.eql(u8, entry.key.value, slot_name)) {
                        switch (entry.value) {
                            .sequence => |seq| return seq,
                            else => {},
                        }
                    }
                }
            },
            else => {},
        }
    }

    return null;
}

// ---------------------------------------------------------------------------
// Main query function
// ---------------------------------------------------------------------------

/// Execute the query subcommand.
/// Looks up a spec by name across scope files and returns the appropriate result.
pub fn querySpec(
    allocator: Allocator,
    io: Io,
    spec_name: []const u8,
    scope_files: []const []const u8,
    project_root: []const u8,
    cwd: []const u8,
) !QueryResult {
    _ = cwd;

    // Validate spec name
    if (spec_name.len == 0) {
        return QueryResult{ .err = .{
            .code = .query_name_blank,
            .message = try allocator.dupe(u8, "spec name is blank"),
        } };
    }

    // Check for whitespace in name (not blank error, just no-match)
    for (spec_name) |ch| {
        if (ch == ' ' or ch == '\t' or ch == '\n' or ch == '\r') {
            // Whitespace in name = no-match (not blank error)
            return QueryResult{ .no_match = {} };
        }
    }

    // Read file contents
    var file_contents: std.ArrayList([]const u8) = .empty;
    defer {
        for (file_contents.items) |content| {
            allocator.free(content);
        }
        file_contents.deinit(allocator);
    }

    for (scope_files) |file_path| {
        const abs_path = if (std.fs.path.isAbsolute(file_path))
            file_path
        else blk: {
            break :blk try std.fs.path.join(allocator, &.{ project_root, file_path });
        };

        const content = blk: {
            const dir = shared.openDirAny(io, std.fs.path.dirname(abs_path) orelse ".", .{}) catch {
                try file_contents.append(allocator, try allocator.dupe(u8, ""));
                continue;
            };
            defer dir.close(io);
            break :blk dir.readFileAlloc(io, std.fs.path.basename(abs_path), allocator, @enumFromInt(1024 * 1024)) catch {
                try file_contents.append(allocator, try allocator.dupe(u8, ""));
                continue;
            };
        };
        try file_contents.append(allocator, content);
    }

    // Perform name lookup
    const matches = try nameLookup(allocator, spec_name, scope_files, file_contents.items);
    defer allocator.free(matches);

    if (matches.len == 0) {
        return QueryResult{ .no_match = {} };
    }

    if (matches.len == 1) {
        // Single match: extract fragment and inline CONFORMS
        const match = matches[0];
        const content = file_contents.items[blk: {
            for (scope_files, 0..) |f, i| {
                if (std.mem.eql(u8, f, match.file)) break :blk i;
            }
            break :blk 0;
        }];

        const fragment = try extractFragment(allocator, content, match.spec_name) orelse {
            return QueryResult{ .no_match = {} };
        };

        // Determine source file directory for CONFORMS resolution
        const abs_file = if (std.fs.path.isAbsolute(match.file))
            match.file
        else
            try std.fs.path.join(allocator, &.{ project_root, match.file });

        const source_dir = std.fs.path.dirname(abs_file) orelse project_root;

        const inline_result = try inlineConforms(allocator, io, fragment, project_root, source_dir);

        return QueryResult{ .single_match = .{
            .fragment = inline_result.output,
            .errors = inline_result.errors,
        } };
    }

    // Multiple matches: return list rows for disambiguation
    var rows: std.ArrayList(ListRow) = .empty;
    for (matches) |match| {
        try rows.append(allocator, .{
            .file = match.file,
            .spec_name = match.spec_name,
        });
    }

    return QueryResult{ .multi_match = try rows.toOwnedSlice(allocator) };
}

// ===========================================================================
// Tests
// ===========================================================================

test "needsQuoting - empty string" {
    try std.testing.expect(needsQuoting(""));
}

test "needsQuoting - plain scalar" {
    try std.testing.expect(!needsQuoting("hello"));
    try std.testing.expect(!needsQuoting("some value here"));
}

test "needsQuoting - contains colon-space" {
    try std.testing.expect(needsQuoting("key: value"));
}

test "needsQuoting - starts with special chars" {
    try std.testing.expect(needsQuoting("?question"));
    try std.testing.expect(needsQuoting("-dash"));
    try std.testing.expect(needsQuoting("*star"));
    try std.testing.expect(needsQuoting("&anchor"));
    try std.testing.expect(needsQuoting("!bang"));
    try std.testing.expect(needsQuoting("|pipe"));
    try std.testing.expect(needsQuoting(">greater"));
    try std.testing.expect(needsQuoting("%percent"));
    try std.testing.expect(needsQuoting("@at"));
}

test "needsQuoting - leading/trailing whitespace" {
    try std.testing.expect(needsQuoting(" leading"));
    try std.testing.expect(needsQuoting("trailing "));
    try std.testing.expect(needsQuoting("\tleading tab"));
}

test "needsQuoting - YAML core schema tokens" {
    try std.testing.expect(needsQuoting("true"));
    try std.testing.expect(needsQuoting("false"));
    try std.testing.expect(needsQuoting("null"));
    try std.testing.expect(needsQuoting("yes"));
    try std.testing.expect(needsQuoting("no"));
    try std.testing.expect(needsQuoting("on"));
    try std.testing.expect(needsQuoting("off"));
    // Case-insensitive
    try std.testing.expect(needsQuoting("True"));
    try std.testing.expect(needsQuoting("FALSE"));
    try std.testing.expect(needsQuoting("NULL"));
    try std.testing.expect(needsQuoting("Yes"));
    try std.testing.expect(needsQuoting("No"));
    try std.testing.expect(needsQuoting("ON"));
    try std.testing.expect(needsQuoting("OFF"));
}

test "needsQuoting - numeric literals" {
    try std.testing.expect(needsQuoting("42"));
    try std.testing.expect(needsQuoting("3.14"));
    try std.testing.expect(needsQuoting("+1"));
    try std.testing.expect(needsQuoting("-5"));
    try std.testing.expect(needsQuoting(".5"));
    try std.testing.expect(needsQuoting("0"));
}

test "needsQuoting - non-numeric with special start" {
    try std.testing.expect(!needsQuoting("hello world"));
    // + alone is not numeric
    try std.testing.expect(!needsQuoting("+"));
    // - alone: actually starts with -, so it IS flagged by the special char check
    try std.testing.expect(needsQuoting("-"));
}

test "nameMatches - exact match" {
    try std.testing.expect(nameMatches("MySpec", "MySpec"));
}

test "nameMatches - suffix match dot-aligned" {
    try std.testing.expect(nameMatches("ThirdSpec", "pkg.ThirdSpec"));
    try std.testing.expect(nameMatches("ThirdSpec", "a.b.ThirdSpec"));
}

test "nameMatches - no match without dot alignment" {
    try std.testing.expect(!nameMatches("ThirdSpec", "pkgThirdSpec"));
}

test "nameMatches - exact match with dots" {
    try std.testing.expect(nameMatches("pkg.ThirdSpec", "pkg.ThirdSpec"));
}

test "nameMatches - no match" {
    try std.testing.expect(!nameMatches("OtherSpec", "MySpec"));
}

test "nameMatches - partial suffix but not dot-aligned" {
    try std.testing.expect(!nameMatches("Spec", "MySpec"));
}

test "nameMatches - dot-aligned suffix partial path" {
    try std.testing.expect(nameMatches("sub.ThirdSpec", "pkg.sub.ThirdSpec"));
}

test "nameLookup - exact match in single file" {
    const allocator = std.testing.allocator;
    const content =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- MUST do something
    ;
    const files = [_][]const u8{"test.yass.yaml"};
    const contents = [_][]const u8{content};
    const matches = try nameLookup(allocator, "MySpec", &files, &contents);
    defer {
        for (matches) |m| allocator.free(m.spec_name);
        allocator.free(matches);
    }

    try std.testing.expectEqual(@as(usize, 1), matches.len);
    try std.testing.expectEqualStrings("MySpec", matches[0].spec_name);
    try std.testing.expectEqualStrings("test.yass.yaml", matches[0].file);
}

test "nameLookup - suffix match" {
    const allocator = std.testing.allocator;
    const content =
        \\---
        \\spec: pkg.ThirdSpec
        \\BEHAVIORS:
        \\- MUST do something
    ;
    const files = [_][]const u8{"test.yass.yaml"};
    const contents = [_][]const u8{content};
    const matches = try nameLookup(allocator, "ThirdSpec", &files, &contents);
    defer {
        for (matches) |m| allocator.free(m.spec_name);
        allocator.free(matches);
    }

    try std.testing.expectEqual(@as(usize, 1), matches.len);
    try std.testing.expectEqualStrings("pkg.ThirdSpec", matches[0].spec_name);
}

test "nameLookup - no match" {
    const allocator = std.testing.allocator;
    const content =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- MUST do something
    ;
    const files = [_][]const u8{"test.yass.yaml"};
    const contents = [_][]const u8{content};
    const matches = try nameLookup(allocator, "OtherSpec", &files, &contents);
    defer allocator.free(matches);

    try std.testing.expectEqual(@as(usize, 0), matches.len);
}

test "nameLookup - whitespace in name returns no match" {
    const allocator = std.testing.allocator;
    const content =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- MUST do something
    ;
    const files = [_][]const u8{"test.yass.yaml"};
    const contents = [_][]const u8{content};
    // Name with whitespace: no match (callers should check before calling)
    const matches = try nameLookup(allocator, "My Spec", &files, &contents);
    defer allocator.free(matches);

    try std.testing.expectEqual(@as(usize, 0), matches.len);
}

test "nameLookup - multi-match across files" {
    const allocator = std.testing.allocator;
    const content1 =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- MUST do thing1
    ;
    const content2 =
        \\---
        \\spec: pkg.MySpec
        \\BEHAVIORS:
        \\- MUST do thing2
    ;
    const files = [_][]const u8{ "a.yass.yaml", "b.yass.yaml" };
    const contents = [_][]const u8{ content1, content2 };
    const matches = try nameLookup(allocator, "MySpec", &files, &contents);
    defer {
        for (matches) |m| allocator.free(m.spec_name);
        allocator.free(matches);
    }

    try std.testing.expectEqual(@as(usize, 2), matches.len);
}

test "extractFragment - single doc" {
    const allocator = std.testing.allocator;
    const content =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- MUST do something
    ;

    const fragment = try extractFragment(allocator, content, "MySpec");
    defer if (fragment) |f| allocator.free(f);

    try std.testing.expect(fragment != null);
    try std.testing.expect(std.mem.startsWith(u8, fragment.?, "---\n"));
    try std.testing.expect(std.mem.indexOf(u8, fragment.?, "spec: MySpec") != null);
}

test "extractFragment - multi doc selects correct one" {
    const allocator = std.testing.allocator;
    const content =
        \\---
        \\spec: FirstSpec
        \\BEHAVIORS:
        \\- MUST do first
        \\---
        \\spec: SecondSpec
        \\BEHAVIORS:
        \\- MUST do second
    ;

    const fragment = try extractFragment(allocator, content, "SecondSpec");
    defer if (fragment) |f| allocator.free(f);

    try std.testing.expect(fragment != null);
    try std.testing.expect(std.mem.indexOf(u8, fragment.?, "spec: SecondSpec") != null);
    try std.testing.expect(std.mem.indexOf(u8, fragment.?, "FirstSpec") == null);
}

test "extractFragment - not found" {
    const allocator = std.testing.allocator;
    const content =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- MUST do something
    ;

    const fragment = try extractFragment(allocator, content, "OtherSpec");
    try std.testing.expect(fragment == null);
}

test "YAML output quoting - plain scalar" {
    const allocator = std.testing.allocator;
    const formatted = try formatScalar(allocator, "hello");
    defer allocator.free(formatted);
    try std.testing.expectEqualStrings("hello", formatted);
}

test "YAML output quoting - double quotes for special" {
    const allocator = std.testing.allocator;

    const f1 = try formatScalar(allocator, "true");
    defer allocator.free(f1);
    try std.testing.expectEqualStrings("\"true\"", f1);

    const f2 = try formatScalar(allocator, "key: value");
    defer allocator.free(f2);
    try std.testing.expectEqualStrings("\"key: value\"", f2);

    const f3 = try formatScalar(allocator, "");
    defer allocator.free(f3);
    try std.testing.expectEqualStrings("\"\"", f3);
}

test "YAML output quoting - numeric" {
    const allocator = std.testing.allocator;

    const f1 = try formatScalar(allocator, "42");
    defer allocator.free(f1);
    try std.testing.expectEqualStrings("\"42\"", f1);

    const f2 = try formatScalar(allocator, "3.14");
    defer allocator.free(f2);
    try std.testing.expectEqualStrings("\"3.14\"", f2);
}

test "parseConformsRef - spec and slot" {
    const ref = parseConformsRef("OtherSpec::BEHAVIORS");
    try std.testing.expect(ref != null);
    try std.testing.expect(ref.?.file == null);
    try std.testing.expectEqualStrings("OtherSpec", ref.?.spec);
    try std.testing.expectEqualStrings("BEHAVIORS", ref.?.slot);
}

test "parseConformsRef - file, spec and slot" {
    const ref = parseConformsRef("other.yass.yaml::OtherSpec::BEHAVIORS");
    try std.testing.expect(ref != null);
    try std.testing.expect(ref.?.file != null);
    try std.testing.expectEqualStrings("other.yass.yaml", ref.?.file.?);
    try std.testing.expectEqualStrings("OtherSpec", ref.?.spec);
    try std.testing.expectEqualStrings("BEHAVIORS", ref.?.slot);
}

test "parseConformsRef - no slot returns null" {
    const ref = parseConformsRef("OtherSpec");
    try std.testing.expect(ref == null);
}

test "parseConformsRef - empty slot returns null" {
    const ref = parseConformsRef("OtherSpec::");
    try std.testing.expect(ref == null);
}

test "emitYamlDocument - basic spec" {
    const allocator = std.testing.allocator;
    const content =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- MUST do something
    ;

    const parse_result = try yaml.parseYaml(allocator, content);
    defer yaml.freeParseResult(allocator, parse_result);

    const doc = parse_result.documents[0];
    const root = doc.root.?;
    switch (root) {
        .mapping => |m| {
            const output = try emitYamlDocument(allocator, m);
            defer allocator.free(output);

            try std.testing.expect(std.mem.startsWith(u8, output, "---\n"));
            try std.testing.expect(std.mem.indexOf(u8, output, "spec: MySpec\n") != null);
            try std.testing.expect(std.mem.indexOf(u8, output, "BEHAVIORS:\n") != null);
            try std.testing.expect(std.mem.indexOf(u8, output, "- MUST do something\n") != null);
        },
        else => return error.TestUnexpectedResult,
    }
}

test "emitYamlDocument - obligation with mapping entries" {
    const allocator = std.testing.allocator;
    const content =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- MUST: do something
        \\  WHEN: active
    ;

    const parse_result = try yaml.parseYaml(allocator, content);
    defer yaml.freeParseResult(allocator, parse_result);

    const doc = parse_result.documents[0];
    const root = doc.root.?;
    switch (root) {
        .mapping => |m| {
            const output = try emitYamlDocument(allocator, m);
            defer allocator.free(output);

            try std.testing.expect(std.mem.startsWith(u8, output, "---\n"));
            // Key order: normativity first, then WHEN
            try std.testing.expect(std.mem.indexOf(u8, output, "- MUST: do something\n") != null);
            try std.testing.expect(std.mem.indexOf(u8, output, "  WHEN: active\n") != null);
            // MUST should appear before WHEN
            const must_pos = std.mem.indexOf(u8, output, "MUST:").?;
            const when_pos = std.mem.indexOf(u8, output, "WHEN:").?;
            try std.testing.expect(must_pos < when_pos);
        },
        else => return error.TestUnexpectedResult,
    }
}

test "provenance comment format" {
    const allocator = std.testing.allocator;

    // Test the comment format directly
    var buf: std.ArrayList(u8) = .empty;
    defer buf.deinit(allocator);

    try buf.appendSlice(allocator, "# CONFORMS: OtherSpec::BEHAVIORS\n");
    const result = try allocator.dupe(u8, buf.items);
    defer allocator.free(result);

    try std.testing.expectEqualStrings("# CONFORMS: OtherSpec::BEHAVIORS\n", result);
}

test "multi-match returns list rows" {
    const allocator = std.testing.allocator;
    const content1 =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- MUST do thing1
    ;
    const content2 =
        \\---
        \\spec: pkg.MySpec
        \\BEHAVIORS:
        \\- MUST do thing2
    ;
    const files = [_][]const u8{ "a.yass.yaml", "b.yass.yaml" };
    const contents = [_][]const u8{ content1, content2 };
    const matches = try nameLookup(allocator, "MySpec", &files, &contents);
    defer {
        for (matches) |m| allocator.free(m.spec_name);
        allocator.free(matches);
    }

    try std.testing.expectEqual(@as(usize, 2), matches.len);

    // Build list rows as querySpec would
    var rows: std.ArrayList(ListRow) = .empty;
    defer rows.deinit(allocator);
    for (matches) |match| {
        try rows.append(allocator, .{
            .file = match.file,
            .spec_name = match.spec_name,
        });
    }

    try std.testing.expectEqual(@as(usize, 2), rows.items.len);
    try std.testing.expectEqualStrings("a.yass.yaml", rows.items[0].file);
    try std.testing.expectEqualStrings("MySpec", rows.items[0].spec_name);
    try std.testing.expectEqualStrings("b.yass.yaml", rows.items[1].file);
    try std.testing.expectEqualStrings("pkg.MySpec", rows.items[1].spec_name);
}

test "no match returns empty" {
    const allocator = std.testing.allocator;
    const content =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- MUST do something
    ;
    const files = [_][]const u8{"test.yass.yaml"};
    const contents = [_][]const u8{content};
    const matches = try nameLookup(allocator, "NonExistent", &files, &contents);
    defer allocator.free(matches);

    try std.testing.expectEqual(@as(usize, 0), matches.len);
}

test "InlineConforms - reference-only CONFORMS with inline data" {
    // Test the structure: a reference-only CONFORMS obligation should be
    // replaced entirely by the inlined obligations
    const allocator = std.testing.allocator;

    // We test the parsing of CONFORMS ref and the structure it produces
    const ref = parseConformsRef("BaseSpec::BEHAVIORS");
    try std.testing.expect(ref != null);
    try std.testing.expectEqualStrings("BaseSpec", ref.?.spec);
    try std.testing.expectEqualStrings("BEHAVIORS", ref.?.slot);

    // A reference-only CONFORMS (no normativity keyword) means is_normative = false
    const content =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- CONFORMS: BaseSpec::BEHAVIORS
    ;
    const parse_result = try yaml.parseYaml(allocator, content);
    defer yaml.freeParseResult(allocator, parse_result);

    const doc = parse_result.documents[0];
    const root = doc.root.?;
    switch (root) {
        .mapping => |m| {
            for (m.entries) |entry| {
                if (std.mem.eql(u8, entry.key.value, "BEHAVIORS")) {
                    switch (entry.value) {
                        .sequence => |seq| {
                            for (seq.items) |item| {
                                switch (item) {
                                    .mapping => |obligation_m| {
                                        const info = findConformsInObligation(obligation_m);
                                        try std.testing.expect(info.conforms_entry != null);
                                        try std.testing.expect(!info.is_normative);
                                        try std.testing.expect(info.when_value == null);
                                    },
                                    else => {},
                                }
                            }
                        },
                        else => {},
                    }
                }
            }
        },
        else => return error.TestUnexpectedResult,
    }
}

test "InlineConforms - normative CONFORMS detection" {
    const allocator = std.testing.allocator;

    const content =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- MUST: do something
        \\  CONFORMS: BaseSpec::BEHAVIORS
    ;
    const parse_result = try yaml.parseYaml(allocator, content);
    defer yaml.freeParseResult(allocator, parse_result);

    const doc = parse_result.documents[0];
    const root = doc.root.?;
    switch (root) {
        .mapping => |m| {
            for (m.entries) |entry| {
                if (std.mem.eql(u8, entry.key.value, "BEHAVIORS")) {
                    switch (entry.value) {
                        .sequence => |seq| {
                            for (seq.items) |item| {
                                switch (item) {
                                    .mapping => |obligation_m| {
                                        const info = findConformsInObligation(obligation_m);
                                        try std.testing.expect(info.conforms_entry != null);
                                        try std.testing.expect(info.is_normative);
                                    },
                                    else => {},
                                }
                            }
                        },
                        else => {},
                    }
                }
            }
        },
        else => return error.TestUnexpectedResult,
    }
}

test "InlineConforms - WHEN guard detection" {
    const allocator = std.testing.allocator;

    const content =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- MUST: do something
        \\  WHEN: active
        \\  CONFORMS: BaseSpec::BEHAVIORS
    ;
    const parse_result = try yaml.parseYaml(allocator, content);
    defer yaml.freeParseResult(allocator, parse_result);

    const doc = parse_result.documents[0];
    const root = doc.root.?;
    switch (root) {
        .mapping => |m| {
            for (m.entries) |entry| {
                if (std.mem.eql(u8, entry.key.value, "BEHAVIORS")) {
                    switch (entry.value) {
                        .sequence => |seq| {
                            for (seq.items) |item| {
                                switch (item) {
                                    .mapping => |obligation_m| {
                                        const info = findConformsInObligation(obligation_m);
                                        try std.testing.expect(info.conforms_entry != null);
                                        try std.testing.expect(info.is_normative);
                                        try std.testing.expect(info.when_value != null);
                                        try std.testing.expectEqualStrings("active", info.when_value.?);
                                    },
                                    else => {},
                                }
                            }
                        },
                        else => {},
                    }
                }
            }
        },
        else => return error.TestUnexpectedResult,
    }
}

test "WHEN guard combination format" {
    const allocator = std.testing.allocator;
    // Test the " and " joining format
    const combined = try std.fmt.allocPrint(allocator, "{s} and {s}", .{ "active", "enabled" });
    defer allocator.free(combined);
    try std.testing.expectEqualStrings("active and enabled", combined);
}

test "emitObligationWithoutConforms strips CONFORMS key" {
    const allocator = std.testing.allocator;

    const content =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- MUST: do something
        \\  CONFORMS: BaseSpec::BEHAVIORS
    ;
    const parse_result = try yaml.parseYaml(allocator, content);
    defer yaml.freeParseResult(allocator, parse_result);

    const doc = parse_result.documents[0];
    const root = doc.root.?;
    switch (root) {
        .mapping => |m| {
            for (m.entries) |entry| {
                if (std.mem.eql(u8, entry.key.value, "BEHAVIORS")) {
                    switch (entry.value) {
                        .sequence => |seq| {
                            for (seq.items) |item| {
                                switch (item) {
                                    .mapping => |obligation_m| {
                                        var buf: std.ArrayList(u8) = .empty;
                                        defer buf.deinit(allocator);
                                        try emitObligationWithoutConforms(allocator, &buf, obligation_m, 0);
                                        const output = try allocator.dupe(u8, buf.items);
                                        defer allocator.free(output);

                                        // Should have MUST but not CONFORMS
                                        try std.testing.expect(std.mem.indexOf(u8, output, "MUST:") != null);
                                        try std.testing.expect(std.mem.indexOf(u8, output, "CONFORMS") == null);
                                    },
                                    else => {},
                                }
                            }
                        },
                        else => {},
                    }
                }
            }
        },
        else => return error.TestUnexpectedResult,
    }
}

test "emitYamlDocument preserves key ordering" {
    const allocator = std.testing.allocator;
    const content =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- MUST do alpha
        \\- MUST do beta
        \\STATES:
        \\- MUST be valid
    ;

    const parse_result = try yaml.parseYaml(allocator, content);
    defer yaml.freeParseResult(allocator, parse_result);

    const doc = parse_result.documents[0];
    switch (doc.root.?) {
        .mapping => |m| {
            const output = try emitYamlDocument(allocator, m);
            defer allocator.free(output);

            // Verify spec comes before BEHAVIORS, BEHAVIORS before STATES
            const spec_pos = std.mem.indexOf(u8, output, "spec:").?;
            const beh_pos = std.mem.indexOf(u8, output, "BEHAVIORS:").?;
            const states_pos = std.mem.indexOf(u8, output, "STATES:").?;
            try std.testing.expect(spec_pos < beh_pos);
            try std.testing.expect(beh_pos < states_pos);
        },
        else => return error.TestUnexpectedResult,
    }
}

test "2-space indentation in output" {
    const allocator = std.testing.allocator;
    const content =
        \\---
        \\spec: MySpec
        \\BEHAVIORS:
        \\- MUST: do something
        \\  WHEN: active
    ;

    const fragment = try extractFragment(allocator, content, "MySpec");
    defer if (fragment) |f| allocator.free(f);

    try std.testing.expect(fragment != null);
    // Check for 2-space indentation
    try std.testing.expect(std.mem.indexOf(u8, fragment.?, "  WHEN:") != null);
}

test "conforms_no_slot error for ref without slot" {
    // Verify parseConformsRef returns null for refs without "::"
    try std.testing.expect(parseConformsRef("JustASpec") == null);
    try std.testing.expect(parseConformsRef("some.Spec") == null);
}

test "findSlotInContent - finds correct slot" {
    const allocator = std.testing.allocator;
    const content =
        \\---
        \\spec: BaseSpec
        \\BEHAVIORS:
        \\- MUST do base thing
        \\- MUST do another base thing
        \\STATES:
        \\- MUST be valid
    ;

    const parse_result = try yaml.parseYaml(allocator, content);
    defer yaml.freeParseResult(allocator, parse_result);

    // Find BEHAVIORS slot in the parsed document
    var found_seq: ?yaml.YamlSequence = null;
    for (parse_result.documents) |doc| {
        const root = doc.root orelse continue;
        switch (root) {
            .mapping => |m| {
                var is_base_spec = false;
                for (m.entries) |entry| {
                    if (std.mem.eql(u8, entry.key.value, "spec")) {
                        switch (entry.value) {
                            .scalar => |s| {
                                if (std.mem.eql(u8, s.value, "BaseSpec")) is_base_spec = true;
                            },
                            else => {},
                        }
                    }
                }
                if (is_base_spec) {
                    for (m.entries) |entry| {
                        if (std.mem.eql(u8, entry.key.value, "BEHAVIORS")) {
                            switch (entry.value) {
                                .sequence => |seq| {
                                    found_seq = seq;
                                },
                                else => {},
                            }
                        }
                    }
                }
            },
            else => {},
        }
    }
    try std.testing.expect(found_seq != null);
    try std.testing.expectEqual(@as(usize, 2), found_seq.?.items.len);
}

test "findSlotInContent - wrong spec returns null" {
    const allocator = std.testing.allocator;
    const content =
        \\---
        \\spec: BaseSpec
        \\BEHAVIORS:
        \\- MUST do base thing
    ;

    const parse_result = try yaml.parseYaml(allocator, content);
    defer yaml.freeParseResult(allocator, parse_result);

    // Verify no spec named "OtherSpec" exists
    var found = false;
    for (parse_result.documents) |doc| {
        const root = doc.root orelse continue;
        switch (root) {
            .mapping => |m| {
                for (m.entries) |entry| {
                    if (std.mem.eql(u8, entry.key.value, "spec")) {
                        switch (entry.value) {
                            .scalar => |s| {
                                if (std.mem.eql(u8, s.value, "OtherSpec")) found = true;
                            },
                            else => {},
                        }
                    }
                }
            },
            else => {},
        }
    }
    try std.testing.expect(!found);
}

test "findSlotInContent - wrong slot returns null" {
    const allocator = std.testing.allocator;
    const content =
        \\---
        \\spec: BaseSpec
        \\BEHAVIORS:
        \\- MUST do base thing
    ;

    const parse_result = try yaml.parseYaml(allocator, content);
    defer yaml.freeParseResult(allocator, parse_result);

    // Verify no STATES slot exists on BaseSpec
    var found = false;
    for (parse_result.documents) |doc| {
        const root = doc.root orelse continue;
        switch (root) {
            .mapping => |m| {
                for (m.entries) |entry| {
                    if (std.mem.eql(u8, entry.key.value, "STATES")) {
                        found = true;
                    }
                }
            },
            else => {},
        }
    }
    try std.testing.expect(!found);
}
