package shared

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	yerrors "github.com/shakefu/yass/internal/errors"
)

// createFIFO creates a named pipe (FIFO) at the given path.
func createFIFO(path string) error {
	return syscall.Mkfifo(path, 0o644)
}

func TestDiscoverSpecFiles_SingleFile(t *testing.T) {
	tmp := t.TempDir()
	specFile := filepath.Join(tmp, "test.yass.yaml")
	if err := os.WriteFile(specFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, nonFatal, err := DiscoverSpecFiles(specFile, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nonFatal) != 0 {
		t.Errorf("unexpected non-fatal errors: %v", nonFatal)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	// File is directly in cwd, so basename should be returned.
	if files[0] != "test.yass.yaml" {
		t.Errorf("expected basename 'test.yass.yaml', got %s", files[0])
	}
}

func TestDiscoverSpecFiles_DirectoryWithFiles(t *testing.T) {
	tmp := t.TempDir()

	// Create nested structure.
	subDir := filepath.Join(tmp, "specs", "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		filepath.Join(tmp, "root.yass.yaml"),
		filepath.Join(tmp, "specs", "mid.yass.yaml"),
		filepath.Join(tmp, "specs", "nested", "deep.yass.yaml"),
	} {
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, nonFatal, err := DiscoverSpecFiles(tmp, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nonFatal) != 0 {
		t.Errorf("unexpected non-fatal errors: %v", nonFatal)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d: %v", len(files), files)
	}
}

func TestDiscoverSpecFiles_HiddenFilesExcluded(t *testing.T) {
	tmp := t.TempDir()

	// Visible file.
	if err := os.WriteFile(filepath.Join(tmp, "visible.yass.yaml"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Hidden file.
	if err := os.WriteFile(filepath.Join(tmp, ".hidden.yass.yaml"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, _, err := DiscoverSpecFiles(tmp, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file (hidden excluded), got %d: %v", len(files), files)
	}
	if files[0] != "visible.yass.yaml" {
		t.Errorf("expected 'visible.yass.yaml', got %s", files[0])
	}
}

func TestDiscoverSpecFiles_HiddenDirsNotDescended(t *testing.T) {
	tmp := t.TempDir()

	hiddenDir := filepath.Join(tmp, ".hidden_dir")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "spec.yass.yaml"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Visible file for comparison.
	if err := os.WriteFile(filepath.Join(tmp, "visible.yass.yaml"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, _, err := DiscoverSpecFiles(tmp, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file (hidden dir not descended), got %d: %v", len(files), files)
	}
}

func TestDiscoverSpecFiles_BadExtension(t *testing.T) {
	tmp := t.TempDir()
	badFile := filepath.Join(tmp, "test.yaml")
	if err := os.WriteFile(badFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := DiscoverSpecFiles(badFile, tmp, tmp)
	if err == nil {
		t.Fatal("expected error for bad extension")
	}
	pathErr, ok := err.(*PathError)
	if !ok {
		t.Fatalf("expected PathError, got %T: %v", err, err)
	}
	if pathErr.Code != yerrors.CodePathBadExtension {
		t.Errorf("expected code %s, got %s", yerrors.CodePathBadExtension, pathErr.Code)
	}
}

func TestDiscoverSpecFiles_NotFound(t *testing.T) {
	tmp := t.TempDir()

	_, _, err := DiscoverSpecFiles(filepath.Join(tmp, "nonexistent.yass.yaml"), tmp, tmp)
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
	pathErr, ok := err.(*PathError)
	if !ok {
		t.Fatalf("expected PathError, got %T: %v", err, err)
	}
	if pathErr.Code != yerrors.CodePathNotFound {
		t.Errorf("expected code %s, got %s", yerrors.CodePathNotFound, pathErr.Code)
	}
}

func TestDiscoverSpecFiles_SortOrder(t *testing.T) {
	tmp := t.TempDir()

	// Create files that would be in non-alphabetical order.
	for _, name := range []string{"c.yass.yaml", "a.yass.yaml", "b.yass.yaml"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, _, err := DiscoverSpecFiles(tmp, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
	if files[0] != "a.yass.yaml" || files[1] != "b.yass.yaml" || files[2] != "c.yass.yaml" {
		t.Errorf("files not sorted: %v", files)
	}
}

func TestDiscoverSpecFiles_PathFormatting(t *testing.T) {
	tmp := t.TempDir()
	cwd := filepath.Join(tmp, "cwd")
	projRoot := filepath.Join(tmp, "project")

	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projRoot, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	// File in project subdir (not under cwd) -> absolute path.
	specFile := filepath.Join(projRoot, "sub", "test.yass.yaml")
	if err := os.WriteFile(specFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, _, err := DiscoverSpecFiles(specFile, cwd, projRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	// Not under cwd, so should be absolute.
	if !filepath.IsAbs(files[0]) {
		t.Errorf("expected absolute path, got %s", files[0])
	}
}

func TestDiscoverSpecFiles_RelativePath(t *testing.T) {
	tmp := t.TempDir()

	subDir := filepath.Join(tmp, "specs")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	specFile := filepath.Join(subDir, "test.yass.yaml")
	if err := os.WriteFile(specFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// cwd is tmp, file is in specs/test.yass.yaml -> relative: specs/test.yass.yaml
	files, _, err := DiscoverSpecFiles(specFile, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0] != "specs/test.yass.yaml" {
		t.Errorf("expected relative 'specs/test.yass.yaml', got %s", files[0])
	}
}

func TestDiscoverSpecFiles_BasenamePath(t *testing.T) {
	tmp := t.TempDir()

	specFile := filepath.Join(tmp, "test.yass.yaml")
	if err := os.WriteFile(specFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// cwd is tmp, file is directly in cwd -> basename.
	files, _, err := DiscoverSpecFiles(specFile, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0] != "test.yass.yaml" {
		t.Errorf("expected basename 'test.yass.yaml', got %s", files[0])
	}
	// Should not have a leading "./"
	if strings.HasPrefix(files[0], "./") {
		t.Errorf("should not have leading './', got %s", files[0])
	}
}

func TestDiscoverSpecFiles_BareYassYamlDoesNotMatch(t *testing.T) {
	tmp := t.TempDir()

	// The bare filename ".yass.yaml" should not match.
	bareFile := filepath.Join(tmp, ".yass.yaml")
	if err := os.WriteFile(bareFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, _, err := DiscoverSpecFiles(tmp, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("bare .yass.yaml should not match, got %v", files)
	}
}

func TestDiscoverSpecFiles_EmptyDirectory(t *testing.T) {
	tmp := t.TempDir()

	files, nonFatal, err := DiscoverSpecFiles(tmp, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nonFatal) != 0 {
		t.Errorf("unexpected non-fatal errors: %v", nonFatal)
	}
	if len(files) != 0 {
		t.Errorf("expected empty list for empty directory, got %v", files)
	}
}

func TestDiscoverSpecFiles_EmptyInputUsesProjectRoot(t *testing.T) {
	tmp := t.TempDir()

	specFile := filepath.Join(tmp, "test.yass.yaml")
	if err := os.WriteFile(specFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Empty inputPath should use projectRoot.
	files, _, err := DiscoverSpecFiles("", tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(files), files)
	}
}

func TestDiscoverSpecFiles_SymlinkDuringRecursionTreatedAsAbsent(t *testing.T) {
	tmp := t.TempDir()

	// Create a real file and a symlink to it during recursion.
	realDir := filepath.Join(tmp, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "spec.yass.yaml"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	searchDir := filepath.Join(tmp, "search")
	if err := os.MkdirAll(searchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a symlink to the real spec file inside searchDir.
	symlink := filepath.Join(searchDir, "linked.yass.yaml")
	if err := os.Symlink(filepath.Join(realDir, "spec.yass.yaml"), symlink); err != nil {
		t.Skip("symlinks not supported")
	}

	files, _, err := DiscoverSpecFiles(searchDir, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Symlinks during recursion are treated as absent.
	if len(files) != 0 {
		t.Errorf("symlinks during recursion should be absent, got %v", files)
	}
}

func TestDiscoverSpecFiles_DirectFileSymlink(t *testing.T) {
	tmp := t.TempDir()

	// Create a real spec file.
	realFile := filepath.Join(tmp, "real.yass.yaml")
	if err := os.WriteFile(realFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink as direct input.
	symlinkFile := filepath.Join(tmp, "linked.yass.yaml")
	if err := os.Symlink(realFile, symlinkFile); err != nil {
		t.Skip("symlinks not supported")
	}

	files, _, err := DiscoverSpecFiles(symlinkFile, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(files), files)
	}
	// Should use the symlink path, not the resolved path.
	if files[0] != "linked.yass.yaml" {
		t.Errorf("expected 'linked.yass.yaml', got %s", files[0])
	}
}

func TestDiscoverSpecFiles_OnlyYamlSuffix(t *testing.T) {
	tmp := t.TempDir()

	// File ending in .yaml but not .yass.yaml.
	plainYaml := filepath.Join(tmp, "test.yaml")
	if err := os.WriteFile(plainYaml, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// When used as directory traversal, .yaml files should not match.
	files, _, err := DiscoverSpecFiles(tmp, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("plain .yaml file should not match, got %v", files)
	}
}

func TestDiscoverSpecFiles_InvalidType(t *testing.T) {
	tmp := t.TempDir()

	// Create a FIFO (named pipe) which is neither file nor directory.
	fifoPath := filepath.Join(tmp, "pipe.yass.yaml")
	if err := createFIFO(fifoPath); err != nil {
		t.Skip("cannot create FIFO on this platform")
	}

	_, _, err := DiscoverSpecFiles(fifoPath, tmp, tmp)
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	pathErr, ok := err.(*PathError)
	if !ok {
		t.Fatalf("expected PathError, got %T: %v", err, err)
	}
	if pathErr.Code != yerrors.CodePathInvalidType {
		t.Errorf("expected code %s, got %s", yerrors.CodePathInvalidType, pathErr.Code)
	}
}

func TestDiscoverSpecFiles_DirectorySymlinkAsInput(t *testing.T) {
	tmp := t.TempDir()

	// Create a real directory with spec files.
	realDir := filepath.Join(tmp, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "spec.yass.yaml"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink to the directory.
	symlinkDir := filepath.Join(tmp, "linked")
	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Skip("symlinks not supported")
	}

	// When a directory symlink is given directly, treat it as the directory.
	files, _, err := DiscoverSpecFiles(symlinkDir, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(files), files)
	}
}

func TestDiscoverSpecFiles_UnreadableSubdirContinues(t *testing.T) {
	tmp := t.TempDir()

	// Create a readable directory with a spec file.
	if err := os.WriteFile(filepath.Join(tmp, "good.yass.yaml"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create an unreadable subdirectory.
	badDir := filepath.Join(tmp, "unreadable")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Put a spec file in it.
	if err := os.WriteFile(filepath.Join(badDir, "bad.yass.yaml"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make it unreadable.
	if err := os.Chmod(badDir, 0o000); err != nil {
		t.Skip("cannot change permissions on this platform")
	}
	defer os.Chmod(badDir, 0o755) // restore for cleanup

	files, nonFatal, err := DiscoverSpecFiles(tmp, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	// Should still find the good file.
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d: %v", len(files), files)
	}
	// Should have a non-fatal error for the unreadable directory.
	if len(nonFatal) != 1 {
		t.Errorf("expected 1 non-fatal error, got %d: %v", len(nonFatal), nonFatal)
	}
	if len(nonFatal) > 0 && !strings.Contains(nonFatal[0], yerrors.CodeDiscoverDirUnreadable) {
		t.Errorf("expected dir_unreadable code in error, got %s", nonFatal[0])
	}
}

func TestDiscoverSpecFiles_CaseSensitiveMatch(t *testing.T) {
	tmp := t.TempDir()

	// On case-insensitive filesystems (macOS HFS+/APFS), creating both
	// "test.YASS.YAML" and "test.yass.yaml" in the same dir causes one to
	// overwrite the other. So we test them in separate directories.

	// Directory with uppercase-only file.
	upperDir := filepath.Join(tmp, "upper")
	if err := os.MkdirAll(upperDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(upperDir, "test.YASS.YAML"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Check: on a case-insensitive FS, the file as listed may have a different case.
	entries, err := os.ReadDir(upperDir)
	if err != nil {
		t.Fatal(err)
	}
	actualName := ""
	if len(entries) > 0 {
		actualName = entries[0].Name()
	}

	// Our matching is case-sensitive byte comparison. If the FS stored the name
	// as-is (case-sensitive), it should NOT match ".yass.yaml". If the FS
	// normalised the name, the stored name won't end in ".yass.yaml" either.
	files, _, err := DiscoverSpecFiles(upperDir, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actualName == "test.YASS.YAML" {
		// Case-sensitive FS: uppercase should not match.
		if len(files) != 0 {
			t.Errorf("uppercase .YASS.YAML should not match case-sensitively, got %v", files)
		}
	} else {
		// Case-insensitive FS stored it differently; just log.
		t.Logf("case-insensitive FS: stored name %q, skipping strict assertion", actualName)
	}

	// Directory with correct-case file should always match.
	lowerDir := filepath.Join(tmp, "lower")
	if err := os.MkdirAll(lowerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lowerDir, "test.yass.yaml"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, _, err = DiscoverSpecFiles(lowerDir, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file for lowercase .yass.yaml, got %d: %v", len(files), files)
	}
}

func TestDiscoverSpecFiles_TopLevelUnreadable(t *testing.T) {
	tmp := t.TempDir()

	// Create an unreadable file as the direct input.
	specFile := filepath.Join(tmp, "secret.yass.yaml")
	if err := os.WriteFile(specFile, []byte("test"), 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(specFile, 0o644) // restore for cleanup

	// Stat should still succeed (we use Lstat), so this should return the file.
	// The spec says top-level unreadable means cannot be accessed (stat failure).
	// A file with 0o000 permissions can still be stat'd, just not read.
	// So this is actually a valid file return.
	files, _, err := DiscoverSpecFiles(specFile, tmp, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(files), files)
	}
}
