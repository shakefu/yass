package list

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/rivo/uniseg"
	"golang.org/x/text/unicode/norm"
	"gopkg.in/yaml.v3"

	yerrors "github.com/shakefu/yass/internal/errors"
	"github.com/shakefu/yass/internal/exitcode"
	"github.com/shakefu/yass/internal/parser"
	"github.com/shakefu/yass/internal/shared"
)

// whitespaceRun matches one or more whitespace characters (space, tab, newline, etc.).
var whitespaceRun = regexp.MustCompile(`\s+`)

// Run executes the list subcommand.
// args = positional arguments after "list".
// Returns an exit code: 0 success, 1 parse errors, 2 usage/path errors.
func Run(args []string, stdout io.Writer, stderr io.Writer, isTTY bool, termWidth int) int {
	cwd, err := os.Getwd()
	if err != nil {
		line := yerrors.FormatError("yass", 0, yerrors.CodeInternalUncaught, fmt.Sprintf("internal error: %s", err))
		fmt.Fprint(stderr, line)
		return exitcode.Processing
	}

	// Determine optional input path from args.
	var inputPath string
	if len(args) > 0 {
		inputPath = args[0]
	}

	// Check for colon in path argument.
	// ErrorLine spec: MUST-NOT emit any input path containing ":".
	if inputPath != "" && strings.Contains(inputPath, ":") {
		line := yerrors.FormatError(
			"yass",
			0,
			yerrors.CodePathColonInPath,
			fmt.Sprintf(yerrors.Messages[yerrors.CodePathColonInPath], inputPath),
		)
		fmt.Fprint(stderr, line)
		return exitcode.Usage
	}

	// When no path given, find project root and discover from there.
	var projectRoot string
	if inputPath == "" {
		root, err := shared.FindProjectRoot(cwd)
		if err != nil {
			if pe, ok := err.(*shared.PathError); ok {
				line := yerrors.FormatError(
					yerrors.FormatPath(cwd, cwd),
					0,
					pe.Code,
					pe.Message,
				)
				fmt.Fprint(stderr, line)
			} else {
				line := yerrors.FormatError("yass", 0, yerrors.CodeInternalUncaught, fmt.Sprintf("internal error: %s", err))
				fmt.Fprint(stderr, line)
			}
			return exitcode.Usage
		}
		projectRoot = root
	}

	// Discover spec files.
	filePaths, nonFatalErrs, discoverErr := shared.DiscoverSpecFiles(inputPath, cwd, projectRoot)
	if discoverErr != nil {
		emitPathError(discoverErr, cwd, stderr)
		return exitcode.Usage
	}
	for _, e := range nonFatalErrs {
		fmt.Fprint(stderr, e)
	}

	// No files found → exit 0, no output.
	if len(filePaths) == 0 {
		return exitcode.Success
	}

	// Sort files by Unicode code-point order on NFC-normalized path.
	sort.Slice(filePaths, func(i, j int) bool {
		ni := norm.NFC.String(filePaths[i])
		nj := norm.NFC.String(filePaths[j])
		return ni < nj
	})

	hadParseError := false

	for _, filePath := range filePaths {
		rows, parseErr := processFile(filePath, cwd)
		if parseErr != nil {
			fmt.Fprint(stderr, parseErr.Error())
			hadParseError = true
			continue
		}
		for _, row := range rows {
			emitRow(stdout, row, isTTY, termWidth)
		}
	}

	if hadParseError {
		return exitcode.Processing
	}
	return exitcode.Success
}

// listRow holds one output row before formatting.
type listRow struct {
	FilePath    string
	SpecName    string
	Description string
}

// processFile parses a single file and returns list rows.
// Returns a formatted error string on parse failure.
func processFile(filePath, cwd string) ([]listRow, *parseErrorLine) {
	result, err := parser.ParseFile(filePath)
	if err != nil {
		formattedPath := yerrors.FormatPath(filePath, cwd)
		var line int
		var code, msg string
		if pe, ok := err.(*parser.ParseError); ok {
			line = pe.Line
			code = pe.Code
			msg = pe.Message
		} else {
			code = yerrors.CodeYamlMalformed
			msg = yerrors.Messages[yerrors.CodeYamlMalformed]
		}
		errLine := yerrors.FormatError(formattedPath, line, code, msg)
		return nil, &parseErrorLine{line: errLine}
	}

	if len(result.Documents) == 0 {
		return nil, nil
	}

	// Extract preamble description from the first document without "spec" key.
	var description string
	var specDocs []*yaml.Node
	startIdx := 0

	// Check if first doc is a preamble (no "spec" key).
	if len(result.Documents) > 0 {
		firstDoc := result.Documents[0]
		if !docHasSpecKey(firstDoc) {
			description = extractDescription(firstDoc)
			startIdx = 1
		}
	}

	// Collect spec documents.
	for i := startIdx; i < len(result.Documents); i++ {
		doc := result.Documents[i]
		if docHasSpecKey(doc) {
			specDocs = append(specDocs, doc)
		}
	}

	if len(specDocs) == 0 {
		return nil, nil
	}

	// Normalize file path: replace literal tab with space.
	displayPath := strings.ReplaceAll(filePath, "\t", " ")

	// Normalize description.
	normalizedDesc := normalizeDescription(description)

	var rows []listRow
	for _, doc := range specDocs {
		specName := extractSpecName(doc)
		rows = append(rows, listRow{
			FilePath:    displayPath,
			SpecName:    specName,
			Description: normalizedDesc,
		})
	}

	return rows, nil
}

// parseErrorLine wraps a formatted error string.
type parseErrorLine struct {
	line string
}

func (e *parseErrorLine) Error() string {
	return e.line
}

// docHasSpecKey checks if a YAML document node has a top-level "spec" key.
func docHasSpecKey(doc *yaml.Node) bool {
	if doc == nil {
		return false
	}
	// Document node wraps the actual content.
	node := doc
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	if node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		if key.Kind == yaml.ScalarNode && key.Value == "spec" {
			return true
		}
	}
	return false
}

// extractDescription extracts the "description" value from a YAML document node.
// Returns empty string if not found or not a string.
func extractDescription(doc *yaml.Node) string {
	if doc == nil {
		return ""
	}
	node := doc
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	if node.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		val := node.Content[i+1]
		if key.Kind == yaml.ScalarNode && key.Value == "description" {
			if val.Kind == yaml.ScalarNode && (val.Tag == "!!str" || val.Tag == "") {
				return val.Value
			}
			return ""
		}
	}
	return ""
}

// extractSpecName extracts the "spec" value from a YAML document node.
func extractSpecName(doc *yaml.Node) string {
	if doc == nil {
		return ""
	}
	node := doc
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	if node.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		val := node.Content[i+1]
		if key.Kind == yaml.ScalarNode && key.Value == "spec" {
			if val.Kind == yaml.ScalarNode {
				return val.Value
			}
			return ""
		}
	}
	return ""
}

// normalizeDescription normalizes a description string:
// - Replace all whitespace runs (newlines, tabs, spaces) with single ASCII space
// - Strip leading/trailing whitespace
// - NFC-normalize
func normalizeDescription(desc string) string {
	if desc == "" {
		return ""
	}
	// Replace all whitespace runs with a single space.
	s := whitespaceRun.ReplaceAllString(desc, " ")
	// Strip leading/trailing whitespace.
	s = strings.TrimSpace(s)
	// NFC-normalize.
	s = norm.NFC.String(s)
	return s
}

// graphemeLen returns the number of grapheme clusters in a string.
func graphemeLen(s string) int {
	count := 0
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		count++
	}
	return count
}

// truncateToGraphemes truncates a string to at most maxGraphemes grapheme clusters.
// Returns the truncated string and whether it was actually shortened.
func truncateToGraphemes(s string, maxGraphemes int) (string, bool) {
	if maxGraphemes <= 0 {
		return "", s != ""
	}
	count := 0
	g := uniseg.NewGraphemes(s)
	var result strings.Builder
	for g.Next() {
		if count >= maxGraphemes {
			return result.String(), true
		}
		result.WriteString(g.Str())
		count++
	}
	return result.String(), false
}

// emitRow writes a single list row to stdout.
func emitRow(w io.Writer, row listRow, isTTY bool, termWidth int) {
	desc := row.Description

	if isTTY && termWidth > 0 {
		desc = truncateDescription(row.FilePath, row.SpecName, desc, termWidth)
	}

	fmt.Fprintf(w, "%s\t%s\t%s\n", row.FilePath, row.SpecName, desc)
}

// truncateDescription handles TTY truncation of the description field.
func truncateDescription(filePath, specName, desc string, width int) string {
	if desc == "" {
		return ""
	}

	pathLen := graphemeLen(filePath)
	nameLen := graphemeLen(specName)

	// Fixed columns: path + tab + name + tab = pathLen + 1 + nameLen + 1
	fixedCols := pathLen + 1 + nameLen + 1
	markerLen := 3 // "..."

	// If path+name+separators+marker >= width → empty third field, no marker
	if fixedCols+markerLen >= width {
		return ""
	}

	// Available space for description
	available := width - fixedCols
	descLen := graphemeLen(desc)

	if descLen <= available {
		// Fits without truncation.
		return desc
	}

	// Need to truncate. Available space for text = available - markerLen
	textSpace := available - markerLen
	if textSpace <= 0 {
		return ""
	}

	truncated, _ := truncateToGraphemes(desc, textSpace)
	return truncated + "..."
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
