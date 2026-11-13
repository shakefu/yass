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
// algorithm holds, so only shapes that are source in a real language match —
// never a bare assignment, a `for each`, or an unqualified call.

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
			lines := strings.Split(outsideCode(n.Value), "\n")
			for i, l := range lines {
				line := contentLine(n, i)
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
	for _, re := range designCode {
		if re.MatchString(l) {
			return true
		}
	}
	return false
}

// contentLine maps the i-th line of a scalar's value to the file line it was
// authored on: a block scalar's text starts on the line after its indicator,
// and any other scalar is reported at the line it starts on.
func contentLine(n *yaml.Node, i int) int {
	if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return n.Line + 1 + i
	}
	return n.Line
}
