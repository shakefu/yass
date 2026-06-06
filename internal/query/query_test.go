package query

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shakefu/yass/internal/exitcode"
)

// setupQueryTestDir creates a temp dir with .git marker and spec files.
func setupQueryTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create .git marker so FindProjectRoot works
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("creating .git: %v", err)
	}

	return dir
}

func TestRun_NoArgs(t *testing.T) {
	dir := setupQueryTestDir(t)
	createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	// Change to the test dir so FindProjectRoot works
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	var stdout, stderr bytes.Buffer
	code := Run([]string{}, &stdout, &stderr)

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.query.name_missing") {
		t.Errorf("expected name_missing error, got: %s", stderr.String())
	}
}

func TestRun_SingleMatch(t *testing.T) {
	dir := setupQueryTestDir(t)
	createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test file
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"MySpec"}, &stdout, &stderr)

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr: %s", exitcode.Success, code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "---\n") {
		t.Error("expected YAML fragment starting with '---\\n'")
	}
	if !strings.Contains(stdout.String(), "spec: MySpec") {
		t.Error("expected fragment to contain 'spec: MySpec'")
	}
	if stderr.Len() > 0 {
		t.Errorf("expected no stderr, got: %s", stderr.String())
	}
}

func TestRun_MultipleMatches(t *testing.T) {
	dir := setupQueryTestDir(t)
	createTestSpecFile(t, dir, "a.yass.yaml", `---
description: file a
version: v1
---
spec: Foo
RETURN:
- MUST: one
`)
	createTestSpecFile(t, dir, "b.yass.yaml", `---
description: file b
version: v1
---
spec: pkg.Foo
RETURN:
- MUST: two
`)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"Foo"}, &stdout, &stderr)

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr: %s", exitcode.Success, code, stderr.String())
	}
	// Should have tab-separated rows
	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %s", len(lines), stdout.String())
	}
	for _, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			t.Errorf("expected 3 tab-separated fields, got %d: %s", len(parts), line)
		}
	}
	// No stderr
	if stderr.Len() > 0 {
		t.Errorf("expected no stderr for multi-match, got: %s", stderr.String())
	}
}

func TestRun_ZeroMatches(t *testing.T) {
	dir := setupQueryTestDir(t)
	createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"NonExistent"}, &stdout, &stderr)

	if code != exitcode.Processing {
		t.Errorf("expected exit %d, got %d", exitcode.Processing, code)
	}
	if !strings.Contains(stderr.String(), "yass.query.no_match") {
		t.Errorf("expected no_match error, got: %s", stderr.String())
	}
	if stdout.Len() > 0 {
		t.Errorf("expected no stdout for zero matches, got: %s", stdout.String())
	}
}

func TestRun_ColonInScope(t *testing.T) {
	dir := setupQueryTestDir(t)
	createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"MySpec", "path:with:colon"}, &stdout, &stderr)

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.path.colon_in_path") {
		t.Errorf("expected colon_in_path error, got: %s", stderr.String())
	}
}

func TestRun_ScopeNotFound(t *testing.T) {
	dir := setupQueryTestDir(t)
	createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"MySpec", "nonexistent_dir"}, &stdout, &stderr)

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.query.scope_not_found") {
		t.Errorf("expected scope_not_found error, got: %s", stderr.String())
	}
}

func TestRun_ScopeEmpty(t *testing.T) {
	dir := setupQueryTestDir(t)
	// Create a subdirectory with no .yass.yaml files
	emptyDir := filepath.Join(dir, "empty")
	os.Mkdir(emptyDir, 0755)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"MySpec", "empty"}, &stdout, &stderr)

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.query.scope_empty") {
		t.Errorf("expected scope_empty error, got: %s", stderr.String())
	}
}

func TestRun_ScopeValidatedBeforeLookup(t *testing.T) {
	dir := setupQueryTestDir(t)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Bad scope + bad name: should get scope error, not name error
	var stdout, stderr bytes.Buffer
	code := Run([]string{"WontMatch", "nonexistent_dir"}, &stdout, &stderr)

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	// Should get scope_not_found, NOT no_match
	if strings.Contains(stderr.String(), "yass.query.no_match") {
		t.Error("should emit scope error, not name match error")
	}
}

func TestRun_ConformsFailure(t *testing.T) {
	dir := setupQueryTestDir(t)
	createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN:
- CONFORMS: ./nonexistent@Target::RETURN
`)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"MySpec"}, &stdout, &stderr)

	if code != exitcode.Processing {
		t.Errorf("expected exit %d, got %d", exitcode.Processing, code)
	}
	if !strings.Contains(stderr.String(), "yass.query.conforms_unresolved") {
		t.Errorf("expected conforms_unresolved error, got: %s", stderr.String())
	}
	if stdout.Len() > 0 {
		t.Errorf("expected no stdout on CONFORMS failure, got: %s", stdout.String())
	}
}

func TestRun_ConformsNoSlotFailure(t *testing.T) {
	dir := setupQueryTestDir(t)
	createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN:
- CONFORMS: SomeSpec
`)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"MySpec"}, &stdout, &stderr)

	if code != exitcode.Processing {
		t.Errorf("expected exit %d, got %d", exitcode.Processing, code)
	}
	if !strings.Contains(stderr.String(), "yass.query.conforms_no_slot") {
		t.Errorf("expected conforms_no_slot error, got: %s", stderr.String())
	}
}

func TestRun_ScopeWithFile(t *testing.T) {
	dir := setupQueryTestDir(t)
	createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)
	createTestSpecFile(t, dir, "other.yass.yaml", `---
description: other
version: v1
---
spec: MySpec
RETURN:
- MUST: do other thing
`)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Scope to specific file: should find only the one in test.yass.yaml
	var stdout, stderr bytes.Buffer
	code := Run([]string{"MySpec", "test.yass.yaml"}, &stdout, &stderr)

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr: %s", exitcode.Success, code, stderr.String())
	}
	// Should emit a single fragment, not a multi-match list
	if !strings.HasPrefix(stdout.String(), "---\n") {
		t.Error("expected YAML fragment")
	}
}

func TestRun_WhitespaceInName(t *testing.T) {
	dir := setupQueryTestDir(t)
	createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Whitespace in name -> treated as no-match (not blank error)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"My Spec"}, &stdout, &stderr)

	if code != exitcode.Processing {
		t.Errorf("expected exit %d (no match), got %d", exitcode.Processing, code)
	}
	if !strings.Contains(stderr.String(), "yass.query.no_match") {
		t.Errorf("expected no_match error for whitespace name, got: %s", stderr.String())
	}
}

func TestRun_EmptySpecName(t *testing.T) {
	dir := setupQueryTestDir(t)
	createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Empty string as name -> name_blank
	var stdout, stderr bytes.Buffer
	code := Run([]string{""}, &stdout, &stderr)

	if code != exitcode.Usage {
		t.Errorf("expected exit %d, got %d", exitcode.Usage, code)
	}
	if !strings.Contains(stderr.String(), "yass.query.name_blank") {
		t.Errorf("expected name_blank error, got: %s", stderr.String())
	}
}

func TestRun_SuffixMatch(t *testing.T) {
	dir := setupQueryTestDir(t)
	createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: pkg.MySpec
RETURN:
- MUST: do something
`)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"MySpec"}, &stdout, &stderr)

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr: %s", exitcode.Success, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "spec: pkg.MySpec") {
		t.Error("expected fragment to contain the full spec name 'pkg.MySpec'")
	}
}

func TestRun_ConformsInlining(t *testing.T) {
	dir := setupQueryTestDir(t)

	createTestSpecFile(t, dir, "base.yass.yaml", `---
description: base spec
version: v1
---
spec: Base
RETURN:
- MUST: exit 0 on success
`)

	createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN:
- MUST: do my thing
  CONFORMS: ./base@Base::RETURN
`)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"MySpec"}, &stdout, &stderr)

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d; stderr: %s", exitcode.Success, code, stderr.String())
	}

	output := stdout.String()
	// Should have the provenance comment
	if !strings.Contains(output, "# CONFORMS: ./base@Base::RETURN") {
		t.Errorf("expected provenance comment in output, got:\n%s", output)
	}
	// Should contain the inlined obligation
	if !strings.Contains(output, "exit 0 on success") {
		t.Errorf("expected inlined obligation content, got:\n%s", output)
	}
}

func TestRun_MultiMatchNoTruncation(t *testing.T) {
	dir := setupQueryTestDir(t)
	longDesc := strings.Repeat("x", 500)
	createTestSpecFile(t, dir, "a.yass.yaml", `---
description: `+longDesc+`
version: v1
---
spec: Foo
RETURN:
- MUST: one
`)
	createTestSpecFile(t, dir, "b.yass.yaml", `---
description: `+longDesc+`
version: v1
---
spec: pkg.Foo
RETURN:
- MUST: two
`)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"Foo"}, &stdout, &stderr)

	if code != exitcode.Success {
		t.Errorf("expected exit %d, got %d", exitcode.Success, code)
	}

	// Descriptions should NOT be truncated regardless of TTY
	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	for _, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) >= 3 && len(parts[2]) < 400 {
			t.Errorf("description appears truncated: %d chars", len(parts[2]))
		}
	}
}
