package validate

import (
	"fmt"

	"gopkg.in/yaml.v3"

	yerrors "github.com/shakefu/yass/internal/errors"
	"github.com/shakefu/yass/internal/parser"
)

// CheckYAML validates YAML well-formedness for a single file.
// It returns the parsed documents on success, or a list of validation errors.
// At most one error is emitted per file, in priority order:
// not_utf8 > has_bom > empty_file > malformed > duplicate_key > anchor_or_alias
func CheckYAML(filePath string) ([]*yaml.Node, []ValidationError) {
	result, err := parser.ParseFile(filePath)
	if err != nil {
		// The parser returns ParseError for YAML issues.
		if pe, ok := err.(*parser.ParseError); ok {
			return nil, []ValidationError{
				{
					File:    filePath,
					Line:    pe.Line,
					Code:    pe.Code,
					Message: pe.Message,
				},
			}
		}
		// Filesystem error — treat as unreadable (not our concern here,
		// but return a generic error).
		return nil, []ValidationError{
			{
				File:    filePath,
				Code:    yerrors.CodeYamlMalformed,
				Message: yerrors.Messages[yerrors.CodeYamlMalformed],
			},
		}
	}

	docs := result.Documents

	// Check for duplicate keys in all documents.
	for _, doc := range docs {
		if key, line := findDuplicateKey(doc); key != "" {
			return nil, []ValidationError{
				{
					File:    filePath,
					Line:    line,
					Code:    yerrors.CodeYamlDuplicateKey,
					Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeYamlDuplicateKey], key),
				},
			}
		}
	}

	// Check for anchors, aliases, and explicit tags in all documents.
	for _, doc := range docs {
		if line := findAnchorAliasOrTag(doc); line > 0 {
			return nil, []ValidationError{
				{
					File:    filePath,
					Line:    line,
					Code:    yerrors.CodeYamlAnchorOrAlias,
					Message: yerrors.Messages[yerrors.CodeYamlAnchorOrAlias],
				},
			}
		}
	}

	return docs, nil
}

// findDuplicateKey walks the yaml.Node tree and returns the first duplicate
// key found in any MappingNode. Returns the key name and line number,
// or ("", 0) if no duplicates are found.
func findDuplicateKey(node *yaml.Node) (string, int) {
	if node == nil {
		return "", 0
	}

	if node.Kind == yaml.MappingNode {
		seen := make(map[string]bool)
		for i := 0; i < len(node.Content)-1; i += 2 {
			keyNode := node.Content[i]
			if keyNode.Kind == yaml.ScalarNode {
				if seen[keyNode.Value] {
					return keyNode.Value, keyNode.Line
				}
				seen[keyNode.Value] = true
			}
		}
	}

	// Recurse into children.
	for _, child := range node.Content {
		if key, line := findDuplicateKey(child); key != "" {
			return key, line
		}
	}

	return "", 0
}

// defaultTags are the standard YAML tags that are allowed.
var defaultTags = map[string]bool{
	"":            true,
	"!!str":       true,
	"!!int":       true,
	"!!float":     true,
	"!!bool":      true,
	"!!null":      true,
	"!!map":       true,
	"!!seq":       true,
	"!!binary":    true,
	"!!merge":     true,
	"!!timestamp": true,
}

// findAnchorAliasOrTag walks the yaml.Node tree looking for anchors,
// aliases, or non-standard explicit tags. Returns the line number of the
// first occurrence, or 0 if none found.
func findAnchorAliasOrTag(node *yaml.Node) int {
	if node == nil {
		return 0
	}

	// Check for anchor.
	if node.Anchor != "" {
		return node.Line
	}

	// Check for alias.
	if node.Kind == yaml.AliasNode {
		return node.Line
	}

	// Check for non-standard explicit tag.
	if node.Tag != "" && !defaultTags[node.Tag] {
		return node.Line
	}

	// Recurse into children.
	for _, child := range node.Content {
		if line := findAnchorAliasOrTag(child); line > 0 {
			return line
		}
	}

	return 0
}
