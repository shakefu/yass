package yass

import (
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// The design-content advisories of 09-lint@Lint::RETURN match fixed character
// patterns line by line, skipping backtick code spans as every prose pattern
// does. The code patterns are deliberately narrow: pseudocode is what an
// algorithm holds, so only shapes that are source in a real language match,
// and a line of pseudocode never does: a `for each` loop, a bare assignment,
// a guard closed by `then` or a colon, or a call that ends no statement.

var (
	// designCode matches one line of source in a real programming language.
	designCode = []*regexp.Regexp{
		// A function definition: Go, Python, JavaScript, Rust.
		regexp.MustCompile(`^\s*func\s+(\([^)]*\)\s*)?\w+\s*\(`),
		regexp.MustCompile(`^\s*(async\s+)?def\s+\w+\s*\(.*\)\s*(->\s*[^:]+)?:\s*$`),
		regexp.MustCompile(`^\s*(export\s+)?(async\s+)?function\b\s*\w*\s*\(.*\)\s*\{`),
		regexp.MustCompile(`^\s*(pub\s+)?fn\s+\w+\s*[<(]`),
		// A statement terminated by a semicolon: an assignment or a call.
		regexp.MustCompile(`^\s*[A-Za-z_][\w.\[\]*]*\s*(=|:=|\+=|-=)\s*[^=\s].*;\s*$`),
		regexp.MustCompile(`^\s*[A-Za-z_][\w.]*\(.*\)\s*;\s*$`),
		// A Go declaration assigned from a qualified call or a composite literal.
		regexp.MustCompile(`\w\s*:=\s*&?[A-Za-z_]\w*\.[A-Za-z_][\w.]*[({]`),
		// A Go composite literal of a map, slice, or address-of type.
		regexp.MustCompile(`\bmap\[[\w.*]+\][\w.*\[\]]+\{|\[\]\*?[A-Za-z_][\w.]*\{|&[A-Za-z_][\w.]*\{`),
		// Control flow opening a brace block, or the error check of Go.
		regexp.MustCompile(`^\s*(\}\s*)?(if|else if|for|while|switch)\b.*\{\s*$`),
		regexp.MustCompile(`^\s*\}\s*else\s*\{`),
		regexp.MustCompile(`\berr\s*!=\s*nil\b`),
		// An import or a package clause.
		regexp.MustCompile(`^\s*(import\s+(\(|"|[\w.]+;)|from\s+[\w.]+\s+import\s+\w|#include\s*[<"]|package\s+[\w.]+;?\s*$|use\s+[\w:]+;)`),
	}
	// pseudocode matches the lines of pseudocode SourcePatterns exempts,
	// whatever code pattern they also match: a `for each` loop, a bare
	// assignment, a guard closed by `then` or a colon, and a call that ends
	// no statement.
	pseudocode = []*regexp.Regexp{
		regexp.MustCompile(`^\s*(\}\s*)?for\s+each\b`),
		regexp.MustCompile(`^\s*[A-Za-z_][\w.\[\]*]*(\s*,\s*[A-Za-z_][\w.\[\]*]*)*\s*(=|:=|\+=|-=)\s*([^=\s{};]|[^=\s{}][^{}]*[^{};\s])\s*$`),
		regexp.MustCompile(`^\s*(if|else\s+if|elif|while)\b.*(\bthen|:)\s*$`),
		regexp.MustCompile(`^\s*[A-Za-z_][\w.]*\([^{}]*\)\s*$`),
	}
	// designTableRow matches a Markdown table row: a line holding a pipe.
	designTableRow = regexp.MustCompile(`\|`)
	// designTableRule matches a Markdown table delimiter row.
	designTableRule = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)+\|?\s*$|^\s*\|\s*:?-+:?\s*\|\s*$`)
)

// designAdvisories returns the design-code and design-table advisories of one
// parsed file, at most one of a given code at a given line.
func designAdvisories(f *File) []Diag {
	var out []Diag
	seen := map[string]bool{}
	warn := func(code, loc string, line int, msg string) {
		key := code + "\x00" + strconv.Itoa(line)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, Diag{Severity: "warn", Code: code, Location: loc, Line: line, Message: msg})
	}
	for _, d := range f.Docs {
		if d.Kind != KindDesign || !d.Addressable() {
			continue
		}
		var texts []*yaml.Node
		if d.HasContent {
			texts = append(texts, d.value("content"))
		}
		for _, r := range d.Catalog {
			for _, fl := range r.Fields {
				if fl.Vn.Kind == yaml.ScalarNode {
					texts = append(texts, fl.Vn)
				}
			}
		}
		codeTyped := d.HasType && (d.Type == "schema" || d.Type == "algorithm")
		for _, n := range texts {
			first, raw := scalarLines(f, n)
			lines := strings.Split(outsideCode(strings.Join(raw, "\n")), "\n")
			for i, l := range lines {
				line := first + i
				if i+1 < len(lines) && designTableRow.MatchString(l) && designTableRule.MatchString(lines[i+1]) {
					warn("yass.lint.design_table", d.Target(), line, "design content holds a markdown table")
				}
				if codeTyped && n == d.value("content") && isSourceLine(l) {
					warn("yass.lint.design_code", d.Target(), line, "design content holds source code")
				}
			}
		}
	}
	return out
}

// isSourceLine reports whether a line of design content has the shape of
// source in a real programming language.
func isSourceLine(l string) bool {
	for _, re := range pseudocode {
		if re.MatchString(l) {
			return false
		}
	}
	for _, re := range designCode {
		if re.MatchString(l) {
			return true
		}
	}
	return false
}

// scalarLines returns the file line a scalar's text starts on and its lines as
// authored. A block scalar's lines are read from the file itself, from the
// line after its indicator to the last line indented at least as deeply as its
// first non-blank line, with that indentation removed, so a folded scalar's
// lines keep the file lines they were written on. Any other scalar, or a block
// scalar with no line indented deeper than its indicator's, is its value,
// reported at the line it starts on.
func scalarLines(f *File, n *yaml.Node) (int, []string) {
	if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) == 0 || n.Line < 1 || n.Line > len(f.Lines) {
		return n.Line, strings.Split(n.Value, "\n")
	}
	head := f.Lines[n.Line-1]
	base := len(head) - len(strings.TrimLeft(head, " "))
	indent := -1
	var raw []string
	for _, l := range f.Lines[n.Line:] {
		trimmed := strings.TrimLeft(l, " ")
		if trimmed == "" {
			raw = append(raw, "")
			continue
		}
		depth := len(l) - len(trimmed)
		if indent < 0 {
			indent = depth
		}
		if depth < indent {
			break
		}
		raw = append(raw, l[indent:])
	}
	if indent <= base {
		return n.Line, strings.Split(n.Value, "\n")
	}
	return n.Line + 1, raw
}
