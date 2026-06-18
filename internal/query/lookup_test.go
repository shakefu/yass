package query

import (
	"os"
	"path/filepath"
	"testing"
)

// Helper to create a temp dir with spec files for testing.
func createTestSpecFile(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
	return path
}

func TestNameLookup_ExactMatch(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test file
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)
	matches, err := NameLookup("MySpec", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].SpecName != "MySpec" {
		t.Errorf("expected SpecName=MySpec, got %s", matches[0].SpecName)
	}
	if matches[0].File != f {
		t.Errorf("expected File=%s, got %s", f, matches[0].File)
	}
}

func TestNameLookup_SuffixMatch(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test file
version: v1
---
spec: pkg.Foo
RETURN:
- MUST: do something
`)
	matches, err := NameLookup("Foo", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].SpecName != "pkg.Foo" {
		t.Errorf("expected SpecName=pkg.Foo, got %s", matches[0].SpecName)
	}
}

func TestNameLookup_MultiSegmentSuffix(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test file
version: v1
---
spec: a.b.Foo
RETURN:
- MUST: do something
`)

	// "Foo" matches a.b.Foo
	matches, err := NameLookup("Foo", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match for 'Foo', got %d", len(matches))
	}

	// "b.Foo" matches a.b.Foo
	matches, err = NameLookup("b.Foo", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match for 'b.Foo', got %d", len(matches))
	}
}

func TestNameLookup_PartialSubstringRejected(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test file
version: v1
---
spec: pkg.FooBar
RETURN:
- MUST: do something
`)

	// "Foo" should NOT match "pkg.FooBar" (not a full trailing component)
	matches, err := NameLookup("Foo", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for partial 'Foo' against 'pkg.FooBar', got %d", len(matches))
	}

	// "Bar" should NOT match "pkg.FooBar"
	matches, err = NameLookup("Bar", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for partial 'Bar' against 'pkg.FooBar', got %d", len(matches))
	}

	// "ooBar" should NOT match "pkg.FooBar"
	matches, err = NameLookup("ooBar", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for partial 'ooBar' against 'pkg.FooBar', got %d", len(matches))
	}
}

func TestNameLookup_NonDotAlignedSuffixRejected(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test file
version: v1
---
spec: a.b.Foo
RETURN:
- MUST: do something
`)

	// "xb.Foo" should NOT match "a.b.Foo"
	matches, err := NameLookup("xb.Foo", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for 'xb.Foo' against 'a.b.Foo', got %d", len(matches))
	}
}

func TestNameLookup_CaseSensitive(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test file
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	// "myspec" should NOT match "MySpec"
	matches, err := NameLookup("myspec", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for case-insensitive 'myspec', got %d", len(matches))
	}

	// "MYSPEC" should NOT match
	matches, err = NameLookup("MYSPEC", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for case-insensitive 'MYSPEC', got %d", len(matches))
	}
}

func TestNameLookup_EmptyNameError(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test file
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	_, err := NameLookup("", []string{f})
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
	if got := err.Error(); got == "" {
		t.Error("expected non-empty error message")
	}
}

func TestNameLookup_WhitespaceNoMatch(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test file
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	// Whitespace in name -> no-match, not error
	matches, err := NameLookup("My Spec", []string{f})
	if err != nil {
		t.Fatalf("expected no error for whitespace name, got: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for whitespace name, got %d", len(matches))
	}

	// Tab in name
	matches, err = NameLookup("My\tSpec", []string{f})
	if err != nil {
		t.Fatalf("expected no error for tab name, got: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for tab name, got %d", len(matches))
	}
}

func TestNameLookup_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	f1 := createTestSpecFile(t, dir, "a.yass.yaml", `---
description: file a
version: v1
---
spec: Foo
RETURN:
- MUST: do a
`)
	f2 := createTestSpecFile(t, dir, "b.yass.yaml", `---
description: file b
version: v1
---
spec: pkg.Foo
RETURN:
- MUST: do b
`)

	matches, err := NameLookup("Foo", []string{f1, f2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
}

func TestNameLookup_MultipleSpecsInOneFile(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test file
version: v1
---
spec: Alpha
RETURN:
- MUST: do a
---
spec: Beta
RETURN:
- MUST: do b
`)

	matches, err := NameLookup("Alpha", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].SpecName != "Alpha" {
		t.Errorf("expected SpecName=Alpha, got %s", matches[0].SpecName)
	}
}

func TestNameLookup_NoMatchReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test file
version: v1
---
spec: Alpha
RETURN:
- MUST: do a
`)

	matches, err := NameLookup("Nonexistent", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestNameLookup_DescriptionExtracted(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: This is a test spec file.
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	matches, err := NameLookup("MySpec", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Description != "This is a test spec file." {
		t.Errorf("expected description 'This is a test spec file.', got %q", matches[0].Description)
	}
}

func TestNameLookup_DescriptionNormalized(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: >
  This is a
  multi-line description.
version: v1
---
spec: MySpec
RETURN:
- MUST: do something
`)

	matches, err := NameLookup("MySpec", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Description != "This is a multi-line description." {
		t.Errorf("expected normalized description, got %q", matches[0].Description)
	}
}

func TestNameLookup_UnparseableFileSkipped(t *testing.T) {
	dir := t.TempDir()
	badFile := createTestSpecFile(t, dir, "bad.yass.yaml", `{{{not valid yaml`)
	goodFile := createTestSpecFile(t, dir, "good.yass.yaml", `---
description: good file
version: v1
---
spec: Found
RETURN:
- MUST: exist
`)

	matches, err := NameLookup("Found", []string{badFile, goodFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
}

func TestNameLookup_DocIndexCorrect(t *testing.T) {
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

	matches, err := NameLookup("Second", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].DocIndex != 2 {
		t.Errorf("expected DocIndex=2 (preamble=0, First=1, Second=2), got %d", matches[0].DocIndex)
	}
}

func TestNameLookup_SuffixMatchNotPrefix(t *testing.T) {
	dir := t.TempDir()
	f := createTestSpecFile(t, dir, "test.yass.yaml", `---
description: test
version: v1
---
spec: Foo.Bar
RETURN:
- MUST: one
`)

	// "Foo" is a prefix, not a trailing suffix - should NOT match
	matches, err := NameLookup("Foo", []string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for prefix 'Foo' against 'Foo.Bar', got %d", len(matches))
	}
}

func TestMatchesName(t *testing.T) {
	tests := []struct {
		specName  string
		queryName string
		want      bool
	}{
		{"Foo", "Foo", true},
		{"pkg.Foo", "Foo", true},
		{"pkg.Foo", "pkg.Foo", true},
		{"a.b.c.Foo", "Foo", true},
		{"a.b.c.Foo", "c.Foo", true},
		{"a.b.c.Foo", "b.c.Foo", true},
		{"a.b.c.Foo", "a.b.c.Foo", true},
		{"pkg.FooBar", "Foo", false},
		{"pkg.FooBar", "Bar", false},
		{"Foo.Bar", "Foo", false},
		{"Foo", "foo", false},
		{"Foo", "FOO", false},
		{"a.b.Foo", "xb.Foo", false},
		{"Foo", "oo", false},
	}

	for _, tt := range tests {
		t.Run(tt.specName+"_"+tt.queryName, func(t *testing.T) {
			got := matchesName(tt.specName, tt.queryName)
			if got != tt.want {
				t.Errorf("matchesName(%q, %q) = %v, want %v", tt.specName, tt.queryName, got, tt.want)
			}
		})
	}
}
