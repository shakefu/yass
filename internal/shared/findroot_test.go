package shared

import (
	"os"
	"path/filepath"
	"testing"

	yerrors "github.com/shakefu/yass/internal/errors"
)

func TestFindProjectRoot_GitInStartDir(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := FindProjectRoot(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmp {
		t.Errorf("expected %s, got %s", tmp, root)
	}
}

func TestFindProjectRoot_GitInParent(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	child := filepath.Join(tmp, "sub", "deep")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := FindProjectRoot(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmp {
		t.Errorf("expected %s, got %s", tmp, root)
	}
}

func TestFindProjectRoot_GitAsFile(t *testing.T) {
	// Git worktrees use a .git file instead of directory.
	tmp := t.TempDir()
	gitFile := filepath.Join(tmp, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: ../some/path"), 0o644); err != nil {
		t.Fatal(err)
	}

	root, err := FindProjectRoot(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmp {
		t.Errorf("expected %s, got %s", tmp, root)
	}
}

func TestFindProjectRoot_NoGitButYassYaml(t *testing.T) {
	tmp := t.TempDir()
	specFile := filepath.Join(tmp, "example.yass.yaml")
	if err := os.WriteFile(specFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	child := filepath.Join(tmp, "subdir")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := FindProjectRoot(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmp {
		t.Errorf("expected %s, got %s", tmp, root)
	}
}

func TestFindProjectRoot_YassYamlInStartDir(t *testing.T) {
	tmp := t.TempDir()
	specFile := filepath.Join(tmp, "my.yass.yaml")
	if err := os.WriteFile(specFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	root, err := FindProjectRoot(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmp {
		t.Errorf("expected %s, got %s", tmp, root)
	}
}

func TestFindProjectRoot_GitTakesPriorityOverYassYaml(t *testing.T) {
	// .git in parent, .yass.yaml in child: should return parent (where .git is).
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	child := filepath.Join(tmp, "sub")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	specFile := filepath.Join(child, "test.yass.yaml")
	if err := os.WriteFile(specFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	root, err := FindProjectRoot(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// .git exists in tmp, so that should be the root, not child.
	if root != tmp {
		t.Errorf("expected %s (git root), got %s", tmp, root)
	}
}

func TestFindProjectRoot_GitIgnoresYassYamlWhenGitExists(t *testing.T) {
	// .git in parent, .yass.yaml also in parent but deeper child has .yass.yaml too.
	// Since .git exists, .yass.yaml markers are ignored entirely.
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	child := filepath.Join(tmp, "a", "b")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	// Put .yass.yaml in the child dir (deeper than .git).
	specFile := filepath.Join(child, "test.yass.yaml")
	if err := os.WriteFile(specFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	root, err := FindProjectRoot(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// .git is in tmp, so root should be tmp, not child.
	if root != tmp {
		t.Errorf("expected %s (git root), got %s", tmp, root)
	}
}

func TestFindProjectRoot_NoMarkers(t *testing.T) {
	tmp := t.TempDir()
	child := filepath.Join(tmp, "empty")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}

	// This test might find .git somewhere up the real filesystem tree.
	// To avoid that, we use a deeply nested temp directory.
	// Actually, t.TempDir() is typically under /tmp which likely has no .git.
	// But on some systems the tmpdir could be under a git repo.
	// We'll just test that the function returns a result or our expected error.
	root, err := FindProjectRoot(child)
	if err != nil {
		pathErr, ok := err.(*PathError)
		if !ok {
			t.Fatalf("expected PathError, got %T: %v", err, err)
		}
		if pathErr.Code != yerrors.CodeFindRootNoMarker {
			t.Errorf("expected code %s, got %s", yerrors.CodeFindRootNoMarker, pathErr.Code)
		}
		return
	}
	// If we get here, there was a .git somewhere up the path (possible in CI).
	t.Logf("found root at %s (possibly a parent git repo)", root)
}

func TestFindProjectRoot_NestedDirs(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	deep := filepath.Join(tmp, "a", "b", "c", "d", "e")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := FindProjectRoot(deep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != tmp {
		t.Errorf("expected %s, got %s", tmp, root)
	}
}

func TestFindProjectRoot_DeepestGitWins(t *testing.T) {
	// .git in startDir itself should be returned even if there's
	// another .git higher up.
	tmp := t.TempDir()
	gitDir1 := filepath.Join(tmp, ".git")
	if err := os.Mkdir(gitDir1, 0o755); err != nil {
		t.Fatal(err)
	}

	child := filepath.Join(tmp, "sub")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	gitDir2 := filepath.Join(child, ".git")
	if err := os.Mkdir(gitDir2, 0o755); err != nil {
		t.Fatal(err)
	}

	root, err := FindProjectRoot(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != child {
		t.Errorf("expected %s (deepest .git), got %s", child, root)
	}
}

func TestFindProjectRoot_BareYassYamlDoesNotCount(t *testing.T) {
	// A file named exactly ".yass.yaml" should not match because it has no
	// non-empty basename prefix. Also it's hidden (starts with ".").
	tmp := t.TempDir()
	bareFile := filepath.Join(tmp, ".yass.yaml")
	if err := os.WriteFile(bareFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := FindProjectRoot(tmp)
	if err == nil {
		// Might have found a .git higher up. That's fine.
		return
	}
	pathErr, ok := err.(*PathError)
	if !ok {
		t.Fatalf("expected PathError, got %T: %v", err, err)
	}
	if pathErr.Code != yerrors.CodeFindRootNoMarker {
		t.Errorf("expected code %s, got %s", yerrors.CodeFindRootNoMarker, pathErr.Code)
	}
}
