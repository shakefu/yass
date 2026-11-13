package yass

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- one physical file, one *File ------------------------------------------

const confRoot = modeline + "---\ndescription: r\nversion: v1\n---\nspec: R\nINPUT:\n- MUST: x\n"

// Every spelling of one file's path that target resolution accepts reaches the
// same loaded file, so incoming edges are found whichever spelling the target
// or the reference uses (02-load@Load::INPUT, 07-refs@Refs::RETURN).
func TestLoadOneFilePerPhysicalFileRefs(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", confRoot)
	write(t, dir, "sub/a.yass.yaml", modeline+`---
description: d
version: v1
---
spec: S
INPUT:
- MUST: a
---
spec: T
INPUT:
- MUST: b
  SEE: S
- MUST: c
  SEE: ./a@S
- MUST: d
  SEE: sub/a@S
`)
	write(t, dir, "b.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: U\nINPUT:\n- MUST: a\n  SEE: sub/a@S\n")
	want := "in\tSEE\tb@U|in\tSEE\tsub/a@T|in\tSEE\tsub/a@T|in\tSEE\tsub/a@T"
	for _, target := range []string{"sub/a@S", "./sub/a@S", "sub/./a@S", "sub/../sub/a@S"} {
		got := run(dir, "refs", target)
		wantStatus(t, got, ExitOK)
		if s := strings.Join(lines(got.stdout), "|"); s != want {
			t.Errorf("refs %s:\n got %s\nwant %s", target, s, want)
		}
	}
}

// From a subdirectory, a `../` target and a `./x/../` reference both reach the
// file the plain spelling names (07-refs@Refs::RETURN).
func TestLoadRefsIncomingAcrossSpellings(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: r\nversion: v1\n---\nspec: R\nINPUT:\n- MUST: y\n  SEE: u@U\n- MUST: z\n  SEE: ./x/../u@U\n")
	write(t, dir, "u.yass.yaml", modeline+"---\ndescription: u\nversion: v1\n---\nspec: U\nINPUT:\n- MUST: x\n")
	if err := os.Mkdir(filepath.Join(dir, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := "in\tSEE\troot@R|in\tSEE\troot@R"
	for _, c := range []struct{ dir, target string }{
		{dir, "u@U"}, {dir, "./u@U"}, {filepath.Join(dir, "x"), "../u@U"},
	} {
		got := run(c.dir, "refs", "--in", c.target)
		wantStatus(t, got, ExitOK)
		if s := strings.Join(lines(got.stdout), "|"); s != want {
			t.Errorf("refs %s from %s:\n got %s\nwant %s", c.target, c.dir, s, want)
		}
	}
}

// Two USES naming one design under different spellings of its file append the
// design exactly once (04-query@Resolution::RETURN).
func TestLoadQueryUsesDedupeAcrossSpellings(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "x.yass.yaml", modeline+"---\ndescription: x\nversion: v1\n---\ndesign: D\ntype: constraint\ncontent: d\n")
	write(t, dir, "sub/y.yass.yaml", modeline+"---\ndescription: y\nversion: v1\n---\ndesign: E\ntype: constraint\ncontent: e\n")
	write(t, dir, "root.yass.yaml", modeline+`---
description: root
version: v1
---
spec: A
RETURN:
- MUST: one
  USES: x@D
- MUST: two
  USES: ./x@D
- MUST: three
  USES: sub/y@E
- MUST: four
  USES: sub/../sub/y@E
`)
	if err := os.Symlink(filepath.Join(dir, "sub"), filepath.Join(dir, "link")); err == nil {
		write(t, dir, "root.yass.yaml", modeline+`---
description: root
version: v1
---
spec: A
RETURN:
- MUST: one
  USES: x@D
- MUST: two
  USES: ./x@D
- MUST: three
  USES: sub/y@E
- MUST: four
  USES: sub/../sub/y@E
- MUST: five
  USES: link/y@E
`)
	}
	got := run(dir, "query", "root@A")
	wantStatus(t, got, ExitOK)
	if n := strings.Count(got.stdout, "design: D\n"); n != 1 {
		t.Errorf("design D appended %d times:\n%s", n, got.stdout)
	}
	if n := strings.Count(got.stdout, "design: E\n"); n != 1 {
		t.Errorf("design E appended %d times:\n%s", n, got.stdout)
	}
}

// Two PATHs naming one file, through a symbolically linked prefix or a
// relative spelling, take it once under its root-relative REF
// (05-list@List::INPUT, List::RETURN).
func TestLoadListFilePathsDedupeAndRootRelative(t *testing.T) {
	dir := t.TempDir() // on macOS under /var, a link to /private/var
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: root\nversion: v1\n")
	write(t, dir, "sub/a.yass.yaml", modeline+"---\ndescription: a\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n")
	want := "sub/a\tfile\t1\ta|sub/a@A\tspec\tINPUT\t-"
	check := func(name string, got result) {
		t.Helper()
		wantStatus(t, got, ExitOK)
		if s := strings.Join(lines(got.stdout), "|"); s != want {
			t.Errorf("%s:\n got %s\nwant %s", name, s, want)
		}
	}
	check("relative and absolute", run(dir, "list", "./sub/a.yass.yaml", filepath.Join(dir, "sub", "a.yass.yaml")))
	if err := os.Symlink(filepath.Join(dir, "sub"), filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	check("linked prefix and directory", run(dir, "list", "link/a.yass.yaml", "sub"))
}

// --- YAML 1.2 core schema ---------------------------------------------------

// A plain scalar the core schema resolves to a string is a string, though
// go-yaml would read it as a YAML 1.1 timestamp, binary, or underscored
// number; core-schema ints, floats, bools, and nulls stay non-strings
// (02-load@Load::INPUT).
func TestLoadCoreSchemaPlainScalars(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", confRoot)
	strs := []string{"2001-12-14", "2001-12-14T10:00:00Z", "0b10", "1_000", "1:20", "<<", "0o9", "0xZ", "1e", "yes", "on"}
	body := modeline + "---\ndescription: d\nversion: v1\n---\nspec: S\nINPUT:\n"
	for _, v := range strs {
		body += "- MUST: " + v + "\n"
	}
	write(t, dir, "a.yass.yaml", body)
	got := run(dir, "validate", "a.yass.yaml")
	wantStatus(t, got, ExitOK)

	for _, v := range []string{"12", "-3", "0o17", "0x1F", "1.5", ".5", "1e3", "+.inf", ".NaN", "true", "FALSE", "null", "~"} {
		write(t, dir, "b.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: S\nINPUT:\n- MUST: "+v+"\n")
		got := run(dir, "validate", "b.yass.yaml")
		if !strings.Contains(got.stdout, "yass.obligation.not_scalar") {
			t.Errorf("%s must stay a non-string: status %d stdout %q", v, got.status, got.stdout)
		}
	}
}

// --- parse-failure line -----------------------------------------------------

// A parse failure go-yaml reports without a line still names the 1-based line
// the parser stopped on, never 0 (02-load@Load::RETURN).
func TestLoadParseFailureLineIsNeverZero(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", confRoot)
	write(t, dir, "c.yass.yaml", modeline+"description: d\n---\nspec: S\nINPUT:\n- MUST: a\x01b\n")
	write(t, dir, "al.yass.yaml", modeline+"description: d\n---\nspec: S\nINPUT:\n- MUST: ok\n- MUST: *x\n")
	for _, c := range []struct{ target, want string }{
		{"c", "error\tyass.document.not_yaml\tc\t6\t"},
		{"al", "error\tyass.document.not_yaml\tal\t7\t"},
	} {
		got := run(dir, "query", c.target)
		wantStatus(t, got, ExitUnresolved)
		if !strings.HasPrefix(got.stderr, c.want) {
			t.Errorf("query %s: stderr = %q, want prefix %q", c.target, got.stderr, c.want)
		}
	}
	// A failure whose line neither go-yaml nor the bytes name falls back to
	// the stream's last line.
	if n := errLine(errors.New("yaml: some failure"), []byte("a\nb\nc\n")); n != 3 {
		t.Errorf("fallback line = %d, want 3", n)
	}
	if n := errLine(errors.New("yaml: some failure"), nil); n != 1 {
		t.Errorf("empty stream line = %d, want 1", n)
	}
}
