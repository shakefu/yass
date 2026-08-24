package yass

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
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
type: prose
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
		"a@Note\tdesign\tprose\t-",
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

// --- target resolution ------------------------------------------------------

func TestTargetResolutionErrors(t *testing.T) {
	dir := project(t)
	write(t, dir, "dupe.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: Twice\nINPUT:\n- MUST: x\n---\ndesign: Twice\ntype: t\ncontent: |\n  c\n")
	cases := []struct {
		target string
		code   string
		status int
	}{
		{"a@Alpha::bad", "yass.ref.syntax", ExitUsage},
		{"a@Alpha::NOPE", "yass.ref.slot_unknown", ExitUsage},
		{"a@Al pha", "yass.ref.syntax", ExitUsage},
		{`a\b@Alpha`, "yass.ref.syntax", ExitUsage},
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
		"- MUST: return b",
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
	if !strings.HasPrefix(got.stdout, "---\nspec: Proj\nINVARIANT:\n") {
		t.Fatalf("a slot fragment still states what was queried:\n%s", got.stdout)
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
type: t
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
		{"name-dup", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n---\ndesign: A\ntype: t\ncontent: |\n  c\n", "yass.document.name_duplicate"},
		{"preamble-missing", modeline + "---\nspec: A\nINPUT:\n- MUST: x\n", "yass.preamble.missing"},
		{"preamble-incomplete", modeline + "---\ndescription: d\n", "yass.preamble.incomplete"},
		{"preamble-version", modeline + "---\ndescription: d\nversion: V1\n", "yass.preamble.version"},
		{"related-shape", modeline + "---\ndescription: d\nversion: v1\nrelated: nope\n", "yass.preamble.related_shape"},
		{"spec-unnamed", modeline + "---\ndescription: d\nversion: v1\n---\nspec:\nINPUT:\n- MUST: x\n", "yass.spec.unnamed"},
		{"spec-grammar", modeline + "---\ndescription: d\nversion: v1\n---\nspec: .Bad\nINPUT:\n- MUST: x\n", "yass.spec.name_grammar"},
		{"spec-keyword", modeline + "---\ndescription: d\nversion: v1\n---\nspec: input\nINPUT:\n- MUST: x\n", "yass.spec.name_keyword"},
		{"design-incomplete", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: t\n", "yass.design.incomplete"},
		{"design-value-type", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: 7\ncontent: |\n  c\n", "yass.design.value_type"},
		{"design-unknown-key", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: D\ntype: t\ncontent: |\n  c\nINPUT:\n- MUST: x\n", "yass.design.unknown_key"},
		{"design-keyword", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: MAY\ntype: t\ncontent: |\n  c\n", "yass.design.name_keyword"},
		{"design-unnamed", modeline + "---\ndescription: d\nversion: v1\n---\ndesign:\ntype: t\ncontent: |\n  c\n", "yass.design.unnamed"},
		{"design-grammar", modeline + "---\ndescription: d\nversion: v1\n---\ndesign: .D\ntype: t\ncontent: |\n  c\n", "yass.design.name_grammar"},
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
		{"ref-not-a-string", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n  SEE:\n  - A\n", "yass.ref.not_a_string"},
		{"ref-syntax", modeline + "---\ndescription: d\nversion: v1\n---\nspec: A\nINPUT:\n- MUST: x\n  SEE: \"a b\"\n", "yass.ref.syntax"},
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

func TestValidateRootSpecCount(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n")
	codes := validateCodes(t, dir)
	if !contains(codes, "yass.root.spec_count") {
		t.Fatalf("codes = %v", codes)
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

func TestLintRootSpecCount(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n")
	got := run(dir, "lint")
	wantStatus(t, got, ExitFindings)
	if !strings.Contains(got.stderr, "yass.root.spec_count") {
		t.Fatalf("stderr = %q", got.stderr)
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

// The block is written to a width, so it survives a narrow terminal without
// reflow. Width is counted in Unicode scalar values, not bytes.
func TestOverviewBlockWidth(t *testing.T) {
	for _, dir := range []string{tinyProject(t), t.TempDir()} {
		for _, l := range lines(run(dir, "overview").stdout) {
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
	for _, args := range [][]string{{"docs"}, {"docs", "reference"}} {
		got := run(t.TempDir(), args...)
		if got.status != ExitOK {
			t.Fatalf("%v: status = %d, want %d", args, got.status, ExitOK)
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
