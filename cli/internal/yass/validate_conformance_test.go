package yass

import (
	"os"
	"path/filepath"
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

// A referenced file that exists but cannot be read ends the request with
// yass.io.unreadable; it is not an unresolved reference.
func TestValidateUnreadableReferencedFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; file permissions are not enforced")
	}
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", conformanceRoot+"  SEE: u@U\n")
	u := write(t, dir, "u.yass.yaml", modeline+"---\ndescription: u\nversion: v1\n---\nspec: U\nINPUT:\n- MUST: x\n")
	if err := os.Chmod(u, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(u, 0o644) //nolint:errcheck
	got := run(dir, "validate", "root.yass.yaml")
	wantStatus(t, got, ExitEnvironment)
	if got.stdout != "" || !strings.Contains(got.stderr, "yass.io.unreadable\tu.yass.yaml") {
		t.Fatalf("stdout = %q, stderr = %q, want only yass.io.unreadable naming u.yass.yaml", got.stdout, got.stderr)
	}
}

// An addressed file outside the governing project root is checked for a root
// file at or above it. A directory on that ascent that cannot be read leaves
// the question undecided, which ends the request with yass.io.unreadable
// naming the directory: it is neither the yass.root.not_found finding nor a
// pass (08-validate@Validate::RETURN, Validate::ERROR,
// 01-locate@ReadableDirectory).
func TestValidateOutsideFileUnreadableAscent(t *testing.T) {
	const orphan = modeline + "---\ndescription: o\nversion: v1\n---\nspec: O\nINPUT:\n- MUST: x\n"
	base := realDir(t, "base")
	proj := filepath.Join(base, "proj")
	write(t, proj, "root.yass.yaml", conformanceRoot)
	anc := filepath.Join(base, "anc")
	write(t, anc, "orphan.yass.yaml", orphan)
	write(t, anc, "rooted/root.yass.yaml", conformanceRoot)
	write(t, anc, "rooted/x.yass.yaml", orphan)

	// With every directory readable the unrooted file is a finding.
	got := run(proj, "validate", "../anc/orphan.yass.yaml")
	wantStatus(t, got, ExitFindings)
	if !strings.Contains(got.stdout, "yass.root.not_found\t../anc/orphan") || got.stderr != "" {
		t.Fatalf("stdout = %q, stderr = %q, want yass.root.not_found for the file", got.stdout, got.stderr)
	}

	denyDir(t, anc, 0o111)
	if loc := unreadableLocation(t, run(proj, "validate", "../anc/orphan.yass.yaml")); loc != anc {
		t.Fatalf("location = %q, want %q", loc, anc)
	}
	// The ascent stops at the nearest root, below the directory that cannot
	// be read.
	got = run(proj, "validate", "../anc/rooted/x.yass.yaml")
	if got.status != ExitOK || got.stdout != "" || got.stderr != "" {
		t.Fatalf("a file under its own root: status %d\nstdout:\n%s\nstderr:\n%s", got.status, got.stdout, got.stderr)
	}
}

// A file addressed by name under the governing project root has that root at
// or above it, so no directory between the two is examined for one: a
// directory that grants search but no listing still lets the file be read and
// checked, as it does for every other command taking the same operand
// (08-validate@Validate::INPUT, Validate::RETURN).
func TestValidateFileOperandUnderUnlistableDirectory(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", conformanceRoot)
	write(t, dir, "locked/x.yass.yaml", modeline+"---\ndescription: x\nversion: v1\n---\nspec: X\nINPUT:\n- MUST: x\n  SEE: nowhere@Gone\n")
	denyDir(t, filepath.Join(dir, "locked"), 0o111)
	got := run(dir, "validate", "locked/x.yass.yaml")
	wantStatus(t, got, ExitFindings)
	if !strings.Contains(got.stdout, "yass.ref.unresolved\tlocked/x@X") || got.stderr != "" {
		t.Fatalf("stdout = %q, stderr = %q, want the file checked", got.stdout, got.stderr)
	}
	if got := run(dir, "list", "locked/x.yass.yaml"); got.status != ExitOK || got.stderr != "" {
		t.Fatalf("list the same operand: status %d, stderr %q", got.status, got.stderr)
	}
	// The traversal does list the directory, so it reports it.
	if loc := unreadableLocation(t, run(dir, "validate")); loc != "locked" {
		t.Fatalf("location = %q, want %q", loc, "locked")
	}
}

// A document carrying both keys is claimed by neither classification: it is
// not counted as the root's spec, and a reference to its name does not
// resolve.
func TestValidateBothKeysDocumentIsNeitherSpecNorDesign(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", conformanceRoot+"---\nspec: B\ndesign: B\n")
	got := run(dir, "validate")
	wantStatus(t, got, ExitFindings)
	codes := map[string]bool{}
	for _, f := range fieldsOf(got.stdout) {
		codes[f[1]] = true
	}
	if !codes["yass.document.both_keys"] || codes["yass.root.spec_count"] {
		t.Fatalf("codes = %v, want both_keys without root.spec_count\n%s", codes, got.stdout)
	}

	dir = t.TempDir()
	write(t, dir, "root.yass.yaml", conformanceRoot+"  USES: b@B\n- MUST: y\n  CONFORMS: b@B\n")
	write(t, dir, "b.yass.yaml", modeline+"---\ndescription: b\nversion: v1\n---\nspec: B\ndesign: B\n")
	got = run(dir, "validate", "root.yass.yaml")
	wantStatus(t, got, ExitFindings)
	var refCodes []string
	for _, f := range fieldsOf(got.stdout) {
		refCodes = append(refCodes, f[1])
	}
	if strings.Join(refCodes, ",") != "yass.ref.unresolved,yass.ref.unresolved" {
		t.Fatalf("codes = %v, want two yass.ref.unresolved\n%s", refCodes, got.stdout)
	}
}

// Only scalar keys with the same resolved tag and value are duplicates:
// distinct complex keys and an int key beside a string key are not.
func TestValidateDuplicateKeyComparesTagAndValue(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", conformanceRoot)
	write(t, dir, "d.yass.yaml", modeline+`---
description: d
version: v1
---
design: C
type: catalog
content:
  ? [x]
  : {f: c}
  ? [y]
  : {f: d}
---
design: E
type: catalog
content:
  1: {f: c}
  "1": {f: d}
  x: {f: e}
  x: {f: g}
`)
	got := run(dir, "validate", "d.yass.yaml")
	wantStatus(t, got, ExitFindings)
	var dups []string
	for _, f := range fieldsOf(got.stdout) {
		if f[1] == "yass.document.duplicate_key" {
			dups = append(dups, f[2]+":"+f[3])
		}
	}
	if strings.Join(dups, ",") != "d@E:20" {
		t.Fatalf("duplicate_key = %v, want only d@E:20\n%s", dups, got.stdout)
	}
}

// The non-specific tag property `!` is an explicit tag; a `!` inside a
// scalar is not, and a tag on a block mapping's first key is reported once.
func TestValidateNonSpecificTagIsYamlFeature(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+`---
description: r
version: v1
---
spec: R
INPUT:
- MUST: ! v
- MUST: !foo v
- MUST: !<tag:yaml.org,2002:str> v
- MUST: "not ! a tag"
- MUST: plain ! text
- ! MUST: y
- MUST: ! [z]
`)
	got := run(dir, "validate")
	wantStatus(t, got, ExitFindings)
	var ls []string
	for _, f := range fieldsOf(got.stdout) {
		if f[1] == "yass.document.yaml_feature" {
			ls = append(ls, f[3])
		}
	}
	if strings.Join(ls, ",") != "8,9,10,13,14" {
		t.Fatalf("yaml_feature lines = %v, want [8 9 10 13 14]\n%s", ls, got.stdout)
	}
}
