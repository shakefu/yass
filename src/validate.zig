const std = @import("std");
const Allocator = std.mem.Allocator;
const Io = std.Io;
const Dir = std.Io.Dir;
const File = std.Io.File;

const err = @import("errors.zig");
const yaml = @import("yaml_parse.zig");
const shared = @import("shared.zig");

// ============================================================================
// Public types
// ============================================================================

pub const ValidationError = struct {
    file: []const u8,
    line: ?usize,
    code: err.ErrorCode,
    message: []const u8,
};

pub const ValidationResult = struct {
    errors: []ValidationError,
    file_count: usize,
    error_count: usize,
};

// ============================================================================
// Constants
// ============================================================================

const slot_keys = [_][]const u8{ "INPUT", "RETURN", "ERROR", "SIDE-EFFECT", "INVARIANT" };
const normativity_keywords = [_][]const u8{ "MUST", "MUST-NOT", "SHOULD", "SHOULD-NOT", "MAY" };
const reference_relations = [_][]const u8{ "CONFORMS", "USES", "SEE" };

/// Reserved names: slots + normativity + reference relations + WHEN
const reserved_names = [_][]const u8{
    "INPUT", "RETURN", "ERROR", "SIDE-EFFECT", "INVARIANT",
    "MUST",  "MUST-NOT", "SHOULD", "SHOULD-NOT", "MAY",
    "WHEN",  "CONFORMS", "USES",   "SEE",
};

// ============================================================================
// Entry point
// ============================================================================

pub fn validate(
    allocator: Allocator,
    io: Io,
    files: []const []const u8,
    project_root: ?[]const u8,
    cwd: []const u8,
) !ValidationResult {
    _ = cwd;
    var all_errors: std.ArrayList(ValidationError) = .empty;

    for (files) |file_path| {
        // Read file contents
        const contents = readFileContents(allocator, io, file_path) orelse {
            // Could not read file at all - treat as unreadable
            try all_errors.append(allocator, .{
                .file = file_path,
                .line = null,
                .code = .yaml_not_utf8,
                .message = try std.fmt.allocPrint(allocator, "file is not valid UTF-8", .{}),
            });
            continue;
        };
        defer allocator.free(contents);

        // Step 1: CheckYAML
        const parse_result = checkYaml(allocator, file_path, contents, &all_errors) catch {
            continue; // YAML failed, skip remaining checks
        };
        defer yaml.freeParseResult(allocator, parse_result);

        // Step 2: CheckPreamble
        checkPreamble(allocator, file_path, parse_result, &all_errors) catch {};

        // Step 3: CheckSpec (per non-preamble document)
        checkSpec(allocator, file_path, parse_result, &all_errors) catch {};

        // Step 4: CheckUniqueness
        checkUniqueness(allocator, file_path, parse_result, &all_errors) catch {};

        // Step 5: CheckRefs
        checkRefs(allocator, io, file_path, parse_result, project_root, &all_errors) catch {};
    }

    const error_count = all_errors.items.len;
    return .{
        .errors = try all_errors.toOwnedSlice(allocator),
        .file_count = files.len,
        .error_count = error_count,
    };
}

// ============================================================================
// File reading helper
// ============================================================================

fn readFileContents(allocator: Allocator, io: Io, file_path: []const u8) ?[]const u8 {
    // Use cwd-relative Dir for reading files (paths may be relative)
    const dir = Dir.cwd();
    return dir.readFileAlloc(io, file_path, allocator, @enumFromInt(10 * 1024 * 1024)) catch return null;
}

// ============================================================================
// CheckYAML
// ============================================================================

/// Attempts to parse YAML. On success, returns ParseResult.
/// On failure, appends one error and returns error.YamlCheckFailed.
pub fn checkYaml(
    allocator: Allocator,
    file_path: []const u8,
    contents: []const u8,
    all_errors: *std.ArrayList(ValidationError),
) !yaml.ParseResult {
    const result = yaml.parseYaml(allocator, contents) catch |e| {
        const code: err.ErrorCode = switch (e) {
            error.not_utf8 => .yaml_not_utf8,
            error.has_bom => .yaml_has_bom,
            error.empty_file => .yaml_empty_file,
            error.malformed => .yaml_malformed,
            error.duplicate_key => .yaml_duplicate_key,
            error.anchor_or_alias => .yaml_anchor_or_alias,
        };
        const message: []const u8 = switch (e) {
            error.not_utf8 => "file is not valid UTF-8",
            error.has_bom => "file starts with a byte-order mark (BOM)",
            error.empty_file => "file is empty",
            error.malformed => "YAML syntax error",
            error.duplicate_key => "duplicate mapping key",
            error.anchor_or_alias => "anchors, aliases, and custom tags are not allowed",
        };
        try all_errors.append(allocator, .{
            .file = file_path,
            .line = null,
            .code = code,
            .message = try allocator.dupe(u8, message),
        });
        return error.YamlCheckFailed;
    };
    return result;
}

// ============================================================================
// CheckPreamble
// ============================================================================

pub fn checkPreamble(
    allocator: Allocator,
    file_path: []const u8,
    parse_result: yaml.ParseResult,
    all_errors: *std.ArrayList(ValidationError),
) !void {
    const docs = parse_result.documents;

    // Must have at least one document
    if (docs.len == 0) {
        try all_errors.append(allocator, .{
            .file = file_path,
            .line = null,
            .code = .yaml_empty_stream,
            .message = try allocator.dupe(u8, "YAML stream contains no documents"),
        });
        return;
    }

    // First document must exist and be a mapping (the preamble)
    const first_doc = docs[0];
    const first_root = first_doc.root orelse {
        // Empty first doc means preamble is missing
        try all_errors.append(allocator, .{
            .file = file_path,
            .line = first_doc.line,
            .code = .preamble_missing,
            .message = try allocator.dupe(u8, "first document must be a preamble (no \"spec\" key)"),
        });
        return;
    };

    // Preamble must be a mapping
    const first_mapping = switch (first_root) {
        .mapping => |m| m,
        else => {
            try all_errors.append(allocator, .{
                .file = file_path,
                .line = first_doc.line,
                .code = .preamble_missing,
                .message = try allocator.dupe(u8, "first document must be a preamble mapping"),
            });
            return;
        },
    };

    // Preamble must NOT have "spec" key
    if (mappingHasKey(first_mapping, "spec")) {
        try all_errors.append(allocator, .{
            .file = file_path,
            .line = first_doc.line,
            .code = .preamble_has_spec_key,
            .message = try allocator.dupe(u8, "preamble must not contain a \"spec\" key"),
        });
        return;
    }

    // Check for duplicate and misplaced preambles (docs without "spec" after index 0)
    for (docs[1..]) |doc| {
        const root = doc.root orelse continue;
        switch (root) {
            .mapping => |m| {
                if (!mappingHasKey(m, "spec")) {
                    try all_errors.append(allocator, .{
                        .file = file_path,
                        .line = doc.line,
                        .code = .preamble_duplicate,
                        .message = try allocator.dupe(u8, "only one preamble document is allowed"),
                    });
                    return;
                }
            },
            else => {
                try all_errors.append(allocator, .{
                    .file = file_path,
                    .line = doc.line,
                    .code = .preamble_misplaced,
                    .message = try allocator.dupe(u8, "preamble-like document found after first position"),
                });
                return;
            },
        }
    }

    // Check required fields: version, description
    const has_version = mappingHasKey(first_mapping, "version");
    const has_description = mappingHasKey(first_mapping, "description");

    if (!has_description) {
        try all_errors.append(allocator, .{
            .file = file_path,
            .line = first_doc.line,
            .code = .preamble_missing_description,
            .message = try allocator.dupe(u8, "preamble must have a \"description\" field"),
        });
        return;
    }

    if (!has_version) {
        try all_errors.append(allocator, .{
            .file = file_path,
            .line = first_doc.line,
            .code = .preamble_missing_version,
            .message = try allocator.dupe(u8, "preamble must have a \"version\" field"),
        });
        return;
    }

    // Version must be "v1"
    const version_value = getMappingValue(first_mapping, "version");
    if (version_value) |v| {
        switch (v) {
            .scalar => |s| {
                if (!std.mem.eql(u8, s.value, "v1")) {
                    try all_errors.append(allocator, .{
                        .file = file_path,
                        .line = s.line,
                        .code = .preamble_unknown_version,
                        .message = try std.fmt.allocPrint(allocator, "unsupported version \"{s}\"; expected \"v1\"", .{s.value}),
                    });
                    return;
                }
            },
            else => {
                try all_errors.append(allocator, .{
                    .file = file_path,
                    .line = first_doc.line,
                    .code = .preamble_unknown_version,
                    .message = try allocator.dupe(u8, "version must be a string"),
                });
                return;
            },
        }
    }

    // Check "related" field if present
    const related_value = getMappingValue(first_mapping, "related");
    if (related_value) |v| {
        switch (v) {
            .sequence => |seq| {
                // Each item must be a scalar (string)
                for (seq.items) |item| {
                    switch (item) {
                        .scalar => {},
                        else => {
                            try all_errors.append(allocator, .{
                                .file = file_path,
                                .line = seq.line,
                                .code = .preamble_bad_related,
                                .message = try allocator.dupe(u8, "\"related\" must be a sequence of strings"),
                            });
                            return;
                        },
                    }
                }
            },
            else => {
                try all_errors.append(allocator, .{
                    .file = file_path,
                    .line = first_doc.line,
                    .code = .preamble_bad_related,
                    .message = try allocator.dupe(u8, "\"related\" must be a sequence of strings"),
                });
                return;
            },
        }
    }
}

// ============================================================================
// CheckSpec
// ============================================================================

pub fn checkSpec(
    allocator: Allocator,
    file_path: []const u8,
    parse_result: yaml.ParseResult,
    all_errors: *std.ArrayList(ValidationError),
) !void {
    const docs = parse_result.documents;
    if (docs.len < 2) return; // No spec documents

    for (docs[1..]) |doc| {
        const root = doc.root orelse continue;

        const mapping = switch (root) {
            .mapping => |m| m,
            else => {
                // Non-mapping spec doc
                try all_errors.append(allocator, .{
                    .file = file_path,
                    .line = doc.line,
                    .code = .spec_no_name,
                    .message = try allocator.dupe(u8, "spec document must be a mapping with a \"spec\" key"),
                });
                continue;
            },
        };

        // Must have "spec" key
        if (!mappingHasKey(mapping, "spec")) {
            // This is actually handled by checkPreamble as duplicate preamble,
            // but if checkPreamble didn't catch it, report as no_name
            try all_errors.append(allocator, .{
                .file = file_path,
                .line = doc.line,
                .code = .spec_no_name,
                .message = try allocator.dupe(u8, "spec document must have a \"spec\" key"),
            });
            continue;
        }

        // Validate spec name
        const spec_value = getMappingValue(mapping, "spec") orelse continue;
        const spec_scalar = switch (spec_value) {
            .scalar => |s| s,
            else => {
                try all_errors.append(allocator, .{
                    .file = file_path,
                    .line = doc.line,
                    .code = .spec_name_not_string,
                    .message = try allocator.dupe(u8, "spec name must be a string"),
                });
                continue;
            },
        };

        // Name must not be empty
        if (spec_scalar.value.len == 0) {
            try all_errors.append(allocator, .{
                .file = file_path,
                .line = spec_scalar.line,
                .code = .spec_name_empty,
                .message = try allocator.dupe(u8, "spec name must not be empty"),
            });
            continue;
        }

        // Name chars must be [A-Za-z0-9._-]
        if (!isValidSpecNameChars(spec_scalar.value)) {
            try all_errors.append(allocator, .{
                .file = file_path,
                .line = spec_scalar.line,
                .code = .spec_name_bad_chars,
                .message = try std.fmt.allocPrint(allocator, "spec name \"{s}\" contains invalid characters; allowed: [A-Za-z0-9._-]", .{spec_scalar.value}),
            });
            continue;
        }

        // Name must match form ^[A-Za-z0-9_-]+(.[A-Za-z0-9_-]+)*$
        if (!isValidSpecNameForm(spec_scalar.value)) {
            try all_errors.append(allocator, .{
                .file = file_path,
                .line = spec_scalar.line,
                .code = .spec_name_bad_form,
                .message = try std.fmt.allocPrint(allocator, "spec name \"{s}\" has invalid form; must match ^[A-Za-z0-9_-]+(\\.[A-Za-z0-9_-]+)*$", .{spec_scalar.value}),
            });
            continue;
        }

        // Name must not be reserved (case-insensitive)
        if (isReservedName(spec_scalar.value)) {
            try all_errors.append(allocator, .{
                .file = file_path,
                .line = spec_scalar.line,
                .code = .spec_name_reserved,
                .message = try std.fmt.allocPrint(allocator, "spec name \"{s}\" is a reserved keyword", .{spec_scalar.value}),
            });
            continue;
        }

        // Validate other keys in the mapping
        for (mapping.entries) |entry| {
            if (std.mem.eql(u8, entry.key.value, "spec")) continue;

            // Must be a valid slot key
            if (!isSlotKey(entry.key.value)) {
                try all_errors.append(allocator, .{
                    .file = file_path,
                    .line = entry.key.line,
                    .code = .spec_unknown_key,
                    .message = try std.fmt.allocPrint(allocator, "unknown key \"{s}\"; expected one of: INPUT, RETURN, ERROR, SIDE-EFFECT, INVARIANT", .{entry.key.value}),
                });
                continue;
            }

            // Slot value must be a sequence
            switch (entry.value) {
                .sequence => |seq| {
                    // Validate each obligation in the slot
                    for (seq.items) |item| {
                        try checkObligation(allocator, file_path, item, all_errors);
                    }
                },
                else => {
                    try all_errors.append(allocator, .{
                        .file = file_path,
                        .line = entry.key.line,
                        .code = .slot_value_not_list,
                        .message = try std.fmt.allocPrint(allocator, "slot \"{s}\" value must be a sequence", .{entry.key.value}),
                    });
                },
            }
        }
    }
}

fn checkObligation(
    allocator: Allocator,
    file_path: []const u8,
    node: yaml.YamlNode,
    all_errors: *std.ArrayList(ValidationError),
) !void {
    // Obligation must be a mapping
    const mapping = switch (node) {
        .mapping => |m| m,
        else => {
            // Scalars in sequences are allowed as simple text obligations.
            // The spec says obligations must be mappings - but non-mapping items
            // just get skipped here.
            return;
        },
    };

    var has_normativity = false;
    var has_reference = false;
    var has_when = false;
    var seen_normativity = std.StringHashMap(void).init(allocator);
    defer seen_normativity.deinit();
    var seen_reference = std.StringHashMap(void).init(allocator);
    defer seen_reference.deinit();

    for (mapping.entries) |entry| {
        const key = entry.key.value;

        // Check for bad value shape: normativity or WHEN key with mapping/sequence/null value
        if (isNormativityKeyword(key) or std.mem.eql(u8, key, "WHEN")) {
            switch (entry.value) {
                .mapping => {
                    try all_errors.append(allocator, .{
                        .file = file_path,
                        .line = entry.key.line,
                        .code = .obligation_bad_value_shape,
                        .message = try std.fmt.allocPrint(allocator, "value of \"{s}\" must not be a mapping", .{key}),
                    });
                    continue;
                },
                .sequence => {
                    try all_errors.append(allocator, .{
                        .file = file_path,
                        .line = entry.key.line,
                        .code = .obligation_bad_value_shape,
                        .message = try std.fmt.allocPrint(allocator, "value of \"{s}\" must not be a sequence", .{key}),
                    });
                    continue;
                },
                .scalar => |s| {
                    if (s.value.len == 0) {
                        try all_errors.append(allocator, .{
                            .file = file_path,
                            .line = entry.key.line,
                            .code = .obligation_bad_value_shape,
                            .message = try std.fmt.allocPrint(allocator, "value of \"{s}\" must not be null/empty", .{key}),
                        });
                        continue;
                    }
                },
            }
        }

        // Also check reference relation values for bad shape
        if (isReferenceRelation(key)) {
            switch (entry.value) {
                .mapping => {
                    try all_errors.append(allocator, .{
                        .file = file_path,
                        .line = entry.key.line,
                        .code = .obligation_bad_value_shape,
                        .message = try std.fmt.allocPrint(allocator, "value of \"{s}\" must not be a mapping", .{key}),
                    });
                    continue;
                },
                .sequence => {
                    try all_errors.append(allocator, .{
                        .file = file_path,
                        .line = entry.key.line,
                        .code = .obligation_bad_value_shape,
                        .message = try std.fmt.allocPrint(allocator, "value of \"{s}\" must not be a sequence", .{key}),
                    });
                    continue;
                },
                .scalar => |s| {
                    if (s.value.len == 0) {
                        try all_errors.append(allocator, .{
                            .file = file_path,
                            .line = entry.key.line,
                            .code = .obligation_bad_value_shape,
                            .message = try std.fmt.allocPrint(allocator, "value of \"{s}\" must not be null/empty", .{key}),
                        });
                        continue;
                    }
                },
            }
        }

        if (isNormativityKeyword(key)) {
            has_normativity = true;
            // Check for duplicate normativity: more than one normativity keyword
            if (seen_normativity.count() > 0) {
                try all_errors.append(allocator, .{
                    .file = file_path,
                    .line = entry.key.line,
                    .code = .obligation_duplicate_normativity,
                    .message = try allocator.dupe(u8, "duplicate Normativity keyword in obligation"),
                });
            }
            seen_normativity.put(key, {}) catch {};
        } else if (isReferenceRelation(key)) {
            has_reference = true;
            // Check for duplicate reference
            if (seen_reference.contains(key)) {
                try all_errors.append(allocator, .{
                    .file = file_path,
                    .line = entry.key.line,
                    .code = .obligation_duplicate_reference,
                    .message = try std.fmt.allocPrint(allocator, "duplicate reference relation \"{s}\"", .{key}),
                });
            } else {
                seen_reference.put(key, {}) catch {};
            }
        } else if (std.mem.eql(u8, key, "WHEN")) {
            has_when = true;
        } else {
            // Check if it looks like a normativity keyword but is unknown
            if (looksLikeNormativityOrRef(key)) {
                if (couldBeNormativity(key)) {
                    try all_errors.append(allocator, .{
                        .file = file_path,
                        .line = entry.key.line,
                        .code = .normativity_unknown,
                        .message = try std.fmt.allocPrint(allocator, "unknown normativity keyword \"{s}\"", .{key}),
                    });
                } else {
                    try all_errors.append(allocator, .{
                        .file = file_path,
                        .line = entry.key.line,
                        .code = .reference_unknown_relation,
                        .message = try std.fmt.allocPrint(allocator, "unknown reference relation \"{s}\"", .{key}),
                    });
                }
            }
        }
    }

    // WHEN without normativity
    if (has_when and !has_normativity) {
        try all_errors.append(allocator, .{
            .file = file_path,
            .line = mapping.line,
            .code = .obligation_guard_without_normativity,
            .message = try allocator.dupe(u8, "WHEN guard requires a normativity keyword (MUST, SHOULD, etc.)"),
        });
    }

    // Must have normativity or reference
    if (!has_normativity and !has_reference) {
        try all_errors.append(allocator, .{
            .file = file_path,
            .line = mapping.line,
            .code = .obligation_missing_normativity_or_ref,
            .message = try allocator.dupe(u8, "obligation must have a normativity keyword or reference relation"),
        });
    }
}

// ============================================================================
// CheckUniqueness
// ============================================================================

pub fn checkUniqueness(
    allocator: Allocator,
    file_path: []const u8,
    parse_result: yaml.ParseResult,
    all_errors: *std.ArrayList(ValidationError),
) !void {
    const docs = parse_result.documents;
    if (docs.len < 2) return;

    var seen = std.StringHashMap(void).init(allocator);
    defer seen.deinit();

    for (docs[1..]) |doc| {
        const root = doc.root orelse continue;
        const mapping = switch (root) {
            .mapping => |m| m,
            else => continue,
        };

        const spec_value = getMappingValue(mapping, "spec") orelse continue;
        const name = switch (spec_value) {
            .scalar => |s| s,
            else => continue,
        };

        if (name.value.len == 0) continue;

        if (seen.contains(name.value)) {
            try all_errors.append(allocator, .{
                .file = file_path,
                .line = name.line,
                .code = .spec_duplicate_name,
                .message = try std.fmt.allocPrint(allocator, "duplicate spec name \"{s}\"", .{name.value}),
            });
        } else {
            seen.put(name.value, {}) catch {};
        }
    }
}

// ============================================================================
// CheckRefs
// ============================================================================

pub fn checkRefs(
    allocator: Allocator,
    io: Io,
    file_path: []const u8,
    parse_result: yaml.ParseResult,
    project_root: ?[]const u8,
    all_errors: *std.ArrayList(ValidationError),
) !void {
    const docs = parse_result.documents;
    if (docs.len < 2) return;

    // Collect spec names in this file for same-file resolution
    var local_specs = std.StringHashMap(SpecInfo).init(allocator);
    defer {
        var it = local_specs.valueIterator();
        while (it.next()) |v| {
            v.slots.deinit();
        }
        local_specs.deinit();
    }

    for (docs[1..]) |doc| {
        const root = doc.root orelse continue;
        const mapping = switch (root) {
            .mapping => |m| m,
            else => continue,
        };

        const spec_value = getMappingValue(mapping, "spec") orelse continue;
        const name = switch (spec_value) {
            .scalar => |s| s,
            else => continue,
        };
        if (name.value.len == 0) continue;

        // Collect which slots this spec has
        var spec_slots = std.StringHashMap(void).init(allocator);
        for (mapping.entries) |entry| {
            if (isSlotKey(entry.key.value)) {
                spec_slots.put(entry.key.value, {}) catch {};
            }
        }

        local_specs.put(name.value, .{ .slots = spec_slots }) catch {};
    }

    // Track already-checked files for at-most-one error per (src, dst) pair
    var checked_files = std.StringHashMap(FileCheckResult).init(allocator);
    defer checked_files.deinit();

    // Walk all obligations looking for reference relations
    for (docs[1..]) |doc| {
        const root = doc.root orelse continue;
        const mapping = switch (root) {
            .mapping => |m| m,
            else => continue,
        };

        for (mapping.entries) |entry| {
            if (!isSlotKey(entry.key.value)) continue;

            switch (entry.value) {
                .sequence => |seq| {
                    for (seq.items) |item| {
                        switch (item) {
                            .mapping => |obl_mapping| {
                                try checkObligationRefs(
                                    allocator,
                                    io,
                                    file_path,
                                    obl_mapping,
                                    &local_specs,
                                    project_root,
                                    &checked_files,
                                    all_errors,
                                );
                            },
                            else => {},
                        }
                    }
                },
                else => {},
            }
        }
    }
}

const SpecInfo = struct {
    slots: std.StringHashMap(void),
};

const FileCheckResult = enum {
    ok,
    not_found,
    not_parseable,
};

fn checkObligationRefs(
    allocator: Allocator,
    io: Io,
    file_path: []const u8,
    mapping: yaml.YamlMapping,
    local_specs: *std.StringHashMap(SpecInfo),
    project_root: ?[]const u8,
    checked_files: *std.StringHashMap(FileCheckResult),
    all_errors: *std.ArrayList(ValidationError),
) !void {
    for (mapping.entries) |entry| {
        if (!isReferenceRelation(entry.key.value)) continue;

        const target_str = switch (entry.value) {
            .scalar => |s| s,
            else => continue,
        };

        if (target_str.value.len == 0) continue;

        // Parse the ref target
        const ref = parseRefTarget(target_str.value) orelse {
            try all_errors.append(allocator, .{
                .file = file_path,
                .line = target_str.line,
                .code = .ref_malformed,
                .message = try std.fmt.allocPrint(allocator, "malformed reference target \"{s}\"", .{target_str.value}),
            });
            continue;
        };

        // Check slot validity if present
        if (ref.slot) |slot| {
            if (!isSlotKey(slot)) {
                try all_errors.append(allocator, .{
                    .file = file_path,
                    .line = target_str.line,
                    .code = .ref_unknown_slot,
                    .message = try std.fmt.allocPrint(allocator, "unknown slot \"{s}\" in reference", .{slot}),
                });
                continue;
            }
        }

        if (ref.file_part) |ref_file| {
            // Cross-file reference
            const resolved_path = resolveRefFilePath(allocator, file_path, ref_file, project_root) orelse {
                try all_errors.append(allocator, .{
                    .file = file_path,
                    .line = target_str.line,
                    .code = .ref_file_not_found,
                    .message = try std.fmt.allocPrint(allocator, "referenced file \"{s}\" not found", .{ref_file}),
                });
                continue;
            };

            // Check if we already processed this file
            if (checked_files.get(resolved_path)) |fcr| {
                switch (fcr) {
                    .not_found => continue,
                    .not_parseable => continue,
                    .ok => {
                        try checkCrossFileSpec(allocator, io, file_path, target_str.line, resolved_path, ref.spec_name, ref.slot, all_errors);
                    },
                }
            } else {
                // First time checking this file
                const file_contents = readFileContents(allocator, io, resolved_path);
                if (file_contents == null) {
                    checked_files.put(resolved_path, .not_found) catch {};
                    try all_errors.append(allocator, .{
                        .file = file_path,
                        .line = target_str.line,
                        .code = .ref_file_not_found,
                        .message = try std.fmt.allocPrint(allocator, "referenced file \"{s}\" not found", .{ref_file}),
                    });
                    continue;
                }

                // Try parsing
                const cross_parse = yaml.parseYaml(allocator, file_contents.?) catch {
                    checked_files.put(resolved_path, .not_parseable) catch {};
                    try all_errors.append(allocator, .{
                        .file = file_path,
                        .line = target_str.line,
                        .code = .ref_file_not_parseable,
                        .message = try std.fmt.allocPrint(allocator, "referenced file \"{s}\" is not parseable", .{ref_file}),
                    });
                    continue;
                };
                defer yaml.freeParseResult(allocator, cross_parse);

                checked_files.put(resolved_path, .ok) catch {};
                try checkCrossFileSpecFromParsed(allocator, file_path, target_str.line, ref_file, ref.spec_name, ref.slot, cross_parse, all_errors);
            }
        } else {
            // Same-file reference
            if (local_specs.get(ref.spec_name)) |spec_info| {
                // Spec found, check slot if specified
                if (ref.slot) |slot| {
                    if (!spec_info.slots.contains(slot)) {
                        try all_errors.append(allocator, .{
                            .file = file_path,
                            .line = target_str.line,
                            .code = .ref_slot_not_declared,
                            .message = try std.fmt.allocPrint(allocator, "spec \"{s}\" does not declare slot \"{s}\"", .{ ref.spec_name, slot }),
                        });
                    }
                }
            } else {
                try all_errors.append(allocator, .{
                    .file = file_path,
                    .line = target_str.line,
                    .code = .ref_spec_not_found_same_file,
                    .message = try std.fmt.allocPrint(allocator, "spec \"{s}\" not found in this file", .{ref.spec_name}),
                });
            }
        }
    }
}

fn checkCrossFileSpec(
    allocator: Allocator,
    io: Io,
    file_path: []const u8,
    line: usize,
    resolved_path: []const u8,
    spec_name: []const u8,
    slot: ?[]const u8,
    all_errors: *std.ArrayList(ValidationError),
) !void {
    const file_contents = readFileContents(allocator, io, resolved_path) orelse return;
    defer allocator.free(file_contents);
    const cross_parse = yaml.parseYaml(allocator, file_contents) catch return;
    defer yaml.freeParseResult(allocator, cross_parse);
    try checkCrossFileSpecFromParsed(allocator, file_path, line, resolved_path, spec_name, slot, cross_parse, all_errors);
}

fn checkCrossFileSpecFromParsed(
    allocator: Allocator,
    file_path: []const u8,
    line: usize,
    ref_file: []const u8,
    spec_name: []const u8,
    slot: ?[]const u8,
    cross_parse: yaml.ParseResult,
    all_errors: *std.ArrayList(ValidationError),
) !void {
    const cross_docs = cross_parse.documents;

    // Find the spec in the cross-file
    for (cross_docs) |doc| {
        const root = doc.root orelse continue;
        const mapping = switch (root) {
            .mapping => |m| m,
            else => continue,
        };

        const sv = getMappingValue(mapping, "spec") orelse continue;
        const name = switch (sv) {
            .scalar => |s| s,
            else => continue,
        };

        if (std.mem.eql(u8, name.value, spec_name)) {
            // Found the spec. Check slot if specified.
            if (slot) |s| {
                var has_slot = false;
                for (mapping.entries) |me| {
                    if (std.mem.eql(u8, me.key.value, s)) {
                        has_slot = true;
                        break;
                    }
                }
                if (!has_slot) {
                    try all_errors.append(allocator, .{
                        .file = file_path,
                        .line = line,
                        .code = .ref_slot_not_declared,
                        .message = try std.fmt.allocPrint(allocator, "spec \"{s}\" in \"{s}\" does not declare slot \"{s}\"", .{ spec_name, ref_file, s }),
                    });
                }
            }
            return; // Spec found
        }
    }

    // Spec not found in the cross-file
    try all_errors.append(allocator, .{
        .file = file_path,
        .line = line,
        .code = .ref_spec_not_found_other_file,
        .message = try std.fmt.allocPrint(allocator, "spec \"{s}\" not found in \"{s}\"", .{ spec_name, ref_file }),
    });
}

// ============================================================================
// Ref target parsing
// ============================================================================

const RefTarget = struct {
    file_part: ?[]const u8, // null for same-file ref
    spec_name: []const u8,
    slot: ?[]const u8, // null if no slot specified
};

/// Parse a reference target string.
/// Grammar: ^([A-Za-z0-9._/-]+@)?[A-Za-z0-9._-]+(::[A-Z-]+)?$
pub fn parseRefTarget(target: []const u8) ?RefTarget {
    if (target.len == 0) return null;

    var remaining = target;
    var file_part: ?[]const u8 = null;

    // Check for file@ prefix
    if (std.mem.indexOf(u8, remaining, "@")) |at_idx| {
        const fp = remaining[0..at_idx];
        if (fp.len == 0) return null;
        if (!isValidFilePartChars(fp)) return null;
        file_part = fp;
        remaining = remaining[at_idx + 1 ..];
    }

    // Check for ::SLOT suffix
    var slot: ?[]const u8 = null;
    if (std.mem.indexOf(u8, remaining, "::")) |colon_idx| {
        const s = remaining[colon_idx + 2 ..];
        if (s.len == 0) return null;
        if (!isValidSlotRefChars(s)) return null;
        slot = s;
        remaining = remaining[0..colon_idx];
    }

    // The remaining part is the spec name
    if (remaining.len == 0) return null;
    if (!isValidSpecRefChars(remaining)) return null;

    return .{
        .file_part = file_part,
        .spec_name = remaining,
        .slot = slot,
    };
}

fn isValidFilePartChars(s: []const u8) bool {
    for (s) |ch| {
        if (!isAlphanumeric(ch) and ch != '.' and ch != '_' and ch != '/' and ch != '-') return false;
    }
    return true;
}

fn isValidSpecRefChars(s: []const u8) bool {
    for (s) |ch| {
        if (!isAlphanumeric(ch) and ch != '.' and ch != '_' and ch != '-') return false;
    }
    return true;
}

fn isValidSlotRefChars(s: []const u8) bool {
    for (s) |ch| {
        if (!isUpperAlpha(ch) and ch != '-') return false;
    }
    return true;
}

fn resolveRefFilePath(allocator: Allocator, source_file: []const u8, ref_file: []const u8, project_root: ?[]const u8) ?[]const u8 {
    // Append .yass.yaml suffix per spec
    const file_with_ext = std.fmt.allocPrint(allocator, "{s}.yass.yaml", .{ref_file}) catch return null;

    if (std.mem.startsWith(u8, ref_file, "./") or std.mem.startsWith(u8, ref_file, "../")) {
        // Relative to the referencing file's directory
        const dir = std.fs.path.dirname(source_file) orelse ".";
        allocator.free(file_with_ext);
        return std.fmt.allocPrint(allocator, "{s}/{s}.yass.yaml", .{ dir, ref_file }) catch null;
    } else if (project_root) |root| {
        // Root-relative
        allocator.free(file_with_ext);
        return std.fmt.allocPrint(allocator, "{s}/{s}.yass.yaml", .{ root, ref_file }) catch null;
    } else {
        // No project root - try relative to source dir as fallback
        const dir = std.fs.path.dirname(source_file) orelse ".";
        allocator.free(file_with_ext);
        return std.fmt.allocPrint(allocator, "{s}/{s}.yass.yaml", .{ dir, ref_file }) catch null;
    }
}

// ============================================================================
// Name validation helpers
// ============================================================================

fn isValidSpecNameChars(name: []const u8) bool {
    for (name) |ch| {
        if (!isAlphanumeric(ch) and ch != '.' and ch != '_' and ch != '-') return false;
    }
    return true;
}

fn isValidSpecNameForm(name: []const u8) bool {
    // ^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$
    if (name.len == 0) return false;

    var i: usize = 0;
    // First segment: [A-Za-z0-9_-]+
    if (!isSegmentChar(name[i])) return false;
    while (i < name.len and isSegmentChar(name[i])) : (i += 1) {}

    // Subsequent segments: (\.[A-Za-z0-9_-]+)*
    while (i < name.len) {
        if (name[i] != '.') return false;
        i += 1;
        if (i >= name.len) return false; // Trailing dot
        if (!isSegmentChar(name[i])) return false;
        while (i < name.len and isSegmentChar(name[i])) : (i += 1) {}
    }

    return i == name.len;
}

fn isSegmentChar(ch: u8) bool {
    return isAlphanumeric(ch) or ch == '_' or ch == '-';
}

fn isAlphanumeric(ch: u8) bool {
    return (ch >= 'A' and ch <= 'Z') or (ch >= 'a' and ch <= 'z') or (ch >= '0' and ch <= '9');
}

fn isUpperAlpha(ch: u8) bool {
    return (ch >= 'A' and ch <= 'Z');
}

fn isReservedName(name: []const u8) bool {
    for (reserved_names) |reserved| {
        if (std.ascii.eqlIgnoreCase(name, reserved)) return true;
    }
    return false;
}

fn isSlotKey(key: []const u8) bool {
    for (slot_keys) |sk| {
        if (std.mem.eql(u8, key, sk)) return true;
    }
    return false;
}

fn isNormativityKeyword(key: []const u8) bool {
    for (normativity_keywords) |nk| {
        if (std.mem.eql(u8, key, nk)) return true;
    }
    return false;
}

fn isReferenceRelation(key: []const u8) bool {
    for (reference_relations) |rr| {
        if (std.mem.eql(u8, key, rr)) return true;
    }
    return false;
}

/// Heuristic: does this key look like it could be a normativity or reference keyword?
/// All-uppercase with possible hyphens.
fn looksLikeNormativityOrRef(key: []const u8) bool {
    if (key.len == 0) return false;
    for (key) |ch| {
        if (!isUpperAlpha(ch) and ch != '-') return false;
    }
    return true;
}

/// Could this unknown all-uppercase key be a normativity keyword?
fn couldBeNormativity(key: []const u8) bool {
    if (std.mem.startsWith(u8, key, "MUST") or
        std.mem.startsWith(u8, key, "SHOULD") or
        std.mem.startsWith(u8, key, "MAY") or
        std.mem.startsWith(u8, key, "SHALL") or
        std.mem.startsWith(u8, key, "REQUIRED") or
        std.mem.startsWith(u8, key, "OPTIONAL"))
    {
        return true;
    }
    return false;
}

// ============================================================================
// Mapping helpers
// ============================================================================

fn mappingHasKey(mapping: yaml.YamlMapping, key: []const u8) bool {
    for (mapping.entries) |entry| {
        if (std.mem.eql(u8, entry.key.value, key)) return true;
    }
    return false;
}

fn getMappingValue(mapping: yaml.YamlMapping, key: []const u8) ?yaml.YamlNode {
    for (mapping.entries) |entry| {
        if (std.mem.eql(u8, entry.key.value, key)) return entry.value;
    }
    return null;
}

// ============================================================================
// Tests
// ============================================================================

const testing = std.testing;

fn makeErrList() std.ArrayList(ValidationError) {
    return .empty;
}

fn freeErrList(allocator: Allocator, list: *std.ArrayList(ValidationError)) void {
    for (list.items) |e| allocator.free(e.message);
    list.deinit(allocator);
}

fn freeParseResult(allocator: Allocator, result: yaml.ParseResult) void {
    for (result.documents) |doc| {
        if (doc.root) |root| {
            freeYamlNode(allocator, root);
        }
    }
    allocator.free(result.documents);
}

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

// ---------------------------------------------------------------------------
// checkYaml tests
// ---------------------------------------------------------------------------

test "checkYaml with valid YAML" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "name: test\n";
    const result = checkYaml(allocator, "test.yass.yaml", input, &errs) catch unreachable;
    defer freeParseResult(allocator, result);
    try testing.expectEqual(@as(usize, 0), errs.items.len);
}

test "checkYaml with BOM" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "\xEF\xBB\xBFname: test\n";
    const result = checkYaml(allocator, "test.yass.yaml", input, &errs);
    try testing.expectError(error.YamlCheckFailed, result);
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .yaml_has_bom);
}

test "checkYaml with empty file" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "";
    const result = checkYaml(allocator, "test.yass.yaml", input, &errs);
    try testing.expectError(error.YamlCheckFailed, result);
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .yaml_empty_file);
}

test "checkYaml with invalid UTF-8" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "\xff\xfe\x00test\n";
    const result = checkYaml(allocator, "test.yass.yaml", input, &errs);
    try testing.expectError(error.YamlCheckFailed, result);
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .yaml_not_utf8);
}

test "checkYaml with duplicate key" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "key: a\nkey: b\n";
    const result = checkYaml(allocator, "test.yass.yaml", input, &errs);
    try testing.expectError(error.YamlCheckFailed, result);
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .yaml_duplicate_key);
}

// ---------------------------------------------------------------------------
// checkPreamble tests
// ---------------------------------------------------------------------------

test "checkPreamble with valid preamble" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: A test spec\nversion: v1\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkPreamble(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 0), errs.items.len);
}

test "checkPreamble missing description" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\nversion: v1\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkPreamble(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .preamble_missing_description);
}

test "checkPreamble wrong version" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v2\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkPreamble(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .preamble_unknown_version);
}

test "checkPreamble bad related" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\nrelated: not-a-list\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkPreamble(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .preamble_bad_related);
}

test "checkPreamble has spec key" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\nspec: my-spec\ndescription: test\nversion: v1\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkPreamble(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .preamble_has_spec_key);
}

test "checkPreamble missing version" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkPreamble(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .preamble_missing_version);
}

test "checkPreamble with empty stream" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const empty_docs: []yaml.YamlDocument = &.{};
    const result = yaml.ParseResult{
        .documents = empty_docs,
    };

    checkPreamble(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .yaml_empty_stream);
}

test "checkPreamble with valid related field" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\nrelated:\n  - other.yass.yaml\n  - another.yass.yaml\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkPreamble(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 0), errs.items.len);
}

// ---------------------------------------------------------------------------
// checkSpec tests
// ---------------------------------------------------------------------------

test "checkSpec with valid spec" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: my-service\nINPUT:\n  - MUST: accept a request\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 0), errs.items.len);
}

test "checkSpec with reserved name" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: MUST\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .spec_name_reserved);
}

test "checkSpec with bad name chars" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: my service!\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .spec_name_bad_chars);
}

test "checkSpec with unknown key" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: my-service\nFOOBAR: something\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .spec_unknown_key);
}

test "checkSpec with obligation missing normativity" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: my-service\nINPUT:\n  - WHEN: something\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};

    var has_guard_err = false;
    var has_missing_err = false;
    for (errs.items) |e| {
        if (e.code == .obligation_guard_without_normativity) has_guard_err = true;
        if (e.code == .obligation_missing_normativity_or_ref) has_missing_err = true;
    }
    try testing.expect(has_guard_err);
    try testing.expect(has_missing_err);
}

test "checkSpec with slot value not list" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: my-service\nINPUT: not-a-list\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .slot_value_not_list);
}

test "checkSpec with empty spec name" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: \"\"\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .spec_name_empty);
}

test "checkSpec with bad name form - trailing dot" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: my-service.\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .spec_name_bad_form);
}

test "checkSpec with bad name form - double dot" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: my..service\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .spec_name_bad_form);
}

test "checkSpec with bad name form - leading dot" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: .my-service\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    // Leading dot is caught as bad_chars since . at start fails isSegmentChar
    // Actually, '.' is a valid spec name char but not a segment char
    // So isValidSpecNameChars passes, but isValidSpecNameForm fails
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .spec_name_bad_form);
}

test "checkSpec with reserved name case insensitive" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: must\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .spec_name_reserved);
}

test "checkSpec with valid dotted name" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: my-service.auth.v2\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 0), errs.items.len);
}

test "checkSpec with CONFORMS reference" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: my-service\nINPUT:\n  - CONFORMS: other-spec\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 0), errs.items.len);
}

test "checkSpec with USES reference" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: my-service\nINPUT:\n  - USES: other-spec\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 0), errs.items.len);
}

test "checkSpec with MUST and WHEN" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: my-service\nINPUT:\n  - MUST: do thing\n    WHEN: condition\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 0), errs.items.len);
}

// ---------------------------------------------------------------------------
// checkUniqueness tests
// ---------------------------------------------------------------------------

test "checkUniqueness with duplicates" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: my-service\n---\nspec: my-service\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkUniqueness(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .spec_duplicate_name);
}

test "checkUniqueness with no duplicates" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: service-a\n---\nspec: service-b\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkUniqueness(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 0), errs.items.len);
}

// ---------------------------------------------------------------------------
// Ref target parsing tests
// ---------------------------------------------------------------------------

test "parseRefTarget simple spec name" {
    const ref = parseRefTarget("my-service").?;
    try testing.expect(ref.file_part == null);
    try testing.expectEqualStrings("my-service", ref.spec_name);
    try testing.expect(ref.slot == null);
}

test "parseRefTarget with file" {
    const ref = parseRefTarget("other.yass.yaml@my-service").?;
    try testing.expectEqualStrings("other.yass.yaml", ref.file_part.?);
    try testing.expectEqualStrings("my-service", ref.spec_name);
    try testing.expect(ref.slot == null);
}

test "parseRefTarget with slot" {
    const ref = parseRefTarget("my-service::INPUT").?;
    try testing.expect(ref.file_part == null);
    try testing.expectEqualStrings("my-service", ref.spec_name);
    try testing.expectEqualStrings("INPUT", ref.slot.?);
}

test "parseRefTarget with file and slot" {
    const ref = parseRefTarget("other.yass.yaml@my-service::INPUT").?;
    try testing.expectEqualStrings("other.yass.yaml", ref.file_part.?);
    try testing.expectEqualStrings("my-service", ref.spec_name);
    try testing.expectEqualStrings("INPUT", ref.slot.?);
}

test "parseRefTarget malformed - empty" {
    try testing.expect(parseRefTarget("") == null);
}

test "parseRefTarget malformed - just @" {
    try testing.expect(parseRefTarget("@") == null);
}

test "parseRefTarget malformed - empty spec after @" {
    try testing.expect(parseRefTarget("file@") == null);
}

test "parseRefTarget malformed - empty slot after ::" {
    try testing.expect(parseRefTarget("spec::") == null);
}

test "parseRefTarget with path in file part" {
    const ref = parseRefTarget("sub/dir/other.yass.yaml@my-service").?;
    try testing.expectEqualStrings("sub/dir/other.yass.yaml", ref.file_part.?);
    try testing.expectEqualStrings("my-service", ref.spec_name);
}

// ---------------------------------------------------------------------------
// Name validation tests
// ---------------------------------------------------------------------------

test "isValidSpecNameChars" {
    try testing.expect(isValidSpecNameChars("my-service"));
    try testing.expect(isValidSpecNameChars("My_Service.v1"));
    try testing.expect(isValidSpecNameChars("abc123"));
    try testing.expect(!isValidSpecNameChars("my service"));
    try testing.expect(!isValidSpecNameChars("my@service"));
    try testing.expect(!isValidSpecNameChars("my/service"));
}

test "isValidSpecNameForm" {
    try testing.expect(isValidSpecNameForm("my-service"));
    try testing.expect(isValidSpecNameForm("my-service.auth"));
    try testing.expect(isValidSpecNameForm("a.b.c"));
    try testing.expect(!isValidSpecNameForm(".leading"));
    try testing.expect(!isValidSpecNameForm("trailing."));
    try testing.expect(!isValidSpecNameForm("double..dot"));
    try testing.expect(!isValidSpecNameForm(""));
}

test "isReservedName" {
    try testing.expect(isReservedName("MUST"));
    try testing.expect(isReservedName("must"));
    try testing.expect(isReservedName("Must"));
    try testing.expect(isReservedName("SHOULD-NOT"));
    try testing.expect(isReservedName("INPUT"));
    try testing.expect(isReservedName("CONFORMS"));
    try testing.expect(isReservedName("WHEN"));
    try testing.expect(isReservedName("SEE"));
    try testing.expect(!isReservedName("my-service"));
    try testing.expect(!isReservedName("foobar"));
}

// ---------------------------------------------------------------------------
// Full validate integration-style tests
// ---------------------------------------------------------------------------

test "full validate with valid file content via all checks" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: A test\nversion: v1\n---\nspec: my-service\nINPUT:\n  - MUST: accept requests\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);

    checkPreamble(allocator, "test.yass.yaml", result, &errs) catch {};
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    checkUniqueness(allocator, "test.yass.yaml", result, &errs) catch {};

    try testing.expectEqual(@as(usize, 0), errs.items.len);
}

test "full validate with preamble error" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\nversion: v1\n---\nspec: my-service\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);

    checkPreamble(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .preamble_missing_description);
}

test "checkRefs with same-file ref to existing spec" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: service-a\nINPUT:\n  - CONFORMS: service-b\n---\nspec: service-b\nINPUT:\n  - MUST: work\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);

    var threaded = std.Io.Threaded.init(allocator, .{});
    const io = threaded.io();

    checkRefs(allocator, io, "test.yass.yaml", result, null, &errs) catch {};
    try testing.expectEqual(@as(usize, 0), errs.items.len);
}

test "checkRefs with same-file ref to missing spec" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: service-a\nINPUT:\n  - CONFORMS: nonexistent\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);

    var threaded = std.Io.Threaded.init(allocator, .{});
    const io = threaded.io();

    checkRefs(allocator, io, "test.yass.yaml", result, null, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .ref_spec_not_found_same_file);
}

test "checkRefs with malformed ref target" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: service-a\nINPUT:\n  - CONFORMS: \"@\"\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);

    var threaded = std.Io.Threaded.init(allocator, .{});
    const io = threaded.io();

    checkRefs(allocator, io, "test.yass.yaml", result, null, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .ref_malformed);
}

test "checkRefs with unknown slot in ref" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: service-a\nINPUT:\n  - CONFORMS: \"service-b::FOOBAR\"\n---\nspec: service-b\nINPUT:\n  - MUST: work\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);

    var threaded = std.Io.Threaded.init(allocator, .{});
    const io = threaded.io();

    checkRefs(allocator, io, "test.yass.yaml", result, null, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .ref_unknown_slot);
}

test "checkRefs with valid slot ref" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: service-a\nINPUT:\n  - CONFORMS: \"service-b::INPUT\"\n---\nspec: service-b\nINPUT:\n  - MUST: work\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);

    var threaded = std.Io.Threaded.init(allocator, .{});
    const io = threaded.io();

    checkRefs(allocator, io, "test.yass.yaml", result, null, &errs) catch {};
    try testing.expectEqual(@as(usize, 0), errs.items.len);
}

test "checkRefs with slot not declared on target spec" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: service-a\nINPUT:\n  - CONFORMS: \"service-b::RETURN\"\n---\nspec: service-b\nINPUT:\n  - MUST: work\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);

    var threaded = std.Io.Threaded.init(allocator, .{});
    const io = threaded.io();

    checkRefs(allocator, io, "test.yass.yaml", result, null, &errs) catch {};
    try testing.expectEqual(@as(usize, 1), errs.items.len);
    try testing.expect(errs.items[0].code == .ref_slot_not_declared);
}

test "checkSpec detects duplicate normativity keywords (different keywords)" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: Foo\nINPUT:\n- MUST: do x\n  SHOULD: also y\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expect(errs.items.len > 0);
    try testing.expectEqual(err.ErrorCode.obligation_duplicate_normativity, errs.items[0].code);
}

test "multiple spec names are all valid" {
    const allocator = testing.allocator;
    var errs = makeErrList();
    defer freeErrList(allocator, &errs);

    const input = "---\ndescription: test\nversion: v1\n---\nspec: auth\n---\nspec: api-gateway\n---\nspec: data.store.v2\n";
    const result = try yaml.parseYaml(allocator, input);
    defer freeParseResult(allocator, result);
    checkSpec(allocator, "test.yass.yaml", result, &errs) catch {};
    checkUniqueness(allocator, "test.yass.yaml", result, &errs) catch {};
    try testing.expectEqual(@as(usize, 0), errs.items.len);
}
