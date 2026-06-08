const std = @import("std");
const testing = std.testing;

/// Machine-stable error codes for the yass CLI.
/// Every code maps to a dotted string representation and an exit code.
pub const ErrorCode = enum {
    // Exit codes
    exit_success,
    exit_processing,
    exit_usage,
    exit_sigint,
    exit_sigterm,

    // Argv errors
    argv_unknown_subcommand,
    argv_no_subcommand,
    argv_unknown_flag,
    argv_empty_argument,
    argv_short_flag,
    argv_case_mismatch,
    argv_abbreviation,
    argv_missing_positional,
    argv_stdin_dash,

    // Path errors
    path_not_found,
    path_bad_extension,
    path_unreadable,
    path_invalid_type,
    path_colon_in_path,

    // Glob errors
    glob_no_match,

    // Discover errors
    discover_no_files,
    discover_dir_unreadable,

    // Findroot errors
    findroot_no_marker,

    // YAML errors
    yaml_not_utf8,
    yaml_has_bom,
    yaml_malformed,
    yaml_empty_file,
    yaml_duplicate_key,
    yaml_anchor_or_alias,
    yaml_empty_stream,

    // Preamble errors
    preamble_has_spec_key,
    preamble_missing,
    preamble_misplaced,
    preamble_duplicate,
    preamble_missing_description,
    preamble_missing_version,
    preamble_unknown_version,
    preamble_bad_related,

    // Spec errors
    spec_no_name,
    spec_name_not_string,
    spec_name_empty,
    spec_name_bad_chars,
    spec_name_bad_form,
    spec_name_reserved,
    spec_unknown_key,
    spec_duplicate_name,

    // Slot errors
    slot_value_not_list,

    // Obligation errors
    obligation_bad_value_shape,
    obligation_missing_normativity_or_ref,
    obligation_guard_without_normativity,
    obligation_duplicate_reference,
    obligation_duplicate_normativity,

    // Normativity errors
    normativity_unknown,

    // Reference errors
    reference_unknown_relation,

    // Ref errors
    ref_malformed,
    ref_unknown_slot,
    ref_slot_not_declared,
    ref_spec_not_found_same_file,
    ref_file_not_found,
    ref_file_not_parseable,
    ref_spec_not_found_other_file,

    // Query errors
    query_name_missing,
    query_name_blank,
    query_no_match,
    query_conforms_unresolved,
    query_conforms_no_slot,
    query_scope_not_found,
    query_scope_empty,

    // Internal errors
    internal_uncaught,

    /// Returns the dotted string representation of this error code,
    /// e.g. "yass.argv.unknown_subcommand".
    pub fn string(self: ErrorCode) []const u8 {
        return switch (self) {
            // Exit
            .exit_success => "yass.exit.success",
            .exit_processing => "yass.exit.processing",
            .exit_usage => "yass.exit.usage",
            .exit_sigint => "yass.exit.sigint",
            .exit_sigterm => "yass.exit.sigterm",

            // Argv
            .argv_unknown_subcommand => "yass.argv.unknown_subcommand",
            .argv_no_subcommand => "yass.argv.no_subcommand",
            .argv_unknown_flag => "yass.argv.unknown_flag",
            .argv_empty_argument => "yass.argv.empty_argument",
            .argv_short_flag => "yass.argv.short_flag",
            .argv_case_mismatch => "yass.argv.case_mismatch",
            .argv_abbreviation => "yass.argv.abbreviation",
            .argv_missing_positional => "yass.argv.missing_positional",
            .argv_stdin_dash => "yass.argv.stdin_dash",

            // Path
            .path_not_found => "yass.path.not_found",
            .path_bad_extension => "yass.path.bad_extension",
            .path_unreadable => "yass.path.unreadable",
            .path_invalid_type => "yass.path.invalid_type",
            .path_colon_in_path => "yass.path.colon_in_path",

            // Glob
            .glob_no_match => "yass.glob.no_match",

            // Discover
            .discover_no_files => "yass.discover.no_files",
            .discover_dir_unreadable => "yass.discover.dir_unreadable",

            // Findroot
            .findroot_no_marker => "yass.findroot.no_marker",

            // YAML
            .yaml_not_utf8 => "yass.yaml.not_utf8",
            .yaml_has_bom => "yass.yaml.has_bom",
            .yaml_malformed => "yass.yaml.malformed",
            .yaml_empty_file => "yass.yaml.empty_file",
            .yaml_duplicate_key => "yass.yaml.duplicate_key",
            .yaml_anchor_or_alias => "yass.yaml.anchor_or_alias",
            .yaml_empty_stream => "yass.yaml.empty_stream",

            // Preamble
            .preamble_has_spec_key => "yass.preamble.has_spec_key",
            .preamble_missing => "yass.preamble.missing",
            .preamble_misplaced => "yass.preamble.misplaced",
            .preamble_duplicate => "yass.preamble.duplicate",
            .preamble_missing_description => "yass.preamble.missing_description",
            .preamble_missing_version => "yass.preamble.missing_version",
            .preamble_unknown_version => "yass.preamble.unknown_version",
            .preamble_bad_related => "yass.preamble.bad_related",

            // Spec
            .spec_no_name => "yass.spec.no_name",
            .spec_name_not_string => "yass.spec.name_not_string",
            .spec_name_empty => "yass.spec.name_empty",
            .spec_name_bad_chars => "yass.spec.name_bad_chars",
            .spec_name_bad_form => "yass.spec.name_bad_form",
            .spec_name_reserved => "yass.spec.name_reserved",
            .spec_unknown_key => "yass.spec.unknown_key",
            .spec_duplicate_name => "yass.spec.duplicate_name",

            // Slot
            .slot_value_not_list => "yass.slot.value_not_list",

            // Obligation
            .obligation_bad_value_shape => "yass.obligation.bad_value_shape",
            .obligation_missing_normativity_or_ref => "yass.obligation.missing_normativity_or_ref",
            .obligation_guard_without_normativity => "yass.obligation.guard_without_normativity",
            .obligation_duplicate_reference => "yass.obligation.duplicate_reference",
            .obligation_duplicate_normativity => "yass.obligation.duplicate_normativity",

            // Normativity
            .normativity_unknown => "yass.normativity.unknown",

            // Reference
            .reference_unknown_relation => "yass.reference.unknown_relation",

            // Ref
            .ref_malformed => "yass.ref.malformed",
            .ref_unknown_slot => "yass.ref.unknown_slot",
            .ref_slot_not_declared => "yass.ref.slot_not_declared",
            .ref_spec_not_found_same_file => "yass.ref.spec_not_found_same_file",
            .ref_file_not_found => "yass.ref.file_not_found",
            .ref_file_not_parseable => "yass.ref.file_not_parseable",
            .ref_spec_not_found_other_file => "yass.ref.spec_not_found_other_file",

            // Query
            .query_name_missing => "yass.query.name_missing",
            .query_name_blank => "yass.query.name_blank",
            .query_no_match => "yass.query.no_match",
            .query_conforms_unresolved => "yass.query.conforms_unresolved",
            .query_conforms_no_slot => "yass.query.conforms_no_slot",
            .query_scope_not_found => "yass.query.scope_not_found",
            .query_scope_empty => "yass.query.scope_empty",

            // Internal
            .internal_uncaught => "yass.internal.uncaught",
        };
    }

    /// Returns the process exit code for this error code.
    pub fn exitCode(self: ErrorCode) u8 {
        return switch (self) {
            .exit_success => 0,
            .exit_processing => 1,
            .exit_usage => 2,
            .exit_sigint => 130,
            .exit_sigterm => 143,

            .argv_unknown_subcommand,
            .argv_no_subcommand,
            .argv_unknown_flag,
            .argv_empty_argument,
            .argv_short_flag,
            .argv_case_mismatch,
            .argv_abbreviation,
            .argv_missing_positional,
            .argv_stdin_dash,
            => 2,

            .path_not_found,
            .path_bad_extension,
            .path_unreadable,
            .path_invalid_type,
            .path_colon_in_path,
            => 2,

            .glob_no_match => 2,

            .discover_no_files => 2,
            // discover_dir_unreadable is non-fatal but when used as an exit code,
            // falls back to processing error
            .discover_dir_unreadable => 1,

            .findroot_no_marker => 2,

            .yaml_not_utf8,
            .yaml_has_bom,
            .yaml_malformed,
            .yaml_empty_file,
            .yaml_duplicate_key,
            .yaml_anchor_or_alias,
            .yaml_empty_stream,
            => 1,

            .preamble_has_spec_key,
            .preamble_missing,
            .preamble_misplaced,
            .preamble_duplicate,
            .preamble_missing_description,
            .preamble_missing_version,
            .preamble_unknown_version,
            .preamble_bad_related,
            => 1,

            .spec_no_name,
            .spec_name_not_string,
            .spec_name_empty,
            .spec_name_bad_chars,
            .spec_name_bad_form,
            .spec_name_reserved,
            .spec_unknown_key,
            .spec_duplicate_name,
            => 1,

            .slot_value_not_list => 1,

            .obligation_bad_value_shape,
            .obligation_missing_normativity_or_ref,
            .obligation_guard_without_normativity,
            .obligation_duplicate_reference,
            .obligation_duplicate_normativity,
            => 1,

            .normativity_unknown => 1,

            .reference_unknown_relation => 1,

            .ref_malformed,
            .ref_unknown_slot,
            .ref_slot_not_declared,
            .ref_spec_not_found_same_file,
            .ref_file_not_found,
            .ref_file_not_parseable,
            .ref_spec_not_found_other_file,
            => 1,

            .query_name_missing,
            .query_name_blank,
            => 2,

            .query_no_match,
            .query_conforms_unresolved,
            .query_conforms_no_slot,
            => 1,

            .query_scope_not_found,
            .query_scope_empty,
            => 2,

            .internal_uncaught => 1,
        };
    }
};

/// Top-level convenience alias.
pub const errorCodeString = ErrorCode.string;

/// Top-level convenience alias.
pub const exitCode = ErrorCode.exitCode;

/// Replaces all newline characters (LF, CR, CRLF) in `message` with a single space.
/// Returns a slice into `buf`.
fn sanitizeMessage(buf: []u8, message: []const u8) []const u8 {
    var out_len: usize = 0;
    var i: usize = 0;
    while (i < message.len) : (i += 1) {
        if (i < buf.len) {
            if (message[i] == '\r') {
                buf[out_len] = ' ';
                out_len += 1;
                // Skip following LF in CRLF
                if (i + 1 < message.len and message[i + 1] == '\n') {
                    i += 1;
                }
            } else if (message[i] == '\n') {
                buf[out_len] = ' ';
                out_len += 1;
            } else {
                buf[out_len] = message[i];
                out_len += 1;
            }
        }
    }
    return buf[0..out_len];
}

/// Converts backslashes to forward slashes in a path.
/// Returns a slice into `buf`.
fn normalizeSlashes(buf: []u8, path: []const u8) []const u8 {
    const len = @min(path.len, buf.len);
    for (0..len) |i| {
        buf[i] = if (path[i] == '\\') '/' else path[i];
    }
    return buf[0..len];
}

/// Makes `file_path` relative to `cwd` for display purposes.
/// - If null, returns "yass".
/// - If under cwd, returns the relative portion (no leading "./").
/// - If directly in cwd (no subdirectory), returns basename only.
/// - If not under cwd, returns the absolute path.
/// - Always uses forward slashes.
fn formatFilePath(buf: []u8, file_path: ?[]const u8, cwd: []const u8) []const u8 {
    const path = file_path orelse return "yass";

    // Normalize slashes in both path and cwd for comparison
    var path_buf: [4096]u8 = undefined;
    const norm_path = normalizeSlashes(&path_buf, path);

    var cwd_buf: [4096]u8 = undefined;
    const norm_cwd = normalizeSlashes(&cwd_buf, cwd);

    // Ensure cwd ends with a separator for prefix matching
    var cwd_prefix_buf: [4097]u8 = undefined;
    var cwd_prefix_len: usize = norm_cwd.len;
    @memcpy(cwd_prefix_buf[0..norm_cwd.len], norm_cwd);
    if (cwd_prefix_len > 0 and cwd_prefix_buf[cwd_prefix_len - 1] != '/') {
        cwd_prefix_buf[cwd_prefix_len] = '/';
        cwd_prefix_len += 1;
    }
    const cwd_prefix = cwd_prefix_buf[0..cwd_prefix_len];

    // Check if path starts with cwd prefix
    if (norm_path.len >= cwd_prefix.len and std.mem.eql(u8, norm_path[0..cwd_prefix.len], cwd_prefix)) {
        const relative = norm_path[cwd_prefix.len..];
        const len = @min(relative.len, buf.len);
        @memcpy(buf[0..len], relative[0..len]);
        return buf[0..len];
    }

    // Not under cwd: return the normalized absolute path
    const len = @min(norm_path.len, buf.len);
    @memcpy(buf[0..len], norm_path[0..len]);
    return buf[0..len];
}

/// Formats a complete error line.
///
/// Format with line number: `<file>:<line>: [<code>] <message>`
/// Format without line number: `<file>: [<code>] <message>`
/// Format without file: `yass: [<code>] <message>`
///
/// The caller owns the returned memory (allocated from `allocator`).
pub fn formatErrorLine(
    allocator: std.mem.Allocator,
    file_path: ?[]const u8,
    line: ?usize,
    code: ErrorCode,
    message: []const u8,
    cwd: []const u8,
) std.mem.Allocator.Error![]const u8 {
    // Sanitize message: replace newlines with spaces
    var msg_buf: [8192]u8 = undefined;
    const clean_message = sanitizeMessage(&msg_buf, message);

    // Format file path
    var file_buf: [4096]u8 = undefined;
    const display_file = formatFilePath(&file_buf, file_path, cwd);

    const code_str = code.string();

    if (line) |ln| {
        return std.fmt.allocPrint(allocator, "{s}:{d}: [{s}] {s}", .{
            display_file,
            ln,
            code_str,
            clean_message,
        });
    } else {
        return std.fmt.allocPrint(allocator, "{s}: [{s}] {s}", .{
            display_file,
            code_str,
            clean_message,
        });
    }
}

// =============================================================================
// Tests
// =============================================================================

test "errorCodeString returns correct dotted strings" {
    // Exit codes
    try testing.expectEqualStrings("yass.exit.success", ErrorCode.exit_success.string());
    try testing.expectEqualStrings("yass.exit.processing", ErrorCode.exit_processing.string());
    try testing.expectEqualStrings("yass.exit.usage", ErrorCode.exit_usage.string());
    try testing.expectEqualStrings("yass.exit.sigint", ErrorCode.exit_sigint.string());
    try testing.expectEqualStrings("yass.exit.sigterm", ErrorCode.exit_sigterm.string());

    // Argv
    try testing.expectEqualStrings("yass.argv.unknown_subcommand", ErrorCode.argv_unknown_subcommand.string());
    try testing.expectEqualStrings("yass.argv.no_subcommand", ErrorCode.argv_no_subcommand.string());
    try testing.expectEqualStrings("yass.argv.unknown_flag", ErrorCode.argv_unknown_flag.string());
    try testing.expectEqualStrings("yass.argv.empty_argument", ErrorCode.argv_empty_argument.string());
    try testing.expectEqualStrings("yass.argv.short_flag", ErrorCode.argv_short_flag.string());
    try testing.expectEqualStrings("yass.argv.case_mismatch", ErrorCode.argv_case_mismatch.string());
    try testing.expectEqualStrings("yass.argv.abbreviation", ErrorCode.argv_abbreviation.string());
    try testing.expectEqualStrings("yass.argv.missing_positional", ErrorCode.argv_missing_positional.string());
    try testing.expectEqualStrings("yass.argv.stdin_dash", ErrorCode.argv_stdin_dash.string());

    // Path
    try testing.expectEqualStrings("yass.path.not_found", ErrorCode.path_not_found.string());
    try testing.expectEqualStrings("yass.path.bad_extension", ErrorCode.path_bad_extension.string());
    try testing.expectEqualStrings("yass.path.unreadable", ErrorCode.path_unreadable.string());
    try testing.expectEqualStrings("yass.path.invalid_type", ErrorCode.path_invalid_type.string());
    try testing.expectEqualStrings("yass.path.colon_in_path", ErrorCode.path_colon_in_path.string());

    // Glob
    try testing.expectEqualStrings("yass.glob.no_match", ErrorCode.glob_no_match.string());

    // Discover
    try testing.expectEqualStrings("yass.discover.no_files", ErrorCode.discover_no_files.string());
    try testing.expectEqualStrings("yass.discover.dir_unreadable", ErrorCode.discover_dir_unreadable.string());

    // Findroot
    try testing.expectEqualStrings("yass.findroot.no_marker", ErrorCode.findroot_no_marker.string());

    // YAML
    try testing.expectEqualStrings("yass.yaml.not_utf8", ErrorCode.yaml_not_utf8.string());
    try testing.expectEqualStrings("yass.yaml.has_bom", ErrorCode.yaml_has_bom.string());
    try testing.expectEqualStrings("yass.yaml.malformed", ErrorCode.yaml_malformed.string());
    try testing.expectEqualStrings("yass.yaml.empty_file", ErrorCode.yaml_empty_file.string());
    try testing.expectEqualStrings("yass.yaml.duplicate_key", ErrorCode.yaml_duplicate_key.string());
    try testing.expectEqualStrings("yass.yaml.anchor_or_alias", ErrorCode.yaml_anchor_or_alias.string());
    try testing.expectEqualStrings("yass.yaml.empty_stream", ErrorCode.yaml_empty_stream.string());

    // Preamble
    try testing.expectEqualStrings("yass.preamble.has_spec_key", ErrorCode.preamble_has_spec_key.string());
    try testing.expectEqualStrings("yass.preamble.missing", ErrorCode.preamble_missing.string());
    try testing.expectEqualStrings("yass.preamble.misplaced", ErrorCode.preamble_misplaced.string());
    try testing.expectEqualStrings("yass.preamble.duplicate", ErrorCode.preamble_duplicate.string());
    try testing.expectEqualStrings("yass.preamble.missing_description", ErrorCode.preamble_missing_description.string());
    try testing.expectEqualStrings("yass.preamble.missing_version", ErrorCode.preamble_missing_version.string());
    try testing.expectEqualStrings("yass.preamble.unknown_version", ErrorCode.preamble_unknown_version.string());
    try testing.expectEqualStrings("yass.preamble.bad_related", ErrorCode.preamble_bad_related.string());

    // Spec
    try testing.expectEqualStrings("yass.spec.no_name", ErrorCode.spec_no_name.string());
    try testing.expectEqualStrings("yass.spec.name_not_string", ErrorCode.spec_name_not_string.string());
    try testing.expectEqualStrings("yass.spec.name_empty", ErrorCode.spec_name_empty.string());
    try testing.expectEqualStrings("yass.spec.name_bad_chars", ErrorCode.spec_name_bad_chars.string());
    try testing.expectEqualStrings("yass.spec.name_bad_form", ErrorCode.spec_name_bad_form.string());
    try testing.expectEqualStrings("yass.spec.name_reserved", ErrorCode.spec_name_reserved.string());
    try testing.expectEqualStrings("yass.spec.unknown_key", ErrorCode.spec_unknown_key.string());
    try testing.expectEqualStrings("yass.spec.duplicate_name", ErrorCode.spec_duplicate_name.string());

    // Slot
    try testing.expectEqualStrings("yass.slot.value_not_list", ErrorCode.slot_value_not_list.string());

    // Obligation
    try testing.expectEqualStrings("yass.obligation.bad_value_shape", ErrorCode.obligation_bad_value_shape.string());
    try testing.expectEqualStrings("yass.obligation.missing_normativity_or_ref", ErrorCode.obligation_missing_normativity_or_ref.string());
    try testing.expectEqualStrings("yass.obligation.guard_without_normativity", ErrorCode.obligation_guard_without_normativity.string());
    try testing.expectEqualStrings("yass.obligation.duplicate_reference", ErrorCode.obligation_duplicate_reference.string());
    try testing.expectEqualStrings("yass.obligation.duplicate_normativity", ErrorCode.obligation_duplicate_normativity.string());

    // Normativity
    try testing.expectEqualStrings("yass.normativity.unknown", ErrorCode.normativity_unknown.string());

    // Reference
    try testing.expectEqualStrings("yass.reference.unknown_relation", ErrorCode.reference_unknown_relation.string());

    // Ref
    try testing.expectEqualStrings("yass.ref.malformed", ErrorCode.ref_malformed.string());
    try testing.expectEqualStrings("yass.ref.unknown_slot", ErrorCode.ref_unknown_slot.string());
    try testing.expectEqualStrings("yass.ref.slot_not_declared", ErrorCode.ref_slot_not_declared.string());
    try testing.expectEqualStrings("yass.ref.spec_not_found_same_file", ErrorCode.ref_spec_not_found_same_file.string());
    try testing.expectEqualStrings("yass.ref.file_not_found", ErrorCode.ref_file_not_found.string());
    try testing.expectEqualStrings("yass.ref.file_not_parseable", ErrorCode.ref_file_not_parseable.string());
    try testing.expectEqualStrings("yass.ref.spec_not_found_other_file", ErrorCode.ref_spec_not_found_other_file.string());

    // Query
    try testing.expectEqualStrings("yass.query.name_missing", ErrorCode.query_name_missing.string());
    try testing.expectEqualStrings("yass.query.name_blank", ErrorCode.query_name_blank.string());
    try testing.expectEqualStrings("yass.query.no_match", ErrorCode.query_no_match.string());
    try testing.expectEqualStrings("yass.query.conforms_unresolved", ErrorCode.query_conforms_unresolved.string());
    try testing.expectEqualStrings("yass.query.conforms_no_slot", ErrorCode.query_conforms_no_slot.string());
    try testing.expectEqualStrings("yass.query.scope_not_found", ErrorCode.query_scope_not_found.string());
    try testing.expectEqualStrings("yass.query.scope_empty", ErrorCode.query_scope_empty.string());

    // Internal
    try testing.expectEqualStrings("yass.internal.uncaught", ErrorCode.internal_uncaught.string());
}

test "exitCode returns correct values" {
    // Exit codes
    try testing.expectEqual(@as(u8, 0), ErrorCode.exit_success.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.exit_processing.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.exit_usage.exitCode());
    try testing.expectEqual(@as(u8, 130), ErrorCode.exit_sigint.exitCode());
    try testing.expectEqual(@as(u8, 143), ErrorCode.exit_sigterm.exitCode());

    // Argv -> 2
    try testing.expectEqual(@as(u8, 2), ErrorCode.argv_unknown_subcommand.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.argv_no_subcommand.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.argv_unknown_flag.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.argv_empty_argument.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.argv_short_flag.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.argv_case_mismatch.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.argv_abbreviation.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.argv_missing_positional.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.argv_stdin_dash.exitCode());

    // Path -> 2
    try testing.expectEqual(@as(u8, 2), ErrorCode.path_not_found.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.path_bad_extension.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.path_unreadable.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.path_invalid_type.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.path_colon_in_path.exitCode());

    // Glob -> 2
    try testing.expectEqual(@as(u8, 2), ErrorCode.glob_no_match.exitCode());

    // Discover
    try testing.expectEqual(@as(u8, 2), ErrorCode.discover_no_files.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.discover_dir_unreadable.exitCode());

    // Findroot -> 2
    try testing.expectEqual(@as(u8, 2), ErrorCode.findroot_no_marker.exitCode());

    // YAML -> 1
    try testing.expectEqual(@as(u8, 1), ErrorCode.yaml_not_utf8.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.yaml_has_bom.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.yaml_malformed.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.yaml_empty_file.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.yaml_duplicate_key.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.yaml_anchor_or_alias.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.yaml_empty_stream.exitCode());

    // Preamble -> 1
    try testing.expectEqual(@as(u8, 1), ErrorCode.preamble_has_spec_key.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.preamble_missing.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.preamble_misplaced.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.preamble_duplicate.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.preamble_missing_description.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.preamble_missing_version.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.preamble_unknown_version.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.preamble_bad_related.exitCode());

    // Spec -> 1
    try testing.expectEqual(@as(u8, 1), ErrorCode.spec_no_name.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.spec_name_not_string.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.spec_name_empty.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.spec_name_bad_chars.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.spec_name_bad_form.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.spec_name_reserved.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.spec_unknown_key.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.spec_duplicate_name.exitCode());

    // Slot -> 1
    try testing.expectEqual(@as(u8, 1), ErrorCode.slot_value_not_list.exitCode());

    // Obligation -> 1
    try testing.expectEqual(@as(u8, 1), ErrorCode.obligation_bad_value_shape.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.obligation_missing_normativity_or_ref.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.obligation_guard_without_normativity.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.obligation_duplicate_reference.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.obligation_duplicate_normativity.exitCode());

    // Normativity -> 1
    try testing.expectEqual(@as(u8, 1), ErrorCode.normativity_unknown.exitCode());

    // Reference -> 1
    try testing.expectEqual(@as(u8, 1), ErrorCode.reference_unknown_relation.exitCode());

    // Ref -> 1
    try testing.expectEqual(@as(u8, 1), ErrorCode.ref_malformed.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.ref_unknown_slot.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.ref_slot_not_declared.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.ref_spec_not_found_same_file.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.ref_file_not_found.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.ref_file_not_parseable.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.ref_spec_not_found_other_file.exitCode());

    // Query mixed exit codes
    try testing.expectEqual(@as(u8, 2), ErrorCode.query_name_missing.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.query_name_blank.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.query_no_match.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.query_conforms_unresolved.exitCode());
    try testing.expectEqual(@as(u8, 1), ErrorCode.query_conforms_no_slot.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.query_scope_not_found.exitCode());
    try testing.expectEqual(@as(u8, 2), ErrorCode.query_scope_empty.exitCode());

    // Internal -> 1
    try testing.expectEqual(@as(u8, 1), ErrorCode.internal_uncaught.exitCode());
}

test "formatErrorLine with no file uses 'yass'" {
    const result = try formatErrorLine(
        testing.allocator,
        null,
        null,
        .yaml_malformed,
        "unexpected token",
        "/home/user/project",
    );
    defer testing.allocator.free(result);
    try testing.expectEqualStrings("yass: [yass.yaml.malformed] unexpected token", result);
}

test "formatErrorLine with line number" {
    const result = try formatErrorLine(
        testing.allocator,
        "/home/user/project/specs/auth.yaml",
        42,
        .yaml_duplicate_key,
        "key 'name' already defined",
        "/home/user/project",
    );
    defer testing.allocator.free(result);
    try testing.expectEqualStrings("specs/auth.yaml:42: [yass.yaml.duplicate_key] key 'name' already defined", result);
}

test "formatErrorLine without line number" {
    const result = try formatErrorLine(
        testing.allocator,
        "/home/user/project/specs/auth.yaml",
        null,
        .path_not_found,
        "file does not exist",
        "/home/user/project",
    );
    defer testing.allocator.free(result);
    try testing.expectEqualStrings("specs/auth.yaml: [yass.path.not_found] file does not exist", result);
}

test "formatErrorLine file directly in cwd shows basename" {
    const result = try formatErrorLine(
        testing.allocator,
        "/home/user/project/auth.yaml",
        null,
        .yaml_empty_file,
        "file is empty",
        "/home/user/project",
    );
    defer testing.allocator.free(result);
    try testing.expectEqualStrings("auth.yaml: [yass.yaml.empty_file] file is empty", result);
}

test "formatErrorLine file not under cwd shows absolute path" {
    const result = try formatErrorLine(
        testing.allocator,
        "/other/location/spec.yaml",
        10,
        .spec_no_name,
        "missing spec name",
        "/home/user/project",
    );
    defer testing.allocator.free(result);
    try testing.expectEqualStrings("/other/location/spec.yaml:10: [yass.spec.no_name] missing spec name", result);
}

test "formatErrorLine replaces newlines in message" {
    const result = try formatErrorLine(
        testing.allocator,
        null,
        null,
        .internal_uncaught,
        "line one\nline two\nline three",
        "/home/user",
    );
    defer testing.allocator.free(result);
    try testing.expectEqualStrings("yass: [yass.internal.uncaught] line one line two line three", result);
}

test "formatErrorLine replaces CRLF in message" {
    const result = try formatErrorLine(
        testing.allocator,
        null,
        null,
        .internal_uncaught,
        "line one\r\nline two\r\nline three",
        "/home/user",
    );
    defer testing.allocator.free(result);
    try testing.expectEqualStrings("yass: [yass.internal.uncaught] line one line two line three", result);
}

test "formatErrorLine replaces CR in message" {
    const result = try formatErrorLine(
        testing.allocator,
        null,
        null,
        .internal_uncaught,
        "line one\rline two",
        "/home/user",
    );
    defer testing.allocator.free(result);
    try testing.expectEqualStrings("yass: [yass.internal.uncaught] line one line two", result);
}

test "formatErrorLine nested subdirectory under cwd" {
    const result = try formatErrorLine(
        testing.allocator,
        "/home/user/project/a/b/c/deep.yaml",
        1,
        .yaml_has_bom,
        "file contains BOM",
        "/home/user/project",
    );
    defer testing.allocator.free(result);
    try testing.expectEqualStrings("a/b/c/deep.yaml:1: [yass.yaml.has_bom] file contains BOM", result);
}

test "formatErrorLine cwd with trailing slash" {
    const result = try formatErrorLine(
        testing.allocator,
        "/home/user/project/specs/auth.yaml",
        null,
        .path_unreadable,
        "permission denied",
        "/home/user/project/",
    );
    defer testing.allocator.free(result);
    try testing.expectEqualStrings("specs/auth.yaml: [yass.path.unreadable] permission denied", result);
}

test "sanitizeMessage handles empty message" {
    var buf: [256]u8 = undefined;
    const result = sanitizeMessage(&buf, "");
    try testing.expectEqualStrings("", result);
}

test "sanitizeMessage preserves message without newlines" {
    var buf: [256]u8 = undefined;
    const result = sanitizeMessage(&buf, "no newlines here");
    try testing.expectEqualStrings("no newlines here", result);
}

test "top-level convenience aliases work" {
    try testing.expectEqualStrings("yass.exit.success", errorCodeString(.exit_success));
    try testing.expectEqual(@as(u8, 0), exitCode(.exit_success));
}

test "every enum variant is exhaustively covered by string()" {
    // This test ensures that if a new variant is added to ErrorCode,
    // the string() switch must be updated (the compiler enforces this,
    // but this test confirms at runtime that every variant returns non-empty).
    const fields = @typeInfo(ErrorCode).@"enum".fields;
    inline for (fields) |field| {
        const code: ErrorCode = @enumFromInt(field.value);
        const s = code.string();
        try testing.expect(s.len > 0);
        // All codes start with "yass."
        try testing.expect(std.mem.startsWith(u8, s, "yass."));
    }
}

test "every enum variant is exhaustively covered by exitCode()" {
    const fields = @typeInfo(ErrorCode).@"enum".fields;
    inline for (fields) |field| {
        const code: ErrorCode = @enumFromInt(field.value);
        const ec = code.exitCode();
        // Exit codes should be one of the valid values
        try testing.expect(ec == 0 or ec == 1 or ec == 2 or ec == 130 or ec == 143);
    }
}
