package parser

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	yerrors "github.com/shakefu/yass/internal/errors"
)

// ParseResult holds the parsed YAML documents and the raw file bytes.
type ParseResult struct {
	Documents []*yaml.Node
	RawBytes  []byte
}

// ParseError carries structured error information for error-line formatting.
type ParseError struct {
	Code    string
	Message string
	Line    int
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// ParseFile reads a file from disk and parses it.
func ParseFile(path string) (*ParseResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", path, err)
	}
	return ParseBytes(data, path)
}

// ParseBytes validates and parses raw YAML bytes into a multi-document stream.
//
// Checks are performed in priority order:
//  1. Not valid UTF-8
//  2. Has UTF-8 BOM
//  3. Empty file (len == 0)
//  4. YAML parse errors
func ParseBytes(data []byte, filename string) (*ParseResult, error) {
	// 1. Check UTF-8 validity.
	if !utf8.Valid(data) {
		return nil, &ParseError{
			Code:    yerrors.CodeYamlNotUTF8,
			Message: yerrors.Messages[yerrors.CodeYamlNotUTF8],
		}
	}

	// 2. Check for UTF-8 BOM.
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return nil, &ParseError{
			Code:    yerrors.CodeYamlHasBOM,
			Message: yerrors.Messages[yerrors.CodeYamlHasBOM],
		}
	}

	// 3. Check for empty file.
	if len(data) == 0 {
		return nil, &ParseError{
			Code:    yerrors.CodeYamlEmptyFile,
			Message: yerrors.Messages[yerrors.CodeYamlEmptyFile],
		}
	}

	// 4. Parse YAML multi-document stream.
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var documents []*yaml.Node

	for {
		var doc yaml.Node
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			msg := yerrors.Messages[yerrors.CodeYamlMalformed]
			// Extract line number from yaml error if available.
			line := 0
			errStr := err.Error()
			if strings.Contains(errStr, "line ") {
				// yaml.v3 errors typically include "line N:"
				fmt.Sscanf(errStr, "yaml: line %d:", &line)
			}
			return nil, &ParseError{
				Code:    yerrors.CodeYamlMalformed,
				Message: msg,
				Line:    line,
			}
		}
		documents = append(documents, &doc)
	}

	// Walk all nodes and fix YAML 1.2 boolean handling:
	// yes/no/on/off should be strings, not booleans.
	for _, doc := range documents {
		fixYAML12Booleans(doc)
	}

	return &ParseResult{
		Documents: documents,
		RawBytes:  data,
	}, nil
}

// fixYAML12Booleans walks the node tree and converts yes/no/on/off
// ScalarNodes with Tag=="!!bool" back to !!str, implementing YAML 1.2
// core schema behavior.
func fixYAML12Booleans(node *yaml.Node) {
	if node == nil {
		return
	}

	if node.Kind == yaml.ScalarNode && node.Tag == "!!bool" {
		lower := strings.ToLower(node.Value)
		if lower == "yes" || lower == "no" || lower == "on" || lower == "off" {
			node.Tag = "!!str"
		}
	}

	for _, child := range node.Content {
		fixYAML12Booleans(child)
	}
}
