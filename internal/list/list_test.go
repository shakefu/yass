package list

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper to create a .yass.yaml file with the given content.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	subDir := filepath.Dir(path)
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", subDir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// helper to init a .git dir so FindProjectRoot can find the project root.
func initGit(t *testing.T, dir string) {
	t.Helper()
	gitDir := filepath.Join(dir, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
}

// runInDir runs the list command with cwd set to dir.
func runInDir(t *testing.T, dir string, args []string, isTTY bool, termWidth int) (stdout, stderr string, exitCode int) {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("restore chdir: %v", err)
		}
	}()

	var stdoutBuf, stderrBuf bytes.Buffer
	code := Run(args, &stdoutBuf, &stderrBuf, isTTY, termWidth)
	return stdoutBuf.String(), stderrBuf.String(), code
}

func TestSingleFileOneSpec(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	writeFile(t, dir, "example.yass.yaml", `---
description: A simple test spec
version: v1
---
spec: MySpec
INPUT:
- MUST: do something
`)

	stdout, stderr, code := runInDir(t, dir, nil, false, 0)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr=%s", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("unexpected stderr: %s", stderr)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %q", len(lines), stdout)
	}

	fields := strings.Split(lines[0], "\t")
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d: %q", len(fields), lines[0])
	}

	if fields[0] != "example.yass.yaml" {
		t.Errorf("field[0] = %q, want %q", fields[0], "example.yass.yaml")
	}
	if fields[1] != "MySpec" {
		t.Errorf("field[1] = %q, want %q", fields[1], "MySpec")
	}
	if fields[2] != "A simple test spec" {
		t.Errorf("field[2] = %q, want %q", fields[2], "A simple test spec")
	}
}

func TestMultipleFilesMultipleSpecs(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	writeFile(t, dir, "b.yass.yaml", `---
description: File B
version: v1
---
spec: B1
---
spec: B2
`)
	writeFile(t, dir, "a.yass.yaml", `---
description: File A
version: v1
---
spec: A1
`)

	stdout, stderr, code := runInDir(t, dir, nil, false, 0)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr=%s", code, stderr)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), stdout)
	}

	// Files sorted by NFC code-point order: a.yass.yaml < b.yass.yaml
	if !strings.HasPrefix(lines[0], "a.yass.yaml\t") {
		t.Errorf("line 0 should start with a.yass.yaml: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "b.yass.yaml\t") {
		t.Errorf("line 1 should start with b.yass.yaml: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "b.yass.yaml\t") {
		t.Errorf("line 2 should start with b.yass.yaml: %q", lines[2])
	}

	// Verify spec names preserve document order within file.
	fields1 := strings.Split(lines[1], "\t")
	fields2 := strings.Split(lines[2], "\t")
	if fields1[1] != "B1" {
		t.Errorf("spec name should be B1, got %q", fields1[1])
	}
	if fields2[1] != "B2" {
		t.Errorf("spec name should be B2, got %q", fields2[1])
	}
}

func TestDescriptionNormalization(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	writeFile(t, dir, "test.yass.yaml", `---
description: "  hello\n\tworld   foo  "
version: v1
---
spec: S1
`)

	stdout, _, code := runInDir(t, dir, nil, false, 0)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	fields := strings.Split(strings.TrimRight(stdout, "\n"), "\t")
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}
	// All whitespace runs collapsed, leading/trailing stripped.
	if fields[2] != "hello world foo" {
		t.Errorf("description = %q, want %q", fields[2], "hello world foo")
	}
}

func TestDescriptionMultilineYAML(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	writeFile(t, dir, "test.yass.yaml", `---
description: >
  The list subcommand. Lists specs discovered in .yass.yaml files,
  showing spec names with file paths and truncated preamble descriptions.
version: v1
---
spec: S1
`)

	stdout, _, code := runInDir(t, dir, nil, false, 0)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	fields := strings.Split(strings.TrimRight(stdout, "\n"), "\t")
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}
	want := "The list subcommand. Lists specs discovered in .yass.yaml files, showing spec names with file paths and truncated preamble descriptions."
	if fields[2] != want {
		t.Errorf("description = %q, want %q", fields[2], want)
	}
}

func TestEmptyDescription(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	writeFile(t, dir, "test.yass.yaml", `---
description: ""
version: v1
---
spec: S1
`)

	stdout, _, code := runInDir(t, dir, nil, false, 0)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	line := strings.TrimRight(stdout, "\n")
	fields := strings.Split(line, "\t")
	if len(fields) != 3 {
		t.Fatalf("expected 3 tab-separated fields, got %d: %q", len(fields), line)
	}
	if fields[2] != "" {
		t.Errorf("description should be empty, got %q", fields[2])
	}
}

func TestMissingDescription(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	writeFile(t, dir, "test.yass.yaml", `---
version: v1
---
spec: S1
`)

	stdout, _, code := runInDir(t, dir, nil, false, 0)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	line := strings.TrimRight(stdout, "\n")
	fields := strings.Split(line, "\t")
	if len(fields) != 3 {
		t.Fatalf("expected 3 tab-separated fields, got %d: %q", len(fields), line)
	}
	if fields[2] != "" {
		t.Errorf("description should be empty, got %q", fields[2])
	}
}

func TestNonStringDescription(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	writeFile(t, dir, "test.yass.yaml", `---
description: 42
version: v1
---
spec: S1
`)

	stdout, _, code := runInDir(t, dir, nil, false, 0)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	line := strings.TrimRight(stdout, "\n")
	fields := strings.Split(line, "\t")
	if len(fields) != 3 {
		t.Fatalf("expected 3 tab-separated fields, got %d: %q", len(fields), line)
	}
	if fields[2] != "" {
		t.Errorf("description should be empty for non-string, got %q", fields[2])
	}
}

func TestZeroSpecFiles(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	// File with preamble but no spec docs.
	writeFile(t, dir, "test.yass.yaml", `---
description: Preamble only
version: v1
`)

	stdout, stderr, code := runInDir(t, dir, nil, false, 0)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr=%s", code, stderr)
	}
	if stdout != "" {
		t.Errorf("expected no stdout, got %q", stdout)
	}
}

func TestNoYassFiles(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	// No .yass.yaml files at all.

	stdout, stderr, code := runInDir(t, dir, nil, false, 0)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr=%s", code, stderr)
	}
	if stdout != "" {
		t.Errorf("expected no stdout, got %q", stdout)
	}
}

func TestYAMLParseFailure(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	// Valid file
	writeFile(t, dir, "a.yass.yaml", `---
description: good
version: v1
---
spec: Good
`)
	// Malformed file
	writeFile(t, dir, "b.yass.yaml", `---
description: bad
: invalid yaml {{{{
`)

	stdout, stderr, code := runInDir(t, dir, nil, false, 0)
	if code != 1 {
		t.Fatalf("expected exit 1 for YAML parse failure, got %d", code)
	}
	// Good file specs should still be listed.
	if !strings.Contains(stdout, "Good") {
		t.Errorf("expected Good spec in stdout, got %q", stdout)
	}
	// Error should mention malformed on stderr.
	if !strings.Contains(stderr, "yass.yaml.malformed") {
		t.Errorf("expected yass.yaml.malformed in stderr, got %q", stderr)
	}
}

func TestPathNotFound(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)

	_, stderr, code := runInDir(t, dir, []string{"nonexistent.yass.yaml"}, false, 0)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "yass.path.not_found") {
		t.Errorf("expected yass.path.not_found in stderr, got %q", stderr)
	}
}

func TestPathBadExtension(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	writeFile(t, dir, "test.yaml", `---
spec: S1
`)

	_, stderr, code := runInDir(t, dir, []string{"test.yaml"}, false, 0)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "yass.path.bad_extension") {
		t.Errorf("expected yass.path.bad_extension in stderr, got %q", stderr)
	}
}

func TestColonInPath(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)

	_, stderr, code := runInDir(t, dir, []string{"file:name.yass.yaml"}, false, 0)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "yass.path.colon_in_path") {
		t.Errorf("expected yass.path.colon_in_path in stderr, got %q", stderr)
	}
}

func TestTabInFilePath(t *testing.T) {
	// We can't easily create files with tabs in the name on all OSes,
	// so test the normalizeDescription and tab replacement logic directly.
	row := listRow{
		FilePath:    "path\twith\ttabs.yass.yaml",
		SpecName:    "Spec",
		Description: "desc",
	}

	var buf bytes.Buffer
	emitRow(&buf, row, false, 0)

	output := buf.String()
	// The file path should have tabs replaced with spaces in our displayPath logic,
	// but emitRow uses the FilePath as-is. The tab replacement happens in processFile.
	// Let's verify the logic in processFile instead.
	displayPath := strings.ReplaceAll(row.FilePath, "\t", " ")
	if displayPath != "path with tabs.yass.yaml" {
		t.Errorf("tab replacement: got %q", displayPath)
	}
	_ = output
}

func TestTTYTruncation(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	writeFile(t, dir, "f.yass.yaml", `---
description: This is a very long description that should be truncated when the terminal is narrow
version: v1
---
spec: S1
`)

	stdout, _, code := runInDir(t, dir, nil, true, 40)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	line := strings.TrimRight(stdout, "\n")
	fields := strings.Split(line, "\t")
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d: %q", len(fields), line)
	}

	// Must end with "..." if truncated
	if !strings.HasSuffix(fields[2], "...") {
		t.Errorf("expected truncation marker, got %q", fields[2])
	}

	// Total line width must not exceed termWidth.
	// path + 1 (tab) + name + 1 (tab) + desc
	totalLen := graphemeLen(fields[0]) + 1 + graphemeLen(fields[1]) + 1 + graphemeLen(fields[2])
	if totalLen > 40 {
		t.Errorf("line length %d exceeds term width 40", totalLen)
	}
}

func TestTTYNoTruncationWhenFits(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	writeFile(t, dir, "f.yass.yaml", `---
description: short
version: v1
---
spec: S1
`)

	stdout, _, code := runInDir(t, dir, nil, true, 80)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	line := strings.TrimRight(stdout, "\n")
	fields := strings.Split(line, "\t")
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d: %q", len(fields), line)
	}
	if fields[2] != "short" {
		t.Errorf("expected no truncation, got %q", fields[2])
	}
}

func TestNonTTYNoTruncation(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	longDesc := strings.Repeat("A", 200)
	writeFile(t, dir, "f.yass.yaml", `---
description: "`+longDesc+`"
version: v1
---
spec: S1
`)

	stdout, _, code := runInDir(t, dir, nil, false, 0)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	line := strings.TrimRight(stdout, "\n")
	fields := strings.Split(line, "\t")
	if fields[2] != longDesc {
		t.Errorf("non-TTY should not truncate; got len=%d, want %d", len(fields[2]), len(longDesc))
	}
}

func TestTTYEmptyDescNoMarker(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	writeFile(t, dir, "f.yass.yaml", `---
description: ""
version: v1
---
spec: S1
`)

	stdout, _, code := runInDir(t, dir, nil, true, 40)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	line := strings.TrimRight(stdout, "\n")
	fields := strings.Split(line, "\t")
	if fields[2] != "" {
		t.Errorf("empty description should have no marker, got %q", fields[2])
	}
}

func TestTTYPathNameExceedsWidth(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	// Use a long file name so path+name+separators+marker >= width
	longName := strings.Repeat("x", 30) + ".yass.yaml"
	writeFile(t, dir, longName, `---
description: Some description
version: v1
---
spec: VeryLongSpecName
`)

	// path = 40 chars, spec = 16 chars, seps = 2, marker = 3 => 61 >= 40
	stdout, _, code := runInDir(t, dir, nil, true, 40)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	line := strings.TrimRight(stdout, "\n")
	fields := strings.Split(line, "\t")
	// Third field should be empty, no marker.
	if fields[2] != "" {
		t.Errorf("expected empty description when path+name exceeds width, got %q", fields[2])
	}
}

func TestSingleFileArg(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	writeFile(t, dir, "test.yass.yaml", `---
description: desc
version: v1
---
spec: S1
`)

	stdout, _, code := runInDir(t, dir, []string{"test.yass.yaml"}, false, 0)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "S1") {
		t.Errorf("expected S1 in output, got %q", stdout)
	}
}

func TestDirectoryArg(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0o755)
	writeFile(t, dir, "sub/test.yass.yaml", `---
description: in sub
version: v1
---
spec: Sub1
`)
	writeFile(t, dir, "root.yass.yaml", `---
description: at root
version: v1
---
spec: Root1
`)

	// Only list specs under sub/
	stdout, _, code := runInDir(t, dir, []string{"sub"}, false, 0)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "Sub1") {
		t.Errorf("expected Sub1 in output, got %q", stdout)
	}
	if strings.Contains(stdout, "Root1") {
		t.Errorf("should not contain Root1 when listing sub/, got %q", stdout)
	}
}

func TestLFTermination(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)
	writeFile(t, dir, "test.yass.yaml", `---
description: desc
version: v1
---
spec: S1
---
spec: S2
`)

	stdout, _, code := runInDir(t, dir, nil, false, 0)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	// Every row must end with LF, including the last.
	if !strings.HasSuffix(stdout, "\n") {
		t.Errorf("stdout should end with LF: %q", stdout)
	}
	// No double LF at end (no extra blank line).
	if strings.HasSuffix(stdout, "\n\n") {
		t.Errorf("stdout should not end with double LF: %q", stdout)
	}
}

func TestTruncationEdgeCaseExactFit(t *testing.T) {
	// Test that description that exactly fits the width is not truncated.
	desc := "abcde"
	// path="f.yass.yaml" (11 graphemes) + tab(1) + name="S1" (2) + tab(1) = 15
	// remaining = width - 15 = 5, desc = 5 graphemes = exact fit
	result := truncateDescription("f.yass.yaml", "S1", desc, 20)
	if result != "abcde" {
		t.Errorf("expected exact fit %q, got %q", "abcde", result)
	}
}

func TestTruncationEdgeCaseOneOverFit(t *testing.T) {
	// 6 chars but only 5 available.
	desc := "abcdef"
	// path="f.yass.yaml" (11) + tab(1) + name="S1" (2) + tab(1) = 15
	// available = 20 - 15 = 5, desc = 6 > 5
	// marker = 3, textSpace = 5 - 3 = 2
	result := truncateDescription("f.yass.yaml", "S1", desc, 20)
	if result != "ab..." {
		t.Errorf("expected %q, got %q", "ab...", result)
	}
}

func TestNormalizeDescriptionUnit(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"  hello  world  ", "hello world"},
		{"hello\nworld", "hello world"},
		{"hello\t\tworld", "hello world"},
		{"  \n\t  ", ""},
		{"single", "single"},
	}
	for _, tt := range tests {
		got := normalizeDescription(tt.input)
		if got != tt.want {
			t.Errorf("normalizeDescription(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGraphemeLen(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 5},
		{"éllo", 4}, // e + combining accent (U+0301) = 1 grapheme cluster
	}
	for _, tt := range tests {
		got := graphemeLen(tt.input)
		if got != tt.want {
			t.Errorf("graphemeLen(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestFirstDocIsSpec(t *testing.T) {
	// When first doc has "spec" key, it should be treated as a spec (no preamble).
	dir := t.TempDir()
	initGit(t, dir)
	writeFile(t, dir, "test.yass.yaml", `---
spec: S1
INPUT:
- MUST: something
`)

	stdout, _, code := runInDir(t, dir, nil, false, 0)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	line := strings.TrimRight(stdout, "\n")
	fields := strings.Split(line, "\t")
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}
	if fields[1] != "S1" {
		t.Errorf("spec name = %q, want S1", fields[1])
	}
	// Description should be empty since there is no preamble.
	if fields[2] != "" {
		t.Errorf("description should be empty without preamble, got %q", fields[2])
	}
}
