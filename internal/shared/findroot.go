package shared

import (
	"fmt"
	"os"
	"path/filepath"

	yerrors "github.com/shakefu/yass/internal/errors"
)

// FindProjectRoot searches upward from startDir toward the filesystem root for
// the project root. It performs two passes:
//
//  1. Look for a .git entry (file or directory) in each ancestor starting from
//     startDir inclusive. Return the deepest ancestor containing .git.
//  2. If no .git found anywhere, restart from startDir and look for any
//     .yass.yaml file. Return the deepest ancestor containing one.
//
// Returns a PathError with code yass.findroot.no_marker when no marker is found.
func FindProjectRoot(startDir string) (string, error) {
	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolving absolute path: %w", err)
	}

	// Pass 1: search for .git
	dir := absStart
	for {
		candidate := filepath.Join(dir, ".git")
		if _, err := os.Lstat(candidate); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}

	// Pass 2: no .git found anywhere, search for .yass.yaml files
	dir = absStart
	for {
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, e := range entries {
				name := e.Name()
				if hasYassSuffix(name) && !isHidden(name) {
					return dir, nil
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}

	return "", &PathError{
		Code:    yerrors.CodeFindRootNoMarker,
		Message: yerrors.Messages[yerrors.CodeFindRootNoMarker],
	}
}

// hasYassSuffix checks if a filename ends with ".yass.yaml" and has a non-empty
// basename prefix before the suffix.
func hasYassSuffix(name string) bool {
	const suffix = ".yass.yaml"
	if len(name) <= len(suffix) {
		return false
	}
	return name[len(name)-len(suffix):] == suffix
}

// isHidden checks if a filename starts with ".".
func isHidden(name string) bool {
	return len(name) > 0 && name[0] == '.'
}
