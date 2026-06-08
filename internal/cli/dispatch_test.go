package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shakefu/yass/internal/exitcode"
	"github.com/shakefu/yass/internal/version"
)

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

// validSpec returns minimal valid .yass.yaml content.
func validSpec() string {
	return "---\ndescription: test\nversion: v1\n---\nspec: MySpec\nINPUT:\n- MUST: \"do something\"\n"
}

// runInDir runs Dispatch while cwd is set to dir.
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
	code := Dispatch(args, &stdout, &stderr, false, 80)
	return stdout.String(), stderr.String(), code
}

func TestDispatch_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch(nil, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.no_subcommand") {
		t.Errorf("expected no_subcommand error in stderr, got: %s", stderr.String())
	}
}

func TestDispatch_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"--help"}, &stdout, &stderr, false, 80)

	if code != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("expected Usage in stdout, got: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr, got: %s", stderr.String())
	}
}

func TestDispatch_Version(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"--version"}, &stdout, &stderr, false, 80)

	if code != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, code)
	}
	expected := "yass " + version.Version + "\n"
	if stdout.String() != expected {
		t.Errorf("expected %q, got %q", expected, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr, got: %s", stderr.String())
	}
}

func TestDispatch_HelpMixedWithArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"validate", "--help", "foo"}, &stdout, &stderr, false, 80)

	if code != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("expected Usage in stdout, got: %s", stdout.String())
	}
}

func TestDispatch_VersionMixedWithArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"validate", "--version", "foo"}, &stdout, &stderr, false, 80)

	if code != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, code)
	}
	expected := "yass " + version.Version + "\n"
	if stdout.String() != expected {
		t.Errorf("expected %q, got %q", expected, stdout.String())
	}
}

func TestDispatch_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"foo"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.unknown_subcommand") {
		t.Errorf("expected unknown_subcommand error, got: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "foo") {
		t.Errorf("expected 'foo' in error message, got: %s", stderr.String())
	}
}

func TestDispatch_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"--foo"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.unknown_flag") {
		t.Errorf("expected unknown_flag error, got: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "--foo") {
		t.Errorf("expected '--foo' in error message, got: %s", stderr.String())
	}
}

func TestDispatch_ShortFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"-v"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.short_flag") {
		t.Errorf("expected short_flag error, got: %s", stderr.String())
	}
}

func TestDispatch_CaseMismatchSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"Validate"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.case_mismatch") {
		t.Errorf("expected case_mismatch error, got: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Validate") {
		t.Errorf("expected 'Validate' in error message, got: %s", stderr.String())
	}
}

func TestDispatch_CaseMismatchFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"--Help"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.case_mismatch") {
		t.Errorf("expected case_mismatch error, got: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "--Help") {
		t.Errorf("expected '--Help' in error message, got: %s", stderr.String())
	}
}

func TestDispatch_Abbreviation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"val"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.abbreviation") {
		t.Errorf("expected abbreviation error, got: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "val") {
		t.Errorf("expected 'val' in error message, got: %s", stderr.String())
	}
}

func TestDispatch_EmptyArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{""}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.empty_argument") {
		t.Errorf("expected empty_argument error, got: %s", stderr.String())
	}
}

func TestDispatch_BareDash(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"-"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.stdin_dash") {
		t.Errorf("expected stdin_dash error, got: %s", stderr.String())
	}
}

func TestDispatch_DashDashFollowedByArgs(t *testing.T) {
	// "--" followed by a known subcommand name should treat it as positional
	// and attempt to dispatch it. This tests the end-of-options marker.
	var stdout, stderr bytes.Buffer
	_ = Dispatch([]string{"--", "validate"}, &stdout, &stderr, false, 80)

	// "validate" after "--" is treated as a positional = subcommand.
	// It should dispatch to validate. validate with no args discovers files.
	// The exact exit code depends on whether there are .yass.yaml files in cwd.
	// Just verify it doesn't fail with "unknown subcommand" - it should dispatch.
	if strings.Contains(stderr.String(), "yass.argv.unknown_subcommand") {
		t.Errorf("should dispatch 'validate' after '--', not treat as unknown subcommand")
	}
}

func TestDispatch_MultipleErrorsFirstWins(t *testing.T) {
	// Empty string comes before unknown flag.
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"", "--foo"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.empty_argument") {
		t.Errorf("expected empty_argument error (first error wins), got: %s", stderr.String())
	}
	_ = stdout
}

func TestDispatch_ValidateDispatches(t *testing.T) {
	// Just verify it dispatches (exit code depends on environment).
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"validate"}, &stdout, &stderr, false, 80)

	// validate with no args tries to discover files from project root.
	// It may succeed or fail depending on cwd. Just verify it doesn't
	// return a dispatch-level error.
	if strings.Contains(stderr.String(), "yass.argv.") {
		t.Errorf("should dispatch to validate, not produce argv error: %s", stderr.String())
	}
	_ = code
}

func TestDispatch_QueryDispatches(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"query", "someName"}, &stdout, &stderr, false, 80)

	// query dispatches with args. May fail with no match or no files, but
	// should not produce argv errors.
	if strings.Contains(stderr.String(), "yass.argv.") {
		t.Errorf("should dispatch to query, not produce argv error: %s", stderr.String())
	}
	_ = code
}

func TestDispatch_ListDispatches(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"list"}, &stdout, &stderr, false, 80)

	// list dispatches. May succeed or fail, but should not produce argv errors.
	if strings.Contains(stderr.String(), "yass.argv.") {
		t.Errorf("should dispatch to list, not produce argv error: %s", stderr.String())
	}
	_ = code
}

func TestDispatch_ShortFlagBeforeSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"-v", "validate"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.short_flag") {
		t.Errorf("expected short_flag error, got: %s", stderr.String())
	}
}

func TestDispatch_UnknownFlagBeforeSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"--verbose", "validate"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.unknown_flag") {
		t.Errorf("expected unknown_flag error, got: %s", stderr.String())
	}
}

func TestDispatch_AbbreviationQue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"que"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.abbreviation") {
		t.Errorf("expected abbreviation error, got: %s", stderr.String())
	}
}

func TestDispatch_AbbreviationLi(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"li"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.abbreviation") {
		t.Errorf("expected abbreviation error, got: %s", stderr.String())
	}
}

func TestDispatch_CaseMismatchVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"--Version"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.case_mismatch") {
		t.Errorf("expected case_mismatch error, got: %s", stderr.String())
	}
}

func TestDispatch_CaseMismatchQuery(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"Query"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.case_mismatch") {
		t.Errorf("expected case_mismatch error, got: %s", stderr.String())
	}
}

func TestDispatch_CaseMismatchList(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"LIST"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.case_mismatch") {
		t.Errorf("expected case_mismatch error, got: %s", stderr.String())
	}
}

func TestDispatch_VersionNoVPrefix(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"--version"}, &stdout, &stderr, false, 80)

	if code != exitcode.Success {
		t.Errorf("expected exit code %d, got %d", exitcode.Success, code)
	}
	output := stdout.String()
	if strings.HasPrefix(output, "yass v") {
		t.Errorf("version should not have 'v' prefix, got: %s", output)
	}
	if !strings.HasPrefix(output, "yass ") {
		t.Errorf("version should start with 'yass ', got: %s", output)
	}
	if !strings.HasSuffix(output, "\n") {
		t.Errorf("version should end with newline, got: %q", output)
	}
}

func TestDispatch_EmptyArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.no_subcommand") {
		t.Errorf("expected no_subcommand error, got: %s", stderr.String())
	}
}

func TestDispatch_DashDashOnly(t *testing.T) {
	// Just "--" with nothing after it: no subcommand.
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"--"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.no_subcommand") {
		t.Errorf("expected no_subcommand error, got: %s", stderr.String())
	}
}

func TestDispatch_DashDashEmptyString(t *testing.T) {
	// "--" followed by empty string.
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"--", ""}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.empty_argument") {
		t.Errorf("expected empty_argument error, got: %s", stderr.String())
	}
}

func TestDispatch_UsageOutputFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	Dispatch([]string{"--help"}, &stdout, &stderr, false, 80)

	output := stdout.String()
	if !strings.Contains(output, "validate") {
		t.Errorf("usage should mention 'validate', got: %s", output)
	}
	if !strings.Contains(output, "query") {
		t.Errorf("usage should mention 'query', got: %s", output)
	}
	if !strings.Contains(output, "list") {
		t.Errorf("usage should mention 'list', got: %s", output)
	}
	if !strings.Contains(output, "--help") {
		t.Errorf("usage should mention '--help', got: %s", output)
	}
	if !strings.Contains(output, "--version") {
		t.Errorf("usage should mention '--version', got: %s", output)
	}
}

func TestDispatch_ErrorLineFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	Dispatch([]string{"foo"}, &stdout, &stderr, false, 80)

	output := stderr.String()
	if !strings.Contains(output, "yass.argv.unknown_subcommand") {
		t.Errorf("expected error code in stderr, got: %s", output)
	}
	// Spec invariant: stderr must not contain non-ErrorLine text like "Usage:".
	if strings.Contains(output, "Usage:") {
		t.Errorf("stderr must not contain non-ErrorLine text like Usage, got: %s", output)
	}
}

func TestDispatch_ShortFlagH(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Dispatch([]string{"-h"}, &stdout, &stderr, false, 80)

	if code != exitcode.Usage {
		t.Errorf("expected exit code %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.argv.short_flag") {
		t.Errorf("expected short_flag error, got: %s", stderr.String())
	}
}

func TestDispatch_ValidateIntegration(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "test.yass.yaml", validSpec())

	stdout, _, code := runInDir(t, dir, []string{"validate", "test.yass.yaml"})
	if code != exitcode.Success {
		t.Errorf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "checked 1 files") {
		t.Errorf("expected 'checked 1 files', got: %s", stdout)
	}
}

func TestDispatch_ValidatePassesRemainingArgs(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "a.yass.yaml", validSpec())
	writeFile(t, dir, "b.yass.yaml", validSpec())

	stdout, _, code := runInDir(t, dir, []string{"validate", "a.yass.yaml", "b.yass.yaml"})
	if code != exitcode.Success {
		t.Errorf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "checked 2 files") {
		t.Errorf("expected 'checked 2 files', got: %s", stdout)
	}
}

func TestDispatch_ListIntegration(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "test.yass.yaml", validSpec())

	stdout, _, code := runInDir(t, dir, []string{"list"})
	if code != exitcode.Success {
		t.Errorf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "MySpec") {
		t.Errorf("expected spec name in output, got: %s", stdout)
	}
}

func TestDispatch_QueryIntegration(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "test.yass.yaml", validSpec())

	stdout, _, code := runInDir(t, dir, []string{"query", "MySpec"})
	if code != exitcode.Success {
		t.Errorf("expected exit 0, got %d", code)
	}
	if stdout == "" {
		t.Error("expected query output, got empty string")
	}
}

func TestDispatch_DashDashValidateWithFile(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, dir, "test.yass.yaml", validSpec())

	stdout, _, code := runInDir(t, dir, []string{"--", "validate", "test.yass.yaml"})
	if code != exitcode.Success {
		t.Errorf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "checked") {
		t.Errorf("expected validate output, got: %s", stdout)
	}
}
