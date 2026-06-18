package shared

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/text/unicode/norm"

	yerrors "github.com/shakefu/yass/internal/errors"
)

// DiscoverSpecFiles finds .yass.yaml spec files based on the input path.
//
// When inputPath is a file: return that single file path (after checking extension).
// When inputPath is a directory: recursively find files with literal suffix ".yass.yaml".
// When inputPath is "": search recursively from projectRoot.
//
// Returns:
//   - filePaths: discovered file paths, formatted relative to cwd when possible
//   - nonFatalErrors: formatted error lines for yass.discover.dir_unreadable
//   - fatalError: fatal error (yass.path.not_found, yass.path.bad_extension, etc.)
func DiscoverSpecFiles(inputPath, cwd, projectRoot string) ([]string, []string, error) {
	// Normalize cwd and projectRoot to absolute paths with forward slashes.
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving cwd: %w", err)
	}
	absCwd = filepath.Clean(absCwd)

	// Determine the effective path to use.
	effectivePath := inputPath
	if effectivePath == "" {
		effectivePath = projectRoot
	}

	// Make effective path absolute for stat.
	absPath := effectivePath
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(absCwd, absPath)
	}
	absPath = filepath.Clean(absPath)

	// Lstat to get info without following symlinks.
	info, err := os.Lstat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, &PathError{
				Code:    yerrors.CodePathNotFound,
				Message: fmt.Sprintf(yerrors.Messages[yerrors.CodePathNotFound], effectivePath),
				Path:    effectivePath,
			}
		}
		if os.IsPermission(err) {
			return nil, nil, &PathError{
				Code:    yerrors.CodePathUnreadable,
				Message: fmt.Sprintf(yerrors.Messages[yerrors.CodePathUnreadable], effectivePath),
				Path:    effectivePath,
			}
		}
		return nil, nil, fmt.Errorf("stat %s: %w", effectivePath, err)
	}

	// If it's a symlink, stat to determine what it points to.
	resolvedInfo := info
	isSymlink := info.Mode()&fs.ModeSymlink != 0
	if isSymlink {
		resolvedInfo, err = os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil, &PathError{
					Code:    yerrors.CodePathNotFound,
					Message: fmt.Sprintf(yerrors.Messages[yerrors.CodePathNotFound], effectivePath),
					Path:    effectivePath,
				}
			}
			return nil, nil, &PathError{
				Code:    yerrors.CodePathUnreadable,
				Message: fmt.Sprintf(yerrors.Messages[yerrors.CodePathUnreadable], effectivePath),
				Path:    effectivePath,
			}
		}
	}

	// Classify: file or directory.
	if resolvedInfo.Mode().IsRegular() {
		// File input: check extension.
		if !hasYassSuffix(filepath.Base(absPath)) {
			return nil, nil, &PathError{
				Code:    yerrors.CodePathBadExtension,
				Message: fmt.Sprintf(yerrors.Messages[yerrors.CodePathBadExtension], effectivePath),
				Path:    effectivePath,
			}
		}
		formatted := formatDiscoverPath(absPath, absCwd)
		return []string{formatted}, nil, nil
	}

	if resolvedInfo.IsDir() {
		// Directory input: recursive search.
		files, nonFatal := discoverInDir(absPath, absCwd)
		return files, nonFatal, nil
	}

	// Neither file nor directory.
	return nil, nil, &PathError{
		Code:    yerrors.CodePathInvalidType,
		Message: fmt.Sprintf(yerrors.Messages[yerrors.CodePathInvalidType], effectivePath),
		Path:    effectivePath,
	}
}

// discoverInDir recursively searches a directory for .yass.yaml files.
// It does not follow symlinks, does not descend into hidden directories,
// and does not match hidden files.
func discoverInDir(dirPath, cwd string) ([]string, []string) {
	var files []string
	var nonFatalErrors []string

	walkDir(dirPath, cwd, &files, &nonFatalErrors)

	// Sort by Unicode code-point order on NFC-normalized path.
	sort.Slice(files, func(i, j int) bool {
		ni := norm.NFC.String(files[i])
		nj := norm.NFC.String(files[j])
		return ni < nj
	})

	return files, nonFatalErrors
}

// walkDir performs recursive directory walking without following symlinks.
func walkDir(dirPath, cwd string, files *[]string, nonFatalErrors *[]string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		// Non-fatal: emit dir_unreadable error line.
		formatted := formatDiscoverPath(dirPath, cwd)
		errLine := yerrors.FormatError(
			formatted,
			0,
			yerrors.CodeDiscoverDirUnreadable,
			fmt.Sprintf(yerrors.Messages[yerrors.CodeDiscoverDirUnreadable], formatted),
		)
		*nonFatalErrors = append(*nonFatalErrors, errLine)
		return
	}

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden entries.
		if isHidden(name) {
			continue
		}

		entryPath := filepath.Join(dirPath, name)

		// Check for symlinks during recursion: treat as absent.
		if entry.Type()&fs.ModeSymlink != 0 {
			continue
		}

		if entry.IsDir() {
			walkDir(entryPath, cwd, files, nonFatalErrors)
			continue
		}

		if entry.Type().IsRegular() {
			if matchesYassSpec(name) {
				formatted := formatDiscoverPath(entryPath, cwd)
				*files = append(*files, formatted)
			}
		}
	}
}

// matchesYassSpec checks if a filename is a valid .yass.yaml spec file:
// - ends with literal suffix ".yass.yaml"
// - has a non-empty basename before the suffix
// - does not start with "."
func matchesYassSpec(name string) bool {
	if isHidden(name) {
		return false
	}
	return hasYassSuffix(name)
}

// formatDiscoverPath formats a discovered path per the spec using
// errors.FormatPath:
// - relative when under cwd (no leading ./)
// - basename when directly in cwd
// - absolute otherwise
// Does not resolve symlinks.
func formatDiscoverPath(absPath, absCwd string) string {
	return yerrors.FormatPath(absPath, absCwd)
}
