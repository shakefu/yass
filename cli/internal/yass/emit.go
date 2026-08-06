package yass

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// The emitter is hand-rolled rather than delegated to a YAML serializer: the
// fragment grammar is small and fully specified, and byte-identical output with
// provenance comments placed between list items is a property of direct string
// formatting rather than a hope about a library's defaults.

// yamlIndicators are the characters that cannot open a plain scalar.
const yamlIndicators = ",[]{}#&*!|>'\"%@`"

// resolvesToNonString reports whether a plain scalar would resolve to
// something other than a string under the YAML 1.2 core schema.
func resolvesToNonString(v string) bool {
	switch v {
	case "null", "Null", "NULL", "~", "true", "True", "TRUE", "false", "False", "FALSE", "":
		return true
	}
	if _, err := strconv.ParseInt(v, 0, 64); err == nil {
		return true
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return true
	}
	switch v {
	case ".inf", ".Inf", ".INF", "-.inf", "-.Inf", "-.INF", ".nan", ".NaN", ".NAN":
		return true
	}
	return false
}

// plainSafe reports whether a value can be written as a plain scalar and read
// back as the identical string.
func plainSafe(v string) bool {
	if v == "" {
		return false
	}
	if strings.ContainsAny(v, "\n\r\t") {
		return false
	}
	for _, r := range v {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	if v[0] == ' ' || v[len(v)-1] == ' ' {
		return false
	}
	if strings.ContainsRune(yamlIndicators, rune(v[0])) {
		return false
	}
	if c := v[0]; c == '-' || c == '?' || c == ':' {
		if len(v) == 1 || v[1] == ' ' {
			return false
		}
	}
	if strings.Contains(v, ": ") || strings.HasSuffix(v, ":") {
		return false
	}
	if strings.Contains(v, " #") {
		return false
	}
	return !resolvesToNonString(v)
}

// quoteDouble writes a double-quoted scalar that reads back byte-for-byte.
func quoteDouble(v string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range v {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || r == 0x7f {
				b.WriteString(fmt.Sprintf(`\x%02x`, r))
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// blockable reports whether a multi-line value can be written as a literal
// block scalar without losing bytes.
func blockable(v string) bool {
	if !strings.Contains(v, "\n") {
		return false
	}
	if strings.ContainsRune(v, '\r') {
		return false
	}
	for _, r := range v {
		if r < 0x20 && r != '\n' {
			return false
		}
		if r == 0x7f {
			return false
		}
	}
	return true
}

// emitScalarValue writes `key: value` for a scalar, choosing plain, quoted, or
// a literal block scalar so the value reads back unchanged.
func emitScalarValue(out *[]string, indent, key, v string) {
	switch {
	case plainSafe(v):
		*out = append(*out, indent+key+": "+v)
	case blockable(v):
		header, body := blockScalar(v, indent+"  ")
		*out = append(*out, indent+key+": "+header)
		*out = append(*out, body...)
	default:
		*out = append(*out, indent+key+": "+quoteDouble(v))
	}
}

// emitScalarItem writes `- value` for a sequence item.
func emitScalarItem(out *[]string, indent, v string) {
	switch {
	case plainSafe(v):
		*out = append(*out, indent+"- "+v)
	case blockable(v):
		header, body := blockScalar(v, indent+"  ")
		*out = append(*out, indent+"- "+header)
		*out = append(*out, body...)
	default:
		*out = append(*out, indent+"- "+quoteDouble(v))
	}
}

// blockScalar renders a literal block scalar header and its indented body. The
// body is always indented two spaces past its parent, so the explicit
// indentation indicator a leading-space first line needs is always 2.
func blockScalar(v, indent string) (string, []string) {
	header := "|"
	if strings.HasPrefix(v, " ") || strings.HasPrefix(v, "\t") {
		header += "2"
	}
	trailing := 0
	for i := len(v) - 1; i >= 0 && v[i] == '\n'; i-- {
		trailing++
	}
	text := v
	switch {
	case trailing == 0:
		header += "-"
	case trailing > 1:
		header += "+"
		text = v[:len(v)-1]
	default:
		text = v[:len(v)-1]
	}
	body := strings.Split(text, "\n")
	lines := make([]string, 0, len(body))
	for _, l := range body {
		if l == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, indent+l)
	}
	return header, lines
}

// emitNode renders an arbitrary node under a key, used for a preamble's or a
// design's authored entries.
func emitNode(out *[]string, indent, key string, n *yaml.Node) {
	if n == nil {
		*out = append(*out, indent+key+":")
		return
	}
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Tag == "!!null" {
			*out = append(*out, indent+key+":")
			return
		}
		if n.Tag != "!!str" {
			*out = append(*out, indent+key+": "+n.Value)
			return
		}
		emitScalarValue(out, indent, key, n.Value)
	case yaml.SequenceNode:
		*out = append(*out, indent+key+":")
		for _, item := range n.Content {
			emitSeqItem(out, indent, item)
		}
	case yaml.MappingNode:
		*out = append(*out, indent+key+":")
		for i := 0; i+1 < len(n.Content); i += 2 {
			emitNode(out, indent+"  ", n.Content[i].Value, n.Content[i+1])
		}
	default:
		*out = append(*out, indent+key+":")
	}
}

func emitSeqItem(out *[]string, indent string, n *yaml.Node) {
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Tag != "!!str" {
			*out = append(*out, indent+"- "+n.Value)
			return
		}
		emitScalarItem(out, indent, n.Value)
	case yaml.MappingNode:
		first := true
		for i := 0; i+1 < len(n.Content); i += 2 {
			var sub []string
			emitNode(&sub, indent+"  ", n.Content[i].Value, n.Content[i+1])
			for j, l := range sub {
				if first && j == 0 {
					*out = append(*out, indent+"- "+strings.TrimPrefix(l, indent+"  "))
					continue
				}
				*out = append(*out, l)
			}
			first = false
		}
	case yaml.SequenceNode:
		var sub []string
		for _, item := range n.Content {
			emitSeqItem(&sub, indent+"  ", item)
		}
		for j, l := range sub {
			if j == 0 {
				*out = append(*out, indent+"- "+strings.TrimPrefix(l, indent+"  "))
				continue
			}
			*out = append(*out, l)
		}
	default:
		*out = append(*out, indent+"-")
	}
}

// emitObligation writes one obligation as a sequence item, keeping the authored
// order of its keys. guardOverride, when non-empty, replaces the WHEN value
// with the conjunction of a carrier's guard and this obligation's own.
func emitObligation(out *[]string, o *Obligation, guardOverride string) {
	if !o.Mapping {
		emitSeqItem(out, "", o.Node)
		return
	}
	var lines []string
	for _, e := range o.Entries {
		var sub []string
		if e.Key == "WHEN" && guardOverride != "" {
			emitScalarValue(&sub, "  ", e.Key, guardOverride)
		} else {
			emitNode(&sub, "  ", e.Key, e.Vn)
		}
		lines = append(lines, sub...)
	}
	for i, l := range lines {
		if i == 0 {
			*out = append(*out, "- "+strings.TrimPrefix(l, "  "))
			continue
		}
		*out = append(*out, l)
	}
}

// emitDocumentEntries writes every authored top-level entry of a document.
func emitDocumentEntries(out *[]string, d *Doc) {
	for _, e := range d.Entries {
		emitNode(out, "", e.Key, e.Vn)
	}
}
