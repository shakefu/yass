package query

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/shakefu/yass/internal/parser"
)

// ExtractFragment parses a file, finds the spec document matching specName,
// and emits it as a single YAML document starting with "---\n".
// It does NOT perform CONFORMS inlining -- the caller handles that separately.
func ExtractFragment(filePath string, specName string) (string, error) {
	result, err := parser.ParseFile(filePath)
	if err != nil {
		return "", fmt.Errorf("cannot parse file %s: %w", filePath, err)
	}

	for _, doc := range result.Documents {
		name := extractSpecNameFromDoc(doc)
		if name == specName {
			mapping := getMappingNode(doc)
			if mapping == nil {
				return "", fmt.Errorf("spec %s has no mapping content", specName)
			}
			return emitYAMLFragment(mapping), nil
		}
	}

	return "", fmt.Errorf("spec %s not found in %s", specName, filePath)
}

// ExtractFragmentFromNode emits a YAML fragment from an already-processed
// mapping node, applying OutputProfile formatting.
func ExtractFragmentFromNode(mapping *yaml.Node) string {
	return emitYAMLFragment(mapping)
}

// emitYAMLFragment renders a mapping node as a YAML fragment with OutputProfile formatting.
func emitYAMLFragment(mapping *yaml.Node) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	emitMapping(&sb, mapping, 0)
	return sb.String()
}

// emitMapping writes a mapping node at the given indent level.
func emitMapping(sb *strings.Builder, node *yaml.Node, indent int) {
	if node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]
		prefix := strings.Repeat("  ", indent)

		sb.WriteString(prefix)
		sb.WriteString(keyNode.Value)
		sb.WriteString(":")

		switch valNode.Kind {
		case yaml.ScalarNode:
			sb.WriteString(" ")
			sb.WriteString(formatScalar(valNode.Value))
			sb.WriteString("\n")

		case yaml.SequenceNode:
			sb.WriteString("\n")
			emitSequence(sb, valNode, indent)

		case yaml.MappingNode:
			sb.WriteString("\n")
			emitMapping(sb, valNode, indent+1)

		default:
			sb.WriteString("\n")
		}
	}
}

// emitSequence writes a sequence node at the given indent level.
func emitSequence(sb *strings.Builder, node *yaml.Node, indent int) {
	if node.Kind != yaml.SequenceNode {
		return
	}
	prefix := strings.Repeat("  ", indent)
	for _, item := range node.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			sb.WriteString(prefix)
			sb.WriteString("- ")
			sb.WriteString(formatScalar(item.Value))
			sb.WriteString("\n")

		case yaml.MappingNode:
			emitMappingAsListItem(sb, item, indent)

		case yaml.SequenceNode:
			sb.WriteString(prefix)
			sb.WriteString("-\n")
			emitSequence(sb, item, indent+1)

		default:
			sb.WriteString(prefix)
			sb.WriteString("-\n")
		}
	}
}

// keyValPair holds a key-value pair from a YAML mapping node.
type keyValPair struct {
	key *yaml.Node
	val *yaml.Node
}

// emitMappingAsListItem writes a mapping as a list item with "- " prefix.
// The first key-value pair shares the "- " line; subsequent pairs are
// indented by 2 more spaces.
func emitMappingAsListItem(sb *strings.Builder, node *yaml.Node, indent int) {
	if node.Kind != yaml.MappingNode || len(node.Content) < 2 {
		prefix := strings.Repeat("  ", indent)
		sb.WriteString(prefix)
		sb.WriteString("-\n")
		return
	}

	prefix := strings.Repeat("  ", indent)

	// Emit provenance comment only (not source YAML comments).
	// Provenance comments start with "CONFORMS: " and are added by addProvenanceComment.
	if node.HeadComment != "" && strings.HasPrefix(node.HeadComment, "CONFORMS: ") {
		sb.WriteString("# ")
		sb.WriteString(node.HeadComment)
		sb.WriteString("\n")
	}

	// Reorder keys per OutputProfile: normativity first, then WHEN, then references.
	ordered := orderObligationKeys(node)

	first := true
	for _, pair := range ordered {
		keyNode := pair.key
		valNode := pair.val

		if first {
			sb.WriteString(prefix)
			sb.WriteString("- ")
			first = false
		} else {
			sb.WriteString(prefix)
			sb.WriteString("  ")
		}

		sb.WriteString(keyNode.Value)
		sb.WriteString(":")

		switch valNode.Kind {
		case yaml.ScalarNode:
			sb.WriteString(" ")
			sb.WriteString(formatScalar(valNode.Value))
			sb.WriteString("\n")

		case yaml.SequenceNode:
			sb.WriteString("\n")
			emitSequence(sb, valNode, indent+2)

		case yaml.MappingNode:
			sb.WriteString("\n")
			emitMapping(sb, valNode, indent+2)

		default:
			sb.WriteString("\n")
		}
	}
}

// orderObligationKeys reorders obligation keys per the OutputProfile:
// Normativity keyword first, then WHEN, then references.
func orderObligationKeys(node *yaml.Node) []keyValPair {
	if node.Kind != yaml.MappingNode {
		return nil
	}

	var normKVs []keyValPair
	var whenKVs []keyValPair
	var refKVs []keyValPair
	var otherKVs []keyValPair

	for i := 0; i < len(node.Content)-1; i += 2 {
		pair := keyValPair{key: node.Content[i], val: node.Content[i+1]}
		key := pair.key.Value

		if normativityKeywords[key] {
			normKVs = append(normKVs, pair)
		} else if key == "WHEN" {
			whenKVs = append(whenKVs, pair)
		} else if key == "CONFORMS" || key == "USES" || key == "SEE" {
			refKVs = append(refKVs, pair)
		} else {
			otherKVs = append(otherKVs, pair)
		}
	}

	var result []keyValPair
	result = append(result, normKVs...)
	result = append(result, whenKVs...)
	result = append(result, refKVs...)
	result = append(result, otherKVs...)
	return result
}

// numericRegex matches YAML numeric literals (integers, floats, octal, hex,
// infinity, NaN, etc.)
var numericRegex = regexp.MustCompile(
	`^(?:` +
		`[+-]?[0-9]+` + // decimal integer
		`|[+-]?0x[0-9a-fA-F]+` + // hex integer
		`|[+-]?0o[0-7]+` + // octal integer
		`|[+-]?0b[01]+` + // binary integer
		`|[+-]?[0-9]*\.[0-9]+(?:[eE][+-]?[0-9]+)?` + // float with decimal point
		`|[+-]?[0-9]+[eE][+-]?[0-9]+` + // float with exponent only
		`|[+-]?\.(?:inf|Inf|INF)` + // infinity
		`|\.(?:nan|NaN|NAN)` + // NaN
		`)$`,
)

// leadingSpecialChars contains characters that trigger quoting when they
// appear as the first character of a scalar.
const leadingSpecialChars = "?-*&!|>%@"

// formatScalar formats a scalar value per the OutputProfile rules.
// Plain scalars are unquoted by default, but double-quoted when they
// contain special characters or match YAML core types.
func formatScalar(value string) string {
	if needsQuoting(value) {
		return doubleQuote(value)
	}
	return value
}

// needsQuoting determines if a scalar value needs double-quoting.
func needsQuoting(value string) bool {
	if value == "" {
		return true
	}

	// Contains newline or carriage return
	if strings.ContainsAny(value, "\n\r") {
		return true
	}

	// Contains ": " (colon-space)
	if strings.Contains(value, ": ") {
		return true
	}

	// Leading character in ?-*&!|>%@
	if strings.ContainsRune(leadingSpecialChars, rune(value[0])) {
		return true
	}

	// Leading or trailing whitespace
	if value[0] == ' ' || value[0] == '\t' ||
		value[len(value)-1] == ' ' || value[len(value)-1] == '\t' {
		return true
	}

	// Contains # preceded by space (inline comment ambiguity)
	if strings.Contains(value, " #") {
		return true
	}

	// YAML 1.2 core-schema type tokens (case variations)
	lower := strings.ToLower(value)
	switch lower {
	case "true", "false", "null", "yes", "no", "on", "off":
		return true
	}

	// Numeric literal
	if numericRegex.MatchString(value) {
		return true
	}

	return false
}

// doubleQuote wraps a string in double quotes with proper escaping.
func doubleQuote(value string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}
