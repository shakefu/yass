package yass

import (
	"strings"
	"testing"
)

// lintRecords writes files into a fresh project directory, runs lint, and
// returns its output records split into their tab-separated fields.
func lintRecords(t *testing.T, files map[string]string, args ...string) [][]string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		write(t, dir, rel, body)
	}
	got := run(dir, append([]string{"lint"}, args...)...)
	var out [][]string
	for _, l := range lines(got.stdout) {
		out = append(out, strings.Split(l, "\t"))
	}
	return out
}

// withCode returns the records of one code as "location\tline".
func withCode(recs [][]string, code string) []string {
	var out []string
	for _, r := range recs {
		if len(r) == 5 && r[1] == code {
			out = append(out, r[2]+"\t"+r[3])
		}
	}
	return out
}

// C18: two checks reaching one finding on one line report it once.
func TestLintConformanceDedupesIdenticalAdvisories(t *testing.T) {
	recs := lintRecords(t, map[string]string{"root.yass.yaml": modeline + `spec: Root
INPUT: [{MUST: "exit Foo with status usage", USES: Codes}, {MUST: "exit Foo with status usage too", USES: Codes}]
---
design: Codes
type: catalog
content:
  Foo:
    status: ok
  Bar:
    status: usage
`})
	if got := withCode(recs, "yass.lint.restated_fact"); len(got) != 1 || got[0] != "root@Root::INPUT\t3" {
		t.Fatalf("restated_fact = %v, want one at root@Root::INPUT line 3", got)
	}
}

// C19: a file that fails to parse still gets the modeline advisory.
func TestLintConformanceModelineOnUnparsedFile(t *testing.T) {
	recs := lintRecords(t, map[string]string{
		"root.yass.yaml": modeline + "spec: Root\nRETURN:\n- MUST: go\n",
		"b.yass.yaml":    "spec: B\nINPUT: [ unclosed\n",
	})
	if got := withCode(recs, "yass.lint.unparsed"); len(got) != 1 || !strings.HasPrefix(got[0], "b\t") {
		t.Fatalf("unparsed = %v, want one naming b", got)
	}
	if got := withCode(recs, "yass.lint.modeline"); len(got) != 1 || got[0] != "b\t1" {
		t.Fatalf("modeline = %v, want one naming b at line 1", got)
	}
}

// C20: a period closing a code span, followed by a capital, breaks a clause.
func TestLintConformanceClauseAfterCodeSpan(t *testing.T) {
	recs := lintRecords(t, map[string]string{"root.yass.yaml": modeline + "spec: Root\nINPUT:\n- MUST: return `x`. Then fail\n- MUST: return x. Then fail\n- MUST: return `x`, e.g. Foo\n"}, "root.yass.yaml")
	got := withCode(recs, "yass.lint.obligation_shape")
	want := []string{"root@Root::INPUT\t4", "root@Root::INPUT\t5"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("obligation_shape = %v, want %v", got, want)
	}
}

// C21: a non-ASCII character that is not a letter or digit bounds a token.
func TestLintConformanceTokenBoundedByEmDash(t *testing.T) {
	recs := lintRecords(t, map[string]string{"root.yass.yaml": modeline + `spec: Root
RETURN:
- MUST: exit Foo—with status usage
  USES: Codes
- MUST: exit Fooé with status usage
  USES: Codes
- MUST: exit Foo with status usage
  USES: Codes
---
design: Codes
type: catalog
content:
  Foo:
    status: ok
  Bar:
    status: usage
`})
	got := withCode(recs, "yass.lint.restated_fact")
	want := []string{"root@Root::RETURN\t4", "root@Root::RETURN\t8"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("restated_fact = %v, want %v", got, want)
	}
}

// C22: a folded scalar's advisory names the file line the source is on.
func TestLintConformanceFoldedContentLine(t *testing.T) {
	recs := lintRecords(t, map[string]string{
		"root.yass.yaml": modeline + "spec: Root\nRETURN:\n- MUST: go\n  USES: a@Fold\n",
		"a.yass.yaml": modeline + `design: Fold
type: algorithm
content: >
  a

  b

  func main() {
---
design: Tab
type: constraint
content: >
  x

  a | b
  ---|---
`,
	})
	if got := withCode(recs, "yass.lint.design_code"); len(got) != 1 || got[0] != "a@Fold\t9" {
		t.Fatalf("design_code = %v, want one at a@Fold line 9", got)
	}
	if got := withCode(recs, "yass.lint.design_table"); len(got) != 1 || got[0] != "a@Tab\t16" {
		t.Fatalf("design_table = %v, want one at a@Tab line 16", got)
	}
}

// C22: a block scalar in a catalogue record maps lines without reading past
// its own indentation, and an empty block scalar reads nothing.
func TestLintConformanceRecordBlockLines(t *testing.T) {
	recs := lintRecords(t, map[string]string{
		"root.yass.yaml": modeline + "spec: Root\nRETURN:\n- MUST: go\n  USES: a@Cat\n",
		"a.yass.yaml": modeline + `design: Cat
type: catalog
content:
  Foo:
    note: |
    other: "a | b"
  Bar:
    note: >
      intro

      a | b
      ---|---
    other: |
      ---|---
`,
	})
	if got := withCode(recs, "yass.lint.design_table"); len(got) != 1 || got[0] != "a@Cat\t12" {
		t.Fatalf("design_table = %v, want one at a@Cat line 12", got)
	}
}

// C23: a name beginning with `function` is not the word `function`.
func TestLintConformanceFunctionIsAWord(t *testing.T) {
	if got := designLint(t, "algorithm", "functional(x) {"); len(got) != 0 {
		t.Fatalf("got %v, want no advisory", got)
	}
	if got := designLint(t, "algorithm", "function(x) {"); len(got) != 1 {
		t.Fatalf("anonymous function: got %v, want one advisory", got)
	}
}

// C24: pseudocode the MUST-NOT exempts is never reported, whatever source
// pattern it also matches.
func TestLintConformancePseudocodeExempt(t *testing.T) {
	for _, c := range []string{
		"for each item in items {",
		"} for each item in items {",
		"y := cache.get(k)",
		"x, err := os.Open(path)",
		"failed = err != nil",
		"if err != nil then",
		"while err != nil:",
		"check(err != nil)",
	} {
		if got := designLint(t, "algorithm", c); len(got) != 0 {
			t.Errorf("%q: got %v, want no advisory", c, got)
		}
	}
	for _, c := range []string{
		"for item in items {",
		"y := cache.Get(k, func() {",
		"m := map[string]int{}",
		"x := &pkg.Thing{}",
		"if err != nil {",
	} {
		if got := designLint(t, "algorithm", c); len(got) != 1 {
			t.Errorf("%q: got %v, want one advisory", c, got)
		}
	}
}

// C25: a nameless spec's USES reference binds its design.
func TestLintConformanceNamelessSpecUsesDesign(t *testing.T) {
	recs := lintRecords(t, map[string]string{
		"root.yass.yaml": modeline + "description: r\n---\nspec: Root\nRETURN:\n- MUST: go\n  SEE: a@Algo\n",
		"a.yass.yaml":    modeline + "description: a\n---\nspec: \"\"\nRETURN:\n- MUST: do it\n  USES: Algo\n---\ndesign: Algo\ntype: constraint\ncontent: x\n",
	})
	if got := withCode(recs, "yass.lint.design_unreferenced"); len(got) != 0 {
		t.Fatalf("design_unreferenced = %v, want none", got)
	}
}
