package yass

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

type result struct {
	stdout string
	stderr string
	status int
}

// run invokes the program with dir as its starting directory.
func run(dir string, args ...string) result {
	var o, e bytes.Buffer
	full := append([]string{"-C", dir}, args...)
	st := Main(full, &o, &e)
	return result{o.String(), e.String(), st}
}

func write(t *testing.T, dir, rel, body string) string {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const modeline = Modeline + "\n"

// project builds a small, well-formed spec set.
func project(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+`---
description: the project
version: v1
---
spec: Proj
INPUT:
- MUST: accept a thing
  SEE: a@Alpha
RETURN:
- MUST: yield a thing
  USES: Policy
INVARIANT:
- USES: Policy
---
design: Policy
type: constraint
content: |
  one
  two
`)
	write(t, dir, "a.yass.yaml", modeline+`---
description: the alpha file
version: v1
---
spec: Alpha
INPUT:
- CONFORMS: Beta::RETURN
- WHEN: it is raining
  MUST: open the umbrella
  CONFORMS: sub/b@Beta::RETURN
RETURN:
- MUST: "yield: a value"
  USES: root@Policy
- MAY: also yield nothing
  USES: root@Policy
---
spec: Beta
RETURN:
- MUST: give back the goods
- WHEN: it is cold
  MUST: give back a coat
---
design: Note
type: constraint
content: |
  a note
`)
	write(t, dir, "sub/b.yass.yaml", modeline+`---
description: the b file
version: v1
---
spec: Beta
RETURN:
- MUST: return b
- WHEN: it is cold
  MUST: return a coat
`)
	// A nested project root takes over its own subtree.
	write(t, dir, "sub/nested/root.yass.yaml", modeline+`---
description: nested
version: v1
---
spec: Nested
INVARIANT:
- MUST: stay out of the outer file set
`)
	write(t, dir, "sub/nested/x.yass.yaml", modeline+`---
description: nested x
version: v1
---
spec: X
INPUT:
- MUST: never be listed by the outer project
`)
	return dir
}

func wantStatus(t *testing.T, got result, want int) {
	t.Helper()
	if got.status != want {
		t.Fatalf("status = %d, want %d\nstdout:\n%s\nstderr:\n%s", got.status, want, got.stdout, got.stderr)
	}
}

func lines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// --- invocation -------------------------------------------------------------

// An argument vector naming no subcommand is well formed: it yields the default
// subcommand and is answered with the orientation block, never a diagnostic.
func TestNoSubcommandRunsOverview(t *testing.T) {
	dir := project(t)
	bare := run(dir)
	if bare.status != ExitOK {
		t.Fatalf("status = %d, want %d", bare.status, ExitOK)
	}
	if bare.stderr != "" {
		t.Fatalf("stderr not empty: %q", bare.stderr)
	}
	if !strings.Contains(bare.stdout, "Yet Another Spec Syntax") {
		t.Fatalf("stdout is not the orientation block: %q", bare.stdout)
	}
	if named := run(dir, "overview"); named.stdout != bare.stdout || named.status != bare.status {
		t.Fatalf("`yass` and `yass overview` differ:\n%q\n%q", bare.stdout, named.stdout)
	}
}

func TestArgumentErrors(t *testing.T) {
	dir := project(t)
	cases := []struct {
		args []string
		code string
	}{
		{[]string{"nope"}, "yass.args.unknown_subcommand"},
		{[]string{"ROOT"}, "yass.args.unknown_subcommand"},
		{[]string{"ro"}, "yass.args.unknown_subcommand"},
		{[]string{"list", "--bogus"}, "yass.args.unknown_option"},
		{[]string{"list", "--filter"}, "yass.args.missing_value"},
		{[]string{"query"}, "yass.args.missing_operand"},
		{[]string{"root", "a", "b"}, "yass.args.extra_operand"},
		{[]string{"find", "a", "b"}, "yass.args.extra_operand"},
		{[]string{"list", ""}, "yass.args.empty"},
		{[]string{"find", ""}, "yass.args.empty"},
		{[]string{"query", "--raw=yes", "a@Alpha"}, "yass.args.unknown_option"},
		{[]string{"-"}, "yass.args.unknown_option"},
		{[]string{"root", "-"}, "yass.args.unknown_option"},
		{[]string{"query", "-"}, "yass.args.unknown_option"},
	}
	for _, c := range cases {
		got := run(dir, c.args...)
		wantStatus(t, got, ExitUsage)
		if !strings.Contains(got.stderr, c.code) {
			t.Errorf("%v: stderr = %q, want code %s", c.args, got.stderr, c.code)
		}
		if got.stdout != "" {
			t.Errorf("%v: stdout should be empty, got %q", c.args, got.stdout)
		}
	}
}

func TestSubcommandOptionBeforeSubcommand(t *testing.T) {
	dir := project(t)
	got := run(dir, "--raw", "query", "a@Alpha")
	wantStatus(t, got, ExitUsage)
	if !strings.Contains(got.stderr, "yass.args.unknown_option") {
		t.Fatalf("stderr = %q", got.stderr)
	}
}

func TestHelpAndVersion(t *testing.T) {
	dir := project(t)
	h := run(dir, "--help")
	wantStatus(t, h, ExitOK)
	if !strings.HasPrefix(h.stdout, "usage: yass ") {
		t.Fatalf("help does not open with the synopsis line: %q", h.stdout)
	}
	// Help takes precedence over every other option and over the subcommand.
	h2 := run(dir, "--help", "--version")
	if h2.stdout != h.stdout {
		t.Fatalf("--help did not take precedence over --version")
	}
	v := run(dir, "--version")
	wantStatus(t, v, ExitOK)
	if v.stdout != programName+" "+programVersion+" "+languageVersion+"\n" {
		t.Fatalf("version line = %q", v.stdout)
	}
	// A subcommand's own help is not offered in v1.
	ph := run(dir, "root", "--help")
	wantStatus(t, ph, ExitUsage)
}

func TestEndOfOptions(t *testing.T) {
	dir := project(t)
	got := run(dir, "--", "root")
	wantStatus(t, got, ExitOK)
	if strings.TrimSpace(got.stdout) == "" {
		t.Fatalf("expected a root record, got %q", got.stdout)
	}
}

func TestLastOccurrenceOfGlobalValueWins(t *testing.T) {
	dir := project(t)
	other := t.TempDir()
	var o, e bytes.Buffer
	st := Main([]string{"-C", other, "-C", dir, "root"}, &o, &e)
	if st != ExitOK {
		t.Fatalf("status = %d stderr=%s", st, e.String())
	}
	if strings.TrimSuffix(o.String(), "\n") != realpath(t, dir) {
		t.Fatalf("root = %q, want %q", o.String(), dir)
	}
}

func realpath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// The `-C=DIR` and `--slot=SLOT` equals spellings select the same value as the
// spaced ones.
func TestOptionEqualsValueForms(t *testing.T) {
	dir := project(t)
	var o, e bytes.Buffer
	st := Main([]string{"-C=" + dir, "find", "--slot=RETURN", "goods"}, &o, &e)
	wantStatus(t, result{o.String(), e.String(), st}, ExitOK)
	if !strings.Contains(o.String(), "a@Beta") {
		t.Fatalf("stdout = %q, want the RETURN obligation of a@Beta", o.String())
	}
}

// An argument that is not valid UTF-8 rejects the vector rather than being
// silently replaced.
func TestNonUTF8ArgumentRejected(t *testing.T) {
	dir := project(t)
	var o, e bytes.Buffer
	st := Main([]string{"-C", dir, "find", string([]byte{0xff, 0xfe})}, &o, &e)
	wantStatus(t, result{o.String(), e.String(), st}, ExitUsage)
	if !strings.Contains(e.String(), "yass.args.unknown_option") {
		t.Fatalf("stderr = %q", e.String())
	}
}

// A starting path that names a file begins at the directory holding it, so
// operands and `./` target tokens resolve against that directory.
func TestStartDirectoryFromCFile(t *testing.T) {
	dir := project(t)
	rootFile := filepath.Join(dir, "root.yass.yaml")
	var o, e bytes.Buffer
	st := Main([]string{"-C", rootFile, "list", "sub"}, &o, &e)
	if st != ExitOK || len(lines(o.String())) != 2 {
		t.Fatalf("list sub via -C FILE: status %d\nstdout:\n%s\nstderr:\n%s", st, o.String(), e.String())
	}
	o.Reset()
	e.Reset()
	st = Main([]string{"-C", rootFile, "query", "./sub/b@Beta"}, &o, &e)
	wantStatus(t, result{o.String(), e.String(), st}, ExitOK)
	if !strings.Contains(o.String(), "return b") {
		t.Fatalf("stdout = %q, want the resolved Beta obligation", o.String())
	}
	o.Reset()
	e.Reset()
	st = Main([]string{"-C", rootFile, "root", "sub"}, &o, &e)
	if st != ExitOK || strings.TrimSuffix(o.String(), "\n") != realpath(t, dir) {
		t.Fatalf("root sub via -C FILE: status %d, root %q", st, o.String())
	}
}

// --- root -------------------------------------------------------------------

func TestRootCommand(t *testing.T) {
	dir := project(t)
	got := run(dir, "root")
	wantStatus(t, got, ExitOK)
	if strings.TrimSuffix(got.stdout, "\n") != realpath(t, dir) {
		t.Fatalf("root = %q", got.stdout)
	}
	// A nested project root wins over an outer one.
	nested := run(dir, "root", "sub/nested/x.yass.yaml")
	wantStatus(t, nested, ExitOK)
	if strings.TrimSuffix(nested.stdout, "\n") != filepath.Join(realpath(t, dir), "sub", "nested") {
		t.Fatalf("nested root = %q", nested.stdout)
	}
	// Exactly one record, terminated by exactly one LF.
	if strings.Count(got.stdout, "\n") != 1 {
		t.Fatalf("expected exactly one record: %q", got.stdout)
	}
}

func TestRootNotFound(t *testing.T) {
	dir := t.TempDir()
	got := run(dir, "root")
	wantStatus(t, got, ExitNoRoot)
	if !strings.Contains(got.stderr, "yass.root.not_found") {
		t.Fatalf("stderr = %q", got.stderr)
	}
	if got.stdout != "" {
		t.Fatalf("stdout should be empty")
	}
}

func TestRootMissingPath(t *testing.T) {
	dir := project(t)
	got := run(dir, "root", "no/such/path")
	wantStatus(t, got, ExitUnresolved)
	if !strings.Contains(got.stderr, "yass.io.missing") {
		t.Fatalf("stderr = %q", got.stderr)
	}
}

// --- list -------------------------------------------------------------------

func TestListExcludesNestedProject(t *testing.T) {
	dir := project(t)
	got := run(dir, "list")
	wantStatus(t, got, ExitOK)
	for _, l := range lines(got.stdout) {
		if strings.Contains(l, "nested") {
			t.Fatalf("nested project leaked into the file set: %s", l)
		}
	}
	want := []string{
		"a\tfile\t3\tthe alpha file",
		"a@Alpha\tspec\tINPUT,RETURN\t-",
		"a@Beta\tspec\tRETURN\t-",
		"a@Note\tdesign\tconstraint\t-",
		"root\tfile\t2\tthe project",
		"root@Proj\tspec\tINPUT,RETURN,INVARIANT\t-",
		"root@Policy\tdesign\tconstraint\t-",
		"sub/b\tfile\t1\tthe b file",
		"sub/b@Beta\tspec\tRETURN\t-",
	}
	got2 := lines(got.stdout)
	if len(got2) != len(want) {
		t.Fatalf("record count = %d, want %d:\n%s", len(got2), len(want), got.stdout)
	}
	for i := range want {
		if got2[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got2[i], want[i])
		}
	}
}

func TestListFilterKeepsFileRecord(t *testing.T) {
	dir := project(t)
	got := run(dir, "list", "--filter", "*@Alpha")
	wantStatus(t, got, ExitOK)
	want := []string{"a\tfile\t3\tthe alpha file", "a@Alpha\tspec\tINPUT,RETURN\t-"}
	if strings.Join(lines(got.stdout), "|") != strings.Join(want, "|") {
		t.Fatalf("got:\n%s", got.stdout)
	}
}

func TestListFilterUnion(t *testing.T) {
	dir := project(t)
	got := run(dir, "list", "--filter", "*@Alpha", "--filter", "*@Policy")
	wantStatus(t, got, ExitOK)
	if len(lines(got.stdout)) != 4 {
		t.Fatalf("got:\n%s", got.stdout)
	}
}

func TestListFilterStarCrossesSlash(t *testing.T) {
	dir := project(t)
	got := run(dir, "list", "--filter", "s*b")
	wantStatus(t, got, ExitOK)
	if len(lines(got.stdout)) != 1 || lines(got.stdout)[0] != "sub/b\tfile\t1\tthe b file" {
		t.Fatalf("`*` must cross `/`; got:\n%s", got.stdout)
	}
}

func TestListEmptyResultIsOK(t *testing.T) {
	dir := project(t)
	got := run(dir, "list", "--filter", "zzz")
	wantStatus(t, got, ExitOK)
	if got.stdout != "" {
		t.Fatalf("expected zero bytes, got %q", got.stdout)
	}
}

func TestListTruncatesDescription(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("é", 150)
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: "+long+"\nversion: v1\n---\nspec: R\nINPUT:\n- MUST: x\n")
	got := run(dir, "list")
	f := strings.Split(lines(got.stdout)[0], "\t")[3]
	if r := []rune(f); len(r) != 100 || r[99] != '…' {
		t.Fatalf("truncation = %d scalars, last %q", len(r), string(r[len(r)-1]))
	}
	// Exactly 100 scalar values is not truncated.
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: "+strings.Repeat("é", 100)+"\nversion: v1\n---\nspec: R\nINPUT:\n- MUST: x\n")
	got = run(dir, "list")
	f = strings.Split(lines(got.stdout)[0], "\t")[3]
	if r := []rune(f); len(r) != 100 || strings.Contains(f, "…") {
		t.Fatalf("a 100-scalar description must not be truncated: %d", len(r))
	}
}

func TestListParseFailure(t *testing.T) {
	dir := project(t)
	write(t, dir, "bad.yass.yaml", "a: b\n  c: d\n- x\n")
	got := run(dir, "list")
	wantStatus(t, got, ExitFindings)
	if !strings.Contains(got.stdout, "bad\tfile\t-\t-\n") {
		t.Fatalf("expected a file record with `-` DETAIL and DESCRIPTION:\n%s", got.stdout)
	}
	for _, l := range lines(got.stdout) {
		if strings.HasPrefix(l, "bad@") {
			t.Fatalf("a file that yielded no documents must yield no document record: %s", l)
		}
	}
}

func TestListNoPreamble(t *testing.T) {
	dir := project(t)
	write(t, dir, "nopre.yass.yaml", modeline+"---\nspec: NoPre\nINPUT:\n- MUST: x\n")
	got := run(dir, "list")
	wantStatus(t, got, ExitFindings)
	if !strings.Contains(got.stdout, "nopre\tfile\t1\t-\n") {
		t.Fatalf("got:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, "nopre@NoPre\tspec\tINPUT\t-\n") {
		t.Fatalf("document records must still be emitted:\n%s", got.stdout)
	}
}

func TestListPathOperands(t *testing.T) {
	dir := project(t)
	got := run(dir, "list", "sub", "sub/b.yass.yaml")
	wantStatus(t, got, ExitOK)
	if len(lines(got.stdout)) != 2 {
		t.Fatalf("a file addressed by two PATHs must be taken once:\n%s", got.stdout)
	}
	bad := run(dir, "list", "root.yass.yaml.txt")
	wantStatus(t, bad, ExitUnresolved)
	write(t, dir, "plain.txt", "hi\n")
	bad2 := run(dir, "list", "plain.txt")
	wantStatus(t, bad2, ExitUsage)
	if !strings.Contains(bad2.stderr, "yass.io.not_a_spec_file") {
		t.Fatalf("stderr = %q", bad2.stderr)
	}
}

// A PATH that is a symbolic link to a directory addresses the tree under the
// directory it names: the walk begins at the directory, not at the link.
func TestListSymlinkedDirectoryOperand(t *testing.T) {
	dir := project(t)
	if err := os.Symlink(filepath.Join(dir, "sub"), filepath.Join(dir, "sublink")); err != nil {
		t.Skipf("cannot create a symbolic link: %v", err)
	}
	for _, sub := range []string{"list", "validate", "lint"} {
		direct := run(dir, sub, "sub")
		viaLink := run(dir, sub, "sublink")
		if viaLink.status != direct.status || viaLink.stdout != direct.stdout || viaLink.stderr != direct.stderr {
			t.Errorf("`%s sublink` differs from `%s sub`:\n%v\n%v", sub, sub, direct, viaLink)
		}
	}
	got := run(dir, "list", "sublink")
	wantStatus(t, got, ExitOK)
	if n := len(lines(got.stdout)); n != 2 {
		t.Fatalf("record count = %d, want the sub tree's two records:\n%s", n, got.stdout)
	}
}

// A directory PATH that is itself a nested project root yields nothing, under
// the same nearer-root exclusion the project file set applies; naming the
// project root itself still lists the project, and from inside the nested
// root that tree is the project.
func TestListNestedRootOperandYieldsNothing(t *testing.T) {
	dir := project(t)
	got := run(dir, "list", "sub/nested")
	wantStatus(t, got, ExitOK)
	if got.stdout != "" {
		t.Fatalf("a nested project root operand must yield nothing:\n%s", got.stdout)
	}
	self := run(dir, "list", ".")
	bare := run(dir, "list")
	if self.status != bare.status || self.stdout != bare.stdout {
		t.Fatalf("`list .` differs from `list`:\n%s\n%s", self.stdout, bare.stdout)
	}
	nested := run(filepath.Join(dir, "sub", "nested"), "list")
	wantStatus(t, nested, ExitOK)
	want := "root\tfile\t1\tnested|root@Nested\tspec\tINVARIANT\t-|" +
		"x\tfile\t1\tnested x|x@X\tspec\tINPUT\t-"
	if joined := strings.Join(lines(nested.stdout), "|"); joined != want {
		t.Fatalf("nested listing = %q, want %q", joined, want)
	}
}

// A directory that cannot be read is named by the diagnostic, both when the
// traversal meets it and when the ascent does.
func TestUnreadableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; directory permissions are not enforced")
	}
	dir := project(t)
	write(t, dir, "locked/x.yass.yaml", modeline+"---\ndescription: locked\nversion: v1\n---\nspec: L\nINPUT:\n- MUST: x\n")
	locked := filepath.Join(dir, "locked")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o755) //nolint:errcheck
	got := run(dir, "list")
	wantStatus(t, got, ExitEnvironment)
	if !strings.Contains(got.stderr, "yass.io.unreadable") || !strings.Contains(got.stderr, "locked") {
		t.Fatalf("traversal stderr = %q, want yass.io.unreadable naming locked", got.stderr)
	}

	outside := t.TempDir()
	shut := filepath.Join(outside, "shut")
	if err := os.Mkdir(shut, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(shut, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(shut, 0o755) //nolint:errcheck
	got = run(shut, "root")
	wantStatus(t, got, ExitEnvironment)
	if !strings.Contains(got.stderr, "yass.io.unreadable") || !strings.Contains(got.stderr, "shut") {
		t.Fatalf("ascent stderr = %q, want yass.io.unreadable naming shut", got.stderr)
	}
}

// A directory named root.yass.yaml is not a root file; the ascent climbs past
// it rather than stopping there.
func TestAscentSkipsDirectoryNamedRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "root.yass.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := run(filepath.Join(dir, "root.yass.yaml"), "root")
	wantStatus(t, got, ExitNoRoot)
	if !strings.Contains(got.stderr, "yass.root.not_found") {
		t.Fatalf("stderr = %q", got.stderr)
	}
}

// A filter that selects no record of a failing file leaves the finding out of
// the answer, and an answer that is empty exits with the ok status.
func TestListFilterEmptyOverFindingsIsOK(t *testing.T) {
	dir := project(t)
	write(t, dir, "bad.yass.yaml", "a: b\n  c: d\n- x\n")
	got := run(dir, "list", "--filter", "zzz*")
	wantStatus(t, got, ExitOK)
	if got.stdout != "" {
		t.Fatalf("expected zero bytes, got %q", got.stdout)
	}
	// A finding whose record is part of the answer still selects the findings
	// status.
	got = run(dir, "list", "--filter", "bad*")
	wantStatus(t, got, ExitFindings)
	if !strings.Contains(got.stdout, "bad\tfile\t-\t-\n") {
		t.Fatalf("stdout = %q, want the failing file's record", got.stdout)
	}
	// A filter that matches other files' records but not the failing file's
	// leaves the finding out of the answer's status.
	got = run(dir, "list", "--filter", "*@Alpha")
	wantStatus(t, got, ExitOK)
	if strings.Contains(got.stdout, "bad\t") {
		t.Fatalf("the filtered-out file must contribute no record:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, "a@Alpha") {
		t.Fatalf("stdout = %q, want a@Alpha", got.stdout)
	}
}

// Every addressed file is loaded before any record is written, so a file that
// cannot be read leaves standard output empty alongside the environment
// status.
func TestListUnreadableLeavesStdoutEmpty(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; file permissions are not enforced")
	}
	dir := project(t)
	m := write(t, dir, "m.yass.yaml", modeline+"---\ndescription: m\nversion: v1\n---\nspec: M\nINPUT:\n- MUST: x\n")
	if err := os.Chmod(m, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(m, 0o644) //nolint:errcheck
	got := run(dir, "list")
	wantStatus(t, got, ExitEnvironment)
	if got.stdout != "" {
		t.Fatalf("stdout must be empty before an environment failure, got %q", got.stdout)
	}
	if !strings.Contains(got.stderr, "yass.io.unreadable") || !strings.Contains(got.stderr, "m") {
		t.Fatalf("stderr = %q, want yass.io.unreadable naming m", got.stderr)
	}
}

// The file record's DETAIL is the count of document records that follow it;
// a document too unnamed to be addressed gets neither a record nor a count.
func TestListDetailCountsEmittedDocuments(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: Named\nINPUT:\n- MUST: x\n---\nspec:\nINPUT:\n- MUST: y\n---\ndesign: D\ntype: constraint\ncontent: |\n  c\n")
	got := run(dir, "list")
	wantStatus(t, got, ExitOK)
	ls := lines(got.stdout)
	if len(ls) != 3 {
		t.Fatalf("record count = %d, want the file record plus two document records:\n%s", len(ls), got.stdout)
	}
	if !strings.HasPrefix(ls[0], "root\tfile\t2\t") {
		t.Fatalf("DETAIL must count the two addressable documents: %q", ls[0])
	}
}

// --- find -------------------------------------------------------------------

func TestFindCaseFolding(t *testing.T) {
	dir := project(t)
	got := run(dir, "find", "OPEN THE UMBRELLA")
	wantStatus(t, got, ExitOK)
	want := "a@Alpha::INPUT\tMUST\topen the umbrella\n"
	if got.stdout != want {
		t.Fatalf("got %q want %q", got.stdout, want)
	}
}

func TestFindSimpleFoldingNotFull(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: R\nINPUT:\n- MUST: straße\n")
	if got := run(dir, "find", "STRASSE"); got.stdout != "" {
		t.Fatalf("full case folding leaked in: %q", got.stdout)
	}
	if got := run(dir, "find", "STRAßE"); got.stdout == "" {
		t.Fatalf("simple folding should match ß")
	}
}

func TestFindGuardAndProse(t *testing.T) {
	dir := project(t)
	got := run(dir, "find", "cold")
	wantStatus(t, got, ExitOK)
	want := []string{
		"a@Beta::RETURN\tWHEN\tit is cold",
		"sub/b@Beta::RETURN\tWHEN\tit is cold",
	}
	if strings.Join(lines(got.stdout), "|") != strings.Join(want, "|") {
		t.Fatalf("got:\n%s", got.stdout)
	}
	// A field matched more than once still yields exactly one record.
	one := run(dir, "find", "a")
	seen := map[string]bool{}
	for _, l := range lines(one.stdout) {
		if seen[l] {
			t.Fatalf("duplicate record: %s", l)
		}
		seen[l] = true
	}
}

// A duplicated key does not make a second field: the loader records the first
// occurrence, and the field — not the authored entry — is the unit of
// retrieval, so each field yields exactly one record no matter how many keys
// carry it.
func TestFindDuplicateKeyOneRecordPerField(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: R\nRETURN:\n- MUST: first prose with umbrella\n  MUST: second prose without\n- WHEN: first guard with umbrella\n  WHEN: second guard without\n  MUST: hold an umbrella\n")
	got := run(dir, "find", "umbrella")
	wantStatus(t, got, ExitOK)
	want := "root@R::RETURN\tMUST\tfirst prose with umbrella\n" +
		"root@R::RETURN\tWHEN\tfirst guard with umbrella\n" +
		"root@R::RETURN\tMUST\thold an umbrella\n"
	if got.stdout != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got.stdout, want)
	}
}

func TestFindSlotRestriction(t *testing.T) {
	dir := project(t)
	got := run(dir, "find", "--slot", "RETURN", "yield")
	wantStatus(t, got, ExitOK)
	for _, l := range lines(got.stdout) {
		if !strings.Contains(l, "::RETURN\t") {
			t.Fatalf("--slot did not restrict: %s", l)
		}
	}
	// A design's content is never restricted by --slot.
	d := run(dir, "find", "--slot", "INPUT", "one")
	found := false
	for _, l := range lines(d.stdout) {
		if strings.HasPrefix(l, "root@Policy\tcontent\t") {
			found = true
		}
	}
	if !found {
		t.Fatalf("design content should be emitted unrestricted:\n%s", d.stdout)
	}
	bad := run(dir, "find", "--slot", "NOPE", "x")
	wantStatus(t, bad, ExitUsage)
	if !strings.Contains(bad.stderr, "yass.ref.slot_unknown") {
		t.Fatalf("stderr = %q", bad.stderr)
	}
}

func TestFindEmptyResultIsOK(t *testing.T) {
	dir := project(t)
	got := run(dir, "find", "zzzznotthere")
	wantStatus(t, got, ExitOK)
	if got.stdout != "" {
		t.Fatalf("expected zero bytes, got %q", got.stdout)
	}
}

func TestFindSkipsReferenceOnlyObligation(t *testing.T) {
	dir := project(t)
	got := run(dir, "find", "Beta::RETURN")
	if got.stdout != "" {
		t.Fatalf("a reference target must not be searched: %q", got.stdout)
	}
}

func TestFindDoesNotSearchDescriptions(t *testing.T) {
	dir := project(t)
	got := run(dir, "find", "the alpha file")
	if got.stdout != "" {
		t.Fatalf("a preamble description must not be searched: %q", got.stdout)
	}
}

// --- refs -------------------------------------------------------------------

func TestRefsDirections(t *testing.T) {
	dir := project(t)
	both := run(dir, "refs", "root@Policy")
	wantStatus(t, both, ExitOK)
	want := []string{
		"in\tUSES\ta@Alpha",
		"in\tUSES\ta@Alpha",
		"in\tUSES\troot@Proj",
		"in\tUSES\troot@Proj",
	}
	if strings.Join(lines(both.stdout), "|") != strings.Join(want, "|") {
		t.Fatalf("got:\n%s", both.stdout)
	}
	out := run(dir, "refs", "--out", "root@Policy")
	if out.stdout != "" {
		t.Fatalf("a design carries no outgoing edge: %q", out.stdout)
	}
	// Both flags mean both, exactly as though neither had been given.
	bb := run(dir, "refs", "--in", "--out", "root@Policy")
	if bb.stdout != both.stdout {
		t.Fatalf("--in --out should equal neither")
	}
}

func TestRefsOutgoingOrderAndSlot(t *testing.T) {
	dir := project(t)
	got := run(dir, "refs", "--out", "a@Alpha")
	want := []string{
		"out\tCONFORMS\ta@Beta::RETURN",
		"out\tCONFORMS\tsub/b@Beta::RETURN",
		"out\tUSES\troot@Policy",
		"out\tUSES\troot@Policy",
	}
	if strings.Join(lines(got.stdout), "|") != strings.Join(want, "|") {
		t.Fatalf("got:\n%s", got.stdout)
	}
	slot := run(dir, "refs", "--out", "a@Alpha::RETURN")
	if len(lines(slot.stdout)) != 2 {
		t.Fatalf("a slot target must restrict outgoing edges:\n%s", slot.stdout)
	}
}

func TestRefsFileTargetIsUsageError(t *testing.T) {
	dir := project(t)
	got := run(dir, "refs", "a")
	wantStatus(t, got, ExitUsage)
	if !strings.Contains(got.stderr, "yass.args.missing_operand") {
		t.Fatalf("stderr = %q", got.stderr)
	}
}

func TestRefsEmptyIsOK(t *testing.T) {
	dir := project(t)
	got := run(dir, "refs", "a@Note")
	wantStatus(t, got, ExitOK)
	if got.stdout != "" {
		t.Fatalf("expected zero bytes: %q", got.stdout)
	}
}

// The file set is taken whatever the direction flags say, so a parse failure
// under the root is a finding even when only outgoing edges were asked for,
// and the edges of every file that did parse are still emitted.
func TestRefsOutReportsParseFailure(t *testing.T) {
	dir := project(t)
	write(t, dir, "sub/broken.yass.yaml", modeline+"a: b\n  c: d\n- x\n")
	got := run(dir, "refs", "--out", "root@Proj")
	wantStatus(t, got, ExitFindings)
	if !strings.Contains(got.stderr, "yass.document.not_yaml") || !strings.Contains(got.stderr, "sub/broken") {
		t.Fatalf("stderr = %q", got.stderr)
	}
	if !strings.Contains(got.stdout, "out\tUSES\troot@Policy") {
		t.Fatalf("edges of files that parsed must still be emitted:\n%s", got.stdout)
	}
}

// --- target resolution ------------------------------------------------------

func TestTargetResolutionErrors(t *testing.T) {
	dir := project(t)
	write(t, dir, "dupe.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: Twice\nINPUT:\n- MUST: x\n---\ndesign: Twice\ntype: constraint\ncontent: |\n  c\n")
	cases := []struct {
		target string
		code   string
		status int
	}{
		{"a@Alpha::bad", "yass.args.target_syntax", ExitUsage},
		{"a@Alpha::NOPE", "yass.ref.slot_unknown", ExitUsage},
		{"a@Al pha", "yass.args.target_syntax", ExitUsage},
		{`a\b@Alpha`, "yass.args.target_syntax", ExitUsage},
		{"/a@Alpha", "yass.args.target_syntax", ExitUsage},
		{"missing@Alpha", "yass.ref.unresolved", ExitUnresolved},
		{"a@Nothing", "yass.ref.unresolved", ExitUnresolved},
		{"a@Note::INPUT", "yass.ref.slot_on_design", ExitUnresolved},
		{"a@Alpha::SIDE-EFFECT", "yass.ref.slot_undeclared", ExitUnresolved},
		{"dupe@Twice", "yass.ref.unresolved", ExitUnresolved},
	}
	for _, c := range cases {
		got := run(dir, "query", c.target)
		wantStatus(t, got, c.status)
		if !strings.Contains(got.stderr, c.code) {
			t.Errorf("%s: stderr = %q, want %s", c.target, got.stderr, c.code)
		}
		if got.stdout != "" {
			t.Errorf("%s: stdout must be empty, got %q", c.target, got.stdout)
		}
	}
}

func TestTargetIsCaseSensitive(t *testing.T) {
	dir := project(t)
	got := run(dir, "query", "a@alpha")
	wantStatus(t, got, ExitUnresolved)
}

// A path token compares case-sensitively on every platform: a differently
// cased spelling of an existing file or directory resolves to nothing, even
// where a case-insensitive filesystem would open the file it spells.
func TestTargetPathTokenIsCaseSensitive(t *testing.T) {
	dir := project(t)
	for _, target := range []string{"A@Alpha", "A", "Root", "SUB/b@Beta", "sub/B@Beta", "./Sub/b@Beta"} {
		got := run(dir, "query", target)
		wantStatus(t, got, ExitUnresolved)
		if !strings.Contains(got.stderr, "yass.ref.unresolved") {
			t.Errorf("%s: stderr = %q", target, got.stderr)
		}
		if got.stdout != "" {
			t.Errorf("%s: stdout must be empty, got %q", target, got.stdout)
		}
	}
	// The exact spellings, including `./` and `../` lead segments, resolve.
	for _, target := range []string{"a@Alpha", "./sub/b@Beta", "sub/../a@Alpha"} {
		wantStatus(t, run(dir, "query", target), ExitOK)
	}
	wantStatus(t, run(filepath.Join(dir, "sub"), "query", "../a@Alpha"), ExitOK)
	wantStatus(t, run(filepath.Join(dir, "sub"), "query", "../A@Alpha"), ExitUnresolved)
}

// A reference inside a document compares its path token case-sensitively too.
func TestInFileRefPathTokenIsCaseSensitive(t *testing.T) {
	cases := []struct{ rel, ref string }{
		{"c.yass.yaml", "A@Alpha"},
		{"c.yass.yaml", "SUB/b@Beta::RETURN"},
		{"sub/c.yass.yaml", "./B@Beta::RETURN"},
		{"sub/c.yass.yaml", "../A@Alpha"},
	}
	for _, c := range cases {
		dir := project(t)
		write(t, dir, c.rel, modeline+"---\ndescription: c\nversion: v1\n---\nspec: C\nINPUT:\n- MUST: x\n  SEE: "+c.ref+"\n")
		codes := validateCodes(t, dir)
		found := false
		for _, code := range codes {
			if strings.Contains(code, "yass.ref.unresolved") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s in %s: codes = %q, want yass.ref.unresolved", c.ref, c.rel, codes)
		}
		// Resolution for a command, not only validation, refuses the spelling:
		// refs drops an unresolvable reference rather than report an edge.
		carrier := strings.TrimSuffix(c.rel, SpecSuffix) + "@C"
		got := run(dir, "refs", "--out", carrier)
		wantStatus(t, got, ExitOK)
		if got.stdout != "" {
			t.Errorf("refs --out %s carrying %s: stdout = %q, want no edge", carrier, c.ref, got.stdout)
		}
	}
}

// The project root's file name compares byte-for-byte, so a differently cased
// root file neither makes a project root nor hides a subtree as a nested one.
func TestRootFileNameIsCaseSensitive(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "ROOT.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: R\nINPUT:\n- MUST: x\n")
	got := run(dir, "root")
	wantStatus(t, got, ExitNoRoot)
	if !strings.Contains(got.stderr, "yass.root.not_found") {
		t.Fatalf("stderr = %q", got.stderr)
	}

	proj := project(t)
	write(t, proj, "sub/other/Root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: R\nINPUT:\n- MUST: x\n")
	list := run(proj, "list", "--format", "json")
	wantStatus(t, list, ExitOK)
	if !strings.Contains(list.stdout, "sub/other/Root") {
		t.Fatalf("a differently cased root file must stay in the outer file set:\n%s", list.stdout)
	}
}

// The spelling check compares each path token component with the directory's
// entry names byte-for-byte, so it refuses a differently cased spelling on any
// filesystem, case-sensitive or not, and accepts every exact one.
func TestSpelledExactly(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.yass.yaml", "")
	write(t, dir, "sub/b.yass.yaml", "")
	sub := filepath.Join(dir, "sub")
	a := &App{Root: dir}
	cases := []struct {
		base, token string
		want        bool
	}{
		{dir, "A", false},
		{dir, "SUB/b", false},
		{dir, "sub/B", false},
		{sub, "../A", false},
		{dir, "a", true},
		{dir, "sub/b", true},
		{dir, "./a", true},
		{sub, "../a", true},
	}
	for _, c := range cases {
		if got := a.spelledExactly(c.base, c.token); got != c.want {
			t.Errorf("spelledExactly(%s, %q) = %v, want %v", relTo(dir, c.base), c.token, got, c.want)
		}
	}
}

// The suffix is appended to the path token exactly as written: `.` and `..`
// segments stay part of the name looked up, so a token spelling a directory
// names no file rather than resolving to the neighbour a cleaned path would
// reach — here a sibling named as the project directory.
func TestTargetTokenIsNotCleaned(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "proj")
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: the project\nversion: v1\n---\nspec: P\nINPUT:\n- MUST: x\n")
	write(t, dir, "sub/b.yass.yaml", modeline+"---\ndescription: the b file\nversion: v1\n---\nspec: Beta\nRETURN:\n- MUST: return b\n")
	write(t, base, "proj.yass.yaml", modeline+"---\ndescription: the sibling\nversion: v1\n---\nspec: S\nINPUT:\n- MUST: x\n")
	for _, target := range []string{".", "sub/.."} {
		got := run(dir, "query", target)
		wantStatus(t, got, ExitUnresolved)
		if !strings.Contains(got.stderr, "yass.ref.unresolved") {
			t.Errorf("%s: stderr = %q", target, got.stderr)
		}
		if got.stdout != "" {
			t.Errorf("%s: a token that names nothing must emit nothing, got %q", target, got.stdout)
		}
	}
	// A trailing slash spells the directory's hidden `.yass.yaml`, not the
	// directory cleaned away; an empty segment is inert and names the same
	// file either way.
	got := run(dir, "query", "sub/")
	wantStatus(t, got, ExitUnresolved)
	got = run(dir, "query", "sub//b")
	wantStatus(t, got, ExitOK)
	if !strings.Contains(got.stdout, "the b file") {
		t.Fatalf("stdout = %q, want the b file's preamble", got.stdout)
	}
}

// --- query ------------------------------------------------------------------

func TestQueryFileEmitsPreambleAlone(t *testing.T) {
	dir := project(t)
	got := run(dir, "query", "a")
	wantStatus(t, got, ExitOK)
	want := "---\ndescription: the alpha file\nversion: v1\n"
	if got.stdout != want {
		t.Fatalf("got %q want %q", got.stdout, want)
	}
}

func TestQueryResolution(t *testing.T) {
	dir := project(t)
	got := run(dir, "query", "a@Alpha")
	wantStatus(t, got, ExitOK)
	want := strings.Join([]string{
		"---",
		"spec: Alpha",
		"INPUT:",
		"# CONFORMS: Beta::RETURN",
		"- MUST: give back the goods",
		"# CONFORMS: Beta::RETURN",
		"- WHEN: it is cold",
		"  MUST: give back a coat",
		"# CONFORMS: sub/b@Beta::RETURN",
		"- WHEN: it is raining",
		"  MUST: return b",
		"# CONFORMS: sub/b@Beta::RETURN",
		"- WHEN: it is raining and it is cold",
		"  MUST: return a coat",
		"- WHEN: it is raining",
		"  MUST: open the umbrella",
		"  CONFORMS: sub/b@Beta::RETURN",
		"RETURN:",
		`- MUST: "yield: a value"`,
		"  USES: root@Policy",
		"- MAY: also yield nothing",
		"  USES: root@Policy",
		"---",
		"# USES: root@Policy",
		"design: Policy",
		"type: constraint",
		"content: |",
		"  one",
		"  two",
		"",
	}, "\n")
	if got.stdout != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got.stdout, want)
	}
}

func TestQueryRawResolvesNothing(t *testing.T) {
	dir := project(t)
	got := run(dir, "query", "--raw", "a@Alpha")
	wantStatus(t, got, ExitOK)
	if strings.Contains(got.stdout, "# CONFORMS:") || strings.Contains(got.stdout, "# USES:") {
		t.Fatalf("--raw must insert no comment:\n%s", got.stdout)
	}
	if strings.Contains(got.stdout, "design: Policy") {
		t.Fatalf("--raw must append nothing:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, "- CONFORMS: Beta::RETURN\n") {
		t.Fatalf("--raw must keep a reference-only obligation:\n%s", got.stdout)
	}
}

func TestQuerySlotTarget(t *testing.T) {
	dir := project(t)
	got := run(dir, "query", "root@Proj::INVARIANT")
	wantStatus(t, got, ExitOK)
	if !strings.HasPrefix(got.stdout, "---\nspec: Proj\nINVARIANT: []\n") {
		t.Fatalf("a slot fragment still states what was queried, and a slot emptied by transclusion is an empty list, not null:\n%s", got.stdout)
	}
	if strings.Contains(got.stdout, "INPUT:") {
		t.Fatalf("a slot target must emit that slot alone:\n%s", got.stdout)
	}
	// A reference-only USES adds no obligation of its own but still appends.
	if strings.Contains(got.stdout, "- USES: Policy\n") {
		t.Fatalf("a transcluding reference-only obligation must emit nothing of its own:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, "# USES: Policy\ndesign: Policy\n") {
		t.Fatalf("the bound design must be appended:\n%s", got.stdout)
	}
}

func TestQueryDesignUnchanged(t *testing.T) {
	dir := project(t)
	got := run(dir, "query", "root@Policy")
	wantStatus(t, got, ExitOK)
	want := "---\ndesign: Policy\ntype: constraint\ncontent: |\n  one\n  two\n"
	if got.stdout != want {
		t.Fatalf("got %q", got.stdout)
	}
}

func TestQueryMultipleTargetsInOrder(t *testing.T) {
	dir := project(t)
	got := run(dir, "query", "root@Policy", "a@Note")
	wantStatus(t, got, ExitOK)
	if strings.Index(got.stdout, "design: Policy") > strings.Index(got.stdout, "design: Note") {
		t.Fatalf("fragments must follow TARGET order:\n%s", got.stdout)
	}
	if strings.Count(got.stdout, "---\n") != 2 {
		t.Fatalf("each fragment is opened by a --- line:\n%s", got.stdout)
	}
}

func TestQueryFailsBeforeEmitting(t *testing.T) {
	dir := project(t)
	got := run(dir, "query", "root@Policy", "a@Nothing")
	wantStatus(t, got, ExitUnresolved)
	if got.stdout != "" {
		t.Fatalf("a failure on any TARGET leaves standard output empty: %q", got.stdout)
	}
}

func TestQueryUnresolvedReferenceAndRawEscapeHatch(t *testing.T) {
	dir := project(t)
	write(t, dir, "broken.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: Broken\nINPUT:\n- MUST: x\n  SEE: nowhere@Gone\n")
	got := run(dir, "query", "broken@Broken")
	wantStatus(t, got, ExitUnresolved)
	if !strings.Contains(got.stderr, "yass.ref.unresolved") {
		t.Fatalf("stderr = %q", got.stderr)
	}
	if !strings.Contains(got.stderr, "nowhere@Gone") {
		t.Fatalf("the diagnostic must name the reference target: %q", got.stderr)
	}
	raw := run(dir, "query", "--raw", "broken@Broken")
	wantStatus(t, raw, ExitOK)
	if !strings.Contains(raw.stdout, "SEE: nowhere@Gone") {
		t.Fatalf("--raw always serves a document whose file parsed:\n%s", raw.stdout)
	}
}

func TestQueryRelationKindMismatch(t *testing.T) {
	dir := project(t)
	write(t, dir, "mix.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: M\nINPUT:\n- MUST: x\n  CONFORMS: root@Policy\nRETURN:\n- MUST: y\n  USES: a@Alpha\n")
	c := run(dir, "query", "mix@M::INPUT")
	wantStatus(t, c, ExitUnresolved)
	if !strings.Contains(c.stderr, "yass.ref.conforms_design") {
		t.Fatalf("stderr = %q", c.stderr)
	}
	u := run(dir, "query", "mix@M::RETURN")
	wantStatus(t, u, ExitUnresolved)
	if !strings.Contains(u.stderr, "yass.ref.uses_spec") {
		t.Fatalf("stderr = %q", u.stderr)
	}
}

// A slot whose every obligation was transcluded away — by a reference-only
// USES, or by a slot-targeted CONFORMS on a slot that declares nothing — emits
// an empty list, never a null slot.
func TestQueryEmptySlotEmitsEmptyList(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: Empty\nRETURN: []\n---\nspec: C\nINPUT:\n- CONFORMS: root@Empty::RETURN\n")
	got := run(dir, "query", "root@C::INPUT")
	wantStatus(t, got, ExitOK)
	if want := "---\nspec: C\nINPUT: []\n"; got.stdout != want {
		t.Fatalf("got %q want %q", got.stdout, want)
	}
}

// An alias emits the value it names, under --raw and everywhere else, instead
// of silently dropping the value.
func TestQueryKeepsAliasValues(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: &d the anchored value\nversion: *d\n---\nspec: R\nINPUT:\n- MUST: *d\n")
	raw := run(dir, "query", "--raw", "root@R")
	wantStatus(t, raw, ExitOK)
	if !strings.Contains(raw.stdout, "- MUST: the anchored value") {
		t.Fatalf("--raw must keep the alias's value:\n%s", raw.stdout)
	}
	file := run(dir, "query", "root")
	wantStatus(t, file, ExitOK)
	if !strings.Contains(file.stdout, "version: the anchored value") {
		t.Fatalf("a preamble alias must keep its value:\n%s", file.stdout)
	}
}

// Prose that a plain scalar would resolve to a timestamp, a positive
// infinity, or a merge key keeps its quotes, because unquoting it would stop
// it reading back as a string; prose that resolves to nothing unquotes fine.
func TestQueryQuotesNonStringProse(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: T\nINPUT:\n- MUST: \"2001-12-14\"\n- MUST: \"2001-12-14T10:00:00Z\"\n- MUST: \"+.inf\"\n- MUST: \"<<\"\n- MUST: \"2001-13-45\"\n")
	got := run(dir, "query", "root@T")
	wantStatus(t, got, ExitOK)
	for _, q := range []string{
		"- MUST: \"2001-12-14\"\n",
		"- MUST: \"2001-12-14T10:00:00Z\"\n",
		"- MUST: \"+.inf\"\n",
		"- MUST: \"<<\"\n",
	} {
		if !strings.Contains(got.stdout, q) {
			t.Fatalf("prose that stops reading back as a string must keep its quotes, missing %q:\n%s", q, got.stdout)
		}
	}
	if !strings.Contains(got.stdout, "- MUST: 2001-13-45\n") {
		t.Fatalf("prose that resolves to nothing unquotes fine:\n%s", got.stdout)
	}
}

// A self-referential anchor graph parses but has no finite rendering; the
// emission cycle guard stops it with a diagnostic instead of a crash.
func TestQueryCyclicAliasIsDiagnostic(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: &a [*a]\nversion: v1\n---\nspec: R\nINPUT:\n- WHEN: &b [*b]\n  MUST: x\n")
	file := run(dir, "query", "root")
	wantStatus(t, file, ExitUnresolved)
	if file.stdout != "" {
		t.Fatalf("stdout must be empty: %q", file.stdout)
	}
	if !strings.Contains(file.stderr, "yass.document.yaml_feature") || !strings.Contains(file.stderr, "cyclic") {
		t.Fatalf("stderr = %q, want yass.document.yaml_feature naming the cycle", file.stderr)
	}
	doc := run(dir, "query", "root@R")
	wantStatus(t, doc, ExitUnresolved)
	if doc.stdout != "" || !strings.Contains(doc.stderr, "yass.document.yaml_feature") {
		t.Fatalf("the obligation path must stop the same way: stdout %q stderr %q", doc.stdout, doc.stderr)
	}
	raw := run(dir, "query", "--raw", "root@R")
	wantStatus(t, raw, ExitUnresolved)
	if raw.stdout != "" || !strings.Contains(raw.stderr, "yass.document.yaml_feature") {
		t.Fatalf("--raw must stop the same way: stdout %q stderr %q", raw.stdout, raw.stderr)
	}
}

// A shared anchored subtree reached more than once — at its own position and
// through alias arms — is a diamond with a finite rendering, not a cycle.
func TestQueryAliasedDiamondRenders(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\nversion: &d v1\ndescription: &base\n  v: *d\nx: *base\ny: *base\n---\nspec: R\nINPUT:\n- MUST: x\n  NOTE:\n  - &d leaf\n  - &g\n    v: *d\n  - *g\n  - *g\n")
	file := run(dir, "query", "root")
	wantStatus(t, file, ExitOK)
	if n := strings.Count(file.stdout, "v: v1"); n != 3 {
		t.Fatalf("an anchored subtree renders once directly and once per alias arm:\n%s", file.stdout)
	}
	doc := run(dir, "query", "root@R")
	wantStatus(t, doc, ExitOK)
	if n := strings.Count(doc.stdout, "v: leaf"); n != 3 {
		t.Fatalf("two alias arms inside one value must both render:\n%s", doc.stdout)
	}
}

// An in-file reference to a duplicated document name resolves to neither, and
// the diagnostic names the carrying document and the reference target.
func TestQueryDuplicateNameInFileRefResolvesToNeither(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: Root\nINPUT:\n- MUST: x\n")
	write(t, dir, "sub/a.yass.yaml", modeline+"---\ndescription: a\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: first\n---\nspec: A\nINPUT:\n- MUST: second\n")
	write(t, dir, "c.yass.yaml", modeline+"---\ndescription: c\nversion: v1\n---\nspec: C\nINPUT:\n- CONFORMS: sub/a@A::INPUT\n")
	got := run(dir, "query", "c@C")
	wantStatus(t, got, ExitUnresolved)
	if !strings.Contains(got.stderr, "yass.ref.unresolved") {
		t.Fatalf("stderr = %q", got.stderr)
	}
	if !strings.Contains(got.stderr, "c@C") || !strings.Contains(got.stderr, "sub/a@A::INPUT") {
		t.Fatalf("the diagnostic must name the carrying document and the target: %q", got.stderr)
	}
}

// A referenced file that exists but cannot be read is an environment failure,
// not an unresolved reference.
func TestQueryUnreadableReferencedFileIsEnvironmentFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; file permissions are not enforced")
	}
	dir := project(t)
	b := filepath.Join(dir, "sub", "b.yass.yaml")
	if err := os.Chmod(b, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(b, 0o644) //nolint:errcheck
	// Through resolution: a@Alpha carries CONFORMS: sub/b@Beta::RETURN.
	got := run(dir, "query", "a@Alpha")
	wantStatus(t, got, ExitEnvironment)
	if got.stdout != "" {
		t.Fatalf("stdout must be empty before an environment failure, got %q", got.stdout)
	}
	if !strings.Contains(got.stderr, "yass.io.unreadable") || !strings.Contains(got.stderr, "sub/b") {
		t.Fatalf("stderr = %q, want yass.io.unreadable naming sub/b", got.stderr)
	}
	// Directly: the target names the unreadable file itself.
	direct := run(dir, "query", "sub/b@Beta")
	wantStatus(t, direct, ExitEnvironment)
	if !strings.Contains(direct.stderr, "yass.io.unreadable") {
		t.Fatalf("stderr = %q", direct.stderr)
	}
}

func TestQueryResolvesOneLevelOnly(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+`---
description: d
version: v1
---
spec: R
INPUT:
- CONFORMS: Mid::RETURN
---
spec: Mid
RETURN:
- MUST: middle
  CONFORMS: Deep::RETURN
  USES: D
---
spec: Deep
RETURN:
- MUST: deep
---
design: D
type: constraint
content: |
  d
`)
	got := run(dir, "query", "root@R")
	wantStatus(t, got, ExitOK)
	if strings.Contains(got.stdout, "deep") {
		t.Fatalf("resolution must stop at one level:\n%s", got.stdout)
	}
	if strings.Contains(got.stdout, "design: D") {
		t.Fatalf("a design bound by an inserted obligation must not be appended:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, "  CONFORMS: Deep::RETURN\n") {
		t.Fatalf("the inserted obligation must be unaltered apart from its guard:\n%s", got.stdout)
	}
}

func TestQueryNeverDeduplicatesInsertions(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+`---
description: d
version: v1
---
spec: R
INPUT:
- MUST: one
  CONFORMS: S::RETURN
- MUST: two
  CONFORMS: S::RETURN
---
spec: S
RETURN:
- MUST: shared
`)
	got := run(dir, "query", "root@R")
	if strings.Count(got.stdout, "- MUST: shared") != 2 {
		t.Fatalf("two carriers inlining one slot each get their own copy:\n%s", got.stdout)
	}
}

// --- validate ---------------------------------------------------------------

func validateCodes(t *testing.T, dir string, args ...string) []string {
	t.Helper()
	got := run(dir, append([]string{"validate"}, args...)...)
	var codes []string
	for _, l := range lines(got.stdout) {
		f := strings.Split(l, "\t")
		if len(f) != 5 {
			t.Fatalf("diagnostic has %d fields, want 5: %q", len(f), l)
		}
		if f[0] != "error" {
			t.Fatalf("validation severity = %q, want error", f[0])
		}
		codes = append(codes, f[1])
	}
	return codes
}

func TestValidateCleanProject(t *testing.T) {
	dir := project(t)
	got := run(dir, "validate")
	wantStatus(t, got, ExitOK)
	if got.stdout != "" {
		t.Fatalf("expected zero bytes:\n%s", got.stdout)
	}
}

func TestValidateCodes(t *testing.T) {
	dir := project(t)
	cases := []struct {
		name string
		body string
		want string
	}{
		{"encoding-bom", "\ufeff" + modeline + "---\ndescription: d\nversion: v1\n", "yass.document.encoding"},
		{"not-yaml", "a: b\n  c: d\n- x\n", "yass.document.not_yaml"},
		{"anchor", modeline + "---\ndescription: &d d\nversion: v1\n", "yass.document.yaml_feature"},
		{"alias", modeline + "---\ndescription: d\nversion: v1\nx: 1\n---\nspec: A\nINPUT:\n- MUST: q\n", "yass.preamble.unknown_key"},
		{"tagged", modeline + "---\ndescription: !!str d\nversion: v1\n", "yass.document.yaml_feature"},
		{"dupkey", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n  MUST: y\n", "yass.document.duplicate_key"},
		{"unknown-kind", modeline + "---\ndescription: d\nversion: v1\n---\nfoo: bar\n", "yass.document.unknown_kind"},
		{"both-keys", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\ndesign: B\nINPUT:\n- MUST: x\n", "yass.document.both_keys"},
		{"name-dup", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n---\ndesign: A\ntype: constraint\ncontent: |\n  c\n", "yass.document.name_duplicate"},
		{"preamble-missing", modeline + "---\nspec: A\nINPUT:\n- MUST: x\n", "yass.preamble.missing"},
		{"preamble-incomplete", modeline + "---\ndescription: d\n", "yass.preamble.incomplete"},
		{"preamble-description-type", modeline + "---\ndescription: 7\nversion: v1\n", "yass.preamble.value_type"},
		{"preamble-version", modeline + "---\ndescription: d\nversion: V1\n", "yass.preamble.version"},
		{"related-shape", modeline + "---\ndescription: d\nversion: v1\nrelated: nope\n", "yass.preamble.related_shape"},
		{"spec-unnamed", modeline + "---\ndescription: d\nversion: v1\n---\nspec:\nINPUT:\n- MUST: x\n", "yass.spec.unnamed"},
		{"spec-grammar", modeline + "---\ndescription: d\nversion: v1\n---\nspec: .Bad\nINPUT:\n- MUST: x\n", "yass.spec.name_grammar"},
		{"spec-keyword", modeline + "---\ndescription: d\nversion: v1\n---\nspec: input\nINPUT:\n- MUST: x\n", "yass.spec.name_keyword"},
		{"spec-keyword-guard", modeline + "---\ndescription: d\nversion: v1\n---\nspec: when\nINPUT:\n- MUST: x\n", "yass.spec.name_keyword"},
		{"spec-keyword-relation", modeline + "---\ndescription: d\nversion: v1\n---\nspec: Conforms\nINPUT:\n- MUST: x\n", "yass.spec.name_keyword"},
		{"design-incomplete", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: constraint\n", "yass.design.incomplete"},
		{"design-value-type", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: 7\ncontent: |\n  c\n", "yass.design.value_type"},
		{"design-unknown-key", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: constraint\ncontent: |\n  c\nINPUT:\n- MUST: x\n", "yass.design.unknown_key"},
		{"design-keyword", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: MAY\ntype: constraint\ncontent: |\n  c\n", "yass.design.name_keyword"},
		{"design-keyword-relation", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: see\ntype: constraint\ncontent: |\n  c\n", "yass.design.name_keyword"},
		{"design-unnamed", modeline + "---\ndescription: d\nversion: v1\n---\ndesign:\ntype: constraint\ncontent: |\n  c\n", "yass.design.unnamed"},
		{"design-grammar", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: .D\ntype: constraint\ncontent: |\n  c\n", "yass.design.name_grammar"},
		{"design-type-unknown", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: table\ncontent: |\n  c\n", "yass.design.type_unknown"},
		{"design-type-code", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: code\ncontent: |\n  c\n", "yass.design.type_unknown"},
		{"design-type-ordered-steps", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: ordered-steps\ncontent: |\n  c\n", "yass.design.type_unknown"},
		{"design-format-missing", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: schema\ncontent: |\n  c\n", "yass.design.format_missing"},
		{"design-format-empty", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: schema\nformat: \"\"\ncontent: |\n  c\n", "yass.design.format_missing"},
		{"design-format-type", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: schema\nformat: 7\ncontent: |\n  c\n", "yass.design.value_type"},
		{"design-format-unexpected", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: constraint\nformat: text\ncontent: |\n  c\n", "yass.design.format_unexpected"},
		{"design-content-mapping", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: constraint\ncontent:\n  a: {f: 1}\n", "yass.design.value_type"},
		{"catalog-string", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: catalog\ncontent: |\n  c\n", "yass.design.catalog_shape"},
		{"catalog-empty", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: catalog\ncontent: {}\n", "yass.design.catalog_shape"},
		{"catalog-record-scalar", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: catalog\ncontent:\n  a: one\n", "yass.design.catalog_shape"},
		{"catalog-key-sequence", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: catalog\ncontent:\n  ? [a, b]\n  : {f: 1}\n", "yass.design.catalog_shape"},
		{"catalog-value", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: catalog\ncontent:\n  a: {f: [1, 2]}\n", "yass.design.catalog_value"},
		{"catalog-fields", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: catalog\ncontent:\n  a: {f: 1, g: 2}\n  b: {f: 1, h: 2}\n", "yass.design.catalog_fields"},
		{"catalog-fields-missing", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: catalog\ncontent:\n  a: {f: 1, g: 2}\n  b: {f: 1}\n", "yass.design.catalog_fields"},
		{"catalog-duplicate", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: catalog\ncontent:\n  a: {f: 1}\n  a: {f: 2}\n", "yass.document.duplicate_key"},
		{"encoding-utf8", modeline + "---\ndescription: \xff\xfe\nversion: v1\n", "yass.document.encoding"},
		{"slot-unknown", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nOUTPUT:\n- MUST: x\n", "yass.slot.unknown"},
		{"slot-not-a-list", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT: nope\n", "yass.slot.not_a_list"},
		{"obl-not-a-mapping", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- plain\n", "yass.obligation.not_a_mapping"},
		{"obl-unknown-key", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n  UNLESS: y\n", "yass.obligation.unknown_key"},
		{"obl-multi", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n  MAY: y\n", "yass.obligation.multiple_normativity"},
		{"obl-empty", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- {}\n", "yass.obligation.empty"},
		{"obl-guard-alone", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- WHEN: x\n  SEE: A\n", "yass.obligation.guard_alone"},
		{"obl-not-scalar-null", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST:\n", "yass.obligation.not_scalar"},
		{"obl-not-scalar-map", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST:\n    prose: with a colon\n", "yass.obligation.not_scalar"},
		{"obl-not-scalar-seq", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nWHEN: x\nINPUT:\n- WHEN:\n  - a\n  MUST: x\n", "yass.obligation.not_scalar"},
		{"obl-not-scalar-int", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: 42\n", "yass.obligation.not_scalar"},
		{"obl-not-scalar-bool", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: true\n", "yass.obligation.not_scalar"},
		{"ref-not-a-string", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n  SEE:\n  - A\n", "yass.ref.not_a_string"},
		{"ref-syntax", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n  SEE: \"a b\"\n", "yass.ref.syntax"},
		{"ref-syntax-absolute", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n  SEE: /a@Alpha\n", "yass.ref.syntax"},
		{"ref-unresolved", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n  SEE: Nope\n", "yass.ref.unresolved"},
		{"ref-conforms-design", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n  CONFORMS: root@Policy\n", "yass.ref.conforms_design"},
		{"ref-uses-spec", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n  USES: a@Alpha\n", "yass.ref.uses_spec"},
		{"ref-slot-unknown", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n  SEE: a@Alpha::NOPE\n", "yass.ref.slot_unknown"},
		{"ref-slot-on-design", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n  SEE: root@Policy::INPUT\n", "yass.ref.slot_on_design"},
		{"ref-slot-undeclared", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n  SEE: a@Alpha::ERROR\n", "yass.ref.slot_undeclared"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rel := "case-" + c.name + ".yass.yaml"
			write(t, dir, rel, c.body)
			defer os.Remove(filepath.Join(dir, rel))
			codes := validateCodes(t, dir, rel)
			if !contains(codes, c.want) {
				t.Fatalf("codes = %v, want %s", codes, c.want)
			}
		})
	}
}

func TestValidateObligationStringValues(t *testing.T) {
	dir := project(t)
	for _, c := range []struct {
		name, value string
		reject      bool
	}{
		{"absent", "", true},
		{"null", "null", true},
		{"null-title", "Null", true},
		{"null-upper", "NULL", true},
		{"tilde", "~", true},
		{"int", "42", true},
		{"bool", "true", true},
		{"float", "1.5", true},
		{"empty-double", `""`, false},
		{"empty-single", "''", false},
		{"quoted-null", `"null"`, false},
		{"quoted-tilde", `"~"`, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			write(t, dir, "scalar.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: "+c.value+"\n")
			codes := validateCodes(t, dir, "scalar.yass.yaml")
			if got := contains(codes, "yass.obligation.not_scalar"); got != c.reject {
				t.Fatalf("codes = %v, want string rejection = %v", codes, c.reject)
			}
			if !c.reject && len(codes) != 0 {
				t.Fatalf("quoted string rejected: %v", codes)
			}
		})
	}
}

func TestValidateRootSpecCount(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n")
	codes := validateCodes(t, dir)
	if !contains(codes, "yass.root.spec_count") {
		t.Fatalf("codes = %v", codes)
	}
}

// A preamble key that is present with a value of the wrong type is not
// omitted: a version of any other type is a version that is not v1, and a
// description of another type fails the string requirement.
func TestValidatePreambleValueTypes(t *testing.T) {
	dir := project(t)
	for _, c := range []struct {
		name, body string
		want       []string
	}{
		{"version-int", "description: d\nversion: 1\n", []string{"yass.preamble.version"}},
		{"version-float", "description: d\nversion: 1.0\n", []string{"yass.preamble.version"}},
		{"version-bool", "description: d\nversion: true\n", []string{"yass.preamble.version"}},
		{"version-null", "description: d\nversion:\n", []string{"yass.preamble.version"}},
		{"version-list", "description: d\nversion: [v1]\n", []string{"yass.preamble.version"}},
		{"version-absent", "description: d\n", []string{"yass.preamble.incomplete"}},
		{"description-int", "description: 123\nversion: v1\n", []string{"yass.preamble.value_type"}},
		{"description-null", "description:\nversion: v1\n", []string{"yass.preamble.value_type"}},
		{"description-list", "description: [d]\nversion: v1\n", []string{"yass.preamble.value_type"}},
		{"description-absent", "version: v1\n", []string{"yass.preamble.incomplete"}},
		{"both-absent", "related: []\n", []string{"yass.preamble.incomplete"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			write(t, dir, "pre.yass.yaml", modeline+"---\n"+c.body)
			codes := validateCodes(t, dir, "pre.yass.yaml")
			if strings.Join(codes, " ") != strings.Join(c.want, " ") {
				t.Fatalf("codes = %v, want %v", codes, c.want)
			}
		})
	}
}

func TestValidateFindingsStatus(t *testing.T) {
	dir := project(t)
	write(t, dir, "bad.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: .X\nINPUT:\n- MUST: x\n")
	got := run(dir, "validate")
	wantStatus(t, got, ExitFindings)
}

func TestValidateOrdering(t *testing.T) {
	dir := project(t)
	write(t, dir, "zz.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: .A\nOUTPUT:\n- MUST: x\n")
	got := run(dir, "validate")
	prev := ""
	for _, l := range lines(got.stdout) {
		f := strings.Split(l, "\t")
		path := locationPath(f[2])
		if path < prev {
			t.Fatalf("diagnostics out of path order: %q after %q", path, prev)
		}
		prev = path
	}
}

func TestValidateAcceptsAnyBasenameForAFilePath(t *testing.T) {
	dir := project(t)
	write(t, dir, "notes.txt", "---\ndescription: d\nversion: V2\n")
	got := run(dir, "validate", "notes.txt")
	wantStatus(t, got, ExitFindings)
	if !strings.Contains(got.stdout, "yass.preamble.version") {
		t.Fatalf("a PATH naming a file is taken whatever its basename:\n%s", got.stdout)
	}
	// list, by contrast, requires the .yass.yaml suffix on a file PATH.
	l := run(dir, "list", "notes.txt")
	wantStatus(t, l, ExitUsage)
}

func TestValidateReportsNothingOutsideAddressedSet(t *testing.T) {
	dir := project(t)
	write(t, dir, "bad.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: .X\nINPUT:\n- MUST: x\n")
	got := run(dir, "validate", "a.yass.yaml")
	wantStatus(t, got, ExitOK)
	if got.stdout != "" {
		t.Fatalf("a violation outside the addressed set must not be reported:\n%s", got.stdout)
	}
}

// --- lint -------------------------------------------------------------------

func TestLintCleanProject(t *testing.T) {
	dir := project(t)
	got := run(dir, "lint")
	// a@Note is bound by nothing and reached by nothing.
	wantStatus(t, got, ExitFindings)
	codes := map[string]bool{}
	for _, l := range lines(got.stdout) {
		f := strings.Split(l, "\t")
		if f[0] != "warn" {
			t.Fatalf("lint severity = %q, want warn", f[0])
		}
		codes[f[1]] = true
	}
	if !codes["yass.lint.design_unreferenced"] || !codes["yass.lint.unreachable"] {
		t.Fatalf("codes = %v\n%s", codes, got.stdout)
	}
}

func TestLintModelineAndEmptySlots(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", "---\ndescription: d\nversion: v1\n---\nspec: R\nINPUT: []\nRETURN:\n- SEE: E\n---\nspec: E\n")
	got := run(dir, "lint")
	wantStatus(t, got, ExitFindings)
	codes := map[string]bool{}
	for _, l := range lines(got.stdout) {
		codes[strings.Split(l, "\t")[1]] = true
	}
	for _, want := range []string{"yass.lint.modeline", "yass.lint.slot_empty", "yass.lint.spec_empty"} {
		if !codes[want] {
			t.Errorf("missing %s in %v\n%s", want, codes, got.stdout)
		}
	}
}

func TestLintUnparsed(t *testing.T) {
	dir := project(t)
	write(t, dir, "bad.yass.yaml", "a: b\n  c: d\n- x\n")
	got := run(dir, "lint")
	wantStatus(t, got, ExitFindings)
	if !strings.Contains(got.stdout, "yass.lint.unparsed") {
		t.Fatalf("got:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, "yass.lint.design_unreferenced") {
		t.Fatalf("every other advisory must still be emitted:\n%s", got.stdout)
	}
}

func TestLintPathSelectsReportingOnly(t *testing.T) {
	dir := project(t)
	got := run(dir, "lint", "a.yass.yaml")
	for _, l := range lines(got.stdout) {
		if !strings.HasPrefix(strings.Split(l, "\t")[2], "a") {
			t.Fatalf("advisory outside the selected file: %s", l)
		}
	}
	// The graph is still built from the whole project, so root@Proj's USES of
	// root@Policy keeps a@... reachable decisions correct.
	if strings.Contains(got.stdout, "root@Policy") {
		t.Fatalf("root@Policy is not selected:\n%s", got.stdout)
	}
}

// A root file with no spec supplies no origin for reachability, which lint
// reports as an advisory on standard output.
func TestLintRootSpecCount(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n")
	got := run(dir, "lint")
	wantStatus(t, got, ExitFindings)
	if want := "warn\tyass.lint.no_origin\troot\t-\troot file supplies no single named spec\n"; got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
	if got.stderr != "" {
		t.Fatalf("stderr = %q, want empty", got.stderr)
	}
}

// A root file that fails to parse supplies no origin for reachability, so lint
// reports both the parse failure and the missing origin on standard output.
func TestLintRootUnparsed(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"a: b\n  c: d\n- x\n")
	got := run(dir, "lint")
	wantStatus(t, got, ExitFindings)
	for _, code := range []string{"yass.lint.unparsed", "yass.lint.no_origin"} {
		if !strings.Contains(got.stdout, "\t"+code+"\t") {
			t.Errorf("missing %s:\n%s", code, got.stdout)
		}
	}
	if got.stderr != "" {
		t.Fatalf("stderr = %q, want empty", got.stderr)
	}
}

// A PATH restricts which files' advisories lint reports but never suppresses
// the missing-origin advisory, which concerns the whole graph.
func TestLintNoOriginWithPath(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n")
	write(t, dir, "a.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n")
	got := run(dir, "lint", "a.yass.yaml")
	wantStatus(t, got, ExitFindings)
	if !strings.Contains(got.stdout, "warn\tyass.lint.no_origin\troot\t-\troot file supplies no single named spec\n") {
		t.Fatalf("stdout = %q", got.stdout)
	}
	if got.stderr != "" {
		t.Fatalf("stderr = %q, want empty", got.stderr)
	}
}

// A root file whose one spec has no name supplies no origin for reachability,
// which lint reports rather than passing the project in silence.
func TestLintRootSpecUnnamed(t *testing.T) {
	for name, key := range map[string]string{
		"missing":    "spec:",
		"empty":      `spec: ""`,
		"not-string": "spec: 5",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\n"+key+"\nINPUT:\n- MUST: x\n")
			got := run(dir, "lint")
			wantStatus(t, got, ExitFindings)
			if want := "warn\tyass.lint.no_origin\troot\t6\troot spec has no name a reference could address\n"; got.stdout != want {
				t.Fatalf("stdout = %q, want %q", got.stdout, want)
			}
			if got.stderr != "" {
				t.Fatalf("stderr = %q, want empty", got.stderr)
			}
		})
	}
}

// With no origin, every advisory that does not depend on reachability is
// still written, sorted with the no-origin advisory, and none that does is
// guessed at.
func TestLintNoOriginKeepsOtherAdvisories(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec:\nINPUT:\n- MUST: x\n  SEE: a@A\n")
	write(t, dir, "a.yass.yaml", "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n---\nspec: Orphan\nINPUT:\n- MUST: y\n")
	got := run(dir, "lint")
	wantStatus(t, got, ExitFindings)
	if got.stderr != "" {
		t.Fatalf("stderr = %q, want empty", got.stderr)
	}
	want := "warn\tyass.lint.modeline\ta\t1\tfirst line is not the schema modeline\n" +
		"warn\tyass.lint.no_origin\troot\t6\troot spec has no name a reference could address\n"
	if got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
}

func TestLintDoesNotCrossProjectRoot(t *testing.T) {
	dir := project(t)
	got := run(dir, "lint")
	for _, l := range lines(got.stdout) {
		if strings.Contains(l, "nested") {
			t.Fatalf("lint crossed a project root boundary: %s", l)
		}
	}
}

// lintAt runs lint over a one-file project whose root spec carries body, and
// returns each advisory of code as "LOCATION:LINE".
func lintAt(t *testing.T, description, body, code string) []string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: "+description+"\nversion: v1\n---\nspec: R\n"+body)
	got := run(dir, "lint")
	var out []string
	for _, l := range lines(got.stdout) {
		f := strings.Split(l, "\t")
		if f[1] == code {
			out = append(out, f[2]+":"+f[3])
		}
	}
	return out
}

func TestLintOpenQuestionMarkers(t *testing.T) {
	body := `INPUT:
- MUST: accept a thing # TODO pick the format
- MUST: emit TBD
- MUST: yield an Open Question
- MUST: "mention ` + "`TODO`" + ` and ` + "`??`" + ` as text"
- MUST: keep TODOS and xxx alone
RETURN:
- MUST: return a ?? b
# FIXME later
`
	got := lintAt(t, "d", body, "yass.lint.open_question")
	want := []string{"root@R::INPUT:8", "root@R::INPUT:9", "root@R::INPUT:10", "root@R::RETURN:14", "root@R::RETURN:15"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLintOpenQuestionInUnparsedFile(t *testing.T) {
	dir := project(t)
	write(t, dir, "bad.yass.yaml", "a: b\n  c: d # TBD\n")
	got := run(dir, "lint")
	if !strings.Contains(got.stdout, "yass.lint.open_question\tbad\t2\t") {
		t.Fatalf("marker in an unparsed file not reported:\n%s", got.stdout)
	}
}

func TestLintOpenQuestionMark(t *testing.T) {
	body := `INPUT:
- MUST: accept a thing?
- WHEN: is it empty? maybe
  MUST: reject it
- MUST: "match ` + "`a?b`" + ` and ` + "`?`" + `"
- MUST: match a?b as a glob
`
	got := lintAt(t, "is this the project?", body, "yass.lint.open_question")
	want := []string{"root:3", "root@R::INPUT:8", "root@R::INPUT:9"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLintProseKeyword(t *testing.T) {
	body := `INPUT:
- MUST: call the handler, when the queue is full
- MUST: If the queue is empty, wait
- MUST: retry and MUST-NOT give up
- MUST: emit a record USES root@Policy
- WHEN: the queue is full and MAY drain
  MUST: wait
- MUST: accept a flag even when it begins with a dash
- MUST: "set the ` + "`WHEN`" + ` key and quote ` + "`USES a@B`" + `"
- MUST: report the error, then stop; when done, exit
- MUST: emit a REF in the form root@Policy fixes
- MUST: mail ops at ` + "`a@B`" + `
`
	got := lintAt(t, "d", body, "yass.lint.prose_keyword")
	want := []string{"root@R::INPUT:8", "root@R::INPUT:9", "root@R::INPUT:10", "root@R::INPUT:11", "root@R::INPUT:12", "root@R::INPUT:16", "root@R::INPUT:17"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLintObligationShape(t *testing.T) {
	body := `INPUT:
- MUST: accept ` + strings.Repeat("a long clause ", 29) + `
- MUST: accept ` + strings.Repeat("a long clause ", 28) + `
- MUST: set x := 1
- MUST: "return Order{ID: id}"
- WHEN: the store implements interface { Get() }
  MUST: use it
- MUST: write the order and then enqueue the job
- MUST: accept a flag; reject a bare dash
- MUST: accept a flag. Reject a bare dash
- MUST: accept a flag, e.g. Foo, or a bare dash
- MUST: accept a flag (because callers send one)
- MUST: accept a flag (so-called switch)
- MUST: "match ` + "`x := 1; then`" + ` literally"
- MUST: map its errors to 500
ERROR:
- MUST: map every other error to 500
`
	got := lintAt(t, "d", body, "yass.lint.obligation_shape")
	want := []string{"root@R::INPUT:8", "root@R::INPUT:10", "root@R::INPUT:11", "root@R::INPUT:12",
		"root@R::INPUT:14", "root@R::INPUT:15", "root@R::INPUT:16", "root@R::INPUT:18", "root@R::INPUT:21"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestLintProseMessages pins the message of each prose advisory case below and
// checks that no advisory message those cases produce holds a "; " clause.
func TestLintProseMessages(t *testing.T) {
	tests := []struct {
		name, description, body, code, want string
	}{
		{"open question on description", "is this the project?", "INPUT:\n- MUST: accept a thing\n",
			"yass.lint.open_question", "description asks a question"},
		{"open question on obligation", "d", "INPUT:\n- MUST: accept a thing?\n",
			"yass.lint.open_question", "obligation asks a question"},
		{"open question marker", "d", "INPUT:\n- MUST: accept a thing # TODO pick it\n",
			"yass.lint.open_question", "open question or placeholder"},
		{"prose condition", "d", "INPUT:\n- MUST: call the handler, when the queue is full\n",
			"yass.lint.prose_keyword", "condition written into prose"},
		{"prose keyword", "d", "INPUT:\n- MUST: retry and MUST-NOT give up\n",
			"yass.lint.prose_keyword", "keyword written into prose"},
		{"prose reference", "d", "INPUT:\n- MUST: emit a record USES root@Policy\n",
			"yass.lint.prose_keyword", "reference written into prose"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "root.yass.yaml", modeline+"---\ndescription: "+tt.description+"\nversion: v1\n---\nspec: R\n"+tt.body)
			got := run(dir, "lint")
			var msgs []string
			for _, l := range lines(got.stdout) {
				f := strings.Split(l, "\t")
				if strings.Contains(f[len(f)-1], "; ") {
					t.Errorf("message holds a remedy clause: %s", l)
				}
				if f[1] == tt.code {
					msgs = append(msgs, f[len(f)-1])
				}
			}
			if len(msgs) != 1 || msgs[0] != tt.want {
				t.Fatalf("got %q, want [%q]\n%s", msgs, tt.want, got.stdout)
			}
		})
	}
}

// --- cross-cutting ----------------------------------------------------------

func TestNorwayWordsStayStrings(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: R\nINPUT:\n- MUST: yes\n- MUST: 'no'\n- MUST: on\n- MUST: off\n")
	got := run(dir, "query", "root@R")
	wantStatus(t, got, ExitOK)
	for _, w := range []string{"yes", "no", "on", "off"} {
		if !strings.Contains(got.stdout, "- MUST: "+w+"\n") && !strings.Contains(got.stdout, `- MUST: "`+w+`"`) {
			t.Fatalf("%q did not survive as a string:\n%s", w, got.stdout)
		}
	}
	v := run(dir, "validate")
	wantStatus(t, v, ExitOK)
}

func TestDeterminism(t *testing.T) {
	dir := project(t)
	for _, args := range [][]string{
		{"list"}, {"find", "a"}, {"refs", "root@Policy"},
		{"validate"}, {"lint"}, {"query", "a@Alpha"}, {"root"},
	} {
		first := run(dir, args...)
		for i := 0; i < 3; i++ {
			again := run(dir, args...)
			if again.stdout != first.stdout || again.stderr != first.stderr || again.status != first.status {
				t.Fatalf("%v is not byte-identical across runs", args)
			}
		}
	}
}

func TestEveryLineEndsWithExactlyOneLF(t *testing.T) {
	dir := project(t)
	for _, args := range [][]string{{"list"}, {"find", "a"}, {"validate"}, {"lint"}, {"root"}} {
		got := run(dir, args...)
		if got.stdout == "" {
			continue
		}
		if !strings.HasSuffix(got.stdout, "\n") || strings.HasSuffix(got.stdout, "\n\n") {
			t.Fatalf("%v: bad termination %q", args, got.stdout)
		}
		for _, l := range lines(got.stdout) {
			if l == "" {
				t.Fatalf("%v: blank line in record output", args)
			}
			if strings.ContainsAny(l, "\r") {
				t.Fatalf("%v: CR in a record", args)
			}
		}
	}
}

func TestRecordFieldCounts(t *testing.T) {
	dir := project(t)
	checks := []struct {
		args []string
		n    int
	}{
		{[]string{"root"}, 1},
		{[]string{"list"}, 4},
		{[]string{"find", "a"}, 3},
		{[]string{"refs", "root@Policy"}, 3},
	}
	for _, c := range checks {
		got := run(dir, c.args...)
		for _, l := range lines(got.stdout) {
			if n := len(strings.Split(l, "\t")); n != c.n {
				t.Fatalf("%v: %d fields, want %d: %q", c.args, n, c.n, l)
			}
			if strings.Contains(l, "\t\t") {
				t.Fatalf("%v: two adjacent TABs: %q", c.args, l)
			}
		}
	}
}

func TestSymlinkPolicy(t *testing.T) {
	dir := project(t)
	real := realpath(t, dir)
	if err := os.Symlink(filepath.Join(real, "a.yass.yaml"), filepath.Join(real, "link.yass.yaml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(real, "sub"), filepath.Join(real, "loop")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	got := run(dir, "list")
	if !strings.Contains(got.stdout, "link\tfile\t") {
		t.Fatalf("a link naming a regular file is a member:\n%s", got.stdout)
	}
	for _, l := range lines(got.stdout) {
		if strings.HasPrefix(l, "loop/") {
			t.Fatalf("the traversal descended through a directory symlink: %s", l)
		}
	}
}

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"*", "anything/at/all", true},
		{"a*", "a", true},
		{"a?c", "abc", true},
		{"a?c", "ac", false},
		{"s*b", "sub/b", true},
		{"*@*", "a@Alpha", true},
		{"a", "ab", false},
		{"", "", true},
		{"", "a", false},
		{"*a*b*", "xaybz", true},
	}
	for _, c := range cases {
		if got := globMatch(c.pattern, c.s); got != c.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", c.pattern, c.s, got, c.want)
		}
	}
}

func TestFieldCollapsing(t *testing.T) {
	cases := [][2]string{
		{"  a  b  ", "a b"},
		{"", "-"},
		{"   ", "-"},
		{"a\tb\nc", "a b c"},
		{"a", "a"},
	}
	for _, c := range cases {
		if got := field(c[0]); got != c[1] {
			t.Errorf("field(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

func TestParseTargetGrammar(t *testing.T) {
	bad := []string{"a@Al pha", `a\b@X`, "a@X::lower", "a@X::", "@X", "a@", "a@X::A B"}
	for _, s := range bad {
		if _, ok := parseTarget(s, true); ok {
			t.Errorf("parseTarget(%q) accepted a malformed target", s)
		}
	}
	good := []string{"a@X", "a/b@X::INPUT", "../up@X", "./here@X", "a.b@X.y", "plain"}
	for _, s := range good {
		if _, ok := parseTarget(s, true); !ok {
			t.Errorf("parseTarget(%q) rejected a well-formed target", s)
		}
	}
}

func TestQueryOutputParsesBackAsYAML(t *testing.T) {
	dir := project(t)
	got := run(dir, "query", "a@Alpha", "root@Proj", "root@Policy", "a")
	wantStatus(t, got, ExitOK)
	f := parseBytes("mem", "mem"+SpecSuffix, []byte(got.stdout))
	if !f.Parsed {
		t.Fatalf("emitted fragment does not parse back: %s\n%s", f.FailCode, got.stdout)
	}
}

func TestUnreadableFileIsAnEnvironmentError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits do not deny access")
	}
	dir := project(t)
	p := filepath.Join(dir, "locked.yass.yaml")
	write(t, dir, "locked.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n")
	if err := os.Chmod(p, 0); err != nil {
		t.Skip(err)
	}
	got := run(dir, "list")
	wantStatus(t, got, ExitEnvironment)
	if !strings.Contains(got.stderr, "yass.io.unreadable") {
		t.Fatalf("stderr = %q", got.stderr)
	}
}

// --- overview ---------------------------------------------------------------

// tinyProject is a project whose file and document counts are known exactly:
// one root file holding one spec and one design, plus one other file holding
// one spec.
func tinyProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+`---
description: a one-line summary of what this project is
version: v1
---
spec: Proj
INPUT:
- MUST: accept a thing
  SEE: a@Alpha
INVARIANT:
- USES: Policy
---
design: Policy
type: constraint
content: |
  one
`)
	write(t, dir, "a.yass.yaml", modeline+`---
description: the alpha file
version: v1
---
spec: Alpha
RETURN:
- MUST: yield a thing
`)
	return dir
}

func TestOverviewReportsTheProject(t *testing.T) {
	dir := tinyProject(t)
	got := run(dir, "overview")
	if got.status != ExitOK {
		t.Fatalf("status = %d, want %d", got.status, ExitOK)
	}
	want := []string{
		"project    ",
		"           a one-line summary of what this project is",
		"holds      2 spec files, 3 documents",
	}
	for _, w := range want {
		if !strings.Contains(got.stdout, w) {
			t.Fatalf("block is missing %q:\n%s", w, got.stdout)
		}
	}
}

// The absence of a project root is a fact the block reports, not a failure: the
// no-root status is never selected, and every other section is written.
func TestOverviewWithoutProjectRoot(t *testing.T) {
	got := run(t.TempDir(), "overview")
	if got.status != ExitOK {
		t.Fatalf("status = %d, want %d", got.status, ExitOK)
	}
	if got.stderr != "" {
		t.Fatalf("stderr not empty: %q", got.stderr)
	}
	if !strings.Contains(got.stdout, "project    none — no root.yass.yaml at or above the starting directory") {
		t.Fatalf("missing the absent-root line:\n%s", got.stdout)
	}
	for _, w := range []string{"Yet Another Spec Syntax", "SIDE-EFFECT", "yass docs", "yass --help"} {
		if !strings.Contains(got.stdout, w) {
			t.Fatalf("section carrying %q was dropped:\n%s", w, got.stdout)
		}
	}
	if strings.Contains(got.stdout, "holds") {
		t.Fatalf("counts written with no project root:\n%s", got.stdout)
	}
}

// Orientation is not a check of the spec set: an unparsed file is counted among
// the files, its documents are left out, and no diagnostic and no findings
// status follow from it.
func TestOverviewIgnoresAnUnparsedFile(t *testing.T) {
	dir := tinyProject(t)
	write(t, dir, "broken.yass.yaml", modeline+"---\n: : :\n")
	got := run(dir, "overview")
	if got.status != ExitOK {
		t.Fatalf("status = %d, want %d", got.status, ExitOK)
	}
	if got.stderr != "" {
		t.Fatalf("stderr not empty: %q", got.stderr)
	}
	if !strings.Contains(got.stdout, "holds      3 spec files, 3 documents") {
		t.Fatalf("unparsed file was not counted among the files only:\n%s", got.stdout)
	}
}

func TestOverviewWithoutARootDescription(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+`---
version: v1
---
spec: Proj
INPUT:
- MUST: accept a thing
`)
	got := run(dir, "overview")
	if got.status != ExitOK {
		t.Fatalf("status = %d, want %d", got.status, ExitOK)
	}
	if got.stderr != "" {
		t.Fatalf("stderr not empty: %q", got.stderr)
	}
	body := lines(got.stdout)
	for i, l := range body {
		if strings.HasPrefix(l, "project    ") && i+1 < len(body) {
			if next := body[i+1]; !strings.HasPrefix(next, "holds      ") {
				t.Fatalf("a description line was written for a root with none: %q", next)
			}
		}
	}
}

// The block's fixed text is written to a width, so it survives a narrow
// terminal without reflow. The project line reports the root path — a datum
// whose length the spec deliberately does not bound — so it is exempt from
// the check. Width is counted in Unicode scalar values, not bytes.
func TestOverviewBlockWidth(t *testing.T) {
	for _, dir := range []string{tinyProject(t), t.TempDir()} {
		for _, l := range lines(run(dir, "overview").stdout) {
			if strings.HasPrefix(l, "project    ") && filepath.IsAbs(l[overviewLabel:]) {
				continue
			}
			if n := utf8.RuneCountInString(l); n > overviewWidth {
				t.Fatalf("line is %d columns, over %d: %q", n, overviewWidth, l)
			}
		}
	}
}

func TestOverviewTakesNoOperandOrOption(t *testing.T) {
	dir := tinyProject(t)
	for _, tc := range []struct {
		args []string
		code string
	}{
		{[]string{"overview", "extra"}, "yass.args.extra_operand"},
		{[]string{"overview", "--raw"}, "yass.args.unknown_option"},
	} {
		got := run(dir, tc.args...)
		if got.status != ExitUsage {
			t.Fatalf("%v: status = %d, want %d", tc.args, got.status, ExitUsage)
		}
		if !strings.Contains(got.stderr, tc.code) {
			t.Fatalf("%v: stderr = %q, want %s", tc.args, got.stderr, tc.code)
		}
		if got.stdout != "" {
			t.Fatalf("%v: stdout not empty: %q", tc.args, got.stdout)
		}
	}
}

func TestOverviewIsByteIdentical(t *testing.T) {
	dir := tinyProject(t)
	if a, b := run(dir, "overview"), run(dir, "overview"); a.stdout != b.stdout {
		t.Fatal("two runs over one tree differ")
	}
}

// --- docs -------------------------------------------------------------------

func TestDocsIndexesTheCorpus(t *testing.T) {
	got := run(t.TempDir(), "docs")
	if got.status != ExitOK {
		t.Fatalf("status = %d, want %d", got.status, ExitOK)
	}
	rows := lines(got.stdout)
	if len(rows) != len(Corpus) {
		t.Fatalf("got %d records, want %d:\n%s", len(rows), len(Corpus), got.stdout)
	}
	for i, row := range rows {
		fields := strings.Split(row, "\t")
		if len(fields) != 4 {
			t.Fatalf("record %d has %d fields, want 4: %q", i, len(fields), row)
		}
		if fields[0] != Corpus[i].Name {
			t.Fatalf("record %d names %q, want %q", i, fields[0], Corpus[i].Name)
		}
		if n, err := strconv.Atoi(fields[1]); err != nil || n <= 0 {
			t.Fatalf("record %d has a non-positive line count %q", i, fields[1])
		}
	}
}

func TestDocsWritesADocumentWhole(t *testing.T) {
	for _, d := range Corpus {
		want, err := d.text()
		if err != nil {
			t.Fatal(err)
		}
		got := run(t.TempDir(), "docs", d.Name)
		if got.status != ExitOK {
			t.Fatalf("%s: status = %d, want %d", d.Name, got.status, ExitOK)
		}
		if countLines(want) > LineBudget {
			want = lengthLine(d.Name, want) + want
		}
		if got.stdout != want {
			t.Fatalf("%s: served text is not the carried text byte-for-byte", d.Name)
		}
		if got.stderr != "" {
			t.Fatalf("%s: stderr not empty: %q", d.Name, got.stderr)
		}
	}
}

// A name resolves only by an exact match, and the diagnostic does not enumerate
// the corpus: `yass docs` is the index, and the reader recovers from it.
func TestDocsUnknownName(t *testing.T) {
	for _, name := range []string{"nope", "REFERENCE", "ref", "reference.md"} {
		got := run(t.TempDir(), "docs", name)
		if got.status != ExitUnresolved {
			t.Fatalf("%s: status = %d, want %d", name, got.status, ExitUnresolved)
		}
		if got.stdout != "" {
			t.Fatalf("%s: stdout not empty: %q", name, got.stdout)
		}
		if !strings.Contains(got.stderr, "yass.docs.unknown") {
			t.Fatalf("%s: stderr = %q", name, got.stderr)
		}
		for _, d := range Corpus {
			if name != d.Name && strings.Contains(got.stderr, "\t"+d.Name) {
				t.Fatalf("%s: diagnostic enumerates the corpus: %q", name, got.stderr)
			}
		}
	}
}

// The corpus is a property of the program, not of a tree, so it is served from
// a directory no project root governs.
func TestDocsNeedsNoProjectRoot(t *testing.T) {
	for _, args := range [][]string{{"docs"}, {"docs", "authoring"}} {
		got := run(t.TempDir(), args...)
		if got.status != ExitOK {
			t.Fatalf("%v: status = %d, want %d", args, got.status, ExitOK)
		}
	}
}

// Every document-form answer fits the line budget a reader commonly cuts its
// read at; a carried document too long for it opens with a computed length
// line. Only the language definition, a spec file, is that long.
func TestDocumentAnswersFitTheLineBudget(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: "+strings.Repeat("word ", 400)+"\nversion: v1\n---\nspec: R\n")
	for _, args := range [][]string{{}, {"--help"}} {
		if n := countLines(run(dir, args...).stdout); n > LineBudget {
			t.Errorf("%v writes %d lines, over the %d-line budget", args, n, LineBudget)
		}
	}
	for _, d := range Corpus {
		text, err := d.text()
		if err != nil {
			t.Fatal(err)
		}
		n := countLines(text)
		got := run(t.TempDir(), "docs", d.Name).stdout
		if d.Name == "language" {
			want := "# yass docs language: " + strconv.Itoa(n) + " lines total."
			if n <= LineBudget || !strings.HasPrefix(got, want) {
				t.Errorf("language: first line %q, want prefix %q", strings.SplitN(got, "\n", 2)[0], want)
			}
			continue
		}
		if n > LineBudget {
			t.Errorf("%s holds %d lines, over the %d-line budget: divide it by topic", d.Name, n, LineBudget)
		}
		if strings.HasPrefix(got, "# yass docs ") {
			t.Errorf("%s: a document within the budget carries a length line", d.Name)
		}
	}
}

// Every markdown document names its reading path on its first lines and the
// next document on its last, and every document of a path is in the corpus.
func TestCorpusDocumentsNameTheirPath(t *testing.T) {
	for _, d := range Corpus {
		if !strings.HasSuffix(d.File, ".md") {
			continue
		}
		text, _ := d.text()
		ls := lines(strings.TrimRight(text, "\n"))
		if len(ls) < 3 || !strings.Contains(ls[2], " path") {
			t.Errorf("%s: third line does not name its reading path: %q", d.Name, ls[2])
		}
		last := ls[len(ls)-1]
		if !strings.HasPrefix(last, "Next in the ") && last != "End of the write path." && last != "End of the implement path." {
			t.Errorf("%s: last line does not name the next document: %q", d.Name, last)
		}
		if i := strings.Index(last, "`yass docs "); i >= 0 {
			next := strings.TrimSuffix(strings.TrimPrefix(last[i:], "`yass docs "), "`.")
			if _, ok := corpusDoc(next); !ok {
				t.Errorf("%s: next document %q is not in the corpus", d.Name, next)
			}
		}
	}
}

// No carried document holds a Markdown table: an agent copies the shape of
// what it reads into the specs it writes.
func TestCorpusCarriesNoTable(t *testing.T) {
	for _, d := range Corpus {
		text, err := d.text()
		if err != nil {
			t.Fatal(err)
		}
		for i, l := range strings.Split(text, "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), "|") {
				t.Errorf("%s:%d holds a Markdown table row: %q", d.Name, i+1, l)
			}
		}
	}
}

func TestDocsTakesAtMostOneName(t *testing.T) {
	got := run(t.TempDir(), "docs", "reference", "guidance")
	if got.status != ExitUsage {
		t.Fatalf("status = %d, want %d", got.status, ExitUsage)
	}
	if !strings.Contains(got.stderr, "yass.args.extra_operand") {
		t.Fatalf("stderr = %q", got.stderr)
	}
}

// The carried copies are built from the checkout by script/sync-docs. When the
// checkout is present, each must still match the source it was taken from;
// when it is not — a binary unpacked on its own — there is nothing to compare.
func TestCorpusInSyncWithCheckout(t *testing.T) {
	for _, d := range Corpus {
		src := filepath.Join("..", "..", "..", filepath.FromSlash(d.Source))
		want, err := os.ReadFile(src)
		if err != nil {
			t.Skipf("checkout not present beside the package: %v", err)
		}
		got, err := d.text()
		if err != nil {
			t.Fatal(err)
		}
		if got != string(want) {
			t.Fatalf("%s has drifted from %s; run script/sync-docs", d.File, d.Source)
		}
	}
}

// Every recognized subcommand is named by the synopsis, so --help never omits
// one that dispatch accepts.
func TestSynopsisNamesEverySubcommand(t *testing.T) {
	var o, e bytes.Buffer
	if st := Main([]string{"--help"}, &o, &e); st != ExitOK {
		t.Fatalf("status = %d, want %d", st, ExitOK)
	}
	for _, name := range Subcommands {
		if !strings.Contains(o.String(), "\n  "+name) {
			t.Fatalf("synopsis does not name subcommand %q:\n%s", name, o.String())
		}
	}
}

// --- contract reconciliation ------------------------------------------------

// The Synopsis design block is the exact text --help writes: nothing is added
// to it and nothing is left out. The block is checked against the live
// implementation whenever the checkout that holds the CLI contract is present.
func TestHelpIsTheSynopsisDesignBlock(t *testing.T) {
	src := filepath.Join("..", "..", "00-invocation.yass.yaml")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("cli contract not present beside the package: %v", err)
	}
	var content string
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
			continue
		}
		m := doc.Content[0]
		if m.Kind != yaml.MappingNode {
			continue
		}
		var design, body string
		for i := 0; i+1 < len(m.Content); i += 2 {
			k, v := m.Content[i], m.Content[i+1]
			switch k.Value {
			case "design":
				design = v.Value
			case "content":
				body = v.Value
			}
		}
		if design == "Synopsis" {
			content = body
			break
		}
	}
	if content == "" {
		t.Fatal("00-invocation.yass.yaml holds no design: Synopsis")
	}
	var o, e bytes.Buffer
	if st := Main([]string{"--help"}, &o, &e); st != ExitOK {
		t.Fatalf("status = %d, want %d", st, ExitOK)
	}
	if o.String() != content {
		t.Fatalf("--help has drifted from the Synopsis design block:\nhelp:\n%q\nblock:\n%q", o.String(), content)
	}
}

// A REF is written root-relative so it can be passed back to `yass query`
// unchanged. That holds for a file under the project root from any starting
// directory. A file outside the root is still spelled root-relative, with
// `../` segments, but a later invocation resolves a leading `./` or `../` token
// from its own starting directory — so the spelling re-addresses the same
// document only when the starting directory is the project root, and a
// misleading path inside the root is taken instead when one exists.
func TestEmittedRefRoundTripOutsideRoot(t *testing.T) {
	base := t.TempDir()
	write(t, base, "other.yass.yaml", modeline+"---\ndescription: the outside file\nversion: v1\n---\nspec: Doc\nINPUT:\n- MUST: x\n")
	write(t, base, "project/root.yass.yaml", modeline+"---\ndescription: the project\nversion: v1\n---\nspec: P\nINPUT:\n- MUST: x\n")
	write(t, base, "project/sub/keep.yass.yaml", modeline+"---\ndescription: the inside file\nversion: v1\n---\nspec: Keep\nINPUT:\n- MUST: y\n")
	root := filepath.Join(base, "project")
	sub := filepath.Join(root, "sub")

	// The emitted REF is the project-root-relative spelling.
	outside := filepath.Join("..", "..", "other.yass.yaml")
	got := run(sub, "list", outside)
	wantStatus(t, got, ExitOK)
	if !strings.Contains(got.stdout, "../other@Doc") {
		t.Fatalf("list did not emit a root-relative REF for the outside file: %q", got.stdout)
	}

	// From the project root the spelling round-trips.
	fromRoot := run(root, "query", "../other@Doc")
	wantStatus(t, fromRoot, ExitOK)
	if !strings.Contains(fromRoot.stdout, "spec: Doc") {
		t.Fatalf("from the project root, the emitted REF must re-address the outside file: %q", fromRoot.stdout)
	}

	// An in-root REF round-trips from any starting directory.
	fromSub := run(sub, "query", "sub/keep@Keep")
	wantStatus(t, fromSub, ExitOK)
	if !strings.Contains(fromSub.stdout, "spec: Keep") {
		t.Fatalf("an in-root REF must re-address from a subdirectory: %q", fromSub.stdout)
	}

	// From a subdirectory with no misleading path, the outside spelling does
	// not silently reach some other file: it names nothing.
	miss := run(sub, "query", "../other@Doc")
	wantStatus(t, miss, ExitUnresolved)
	if miss.stdout != "" {
		t.Fatalf("a spelling that addresses nothing must emit nothing: %q", miss.stdout)
	}

	// With a misleading inside-root path present, the same spelling takes
	// that file instead — which is why the round-trip is not promised for
	// files outside the root. The outside file's document carries `MUST: x`;
	// the misleading one carries `MUST: z`.
	write(t, base, "project/other.yass.yaml", modeline+"---\ndescription: the misleading file\nversion: v1\n---\nspec: Doc\nINPUT:\n- MUST: z\n")
	mis := run(sub, "query", "../other@Doc")
	wantStatus(t, mis, ExitOK)
	if !strings.Contains(mis.stdout, "MUST: z") {
		t.Fatalf("want the misleading inside-root file, got %q", mis.stdout)
	}
	if strings.Contains(mis.stdout, "MUST: x") {
		t.Fatalf("the outside file must not be reached from a subdirectory: %q", mis.stdout)
	}
}

// Go's SIGPIPE disposition is process-level: a write to a closed standard
// output must select the ok status the output policy names rather than ending
// the process with a signal outside the exit-status table. Main alone does not
// exercise it, so the real binary is run against a pipe with no reader.
func TestProcessClosedStdoutPipeExitsZero(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "yass")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = filepath.Join("..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	r.Close()

	cmd := exec.Command(bin, "-C", project(t), "list")
	cmd.Stdout = w
	cmd.Stderr = new(bytes.Buffer)
	runErr := cmd.Run()
	w.Close()
	code := 0
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	} else if runErr != nil {
		t.Fatal(runErr)
	}
	if code != 0 {
		t.Fatalf("closed stdout pipe exited %d, want 0", code)
	}
}

// --- design types -----------------------------------------------------------

// catalogProject adds a schema design and a catalog design, each bound by USES,
// to the fixture project.
func catalogProject(t *testing.T) string {
	t.Helper()
	dir := project(t)
	write(t, dir, "c.yass.yaml", modeline+`---
description: designs of every shape
version: v1
---
spec: C
INPUT:
- MUST: report each code the catalogue lists
  USES: Codes
- MUST: write the wire shape
  USES: Wire
---
design: Wire
type: schema
format: json-schema
content: |
  {"type": "object"}
---
design: Codes
type: catalog
content:
  yass.io.missing: {severity: error, exit: 3, meaning: the named file does not exist}
  "odd: key": {severity: warn, exit: 1, meaning: a key that must be quoted}
  0: {severity: error, exit: 0, meaning: a numeric identifier}
`)
	return dir
}

func TestValidateAcceptsEveryDesignType(t *testing.T) {
	dir := catalogProject(t)
	for _, typ := range []string{"constraint", "algorithm", "procedure"} {
		write(t, dir, "t.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: "+typ+"\ncontent: |\n  c\n")
		if codes := validateCodes(t, dir, "t.yass.yaml"); len(codes) != 0 {
			t.Fatalf("%s: codes = %v, want none", typ, codes)
		}
	}
	if codes := validateCodes(t, dir, "c.yass.yaml"); len(codes) != 0 {
		t.Fatalf("schema and catalog: codes = %v, want none", codes)
	}
}

// A repeated catalogue identifier is reported by name, never dropped.
func TestValidateNamesADuplicateCatalogKey(t *testing.T) {
	dir := project(t)
	write(t, dir, "d.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: catalog\ncontent:\n  yass.io.missing: {f: 1}\n  yass.io.missing: {f: 2}\n")
	got := run(dir, "validate", "d.yass.yaml")
	want := "error\tyass.document.duplicate_key\td@D\t10\tmapping carries duplicate key yass.io.missing\n"
	if got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
	q := run(dir, "query", "d@D")
	wantStatus(t, q, ExitOK)
	if strings.Count(q.stdout, "yass.io.missing:") != 2 {
		t.Fatalf("query dropped a repeated record:\n%s", q.stdout)
	}
}

func TestQueryEmitsACatalogAsAuthored(t *testing.T) {
	dir := catalogProject(t)
	got := run(dir, "query", "c@Codes")
	wantStatus(t, got, ExitOK)
	want := `---
design: Codes
type: catalog
content:
  yass.io.missing:
    severity: error
    exit: 3
    meaning: the named file does not exist
  "odd: key":
    severity: warn
    exit: 1
    meaning: a key that must be quoted
  0:
    severity: error
    exit: 0
    meaning: a numeric identifier
`
	if got.stdout != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got.stdout, want)
	}
	var back map[string]any
	if err := yaml.Unmarshal([]byte(got.stdout), &back); err != nil {
		t.Fatalf("emitted catalog does not read back: %v", err)
	}
	w := run(dir, "query", "c@Wire")
	if !strings.Contains(w.stdout, "type: schema\nformat: json-schema\n") {
		t.Fatalf("schema format not emitted:\n%s", w.stdout)
	}
}

// find searches a catalogue's identifiers and field values, never its field
// names, and answers with the whole record on one line.
func TestFindSearchesCatalogRecords(t *testing.T) {
	dir := catalogProject(t)
	got := run(dir, "find", "does not exist")
	wantStatus(t, got, ExitOK)
	want := "c@Codes\tcontent\tyass.io.missing: {severity: error, exit: 3, meaning: the named file does not exist}\n"
	if got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
	if got := run(dir, "find", "io.miss"); got.stdout != want {
		t.Fatalf("identifier search: stdout = %q", got.stdout)
	}
	if got := run(dir, "find", "severity"); got.stdout != "" {
		t.Fatalf("field names are not searched: %q", got.stdout)
	}
	if got := run(dir, "find", "error"); strings.Count(got.stdout, "c@Codes") != 2 {
		t.Fatalf("one record per matching record: %q", got.stdout)
	}
}

// --- design-content lint ----------------------------------------------------

// designLint writes one design of the given type and content, bound by the
// fixture's root spec, and returns the lint records naming it.
func designLint(t *testing.T, typ, content string) []string {
	t.Helper()
	dir := project(t)
	format := "# no format\n"
	if typ == "schema" {
		format = "format: text\n"
	}
	body := "content: |\n"
	for _, l := range strings.Split(content, "\n") {
		body += "  " + l + "\n"
	}
	write(t, dir, "z.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\ndesign: Z\ntype: "+typ+"\n"+format+body)
	got := run(dir, "lint", "z.yass.yaml")
	var out []string
	for _, l := range lines(got.stdout) {
		if f := strings.Split(l, "\t"); len(f) == 5 && (f[1] == "yass.lint.design_code" || f[1] == "yass.lint.design_table") {
			out = append(out, f[1]+"\t"+f[3])
		}
	}
	return out
}

func TestLintDesignCode(t *testing.T) {
	for _, c := range []struct{ name, typ, content string }{
		{"go-qualified-call", "algorithm", `cfg := huma.DefaultConfig("Service Name", version.Version)`},
		{"go-map-literal", "schema", `cfg.Formats = map[string]huma.Format{"json": huma.DefaultJSONFormat}`},
		{"go-slice-literal", "schema", `cfg.Servers = []*huma.Server{{URL: "http://localhost:8080"}}`},
		{"go-func", "algorithm", "func Resolve(t Target) (*Doc, error) {"},
		{"go-method", "algorithm", "func (l *Loader) Load(abs string) (*File, error) {"},
		{"go-err-check", "algorithm", "if err != nil {"},
		{"python-def", "algorithm", "def resolve(target, root):"},
		{"js-function", "algorithm", "function resolve(target) {"},
		{"rust-fn", "algorithm", "pub fn resolve(target: &str) -> Doc {"},
		{"c-assignment", "algorithm", "count = count + 1;"},
		{"c-call", "algorithm", "printf(\"%d\", count);"},
		{"c-if-brace", "algorithm", "if (count > 0) {"},
		{"else-brace", "algorithm", "} else {"},
		{"import", "schema", `import "fmt"`},
		{"python-import", "schema", "from os import path"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := designLint(t, c.typ, c.content)
			if len(got) != 1 || got[0] != "yass.lint.design_code\t10" {
				t.Fatalf("got %v, want one design_code advisory at line 10", got)
			}
		})
	}
}

// Pseudocode is what an algorithm holds, and a schema states data, so neither
// shape is reported.
func TestLintDesignCodeSkipsPseudocodeAndData(t *testing.T) {
	for _, c := range []struct{ name, typ, content string }{
		{"pseudo-assign", "algorithm", "x := x + 1"},
		{"pseudo-arrow", "algorithm", "best ← max(best, score)"},
		{"pseudo-for-each", "algorithm", "for each node in graph:\n  visit(node)"},
		{"pseudo-if-then", "algorithm", "if count > 0 then\n  return count\nend if"},
		{"pseudo-while", "algorithm", "while queue is not empty:\n  node ← pop(queue)"},
		{"pseudo-function", "algorithm", "function Search(tree, key)\n  return Search(tree.left, key)"},
		{"pseudo-call", "algorithm", "queue.push(start)\nreturn dist[target]"},
		{"pseudo-member-assign", "algorithm", "node.visited = true"},
		{"json-schema", "schema", `{"type": "object", "required": ["name"],` + "\n" + ` "properties": {"name": {"type": "string"}}}`},
		{"yaml-payload", "schema", "name: alice\nroles:\n  - admin"},
		{"wire-header", "schema", "Content-Type: text/plain; charset=utf-8;"},
		{"usage-text", "schema", "usage: yass [-C DIR] [--version] [SUBCOMMAND [ARGS...]]"},
		{"backtick-mention", "schema", "the field is never `cfg := huma.DefaultConfig(name)`"},
		{"url", "schema", "see https://example.com/api for the endpoint"},
		{"constraint-source", "constraint", `cfg := huma.DefaultConfig("Service Name", version.Version)`},
		{"procedure-source", "procedure", "if err != nil {"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := designLint(t, c.typ, c.content); len(got) != 0 {
				t.Fatalf("got %v, want no advisory", got)
			}
		})
	}
}

func TestLintDesignTable(t *testing.T) {
	for _, typ := range []string{"constraint", "algorithm", "procedure", "schema"} {
		got := designLint(t, typ, "Flags per command.\n\n| Command | Flag |\n|---------|------|\n| serve   | `--listen` |")
		if len(got) != 1 || got[0] != "yass.lint.design_table\t12" {
			t.Fatalf("%s: got %v, want one design_table advisory at line 12", typ, got)
		}
	}
	for _, c := range []string{
		"a | b",
		"usage: yass [--in | --out]\n--- separator ---",
		"a|b\nplain prose",
		"no pipe\n|---|---|",
	} {
		if got := designLint(t, "constraint", c); len(got) != 0 {
			t.Fatalf("%q: got %v, want no advisory", c, got)
		}
	}
}

// --- restatement lint ---------------------------------------------------------

// restateCatalogs is the fixture vocabulary and code catalogue of the
// restatement lint, after the catalogue experiments of issue #120: an
// ExitStatus catalogue keyed by status, and a Diagnostics catalogue whose exit
// field takes its values from ExitStatus.
const restateCatalogs = `---
design: ExitStatus
type: catalog
content:
  0: {label: ok, behaviour: the command completed and reported nothing}
  1: {label: findings, behaviour: the command reported at least one finding}
  2: {label: usage, behaviour: the invocation was malformed}
  3: {label: absent, behaviour: the request names something that does not exist}
  4: {label: conflict, behaviour: two sources the command reads disagree}
  5: {label: environment, behaviour: the system refused an operation the command needed}
---
design: Diagnostics
type: catalog
content:
  yass.io.file_unreadable: {severity: error, owner: resolve, exit: 5}
  yass.ref.target_ambiguous: {severity: error, owner: resolve, exit: 4}
  yass.foo.operand_malformed: {severity: error, owner: foo, exit: 2}
  yass.foo.target_absent: {severity: error, owner: foo, exit: 3}
  yass.diff.pair_none: {severity: warning, owner: diff, exit: 1}
`

// restateLint lints one obligation, written as a YAML sequence item of the root
// spec's INPUT slot, beside the fixture catalogues, and returns each
// restatement advisory as "CODE: MESSAGE".
func restateLint(t *testing.T, obligation string) []string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: R\nINPUT:\n"+obligation+
		"\n- USES: ExitStatus\n- USES: Diagnostics\n"+restateCatalogs)
	got := run(dir, "lint")
	var out []string
	for _, l := range lines(got.stdout) {
		if f := strings.Split(l, "\t"); len(f) == 5 && strings.HasPrefix(f[1], "yass.lint.restated_") {
			if f[3] != "8" {
				t.Fatalf("advisory at line %s, want 8: %s", f[3], l)
			}
			out = append(out, f[1]+": "+f[4])
		}
	}
	return out
}

func TestLintRestatedFact(t *testing.T) {
	for _, c := range []struct{ name, obligation, want string }{
		// The round 4 plants: a conflicting status by number and by label.
		{"number-conflict", "- MUST: report yass.io.file_unreadable and exit with status 3",
			"yass.lint.restated_fact: obligation states 3 for field exit of yass.io.file_unreadable, which catalog Diagnostics records as 5"},
		{"label-conflict", "- MUST: report yass.ref.target_ambiguous and exit with the absent status",
			"yass.lint.restated_fact: obligation states absent for field exit of yass.ref.target_ambiguous, which catalog Diagnostics records as 4"},
		// The round 3 plant: a behaviour borrowed from another case.
		{"comparison", "- MUST: report yass.io.file_unreadable and fail as when the file does not exist",
			"yass.lint.restated_fact: obligation borrows another case's outcome for yass.io.file_unreadable, which catalog Diagnostics records"},
		// Restatements authors wrote: each agrees with the catalogue.
		{"label-agrees", "- MUST: report yass.foo.operand_malformed and exit with the usage status",
			"yass.lint.restated_fact: obligation restates field exit of yass.foo.operand_malformed, which catalog Diagnostics records"},
		{"number-and-severity", "- MUST: report yass.foo.target_absent at severity error and exit with status 3",
			"yass.lint.restated_fact: obligation restates field severity of yass.foo.target_absent, which catalog Diagnostics records"},
		{"behaviour", "- MUST: reject the invocation as malformed and report yass.foo.operand_malformed",
			"yass.lint.restated_fact: obligation restates field exit of yass.foo.operand_malformed, which catalog Diagnostics records"},
		{"behaviour-in-guard", "- WHEN: the invocation is malformed\n  MUST: report yass.foo.operand_malformed",
			"yass.lint.restated_fact: obligation restates field exit of yass.foo.operand_malformed, which catalog Diagnostics records"},
		{"quoted-identifier", "- MUST: \"report `yass.diff.pair_none` and exit with the findings status\"",
			"yass.lint.restated_fact: obligation restates field exit of yass.diff.pair_none, which catalog Diagnostics records"},
		// Two identifiers, each with its own value: neither value conflicts.
		{"two-identifiers", "- MUST: report yass.foo.operand_malformed with status 2 and yass.foo.target_absent with status 3",
			"yass.lint.restated_fact: obligation restates field exit of yass.foo.operand_malformed, which catalog Diagnostics records"},
		// A content word in another inflection: names and named meet.
		{"inflected-content-word", "- MUST: report yass.foo.target_absent when something the request named does not exist",
			"yass.lint.restated_fact: obligation restates field exit of yass.foo.target_absent, which catalog Diagnostics records"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := restateLint(t, c.obligation)
			if len(got) != 1 || got[0] != c.want {
				t.Fatalf("got %q, want [%q]", got, c.want)
			}
		})
	}
}

func TestLintRestatedValue(t *testing.T) {
	for _, c := range []struct{ name, obligation, want string }{
		{"behaviour", "- WHEN: OPERAND is malformed\n  MUST: reject the invocation as malformed",
			"yass.lint.restated_value: obligation states the invocation was malformed, a value of field exit in catalog Diagnostics, without naming an identifier"},
		{"label", "- MUST: reject a malformed operand with the usage status",
			"yass.lint.restated_value: obligation states usage, a value of field exit in catalog Diagnostics, without naming an identifier"},
		{"number", "- WHEN: OPERAND names nothing\n  MUST: exit with status 3",
			"yass.lint.restated_value: obligation states 3, a value of field exit in catalog Diagnostics, without naming an identifier"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := restateLint(t, c.obligation)
			if len(got) != 1 || got[0] != c.want {
				t.Fatalf("got %q, want [%q]", got, c.want)
			}
		})
	}
}

// An obligation that names the code and binds the catalogue states no value,
// and neither does a value written as an ordinary word.
func TestLintRestatementQuiet(t *testing.T) {
	for _, c := range []struct{ name, obligation string }{
		{"binding", "- MUST: report each code the Diagnostics catalogue assigns to owner `foo`, at the severity and exit status it gives\n  USES: Diagnostics"},
		{"code-and-condition", "- WHEN: OPERAND is malformed\n  MUST: report yass.foo.operand_malformed"},
		{"condition-near-behaviour", "- WHEN: OPERAND names something that does not exist\n  MUST: report yass.foo.target_absent"},
		{"code-named-in-a-word", "- MUST: forward the yass.diff.pair_none findings of load unchanged"},
		{"code-quantity", "- MUST: report yass.io.file_unreadable naming the 1-based line and the 3 files read"},
		{"value-no-record-holds", "- MUST: exit with the ok status once the line is written"},
		{"value-as-a-word", "- MUST: be usable with no environment variable"},
		{"number-as-a-quantity", "- MUST: accept 2 operands"},
		{"bound-to-vocabulary", "- MUST-NOT: select status 4 from foo, since foo reads one source\n  USES: ExitStatus"},
		// Each identifier is one word of the context window: usage stands six
		// words from exit.
		{"window-counts-identifiers", "- MUST: exit after yass.io.file_unreadable yass.foo.target_absent and yass.diff.pair_none usage"},
		{"digit-key-names-nothing", "- WHEN: a run returns status 1\n  MUST: print the findings status line\n  USES: ExitStatus"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := restateLint(t, c.obligation); len(got) != 0 {
				t.Fatalf("got %q, want no advisory", got)
			}
		})
	}
}

// A field whose values are keys of no other catalogue is matched only as its
// own values, and a value-only obligation is never reported for it.
func TestLintRestatementFreeField(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+`---
description: d
version: v1
---
spec: R
INPUT:
- MUST: log the event at level warn
- MUST: log app.start at level warn
- MUST: log app.stop when the level is warn
- USES: Events
---
design: Events
type: catalog
content:
  app.start: {level: info}
  app.stop: {level: warn}
`)
	got := run(dir, "lint")
	var out []string
	for _, l := range lines(got.stdout) {
		if f := strings.Split(l, "\t"); len(f) == 5 && strings.HasPrefix(f[1], "yass.lint.restated_") {
			out = append(out, f[1]+"\t"+f[3]+"\t"+f[4])
		}
	}
	want := []string{
		"yass.lint.restated_fact\t9\tobligation states warn for field level of app.start, which catalog Events records as info",
		"yass.lint.restated_fact\t10\tobligation restates field level of app.stop, which catalog Events records",
	}
	if strings.Join(out, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(out, "\n"), strings.Join(want, "\n"))
	}
}

// A wording matched as written and one matched by its content words are
// distinct even when their words coincide.
func TestRestateWordingSame(t *testing.T) {
	if (wording{phrase: "file path"}).same(wording{content: []string{"file", "path"}}) {
		t.Fatal("phrase and content wordings compared equal")
	}
	if !(wording{phrase: "usage"}).same(wording{phrase: "usage"}) {
		t.Fatal("equal phrase wordings compared unequal")
	}
}

// A value stated for an identifier whose record lacks the field is no
// conflict with a recorded value.
func TestRestatedFactMissingField(t *testing.T) {
	c := &catalogue{doc: &Doc{Name: "C"}, keys: []string{"yass.a.one", "yass.a.two"},
		recs: map[string]map[string]string{"yass.a.one": {"exit": "usage"}, "yass.a.two": {}}}
	c.fields = []*fieldDomain{{name: "exit"}}
	c.fields[0].build(c, []*catalogue{c})
	if got := restatedFact("report yass.a.two and exit usage", []*catalogue{c}); got != "" {
		t.Fatalf("got %q, want no advisory", got)
	}
	if got := restatedFact("report yass.a.one and exit usage", []*catalogue{c}); got == "" {
		t.Fatal("want an advisory for the record that holds the field")
	}
}

// --- --format json ------------------------------------------------------------

// jsonProject is a one-file project whose description carries the characters
// HTML escaping would rewrite, and non-ASCII text JSON could escape.
func jsonProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+`---
description: a <b> & "c" café
version: v1
---
spec: R
INPUT:
- MUST: "take <x> & y \\ z \x01"
  USES: D
---
design: D
type: constraint
content: |
  one
`)
	return dir
}

func TestFormatJSONRoot(t *testing.T) {
	dir := jsonProject(t)
	got := run(dir, "root", "--format", "json")
	wantStatus(t, got, ExitOK)
	want := `{"path":"` + realpath(t, dir) + `"}` + "\n"
	if got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
}

func TestFormatJSONList(t *testing.T) {
	dir := jsonProject(t)
	got := run(dir, "list", "--format=json")
	wantStatus(t, got, ExitOK)
	want := `{"ref":"root","kind":"file","detail":"2","description":"a <b> & \"c\" café"}` + "\n" +
		`{"ref":"root@R","kind":"spec","detail":"INPUT","description":null}` + "\n" +
		`{"ref":"root@D","kind":"design","detail":"constraint","description":null}` + "\n"
	if got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
	// The text form is unchanged, and naming it explicitly selects it.
	text := run(dir, "list")
	explicit := run(dir, "list", "--format", "text")
	if explicit.stdout != text.stdout || !strings.HasPrefix(text.stdout, "root\tfile\t2\t") {
		t.Fatalf("text form changed: %q vs %q", explicit.stdout, text.stdout)
	}
}

func TestFormatJSONFind(t *testing.T) {
	dir := jsonProject(t)
	got := run(dir, "find", "--format", "json", "x>")
	wantStatus(t, got, ExitOK)
	want := `{"ref":"root@R::INPUT","what":"MUST","text":"take <x> & y \\ z \u0001"}` + "\n"
	if got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
	empty := run(dir, "find", "--format", "json", "absent text")
	wantStatus(t, empty, ExitOK)
	if empty.stdout != "" {
		t.Fatalf("an empty result must write zero bytes: %q", empty.stdout)
	}
}

func TestFormatJSONRefs(t *testing.T) {
	dir := project(t)
	got := run(dir, "refs", "--format", "json", "--out", "a@Alpha::INPUT")
	wantStatus(t, got, ExitOK)
	want := `{"dir":"out","relation":"CONFORMS","ref":"a@Beta::RETURN"}` + "\n" +
		`{"dir":"out","relation":"CONFORMS","ref":"sub/b@Beta::RETURN"}` + "\n"
	if got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
}

func TestFormatJSONValidate(t *testing.T) {
	dir := project(t)
	write(t, dir, "bad.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: .X\nINPUT:\n- MUST: x\n")
	text := run(dir, "validate")
	got := run(dir, "validate", "--format", "json")
	wantStatus(t, got, ExitFindings)
	tl, jl := lines(text.stdout), lines(got.stdout)
	if len(tl) == 0 || len(tl) != len(jl) {
		t.Fatalf("text and json record counts differ:\n%s\n%s", text.stdout, got.stdout)
	}
	for i, l := range tl {
		f := strings.Split(l, "\t")
		line := f[3]
		if line == "-" {
			line = "null"
		}
		loc := `"` + f[2] + `"`
		if f[2] == "-" {
			loc = "null"
		}
		want := `{"severity":"` + f[0] + `","code":"` + f[1] + `","location":` + loc +
			`,"line":` + line + `,"message":"` + f[4] + `"}`
		if jl[i] != want {
			t.Fatalf("record %d = %q, want %q", i, jl[i], want)
		}
	}
	if !strings.Contains(got.stdout, `"line":6,`) {
		t.Fatalf("line must be a JSON integer: %s", got.stdout)
	}
}

func TestFormatJSONLint(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", "---\ndescription: d\nversion: v1\n---\nspec: R\nINPUT: []\n")
	got := run(dir, "lint", "--format", "json")
	wantStatus(t, got, ExitFindings)
	want := `{"severity":"warn","code":"yass.lint.modeline","location":"root","line":1,"message":"first line is not the schema modeline"}` + "\n" +
		`{"severity":"warn","code":"yass.lint.slot_empty","location":"root@R::INPUT","line":6,"message":"slot declares no obligation"}` + "\n"
	if got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
	clean := run(project(t), "lint", "--format", "json", "sub/b.yass.yaml")
	wantStatus(t, clean, ExitOK)
	if clean.stdout != "" {
		t.Fatalf("a clean lint must write zero bytes: %q", clean.stdout)
	}
}

// Standard error stays text under every format.
func TestFormatJSONLeavesStandardErrorText(t *testing.T) {
	dir := project(t)
	missing := run(dir, "lint", "--format", "json", "nope.yass.yaml")
	wantStatus(t, missing, ExitUnresolved)
	if missing.stdout != "" || missing.stderr != "error\tyass.io.missing\tnope.yass.yaml\t-\tpath does not exist\n" {
		t.Fatalf("stdout = %q stderr = %q", missing.stdout, missing.stderr)
	}
	write(t, dir, "sub/broken.yass.yaml", modeline+"a: b\n  c: d\n- x\n")
	got := run(dir, "refs", "--format", "json", "--out", "root@Proj")
	wantStatus(t, got, ExitFindings)
	if !strings.HasPrefix(got.stderr, "error\tyass.document.not_yaml\tsub/broken\t") {
		t.Fatalf("stderr = %q", got.stderr)
	}
	if !strings.Contains(got.stdout, `{"dir":"out","relation":"USES","ref":"root@Policy"}`) {
		t.Fatalf("stdout = %q", got.stdout)
	}
}

func TestFormatArgumentErrors(t *testing.T) {
	dir := project(t)
	for _, args := range [][]string{
		{"list", "--format", "xml"},
		{"root", "--format=JSON"},
		{"lint", "--format="},
	} {
		got := run(dir, args...)
		wantStatus(t, got, ExitUsage)
		if got.stdout != "" || !strings.HasPrefix(got.stderr, "error\tyass.args.unknown_format\t-\t-\tunknown format") {
			t.Errorf("%v: stdout = %q stderr = %q", args, got.stdout, got.stderr)
		}
	}
	for _, args := range [][]string{
		{"query", "--format", "json", "a@Alpha"},
		{"docs", "--format", "json"},
		{"overview", "--format", "json"},
		{"--format", "json", "list"},
	} {
		got := run(dir, args...)
		wantStatus(t, got, ExitUsage)
		if !strings.Contains(got.stderr, "yass.args.unknown_option") {
			t.Errorf("%v: stderr = %q", args, got.stderr)
		}
	}
	got := run(dir, "list", "--format")
	wantStatus(t, got, ExitUsage)
	if !strings.Contains(got.stderr, "yass.args.missing_value") {
		t.Fatalf("stderr = %q", got.stderr)
	}
}

// An unknown line and an absent location are JSON null, and a field's value is
// the collapsed text the text form writes.
func TestFormatJSONDiagnosticNulls(t *testing.T) {
	a := &App{Inv: &Invocation{Format: FormatJSON}}
	got := a.diag(Diag{Severity: "error", Code: "yass.x.y", Message: "two\t words  here"})
	want := `{"severity":"error","code":"yass.x.y","location":null,"line":null,"message":"two words here"}`
	if got != want {
		t.Fatalf("diag = %q, want %q", got, want)
	}
	text := &App{Inv: &Invocation{}}
	if text.diag(Diag{Severity: "error", Code: "yass.x.y", Message: "m"}) != "error\tyass.x.y\t-\t-\tm" {
		t.Fatal("text form changed")
	}
}

// A file that does not parse keeps its list record, its unknown fields null.
func TestFormatJSONListUnparsedFileIsNull(t *testing.T) {
	dir := project(t)
	write(t, dir, "bad.yass.yaml", "a: b\n  c: d\n- x\n")
	got := run(dir, "list", "--format", "json", "bad.yass.yaml")
	wantStatus(t, got, ExitFindings)
	if got.stdout != `{"ref":"bad","kind":"file","detail":null,"description":null}`+"\n" {
		t.Fatalf("stdout = %q", got.stdout)
	}
}

// The JSON form keeps a real "-" apart from an absent field, which the text
// form cannot.
func TestFormatJSONKeepsRealDash(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: \"-\"\nversion: v1\n---\nspec: R\nINPUT:\n- MUST: x\n")
	write(t, dir, "a.yass.yaml", modeline+"---\ndescription: \"\"\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: y\n")
	got := run(dir, "list", "--format", "json", "--filter", "*")
	wantStatus(t, got, ExitOK)
	if !strings.Contains(got.stdout, `{"ref":"root","kind":"file","detail":"1","description":"-"}`) {
		t.Fatalf("a real - must stay a string:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, `{"ref":"a","kind":"file","detail":"1","description":null}`) {
		t.Fatalf("an empty description must be null:\n%s", got.stdout)
	}
}

// An unknown format names the value quoted, so an empty or padded value
// is still visible.
func TestFormatUnknownNamesValueQuoted(t *testing.T) {
	got := run(project(t), "list", "--format=")
	wantStatus(t, got, ExitUsage)
	if !strings.Contains(got.stderr, "yass.args.unknown_format") || !strings.Contains(got.stderr, `unknown format ""`) {
		t.Fatalf("stderr = %q", got.stderr)
	}
}
