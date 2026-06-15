const std = @import("std");
const Allocator = std.mem.Allocator;
const c = @cImport({
    @cInclude("yaml.h");
});

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

pub const YamlScalar = struct {
    value: []const u8,
    line: usize, // 1-based
};

pub const MappingEntry = struct {
    key: YamlScalar,
    value: YamlNode,
};

pub const YamlMapping = struct {
    entries: []MappingEntry,
    line: usize, // 1-based
};

pub const YamlSequence = struct {
    items: []YamlNode,
    line: usize, // 1-based
};

pub const YamlNode = union(enum) {
    scalar: YamlScalar,
    mapping: YamlMapping,
    sequence: YamlSequence,
};

pub const YamlDocument = struct {
    root: ?YamlNode,
    line: usize, // 1-based
};

pub const ParseResult = struct {
    documents: []YamlDocument,
};

pub const ParseError = error{
    not_utf8,
    has_bom,
    empty_file,
    malformed,
    duplicate_key,
    anchor_or_alias,
};

// ---------------------------------------------------------------------------
// UTF-8 validation
// ---------------------------------------------------------------------------

pub fn isValidUtf8(bytes: []const u8) bool {
    // Use std.unicode.Utf8View which validates on init
    _ = std.unicode.Utf8View.init(bytes) catch return false;
    return true;
}

// ---------------------------------------------------------------------------
// Core parser
// ---------------------------------------------------------------------------

pub fn parseYaml(allocator: Allocator, input: []const u8) ParseError!ParseResult {
    // Pre-checks
    if (input.len == 0) return ParseError.empty_file;

    // BOM detection (UTF-8 BOM: EF BB BF)
    if (input.len >= 3 and input[0] == 0xEF and input[1] == 0xBB and input[2] == 0xBF) {
        return ParseError.has_bom;
    }

    // UTF-8 validity
    if (!isValidUtf8(input)) {
        return ParseError.not_utf8;
    }

    // Initialize libyaml parser
    var parser: c.yaml_parser_t = undefined;
    if (c.yaml_parser_initialize(&parser) == 0) {
        return ParseError.malformed;
    }
    defer c.yaml_parser_delete(&parser);

    c.yaml_parser_set_input_string(&parser, input.ptr, input.len);

    // Parse documents via event API
    var documents: std.ArrayList(YamlDocument) = .empty;
    errdefer {
        for (documents.items) |doc| {
            if (doc.root) |root| freeNode(allocator, root);
        }
        documents.deinit(allocator);
    }

    // We need to consume events in order
    // First event should be STREAM_START
    var event: c.yaml_event_t = undefined;

    // Get STREAM_START
    if (c.yaml_parser_parse(&parser, &event) == 0) {
        return ParseError.malformed;
    }
    if (event.type != c.YAML_STREAM_START_EVENT) {
        c.yaml_event_delete(&event);
        return ParseError.malformed;
    }
    c.yaml_event_delete(&event);

    // Parse documents until STREAM_END
    while (true) {
        if (c.yaml_parser_parse(&parser, &event) == 0) {
            return ParseError.malformed;
        }

        if (event.type == c.YAML_STREAM_END_EVENT) {
            c.yaml_event_delete(&event);
            break;
        }

        if (event.type == c.YAML_DOCUMENT_START_EVENT) {
            const doc_line = event.start_mark.line + 1;
            c.yaml_event_delete(&event);

            // Parse the document content
            const parse_result = parseNode(allocator, &parser) catch |err| {
                return err;
            };
            const root = parse_result.node;
            const consumed_doc_end = parse_result.consumed_doc_end;

            // Consume DOCUMENT_END event if parseNode didn't already
            if (!consumed_doc_end) {
                if (c.yaml_parser_parse(&parser, &event) == 0) {
                    return ParseError.malformed;
                }
                if (event.type != c.YAML_DOCUMENT_END_EVENT) {
                    c.yaml_event_delete(&event);
                    return ParseError.malformed;
                }
                c.yaml_event_delete(&event);
            }

            documents.append(allocator, .{
                .root = root,
                .line = doc_line,
            }) catch return ParseError.malformed;
        } else {
            c.yaml_event_delete(&event);
            return ParseError.malformed;
        }
    }

    return ParseResult{
        .documents = documents.toOwnedSlice(allocator) catch return ParseError.malformed,
    };
}

const ParseNodeResult = struct {
    node: ?YamlNode,
    consumed_doc_end: bool,
};

/// Parse a single YAML node from the event stream.
/// Returns null node if the document is empty (DOCUMENT_END follows DOCUMENT_START).
/// Also indicates whether the DOCUMENT_END event was consumed.
fn parseNode(allocator: Allocator, parser: *c.yaml_parser_t) ParseError!ParseNodeResult {
    var event: c.yaml_event_t = undefined;
    if (c.yaml_parser_parse(parser, &event) == 0) {
        return ParseError.malformed;
    }

    const event_type = event.type;

    switch (event_type) {
        c.YAML_SCALAR_EVENT => {
            // Check for anchor
            if (event.data.scalar.anchor != null) {
                c.yaml_event_delete(&event);
                return ParseError.anchor_or_alias;
            }
            // Check for non-default tag
            if (hasNonDefaultTag(event.data.scalar.tag)) {
                c.yaml_event_delete(&event);
                return ParseError.anchor_or_alias;
            }

            const line = event.start_mark.line + 1;
            const len = event.data.scalar.length;
            const raw_ptr: [*]const u8 = @ptrCast(event.data.scalar.value);
            const value_slice = raw_ptr[0..len];

            // Copy the value into our allocator
            const value = allocator.dupe(u8, value_slice) catch {
                c.yaml_event_delete(&event);
                return ParseError.malformed;
            };

            c.yaml_event_delete(&event);

            return .{
                .node = YamlNode{
                    .scalar = YamlScalar{
                        .value = value,
                        .line = line,
                    },
                },
                .consumed_doc_end = false,
            };
        },

        c.YAML_MAPPING_START_EVENT => {
            // Check for anchor
            if (event.data.mapping_start.anchor != null) {
                c.yaml_event_delete(&event);
                return ParseError.anchor_or_alias;
            }
            // Check for non-default tag
            if (hasNonDefaultTag(event.data.mapping_start.tag)) {
                c.yaml_event_delete(&event);
                return ParseError.anchor_or_alias;
            }

            const line = event.start_mark.line + 1;
            c.yaml_event_delete(&event);

            return .{
                .node = try parseMapping(allocator, parser, line),
                .consumed_doc_end = false,
            };
        },

        c.YAML_SEQUENCE_START_EVENT => {
            // Check for anchor
            if (event.data.sequence_start.anchor != null) {
                c.yaml_event_delete(&event);
                return ParseError.anchor_or_alias;
            }
            // Check for non-default tag
            if (hasNonDefaultTag(event.data.sequence_start.tag)) {
                c.yaml_event_delete(&event);
                return ParseError.anchor_or_alias;
            }

            const line = event.start_mark.line + 1;
            c.yaml_event_delete(&event);

            return .{
                .node = try parseSequence(allocator, parser, line),
                .consumed_doc_end = false,
            };
        },

        c.YAML_ALIAS_EVENT => {
            c.yaml_event_delete(&event);
            return ParseError.anchor_or_alias;
        },

        c.YAML_DOCUMENT_END_EVENT => {
            // Empty document - DOCUMENT_END consumed here
            c.yaml_event_delete(&event);
            return .{
                .node = null,
                .consumed_doc_end = true,
            };
        },

        else => {
            c.yaml_event_delete(&event);
            return ParseError.malformed;
        },
    }
}

/// Parse a mapping from events until MAPPING_END.
fn parseMapping(allocator: Allocator, parser: *c.yaml_parser_t, line: usize) ParseError!YamlNode {
    var entries: std.ArrayList(MappingEntry) = .empty;
    errdefer {
        for (entries.items) |entry| {
            allocator.free(entry.key.value);
            freeNode(allocator, entry.value);
        }
        entries.deinit(allocator);
    }
    // Track keys we've seen for duplicate detection
    var seen_keys = std.StringHashMap(void).init(allocator);
    defer seen_keys.deinit();

    while (true) {
        // Peek at next event to check for MAPPING_END
        var event: c.yaml_event_t = undefined;
        if (c.yaml_parser_parse(parser, &event) == 0) {
            return ParseError.malformed;
        }

        if (event.type == c.YAML_MAPPING_END_EVENT) {
            c.yaml_event_delete(&event);
            break;
        }

        // This event must be a scalar key
        if (event.type != c.YAML_SCALAR_EVENT) {
            c.yaml_event_delete(&event);
            return ParseError.malformed;
        }

        // Check for anchor on key
        if (event.data.scalar.anchor != null) {
            c.yaml_event_delete(&event);
            return ParseError.anchor_or_alias;
        }
        // Check for non-default tag on key
        if (hasNonDefaultTag(event.data.scalar.tag)) {
            c.yaml_event_delete(&event);
            return ParseError.anchor_or_alias;
        }

        const key_line = event.start_mark.line + 1;
        const key_len = event.data.scalar.length;
        const key_raw: [*]const u8 = @ptrCast(event.data.scalar.value);
        const key_slice = key_raw[0..key_len];

        const key_value = allocator.dupe(u8, key_slice) catch {
            c.yaml_event_delete(&event);
            return ParseError.malformed;
        };

        c.yaml_event_delete(&event);

        // Duplicate key detection
        if (seen_keys.contains(key_value)) {
            allocator.free(key_value);
            return ParseError.duplicate_key;
        }
        seen_keys.put(key_value, {}) catch {
            allocator.free(key_value);
            return ParseError.malformed;
        };

        // Parse value node
        const value_result = parseNode(allocator, parser) catch |err| {
            allocator.free(key_value);
            return err;
        };
        const value_node = value_result.node orelse {
            // Null value in mapping - treat as empty scalar
            allocator.free(key_value);
            return ParseError.malformed;
        };

        entries.append(allocator, .{
            .key = YamlScalar{
                .value = key_value,
                .line = key_line,
            },
            .value = value_node,
        }) catch {
            allocator.free(key_value);
            freeNode(allocator, value_node);
            return ParseError.malformed;
        };
    }

    return YamlNode{
        .mapping = YamlMapping{
            .entries = entries.toOwnedSlice(allocator) catch return ParseError.malformed,
            .line = line,
        },
    };
}

/// Parse a sequence from events until SEQUENCE_END.
fn parseSequence(allocator: Allocator, parser: *c.yaml_parser_t, line: usize) ParseError!YamlNode {
    var items: std.ArrayList(YamlNode) = .empty;
    errdefer {
        for (items.items) |item| {
            freeNode(allocator, item);
        }
        items.deinit(allocator);
    }

    while (true) {
        // Peek at next event
        var event: c.yaml_event_t = undefined;
        if (c.yaml_parser_parse(parser, &event) == 0) {
            return ParseError.malformed;
        }

        if (event.type == c.YAML_SEQUENCE_END_EVENT) {
            c.yaml_event_delete(&event);
            break;
        }

        // We consumed the event but it's the start of a node.
        // We need to handle it here since parseNode expects to read the event itself.
        // Re-route based on event type.
        const node = try handleEventAsNode(allocator, parser, &event);
        items.append(allocator, node) catch return ParseError.malformed;
    }

    return YamlNode{
        .sequence = YamlSequence{
            .items = items.toOwnedSlice(allocator) catch return ParseError.malformed,
            .line = line,
        },
    };
}

/// Handle an already-consumed event as a node parse.
fn handleEventAsNode(allocator: Allocator, parser: *c.yaml_parser_t, event: *c.yaml_event_t) ParseError!YamlNode {
    switch (event.type) {
        c.YAML_SCALAR_EVENT => {
            if (event.data.scalar.anchor != null) {
                c.yaml_event_delete(event);
                return ParseError.anchor_or_alias;
            }
            if (hasNonDefaultTag(event.data.scalar.tag)) {
                c.yaml_event_delete(event);
                return ParseError.anchor_or_alias;
            }

            const line = event.start_mark.line + 1;
            const len = event.data.scalar.length;
            const raw_ptr: [*]const u8 = @ptrCast(event.data.scalar.value);
            const value_slice = raw_ptr[0..len];

            const value = allocator.dupe(u8, value_slice) catch {
                c.yaml_event_delete(event);
                return ParseError.malformed;
            };

            c.yaml_event_delete(event);

            return YamlNode{
                .scalar = YamlScalar{
                    .value = value,
                    .line = line,
                },
            };
        },

        c.YAML_MAPPING_START_EVENT => {
            if (event.data.mapping_start.anchor != null) {
                c.yaml_event_delete(event);
                return ParseError.anchor_or_alias;
            }
            if (hasNonDefaultTag(event.data.mapping_start.tag)) {
                c.yaml_event_delete(event);
                return ParseError.anchor_or_alias;
            }

            const line = event.start_mark.line + 1;
            c.yaml_event_delete(event);

            return parseMapping(allocator, parser, line);
        },

        c.YAML_SEQUENCE_START_EVENT => {
            if (event.data.sequence_start.anchor != null) {
                c.yaml_event_delete(event);
                return ParseError.anchor_or_alias;
            }
            if (hasNonDefaultTag(event.data.sequence_start.tag)) {
                c.yaml_event_delete(event);
                return ParseError.anchor_or_alias;
            }

            const line = event.start_mark.line + 1;
            c.yaml_event_delete(event);

            return parseSequence(allocator, parser, line);
        },

        c.YAML_ALIAS_EVENT => {
            c.yaml_event_delete(event);
            return ParseError.anchor_or_alias;
        },

        else => {
            c.yaml_event_delete(event);
            return ParseError.malformed;
        },
    }
}

/// Check if a tag is non-default (not null and not one of the standard YAML tags).
fn hasNonDefaultTag(tag: ?[*]const u8) bool {
    const tag_ptr = tag orelse return false;
    const tag_str = std.mem.span(@as([*:0]const u8, @ptrCast(tag_ptr)));

    // Default tags that libyaml auto-assigns are fine
    const default_tags = [_][]const u8{
        "tag:yaml.org,2002:str",
        "tag:yaml.org,2002:null",
        "tag:yaml.org,2002:bool",
        "tag:yaml.org,2002:int",
        "tag:yaml.org,2002:float",
        "tag:yaml.org,2002:seq",
        "tag:yaml.org,2002:map",
        "tag:yaml.org,2002:timestamp",
    };

    for (default_tags) |dt| {
        if (std.mem.eql(u8, tag_str, dt)) return false;
    }

    return true;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test "parse simple key-value mapping" {
    const allocator = std.testing.allocator;
    const input = "name: yass\nversion: 1.0\n";
    const result = try parseYaml(allocator, input);
    defer allocator.free(result.documents);

    try std.testing.expectEqual(@as(usize, 1), result.documents.len);
    const doc = result.documents[0];
    try std.testing.expect(doc.root != null);

    const root = doc.root.?;
    switch (root) {
        .mapping => |m| {
            try std.testing.expectEqual(@as(usize, 2), m.entries.len);
            try std.testing.expectEqualStrings("name", m.entries[0].key.value);
            switch (m.entries[0].value) {
                .scalar => |s| try std.testing.expectEqualStrings("yass", s.value),
                else => return error.TestUnexpectedResult,
            }
            try std.testing.expectEqualStrings("version", m.entries[1].key.value);
            switch (m.entries[1].value) {
                .scalar => |s| try std.testing.expectEqualStrings("1.0", s.value),
                else => return error.TestUnexpectedResult,
            }
            // Free mapping entries and their owned strings
            for (m.entries) |entry| {
                allocator.free(entry.key.value);
                switch (entry.value) {
                    .scalar => |s| allocator.free(s.value),
                    else => {},
                }
            }
            allocator.free(m.entries);
        },
        else => return error.TestUnexpectedResult,
    }
}

test "parse multi-document stream" {
    const allocator = std.testing.allocator;
    const input = "---\nfoo: bar\n---\nbaz: qux\n";
    const result = try parseYaml(allocator, input);
    defer allocator.free(result.documents);

    try std.testing.expectEqual(@as(usize, 2), result.documents.len);

    // First document
    const doc1 = result.documents[0];
    try std.testing.expect(doc1.root != null);
    switch (doc1.root.?) {
        .mapping => |m| {
            try std.testing.expectEqual(@as(usize, 1), m.entries.len);
            try std.testing.expectEqualStrings("foo", m.entries[0].key.value);
            switch (m.entries[0].value) {
                .scalar => |s| {
                    try std.testing.expectEqualStrings("bar", s.value);
                    allocator.free(s.value);
                },
                else => return error.TestUnexpectedResult,
            }
            allocator.free(m.entries[0].key.value);
            allocator.free(m.entries);
        },
        else => return error.TestUnexpectedResult,
    }

    // Second document
    const doc2 = result.documents[1];
    try std.testing.expect(doc2.root != null);
    switch (doc2.root.?) {
        .mapping => |m| {
            try std.testing.expectEqual(@as(usize, 1), m.entries.len);
            try std.testing.expectEqualStrings("baz", m.entries[0].key.value);
            switch (m.entries[0].value) {
                .scalar => |s| {
                    try std.testing.expectEqualStrings("qux", s.value);
                    allocator.free(s.value);
                },
                else => return error.TestUnexpectedResult,
            }
            allocator.free(m.entries[0].key.value);
            allocator.free(m.entries);
        },
        else => return error.TestUnexpectedResult,
    }
}

test "parse sequences" {
    const allocator = std.testing.allocator;
    const input = "items:\n  - one\n  - two\n  - three\n";
    const result = try parseYaml(allocator, input);
    defer allocator.free(result.documents);

    try std.testing.expectEqual(@as(usize, 1), result.documents.len);
    const root = result.documents[0].root.?;

    switch (root) {
        .mapping => |m| {
            defer {
                for (m.entries) |entry| {
                    allocator.free(entry.key.value);
                    switch (entry.value) {
                        .sequence => |seq| {
                            for (seq.items) |item| {
                                switch (item) {
                                    .scalar => |s| allocator.free(s.value),
                                    else => {},
                                }
                            }
                            allocator.free(seq.items);
                        },
                        .scalar => |s| allocator.free(s.value),
                        else => {},
                    }
                }
                allocator.free(m.entries);
            }

            try std.testing.expectEqual(@as(usize, 1), m.entries.len);
            try std.testing.expectEqualStrings("items", m.entries[0].key.value);

            switch (m.entries[0].value) {
                .sequence => |seq| {
                    try std.testing.expectEqual(@as(usize, 3), seq.items.len);
                    switch (seq.items[0]) {
                        .scalar => |s| try std.testing.expectEqualStrings("one", s.value),
                        else => return error.TestUnexpectedResult,
                    }
                    switch (seq.items[1]) {
                        .scalar => |s| try std.testing.expectEqualStrings("two", s.value),
                        else => return error.TestUnexpectedResult,
                    }
                    switch (seq.items[2]) {
                        .scalar => |s| try std.testing.expectEqualStrings("three", s.value),
                        else => return error.TestUnexpectedResult,
                    }
                },
                else => return error.TestUnexpectedResult,
            }
        },
        else => return error.TestUnexpectedResult,
    }
}

test "parse nested structures" {
    const allocator = std.testing.allocator;
    const input =
        \\server:
        \\  host: localhost
        \\  port: 8080
        \\
    ;
    const result = try parseYaml(allocator, input);
    defer allocator.free(result.documents);

    try std.testing.expectEqual(@as(usize, 1), result.documents.len);
    const root = result.documents[0].root.?;

    switch (root) {
        .mapping => |m| {
            defer freeMapping(allocator, m);
            try std.testing.expectEqual(@as(usize, 1), m.entries.len);
            try std.testing.expectEqualStrings("server", m.entries[0].key.value);

            switch (m.entries[0].value) {
                .mapping => |inner| {
                    try std.testing.expectEqual(@as(usize, 2), inner.entries.len);
                    try std.testing.expectEqualStrings("host", inner.entries[0].key.value);
                    switch (inner.entries[0].value) {
                        .scalar => |s| try std.testing.expectEqualStrings("localhost", s.value),
                        else => return error.TestUnexpectedResult,
                    }
                    try std.testing.expectEqualStrings("port", inner.entries[1].key.value);
                    switch (inner.entries[1].value) {
                        .scalar => |s| try std.testing.expectEqualStrings("8080", s.value),
                        else => return error.TestUnexpectedResult,
                    }
                },
                else => return error.TestUnexpectedResult,
            }
        },
        else => return error.TestUnexpectedResult,
    }
}

test "detect BOM" {
    const allocator = std.testing.allocator;
    const input = "\xEF\xBB\xBFname: test\n";
    const result = parseYaml(allocator, input);
    try std.testing.expectError(ParseError.has_bom, result);
}

test "detect empty file" {
    const allocator = std.testing.allocator;
    const input = "";
    const result = parseYaml(allocator, input);
    try std.testing.expectError(ParseError.empty_file, result);
}

test "detect duplicate keys" {
    const allocator = std.testing.allocator;
    const input = "name: foo\nname: bar\n";
    const result = parseYaml(allocator, input);
    try std.testing.expectError(ParseError.duplicate_key, result);
}

test "detect anchors" {
    const allocator = std.testing.allocator;
    const input = "defaults: &defaults\n  host: localhost\n";
    const result = parseYaml(allocator, input);
    try std.testing.expectError(ParseError.anchor_or_alias, result);
}

test "detect aliases" {
    const allocator = std.testing.allocator;
    // libyaml needs the anchor defined first to parse the alias
    const input = "defaults: &defaults\n  host: localhost\nserver:\n  <<: *defaults\n";
    const result = parseYaml(allocator, input);
    try std.testing.expectError(ParseError.anchor_or_alias, result);
}

test "detect non-default tags" {
    const allocator = std.testing.allocator;
    const input = "value: !custom tagged\n";
    const result = parseYaml(allocator, input);
    try std.testing.expectError(ParseError.anchor_or_alias, result);
}

test "verify line numbers" {
    const allocator = std.testing.allocator;
    const input = "first: a\nsecond: b\nthird: c\n";
    const result = try parseYaml(allocator, input);
    defer allocator.free(result.documents);

    const root = result.documents[0].root.?;
    switch (root) {
        .mapping => |m| {
            defer freeMapping(allocator, m);

            // First key should be on line 1
            try std.testing.expectEqual(@as(usize, 1), m.entries[0].key.line);
            // Second key should be on line 2
            try std.testing.expectEqual(@as(usize, 2), m.entries[1].key.line);
            // Third key should be on line 3
            try std.testing.expectEqual(@as(usize, 3), m.entries[2].key.line);
        },
        else => return error.TestUnexpectedResult,
    }
}

test "parse yass format" {
    const allocator = std.testing.allocator;
    const input =
        \\spec: my-service
        \\slots:
        \\  - name: port
        \\    type: integer
        \\  - name: host
        \\    type: string
        \\obligations:
        \\  - MUST expose port
        \\  - MUST resolve host
        \\
    ;
    const result = try parseYaml(allocator, input);
    defer allocator.free(result.documents);

    try std.testing.expectEqual(@as(usize, 1), result.documents.len);
    const root = result.documents[0].root.?;

    switch (root) {
        .mapping => |m| {
            defer freeMapping(allocator, m);

            try std.testing.expectEqual(@as(usize, 3), m.entries.len);
            try std.testing.expectEqualStrings("spec", m.entries[0].key.value);
            try std.testing.expectEqualStrings("slots", m.entries[1].key.value);
            try std.testing.expectEqualStrings("obligations", m.entries[2].key.value);

            // Check spec value
            switch (m.entries[0].value) {
                .scalar => |s| try std.testing.expectEqualStrings("my-service", s.value),
                else => return error.TestUnexpectedResult,
            }

            // Check slots is a sequence
            switch (m.entries[1].value) {
                .sequence => |seq| {
                    try std.testing.expectEqual(@as(usize, 2), seq.items.len);
                    // First slot entry should be a mapping with name and type
                    switch (seq.items[0]) {
                        .mapping => |slot_m| {
                            try std.testing.expectEqual(@as(usize, 2), slot_m.entries.len);
                            try std.testing.expectEqualStrings("name", slot_m.entries[0].key.value);
                            switch (slot_m.entries[0].value) {
                                .scalar => |s| try std.testing.expectEqualStrings("port", s.value),
                                else => return error.TestUnexpectedResult,
                            }
                        },
                        else => return error.TestUnexpectedResult,
                    }
                },
                else => return error.TestUnexpectedResult,
            }

            // Check obligations is a sequence
            switch (m.entries[2].value) {
                .sequence => |seq| {
                    try std.testing.expectEqual(@as(usize, 2), seq.items.len);
                    switch (seq.items[0]) {
                        .scalar => |s| try std.testing.expectEqualStrings("MUST expose port", s.value),
                        else => return error.TestUnexpectedResult,
                    }
                },
                else => return error.TestUnexpectedResult,
            }
        },
        else => return error.TestUnexpectedResult,
    }
}

test "yes/no/on/off treated as strings" {
    const allocator = std.testing.allocator;
    const input = "a: yes\nb: no\nc: on\nd: off\n";
    const result = try parseYaml(allocator, input);
    defer allocator.free(result.documents);

    const root = result.documents[0].root.?;
    switch (root) {
        .mapping => |m| {
            defer freeMapping(allocator, m);

            // libyaml in YAML 1.1 mode may convert these to true/false,
            // but our parser treats them as plain strings since we just
            // take the scalar value as-is from libyaml events.
            // In event mode, libyaml preserves the original text.
            try std.testing.expectEqual(@as(usize, 4), m.entries.len);
            switch (m.entries[0].value) {
                .scalar => |s| try std.testing.expectEqualStrings("yes", s.value),
                else => return error.TestUnexpectedResult,
            }
            switch (m.entries[1].value) {
                .scalar => |s| try std.testing.expectEqualStrings("no", s.value),
                else => return error.TestUnexpectedResult,
            }
            switch (m.entries[2].value) {
                .scalar => |s| try std.testing.expectEqualStrings("on", s.value),
                else => return error.TestUnexpectedResult,
            }
            switch (m.entries[3].value) {
                .scalar => |s| try std.testing.expectEqualStrings("off", s.value),
                else => return error.TestUnexpectedResult,
            }
        },
        else => return error.TestUnexpectedResult,
    }
}

test "UTF-8 validation" {
    try std.testing.expect(isValidUtf8("hello world"));
    try std.testing.expect(isValidUtf8("unicode: \xc3\xa9\xc3\xa0\xc3\xbc")); // e-acute, a-grave, u-umlaut
    try std.testing.expect(!isValidUtf8("\xff\xfe")); // invalid UTF-8
    try std.testing.expect(!isValidUtf8("\xc0\xaf")); // overlong encoding
}

test "parse empty document in stream" {
    const allocator = std.testing.allocator;
    // Two documents: first is empty (implicit null scalar), second has content
    const input = "---\n...\n---\nfoo: bar\n";
    const result = try parseYaml(allocator, input);
    defer allocator.free(result.documents);

    try std.testing.expectEqual(@as(usize, 2), result.documents.len);

    // First document: libyaml produces an implicit empty scalar for ---\n...
    if (result.documents[0].root) |root| {
        freeNode(allocator, root);
    }

    // Second document should have content
    try std.testing.expect(result.documents[1].root != null);
    switch (result.documents[1].root.?) {
        .mapping => |m| {
            defer freeMapping(allocator, m);
            try std.testing.expectEqualStrings("foo", m.entries[0].key.value);
        },
        else => return error.TestUnexpectedResult,
    }
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

/// Free all memory owned by a ParseResult (documents and their node trees).
pub fn freeParseResult(allocator: Allocator, result: ParseResult) void {
    for (result.documents) |doc| {
        if (doc.root) |root| freeNode(allocator, root);
    }
    allocator.free(result.documents);
}

fn freeMapping(allocator: Allocator, m: YamlMapping) void {
    for (m.entries) |entry| {
        allocator.free(entry.key.value);
        freeNode(allocator, entry.value);
    }
    allocator.free(m.entries);
}

pub fn freeNode(allocator: Allocator, node: YamlNode) void {
    switch (node) {
        .scalar => |s| allocator.free(s.value),
        .mapping => |m| freeMapping(allocator, m),
        .sequence => |seq| {
            for (seq.items) |item| {
                freeNode(allocator, item);
            }
            allocator.free(seq.items);
        },
    }
}
