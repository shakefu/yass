const std = @import("std");
const c = @cImport({
    @cInclude("yaml.h");
});

pub const errors_mod = @import("errors.zig");
pub const yaml_parse = @import("yaml_parse.zig");
pub const shared = @import("shared.zig");
pub const validate_mod = @import("validate.zig");
pub const list_mod = @import("list.zig");
pub const query_mod = @import("query.zig");

const ErrorCode = errors_mod.ErrorCode;
const File = std.Io.File;
const Io = std.Io;
const Dir = std.Io.Dir;
const Allocator = std.mem.Allocator;
const posix = std.posix;

const version = std.mem.trimEnd(u8, @embedFile("VERSION"), "\n\r ");

const usage_text =
    \\Usage: yass <command> [arguments]
    \\
    \\Commands:
    \\  validate  Validate .yass.yaml files
    \\  list      List specs in .yass.yaml files
    \\  query     Query a spec by name
    \\
    \\Global flags:
    \\  --help     Show this help
    \\  --version  Show version
    \\
;

const subcommand_names = [_][]const u8{ "validate", "list", "query" };
const known_flag_names = [_][]const u8{ "help", "version" };

// ---------------------------------------------------------------------------
// Main entry
// ---------------------------------------------------------------------------

pub fn main(init: std.process.Init) u8 {
    installSignalHandlers();

    const io = init.io;
    const allocator = init.arena.allocator();
    const stdout = File.stdout();
    const stderr = File.stderr();

    // Get CWD
    var cwd_buf: [4096]u8 = undefined;
    const cwd_len = std.process.currentPath(io, &cwd_buf) catch {
        writeErr(stderr, io, "yass: [yass.internal.uncaught] internal error: cannot determine cwd\n");
        return 1;
    };
    const cwd = cwd_buf[0..cwd_len];

    // Parse arguments
    var args_it = init.minimal.args.iterate();
    _ = args_it.next(); // skip argv[0]

    var positionals: std.ArrayListAligned([]const u8, null) = .empty;
    defer positionals.deinit(allocator);
    var found_help = false;
    var found_version = false;
    var found_end_of_opts = false;

    while (args_it.next()) |arg| {
        if (!found_end_of_opts and std.mem.eql(u8, arg, "--")) {
            found_end_of_opts = true;
            continue;
        }
        if (!found_end_of_opts and std.mem.eql(u8, arg, "--help")) {
            found_help = true;
            continue;
        }
        if (!found_end_of_opts and std.mem.eql(u8, arg, "--version")) {
            found_version = true;
            continue;
        }
        positionals.append(allocator, arg) catch {
            writeErr(stderr, io, "yass: [yass.internal.uncaught] internal error: out of memory\n");
            return 1;
        };
    }

    // --help anywhere in argv
    if (found_help) {
        writeOut(stdout, io, usage_text);
        return 0;
    }
    // --version anywhere in argv
    if (found_version) {
        var buf: [128]u8 = undefined;
        const line = std.fmt.bufPrint(&buf, "yass {s}\n", .{version}) catch "yass 0.0.0\n";
        writeOut(stdout, io, line);
        return 0;
    }

    if (positionals.items.len == 0) {
        writeErr(stderr, io, "yass: [yass.argv.no_subcommand] no subcommand given\n");
        writeErr(stderr, io, usage_text);
        return 2;
    }

    const subcmd_arg = positionals.items[0];
    const rest = positionals.items[1..];

    // Validate subcmd_arg
    if (subcmd_arg.len == 0) {
        writeErr(stderr, io, "yass: [yass.argv.empty_argument] empty argument\n");
        writeErr(stderr, io, usage_text);
        return 2;
    }
    if (std.mem.eql(u8, subcmd_arg, "-")) {
        writeErr(stderr, io, "yass: [yass.argv.stdin_dash] stdin marker `-` is not supported; pass a file path\n");
        writeErr(stderr, io, usage_text);
        return 2;
    }
    if (subcmd_arg.len > 1 and subcmd_arg[0] == '-' and subcmd_arg[1] != '-') {
        var buf: [256]u8 = undefined;
        const msg = std.fmt.bufPrint(&buf, "yass: [yass.argv.short_flag] short-form flags are not supported in v1: {s}\n", .{subcmd_arg}) catch "yass: [yass.argv.short_flag] short flag\n";
        writeErr(stderr, io, msg);
        writeErr(stderr, io, usage_text);
        return 2;
    }
    if (subcmd_arg.len > 2 and subcmd_arg[0] == '-' and subcmd_arg[1] == '-') {
        // Check case mismatch on flags (e.g. --Help, --VERSION)
        if (isCaseMismatchFlag(subcmd_arg[2..])) {
            var flagbuf: [256]u8 = undefined;
            const flagmsg = std.fmt.bufPrint(&flagbuf, "yass: [yass.argv.case_mismatch] flag case mismatch: {s}\n", .{subcmd_arg}) catch "yass: [yass.argv.case_mismatch] case mismatch\n";
            writeErr(stderr, io, flagmsg);
            writeErr(stderr, io, usage_text);
            return 2;
        }
        var buf: [256]u8 = undefined;
        const msg = std.fmt.bufPrint(&buf, "yass: [yass.argv.unknown_flag] unknown flag: {s}\n", .{subcmd_arg}) catch "yass: [yass.argv.unknown_flag] unknown flag\n";
        writeErr(stderr, io, msg);
        writeErr(stderr, io, usage_text);
        return 2;
    }

    // Exact match
    for (subcommand_names) |cmd| {
        if (std.mem.eql(u8, subcmd_arg, cmd))
            return dispatch(allocator, io, cmd, rest, cwd, stdout, stderr, init);
    }
    // Case mismatch
    for (subcommand_names) |cmd| {
        if (std.ascii.eqlIgnoreCase(subcmd_arg, cmd) and !std.mem.eql(u8, subcmd_arg, cmd)) {
            var buf: [256]u8 = undefined;
            const msg = std.fmt.bufPrint(&buf, "yass: [yass.argv.case_mismatch] subcommand or flag case mismatch: {s}\n", .{subcmd_arg}) catch "";
            writeErr(stderr, io, msg);
            writeErr(stderr, io, usage_text);
            return 2;
        }
    }
    // Abbreviation
    for (subcommand_names) |cmd| {
        if (cmd.len > subcmd_arg.len and std.mem.startsWith(u8, cmd, subcmd_arg)) {
            var buf: [256]u8 = undefined;
            const msg = std.fmt.bufPrint(&buf, "yass: [yass.argv.abbreviation] abbreviations are not supported: {s}\n", .{subcmd_arg}) catch "";
            writeErr(stderr, io, msg);
            writeErr(stderr, io, usage_text);
            return 2;
        }
    }
    // Unknown
    {
        var buf: [256]u8 = undefined;
        const msg = std.fmt.bufPrint(&buf, "yass: [yass.argv.unknown_subcommand] unknown subcommand: {s}\n", .{subcmd_arg}) catch "";
        writeErr(stderr, io, msg);
        writeErr(stderr, io, usage_text);
        return 2;
    }
}

// ---------------------------------------------------------------------------
// Dispatch
// ---------------------------------------------------------------------------

fn dispatch(
    allocator: Allocator,
    io: Io,
    subcmd: []const u8,
    args: []const []const u8,
    cwd: []const u8,
    stdout: File,
    stderr: File,
    init: std.process.Init,
) u8 {
    // Validate args for all subcommands
    for (args) |arg| {
        if (arg.len == 0) {
            writeErr(stderr, io, "yass: [yass.argv.empty_argument] empty argument\n");
            return 2;
        }
        if (std.mem.eql(u8, arg, "-")) {
            writeErr(stderr, io, "yass: [yass.argv.stdin_dash] stdin marker `-` is not supported; pass a file path\n");
            return 2;
        }
        if (arg.len > 1 and arg[0] == '-' and arg[1] != '-') {
            var buf: [256]u8 = undefined;
            const msg = std.fmt.bufPrint(&buf, "yass: [yass.argv.short_flag] short-form flags are not supported in v1: {s}\n", .{arg}) catch "";
            writeErr(stderr, io, msg);
            return 2;
        }
        if (arg.len > 2 and arg[0] == '-' and arg[1] == '-') {
            if (isCaseMismatchFlag(arg[2..])) {
                var flagbuf2: [256]u8 = undefined;
                const flagmsg2 = std.fmt.bufPrint(&flagbuf2, "yass: [yass.argv.case_mismatch] flag case mismatch: {s}\n", .{arg}) catch "";
                writeErr(stderr, io, flagmsg2);
                return 2;
            }
            var buf: [256]u8 = undefined;
            const msg = std.fmt.bufPrint(&buf, "yass: [yass.argv.unknown_flag] unknown flag: {s}\n", .{arg}) catch "";
            writeErr(stderr, io, msg);
            return 2;
        }
        if (std.mem.indexOfScalar(u8, arg, ':') != null) {
            var buf: [512]u8 = undefined;
            const msg = std.fmt.bufPrint(&buf, "yass: [yass.path.colon_in_path] path contains an unsupported colon character: {s}\n", .{arg}) catch "";
            writeErr(stderr, io, msg);
            return 2;
        }
    }

    if (std.mem.eql(u8, subcmd, "validate")) return runValidate(allocator, io, args, cwd, stdout, stderr);
    if (std.mem.eql(u8, subcmd, "list")) return runList(allocator, io, args, cwd, stdout, stderr, init);
    if (std.mem.eql(u8, subcmd, "query")) return runQuery(allocator, io, args, cwd, stdout, stderr);
    return 1;
}

// ---------------------------------------------------------------------------
// validate
// ---------------------------------------------------------------------------

fn runValidate(allocator: Allocator, io: Io, args: []const []const u8, cwd: []const u8, stdout: File, stderr: File) u8 {
    const files = discoverFiles(allocator, io, args, cwd, stderr) orelse return 2;

    if (files.len == 0) {
        writeErr(stderr, io, "yass: [yass.discover.no_files] no .yass.yaml files found\n");
        return 2;
    }

    const project_root = shared.findProjectRoot(allocator, io, cwd) catch null orelse cwd;
    const result = validate_mod.validate(allocator, io, files, project_root, cwd) catch {
        writeErr(stderr, io, "yass: [yass.internal.uncaught] internal error: validation failed\n");
        return 1;
    };

    for (result.errors) |ve| {
        const line = errors_mod.formatErrorLine(allocator, ve.file, ve.line, ve.code, ve.message, cwd) catch continue;
        defer allocator.free(line);
        var buf: [8192]u8 = undefined;
        const msg = std.fmt.bufPrint(&buf, "{s}\n", .{line}) catch continue;
        writeErr(stderr, io, msg);
    }

    var summary_buf: [256]u8 = undefined;
    const summary = std.fmt.bufPrint(&summary_buf, "checked {d} files, found {d} errors\n", .{ result.file_count, result.error_count }) catch "checked 0 files, found 0 errors\n";
    writeOut(stdout, io, summary);

    return if (result.error_count > 0) 1 else 0;
}

// ---------------------------------------------------------------------------
// list
// ---------------------------------------------------------------------------

fn runList(allocator: Allocator, io: Io, args: []const []const u8, cwd: []const u8, stdout: File, stderr: File, init: std.process.Init) u8 {
    const files = discoverFilesForList(allocator, io, args, cwd, stderr) orelse return 2;
    if (files.len == 0) return 0;

    const tw: ?u16 = getTerminalWidth(io, stdout, init);
    const result = list_mod.listSpecs(allocator, io, files, cwd, tw);

    for (result.rows) |row| {
        var buf: [8192]u8 = undefined;
        const line = std.fmt.bufPrint(&buf, "{s}\t{s}\t{s}\n", .{ row.file, row.spec_name, row.description }) catch continue;
        writeOut(stdout, io, line);
    }
    for (result.errors) |le| {
        var buf: [4096]u8 = undefined;
        const msg = std.fmt.bufPrint(&buf, "{s}: [{s}] {s}\n", .{ le.file, le.code.string(), le.message }) catch continue;
        writeErr(stderr, io, msg);
    }
    return if (result.has_parse_errors) 1 else 0;
}

// ---------------------------------------------------------------------------
// query
// ---------------------------------------------------------------------------

fn runQuery(allocator: Allocator, io: Io, args: []const []const u8, cwd: []const u8, stdout: File, stderr: File) u8 {
    if (args.len == 0) {
        writeErr(stderr, io, "yass: [yass.query.name_missing] missing spec name\n");
        return 2;
    }
    const spec_name = args[0];
    if (spec_name.len == 0) {
        writeErr(stderr, io, "yass: [yass.query.name_blank] spec name is blank or contains whitespace\n");
        return 2;
    }

    const scope_arg: ?[]const u8 = if (args.len > 1) args[1] else null;

    // Find scope
    const scope_path = scope_arg orelse blk: {
        const pr = shared.findProjectRoot(allocator, io, cwd) catch {
            writeErr(stderr, io, "yass: [yass.internal.uncaught] internal error: filesystem error\n");
            return 1;
        } orelse {
            writeErr(stderr, io, "yass: [yass.findroot.no_marker] no project root marker found\n");
            return 2;
        };
        break :blk pr;
    };

    // Validate scope
    if (scope_arg != null) {
        _ = Dir.cwd().statFile(io, scope_path, .{}) catch {
            var buf: [512]u8 = undefined;
            const msg = std.fmt.bufPrint(&buf, "yass: [yass.query.scope_not_found] scope path does not exist: {s}\n", .{scope_path}) catch "";
            writeErr(stderr, io, msg);
            return 2;
        };
    }

    var scope_files: std.ArrayListAligned([]const u8, null) = .empty;
    defer scope_files.deinit(allocator);
    const discover = shared.discoverSpecFiles(allocator, io, scope_path, cwd) catch return 1;
    for (discover.errors) |e| {
        var buf: [1024]u8 = undefined;
        const msg = std.fmt.bufPrint(&buf, "{s}: [{s}] error\n", .{ shared.relativePath(e.path, cwd), @tagName(e.code) }) catch "";
        writeErr(stderr, io, msg);
        return 2;
    }
    for (discover.files) |f| scope_files.append(allocator, f) catch {};

    if (scope_arg != null and scope_files.items.len == 0) {
        var buf: [512]u8 = undefined;
        const msg = std.fmt.bufPrint(&buf, "yass: [yass.query.scope_empty] no .yass.yaml files found in scope: {s}\n", .{scope_path}) catch "";
        writeErr(stderr, io, msg);
        return 2;
    }

    const project_root = shared.findProjectRoot(allocator, io, cwd) catch null orelse cwd;
    const result = query_mod.querySpec(allocator, io, spec_name, scope_files.items, project_root, cwd) catch {
        writeErr(stderr, io, "yass: [yass.internal.uncaught] internal error: query failed\n");
        return 1;
    };

    switch (result) {
        .single_match => |sm| {
            writeOut(stdout, io, sm.fragment);
            for (sm.errors) |qe| {
                var buf: [4096]u8 = undefined;
                const msg = std.fmt.bufPrint(&buf, "yass: [{s}] {s}\n", .{ qe.code.string(), qe.message }) catch continue;
                writeErr(stderr, io, msg);
            }
            return if (sm.errors.len > 0) 1 else 0;
        },
        .multi_match => |rows| {
            for (rows) |row| {
                var buf: [8192]u8 = undefined;
                const line = std.fmt.bufPrint(&buf, "{s}\t{s}\n", .{ row.file, row.spec_name }) catch continue;
                writeOut(stdout, io, line);
            }
            return 0;
        },
        .no_match => {
            var buf: [512]u8 = undefined;
            const msg = std.fmt.bufPrint(&buf, "yass: [yass.query.no_match] no spec matches: {s}\n", .{spec_name}) catch "";
            writeErr(stderr, io, msg);
            return 1;
        },
        .err => |e| {
            var buf: [512]u8 = undefined;
            const msg = std.fmt.bufPrint(&buf, "yass: [{s}] {s}\n", .{ e.code.string(), e.message }) catch "";
            writeErr(stderr, io, msg);
            return e.code.exitCode();
        },
    }
}

// ---------------------------------------------------------------------------
// File discovery helpers
// ---------------------------------------------------------------------------

fn discoverFiles(allocator: Allocator, io: Io, args: []const []const u8, cwd: []const u8, stderr: File) ?[][]const u8 {
    var all: std.ArrayListAligned([]const u8, null) = .empty;

    if (args.len == 0) {
        const root = shared.findProjectRoot(allocator, io, cwd) catch {
            writeErr(stderr, io, "yass: [yass.internal.uncaught] internal error: filesystem\n");
            return null;
        } orelse {
            writeErr(stderr, io, "yass: [yass.findroot.no_marker] no project root marker found\n");
            return null;
        };
        defer allocator.free(root);
        const d = shared.discoverSpecFiles(allocator, io, root, cwd) catch return null;
        for (d.errors) |e| {
            var buf: [1024]u8 = undefined;
            const msg = std.fmt.bufPrint(&buf, "{s}: [{s}] error\n", .{ shared.relativePath(e.path, cwd), @tagName(e.code) }) catch "";
            writeErr(stderr, io, msg);
        }
        for (d.files) |f| all.append(allocator, f) catch {};
    } else {
        for (args) |arg| {
            if (shared.hasGlobMetachars(arg)) {
                const expanded = shared.expandGlob(allocator, io, arg) catch {
                    var buf: [512]u8 = undefined;
                    const msg = std.fmt.bufPrint(&buf, "yass: [yass.glob.no_match] no files matched pattern: {s}\n", .{arg}) catch "";
                    writeErr(stderr, io, msg);
                    return null;
                };
                for (expanded) |path| {
                    const bn = std.fs.path.basename(path);
                    if (isYassYamlFile(bn)) all.append(allocator, path) catch {};
                }
            } else {
                const d = shared.discoverSpecFiles(allocator, io, arg, cwd) catch return null;
                for (d.errors) |e| {
                    var buf: [1024]u8 = undefined;
                    const msg = std.fmt.bufPrint(&buf, "{s}: [{s}] error\n", .{ shared.relativePath(e.path, cwd), @tagName(e.code) }) catch "";
                    writeErr(stderr, io, msg);
                    return null;
                }
                for (d.files) |f| all.append(allocator, f) catch {};
            }
        }
    }
    return all.toOwnedSlice(allocator) catch null;
}

fn discoverFilesForList(allocator: Allocator, io: Io, args: []const []const u8, cwd: []const u8, stderr: File) ?[][]const u8 {
    return discoverFiles(allocator, io, args, cwd, stderr);
}

fn getTerminalWidth(io: Io, stdout_file: File, init: std.process.Init) ?u16 {
    const is_tty = stdout_file.isTty(io) catch return null;
    if (!is_tty) return null;
    if (init.environ_map.get("COLUMNS")) |cols_str| {
        const cols = std.fmt.parseInt(u16, cols_str, 10) catch 0;
        if (cols > 0) return cols;
    }
    return 80;
}

// ---------------------------------------------------------------------------
// Signal handling
// ---------------------------------------------------------------------------

fn installSignalHandlers() void {
    // SIGPIPE -> exit 0
    const pipe_act = posix.Sigaction{
        .handler = .{ .handler = handleSigpipe },
        .mask = posix.sigemptyset(),
        .flags = 0,
    };
    posix.sigaction(posix.SIG.PIPE, &pipe_act, null);

    // SIGINT -> exit 130
    const int_act = posix.Sigaction{
        .handler = .{ .handler = handleSigint },
        .mask = posix.sigemptyset(),
        .flags = 0,
    };
    posix.sigaction(posix.SIG.INT, &int_act, null);

    // SIGTERM -> exit 143
    const term_act = posix.Sigaction{
        .handler = .{ .handler = handleSigterm },
        .mask = posix.sigemptyset(),
        .flags = 0,
    };
    posix.sigaction(posix.SIG.TERM, &term_act, null);
}

fn handleSigpipe(_: posix.SIG) callconv(.c) void {
    std.process.exit(0);
}

fn handleSigint(_: posix.SIG) callconv(.c) void {
    std.process.exit(130);
}

fn handleSigterm(_: posix.SIG) callconv(.c) void {
    std.process.exit(143);
}

// ---------------------------------------------------------------------------
// IO helpers
// ---------------------------------------------------------------------------

fn writeOut(f: File, io: Io, data: []const u8) void {
    f.writeStreamingAll(io, data) catch {};
}

fn writeErr(f: File, io: Io, data: []const u8) void {
    f.writeStreamingAll(io, data) catch {};
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test {
    _ = errors_mod;
    _ = yaml_parse;
    _ = shared;
    _ = validate_mod;
    _ = list_mod;
    _ = query_mod;
}

test "libyaml initializes" {
    var parser: c.yaml_parser_t = undefined;
    try std.testing.expect(c.yaml_parser_initialize(&parser) != 0);
    c.yaml_parser_delete(&parser);
}

test "version string" {
    try std.testing.expectEqualStrings("0.0.3", version);
}

// Pure-logic arg parsing helpers for testing
fn isSubcommand(arg: []const u8) bool {
    for (subcommand_names) |cmd| {
        if (std.mem.eql(u8, arg, cmd)) return true;
    }
    return false;
}

fn isShortFlag(arg: []const u8) bool {
    return arg.len > 1 and arg[0] == '-' and arg[1] != '-';
}

fn isCaseMismatch(arg: []const u8) bool {
    for (subcommand_names) |cmd| {
        if (std.ascii.eqlIgnoreCase(arg, cmd) and !std.mem.eql(u8, arg, cmd)) return true;
    }
    return false;
}

/// Check whether `name` is a valid .yass.yaml filename.
fn isYassYamlFile(name: []const u8) bool {
    const suffix = ".yass.yaml";
    if (!std.mem.endsWith(u8, name, suffix)) return false;
    if (name.len <= suffix.len) return false;
    return true;
}

fn isCaseMismatchFlag(flag_name: []const u8) bool {
    for (known_flag_names) |kf| {
        if (std.ascii.eqlIgnoreCase(flag_name, kf) and !std.mem.eql(u8, flag_name, kf)) return true;
    }
    return false;
}

fn isAbbreviation(arg: []const u8) bool {
    if (arg.len == 0) return false;
    for (subcommand_names) |cmd| {
        if (cmd.len > arg.len and std.mem.startsWith(u8, cmd, arg)) return true;
    }
    return false;
}

test "isSubcommand" {
    try std.testing.expect(isSubcommand("validate"));
    try std.testing.expect(isSubcommand("list"));
    try std.testing.expect(isSubcommand("query"));
    try std.testing.expect(!isSubcommand("unknown"));
    try std.testing.expect(!isSubcommand("Validate"));
}

test "isShortFlag" {
    try std.testing.expect(isShortFlag("-h"));
    try std.testing.expect(isShortFlag("-v"));
    try std.testing.expect(!isShortFlag("--help"));
    try std.testing.expect(!isShortFlag("-"));
}

test "isCaseMismatch" {
    try std.testing.expect(isCaseMismatch("Validate"));
    try std.testing.expect(isCaseMismatch("LIST"));
    try std.testing.expect(!isCaseMismatch("validate"));
    try std.testing.expect(!isCaseMismatch("unknown"));
}

test "isAbbreviation" {
    try std.testing.expect(isAbbreviation("val"));
    try std.testing.expect(isAbbreviation("lis"));
    try std.testing.expect(isAbbreviation("q"));
    try std.testing.expect(isAbbreviation("v"));
    try std.testing.expect(isAbbreviation("valid"));
    try std.testing.expect(!isAbbreviation("validate"));
    try std.testing.expect(!isAbbreviation("xyz"));
    try std.testing.expect(!isAbbreviation(""));
}

test "isCaseMismatchFlag detects flag case variants" {
    try std.testing.expect(isCaseMismatchFlag("Help"));
    try std.testing.expect(isCaseMismatchFlag("HELP"));
    try std.testing.expect(isCaseMismatchFlag("Version"));
    try std.testing.expect(isCaseMismatchFlag("VERSION"));
}

test "isCaseMismatchFlag rejects exact matches" {
    try std.testing.expect(!isCaseMismatchFlag("help"));
    try std.testing.expect(!isCaseMismatchFlag("version"));
}

test "isCaseMismatchFlag rejects unrelated flags" {
    try std.testing.expect(!isCaseMismatchFlag("verbose"));
    try std.testing.expect(!isCaseMismatchFlag("debug"));
}

test "isYassYamlFile valid names" {
    try std.testing.expect(isYassYamlFile("example.yass.yaml"));
    try std.testing.expect(isYassYamlFile("foo.bar.yass.yaml"));
}

test "isYassYamlFile rejects bare suffix" {
    try std.testing.expect(!isYassYamlFile(".yass.yaml"));
}

test "isYassYamlFile rejects wrong suffix" {
    try std.testing.expect(!isYassYamlFile("example.yaml"));
    try std.testing.expect(!isYassYamlFile("example.txt"));
}

test "usage text starts with Usage:" {
    try std.testing.expect(std.mem.startsWith(u8, usage_text, "Usage:"));
}

test "usage text ends with newline" {
    try std.testing.expect(usage_text[usage_text.len - 1] == '\n');
}
