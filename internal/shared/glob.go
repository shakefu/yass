package shared

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"golang.org/x/text/unicode/norm"

	yerrors "github.com/shakefu/yass/internal/errors"
)

// globMetaChars are the characters that indicate a pattern contains glob metacharacters.
const globMetaChars = "*?["

// ExpandGlob expands a single command-line argument string using doublestar
// semantics. When the argument contains no glob metacharacters, it returns the
// literal path unchanged. Otherwise it expands the pattern and returns results
// sorted by Unicode code-point order on NFC-normalized paths.
//
// Hidden files/directories (starting with ".") are excluded from matches.
// Symlinks are not followed. Brace, tilde, and env expansion are not performed.
//
// Returns a PathError with code yass.glob.no_match when zero files match.
func ExpandGlob(pattern string) ([]string, error) {
	// If no glob metacharacters, return literal path unchanged.
	if !containsGlobMeta(pattern) {
		return []string{pattern}, nil
	}

	// Use FilepathGlob which handles absolute/relative patterns properly.
	// It splits the pattern into a base path and glob part internally.
	matches, err := doublestar.FilepathGlob(pattern, doublestar.WithNoFollow())
	if err != nil {
		return nil, fmt.Errorf("glob error: %w", err)
	}

	// Filter out hidden files and directories.
	var filtered []string
	for _, m := range matches {
		if containsHiddenSegment(m) {
			continue
		}
		filtered = append(filtered, m)
	}

	if len(filtered) == 0 {
		return nil, &PathError{
			Code:    yerrors.CodeGlobNoMatch,
			Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeGlobNoMatch], pattern),
			Path:    pattern,
		}
	}

	// Sort by Unicode code-point order on NFC-normalized path.
	sort.Slice(filtered, func(i, j int) bool {
		ni := norm.NFC.String(filtered[i])
		nj := norm.NFC.String(filtered[j])
		return ni < nj
	})

	return filtered, nil
}

// containsGlobMeta returns true if the pattern contains glob metacharacters.
func containsGlobMeta(pattern string) bool {
	return strings.ContainsAny(pattern, globMetaChars)
}

// containsHiddenSegment returns true if any segment of the path starts with ".".
func containsHiddenSegment(path string) bool {
	// Use forward slashes for consistent splitting.
	p := filepath.ToSlash(path)
	parts := strings.Split(p, "/")
	for _, part := range parts {
		if len(part) > 0 && part[0] == '.' {
			return true
		}
	}
	return false
}
