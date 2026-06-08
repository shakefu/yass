package errors

import (
	"fmt"
	"path/filepath"
	"strings"
)

// FormatPath formats a file path per the cli.ErrorLine spec.
//
// When filePath is under cwd: emit relative, no leading "./".
// When directly inside cwd: emit just the basename.
// When NOT under cwd: emit absolute path.
// Always use forward slashes.
// Do NOT resolve symlinks.
// When filePath is "": use literal "yass".
func FormatPath(filePath, cwd string) string {
	if filePath == "" {
		return "yass"
	}

	// Clean paths for comparison (but do NOT resolve symlinks).
	absFile := filepath.Clean(filePath)
	absCwd := filepath.Clean(cwd)

	// Make sure both are absolute for comparison.
	if !filepath.IsAbs(absFile) {
		absFile = filepath.Join(absCwd, absFile)
	}

	// Normalize to forward slashes for comparison and output.
	absFileSlash := filepath.ToSlash(absFile)
	absCwdSlash := filepath.ToSlash(absCwd)

	// Ensure cwd ends with "/" for prefix matching.
	cwdPrefix := absCwdSlash
	if !strings.HasSuffix(cwdPrefix, "/") {
		cwdPrefix += "/"
	}

	// Check if file is under cwd.
	if strings.HasPrefix(absFileSlash, cwdPrefix) {
		rel := absFileSlash[len(cwdPrefix):]
		// If directly inside cwd (no further "/" in the relative part), return basename.
		if !strings.Contains(rel, "/") {
			return rel
		}
		// Otherwise return relative path without leading "./".
		return rel
	}

	// Not under cwd: return absolute path with forward slashes.
	return absFileSlash
}

// FormatError formats one error line per the cli.ErrorLine spec.
//
// With line > 0: "<file>:<line>: [<code>] <message>"
// With line == 0: "<file>: [<code>] <message>"
// Replace any newline in message with a single space.
// Terminated by LF.
func FormatError(file string, line int, code, message string) string {
	// Replace newlines in message with a single space.
	msg := strings.ReplaceAll(message, "\n", " ")
	msg = strings.ReplaceAll(msg, "\r", " ")

	if line > 0 {
		return fmt.Sprintf("%s:%d: [%s] %s\n", file, line, code, msg)
	}
	return fmt.Sprintf("%s: [%s] %s\n", file, code, msg)
}
