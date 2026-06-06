package query

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"gopkg.in/yaml.v3"

	yerrors "github.com/shakefu/yass/internal/errors"
	"github.com/shakefu/yass/internal/parser"
)

// Match represents a single spec that matched a name query.
type Match struct {
	File        string
	SpecName    string
	Description string
	DocIndex    int
}

// NameLookup searches files for specs matching the given name.
// Matches full spec name with case-sensitive byte comparison, and also
// matches any dot-aligned trailing suffix of a namespaced spec name.
func NameLookup(name string, files []string) ([]Match, error) {
	// Empty name -> error
	if name == "" {
		return nil, fmt.Errorf("[%s] %s", yerrors.CodeQueryNameBlank, yerrors.Messages[yerrors.CodeQueryNameBlank])
	}

	// Whitespace in name -> treat as no-match (not blank error)
	for _, r := range name {
		if unicode.IsSpace(r) {
			return nil, nil
		}
	}

	var matches []Match

	for _, file := range files {
		result, err := parser.ParseFile(file)
		if err != nil {
			// Skip unparseable files during lookup
			continue
		}

		// Extract preamble description from the first document.
		desc := extractPreambleDescription(result.Documents)

		for docIdx, doc := range result.Documents {
			specName := extractSpecNameFromDoc(doc)
			if specName == "" {
				continue
			}

			if matchesName(specName, name) {
				matches = append(matches, Match{
					File:        file,
					SpecName:    specName,
					Description: desc,
					DocIndex:    docIdx,
				})
			}
		}
	}

	return matches, nil
}

// matchesName checks if a spec name matches the query name.
// It matches the full name exactly, or any dot-aligned trailing suffix.
// e.g., query "Foo" matches "a.b.Foo"; query "b.Foo" matches "a.b.Foo";
// query "oo" does NOT match "Foo"; query "xb.Foo" does NOT match "a.b.Foo".
func matchesName(specName, queryName string) bool {
	// Exact match
	if specName == queryName {
		return true
	}

	// Dot-aligned trailing suffix: queryName must be shorter than specName,
	// the character before the suffix in specName must be '.', and the
	// suffix must equal queryName byte-for-byte.
	if len(queryName) < len(specName) &&
		specName[len(specName)-len(queryName)-1] == '.' &&
		specName[len(specName)-len(queryName):] == queryName {
		return true
	}

	return false
}

// extractSpecNameFromDoc extracts the spec name from a YAML document node.
func extractSpecNameFromDoc(doc *yaml.Node) string {
	mapping := getMappingNode(doc)
	if mapping == nil {
		return ""
	}

	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		valNode := mapping.Content[i+1]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == "spec" {
			if valNode.Kind == yaml.ScalarNode && valNode.Value != "" {
				return valNode.Value
			}
			return ""
		}
	}

	return ""
}

// extractPreambleDescription extracts the description from the preamble (first doc).
func extractPreambleDescription(docs []*yaml.Node) string {
	if len(docs) == 0 {
		return ""
	}

	firstDoc := docs[0]
	mapping := getMappingNode(firstDoc)
	if mapping == nil {
		return ""
	}

	// If it has a spec key, it's not a preamble.
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == "spec" {
			return ""
		}
	}

	// Look for description key.
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		valNode := mapping.Content[i+1]
		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == "description" {
			if valNode.Kind == yaml.ScalarNode {
				return normalizeDescription(valNode.Value)
			}
			return ""
		}
	}

	return ""
}

// normalizeDescription replaces runs of whitespace with a single space,
// trims leading/trailing whitespace, and NFC-normalizes the result.
func normalizeDescription(desc string) string {
	var builder strings.Builder
	inSpace := false
	for _, r := range desc {
		if unicode.IsSpace(r) {
			if !inSpace {
				builder.WriteRune(' ')
				inSpace = true
			}
		} else {
			builder.WriteRune(r)
			inSpace = false
		}
	}
	s := strings.TrimSpace(builder.String())
	return norm.NFC.String(s)
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
