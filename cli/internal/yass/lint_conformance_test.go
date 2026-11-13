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
