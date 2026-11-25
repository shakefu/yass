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

// A PATH operand anchors root discovery, so a file named by PATH is answered
// from its own ascent: no root at or above it ends the request with
// yass.root.not_found naming the operand, before any file is checked. A
// directory on that ascent that cannot be read leaves the question undecided,
// which ends the request with yass.io.unreadable naming the directory: it is
// neither the yass.root.not_found answer nor a pass
// (01-locate@RootDiscovery::INPUT, RootDiscovery::ERROR,
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

	// With every directory readable the unrooted operand has no project root
	// at or above it, and the request ends before any file is checked.
	got := run(proj, "validate", "../anc/orphan.yass.yaml")
	wantStatus(t, got, ExitNoRoot)
	if got.stdout != "" || !strings.Contains(got.stderr, "yass.root.not_found\t../anc/orphan.yass.yaml") {
		t.Fatalf("stdout = %q, stderr = %q, want yass.root.not_found naming the operand", got.stdout, got.stderr)
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

// Root discovery anchors on a PATH operand, so a file under a directory that
// grants read but no search has an ascent that cannot decide whether a root
// file lies at or above it: the request ends with yass.io.unreadable naming
// that directory, as it does for `root` taking the same operand. With no PATH
// the ascent starts at the project root and never examines the directory; the
// traversal that reaches the file reports it relative to the root
// (01-locate@RootDiscovery::INPUT, RootDiscovery::ERROR,
// 01-locate@ReadableDirectory, 01-locate@FileSet::ERROR).
func TestValidateFileOperandUnderUnlistableDirectory(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", conformanceRoot)
	write(t, dir, "locked/x.yass.yaml", modeline+"---\ndescription: x\nversion: v1\n---\nspec: X\nINPUT:\n- MUST: x\n  SEE: nowhere@Gone\n")
	locked := filepath.Join(dir, "locked")
	denyDir(t, locked, 0o111)
	want := realpath(t, locked)
	if loc := unreadableLocation(t, run(dir, "validate", "locked/x.yass.yaml")); loc != want {
		t.Fatalf("validate location = %q, want %q", loc, want)
	}
	if loc := unreadableLocation(t, run(dir, "list", "locked/x.yass.yaml")); loc != want {
		t.Fatalf("list location = %q, want %q", loc, want)
	}
	// With no PATH the ascent starts at the project root, and the traversal
	// reports the directory relative to it.
	if loc := unreadableLocation(t, run(dir, "validate")); loc != "locked" {
		t.Fatalf("location = %q, want %q", loc, "locked")
	}
}

// Root discovery anchors on the first PATH operand, so the project the caller
// named governs the request whatever directory the process runs in: a working
// directory inside no project no longer reports yass.root.not_found for a
// PATH that names one, and a working directory inside one project no longer
// answers for another tree with its own green silence
// (01-locate@RootDiscovery::INPUT, 08-validate@Validate::INPUT).
func TestValidateAnchorsDiscoveryOnFirstPathOperand(t *testing.T) {
	base := realDir(t, "anchor")
	green := filepath.Join(base, "green")
	write(t, green, "root.yass.yaml", conformanceRoot)
	broken := filepath.Join(base, "broken")
	write(t, broken, "root.yass.yaml", conformanceRoot)
	bad := write(t, broken, "b.yass.yaml", modeline+"---\ndescription: b\nversion: v2\n")
	neutral := filepath.Join(base, "neutral")
	if err := os.Mkdir(neutral, 0o755); err != nil {
		t.Fatal(err)
	}

	// From a working directory inside no project, a PATH naming a project
	// validates that project.
	t.Chdir(neutral)
	if got := mainIn("validate", green); got.status != ExitOK || got.stdout != "" || got.stderr != "" {
		t.Fatalf("from a neutral directory: status %d\nstdout:\n%s\nstderr:\n%s", got.status, got.stdout, got.stderr)
	}

	// From the same neutral directory, a PATH naming a file of another
	// project reports that file's own finding.
	got := mainIn("validate", bad)
	wantStatus(t, got, ExitFindings)
	if !strings.Contains(got.stdout, "yass.preamble.version\tb") || got.stderr != "" {
		t.Fatalf("stdout = %q, stderr = %q, want b's finding", got.stdout, got.stderr)
	}

	// From a working directory inside the green project, a PATH naming the
	// broken project's file is answered for the tree the operand names, so
	// the finding is reported rather than the working tree's green silence.
	t.Chdir(green)
	got = mainIn("validate", bad)
	wantStatus(t, got, ExitFindings)
	if !strings.Contains(got.stdout, "yass.preamble.version\tb") || got.stderr != "" {
		t.Fatalf("stdout = %q, stderr = %q, want b's finding", got.stdout, got.stderr)
	}

	// A relative PATH still resolves against the starting directory, and
	// anchors discovery on the file it names; the finding names the file
	// relative to the root it is checked against.
	if got := run(green, "validate", "../broken/b.yass.yaml"); got.status != ExitFindings ||
		!strings.Contains(got.stdout, "yass.preamble.version\tb") || got.stderr != "" {
		t.Fatalf("stdout = %q, stderr = %q, want b's finding", got.stdout, got.stderr)
	}
}

// A second PATH naming another project's file ends the request with
// yass.root.outside before any file is checked: one invocation answers for
// one project, and a second tree's files would be read against the wrong
// root's model (08-validate@Validate::ERROR, root@ExitPolicy).
func TestValidateSecondOperandOutsideProjectIsRefused(t *testing.T) {
	base := realDir(t, "refuse")
	a := filepath.Join(base, "a")
	write(t, a, "root.yass.yaml", conformanceRoot)
	write(t, a, "x.yass.yaml", modeline+"---\ndescription: x\nversion: v2\n")
	b := filepath.Join(base, "b")
	write(t, b, "root.yass.yaml", conformanceRoot)
	write(t, b, "y.yass.yaml", modeline+"---\ndescription: y\nversion: v2\n")

	// The first operand anchors the project; the second lies outside it, and
	// the request ends even though the first operand's own tree holds a
	// finding worth reporting.
	got := run(a, "validate", "root.yass.yaml", filepath.Join(b, "y.yass.yaml"))
	wantStatus(t, got, ExitNoRoot)
	if got.stdout != "" || !strings.Contains(got.stderr, "yass.root.outside\t"+filepath.Join(b, "y.yass.yaml")) {
		t.Fatalf("stdout = %q, stderr = %q, want yass.root.outside naming the operand", got.stdout, got.stderr)
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
