package yass

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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
	case "null", "Null", "NULL", "~", "true", "True", "TRUE", "false", "False", "FALSE", "",
		"+.inf", "+.Inf", "+.INF", "<<":
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
	return resolvesToTimestamp(v)
}

// timestampFormats mirrors the layouts go-yaml resolves plain scalars against
// to the !!timestamp tag, so prose that would stop reading back as a string is
// quoted instead.
var timestampFormats = []string{
	"2006-1-2T15:4:5.999999999Z07:00",
	"2006-1-2t15:4:5.999999999Z07:00",
	"2006-1-2 15:4:5.999999999",
	"2006-1-2",
}

// resolvesToTimestamp mirrors go-yaml's plain-scalar timestamp resolution:
// four leading digits, a hyphen, then one of the layouts above.
func resolvesToTimestamp(v string) bool {
	i := 0
	for ; i < len(v); i++ {
		if c := v[i]; c < '0' || c > '9' {
			break
		}
	}
	if i != 4 || i == len(v) || v[i] != '-' {
		return false
	}
	for _, format := range timestampFormats {
		if _, err := time.Parse(format, v); err == nil {
			return true
		}
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

// cyclicAliasError stops emission when alias resolution re-enters an alias
// already being resolved: the value graph is cyclic and has no finite
// rendering.
type cyclicAliasError struct{ line int }

func (e *cyclicAliasError) Error() string { return "alias value graph is cyclic" }

// aliasDiagnostic turns the emission cycle guard's stop into the diagnostic
// validate already defines for a yaml alias, named for the target being
// emitted.
func aliasDiagnostic(target string, err error) error {
	if c, ok := err.(*cyclicAliasError); ok {
		return fail(ExitUnresolved, "yass.document.yaml_feature", target, c.line, c.Error())
	}
	return err
}

// emitNode renders an arbitrary node under a key, used for a preamble's or a
// design's authored entries. seen holds the alias nodes on the resolution
// path, so a cyclic anchor graph stops at a diagnostic instead of recursing
// forever.
func emitNode(out *[]string, indent, key string, n *yaml.Node, seen map[*yaml.Node]bool) error {
	if n == nil {
		*out = append(*out, indent+key+":")
		return nil
	}
	if n.Kind == yaml.AliasNode && n.Alias != nil {
		// An alias emits the value it names; the anchor's own spelling cannot
		// survive into a standalone fragment. seen is the path being resolved,
		// not every alias visited: an anchored subtree is one shared node
		// graph, so a second alias arm must re-walk its inner aliases.
		if seen[n] {
			return &cyclicAliasError{line: n.Line}
		}
		if seen == nil {
			seen = map[*yaml.Node]bool{}
		}
		seen[n] = true
		err := emitNode(out, indent, key, n.Alias, seen)
		delete(seen, n)
		return err
	}
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Tag == "!!null" {
			*out = append(*out, indent+key+":")
			return nil
		}
		if n.Tag != "!!str" {
			*out = append(*out, indent+key+": "+n.Value)
			return nil
		}
		emitScalarValue(out, indent, key, n.Value)
	case yaml.SequenceNode:
		*out = append(*out, indent+key+":")
		for _, item := range n.Content {
			if err := emitSeqItem(out, indent, item, seen); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		*out = append(*out, indent+key+":")
		for i := 0; i+1 < len(n.Content); i += 2 {
			if err := emitNode(out, indent+"  ", n.Content[i].Value, n.Content[i+1], seen); err != nil {
				return err
			}
		}
	default:
		*out = append(*out, indent+key+":")
	}
	return nil
}

func emitSeqItem(out *[]string, indent string, n *yaml.Node, seen map[*yaml.Node]bool) error {
	if n.Kind == yaml.AliasNode && n.Alias != nil {
		if seen[n] {
			return &cyclicAliasError{line: n.Line}
		}
		if seen == nil {
			seen = map[*yaml.Node]bool{}
		}
		seen[n] = true
		err := emitSeqItem(out, indent, n.Alias, seen)
		delete(seen, n)
		return err
	}
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Tag != "!!str" {
			*out = append(*out, indent+"- "+n.Value)
			return nil
		}
		emitScalarItem(out, indent, n.Value)
	case yaml.MappingNode:
		first := true
		for i := 0; i+1 < len(n.Content); i += 2 {
			var sub []string
			if err := emitNode(&sub, indent+"  ", n.Content[i].Value, n.Content[i+1], seen); err != nil {
				return err
			}
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
			if err := emitSeqItem(&sub, indent+"  ", item, seen); err != nil {
				return err
			}
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
	return nil
}

// obligationLines renders one obligation as sequence-item lines, keeping the
// authored order of its keys. guardOverride, when non-empty, replaces the WHEN
// value with the conjunction of a carrier's guard and this obligation's own.
func obligationLines(o *Obligation, guardOverride string) ([]string, error) {
	if !o.Mapping {
		var sub []string
		err := emitSeqItem(&sub, "", o.Node, nil)
		return sub, err
	}
	var lines []string
	for _, e := range o.Entries {
		var sub []string
		if e.Key == "WHEN" && guardOverride != "" {
			emitScalarValue(&sub, "  ", e.Key, guardOverride)
		} else {
			if err := emitNode(&sub, "  ", e.Key, e.Vn, nil); err != nil {
				return nil, err
			}
		}
		lines = append(lines, sub...)
	}
	if len(lines) > 0 {
		lines[0] = "- " + strings.TrimPrefix(lines[0], "  ")
	}
	return lines, nil
}

// emitDocumentEntries writes every authored top-level entry of a document.
func emitDocumentEntries(out *[]string, d *Doc) error {
	seen := map[*yaml.Node]bool{}
	for _, e := range d.Entries {
		if err := emitNode(out, "", e.Key, e.Vn, seen); err != nil {
			return err
		}
	}
	return nil
}
