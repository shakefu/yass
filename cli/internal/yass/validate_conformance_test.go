package yass

import (
	"strings"
	"testing"
)

const conformanceRoot = modeline + "---\ndescription: r\nversion: v1\n---\nspec: R\nINPUT:\n- MUST: x\n"

// fieldsOf splits each output record into its TAB-separated fields.
func fieldsOf(out string) [][]string {
	var rs [][]string
	for _, l := range lines(out) {
		rs = append(rs, strings.Split(l, "\t"))
	}
	return rs
}

// Diagnostics order by the file's root-relative path with its suffix:
// "a-b.yass.yaml" sorts before "a.yass.yaml" because '-' < '.', although the
// suffix-stripped ref "a" sorts before "a-b".
func TestValidateOrdersByFilePathWithSuffix(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", conformanceRoot)
	write(t, dir, "a.yass.yaml", modeline+"---\ndescription: a\nversion: v2\n")
	write(t, dir, "a-b.yass.yaml", modeline+"---\ndescription: a\nversion: v2\n")
	got := run(dir, "validate")
	wantStatus(t, got, ExitFindings)
	var locs []string
	for _, f := range fieldsOf(got.stdout) {
		locs = append(locs, f[2])
	}
	if strings.Join(locs, ",") != "a-b,a" {
		t.Fatalf("validate locations = %v, want [a-b a]\n%s", locs, got.stdout)
	}
}

func TestLintOrdersByFilePathWithSuffix(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", conformanceRoot)
	write(t, dir, "a.yass.yaml", "description: a\n")
	write(t, dir, "a-b.yass.yaml", "description: ab\n")
	got := run(dir, "lint")
	var locs []string
	for _, f := range fieldsOf(got.stdout) {
		if f[1] == "yass.lint.modeline" {
			locs = append(locs, f[2])
		}
	}
	if strings.Join(locs, ",") != "a-b,a" {
		t.Fatalf("lint modeline locations = %v, want [a-b a]\n%s", locs, got.stdout)
	}
}
