package validate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	yerrors "github.com/shakefu/yass/internal/errors"
	"github.com/shakefu/yass/internal/exitcode"
	"github.com/shakefu/yass/internal/shared"
)

// Run executes the validate subcommand.
// args = positional arguments after "validate".
// Returns an exit code: 0 success, 1 validation errors, 2 usage/path errors.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", yerrors.FormatError("yass", 0, yerrors.CodeInternalUncaught, fmt.Sprintf("internal error: %s", err)))
		fmt.Fprintf(stdout, "checked 0 files, found 0 errors\n")
		return exitcode.Usage
	}

	// Check for colon in path arguments.
	// ErrorLine spec: MUST-NOT emit any input path containing ":".
	for _, arg := range args {
		if strings.Contains(arg, ":") {
			line := yerrors.FormatError(
				"yass",
				0,
				yerrors.CodePathColonInPath,
				fmt.Sprintf(yerrors.Messages[yerrors.CodePathColonInPath], arg),
			)
			fmt.Fprint(stderr, line)
			fmt.Fprintf(stdout, "checked 0 files, found 0 errors\n")
			return exitcode.Usage
		}
	}

	// Compute project root once.
	projectRoot, err := shared.FindProjectRoot(cwd)
	if err != nil {
		line := yerrors.FormatError(
			yerrors.FormatPath(cwd, cwd),
			0,
			yerrors.CodeFindRootNoMarker,
			yerrors.Messages[yerrors.CodeFindRootNoMarker],
		)
		fmt.Fprint(stderr, line)
		fmt.Fprintf(stdout, "checked 0 files, found 0 errors\n")
		return exitcode.Usage
	}

	// Expand arguments into file paths.
	files, exitCode := expandArgs(args, cwd, projectRoot, stderr)
	if exitCode >= 0 {
		fmt.Fprintf(stdout, "checked 0 files, found 0 errors\n")
		return exitCode
	}

	// Deduplicate by absolute path, preserving order.
	files = deduplicateFiles(files, cwd)

	// Validate each file.
	totalErrors := 0

	for _, file := range files {
		fileErrors := validateFile(file, cwd, projectRoot, stderr)
		totalErrors += fileErrors
	}

	fmt.Fprintf(stdout, "checked %d files, found %d errors\n", len(files), totalErrors)

	if totalErrors > 0 {
		return exitcode.Processing
	}
	return exitcode.Success
}

// expandArgs expands positional arguments into a list of file paths.
// Returns (files, -1) on success, or (nil, exitCode) on fatal error.
func expandArgs(args []string, cwd string, projectRoot string, stderr io.Writer) ([]string, int) {
	if len(args) == 0 {
		// Discover from project root.
		filePaths, nonFatalErrs, err := shared.DiscoverSpecFiles("", cwd, projectRoot)
		if err != nil {
			emitPathError(err, cwd, stderr)
			return nil, exitcode.Usage
		}
		for _, e := range nonFatalErrs {
			fmt.Fprint(stderr, e)
		}
		if len(filePaths) == 0 {
			line := yerrors.FormatError(
				"yass",
				0,
				yerrors.CodeDiscoverNoFiles,
				yerrors.Messages[yerrors.CodeDiscoverNoFiles],
			)
			fmt.Fprint(stderr, line)
			return nil, exitcode.Usage
		}
		return filePaths, -1
	}

	var allFiles []string

	for _, arg := range args {
		// Expand globs first.
		expanded, err := shared.ExpandGlob(arg)
		if err != nil {
			emitPathError(err, cwd, stderr)
			return nil, exitcode.Usage
		}

		for _, path := range expanded {
			// Check if glob-expanded path is not .yass.yaml — skip silently.
			if isGlobExpanded(arg) && !hasYassExtension(path) {
				continue
			}

			// Discover files.
			filePaths, nonFatalErrs, err := shared.DiscoverSpecFiles(path, cwd, projectRoot)
			if err != nil {
				emitPathError(err, cwd, stderr)
				return nil, exitcode.Usage
			}
			for _, e := range nonFatalErrs {
				fmt.Fprint(stderr, e)
			}
			allFiles = append(allFiles, filePaths...)
		}
	}

	if len(allFiles) == 0 {
		line := yerrors.FormatError(
			"yass",
			0,
			yerrors.CodeDiscoverNoFiles,
			yerrors.Messages[yerrors.CodeDiscoverNoFiles],
		)
		fmt.Fprint(stderr, line)
		return nil, exitcode.Usage
	}

	return allFiles, -1
}

// deduplicateFiles removes duplicate file paths by absolute path.
func deduplicateFiles(files []string, cwd string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, f := range files {
		abs := f
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(cwd, abs)
		}
		abs = filepath.Clean(abs)

		if !seen[abs] {
			seen[abs] = true
			result = append(result, f)
		}
	}

	return result
}

// validateFile runs all validation checks on a single file.
// Returns the number of errors found.
func validateFile(file string, cwd string, projectRoot string, stderr io.Writer) int {
	formattedPath := yerrors.FormatPath(file, cwd)

	// Step 1: CheckYAML
	docs, yamlErrs := CheckYAML(file)
	if len(yamlErrs) > 0 {
		// Emit exactly one error for CheckYAML failure.
		e := yamlErrs[0]
		line := yerrors.FormatError(formattedPath, e.Line, e.Code, e.Message)
		fmt.Fprint(stderr, line)
		return 1
	}

	var allErrors []ValidationError

	// Step 2: CheckPreamble
	preambleErrs := CheckPreamble(docs, file)
	allErrors = append(allErrors, preambleErrs...)

	// Step 3: CheckSpec (for each spec document)
	for i, doc := range docs {
		if i == 0 && !docHasSpecKey(doc) {
			// First doc is preamble — skip CheckSpec for it.
			continue
		}
		specErrs := CheckSpec(doc, file)
		allErrors = append(allErrors, specErrs...)
	}

	// Step 4: CheckUniqueness
	uniqueErrs := CheckUniqueness(docs, file)
	allErrors = append(allErrors, uniqueErrs...)

	// Step 5: CheckRefs
	basePath := filepath.Dir(file)
	if !filepath.IsAbs(basePath) {
		basePath = filepath.Join(cwd, basePath)
	}
	refsErrs := CheckRefs(docs, file, basePath, projectRoot)
	allErrors = append(allErrors, refsErrs...)

	// Sort errors by line ascending, then by code as tiebreaker.
	sort.Slice(allErrors, func(i, j int) bool {
		if allErrors[i].Line != allErrors[j].Line {
			return allErrors[i].Line < allErrors[j].Line
		}
		return allErrors[i].Code < allErrors[j].Code
	})

	// Emit errors.
	for _, e := range allErrors {
		line := yerrors.FormatError(formattedPath, e.Line, e.Code, e.Message)
		fmt.Fprint(stderr, line)
	}

	return len(allErrors)
}

// emitPathError emits a path error to stderr.
func emitPathError(err error, cwd string, stderr io.Writer) {
	if pe, ok := err.(*shared.PathError); ok {
		formattedPath := yerrors.FormatPath(pe.Path, cwd)
		line := yerrors.FormatError(formattedPath, 0, pe.Code, pe.Message)
		fmt.Fprint(stderr, line)
	} else {
		line := yerrors.FormatError("yass", 0, yerrors.CodeInternalUncaught, fmt.Sprintf("internal error: %s", err))
		fmt.Fprint(stderr, line)
	}
}

// isGlobExpanded returns true if the argument contains glob metacharacters.
func isGlobExpanded(arg string) bool {
	return strings.ContainsAny(arg, "*?[")
}

// hasYassExtension checks if a path has the .yass.yaml extension.
func hasYassExtension(path string) bool {
	return strings.HasSuffix(path, ".yass.yaml")
}
