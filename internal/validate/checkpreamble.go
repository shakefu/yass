package validate

import (
	"fmt"

	"gopkg.in/yaml.v3"

	yerrors "github.com/shakefu/yass/internal/errors"
)

// CheckPreamble validates the preamble structure of a parsed YAML document stream.
// It emits at most one error per file, stopping at the first match in priority order:
//  1. yass.preamble.has_spec_key
//  2. yass.yaml.empty_stream
//  3. yass.preamble.missing
//  4. yass.preamble.duplicate
//  5. yass.preamble.misplaced
//  6. yass.preamble.missing_description
//  7. yass.preamble.missing_version
//  8. yass.preamble.unknown_version
//  9. yass.preamble.bad_related
func CheckPreamble(docs []*yaml.Node, filePath string) []ValidationError {
	// (1) Check if first document has a "spec" key — must check before empty_stream
	// because if we have docs and the first is a spec, that takes priority.
	if len(docs) > 0 {
		firstDoc := docs[0]
		if docHasSpecKey(firstDoc) {
			return []ValidationError{
				{
					File:    filePath,
					Line:    firstDoc.Line,
					Code:    yerrors.CodePreambleHasSpecKey,
					Message: yerrors.Messages[yerrors.CodePreambleHasSpecKey],
				},
			}
		}
	}

	// (2) Check for empty stream.
	if len(docs) == 0 {
		return []ValidationError{
			{
				File:    filePath,
				Code:    yerrors.CodeYamlEmptyStream,
				Message: yerrors.Messages[yerrors.CodeYamlEmptyStream],
			},
		}
	}

	firstDoc := docs[0]

	// (3) Check if first doc is a mapping (missing preamble if not).
	if !isMapping(firstDoc) {
		return []ValidationError{
			{
				File:    filePath,
				Line:    firstDoc.Line,
				Code:    yerrors.CodePreambleMissing,
				Message: yerrors.Messages[yerrors.CodePreambleMissing],
			},
		}
	}

	// (4) Check for duplicate preambles and (5) misplaced preambles.
	// A "preamble" is any document that does NOT have a "spec" key.
	// We already know docs[0] is a preamble. Check the rest.
	for i := 1; i < len(docs); i++ {
		if !docHasSpecKey(docs[i]) {
			// This is a non-spec document at a non-first position.
			// Per priority order, check duplicate (4) before misplaced (5).
			// A duplicate means more than one preamble. Since we found one
			// at non-first position, it is both duplicate and misplaced.
			// Priority says duplicate first.
			return []ValidationError{
				{
					File:    filePath,
					Line:    docs[i].Line,
					Code:    yerrors.CodePreambleDuplicate,
					Message: yerrors.Messages[yerrors.CodePreambleDuplicate],
				},
			}
		}
	}

	// Now validate the first doc (which is the preamble) for required fields.
	mapping := getMappingNode(firstDoc)
	if mapping == nil {
		return []ValidationError{
			{
				File:    filePath,
				Line:    firstDoc.Line,
				Code:    yerrors.CodePreambleMissing,
				Message: yerrors.Messages[yerrors.CodePreambleMissing],
			},
		}
	}

	keys := extractKeys(mapping)

	// (6) Check for missing description.
	if !keys["description"] {
		return []ValidationError{
			{
				File:    filePath,
				Line:    mapping.Line,
				Code:    yerrors.CodePreambleMissingDescription,
				Message: yerrors.Messages[yerrors.CodePreambleMissingDescription],
			},
		}
	}

	// (7) Check for missing version.
	if !keys["version"] {
		return []ValidationError{
			{
				File:    filePath,
				Line:    mapping.Line,
				Code:    yerrors.CodePreambleMissingVersion,
				Message: yerrors.Messages[yerrors.CodePreambleMissingVersion],
			},
		}
	}

	// (8) Check version value.
	versionValue := getKeyValue(mapping, "version")
	if versionValue != "v1" {
		return []ValidationError{
			{
				File:    filePath,
				Line:    getKeyLine(mapping, "version"),
				Code:    yerrors.CodePreambleUnknownVersion,
				Message: fmt.Sprintf(yerrors.Messages[yerrors.CodePreambleUnknownVersion], versionValue),
			},
		}
	}

	// (9) Check related field.
	if keys["related"] {
		relatedNode := getKeyNode(mapping, "related")
		if relatedNode != nil {
			if !isSequenceOfStrings(relatedNode) {
				return []ValidationError{
					{
						File:    filePath,
						Line:    relatedNode.Line,
						Code:    yerrors.CodePreambleBadRelated,
						Message: yerrors.Messages[yerrors.CodePreambleBadRelated],
					},
				}
			}
		}
	}

	return nil
}

// docHasSpecKey checks if a document node contains a top-level "spec" key.
func docHasSpecKey(doc *yaml.Node) bool {
	mapping := getMappingNode(doc)
	if mapping == nil {
		return false
	}
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == "spec" {
			return true
		}
	}
	return false
}

// isMapping checks if a document node contains a mapping as its root.
func isMapping(doc *yaml.Node) bool {
	return getMappingNode(doc) != nil
}

// getMappingNode extracts the MappingNode from a document node.
func getMappingNode(doc *yaml.Node) *yaml.Node {
	if doc == nil {
		return nil
	}
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) > 0 && doc.Content[0].Kind == yaml.MappingNode {
			return doc.Content[0]
		}
		return nil
	}
	if doc.Kind == yaml.MappingNode {
		return doc
	}
	return nil
}

// extractKeys returns a set of key names from a MappingNode.
func extractKeys(mapping *yaml.Node) map[string]bool {
	keys := make(map[string]bool)
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return keys
	}
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		if keyNode.Kind == yaml.ScalarNode {
			keys[keyNode.Value] = true
		}
	}
	return keys
}

// getKeyValue returns the string value of a key in a MappingNode.
func getKeyValue(mapping *yaml.Node, key string) string {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		valNode := mapping.Content[i+1]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == key {
			if valNode.Kind == yaml.ScalarNode {
				return valNode.Value
			}
			return ""
		}
	}
	return ""
}

// getKeyLine returns the line number of a key in a MappingNode.
func getKeyLine(mapping *yaml.Node, key string) int {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return 0
	}
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == key {
			return keyNode.Line
		}
	}
	return 0
}

// getKeyNode returns the value node for a key in a MappingNode.
func getKeyNode(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		valNode := mapping.Content[i+1]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == key {
			return valNode
		}
	}
	return nil
}

// isSequenceOfStrings checks if a node is a YAML sequence where every element
// is a string scalar.
func isSequenceOfStrings(node *yaml.Node) bool {
	if node == nil || node.Kind != yaml.SequenceNode {
		return false
	}
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			return false
		}
		// Check that the tag is a string tag (or untagged, which defaults to string for scalars).
		if item.Tag != "" && item.Tag != "!!str" {
			return false
		}
	}
	return true
}
