package query

import (
	"strings"
	"testing"
)

func TestExtractFragment_BasicSpec(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test file
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	fragment, err := ExtractFragment(f, "MySpec")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must start with "---\n"
	if !strings.HasPrefix(fragment, "---\n") {
		t.Errorf("fragment should start with '---\\n', got %q", fragment[:min(20, len(fragment))])
	}

	// Must not have trailing "..."
	if strings.Contains(fragment, "...") {
		t.Error("fragment should not contain '...' marker")
	}

	// Must end with exactly one LF
	if !strings.HasSuffix(fragment, "\n") {
		t.Error("fragment should end with LF")
	}
	if strings.HasSuffix(fragment, "\n\n") {
		t.Error("fragment should not end with double LF")
	}

	// Must contain spec key
	if !strings.Contains(fragment, "spec: MySpec") {
		t.Error("fragment should contain 'spec: MySpec'")
	}
}

func TestExtractFragment_PreservesKeyOrder(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
INPUT:
- MUST: accept input
RETURN:
- MUST: return output
ERROR:
- MUST: report errors
`)

	fragment, err := ExtractFragment(f, "MySpec")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	inputIdx := strings.Index(fragment, "INPUT:")
	returnIdx := strings.Index(fragment, "RETURN:")
	errorIdx := strings.Index(fragment, "ERROR:")

	if inputIdx < 0 || returnIdx < 0 || errorIdx < 0 {
		t.Fatalf("fragment missing keys: INPUT=%d, RETURN=%d, ERROR=%d", inputIdx, returnIdx, errorIdx)
	}
	if inputIdx >= returnIdx || returnIdx >= errorIdx {
		t.Error("key ordering should be preserved: INPUT < RETURN < ERROR")
	}
}

func TestExtractFragment_NoPreamble(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test file with description
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	fragment, err := ExtractFragment(f, "MySpec")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must NOT include preamble content
	if strings.Contains(fragment, "description:") {
		t.Error("fragment should not include preamble description")
	}
	if strings.Contains(fragment, "version:") {
		t.Error("fragment should not include preamble version")
	}
}

func TestExtractFragment_SpecNotFound(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	_, err := ExtractFragment(f, "NonExistent")
	if err == nil {
		t.Fatal("expected error for non-existent spec")
	}
}

func TestExtractFragment_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `{{{not valid yaml`)

	_, err := ExtractFragment(f, "MySpec")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestExtractFragment_MultipleSpecs(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: First
RETURN:
- MUST: one
---
spec: Second
RETURN:
- MUST: two
`)

	fragment, err := ExtractFragment(f, "Second")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(fragment, "spec: Second") {
		t.Error("fragment should contain 'spec: Second'")
	}
	if strings.Contains(fragment, "spec: First") {
		t.Error("fragment should not contain 'spec: First'")
	}
}

func TestFormatScalar_Unquoted(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"do something", "do something"},
		{"spec name", "spec name"},
		{"MUST", "MUST"},
		{"INPUT", "INPUT"},
	}

	for _, tt := range tests {
		got := formatScalar(tt.input)
		if got != tt.want {
			t.Errorf("formatScalar(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatScalar_Quoted(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"true", `"true"`},
		{"false", `"false"`},
		{"null", `"null"`},
		{"yes", `"yes"`},
		{"no", `"no"`},
		{"on", `"on"`},
		{"off", `"off"`},
		{"True", `"True"`},
		{"FALSE", `"FALSE"`},
		{"contains: colon space", `"contains: colon space"`},
		{"*starts with star", `"*starts with star"`},
		{"&starts with amp", `"&starts with amp"`},
		{"!starts with bang", `"!starts with bang"`},
		{"|starts with pipe", `"|starts with pipe"`},
		{">starts with gt", `">starts with gt"`},
		{"%starts with pct", `"%starts with pct"`},
		{"@starts with at", `"@starts with at"`},
		{"?starts with q", `"?starts with q"`},
		{"-starts with dash", `"-starts with dash"`},
		{" leading space", `" leading space"`},
		{"trailing space ", `"trailing space "`},
		{"42", `"42"`},
		{"3.14", `"3.14"`},
		{"0x1F", `"0x1F"`},
		{".inf", `".inf"`},
		{".nan", `".nan"`},
		{"", `""`},
	}

	for _, tt := range tests {
		got := formatScalar(tt.input)
		if got != tt.want {
			t.Errorf("formatScalar(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNeedsQuoting_NoQuote(t *testing.T) {
	cases := []string{
		"hello",
		"do something",
		"MUST",
		"accept input",
		"a_b-c",
		"spec.Name",
	}

	for _, s := range cases {
		if needsQuoting(s) {
			t.Errorf("needsQuoting(%q) = true, want false", s)
		}
	}
}

func TestNeedsQuoting_YesQuote(t *testing.T) {
	cases := []string{
		"true",
		"True",
		"TRUE",
		"false",
		"null",
		"yes",
		"no",
		"on",
		"off",
		"42",
		"3.14",
		"0x1F",
		".inf",
		".nan",
		"contains: colon space",
		"*special",
		"-dash",
		" leading",
		"trailing ",
		"",
	}

	for _, s := range cases {
		if !needsQuoting(s) {
			t.Errorf("needsQuoting(%q) = false, want true", s)
		}
	}
}

func TestDoubleQuote_Escaping(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`hello`, `"hello"`},
		{`he"llo`, `"he\"llo"`},
		{"he\nllo", `"he\nllo"`},
		{"he\\llo", `"he\\llo"`},
		{"he\tllo", `"he\tllo"`},
		{"he\rllo", `"he\rllo"`},
	}

	for _, tt := range tests {
		got := doubleQuote(tt.input)
		if got != tt.want {
			t.Errorf("doubleQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractFragment_ObligationKeyOrder(t *testing.T) {
	dir := t.TempDir()
	// Put WHEN before MUST in source to verify reordering
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN:
- WHEN: condition is met
  MUST: do something
  USES: some.Ref
`)

	fragment, err := ExtractFragment(f, "MySpec")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// MUST should come before WHEN in the output
	mustIdx := strings.Index(fragment, "MUST:")
	whenIdx := strings.Index(fragment, "WHEN:")
	usesIdx := strings.Index(fragment, "USES:")

	if mustIdx < 0 || whenIdx < 0 || usesIdx < 0 {
		t.Fatalf("fragment missing keys: MUST=%d, WHEN=%d, USES=%d\nfragment:\n%s", mustIdx, whenIdx, usesIdx, fragment)
	}

	if mustIdx >= whenIdx {
		t.Errorf("MUST (%d) should come before WHEN (%d) in output", mustIdx, whenIdx)
	}
	if whenIdx >= usesIdx {
		t.Errorf("WHEN (%d) should come before USES (%d) in output", whenIdx, usesIdx)
	}
}

func TestExtractFragment_TwoSpaceIndent(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	fragment, err := ExtractFragment(f, "MySpec")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(fragment, "\n")
	for _, line := range lines {
		if line == "" || line == "---" {
			continue
		}
		stripped := strings.TrimLeft(line, " ")
		indent := len(line) - len(stripped)
		if indent > 0 && indent%2 != 0 {
			t.Errorf("non-2-space indent on line: %q (indent=%d)", line, indent)
		}
	}
}

func TestExtractFragment_QuotedColonSpace(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN:
- MUST: "emit as: formatted"
`)

	fragment, err := ExtractFragment(f, "MySpec")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The value contains ": " so must be double-quoted in output
	if !strings.Contains(fragment, `"emit as: formatted"`) {
		t.Errorf("expected quoted scalar for colon-space value, got:\n%s", fragment)
	}
}

func TestExtractFragment_ListItems(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN:
- MUST: first thing
- MUST: second thing
- MUST: third thing
`)

	fragment, err := ExtractFragment(f, "MySpec")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count list items
	count := strings.Count(fragment, "- MUST:")
	if count != 3 {
		t.Errorf("expected 3 list items with '- MUST:', got %d in:\n%s", count, fragment)
	}
}

func TestExtractFragment_EmptySlot(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: MySpec
RETURN: []
`)

	fragment, err := ExtractFragment(f, "MySpec")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(fragment, "spec: MySpec") {
		t.Error("fragment should contain spec name")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
