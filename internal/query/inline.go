package query

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	yerrors "github.com/shakefu/yass/internal/errors"
	"github.com/shakefu/yass/internal/parser"
)

// slotKeys is the set of valid slot keys.
var slotKeys = map[string]bool{
	"INPUT":       true,
	"RETURN":      true,
	"ERROR":       true,
	"SIDE-EFFECT": true,
	"INVARIANT":   true,
}

// normativityKeywords is the set of recognized normativity keywords.
var normativityKeywords = map[string]bool{
	"MUST":       true,
	"MUST-NOT":   true,
	"SHOULD":     true,
	"SHOULD-NOT": true,
	"MAY":        true,
}

// InlineConforms resolves CONFORMS references in a spec document by inlining
// the referenced obligations. It modifies the document in place and returns
// it along with any errors encountered.
//
// filePath is the absolute path of the spec file being processed.
// projectRoot is the absolute path of the project root for root-anchored refs.
func InlineConforms(specDoc *yaml.Node, filePath string, projectRoot string) (*yaml.Node, []error) {
	var errs []error

	mapping := getMappingNode(specDoc)
	if mapping == nil {
		return specDoc, nil
	}

	basePath := filepath.Dir(filePath)

	// Walk all slots.
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		valNode := mapping.Content[i+1]

		if keyNode.Kind != yaml.ScalarNode {
			continue
		}
		if !slotKeys[keyNode.Value] {
			continue
		}
		if valNode.Kind != yaml.SequenceNode {
			continue
		}

		// Process obligations in this slot.
		newObligations := processSlotObligations(valNode.Content, filePath, basePath, projectRoot, &errs)
		valNode.Content = newObligations
	}

	return specDoc, errs
}

// processSlotObligations processes each obligation in a slot, inlining CONFORMS refs.
func processSlotObligations(obligations []*yaml.Node, filePath, basePath, projectRoot string, errs *[]error) []*yaml.Node {
	var result []*yaml.Node

	for _, oblNode := range obligations {
		if oblNode.Kind != yaml.MappingNode {
			result = append(result, oblNode)
			continue
		}

		conformsRef, conformsIdx := findConformsRef(oblNode)
		if conformsRef == "" {
			// No CONFORMS ref, keep as-is.
			result = append(result, oblNode)
			continue
		}

		// Parse the CONFORMS ref target.
		path, specName, slot := parseRefTarget(conformsRef)

		// CONFORMS without ::SLOT -> error.
		if slot == "" {
			*errs = append(*errs, fmt.Errorf("[%s] %s",
				yerrors.CodeQueryConformsNoSlot,
				fmt.Sprintf(yerrors.Messages[yerrors.CodeQueryConformsNoSlot], conformsRef)))
			result = append(result, oblNode)
			continue
		}

		// Resolve the referenced obligations.
		referencedObligations, err := resolveConformsRef(path, specName, slot, filePath, basePath, projectRoot)
		if err != nil {
			*errs = append(*errs, fmt.Errorf("[%s] %s",
				yerrors.CodeQueryConformsUnresolved,
				fmt.Sprintf(yerrors.Messages[yerrors.CodeQueryConformsUnresolved], conformsRef)))
			result = append(result, oblNode)
			continue
		}

		// Determine if this is reference-only or normative.
		isRefOnly := isReferenceOnlyObligation(oblNode)
		whenGuard := getWhenGuard(oblNode)

		if isRefOnly {
			// Reference-only: replace with inlined obligations.
			for _, inlined := range referencedObligations {
				clone := cloneNode(inlined)
				if whenGuard != "" {
					applyWhenGuard(clone, whenGuard)
				}
				addProvenanceComment(clone, conformsRef)
				result = append(result, clone)
			}
		} else {
			// Normative: keep the carrier (minus CONFORMS), append inlined after.
			stripped := stripConformsRef(oblNode, conformsIdx)
			result = append(result, stripped)

			for _, inlined := range referencedObligations {
				clone := cloneNode(inlined)
				if whenGuard != "" {
					applyWhenGuard(clone, whenGuard)
				}
				addProvenanceComment(clone, conformsRef)
				result = append(result, clone)
			}
		}
	}

	return result
}

// findConformsRef finds a CONFORMS reference in an obligation node.
// Returns the ref target string and the index of the CONFORMS key in Content.
func findConformsRef(oblNode *yaml.Node) (string, int) {
	if oblNode.Kind != yaml.MappingNode {
		return "", -1
	}

	for i := 0; i < len(oblNode.Content)-1; i += 2 {
		keyNode := oblNode.Content[i]
		valNode := oblNode.Content[i+1]

		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == "CONFORMS" {
			if valNode.Kind == yaml.ScalarNode {
				return valNode.Value, i
			}
			return "", -1
		}
	}

	return "", -1
}

// isReferenceOnlyObligation checks if an obligation is reference-only:
// has at least one reference but no normativity keyword and no WHEN guard.
func isReferenceOnlyObligation(oblNode *yaml.Node) bool {
	if oblNode.Kind != yaml.MappingNode {
		return false
	}

	hasNormativity := false
	hasWhen := false

	for i := 0; i < len(oblNode.Content)-1; i += 2 {
		keyNode := oblNode.Content[i]
		if keyNode.Kind != yaml.ScalarNode {
			continue
		}
		if normativityKeywords[keyNode.Value] {
			hasNormativity = true
		}
		if keyNode.Value == "WHEN" {
			hasWhen = true
		}
	}

	return !hasNormativity && !hasWhen
}

// getWhenGuard extracts the WHEN guard value from an obligation, if present.
func getWhenGuard(oblNode *yaml.Node) string {
	if oblNode.Kind != yaml.MappingNode {
		return ""
	}

	for i := 0; i < len(oblNode.Content)-1; i += 2 {
		keyNode := oblNode.Content[i]
		valNode := oblNode.Content[i+1]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == "WHEN" {
			if valNode.Kind == yaml.ScalarNode {
				return valNode.Value
			}
		}
	}

	return ""
}

// applyWhenGuard applies or combines an outer WHEN guard to an obligation.
// If the obligation already has a WHEN guard, combine: "<outer> and <inner>".
// If it does not, insert a WHEN after the normativity keyword.
func applyWhenGuard(oblNode *yaml.Node, outerWhen string) {
	if oblNode.Kind != yaml.MappingNode {
		return
	}

	// Look for existing WHEN.
	for i := 0; i < len(oblNode.Content)-1; i += 2 {
		keyNode := oblNode.Content[i]
		valNode := oblNode.Content[i+1]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == "WHEN" {
			if valNode.Kind == yaml.ScalarNode {
				// Combine: "<outer> and <inner>"
				valNode.Value = outerWhen + " and " + valNode.Value
			}
			return
		}
	}

	// No existing WHEN, add one. Insert after any normativity keyword.
	whenKey := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "WHEN"}
	whenVal := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: outerWhen}

	insertIdx := 0
	for i := 0; i < len(oblNode.Content)-1; i += 2 {
		keyNode := oblNode.Content[i]
		if keyNode.Kind == yaml.ScalarNode && normativityKeywords[keyNode.Value] {
			insertIdx = i + 2
			break
		}
	}

	newContent := make([]*yaml.Node, 0, len(oblNode.Content)+2)
	newContent = append(newContent, oblNode.Content[:insertIdx]...)
	newContent = append(newContent, whenKey, whenVal)
	newContent = append(newContent, oblNode.Content[insertIdx:]...)
	oblNode.Content = newContent
}

// addProvenanceComment adds a "# CONFORMS: <ref-target>" comment.
func addProvenanceComment(oblNode *yaml.Node, refTarget string) {
	oblNode.HeadComment = fmt.Sprintf("CONFORMS: %s", refTarget)
}

// stripConformsRef creates a copy of an obligation with the CONFORMS ref removed.
func stripConformsRef(oblNode *yaml.Node, conformsIdx int) *yaml.Node {
	clone := cloneNode(oblNode)
	if clone.Kind != yaml.MappingNode || conformsIdx < 0 || conformsIdx+1 >= len(clone.Content) {
		return clone
	}

	newContent := make([]*yaml.Node, 0, len(clone.Content)-2)
	newContent = append(newContent, clone.Content[:conformsIdx]...)
	newContent = append(newContent, clone.Content[conformsIdx+2:]...)
	clone.Content = newContent

	return clone
}

// resolveConformsRef resolves a CONFORMS reference to its obligations.
func resolveConformsRef(path, specName, slot, filePath, basePath, projectRoot string) ([]*yaml.Node, error) {
	var resolvedPath string

	if path == "" {
		// Same-file ref.
		resolvedPath = filePath
	} else if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		// Relative to the containing directory.
		resolvedPath = filepath.Join(basePath, path+".yass.yaml")
	} else {
		// Root-anchored.
		resolvedPath = filepath.Join(projectRoot, path+".yass.yaml")
	}
	resolvedPath = filepath.Clean(resolvedPath)

	// Check file exists.
	if _, err := os.Stat(resolvedPath); err != nil {
		return nil, fmt.Errorf("file not found: %s", resolvedPath)
	}

	// Parse the file.
	result, err := parser.ParseFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("cannot parse file: %s", resolvedPath)
	}

	// Find the spec.
	for _, doc := range result.Documents {
		name := extractSpecNameFromDoc(doc)
		if name != specName {
			continue
		}

		// Found the spec, now find the slot.
		mapping := getMappingNode(doc)
		if mapping == nil {
			return nil, fmt.Errorf("spec %s has no mapping", specName)
		}

		for i := 0; i < len(mapping.Content)-1; i += 2 {
			keyNode := mapping.Content[i]
			valNode := mapping.Content[i+1]

			if keyNode.Kind == yaml.ScalarNode && keyNode.Value == slot {
				if valNode.Kind != yaml.SequenceNode {
					return nil, fmt.Errorf("slot %s is not a sequence", slot)
				}
				return valNode.Content, nil
			}
		}

		return nil, fmt.Errorf("spec %s does not declare slot %s", specName, slot)
	}

	return nil, fmt.Errorf("spec %s not found in %s", specName, resolvedPath)
}

// parseRefTarget parses a ref target into its components.
// Grammar: (path@)?specName(::SLOT)?
func parseRefTarget(target string) (path, specName, slot string) {
	// Extract slot.
	if idx := strings.Index(target, "::"); idx >= 0 {
		slot = target[idx+2:]
		target = target[:idx]
	}

	// Extract path.
	if idx := strings.LastIndex(target, "@"); idx >= 0 {
		path = target[:idx]
		specName = target[idx+1:]
	} else {
		specName = target
	}

	return
}

// cloneNode performs a deep clone of a yaml.Node.
func cloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}

	clone := &yaml.Node{
		Kind:        node.Kind,
		Style:       node.Style,
		Tag:         node.Tag,
		Value:       node.Value,
		Anchor:      node.Anchor,
		HeadComment: node.HeadComment,
		LineComment: node.LineComment,
		FootComment: node.FootComment,
		Line:        node.Line,
		Column:      node.Column,
	}

	if len(node.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			clone.Content[i] = cloneNode(child)
		}
	}

	return clone
}
