package validate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shakefu/yass/internal/exitcode"
)

// validSpec returns minimal valid .yass.yaml content with preamble and one spec.
func validSpec() string {
	return "---\ndescription: test\nversion: v1\n---\nspec: MySpec\nINPUT:\n- MUST: \"do something\"\n"
}

// setupProject creates a temp dir with a .git marker so FindProjectRoot works.
func setupProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeFile writes content to dir/name and returns the full path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// runInDir runs validate.Run while cwd is set to dir.
// Returns (stdout, stderr, exitCode).
func runInDir(t *testing.T, dir string, args []string) (string, string, int) {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatal(err)
		}
	}()

	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

// countLines counts non-empty lines in a string.
func countLines(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}

// --- No-args discovery tests ---

func TestRun_NoArgs_ValidFiles(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "spec/a.yass.yaml", validSpec())

	stdout, stderr, code := runInDir(t, dir, nil)

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr=%q", exitcode.Success, code, stderr)
	}
	if stderr != "" {
		t.Errorf("expected no stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "checked 1 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_NoArgs_MultipleValidFiles(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "spec/a.yass.yaml", validSpec())
	writeFile(t, dir, "spec/b.yass.yaml", validSpec())

	stdout, stderr, code := runInDir(t, dir, nil)

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr=%q", exitcode.Success, code, stderr)
	}
	if !strings.Contains(stdout, "checked 2 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_NoArgs_NoFiles_Exit2(t *testing.T) {
	dir := setupProject(t)
	// Empty project, no .yass.yaml files

	stdout, stderr, code := runInDir(t, dir, nil)

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr, "yass.discover.no_files") {
		t.Errorf("expected discover.no_files error, got %q", stderr)
	}
	if !strings.Contains(stdout, "checked 0 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

// --- Single file arg tests ---

func TestRun_SingleFileArg_Valid(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "test.yass.yaml", validSpec())

	stdout, stderr, code := runInDir(t, dir, []string{"test.yass.yaml"})

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr=%q", exitcode.Success, code, stderr)
	}
	if !strings.Contains(stdout, "checked 1 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_SingleFileArg_Absolute(t *testing.T) {
	dir := setupProject(t)
	fp := writeFile(t, dir, "abs.yass.yaml", validSpec())

	stdout, stderr, code := runInDir(t, dir, []string{fp})

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr=%q", exitcode.Success, code, stderr)
	}
	if !strings.Contains(stdout, "checked 1 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

// --- Directory arg tests ---

func TestRun_DirectoryArg(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "specs/one.yass.yaml", validSpec())
	writeFile(t, dir, "specs/two.yass.yaml", validSpec())

	stdout, stderr, code := runInDir(t, dir, []string{"specs"})

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr=%q", exitcode.Success, code, stderr)
	}
	if !strings.Contains(stdout, "checked 2 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_DirectoryArg_Nested(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "a/b/c/deep.yass.yaml", validSpec())

	stdout, stderr, code := runInDir(t, dir, []string{"a"})

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr=%q", exitcode.Success, code, stderr)
	}
	if !strings.Contains(stdout, "checked 1 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

// --- Validation errors tests ---

func TestRun_FileWithValidationErrors_Exit1(t *testing.T) {
	dir := setupProject(t)
	// Missing version in preamble
	writeFile(t, dir, "bad.yass.yaml", "---\ndescription: test\n---\nspec: MySpec\n")

	stdout, stderr, code := runInDir(t, dir, []string{"bad.yass.yaml"})

	if code != exitcode.Processing {
		t.Errorf("expected exit %d, got %d", exitcode.Processing, code)
	}
	if !strings.Contains(stderr, "yass.preamble.missing_version") {
		t.Errorf("expected preamble.missing_version in stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "found 1 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_MultipleValidationErrors_AllReported(t *testing.T) {
	dir := setupProject(t)
	// File with multiple unknown keys
	writeFile(t, dir, "multi.yass.yaml",
		"---\ndescription: test\nversion: v1\n---\nspec: MySpec\nBADKEY1: invalid\nBADKEY2: invalid\n")

	_, stderr, code := runInDir(t, dir, []string{"multi.yass.yaml"})

	if code != exitcode.Processing {
		t.Errorf("expected exit %d, got %d", exitcode.Processing, code)
	}
	errCount := strings.Count(stderr, "yass.spec.unknown_key")
	if errCount < 2 {
		t.Errorf("expected at least 2 unknown_key errors, got %d; stderr=%q", errCount, stderr)
	}
}

func TestRun_MultipleFiles_SomeWithErrors(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "spec/good.yass.yaml", validSpec())
	writeFile(t, dir, "spec/bad.yass.yaml", "---\ndescription: test\n---\nspec: MySpec\n")

	stdout, stderr, code := runInDir(t, dir, nil)

	if code != exitcode.Processing {
		t.Errorf("expected exit %d, got %d", exitcode.Processing, code)
	}
	if !strings.Contains(stderr, "yass.preamble.missing_version") {
		t.Errorf("expected preamble.missing_version in stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "checked 2 files, found 1 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_ContinuesCheckingAfterFileFailure(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "spec/a.yass.yaml", validSpec())
	// b.yass.yaml is empty -> CheckYAML failure
	writeFile(t, dir, "spec/b.yass.yaml", "")

	stdout, stderr, code := runInDir(t, dir, nil)

	if code != exitcode.Processing {
		t.Errorf("expected exit %d, got %d", exitcode.Processing, code)
	}
	if !strings.Contains(stderr, "yass.yaml.empty_file") {
		t.Errorf("expected empty_file error, got %q", stderr)
	}
	// Both files should be counted
	if !strings.Contains(stdout, "checked 2 files") {
		t.Errorf("expected 2 files checked, got %q", stdout)
	}
}

// --- Path error tests ---

func TestRun_PathNotFound_Exit2(t *testing.T) {
	dir := setupProject(t)

	stdout, stderr, code := runInDir(t, dir, []string{"nonexistent.yass.yaml"})

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr, "yass.path.not_found") {
		t.Errorf("expected path.not_found, got %q", stderr)
	}
	if !strings.Contains(stdout, "checked 0 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_BadExtension_Exit2(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "test.yaml", "key: value\n")

	stdout, stderr, code := runInDir(t, dir, []string{"test.yaml"})

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr, "yass.path.bad_extension") {
		t.Errorf("expected path.bad_extension, got %q", stderr)
	}
	if !strings.Contains(stdout, "checked 0 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_ColonInPath_Exit2(t *testing.T) {
	dir := setupProject(t)

	stdout, stderr, code := runInDir(t, dir, []string{"path:with:colon.yass.yaml"})

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr, "yass.path.colon_in_path") {
		t.Errorf("expected colon_in_path, got %q", stderr)
	}
	if !strings.Contains(stdout, "checked 0 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_ColonCheckedBeforeProjectRoot(t *testing.T) {
	// Even without .git marker, colon error should fire first.
	dir := t.TempDir() // No .git

	_, stderr, code := runInDir(t, dir, []string{"has:colon.yass.yaml"})

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr, "yass.path.colon_in_path") {
		t.Errorf("expected colon_in_path, got %q", stderr)
	}
}

// --- FindProjectRoot failure ---

func TestRun_FindProjectRootFailure_Exit2(t *testing.T) {
	dir := t.TempDir() // No .git, no .yass.yaml

	stdout, stderr, code := runInDir(t, dir, nil)

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr, "yass.findroot.no_marker") {
		t.Errorf("expected findroot.no_marker, got %q", stderr)
	}
	if !strings.Contains(stdout, "checked 0 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_FindProjectRootFailure_WithArgs(t *testing.T) {
	dir := t.TempDir() // No .git, no .yass.yaml

	_, stderr, code := runInDir(t, dir, []string{"some.yass.yaml"})

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr, "yass.findroot.no_marker") {
		t.Errorf("expected findroot.no_marker, got %q", stderr)
	}
}

// --- Summary format tests ---

func TestRun_SummaryFormat_ZeroFiles(t *testing.T) {
	dir := setupProject(t)

	stdout, _, _ := runInDir(t, dir, nil)

	if stdout != "checked 0 files, found 0 errors\n" {
		t.Errorf("expected exact summary, got %q", stdout)
	}
}

func TestRun_SummaryFormat_OneFileZeroErrors(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "test.yass.yaml", validSpec())

	stdout, _, _ := runInDir(t, dir, []string{"test.yass.yaml"})

	if stdout != "checked 1 files, found 0 errors\n" {
		t.Errorf("expected exact summary, got %q", stdout)
	}
}

func TestRun_SummaryFormat_WithErrors(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "test.yass.yaml", "---\ndescription: test\n---\nspec: MySpec\n")

	stdout, _, _ := runInDir(t, dir, []string{"test.yass.yaml"})

	if !strings.Contains(stdout, "checked 1 files, found 1 errors\n") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_SummaryAlwaysEmitted_ColonError(t *testing.T) {
	dir := setupProject(t)

	stdout, _, _ := runInDir(t, dir, []string{"bad:path"})

	if !strings.Contains(stdout, "checked 0 files, found 0 errors") {
		t.Errorf("expected summary even on colon error, got %q", stdout)
	}
}

func TestRun_SummaryAlwaysEmitted_PathNotFound(t *testing.T) {
	dir := setupProject(t)

	stdout, _, _ := runInDir(t, dir, []string{"missing.yass.yaml"})

	if !strings.Contains(stdout, "checked 0 files, found 0 errors") {
		t.Errorf("expected summary even on path error, got %q", stdout)
	}
}

func TestRun_SummaryIsFinalOutput(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "test.yass.yaml", "---\ndescription: test\n---\nspec: MySpec\n")

	stdout, _, _ := runInDir(t, dir, []string{"test.yass.yaml"})

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	lastLine := lines[len(lines)-1]
	if !strings.HasPrefix(lastLine, "checked ") {
		t.Errorf("expected summary as last output, got %q", lastLine)
	}
}

// --- Error output destination tests ---

func TestRun_ErrorsToStderr_SummaryToStdout(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "test.yass.yaml", "---\ndescription: test\n---\nspec: MySpec\n")

	stdout, stderr, _ := runInDir(t, dir, []string{"test.yass.yaml"})

	// Errors should be in stderr
	if !strings.Contains(stderr, "[yass.preamble.missing_version]") {
		t.Errorf("expected errors in stderr, got %q", stderr)
	}
	// Stdout should only have the summary, not error codes
	if strings.Contains(stdout, "[yass.") {
		t.Errorf("expected no error codes in stdout, got %q", stdout)
	}
	if !strings.Contains(stdout, "checked") {
		t.Errorf("expected summary in stdout, got %q", stdout)
	}
}

// --- Error line ordering tests ---

func TestRun_ErrorLineOrdering_WithinFile(t *testing.T) {
	dir := setupProject(t)
	// Two spec docs with unknown keys at different lines
	writeFile(t, dir, "multi.yass.yaml",
		"---\ndescription: test\nversion: v1\n---\nspec: MySpec\nINPUT:\n- MUST: \"do something\"\nBADKEY: invalid\n---\nspec: Other\nBADKEY2: invalid\n")

	_, stderr, code := runInDir(t, dir, []string{"multi.yass.yaml"})

	if code != exitcode.Processing {
		t.Errorf("expected exit %d, got %d", exitcode.Processing, code)
	}

	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 error lines, got %d: %q", len(lines), stderr)
	}

	// Verify line numbers are non-decreasing
	for i := 0; i < len(lines)-1; i++ {
		n1 := extractLineNumber(lines[i])
		n2 := extractLineNumber(lines[i+1])
		if n1 > n2 {
			t.Errorf("error lines not in order: line %d before line %d\n  %s\n  %s",
				n1, n2, lines[i], lines[i+1])
		}
	}
}

func TestRun_NoInterleaving(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "spec/a.yass.yaml", "---\ndescription: test\n---\nspec: A\n")
	writeFile(t, dir, "spec/b.yass.yaml", "---\ndescription: test\n---\nspec: B\n")

	_, stderr, code := runInDir(t, dir, nil)

	if code != exitcode.Processing {
		t.Errorf("expected exit %d, got %d", exitcode.Processing, code)
	}

	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 error lines, got %d", len(lines))
	}

	// Errors from a.yass.yaml should all precede errors from b.yass.yaml
	seenA, seenB, backToA := false, false, false
	for _, line := range lines {
		if strings.Contains(line, "a.yass.yaml") {
			seenA = true
			if seenB {
				backToA = true
			}
		}
		if strings.Contains(line, "b.yass.yaml") {
			seenB = true
		}
	}
	if backToA {
		t.Error("errors from different files were interleaved")
	}
	if !seenA || !seenB {
		t.Errorf("expected errors from both files; seenA=%v seenB=%v; stderr=%q", seenA, seenB, stderr)
	}
}

// extractLineNumber extracts the line number from an error line.
// Format: "<file>:<line>: [<code>] <message>" or "<file>: [<code>] <message>"
func extractLineNumber(errLine string) int {
	parts := strings.SplitN(errLine, ":", 3)
	if len(parts) < 3 {
		return 0
	}
	var n int
	fmt.Sscanf(parts[1], "%d", &n)
	return n
}

// --- Glob expansion tests ---

func TestRun_GlobExpansion_MatchesYassFiles(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "specs/a.yass.yaml", validSpec())
	writeFile(t, dir, "specs/b.yass.yaml", validSpec())
	writeFile(t, dir, "specs/readme.txt", "hello")

	stdout, stderr, code := runInDir(t, dir, []string{"specs/*.yass.yaml"})

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr=%q", exitcode.Success, code, stderr)
	}
	if !strings.Contains(stdout, "checked 2 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_GlobExpansion_SkipsNonYassFiles(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "specs/a.yass.yaml", validSpec())
	writeFile(t, dir, "specs/b.txt", "hello")

	// Glob matching everything in specs/*
	stdout, stderr, code := runInDir(t, dir, []string{"specs/*"})

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr=%q", exitcode.Success, code, stderr)
	}
	// Only the .yass.yaml file should be processed
	if !strings.Contains(stdout, "checked 1 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_GlobNoMatch_Exit2(t *testing.T) {
	dir := setupProject(t)

	stdout, stderr, code := runInDir(t, dir, []string{"nonexistent*.yass.yaml"})

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr, "yass.glob.no_match") {
		t.Errorf("expected glob.no_match, got %q", stderr)
	}
	if !strings.Contains(stdout, "checked 0 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_GlobExpansion_WildcardInDir(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "a.yass.yaml", validSpec())
	writeFile(t, dir, "b.yass.yaml", validSpec())

	stdout, stderr, code := runInDir(t, dir, []string{"*.yass.yaml"})

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr=%q", exitcode.Success, code, stderr)
	}
	if !strings.Contains(stdout, "checked 2 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

// --- Deduplication tests ---

func TestRun_Deduplication_SameFileTwice(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "test.yass.yaml", validSpec())

	stdout, stderr, code := runInDir(t, dir, []string{"test.yass.yaml", "test.yass.yaml"})

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr=%q", exitcode.Success, code, stderr)
	}
	if !strings.Contains(stdout, "checked 1 files, found 0 errors") {
		t.Errorf("expected 1 file after dedup, got %q", stdout)
	}
}

// --- CheckYAML failure skips other checks ---

func TestRun_CheckYAMLFailure_SkipsOtherChecks(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "empty.yass.yaml", "")

	_, stderr, code := runInDir(t, dir, []string{"empty.yass.yaml"})

	if code != exitcode.Processing {
		t.Errorf("expected exit %d, got %d", exitcode.Processing, code)
	}
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	if len(lines) != 1 {
		t.Errorf("expected exactly 1 error line (CheckYAML only), got %d: %v", len(lines), lines)
	}
	if !strings.Contains(stderr, "yass.yaml.empty_file") {
		t.Errorf("expected yaml.empty_file, got %q", stderr)
	}
}

func TestRun_CheckYAMLFailure_CountsAsOneError(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "malformed.yass.yaml", ":\n  - [invalid\n")

	stdout, stderr, code := runInDir(t, dir, []string{"malformed.yass.yaml"})

	if code != exitcode.Processing {
		t.Errorf("expected exit %d, got %d", exitcode.Processing, code)
	}
	if !strings.Contains(stdout, "found 1 errors") {
		t.Errorf("expected 1 error in summary, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestRun_CheckYAMLFailure_MalformedSingleError(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "malformed.yass.yaml", "not: valid: yaml: {{{\n")

	stdout, stderr, code := runInDir(t, dir, []string{"malformed.yass.yaml"})

	if code != exitcode.Processing {
		t.Errorf("expected exit %d, got %d", exitcode.Processing, code)
	}
	errorLines := strings.Split(strings.TrimSpace(stderr), "\n")
	if len(errorLines) != 1 {
		t.Errorf("expected exactly 1 error line for CheckYAML failure, got %d: %v", len(errorLines), errorLines)
	}
	if !strings.Contains(stdout, "found 1 errors") {
		t.Errorf("expected 1 error in summary, got %q", stdout)
	}
}

// --- Exit code tests ---

func TestRun_Exit0_NoErrors(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "clean.yass.yaml", validSpec())

	_, _, code := runInDir(t, dir, []string{"clean.yass.yaml"})

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d", exitcode.Success, code)
	}
}

func TestRun_Exit1_ValidationErrors(t *testing.T) {
	dir := setupProject(t)
	// Reserved name
	writeFile(t, dir, "reserved.yass.yaml",
		"---\ndescription: test\nversion: v1\n---\nspec: MUST\n")

	_, _, code := runInDir(t, dir, []string{"reserved.yass.yaml"})

	if code != exitcode.Processing {
		t.Errorf("expected exit %d, got %d", exitcode.Processing, code)
	}
}

func TestRun_Exit2_PathErrors(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "test.txt", "hello")

	tests := []struct {
		name string
		args []string
		code string
	}{
		{"not found", []string{"missing.yass.yaml"}, "yass.path.not_found"},
		{"bad extension", []string{"test.txt"}, "yass.path.bad_extension"},
		{"colon in path", []string{"foo:bar.yass.yaml"}, "yass.path.colon_in_path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, stderr, code := runInDir(t, dir, tt.args)
			if code != exitcode.Usage {
				t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
			}
			if !strings.Contains(stderr, tt.code) {
				t.Errorf("expected %s in stderr, got %q", tt.code, stderr)
			}
		})
	}
}

// --- Error line format tests ---

func TestRun_ErrorLineFormat_WithLine(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "spec.yass.yaml",
		"---\ndescription: test\nversion: v1\n---\nspec: .BadName\n")

	_, stderr, _ := runInDir(t, dir, []string{"spec.yass.yaml"})

	// Should have format: "file:line: [code] message"
	if !strings.Contains(stderr, "spec.yass.yaml:") {
		t.Errorf("expected file:line format, got %q", stderr)
	}
	if !strings.Contains(stderr, "[yass.spec.name_bad_form]") {
		t.Errorf("expected name_bad_form code, got %q", stderr)
	}
}

func TestRun_ErrorLineFormat_WithoutLine(t *testing.T) {
	dir := setupProject(t)

	_, stderr, _ := runInDir(t, dir, []string{"nonexistent.yass.yaml"})

	// Path not found should not have a line number
	if !strings.Contains(stderr, ": [yass.path.not_found]") {
		t.Errorf("expected path.not_found without line number, got %q", stderr)
	}
}

// --- File counting (N) tests ---

func TestRun_FileCount_IncludesFailedParseFiles(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "spec/good.yass.yaml", validSpec())
	// Empty file triggers CheckYAML failure
	writeFile(t, dir, "spec/bad.yass.yaml", "")

	stdout, _, code := runInDir(t, dir, nil)

	if code != exitcode.Processing {
		t.Errorf("expected exit %d, got %d", exitcode.Processing, code)
	}
	// Both files count toward N, even the failed one
	if !strings.Contains(stdout, "checked 2 files") {
		t.Errorf("expected 2 files counted, got %q", stdout)
	}
}

// --- Error count (M) tests ---

func TestRun_ErrorCount_MatchesStderrLines(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "badspec.yass.yaml",
		"---\ndescription: test\nversion: v1\n---\nFoo: bar\n")

	stdout, stderr, _ := runInDir(t, dir, []string{"badspec.yass.yaml"})

	stderrCount := countLines(stderr)
	expected := fmt.Sprintf("found %d errors", stderrCount)
	if !strings.Contains(stdout, expected) {
		t.Errorf("summary M=%q does not match stderr line count %d; stderr=%q", stdout, stderrCount, stderr)
	}
}

// --- Edge cases ---

func TestRun_EmptyDirectoryArg_NoFiles(t *testing.T) {
	dir := setupProject(t)
	emptyDir := filepath.Join(dir, "empty_sub")
	os.MkdirAll(emptyDir, 0755)

	stdout, stderr, code := runInDir(t, dir, []string{"empty_sub"})

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr, "yass.discover.no_files") {
		t.Errorf("expected no_files error, got %q", stderr)
	}
	if !strings.Contains(stdout, "checked 0 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_MultipleColonsInPath(t *testing.T) {
	dir := setupProject(t)

	_, stderr, code := runInDir(t, dir, []string{"a:b:c.yass.yaml"})

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr, "yass.path.colon_in_path") {
		t.Errorf("expected colon_in_path, got %q", stderr)
	}
}

func TestRun_PreambleVersionError(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "badversion.yass.yaml",
		"---\ndescription: test\nversion: v2\n")

	_, stderr, code := runInDir(t, dir, []string{"badversion.yass.yaml"})

	if code != exitcode.Processing {
		t.Errorf("expected exit %d, got %d", exitcode.Processing, code)
	}
	if !strings.Contains(stderr, "yass.preamble.unknown_version") {
		t.Errorf("expected unknown_version, got %q", stderr)
	}
}

func TestRun_ValidCompleteFile(t *testing.T) {
	dir := setupProject(t)
	content := "---\ndescription: A test spec\nversion: v1\n---\nspec: Valid.Spec-Name\nINPUT:\n- MUST: \"accept valid input\"\nRETURN:\n- MUST: \"return valid output\"\n"
	writeFile(t, dir, "valid.yass.yaml", content)

	stdout, stderr, code := runInDir(t, dir, []string{"valid.yass.yaml"})

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr=%q", exitcode.Success, code, stderr)
	}
	if stderr != "" {
		t.Errorf("expected no stderr, got %q", stderr)
	}
	if stdout != "checked 1 files, found 0 errors\n" {
		t.Errorf("expected exact summary, got %q", stdout)
	}
}

func TestRun_ProjectRootComputedOnce(t *testing.T) {
	dir := setupProject(t)
	// File A references file B via project-root-relative path
	writeFile(t, dir, "spec/a.yass.yaml",
		"---\ndescription: test\nversion: v1\n---\nspec: SpecA\nINPUT:\n- MUST: \"do A\"\n  USES: spec/b@SpecB\n")
	writeFile(t, dir, "spec/b.yass.yaml",
		"---\ndescription: test\nversion: v1\n---\nspec: SpecB\nINPUT:\n- MUST: \"do B\"\n")

	stdout, stderr, code := runInDir(t, dir, nil)

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr=%q", exitcode.Success, code, stderr)
	}
	if !strings.Contains(stdout, "checked 2 files, found 0 errors") {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestRun_FilePermissionDenied(t *testing.T) {
	dir := setupProject(t)
	fp := writeFile(t, dir, "unreadable.yass.yaml", validSpec())
	if err := os.Chmod(fp, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chmod(fp, 0644)
	})

	_, stderr, code := runInDir(t, dir, []string{"unreadable.yass.yaml"})

	// File exists but cannot be read -- CheckYAML will fail (exit 1)
	// or DiscoverSpecFiles might catch it (exit 2).
	if code != exitcode.Processing && code != exitcode.Usage {
		t.Errorf("expected exit 1 or 2, got %d; stderr=%q", code, stderr)
	}
	if stderr == "" {
		t.Error("expected some error in stderr")
	}
}
