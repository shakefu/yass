const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    // Create the main module with libyaml linkage
    const main_mod = b.createModule(.{
        .root_source_file = b.path("src/main.zig"),
        .target = target,
        .optimize = optimize,
    });
    main_mod.linkSystemLibrary("yaml", .{});
    main_mod.addIncludePath(.{ .cwd_relative = "/opt/homebrew/include" });
    main_mod.addLibraryPath(.{ .cwd_relative = "/opt/homebrew/lib" });
    main_mod.addEmbedPath(b.path("."));

    // Main executable
    const exe = b.addExecutable(.{
        .name = "yass",
        .root_module = main_mod,
    });
    b.installArtifact(exe);

    // Run step
    const run_cmd = b.addRunArtifact(exe);
    run_cmd.step.dependOn(b.getInstallStep());
    if (b.args) |args| {
        run_cmd.addArgs(args);
    }
    const run_step = b.step("run", "Run the yass CLI");
    run_step.dependOn(&run_cmd.step);

    // Unit tests - main
    const test_mod = b.createModule(.{
        .root_source_file = b.path("src/main.zig"),
        .target = target,
        .optimize = optimize,
    });
    test_mod.linkSystemLibrary("yaml", .{});
    test_mod.addIncludePath(.{ .cwd_relative = "/opt/homebrew/include" });
    test_mod.addLibraryPath(.{ .cwd_relative = "/opt/homebrew/lib" });
    test_mod.addEmbedPath(b.path("."));

    const unit_tests = b.addTest(.{
        .root_module = test_mod,
    });
    const run_unit_tests = b.addRunArtifact(unit_tests);
    const test_step = b.step("test", "Run unit tests");
    test_step.dependOn(&run_unit_tests.step);

    // Unit tests - shared
    const shared_test_mod = b.createModule(.{
        .root_source_file = b.path("src/shared.zig"),
        .target = target,
        .optimize = optimize,
    });

    const shared_tests = b.addTest(.{
        .root_module = shared_test_mod,
    });
    const run_shared_tests = b.addRunArtifact(shared_tests);
    test_step.dependOn(&run_shared_tests.step);

    // Unit tests - validate
    const validate_test_mod = b.createModule(.{
        .root_source_file = b.path("src/validate.zig"),
        .target = target,
        .optimize = optimize,
    });
    validate_test_mod.linkSystemLibrary("yaml", .{});
    validate_test_mod.addIncludePath(.{ .cwd_relative = "/opt/homebrew/include" });
    validate_test_mod.addLibraryPath(.{ .cwd_relative = "/opt/homebrew/lib" });

    const validate_tests = b.addTest(.{
        .root_module = validate_test_mod,
    });
    const run_validate_tests = b.addRunArtifact(validate_tests);
    test_step.dependOn(&run_validate_tests.step);
}
