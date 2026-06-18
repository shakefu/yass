package query

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	yerrors "github.com/shakefu/yass/internal/errors"
	"github.com/shakefu/yass/internal/exitcode"
	"github.com/shakefu/yass/internal/parser"
	"github.com/shakefu/yass/internal/shared"
)

// Run executes the query subcommand.
// args[0] = spec name (required), args[1] = optional scope path.
// Returns an exit code: 0 success, 1 processing errors, 2 usage errors.
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprint(stderr, yerrors.FormatError("yass", 0,
			yerrors.CodeInternalUncaught,
			fmt.Sprintf("internal error: %s", err)))
		return exitcode.Processing
	}

	// No spec name -> error.
	if len(args) == 0 {
		fmt.Fprint(stderr, yerrors.FormatError("yass", 0,
			yerrors.CodeQueryNameMissing,
			yerrors.Messages[yerrors.CodeQueryNameMissing]))
		return exitcode.Usage
	}

	specName := args[0]
	scopePath := ""
	if len(args) > 1 {
		scopePath = args[1]
	}

	// Check colon in scope path BEFORE anything else.
	// ErrorLine spec: MUST-NOT emit any input path containing ":".
	if scopePath != "" && strings.Contains(scopePath, ":") {
		fmt.Fprint(stderr, yerrors.FormatError(
			"yass", 0,
			yerrors.CodePathColonInPath,
			fmt.Sprintf(yerrors.Messages[yerrors.CodePathColonInPath], scopePath)))
		return exitcode.Usage
	}

	// Validate scope BEFORE name lookup.
	// Compute project root once.
	projectRoot, err := shared.FindProjectRoot(cwd)
	if err != nil {
		if pe, ok := err.(*shared.PathError); ok {
			fmt.Fprint(stderr, yerrors.FormatError(
				yerrors.FormatPath(cwd, cwd), 0, pe.Code, pe.Message))
		} else {
			fmt.Fprint(stderr, yerrors.FormatError("yass", 0,
				yerrors.CodeInternalUncaught,
				fmt.Sprintf("internal error: %s", err)))
		}
		return exitcode.Usage
	}

	// Discover files based on scope.
	var files []string
	if scopePath != "" {
		// Validate scope path exists.
		absScope := scopePath
		if !filepath.IsAbs(absScope) {
			absScope = filepath.Join(cwd, absScope)
		}
		absScope = filepath.Clean(absScope)

		info, statErr := os.Stat(absScope)
		if statErr != nil {
			fmt.Fprint(stderr, yerrors.FormatError(
				yerrors.FormatPath(scopePath, cwd), 0,
				yerrors.CodeQueryScopeNotFound,
				fmt.Sprintf(yerrors.Messages[yerrors.CodeQueryScopeNotFound], scopePath)))
			return exitcode.Usage
		}

		if info.IsDir() {
			discovered, _, discoverErr := shared.DiscoverSpecFiles(scopePath, cwd, projectRoot)
			if discoverErr != nil {
				if pe, ok := discoverErr.(*shared.PathError); ok {
					fmt.Fprint(stderr, yerrors.FormatError(
						yerrors.FormatPath(pe.Path, cwd), 0, pe.Code, pe.Message))
				} else {
					fmt.Fprint(stderr, yerrors.FormatError("yass", 0,
						yerrors.CodeInternalUncaught,
						fmt.Sprintf("internal error: %s", discoverErr)))
				}
				return exitcode.Usage
			}
			if len(discovered) == 0 {
				fmt.Fprint(stderr, yerrors.FormatError(
					yerrors.FormatPath(scopePath, cwd), 0,
					yerrors.CodeQueryScopeEmpty,
					fmt.Sprintf(yerrors.Messages[yerrors.CodeQueryScopeEmpty], scopePath)))
				return exitcode.Usage
			}
			files = discovered
		} else {
			// File scope: use DiscoverSpecFiles for extension checking.
			discovered, _, discoverErr := shared.DiscoverSpecFiles(scopePath, cwd, projectRoot)
			if discoverErr != nil {
				if pe, ok := discoverErr.(*shared.PathError); ok {
					// Map bad_extension to scope_empty for file scope.
					if pe.Code == yerrors.CodePathBadExtension {
						fmt.Fprint(stderr, yerrors.FormatError(
							yerrors.FormatPath(scopePath, cwd), 0,
							yerrors.CodeQueryScopeEmpty,
							fmt.Sprintf(yerrors.Messages[yerrors.CodeQueryScopeEmpty], scopePath)))
					} else {
						fmt.Fprint(stderr, yerrors.FormatError(
							yerrors.FormatPath(pe.Path, cwd), 0, pe.Code, pe.Message))
					}
				} else {
					fmt.Fprint(stderr, yerrors.FormatError("yass", 0,
						yerrors.CodeInternalUncaught,
						fmt.Sprintf("internal error: %s", discoverErr)))
				}
				return exitcode.Usage
			}
			files = discovered
		}
	} else {
		// No scope: discover from project root.
		discovered, _, discoverErr := shared.DiscoverSpecFiles("", cwd, projectRoot)
		if discoverErr != nil {
			if pe, ok := discoverErr.(*shared.PathError); ok {
				fmt.Fprint(stderr, yerrors.FormatError(
					yerrors.FormatPath(pe.Path, cwd), 0, pe.Code, pe.Message))
			} else {
				fmt.Fprint(stderr, yerrors.FormatError("yass", 0,
					yerrors.CodeInternalUncaught,
					fmt.Sprintf("internal error: %s", discoverErr)))
			}
			return exitcode.Usage
		}
		if len(discovered) == 0 {
			fmt.Fprint(stderr, yerrors.FormatError("yass", 0,
				yerrors.CodeDiscoverNoFiles,
				yerrors.Messages[yerrors.CodeDiscoverNoFiles]))
			return exitcode.Usage
		}
		files = discovered
	}

	// Perform name lookup.
	matches, lookupErr := NameLookup(specName, files)
	if lookupErr != nil {
		// name_blank error
		fmt.Fprint(stderr, yerrors.FormatError("yass", 0,
			yerrors.CodeQueryNameBlank,
			yerrors.Messages[yerrors.CodeQueryNameBlank]))
		return exitcode.Usage
	}

	// Handle results.
	switch len(matches) {
	case 0:
		// No matches.
		fmt.Fprint(stderr, yerrors.FormatError("yass", 0,
			yerrors.CodeQueryNoMatch,
			fmt.Sprintf(yerrors.Messages[yerrors.CodeQueryNoMatch], specName)))
		return exitcode.Processing

	case 1:
		// Single match: extract, inline CONFORMS, emit YAML fragment.
		match := matches[0]
		return emitSingleMatch(match, cwd, projectRoot, stdout, stderr)

	default:
		// Multiple matches: emit disambiguation rows in list format.
		for _, match := range matches {
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", match.File, match.SpecName, match.Description)
		}
		return exitcode.Success
	}
}

// emitSingleMatch handles the single-match case: parse, inline CONFORMS, emit fragment.
func emitSingleMatch(match Match, cwd, projectRoot string, stdout, stderr io.Writer) int {
	// Resolve the file path to absolute for InlineConforms.
	absFile := match.File
	if !filepath.IsAbs(absFile) {
		absFile = filepath.Join(cwd, absFile)
	}
	absFile = filepath.Clean(absFile)

	// Parse the file.
	result, err := parser.ParseFile(absFile)
	if err != nil {
		fmt.Fprint(stderr, yerrors.FormatError("yass", 0,
			yerrors.CodeInternalUncaught,
			fmt.Sprintf("internal error: %s", err)))
		return exitcode.Processing
	}

	// Find the spec document.
	var specDoc *yaml.Node
	for _, doc := range result.Documents {
		name := extractSpecNameFromDoc(doc)
		if name == match.SpecName {
			specDoc = doc
			break
		}
	}

	if specDoc == nil {
		fmt.Fprint(stderr, yerrors.FormatError("yass", 0,
			yerrors.CodeInternalUncaught,
			fmt.Sprintf("internal error: spec %s not found after lookup", match.SpecName)))
		return exitcode.Processing
	}

	// Apply InlineConforms.
	specDoc, inlineErrs := InlineConforms(specDoc, absFile, projectRoot)
	if len(inlineErrs) > 0 {
		// Emit each error, do not emit fragment.
		for _, ie := range inlineErrs {
			errStr := ie.Error()
			code := extractErrorCode(errStr)
			msg := extractErrorMessage(errStr)
			fmt.Fprint(stderr, yerrors.FormatError(
				yerrors.FormatPath(absFile, cwd), 0, code, msg))
		}
		return exitcode.Processing
	}

	// Emit the fragment using the OutputProfile.
	mapping := getMappingNode(specDoc)
	if mapping == nil {
		fmt.Fprint(stderr, yerrors.FormatError("yass", 0,
			yerrors.CodeInternalUncaught,
			"internal error: spec has no mapping"))
		return exitcode.Processing
	}

	fragment := ExtractFragmentFromNode(mapping)
	fmt.Fprint(stdout, fragment)
	return exitcode.Success
}

// extractErrorCode extracts the error code from a formatted error string.
func extractErrorCode(errStr string) string {
	if strings.Contains(errStr, yerrors.CodeQueryConformsNoSlot) {
		return yerrors.CodeQueryConformsNoSlot
	}
	if strings.Contains(errStr, yerrors.CodeQueryConformsUnresolved) {
		return yerrors.CodeQueryConformsUnresolved
	}
	return yerrors.CodeInternalUncaught
}

// extractErrorMessage extracts the message from a formatted "[code] message" string.
func extractErrorMessage(errStr string) string {
	if idx := strings.Index(errStr, "] "); idx >= 0 {
		return errStr[idx+2:]
	}
	return errStr
}
