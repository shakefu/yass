package yass

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// cited writes a citation as a test in a `//`-comment language carries it. It
// is assembled from its parts: a citation spelled whole in this file would be a
// citation of this file, and these name a fixture rather than this spec set.
func citeComment(token string) string {
	return "// " + citationMarker + " " + token
}

// citedProject builds citeProject's spec set with test files citing it, and
// with files no test is named as that carry broken citations: the checker must
// read the first and never the second. pkg holds them all, so a PATH can name
// the subtree.
func citedProject(t *testing.T) string {
	t.Helper()
	dir := citeProject(t)
	write(t, dir, "pkg/a_test.go", "package pkg\n\n"+citeComment(startToken)+"\nfunc TestStart(t *testing.T) {}\n")
	write(t, dir, "pkg/b_test.py", "def test_thing():\n    # "+citationMarker+" "+thingToken+"\n    pass\n")
	write(t, dir, "pkg/inner/x.spec.ts", citeComment(copyToken)+"\n"+citeComment(goodsToken)+"\n")
	write(t, dir, "pkg/notes.md", "See "+citationMarker+" nowhere@Spec::INPUT[1]#00000000 in the prose.\n")
	write(t, dir, "pkg/latest.go", "package pkg\n\n"+citeComment("nowhere@Spec::INPUT[1]#00000000")+"\n")
	return dir
}

// wantCitations fails unless `yass citations args...` run in dir yields the
// given status and exactly the want records on the stream says.
func wantCitations(t *testing.T, dir string, status int, want []string, args ...string) {
	t.Helper()
	got := run(dir, append([]string{"citations"}, args...)...)
	wantStatus(t, got, status)
	if got.stdout != strings.Join(want, "\n")+"\n" && (len(want) > 0 || got.stdout != "") {
		t.Fatalf("citations %v:\nstdout:\n%s\nstderr:\n%s\nwant:\n%s", args, got.stdout, got.stderr, strings.Join(want, "\n"))
	}
	if got.stderr != "" {
		t.Fatalf("citations %v: stderr = %q, want empty", args, got.stderr)
	}
}

// broken writes one test file holding the citations given, and runs the
// checker over that file alone. It returns the records it wrote.
func broken(t *testing.T, tokens ...string) []string {
	t.Helper()
	dir := citedProject(t)
	var body strings.Builder
	body.WriteString("package pkg\n\n")
	for _, token := range tokens {
		body.WriteString(citeComment(token) + "\n")
	}
	write(t, dir, "pkg/z_test.go", body.String())
	got := run(dir, "citations", filepath.Join("pkg", "z_test.go"))
	wantStatus(t, got, ExitFindings)
	if got.stderr != "" {
		t.Fatalf("stderr = %q, want empty", got.stderr)
	}
	return lines(got.stdout)
}

// yass: 15-citations@Citations::INPUT[5]#cf3ef670
// yass: 15-citations@Citations::RETURN[1]#088aef8c
// yass: 15-citations@Citations::RETURN[12]#aa6b19e6
// yass: 15-citations@Citations::INVARIANT[2]#759e3c86
func TestCitationsChecksTheFilesNamedAsTests(t *testing.T) {
	// Three test files cite four obligations and nothing is wrong, while a
	// document and a source file that no test is named as carry broken
	// citations the checker never reads.
	wantCitations(t, citedProject(t), ExitOK, nil)
	// A file with no citation at all is still a scope that was walked.
	wantCitations(t, citeProject(t), ExitOK, nil)
}

// yass: 15-citations@Citations::RETURN[3]#558c871c
// yass: 15-citations@Citations::RETURN[6]#4b1fa431
// yass: 15-citations@Citations::RETURN[8]#3a225a6f
// yass: 15-citations@Citations::RETURN[10]#73489f04
// yass: 15-citations@Citations::RETURN[11]#a2f9f88c
// yass: 15-citations@Citations::SIDE-EFFECT[1]#9008b6e8
func TestCitationsReportsAMalformedCitation(t *testing.T) {
	tokens := []string{
		"a@Alpha::RETURN", "a@Alpha::RETURN[1]", "a@Alpha::RETURN[0]", "a@Alpha::RETURN[01]",
		"a@Alpha::RETURN[2]#309207A", "a@Alpha::RETURN[2]#309207", "a@Alpha::RETURN[2]#309207aa0",
		"a@Alpha[2]#309207aa", "Alpha::RETURN[2]#309207aa", "a@Alpha::INPUTT[1]#309207aa",
		"a@Alpha::RETURN[x]#309207aa", "query",
	}
	got := broken(t, tokens...)
	if len(got) != len(tokens) {
		t.Fatalf("got %d records, want %d:\n%s", len(got), len(tokens), strings.Join(got, "\n"))
	}
	for i, token := range tokens {
		want := "error\tyass.citation.syntax\tpkg/z_test.go\t" + strconv.Itoa(3+i) + "\tcitation is malformed"
		if got[i] != want {
			t.Errorf("%q: got %q, want %q", token, got[i], want)
		}
	}
	// A marker with nothing after it is malformed too.
	if got := broken(t, ""); len(got) != 1 || got[0] != "error\tyass.citation.syntax\tpkg/z_test.go\t3\tcitation is malformed" {
		t.Fatalf("empty token: got %v", got)
	}
}

// yass: 14-cite@CitationTarget::RETURN[10]#52cd00aa
// yass: 15-citations@Citations::RETURN[14]#564f5823
// yass: 15-citations@Citations::RETURN[15]#42c1d377
func TestCitationsListsNoDocumentATargetCannotAddress(t *testing.T) {
	dir := citedProject(t)
	// A spec under a path holding a space, and one of two documents sharing a
	// name, can be cited by no target. A citation written for their
	// obligations would not resolve once pasted, so neither report names them.
	write(t, dir, "user flows/signup.yass.yaml", modeline+"---\ndescription: s\nversion: v1\n---\nspec: Signup\nRETURN:\n- MUST: sign up\n")
	write(t, dir, "dup.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: Twin\nRETURN:\n- MUST: first twin\n---\nspec: Twin\nRETURN:\n- MUST: second twin\n")
	wantCitations(t, dir, ExitOK, []string{
		citedGoods2, citedWrapped, citedSpaced,
		citationMarker + " root@R::INPUT[1]#4ce36fb4\tMUST\tx",
	}, "--missing")
	wantCitations(t, dir, ExitOK, []string{
		"a@Alpha\t3\t3\t0\t0",
		"a@Beta\t2\t1\t1\t0",
		"a@Format\t1\t0\t0\t0",
		"root@R\t1\t0\t0\t0",
		"-\t7\t4\t1\t0",
	}, "--coverage")
}

// yass: 15-citations@Citations::RETURN[17]#efbbdcfc
// yass: 15-citations@Citations::ERROR[6]#44c17eb0
func TestCitationsHoldsFindingsUntilTheScopeIsRead(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission bits do not bind here")
	}
	dir := citedProject(t)
	// A finding is waiting in one file, and a later file cannot be read: the
	// request ends with the environment status and nothing on standard output.
	write(t, dir, "pkg/m_test.go", citeComment("a@Alpha::RETURN[2]#00000000")+"\n")
	shut := write(t, dir, "pkg/z_test.go", citeComment(startToken)+"\n")
	if err := os.Chmod(shut, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(shut, 0o755) })
	got := run(dir, "citations")
	wantStatus(t, got, ExitEnvironment)
	if got.stdout != "" || !strings.Contains(got.stderr, "yass.io.unreadable\tpkg/z_test.go") {
		t.Fatalf("stdout = %q, stderr = %q; want no record and yass.io.unreadable naming the file", got.stdout, got.stderr)
	}
}

// yass: 15-citations@Citations::RETURN[4]#d7ba58b7
// yass: 15-citations@Citations::RETURN[6]#4b1fa431
// yass: 15-citations@Citations::RETURN[9]#f5ceec4e
func TestCitationsReportsAnUnresolvedCitation(t *testing.T) {
	for i, token := range []string{
		"a@Alpha::RETURN[9]#309207aa", "a@Nowhere::RETURN[1]#309207aa", "nowhere@Alpha::RETURN[1]#309207aa",
		"a@Alpha::SIDE-EFFECT[1]#309207aa", "root@Policy::RETURN[1]#309207aa",
	} {
		got := broken(t, token)
		want := "error\tyass.citation.unresolved\tpkg/z_test.go\t3\tcitation " + token + " names no obligation"
		if len(got) != 1 || got[0] != want {
			t.Errorf("case %d: got %q, want %q", i, got, want)
		}
	}
}

// yass: 15-citations@Citations::RETURN[5]#7f69eb01
// yass: 15-citations@Citations::RETURN[6]#4b1fa431
// yass: 15-citations@Citations::RETURN[9]#f5ceec4e
func TestCitationsReportsAStaleCitation(t *testing.T) {
	// The number names an entry, and the entry carries another stamp.
	token := "a@Alpha::RETURN[2]#00000000"
	got := broken(t, token)
	want := "warn\tyass.citation.stale\tpkg/z_test.go\t3\tcitation " + token + " carries a stamp the obligation at that number does not"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// yass: 15-citations@Citations::RETURN[2]#3060665c
// yass: 15-citations@Citations::RETURN[7]#5f080678
func TestCitationsOrdersFindingsByFileThenLine(t *testing.T) {
	dir := citedProject(t)
	write(t, dir, "pkg/m_test.go", citeComment("a@Alpha::RETURN[9]#309207aa")+"\n"+citeComment(copyToken)+"\n")
	write(t, dir, "pkg/a_test.go", "package pkg\n\n"+citeComment(startToken)+"\n"+citeComment("a@Alpha::RETURN")+"\n")
	want := []string{
		"error\tyass.citation.syntax\tpkg/a_test.go\t4\tcitation is malformed",
		"error\tyass.citation.unresolved\tpkg/m_test.go\t1\tcitation a@Alpha::RETURN[9]#309207aa names no obligation",
	}
	wantCitations(t, dir, ExitFindings, want)
}

// yass: 15-citations@Citations::INPUT[9]#5b389dee
func TestCitationsRecognizesAMarkerWhereverItStands(t *testing.T) {
	dir := citedProject(t)
	// After code, after a block-comment opener, with a tab, with no space, and
	// twice on one line.
	write(t, dir, "pkg/m_test.go",
		"package pkg\n\nx := 1 "+citeComment(copyToken)+"\n/* "+citationMarker+" "+startToken+" */\n"+
			"// "+citationMarker+"\t"+thingToken+"\n// "+citationMarker+startToken+" "+citationMarker+" "+copyToken+"\n")
	wantCitations(t, dir, ExitOK, nil, filepath.Join("pkg", "m_test.go"))
	// Text glued to the token's end is part of the token.
	write(t, dir, "pkg/m_test.go", "package pkg\n\n/* "+citationMarker+" "+copyToken+"*/\n")
	wantCitations(t, dir, ExitFindings,
		[]string{"error\tyass.citation.syntax\tpkg/m_test.go\t3\tcitation is malformed"}, filepath.Join("pkg", "m_test.go"))
}

// yass: 15-citations@Citations::INPUT[9]#5b389dee
func TestCitationsIgnoresAMarkerThatIsNotAWholeWord(t *testing.T) {
	dir := citedProject(t)
	write(t, dir, "pkg/m_test.go", "package pkg\n\n// myyass: nowhere@Spec::INPUT[1]#00000000\n// sub_yass: nowhere@Spec::INPUT[1]#00000000\n// Yass9: nowhere\n")
	wantCitations(t, dir, ExitOK, nil, filepath.Join("pkg", "m_test.go"))
}

// yass: 15-citations@Citations::INPUT[9]#5b389dee
func TestCitationsEndsATokenAtACarriageReturn(t *testing.T) {
	dir := citedProject(t)
	write(t, dir, "pkg/m_test.go", "package pkg\r\n\r\n"+citeComment(startToken)+"\r\n")
	wantCitations(t, dir, ExitOK, nil, filepath.Join("pkg", "m_test.go"))
}

// yass: 15-citations@Citations::INPUT[7]#7b800e11
// yass: 15-citations@Citations::INPUT[8]#2b0cf4b4
func TestCitationsScansOnlyWhatPathSelects(t *testing.T) {
	dir := citedProject(t)
	// A file is taken whatever its name, so the document's broken citation
	// is read once it is named, and the test files left unnamed are not.
	wantCitations(t, dir, ExitFindings,
		[]string{"error\tyass.citation.unresolved\tpkg/notes.md\t1\tcitation nowhere@Spec::INPUT[1]#00000000 names no obligation"},
		filepath.Join("pkg", "notes.md"), filepath.Join("pkg", "notes.md"))
	// A directory yields the test files below it and nothing else.
	wantCitations(t, dir, ExitOK, nil, "pkg")
	wantCitations(t, dir, ExitFindings,
		[]string{"error\tyass.citation.unresolved\tpkg/latest.go\t3\tcitation nowhere@Spec::INPUT[1]#00000000 names no obligation"},
		filepath.Join("pkg", "latest.go"))
}

// yass: 15-citations@Citations::INPUT[6]#1c0aa631
func TestCitationsTestsGlobReplacesTheTestFileNames(t *testing.T) {
	dir := citedProject(t)
	// Only the Go files are read: the Python and TypeScript citations are
	// neither checked nor counted, and the Go source file is.
	bad := "error\tyass.citation.unresolved\tpkg/latest.go\t3\tcitation nowhere@Spec::INPUT[1]#00000000 names no obligation"
	wantCitations(t, dir, ExitFindings, []string{bad}, "--tests", "*.go")
	// A glob is matched against the whole project-root-relative path.
	wantCitations(t, dir, ExitOK, nil, "--tests", "pkg/*.py")
	// A directory named as tests holds files no test is named as.
	write(t, dir, "tests/integration.rs", citeComment(startToken)+"\n")
	wantCitations(t, dir, ExitOK, nil, "--tests", "tests/*.rs")
}

// yass: 15-citations@Citations::INPUT[5]#cf3ef670
func TestCitationsSkipsANestedProject(t *testing.T) {
	dir := citedProject(t)
	write(t, dir, "pkg/nested/root.yass.yaml", conformanceRoot)
	write(t, dir, "pkg/nested/n_test.go", citeComment("nowhere@Spec::INPUT[1]#00000000")+"\n")
	wantCitations(t, dir, ExitOK, nil)
}

// yass: 15-citations@Citations::RETURN[14]#564f5823
// yass: 15-citations@Citations::RETURN[12]#aa6b19e6
func TestCitationsMissingListsUncoveredObligations(t *testing.T) {
	// Every record is one `yass cite` would write, in file-set order.
	wantCitations(t, citedProject(t), ExitOK, []string{
		citedGoods2, citedWrapped, citedSpaced,
		citationMarker + " root@R::INPUT[1]#4ce36fb4\tMUST\tx",
	}, "--missing")
	// With no test citing anything, every obligation is uncovered.
	dir := citeProject(t)
	got := run(dir, "citations", "--missing")
	wantStatus(t, got, ExitOK)
	if len(lines(got.stdout)) != 8 {
		t.Fatalf("got %d records, want 8:\n%s", len(lines(got.stdout)), got.stdout)
	}
}

// yass: 15-citations@Citations::RETURN[15]#42c1d377
// yass: 15-citations@Citations::RETURN[13]#473f5d16
func TestCitationsCoverageCountsObligations(t *testing.T) {
	wantCitations(t, citedProject(t), ExitOK, []string{
		"a@Alpha\t3\t3\t0\t0",
		"a@Beta\t2\t1\t1\t0",
		"a@Format\t1\t0\t0\t0",
		"root@R\t1\t0\t0\t0",
		"-\t7\t4\t1\t0",
	}, "--coverage")
	// A project with no numbered obligation reports nothing at all.
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: r\nversion: v1\n---\nspec: R\n")
	wantCitations(t, dir, ExitOK, nil, "--coverage")
}

// yass: 15-citations@Citations::RETURN[13]#473f5d16
func TestCitationsLeavesMayOutOfBothReports(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: r\nversion: v1\n---\nspec: R\n")
	write(t, dir, "a.yass.yaml", modeline+"---\ndescription: a\nversion: v1\n---\nspec: A\nRETURN:\n- MAY: wrap nothing\n- MUST: give back the goods\n- MAY: also wrap nothing\n")
	wantCitations(t, dir, ExitOK, []string{citationMarker + " a@A::RETURN[2]#e6c459d2\tMUST\tgive back the goods"}, "--missing")
	// A citation of a MAY resolves and is checked like any other.
	write(t, dir, "a_test.go", citeComment("a@A::RETURN[1]#39a8682b")+"\n")
	wantCitations(t, dir, ExitOK, nil)
	wantCitations(t, dir, ExitOK, []string{"a@A\t1\t0\t0\t0", "-\t1\t0\t0\t0"}, "--coverage")
}

// yass: 15-citations@Citations::RETURN[16]#79685a1c
// yass: 15-citations@Citations::RETURN[12]#aa6b19e6
func TestCitationsReportsCheckNothing(t *testing.T) {
	dir := citeProject(t)
	// One test file carries a malformed, an unresolved, and a stale citation
	// of three obligations: nothing is covered, and still no diagnostic is
	// written and the status is ok. Each report answers from the citations
	// that carry a matching stamp, and checks none.
	write(t, dir, "pkg/m_test.go", citeComment("a@Alpha::INPUT")+"\n"+citeComment("a@Alpha::RETURN[9]#309207aa")+"\n"+citeComment("a@Alpha::RETURN[2]#00000000")+"\n")
	wantCitations(t, dir, ExitOK, []string{
		citedStart, citedThing, citedCopy, citedGoods, citedGoods2, citedWrapped, citedSpaced,
		citationMarker + " root@R::INPUT[1]#4ce36fb4\tMUST\tx",
	}, "--missing")
	wantCitations(t, dir, ExitOK, []string{
		"a@Alpha\t3\t0\t0\t0", "a@Beta\t2\t0\t1\t0", "a@Format\t1\t0\t0\t0", "root@R\t1\t0\t0\t0", "-\t7\t0\t1\t0",
	}, "--coverage")
}

// yass: 15-citations@Citations::INPUT[10]#6b6c07ec
func TestCitationsResolvesFromTheProjectRoot(t *testing.T) {
	dir := citedProject(t)
	// A path with no leading dot is read from the project root, and one with
	// a leading dot likewise, whatever directory the command starts in.
	write(t, dir, "pkg/m_test.go", citeComment("./pkg/../a@Alpha::RETURN[2]#309207aa")+"\n"+citeComment("./a@Alpha::RETURN[1]#36356b93")+"\n")
	wantCitations(t, dir, ExitOK, nil)
	wantCitations(t, filepath.Join(dir, "pkg", "inner"), ExitOK, nil)
}

// yass: 15-citations@Citations::INPUT[3]#1f61ea3f
// yass: 15-citations@Citations::INPUT[4]#4f07b4d5
func TestCitationsResolvesAgainstTheWholeFileSet(t *testing.T) {
	dir := citedProject(t)
	// The root file lies outside the PATH, and a spec that would not validate
	// still loads: both resolve.
	write(t, dir, "pkg/n.yass.yaml", modeline+"---\ndescription: n\nversion: v1\n---\nspec: N\nRETURN:\n- MUST: [not, a, string]\n- MUST: give back a coat\n")
	write(t, dir, "pkg/m_test.go", citeComment("root@R::INPUT[1]#4ce36fb4")+"\n"+citeComment("pkg/n@N::RETURN[2]#ef36b47e")+"\n")
	wantCitations(t, dir, ExitOK, nil, "pkg")
}

// yass: 15-citations@Citations::ERROR[7]#80c7ab3a
// yass: 15-citations@Citations::SIDE-EFFECT[2]#4bf8aece
func TestCitationsReportsAnUnparsedSpecFileAndGoesOn(t *testing.T) {
	dir := citedProject(t)
	write(t, dir, "pkg/broken.yass.yaml", "spec: [ unclosed\n")
	write(t, dir, "pkg/m_test.go", citeComment("a@Alpha::RETURN")+"\n")
	got := run(dir, "citations")
	wantStatus(t, got, ExitFindings)
	wantErr := "error\tyass.document.not_yaml\tpkg/broken\t"
	if !strings.HasPrefix(got.stderr, wantErr) || strings.Count(got.stderr, "\n") != 1 {
		t.Fatalf("stderr = %q, want one %s diagnostic", got.stderr, wantErr)
	}
	if want := "error\tyass.citation.syntax\tpkg/m_test.go\t1\tcitation is malformed\n"; got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
	// A report is answered too, and still ends with the findings status.
	got = run(dir, "citations", "--coverage")
	wantStatus(t, got, ExitFindings)
	if !strings.HasSuffix(got.stdout, "-\t7\t4\t1\t0\n") || !strings.HasPrefix(got.stderr, wantErr) {
		t.Fatalf("stdout = %q, stderr = %q", got.stdout, got.stderr)
	}
}

// yass: 15-citations@Citations::ERROR[4]#119b21b7
// yass: 15-citations@Citations::ERROR[5]#953fb2f9
// yass: 15-citations@Citations::ERROR[2]#c6976b8e
func TestCitationsReportsABadPathBeforeAnythingIsWritten(t *testing.T) {
	dir := citedProject(t)
	// A broken citation is in scope, and still nothing is written: a PATH that
	// cannot be taken ends the request first.
	for _, tc := range []struct {
		path   string
		status int
		code   string
	}{
		{"pkg/absent_test.go", ExitUnresolved, "yass.io.missing"},
		{filepath.Join(dir, ".."), ExitNoRoot, "yass.root.not_found"},
	} {
		got := run(dir, "citations", tc.path)
		wantStatus(t, got, tc.status)
		if got.stdout != "" || !strings.Contains(got.stderr, tc.code) {
			t.Errorf("%q: status = %d, stdout = %q, stderr = %q; want %s and no record", tc.path, got.status, got.stdout, got.stderr, tc.code)
		}
	}
	// A directory a nearer project root governs is outside the project, and
	// the first PATH is what anchors discovery.
	write(t, dir, "pkg/nested/root.yass.yaml", conformanceRoot)
	write(t, dir, "pkg/nested/n_test.go", citeComment("nowhere@Spec::INPUT[1]#00000000")+"\n")
	got := run(dir, "citations", filepath.Join("pkg", "a_test.go"), filepath.Join("pkg", "nested"))
	wantStatus(t, got, ExitNoRoot)
	if got.stdout != "" || !strings.Contains(got.stderr, "yass.root.outside") {
		t.Fatalf("stdout = %q, stderr = %q; want yass.root.outside and no record", got.stdout, got.stderr)
	}
}

// yass: 15-citations@Citations::ERROR[1]#82a48f5e
// yass: 15-citations@Citations::INPUT[2]#597fdd95
func TestCitationsAnchorsRootDiscoveryOnTheFirstPath(t *testing.T) {
	dir := citedProject(t)
	elsewhere := t.TempDir()
	write(t, elsewhere, "loose_test.go", citeComment("nowhere@Spec::INPUT[1]#00000000")+"\n")
	// From a directory no project governs, a PATH into the project finds its
	// root, and a PATH in that directory finds none.
	got := run(elsewhere, "citations", filepath.Join(dir, "pkg"))
	wantStatus(t, got, ExitOK)
	if got.stdout != "" || got.stderr != "" {
		t.Fatalf("stdout = %q, stderr = %q", got.stdout, got.stderr)
	}
	got = run(elsewhere, "citations", "loose_test.go")
	wantStatus(t, got, ExitNoRoot)
	if got.stdout != "" || !strings.Contains(got.stderr, "yass.root.not_found") {
		t.Fatalf("stdout = %q, stderr = %q; want yass.root.not_found and no record", got.stdout, got.stderr)
	}
}

// yass: 15-citations@Citations::ERROR[6]#44c17eb0
func TestCitationsReportsAFileOfTheScopeItCannotRead(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission bits do not bind here")
	}
	dir := citedProject(t)
	shut := filepath.Join(dir, "pkg", "shut_test.go")
	if err := os.WriteFile(shut, []byte(citeComment(startToken)+"\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(shut, 0o755) })
	got := run(dir, "citations")
	wantStatus(t, got, ExitEnvironment)
	if got.stdout != "" || !strings.Contains(got.stderr, "yass.io.unreadable\tpkg/shut_test.go") {
		t.Fatalf("stdout = %q, stderr = %q; want yass.io.unreadable naming the file", got.stdout, got.stderr)
	}
}

// yass: 15-citations@Citations::INPUT[1]#e3b77de9
func TestCitationsInvocation(t *testing.T) {
	dir := citedProject(t)
	for _, tc := range []struct {
		args []string
		code string
	}{
		{[]string{"citations", "--missing", "--coverage"}, "yass.args.unknown_option\t-\t-\tunknown option --coverage"},
		{[]string{"citations", "--coverage", "--missing"}, "yass.args.unknown_option\t-\t-\tunknown option --missing"},
		{[]string{"citations", "--tests"}, "yass.args.missing_value"},
		{[]string{"citations", "--raw"}, "yass.args.unknown_option"},
		{[]string{"citations", "--format", "yaml"}, "yass.args.unknown_format"},
		{[]string{"citations", ""}, "yass.args.empty"},
	} {
		got := run(dir, tc.args...)
		wantStatus(t, got, ExitUsage)
		if got.stdout != "" || !strings.Contains(got.stderr, tc.code) {
			t.Errorf("%v: status = %d, stdout = %q, stderr = %q; want %s", tc.args, got.status, got.stdout, got.stderr, tc.code)
		}
	}
}

// yass: 15-citations@Citations::RETURN[10]#73489f04
// yass: 15-citations@Citations::RETURN[14]#564f5823
// yass: 15-citations@Citations::RETURN[15]#42c1d377
func TestCitationsRecordForms(t *testing.T) {
	dir := citedProject(t)
	write(t, dir, "pkg/m_test.go", citeComment("a@Alpha::RETURN[2]#00000000")+"\n")
	got := run(dir, "citations", "--format", "json", filepath.Join("pkg", "m_test.go"))
	wantStatus(t, got, ExitFindings)
	if want := `{"severity":"warn","code":"yass.citation.stale","location":"pkg/m_test.go","line":1,"message":"citation a@Alpha::RETURN[2]#00000000 carries a stamp the obligation at that number does not"}` + "\n"; got.stdout != want {
		t.Fatalf("stdout = %q, want %q", got.stdout, want)
	}
	got = run(dir, "citations", "--format=json", "--missing")
	if want := `{"citation":"` + citationMarker + " a@Beta::RETURN[2]#e6c459d2" + `","what":"MUST","text":"give back the goods"}` + "\n"; !strings.HasPrefix(got.stdout, want) || got.stderr != "" {
		t.Fatalf("stdout = %q, want it to open with %q", got.stdout, want)
	}
	got = run(dir, "citations", "--format=json", "--coverage")
	first := `{"ref":"a@Alpha","must":"3","must_cited":"3","should":"0","should_cited":"0"}` + "\n"
	last := `{"ref":"-","must":"7","must_cited":"4","should":"1","should_cited":"0"}` + "\n"
	if !strings.HasPrefix(got.stdout, first) || !strings.HasSuffix(got.stdout, last) || got.stderr != "" {
		t.Fatalf("stdout = %q, want it to open with %q and end with %q", got.stdout, first, last)
	}
	for _, row := range lines(run(dir, "citations", "--coverage").stdout) {
		if n := len(strings.Split(row, "\t")); n != 5 || strings.Contains(row, "\t\t") {
			t.Fatalf("%d fields in %q, want 5 and no adjacent TABs", n, row)
		}
	}
}

// yass: 15-citations@Citations::SIDE-EFFECT[3]#9bb9f0b8
func TestCitationsTouchesNothing(t *testing.T) {
	dir := citedProject(t)
	write(t, dir, "pkg/m_test.go", citeComment("a@Alpha::RETURN[2]#00000000")+"\n")
	before := treeSnapshot(t, dir)
	for _, args := range [][]string{{"citations"}, {"citations", "--missing"}, {"citations", "--coverage"}} {
		run(dir, args...)
	}
	if after := treeSnapshot(t, dir); after != before {
		t.Fatal("citations changed the file tree")
	}
}

// yass: 15-citations@Citations::INPUT[5]#cf3ef670
func TestTestFileNames(t *testing.T) {
	for name, want := range map[string]bool{
		"x_test.go": true, "test_x.py": true, "x.test.ts": true, "XTest.java": true,
		"x_spec.rb": true, "tests.bats": true, "MyTests.cs": true, "a-b-tests.js": true,
		"HTTPServerTest.kt": true, "x.SPEC.tsx": true, "Test.swift": true, "x_TESTS.py": true,
		"FooTest.go": true, "JSONSpec.groovy": true, "xTest2.go": false,
		"latest.go": false, "contest.tsx": false, "testament.py": false, "specification.md": false,
		"xtest.go": false, "test2.go": false, "root.yass.yaml": false, "14-cite.yass.yaml": false,
		"integration.rs": false, "x.go": false, "": false,
	} {
		if got := isTestFileName(name); got != want {
			t.Errorf("%q = %v, want %v", name, got, want)
		}
	}
}
