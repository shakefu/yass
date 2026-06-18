package validate

import (
	"fmt"

	"gopkg.in/yaml.v3"

	yerrors "github.com/shakefu/yass/internal/errors"
)

// CheckUniqueness validates that spec names are unique within a file.
// It reports yass.spec.duplicate_name for each occurrence after the first.
func CheckUniqueness(docs []*yaml.Node, filePath string) []ValidationError {
	var errs []ValidationError

	seen := make(map[string]bool)

	for _, doc := range docs {
		name, line := extractSpecName(doc)
		if name == "" {
			continue // Not a spec document or no valid name.
		}

		if seen[name] {
			errs = append(errs, ValidationError{
				File:    filePath,
				Line:    line,
				Code:    yerrors.CodeSpecDuplicateName,
				Message: fmt.Sprintf(yerrors.Messages[yerrors.CodeSpecDuplicateName], name),
			})
		} else {
			seen[name] = true
		}
	}

	return errs
}

// extractSpecName extracts the spec name and its line number from a document.
// Returns ("", 0) if the document is not a spec or has no valid name.
func extractSpecName(doc *yaml.Node) (string, int) {
	mapping := getMappingNode(doc)
	if mapping == nil {
		return "", 0
	}

	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		valNode := mapping.Content[i+1]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == "spec" {
			if valNode.Kind == yaml.ScalarNode && valNode.Value != "" {
				return valNode.Value, valNode.Line
			}
			return "", 0
		}
	}

	return "", 0
}
