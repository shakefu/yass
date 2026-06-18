package shared

import (
	"os"
	"path/filepath"
	"testing"

	yerrors "github.com/shakefu/yass/internal/errors"
)

func TestExpandGlob_LiteralPath(t *testing.T) {
	result, err := ExpandGlob("some/literal/path.yass.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0] != "some/literal/path.yass.yaml" {
		t.Errorf("expected [some/literal/path.yass.yaml], got %v", result)
	}
}

func TestExpandGlob_StarPattern(t *testing.T) {
	tmp := t.TempDir()
	// Create test files.
	for _, name := range []string{"a.yass.yaml", "b.yass.yaml", "c.txt"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Change to tmp dir for relative glob.
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	result, err := ExpandGlob("*.yass.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 matches, got %d: %v", len(result), result)
	}
	if result[0] != "a.yass.yaml" || result[1] != "b.yass.yaml" {
		t.Errorf("unexpected results: %v", result)
	}
}

func TestExpandGlob_DoubleStarPattern(t *testing.T) {
	tmp := t.TempDir()
	// Create nested structure.
	subDir := filepath.Join(tmp, "sub", "deep")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(tmp, "root.yass.yaml"),
		filepath.Join(tmp, "sub", "mid.yass.yaml"),
		filepath.Join(tmp, "sub", "deep", "deep.yass.yaml"),
	} {
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	result, err := ExpandGlob("**/*.yass.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ** should match all levels. The root-level file is also matched by **/*.
	if len(result) < 2 {
		t.Fatalf("expected at least 2 matches, got %d: %v", len(result), result)
	}
}

func TestExpandGlob_NoMatches(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	_, err := ExpandGlob("*.nonexistent")
	if err == nil {
		t.Fatal("expected error for no matches")
	}
	pathErr, ok := err.(*PathError)
	if !ok {
		t.Fatalf("expected PathError, got %T: %v", err, err)
	}
	if pathErr.Code != yerrors.CodeGlobNoMatch {
		t.Errorf("expected code %s, got %s", yerrors.CodeGlobNoMatch, pathErr.Code)
	}
}

func TestExpandGlob_HiddenFilesExcluded(t *testing.T) {
	tmp := t.TempDir()
	// Create visible and hidden files.
	for _, name := range []string{"visible.yass.yaml", ".hidden.yass.yaml"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	result, err := ExpandGlob("*.yass.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0] != "visible.yass.yaml" {
		t.Errorf("expected [visible.yass.yaml], got %v", result)
	}
}

func TestExpandGlob_HiddenDirsExcluded(t *testing.T) {
	tmp := t.TempDir()
	// Create hidden directory with a spec file inside.
	hiddenDir := filepath.Join(tmp, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "spec.yass.yaml"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create visible file.
	if err := os.WriteFile(filepath.Join(tmp, "visible.yass.yaml"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	result, err := ExpandGlob("**/*.yass.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range result {
		if containsHiddenSegment(r) {
			t.Errorf("hidden path should be excluded: %s", r)
		}
	}
}

func TestExpandGlob_SortedResults(t *testing.T) {
	tmp := t.TempDir()
	// Create files in non-alphabetical order.
	for _, name := range []string{"c.yass.yaml", "a.yass.yaml", "b.yass.yaml"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	result, err := ExpandGlob("*.yass.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}
	if result[0] != "a.yass.yaml" || result[1] != "b.yass.yaml" || result[2] != "c.yass.yaml" {
		t.Errorf("results not sorted: %v", result)
	}
}

func TestExpandGlob_BracePatternsNotExpanded(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"a.yass.yaml", "b.yass.yaml"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Brace pattern should be treated as literal (no brace expansion).
	// "{a,b}.yass.yaml" contains no glob metacharacters (* ? [) so it
	// should be returned as a literal path.
	result, err := ExpandGlob("{a,b}.yass.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0] != "{a,b}.yass.yaml" {
		t.Errorf("brace pattern should not be expanded, got %v", result)
	}
}

func TestExpandGlob_QuestionMarkPattern(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"a.yass.yaml", "b.yass.yaml", "cc.yass.yaml"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// ? matches exactly one character, so "cc.yass.yaml" should not match.
	result, err := ExpandGlob("?.yass.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 matches, got %d: %v", len(result), result)
	}
	if result[0] != "a.yass.yaml" || result[1] != "b.yass.yaml" {
		t.Errorf("unexpected results: %v", result)
	}
}

func TestExpandGlob_BracketExpression(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"a.yass.yaml", "b.yass.yaml", "c.yass.yaml"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// [ab] should match a and b but not c.
	result, err := ExpandGlob("[ab].yass.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 matches, got %d: %v", len(result), result)
	}
	if result[0] != "a.yass.yaml" || result[1] != "b.yass.yaml" {
		t.Errorf("unexpected results: %v", result)
	}
}

func TestExpandGlob_SymlinksNotFollowed(t *testing.T) {
	tmp := t.TempDir()

	// Create a real directory with spec files.
	realDir := filepath.Join(tmp, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "spec.yass.yaml"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink directory.
	symlinkDir := filepath.Join(tmp, "linked")
	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Skip("symlinks not supported")
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// ** should not follow the symlink directory.
	result, err := ExpandGlob("**/*.yass.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should only find real/spec.yass.yaml, not linked/spec.yass.yaml.
	for _, r := range result {
		if filepath.Base(filepath.Dir(r)) == "linked" {
			t.Errorf("symlink directory should not be followed, found: %s", r)
		}
	}
}
