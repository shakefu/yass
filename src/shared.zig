const std = @import("std");
const Allocator = std.mem.Allocator;
const Dir = std.Io.Dir;
const File = std.Io.File;
const Io = std.Io;
const path = std.fs.path;

// ---------------------------------------------------------------------------
// Path helpers
// ---------------------------------------------------------------------------

/// Open a directory, handling both absolute and relative paths.
pub fn openDirAny(io: Io, dir_path: []const u8, options: Dir.OpenOptions) Dir.OpenError!Dir {
    if (std.fs.path.isAbsolute(dir_path)) {
        return Dir.openDirAbsolute(io, dir_path, options);
    } else {
        return Dir.cwd().openDir(io, dir_path, options);
    }
}

// ---------------------------------------------------------------------------
// Error codes
// ---------------------------------------------------------------------------

pub const ErrorCode = enum {
    /// No project root marker found.
    @"yass.findroot.no_marker",
    /// Path does not exist on the filesystem.
    @"yass.path.not_found",
    /// File does not have the required .yass.yaml suffix.
    @"yass.path.bad_extension",
    /// Path exists but cannot be read.
    @"yass.path.unreadable",
    /// Glob pattern matched zero files.
    @"yass.glob.no_match",
};

pub const ErrorInfo = struct {
    path: []const u8,
    code: ErrorCode,
};

// ---------------------------------------------------------------------------
// DiscoverResult
// ---------------------------------------------------------------------------

pub const DiscoverResult = struct {
    files: [][]const u8,
    errors: []ErrorInfo,
};

// ---------------------------------------------------------------------------
// 1. findProjectRoot
// ---------------------------------------------------------------------------

/// Walk upward from `start_path` looking for a project root marker.
///
/// First pass: look for a `.git` entry (file or directory) in each ancestor.
/// Second pass (only if no `.git` found): look for any `.yass.yaml` file.
///
/// Returns the absolute path of the directory containing the marker, or `null`
/// if no marker is found.  The returned slice is allocated with `allocator`.
pub fn findProjectRoot(allocator: Allocator, io: Io, start_path: []const u8) !?[]const u8 {
    // First pass: look for .git
    var cur: ?[]const u8 = start_path;
    while (cur) |dir_path| {
        if (checkEntryExists(io, dir_path, ".git")) {
            return try allocator.dupe(u8, dir_path);
        }
        cur = path.dirname(dir_path);
    }

    // Second pass: look for any .yass.yaml file
    cur = start_path;
    while (cur) |dir_path| {
        if (dirContainsYassYaml(io, dir_path)) {
            return try allocator.dupe(u8, dir_path);
        }
        cur = path.dirname(dir_path);
    }

    return null;
}

/// Check whether `name` exists as a direct child of the directory at
/// `dir_path`.
fn checkEntryExists(io: Io, dir_path: []const u8, name: []const u8) bool {
    const dir = openDirAny(io, dir_path, .{}) catch return false;
    defer dir.close(io);
    dir.access(io, name, .{}) catch return false;
    return true;
}

/// Return `true` when `dir_path` contains at least one file whose name ends
/// with `.yass.yaml` (and whose basename is longer than `.yass.yaml` itself,
/// i.e. bare `.yass.yaml` with no prefix is excluded).
fn dirContainsYassYaml(io: Io, dir_path: []const u8) bool {
    const dir = openDirAny(io, dir_path, .{ .iterate = true }) catch return false;
    defer dir.close(io);
    var iter = dir.iterate();
    while (iter.next(io) catch null) |entry| {
        if (entry.kind != .file and entry.kind != .sym_link) continue;
        if (isYassYamlFile(entry.name)) return true;
    }
    return false;
}

// ---------------------------------------------------------------------------
// 2. discoverSpecFiles
// ---------------------------------------------------------------------------

/// Discover `.yass.yaml` spec files.
///
/// * If `file_path` is a regular file, validate its suffix and return it.
/// * If `file_path` is a directory, recursively find all `.yass.yaml` files,
///   skipping hidden directories and hidden files, without following symlinks.
/// * Results are sorted by Unicode code-point order on the full relative path.
/// * Paths are formatted relative to `cwd`.
pub fn discoverSpecFiles(allocator: Allocator, io: Io, file_path: []const u8, cwd: []const u8) !DiscoverResult {
    var files: std.ArrayList([]const u8) = .empty;
    var errors: std.ArrayList(ErrorInfo) = .empty;

    // Stat the path (following symlinks for the argument itself).
    const stat_result = Dir.cwd().statFile(io, file_path, .{ .follow_symlinks = true });
    const stat = stat_result catch |err| {
        switch (err) {
            error.FileNotFound => {
                try errors.append(allocator, .{
                    .path = try allocator.dupe(u8, file_path),
                    .code = .@"yass.path.not_found",
                });
            },
            else => {
                try errors.append(allocator, .{
                    .path = try allocator.dupe(u8, file_path),
                    .code = .@"yass.path.unreadable",
                });
            },
        }
        return .{
            .files = try files.toOwnedSlice(allocator),
            .errors = try errors.toOwnedSlice(allocator),
        };
    };

    if (stat.kind == .directory) {
        // Recursive walk
        try walkDirectory(allocator, io, file_path, cwd, &files, &errors);
    } else {
        // Single file
        const bname = path.basename(file_path);
        if (!isYassYamlFile(bname)) {
            try errors.append(allocator, .{
                .path = try allocator.dupe(u8, file_path),
                .code = .@"yass.path.bad_extension",
            });
        } else {
            const rel = try allocator.dupe(u8, relativePath(file_path, cwd));
            try files.append(allocator, rel);
        }
    }

    // Sort files by code-point order
    std.mem.sort([]const u8, files.items, {}, struct {
        fn lessThan(_: void, a: []const u8, b: []const u8) bool {
            return std.mem.order(u8, a, b) == .lt;
        }
    }.lessThan);

    return .{
        .files = try files.toOwnedSlice(allocator),
        .errors = try errors.toOwnedSlice(allocator),
    };
}

/// Recursively walk a directory, collecting .yass.yaml files.
fn walkDirectory(
    allocator: Allocator,
    io: Io,
    dir_path: []const u8,
    cwd: []const u8,
    files: *std.ArrayList([]const u8),
    errors: *std.ArrayList(ErrorInfo),
) !void {
    const dir = openDirAny(io, dir_path, .{ .iterate = true }) catch |err| {
        switch (err) {
            error.FileNotFound => {
                try errors.append(allocator, .{
                    .path = try allocator.dupe(u8, dir_path),
                    .code = .@"yass.path.not_found",
                });
            },
            else => {
                try errors.append(allocator, .{
                    .path = try allocator.dupe(u8, dir_path),
                    .code = .@"yass.path.unreadable",
                });
            },
        }
        return;
    };
    defer dir.close(io);

    var walker = try dir.walkSelectively(allocator);
    defer walker.deinit();

    while (walker.next(io) catch null) |entry| {
        // Skip hidden entries (basename starts with .)
        if (entry.basename.len > 0 and entry.basename[0] == '.') continue;

        if (entry.kind == .directory) {
            // Enter non-hidden, non-symlink directories
            walker.enter(io, entry) catch {};
            continue;
        }

        // Skip symlinks during traversal
        if (entry.kind == .sym_link) continue;

        // Check if it is a .yass.yaml file
        if (entry.kind == .file and isYassYamlFile(entry.basename)) {
            // Build full path
            const full = try path.join(allocator, &.{ dir_path, entry.path });
            const rel = try allocator.dupe(u8, relativePath(full, cwd));
            // Free intermediate full path if different from rel
            if (rel.ptr != full.ptr) allocator.free(full);
            try files.append(allocator, rel);
        }
    }
}

// ---------------------------------------------------------------------------
// 3. relativePath
// ---------------------------------------------------------------------------

/// Format `abs_path` relative to `cwd`.
///
/// * If `abs_path` starts with `cwd` + "/", return the portion after the slash.
/// * If `abs_path` equals `cwd`, return ".".
/// * Otherwise return `abs_path` unchanged (absolute).
/// * Never returns a "./" prefix.
pub fn relativePath(abs_path: []const u8, cwd: []const u8) []const u8 {
    if (std.mem.eql(u8, abs_path, cwd)) return ".";
    if (std.mem.startsWith(u8, abs_path, cwd)) {
        if (abs_path.len > cwd.len and abs_path[cwd.len] == '/') {
            return abs_path[cwd.len + 1 ..];
        }
    }
    return abs_path;
}

// ---------------------------------------------------------------------------
// 4. hasGlobMetachars
// ---------------------------------------------------------------------------

/// Return `true` if `arg` contains any glob metacharacters: `*`, `?`, `[`.
pub fn hasGlobMetachars(arg: []const u8) bool {
    for (arg) |c| {
        switch (c) {
            '*', '?', '[' => return true,
            else => {},
        }
    }
    return false;
}

// ---------------------------------------------------------------------------
// 5. expandGlob
// ---------------------------------------------------------------------------

/// Expand a glob pattern and return matching paths, sorted by code-point order.
///
/// Supports `*` (any non-`/` chars), `?` (single non-`/` char),
/// `[...]` (character class), and `**` (zero or more path segments).
///
/// Hidden files/directories (basename starting with `.`) are skipped.
/// Symlinks are not followed.
/// Case-sensitive matching.
///
/// Returns `error.NoMatch` (mapped to `yass.glob.no_match`) when zero paths
/// match.
pub fn expandGlob(allocator: Allocator, io: Io, pattern: []const u8) ![][]const u8 {
    var results: std.ArrayList([]const u8) = .empty;

    // Split pattern into base directory (literal prefix) and the glob tail.
    const split = splitGlobPattern(pattern);

    const base_dir_path = if (split.base.len == 0) "." else split.base;
    const glob_tail = split.glob;

    const dir = openDirAny(io, base_dir_path, .{ .iterate = true }) catch {
        if (results.items.len == 0) {
            results.deinit(allocator);
            return error.NoMatch;
        }
        return try results.toOwnedSlice(allocator);
    };
    defer dir.close(io);

    var walker = try dir.walkSelectively(allocator);
    defer walker.deinit();

    while (walker.next(io) catch null) |entry| {
        // Skip hidden
        if (entry.basename.len > 0 and entry.basename[0] == '.') continue;

        if (entry.kind == .directory) {
            walker.enter(io, entry) catch {};
            continue;
        }

        // Skip symlinks
        if (entry.kind == .sym_link) continue;

        if (entry.kind == .file) {
            if (globMatch(glob_tail, entry.path)) {
                const full = if (split.base.len == 0)
                    try allocator.dupe(u8, entry.path)
                else
                    try path.join(allocator, &.{ split.base, entry.path });
                try results.append(allocator, full);
            }
        }
    }

    if (results.items.len == 0) {
        results.deinit(allocator);
        return error.NoMatch;
    }

    // Sort by code-point order
    std.mem.sort([]const u8, results.items, {}, struct {
        fn lessThan(_: void, a: []const u8, b: []const u8) bool {
            return std.mem.order(u8, a, b) == .lt;
        }
    }.lessThan);

    return try results.toOwnedSlice(allocator);
}

/// Split a glob pattern into a literal base directory prefix and the
/// remaining glob portion.  For example:
///
///   "src/**/*.zig"  -> base="src", glob="**/*.zig"
///   "**/*.yaml"     -> base="",    glob="**/*.yaml"
///   "/a/b/*.yaml"   -> base="/a/b", glob="*.yaml"
fn splitGlobPattern(pattern: []const u8) struct { base: []const u8, glob: []const u8 } {
    // Find the first metachar
    var first_meta: ?usize = null;
    for (pattern, 0..) |c, i| {
        switch (c) {
            '*', '?', '[' => {
                first_meta = i;
                break;
            },
            else => {},
        }
    }

    if (first_meta) |meta_idx| {
        // Walk backward to the last '/' before the metachar
        var slash_pos: ?usize = null;
        var i: usize = meta_idx;
        while (i > 0) {
            i -= 1;
            if (pattern[i] == '/') {
                slash_pos = i;
                break;
            }
        }
        if (slash_pos) |sp| {
            return .{ .base = pattern[0..sp], .glob = pattern[sp + 1 ..] };
        } else {
            return .{ .base = "", .glob = pattern };
        }
    } else {
        // No metachar - shouldn't happen if hasGlobMetachars was checked first,
        // but handle gracefully: treat entire pattern as literal.
        if (path.dirname(pattern)) |dir| {
            const bname = path.basename(pattern);
            return .{ .base = dir, .glob = bname };
        }
        return .{ .base = "", .glob = pattern };
    }
}

/// Match a path against a glob pattern supporting `*`, `?`, `[...]`, `**`.
///
/// `**` matches zero or more path segments (including the separator).
/// `*` matches any number of non-`/` characters.
/// `?` matches exactly one non-`/` character.
/// `[...]` matches one character in the set; `[!...]` or `[^...]` for negation.
pub fn globMatch(pattern: []const u8, str: []const u8) bool {
    return globMatchInner(pattern, str);
}

fn globMatchInner(pat: []const u8, str: []const u8) bool {
    var pi: usize = 0;
    var si: usize = 0;

    // For backtracking on `*`
    var star_pi: ?usize = null;
    var star_si: usize = 0;

    while (si < str.len or pi < pat.len) {
        if (pi < pat.len) {
            // Handle **
            if (pi + 1 < pat.len and pat[pi] == '*' and pat[pi + 1] == '*') {
                // Consume the **
                var new_pi = pi + 2;
                // Skip trailing slash after **
                if (new_pi < pat.len and pat[new_pi] == '/') {
                    new_pi += 1;
                }
                // ** can match zero or more path segments.
                // Try matching the rest of the pattern at every position.
                var try_si = si;
                while (try_si <= str.len) {
                    if (globMatchInner(pat[new_pi..], str[try_si..])) {
                        return true;
                    }
                    if (try_si >= str.len) break;
                    try_si += 1;
                }
                return false;
            }

            // Handle single *
            if (pat[pi] == '*') {
                star_pi = pi;
                star_si = si;
                pi += 1;
                continue;
            }

            if (si < str.len) {
                // Handle [...]
                if (pat[pi] == '[') {
                    if (matchCharClass(pat[pi..], str[si])) |class_len| {
                        pi += class_len;
                        si += 1;
                        continue;
                    }
                }

                // Handle ?
                if (pat[pi] == '?' and str[si] != '/') {
                    pi += 1;
                    si += 1;
                    continue;
                }

                // Literal match
                if (pat[pi] == str[si]) {
                    pi += 1;
                    si += 1;
                    continue;
                }
            }
        }

        // Backtrack to last *
        if (star_pi) |sp| {
            pi = sp + 1;
            star_si += 1;
            // * cannot match /
            if (star_si <= str.len and (star_si == str.len or str[star_si - 1] != '/')) {
                si = star_si;
                continue;
            } else if (star_si <= str.len and str[star_si - 1] == '/') {
                // * cannot cross path separator
                star_pi = null;
            }
        }

        return false;
    }

    return true;
}

/// Try to match a character class `[...]` at the start of `pat` against `ch`.
/// Returns the length consumed from `pat` (including the closing `]`) on
/// success, or `null` on failure.
fn matchCharClass(pat: []const u8, ch: u8) ?usize {
    if (pat.len == 0 or pat[0] != '[') return null;

    var i: usize = 1;
    var negate = false;

    if (i < pat.len and (pat[i] == '!' or pat[i] == '^')) {
        negate = true;
        i += 1;
    }

    var matched = false;
    // Allow ] as first char in class
    const start = i;

    while (i < pat.len) {
        if (pat[i] == ']' and i > start) {
            // End of class
            if (negate) {
                return if (!matched) i + 1 else null;
            } else {
                return if (matched) i + 1 else null;
            }
        }

        // Range: a-z
        if (i + 2 < pat.len and pat[i + 1] == '-' and pat[i + 2] != ']') {
            const lo = pat[i];
            const hi = pat[i + 2];
            if (ch >= lo and ch <= hi) matched = true;
            i += 3;
        } else {
            if (pat[i] == ch) matched = true;
            i += 1;
        }
    }

    // Unterminated class - no match
    return null;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Check whether `name` is a valid .yass.yaml filename.
/// The name must end with ".yass.yaml" and have at least one character
/// before the suffix (bare ".yass.yaml" does not match).
pub fn isYassYamlFile(name: []const u8) bool {
    const suffix = ".yass.yaml";
    if (!std.mem.endsWith(u8, name, suffix)) return false;
    // Must have a non-empty basename before the suffix
    if (name.len <= suffix.len) return false;
    return true;
}

// ===========================================================================
// Tests
// ===========================================================================

fn getTestIo() std.Io.Threaded {
    return std.Io.Threaded.init(std.testing.allocator, .{});
}

/// Create a directory path (all parents) for testing.
fn testCreateDirPath(io: Io, abs_path: []const u8) !void {
    Dir.cwd().createDirPath(io, abs_path) catch {};
}

/// Create an empty file for testing.
fn testCreateFile(io: Io, abs_path: []const u8) !void {
    const f = try Dir.cwd().createFile(io, abs_path, .{});
    f.close(io);
}

/// Remove a test tree.
fn testCleanup(io: Io, abs_path: []const u8) void {
    Dir.cwd().deleteTree(io, abs_path) catch {};
}

// ---------------------------------------------------------------------------
// findProjectRoot tests
// ---------------------------------------------------------------------------

test "findProjectRoot with .git marker" {
    var threaded = getTestIo();
    const io = threaded.io();
    const base = "/tmp/yass-test-fpr-git";
    testCleanup(io, base);
    try testCreateDirPath(io, base ++ "/sub/deep");
    try testCreateDirPath(io, base ++ "/.git");

    const result = try findProjectRoot(std.testing.allocator, io, base ++ "/sub/deep");
    defer if (result) |r| std.testing.allocator.free(r);

    try std.testing.expect(result != null);
    try std.testing.expectEqualStrings(base, result.?);

    testCleanup(io, base);
}

test "findProjectRoot with .yass.yaml marker" {
    var threaded = getTestIo();
    const io = threaded.io();
    const base = "/tmp/yass-test-fpr-yaml";
    testCleanup(io, base);
    try testCreateDirPath(io, base ++ "/sub/deep");
    try testCreateFile(io, base ++ "/example.yass.yaml");

    const result = try findProjectRoot(std.testing.allocator, io, base ++ "/sub/deep");
    defer if (result) |r| std.testing.allocator.free(r);

    try std.testing.expect(result != null);
    try std.testing.expectEqualStrings(base, result.?);

    testCleanup(io, base);
}

test "findProjectRoot with no marker" {
    var threaded = getTestIo();
    const io = threaded.io();
    const base = "/tmp/yass-test-fpr-none";
    testCleanup(io, base);
    try testCreateDirPath(io, base ++ "/sub/deep");

    // This test walks up to / which has .git in many systems.
    // Use the base itself as start_path so it only checks base/sub/deep, base/sub, base.
    // Since none contain .git or .yass.yaml, it should return null...
    // Actually it will keep going up to /tmp and / so if there is a .git
    // somewhere it will find it. We test with a controlled subpath.
    // We will search only within base - but findProjectRoot walks to /.
    // For a reliable test, we just verify the function does not crash.
    // If your system has .git above /tmp, this may return non-null.
    const result = try findProjectRoot(std.testing.allocator, io, base ++ "/sub/deep");
    if (result) |r| std.testing.allocator.free(r);

    testCleanup(io, base);
}

// ---------------------------------------------------------------------------
// discoverSpecFiles tests
// ---------------------------------------------------------------------------

test "discoverSpecFiles with directory" {
    var threaded = getTestIo();
    const io = threaded.io();
    const base = "/tmp/yass-test-dsf-dir";
    testCleanup(io, base);
    try testCreateDirPath(io, base ++ "/sub");
    try testCreateFile(io, base ++ "/alpha.yass.yaml");
    try testCreateFile(io, base ++ "/sub/beta.yass.yaml");
    try testCreateFile(io, base ++ "/other.txt");

    const result = try discoverSpecFiles(std.testing.allocator, io, base, base);
    defer {
        for (result.files) |f| std.testing.allocator.free(f);
        std.testing.allocator.free(result.files);
        for (result.errors) |e| std.testing.allocator.free(e.path);
        std.testing.allocator.free(result.errors);
    }

    try std.testing.expectEqual(@as(usize, 2), result.files.len);
    try std.testing.expectEqual(@as(usize, 0), result.errors.len);
    // Should be sorted
    try std.testing.expectEqualStrings("alpha.yass.yaml", result.files[0]);
    try std.testing.expectEqualStrings("sub/beta.yass.yaml", result.files[1]);

    testCleanup(io, base);
}

test "discoverSpecFiles with single file" {
    var threaded = getTestIo();
    const io = threaded.io();
    const base = "/tmp/yass-test-dsf-file";
    testCleanup(io, base);
    try testCreateDirPath(io, base);
    try testCreateFile(io, base ++ "/spec.yass.yaml");

    const file_path = base ++ "/spec.yass.yaml";
    const result = try discoverSpecFiles(std.testing.allocator, io, file_path, base);
    defer {
        for (result.files) |f| std.testing.allocator.free(f);
        std.testing.allocator.free(result.files);
        for (result.errors) |e| std.testing.allocator.free(e.path);
        std.testing.allocator.free(result.errors);
    }

    try std.testing.expectEqual(@as(usize, 1), result.files.len);
    try std.testing.expectEqualStrings("spec.yass.yaml", result.files[0]);

    testCleanup(io, base);
}

test "discoverSpecFiles skips hidden dirs and files" {
    var threaded = getTestIo();
    const io = threaded.io();
    const base = "/tmp/yass-test-dsf-hidden";
    testCleanup(io, base);
    try testCreateDirPath(io, base ++ "/.hidden_dir");
    try testCreateFile(io, base ++ "/.hidden_dir/secret.yass.yaml");
    try testCreateFile(io, base ++ "/.hidden.yass.yaml");
    try testCreateFile(io, base ++ "/visible.yass.yaml");

    const result = try discoverSpecFiles(std.testing.allocator, io, base, base);
    defer {
        for (result.files) |f| std.testing.allocator.free(f);
        std.testing.allocator.free(result.files);
        for (result.errors) |e| std.testing.allocator.free(e.path);
        std.testing.allocator.free(result.errors);
    }

    try std.testing.expectEqual(@as(usize, 1), result.files.len);
    try std.testing.expectEqualStrings("visible.yass.yaml", result.files[0]);

    testCleanup(io, base);
}

test "discoverSpecFiles rejects bad extension" {
    var threaded = getTestIo();
    const io = threaded.io();
    const base = "/tmp/yass-test-dsf-badext";
    testCleanup(io, base);
    try testCreateDirPath(io, base);
    try testCreateFile(io, base ++ "/readme.txt");

    const result = try discoverSpecFiles(std.testing.allocator, io, base ++ "/readme.txt", base);
    defer {
        for (result.files) |f| std.testing.allocator.free(f);
        std.testing.allocator.free(result.files);
        for (result.errors) |e| std.testing.allocator.free(e.path);
        std.testing.allocator.free(result.errors);
    }

    try std.testing.expectEqual(@as(usize, 0), result.files.len);
    try std.testing.expectEqual(@as(usize, 1), result.errors.len);
    try std.testing.expect(result.errors[0].code == .@"yass.path.bad_extension");

    testCleanup(io, base);
}

test "discoverSpecFiles rejects bare .yass.yaml" {
    var threaded = getTestIo();
    const io = threaded.io();
    const base = "/tmp/yass-test-dsf-bare";
    testCleanup(io, base);
    try testCreateDirPath(io, base);
    try testCreateFile(io, base ++ "/.yass.yaml");

    // As a single file argument
    const result = try discoverSpecFiles(std.testing.allocator, io, base ++ "/.yass.yaml", base);
    defer {
        for (result.files) |f| std.testing.allocator.free(f);
        std.testing.allocator.free(result.files);
        for (result.errors) |e| std.testing.allocator.free(e.path);
        std.testing.allocator.free(result.errors);
    }

    // Should be rejected: basename is ".yass.yaml" which starts with dot (hidden),
    // and the bare name equals the suffix exactly.
    try std.testing.expectEqual(@as(usize, 0), result.files.len);
    try std.testing.expectEqual(@as(usize, 1), result.errors.len);
    try std.testing.expect(result.errors[0].code == .@"yass.path.bad_extension");

    testCleanup(io, base);
}

test "discoverSpecFiles path not found" {
    var threaded = getTestIo();
    const io = threaded.io();

    const result = try discoverSpecFiles(std.testing.allocator, io, "/tmp/yass-test-nonexistent-path-xyz", "/tmp");
    defer {
        for (result.files) |f| std.testing.allocator.free(f);
        std.testing.allocator.free(result.files);
        for (result.errors) |e| std.testing.allocator.free(e.path);
        std.testing.allocator.free(result.errors);
    }

    try std.testing.expectEqual(@as(usize, 0), result.files.len);
    try std.testing.expectEqual(@as(usize, 1), result.errors.len);
    try std.testing.expect(result.errors[0].code == .@"yass.path.not_found");
}

// ---------------------------------------------------------------------------
// relativePath tests
// ---------------------------------------------------------------------------

test "relativePath strips cwd prefix" {
    try std.testing.expectEqualStrings(
        "sub/file.yaml",
        relativePath("/home/user/project/sub/file.yaml", "/home/user/project"),
    );
}

test "relativePath returns basename for file in cwd" {
    try std.testing.expectEqualStrings(
        "file.yaml",
        relativePath("/home/user/project/file.yaml", "/home/user/project"),
    );
}

test "relativePath returns absolute when not under cwd" {
    try std.testing.expectEqualStrings(
        "/other/path/file.yaml",
        relativePath("/other/path/file.yaml", "/home/user/project"),
    );
}

test "relativePath no ./ prefix" {
    const result = relativePath("/home/user/project/file.yaml", "/home/user/project");
    try std.testing.expect(!std.mem.startsWith(u8, result, "./"));
}

test "relativePath handles same path" {
    try std.testing.expectEqualStrings(
        ".",
        relativePath("/home/user/project", "/home/user/project"),
    );
}

test "relativePath no false match on prefix substring" {
    // /home/user/project-extra should NOT be treated as under /home/user/project
    try std.testing.expectEqualStrings(
        "/home/user/project-extra/file.yaml",
        relativePath("/home/user/project-extra/file.yaml", "/home/user/project"),
    );
}

// ---------------------------------------------------------------------------
// hasGlobMetachars tests
// ---------------------------------------------------------------------------

test "hasGlobMetachars detects *" {
    try std.testing.expect(hasGlobMetachars("*.yaml"));
}

test "hasGlobMetachars detects ?" {
    try std.testing.expect(hasGlobMetachars("file?.yaml"));
}

test "hasGlobMetachars detects [" {
    try std.testing.expect(hasGlobMetachars("file[abc].yaml"));
}

test "hasGlobMetachars detects **" {
    try std.testing.expect(hasGlobMetachars("src/**/*.yaml"));
}

test "hasGlobMetachars returns false for literal" {
    try std.testing.expect(!hasGlobMetachars("src/file.yaml"));
}

test "hasGlobMetachars returns false for empty" {
    try std.testing.expect(!hasGlobMetachars(""));
}

// ---------------------------------------------------------------------------
// expandGlob tests
// ---------------------------------------------------------------------------

test "expandGlob basic star pattern" {
    var threaded = getTestIo();
    const io = threaded.io();
    const base = "/tmp/yass-test-glob-star";
    testCleanup(io, base);
    try testCreateDirPath(io, base);
    try testCreateFile(io, base ++ "/alpha.yaml");
    try testCreateFile(io, base ++ "/beta.yaml");
    try testCreateFile(io, base ++ "/gamma.txt");

    const results = try expandGlob(std.testing.allocator, io, base ++ "/*.yaml");
    defer {
        for (results) |r| std.testing.allocator.free(r);
        std.testing.allocator.free(results);
    }

    try std.testing.expectEqual(@as(usize, 2), results.len);
    try std.testing.expectEqualStrings(base ++ "/alpha.yaml", results[0]);
    try std.testing.expectEqualStrings(base ++ "/beta.yaml", results[1]);

    testCleanup(io, base);
}

test "expandGlob doublestar pattern" {
    var threaded = getTestIo();
    const io = threaded.io();
    const base = "/tmp/yass-test-glob-dstar";
    testCleanup(io, base);
    try testCreateDirPath(io, base ++ "/sub/deep");
    try testCreateFile(io, base ++ "/top.yaml");
    try testCreateFile(io, base ++ "/sub/mid.yaml");
    try testCreateFile(io, base ++ "/sub/deep/bot.yaml");

    const results = try expandGlob(std.testing.allocator, io, base ++ "/**/*.yaml");
    defer {
        for (results) |r| std.testing.allocator.free(r);
        std.testing.allocator.free(results);
    }

    try std.testing.expectEqual(@as(usize, 3), results.len);
    // ** matches zero or more segments, so top.yaml matches too
    try std.testing.expectEqualStrings(base ++ "/sub/deep/bot.yaml", results[0]);
    try std.testing.expectEqualStrings(base ++ "/sub/mid.yaml", results[1]);
    try std.testing.expectEqualStrings(base ++ "/top.yaml", results[2]);

    testCleanup(io, base);
}

test "expandGlob skips hidden files" {
    var threaded = getTestIo();
    const io = threaded.io();
    const base = "/tmp/yass-test-glob-hidden";
    testCleanup(io, base);
    try testCreateDirPath(io, base);
    try testCreateFile(io, base ++ "/visible.yaml");
    try testCreateFile(io, base ++ "/.hidden.yaml");

    const results = try expandGlob(std.testing.allocator, io, base ++ "/*.yaml");
    defer {
        for (results) |r| std.testing.allocator.free(r);
        std.testing.allocator.free(results);
    }

    try std.testing.expectEqual(@as(usize, 1), results.len);
    try std.testing.expectEqualStrings(base ++ "/visible.yaml", results[0]);

    testCleanup(io, base);
}

test "expandGlob no match returns error" {
    var threaded = getTestIo();
    const io = threaded.io();
    const base = "/tmp/yass-test-glob-nomatch";
    testCleanup(io, base);
    try testCreateDirPath(io, base);
    try testCreateFile(io, base ++ "/file.txt");

    const result = expandGlob(std.testing.allocator, io, base ++ "/*.yaml");
    try std.testing.expectError(error.NoMatch, result);

    testCleanup(io, base);
}

test "expandGlob question mark pattern" {
    var threaded = getTestIo();
    const io = threaded.io();
    const base = "/tmp/yass-test-glob-qmark";
    testCleanup(io, base);
    try testCreateDirPath(io, base);
    try testCreateFile(io, base ++ "/a1.txt");
    try testCreateFile(io, base ++ "/a2.txt");
    try testCreateFile(io, base ++ "/ab.txt");
    try testCreateFile(io, base ++ "/b1.txt");

    const results = try expandGlob(std.testing.allocator, io, base ++ "/a?.txt");
    defer {
        for (results) |r| std.testing.allocator.free(r);
        std.testing.allocator.free(results);
    }

    try std.testing.expectEqual(@as(usize, 3), results.len);
    try std.testing.expectEqualStrings(base ++ "/a1.txt", results[0]);
    try std.testing.expectEqualStrings(base ++ "/a2.txt", results[1]);
    try std.testing.expectEqualStrings(base ++ "/ab.txt", results[2]);

    testCleanup(io, base);
}

test "expandGlob character class" {
    var threaded = getTestIo();
    const io = threaded.io();
    const base = "/tmp/yass-test-glob-class";
    testCleanup(io, base);
    try testCreateDirPath(io, base);
    try testCreateFile(io, base ++ "/a.txt");
    try testCreateFile(io, base ++ "/b.txt");
    try testCreateFile(io, base ++ "/c.txt");
    try testCreateFile(io, base ++ "/d.txt");

    const results = try expandGlob(std.testing.allocator, io, base ++ "/[ab].txt");
    defer {
        for (results) |r| std.testing.allocator.free(r);
        std.testing.allocator.free(results);
    }

    try std.testing.expectEqual(@as(usize, 2), results.len);
    try std.testing.expectEqualStrings(base ++ "/a.txt", results[0]);
    try std.testing.expectEqualStrings(base ++ "/b.txt", results[1]);

    testCleanup(io, base);
}

// ---------------------------------------------------------------------------
// globMatch unit tests
// ---------------------------------------------------------------------------

test "globMatch literal" {
    try std.testing.expect(globMatch("foo.txt", "foo.txt"));
    try std.testing.expect(!globMatch("foo.txt", "bar.txt"));
}

test "globMatch star" {
    try std.testing.expect(globMatch("*.txt", "foo.txt"));
    try std.testing.expect(!globMatch("*.txt", "foo.yaml"));
    try std.testing.expect(!globMatch("*.txt", "dir/foo.txt"));
}

test "globMatch doublestar" {
    try std.testing.expect(globMatch("**/*.txt", "foo.txt"));
    try std.testing.expect(globMatch("**/*.txt", "a/foo.txt"));
    try std.testing.expect(globMatch("**/*.txt", "a/b/c/foo.txt"));
}

test "globMatch question" {
    try std.testing.expect(globMatch("?.txt", "a.txt"));
    try std.testing.expect(!globMatch("?.txt", "ab.txt"));
    try std.testing.expect(!globMatch("?.txt", ".txt"));
}

test "globMatch char class" {
    try std.testing.expect(globMatch("[abc].txt", "a.txt"));
    try std.testing.expect(globMatch("[abc].txt", "b.txt"));
    try std.testing.expect(!globMatch("[abc].txt", "d.txt"));
}

test "globMatch negated class" {
    try std.testing.expect(globMatch("[!abc].txt", "d.txt"));
    try std.testing.expect(!globMatch("[!abc].txt", "a.txt"));
}

test "globMatch char range" {
    try std.testing.expect(globMatch("[a-c].txt", "b.txt"));
    try std.testing.expect(!globMatch("[a-c].txt", "d.txt"));
}

// ---------------------------------------------------------------------------
// isYassYamlFile tests
// ---------------------------------------------------------------------------

test "isYassYamlFile valid" {
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
