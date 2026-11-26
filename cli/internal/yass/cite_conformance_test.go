package yass

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// citeProject builds a project whose spec file holds the cases the citation
// rules turn on: a reference-only entry that takes no number, a guarded entry
// carrying two relations, a repeated obligation, a spec whose prose collapses,
// and a spec with no numbered entry. RETURN is authored before INPUT in Alpha,
// so a listing in canonical order is told from one in authored order.
func citeProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	citeProjectAt(t, dir)
	return dir
}

// citeProjectAt writes citeProject's files into dir, so a test can place the
// project where it chooses, with the files of other projects beside or in it.
func citeProjectAt(t *testing.T, dir string) {
	t.Helper()
	write(t, dir, "root.yass.yaml", modeline+`---
description: r
version: v1
---
spec: R
INPUT:
- MUST: x
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
- CONFORMS: Beta::RETURN
- MUST: accept a thing
  WHEN: it is raining
  SEE: a@Alpha
  USES: root@Policy
- MUST-NOT: keep a copy
INPUT:
- MUST: begin at the start
---
spec: Beta
RETURN:
- MUST: give back the goods
- MUST: give back the goods
- SHOULD: wrap the goods
---
spec: Format
RETURN:
- MUST: "line one   line two"
---
spec: Empty
INVARIANT: []
`)
}

// Stamps below were computed from 14-cite@ObligationStamp's definition outside
// this program, so they pin the algorithm and not its implementation. A record
// is assembled from its parts: a citation spelled whole in this file would be a
// citation of this file, and these name a fixture rather than this spec set.
const (
	startToken   = "a@Alpha::INPUT[1]#6320b2a3"
	thingToken   = "a@Alpha::RETURN[1]#36356b93"
	copyToken    = "a@Alpha::RETURN[2]#309207aa"
	goodsToken   = "a@Beta::RETURN[1]#e6c459d2"
	spacedToken  = "a@Format::RETURN[1]#8bdedc70"
	thingStamp   = "36356b93"
	citedStart   = citationMarker + " " + startToken + "\tMUST\tbegin at the start"
	citedThing   = citationMarker + " " + thingToken + "\tMUST\taccept a thing"
	citedCopy    = citationMarker + " " + copyToken + "\tMUST-NOT\tkeep a copy"
	citedGoods   = citationMarker + " " + goodsToken + "\tMUST\tgive back the goods"
	citedGoods2  = citationMarker + " a@Beta::RETURN[2]#e6c459d2\tMUST\tgive back the goods"
	citedWrapped = citationMarker + " a@Beta::RETURN[3]#fc753547\tSHOULD\twrap the goods"
	citedSpaced  = citationMarker + " " + spacedToken + "\tMUST\tline one line two"
	citedCoat    = citationMarker + " m@M::RETURN[2]#ef36b47e\tMUST\tgive back a coat"
)

// wantCite fails unless `yass cite args...` run in dir exits ok, writes
// nothing to standard error, and writes exactly the want records.
func wantCite(t *testing.T, dir string, want []string, args ...string) {
	t.Helper()
	got := run(dir, append([]string{"cite"}, args...)...)
	wantStatus(t, got, ExitOK)
	if got.stderr != "" {
		t.Fatalf("stderr = %q, want empty", got.stderr)
	}
	wantText := ""
	if len(want) > 0 {
		wantText = strings.Join(want, "\n") + "\n"
	}
	if got.stdout != wantText {
		t.Fatalf("cite %v:\nstdout:\n%s\nwant:\n%s", args, got.stdout, strings.Join(want, "\n"))
	}
}

// yass: 14-cite@Cite::RETURN[8]#3a75ca21
// yass: 14-cite@Cite::RETURN[10]#84184996
// yass: 14-cite@CitationTarget::INPUT[4]#ac38398e
// yass: 14-cite@CitationTarget::INVARIANT[1]#3f9826cb
func TestCiteListsASpecInCanonicalSlotOrder(t *testing.T) {
	// The reference-only entry takes no number, and an obligation another
	// spec's reference inserts is cited nowhere: only Alpha's own are listed.
	wantCite(t, citeProject(t), []string{citedStart, citedThing, citedCopy}, "a@Alpha")
}

// yass: 14-cite@CitationTarget::RETURN[4]#f027e62f
func TestCiteListsOneSlot(t *testing.T) {
	wantCite(t, citeProject(t), []string{citedThing, citedCopy}, "a@Alpha::RETURN")
}

// yass: 14-cite@CitationTarget::RETURN[5]#20b62ba0
// yass: 14-cite@Cite::RETURN[2]#7c40bb13
func TestCiteYieldsTheEntryHoldingANumber(t *testing.T) {
	dir := citeProject(t)
	wantCite(t, dir, []string{citedCopy}, "a@Alpha::RETURN[2]")
	// Several TARGETs keep the order they were given.
	wantCite(t, dir, []string{citedGoods, citedCopy, citedThing},
		"a@Beta::RETURN[1]", "a@Alpha::RETURN[2]", "a@Alpha::RETURN[1]")
}

// yass: 14-cite@Cite::RETURN[7]#f76f2187
// yass: 14-cite@Cite::RETURN[9]#3e4bdce5
func TestCiteListsAWholeFile(t *testing.T) {
	// Every spec the file holds is listed in stream order, slots of each in
	// canonical order, and the spec with no numbered entry yields nothing.
	wantCite(t, citeProject(t),
		[]string{citedStart, citedThing, citedCopy, citedGoods, citedGoods2, citedWrapped, citedSpaced}, "a")
}

// yass: 14-cite@CitationTarget::RETURN[10]#52cd00aa
// yass: 14-cite@Cite::RETURN[7]#f76f2187
func TestCiteListsNoDocumentATargetCannotAddress(t *testing.T) {
	dir := citeProject(t)
	// Two documents of one file carry a name, so no target resolves to either,
	// and a citation written for one would not resolve. The whole-file listing
	// names the document a target can address and passes the other two by.
	write(t, dir, "dup.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: Twin\nRETURN:\n- MUST: first twin\n---\nspec: Twin\nRETURN:\n- MUST: second twin\n---\nspec: Solo\nRETURN:\n- MUST: only child\n")
	wantCite(t, dir, []string{citationMarker + " dup@Solo::RETURN[1]#24d75b8f\tMUST\tonly child"}, "dup")
	// A name holding a slot token parses as a target naming another document
	// and a slot, so no target names this one, and a citation would carry two.
	write(t, dir, "odd.yass.yaml", modeline+"---\ndescription: o\nversion: v1\n---\nspec: Example::RETURN\nRETURN:\n- MUST: odd one out\n")
	wantCite(t, dir, nil, "odd")
}

// yass: 14-cite@CitationTarget::INPUT[5]#9d94c3df
// yass: 14-cite@CitationTarget::ERROR[5]#7d9e6be3
// yass: 14-cite@Cite::RETURN[9]#3e4bdce5
func TestCiteNamesNothingOutsideTheProjectFileSet(t *testing.T) {
	// A file above the project root and a file of a nested project are read
	// when the command line names them, and hold no obligation of this
	// project. No citation is written for their entries, and a number into
	// them names none.
	dir := realDir(t, "proj")
	citeProjectAt(t, dir)
	write(t, filepath.Dir(dir), "outside.yass.yaml", modeline+"---\ndescription: o\nversion: v1\n---\nspec: Out\nRETURN:\n- MUST: live outside\n")
	write(t, dir, "sub/root.yass.yaml", conformanceRoot)
	wantCite(t, dir, nil, "../outside@Out", "../outside@Out::RETURN", "sub/root@R")
	for _, target := range []string{"../outside@Out::RETURN[1]", "sub/root@R::INPUT[1]"} {
		got := run(dir, "cite", target)
		wantStatus(t, got, ExitUnresolved)
		if got.stdout != "" || !strings.Contains(got.stderr, "yass.ref.unresolved\t"+target+"\t") {
			t.Errorf("%q: status = %d, stdout = %q, stderr = %q; want yass.ref.unresolved naming the target", target, got.status, got.stdout, got.stderr)
		}
	}
}

// yass: 14-cite@CitationTarget::ERROR[1]#bc14e736
func TestCiteNamesTheFileForATargetThatFailedToParse(t *testing.T) {
	dir := citeProject(t)
	write(t, dir, "bad.yass.yaml", "spec: [ unclosed\n")
	// The file and the line of its failure are what a reader can jump to. The
	// target as given names a document the file never yielded.
	got := run(dir, "cite", "bad@X::INPUT[1]#12345678")
	wantStatus(t, got, ExitUnresolved)
	if got.stdout != "" || !strings.HasPrefix(got.stderr, "error\tyass.document.not_yaml\tbad\t") ||
		!strings.HasSuffix(got.stderr, "\tstream is not well-formed yaml\n") {
		t.Fatalf("stdout = %q, stderr = %q; want the parse failure naming the file and its line", got.stdout, got.stderr)
	}
}

// yass: 14-cite@CitationTarget::INPUT[6]#25d18e24
// yass: 14-cite@CitationTarget::ERROR[5]#7d9e6be3
// yass: 14-cite@Cite::RETURN[8]#3a75ca21
func TestCiteNamesNoSlotADocumentDeclaresTwice(t *testing.T) {
	dir := citeProject(t)
	// A spec that would not validate still loads, with two slots of one name.
	// Each numbers its entries from 1, so a citation of either is ambiguous:
	// a listing passes the slot by, and a number into it names none.
	write(t, dir, "d.yass.yaml", modeline+"---\ndescription: d\nversion: v1\n---\nspec: D\nRETURN:\n- MUST: first return\nRETURN:\n- MUST: second return\nINPUT:\n- MUST: take input\n")
	wantCite(t, dir, []string{citationMarker + " d@D::INPUT[1]#cbf13218\tMUST\ttake input"}, "d@D")
	wantCite(t, dir, nil, "d@D::RETURN")
	got := run(dir, "cite", "d@D::RETURN[1]")
	wantStatus(t, got, ExitUnresolved)
	if got.stdout != "" || !strings.Contains(got.stderr, "yass.ref.unresolved\td@D::RETURN[1]\t") {
		t.Fatalf("stdout = %q, stderr = %q; want yass.ref.unresolved naming the target", got.stdout, got.stderr)
	}
}

// yass: 14-cite@CitationTarget::RETURN[8]#c5feb4da
func TestCiteYieldsNothingForADesign(t *testing.T) {
	wantCite(t, citeProject(t), nil, "root@Policy", "a@Empty")
}

// yass: 14-cite@Cite::RETURN[1]#518e1bdd
// yass: 14-cite@Cite::ERROR[2]#a5e69af0
func TestCiteResolvesEveryTargetBeforeWriting(t *testing.T) {
	got := run(citeProject(t), "cite", "a@Alpha::RETURN[2]", "a@Nowhere")
	wantStatus(t, got, ExitUnresolved)
	if got.stdout != "" || !strings.Contains(got.stderr, "yass.ref.unresolved") {
		t.Fatalf("stdout = %q, stderr = %q; want no record and one unresolved diagnostic", got.stdout, got.stderr)
	}
}

// yass: 14-cite@Cite::ERROR[1]#d330f1d0
// yass: 14-cite@CitationTarget::INPUT[1]#3df29650
func TestCiteRejectsAMalformedIndexOrStamp(t *testing.T) {
	for _, target := range []string{
		"a@Alpha::RETURN[0]", "a@Alpha::RETURN[01]", "a@Alpha::RETURN[x]", "a@Alpha::RETURN[1",
		"a@Alpha::RETURN[1]#1234567", "a@Alpha::RETURN[1]#123456789", "a@Alpha::RETURN[1]#ABCDEF01",
		"a@Alpha::RETURN[1]#1234567g", "a@Alpha::RETURN#12345678", "a@Alpha[1]", "a[1]",
	} {
		got := run(citeProject(t), "cite", target)
		wantStatus(t, got, ExitUsage)
		want := record("error", "yass.args.target_syntax", target, "-", "reference target is malformed") + "\n"
		if got.stdout != "" || got.stderr != want {
			t.Errorf("%q: stdout = %q, stderr = %q; want none and %q", target, got.stdout, got.stderr, want)
		}
	}
}

// yass: 14-cite@CitationTarget::ERROR[1]#bc14e736
// yass: 14-cite@CitationTarget::ERROR[2]#84be25c2
// yass: 14-cite@CitationTarget::ERROR[4]#f8d96021
func TestCiteReportsWhatItCannotResolve(t *testing.T) {
	dir := citeProject(t)
	for _, tc := range []struct {
		target string
		status int
		code   string
	}{
		{"a@Alpha::SIDE-EFFECT[1]", ExitUnresolved, "yass.ref.slot_undeclared"},
		{"a@Alpha::INPUTT[1]", ExitUsage, "yass.ref.slot_unknown"},
		{"root@Policy::RETURN[1]", ExitUnresolved, "yass.ref.slot_on_design"},
		{"a@Nowhere::RETURN[1]", ExitUnresolved, "yass.ref.unresolved"},
		{"nowhere@Alpha::RETURN[1]", ExitUnresolved, "yass.ref.unresolved"},
		{"a@Alpha::RETURN[3]", ExitUnresolved, "yass.ref.unresolved"},
		// A number too large to hold is well formed, and names no entry.
		{"a@Alpha::RETURN[99999999999999999999]", ExitUnresolved, "yass.ref.unresolved"},
	} {
		got := run(dir, "cite", tc.target)
		wantStatus(t, got, tc.status)
		if got.stdout != "" || !strings.Contains(got.stderr, tc.code+"\t"+tc.target+"\t") {
			t.Errorf("%q: status = %d, stdout = %q, stderr = %q; want %s naming the target", tc.target, tc.status, got.stdout, got.stderr, tc.code)
		}
	}
}

// coatProject writes a spec whose RETURN slot holds the goods and the coat, in
// the order coatFirst says.
func coatProject(t *testing.T, coatFirst bool) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", conformanceRoot)
	first, second := "give back the goods", "give back a coat"
	if coatFirst {
		first, second = second, first
	}
	write(t, dir, "m.yass.yaml", modeline+"---\ndescription: m\nversion: v1\n---\nspec: M\nRETURN:\n- MUST: "+first+"\n- MUST: "+second+"\n")
	return dir
}

// yass: 14-cite@CitationTarget::RETURN[6]#e611bc7d
// yass: 14-cite@CitationTarget::RETURN[7]#e42e3b25
// yass: 14-cite@CitationTarget::INVARIANT[2]#3b4b53b6
func TestCiteFollowsAMovedObligationByStamp(t *testing.T) {
	// The coat is the second entry when it is written.
	wantCite(t, coatProject(t, false), []string{citedCoat}, "m@M::RETURN[2]#ef36b47e")
	// An entry inserted above it moves it to the first place, and the old
	// citation still names the coat rather than the goods now at [2].
	wantCite(t, coatProject(t, true),
		[]string{citationMarker + " m@M::RETURN[1]#ef36b47e\tMUST\tgive back a coat"}, "m@M::RETURN[2]#ef36b47e")
}

// yass: 14-cite@CitationTarget::ERROR[3]#a59b97db
func TestCiteReportsARewordedObligationAsUnresolved(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", conformanceRoot)
	write(t, dir, "m.yass.yaml", modeline+"---\ndescription: m\nversion: v1\n---\nspec: M\nRETURN:\n- MUST: give back the goods\n- MUST: give back a jacket\n")
	// Nothing carries the coat's stamp any more, so the old citation resolves
	// to nothing rather than to whatever sits at its number.
	got := run(dir, "cite", "m@M::RETURN[2]#ef36b47e")
	wantStatus(t, got, ExitUnresolved)
	if got.stdout != "" || !strings.Contains(got.stderr, "yass.ref.unresolved\tm@M::RETURN[2]#ef36b47e") {
		t.Fatalf("stdout = %q, stderr = %q; want no record and one unresolved diagnostic", got.stdout, got.stderr)
	}
	// Two identical entries share a stamp, so off its number a stamp names
	// neither alone, while on its number it still names its own entry.
	wantCite(t, citeProject(t), []string{citedGoods}, "a@Beta::RETURN[1]#e6c459d2")
	got = run(citeProject(t), "cite", "a@Beta::RETURN[3]#e6c459d2")
	wantStatus(t, got, ExitUnresolved)
	if got.stdout != "" {
		t.Fatalf("two entries carry the stamp: stdout = %q, want none", got.stdout)
	}
}

// yass: 14-cite@CitationTarget::RETURN[3]#1d72b5b1
// yass: 14-cite@CitationTarget::RETURN[9]#acde0c8c
func TestStampIgnoresQuotingLayoutAndKeyOrder(t *testing.T) {
	dir := citeProject(t)
	// The same obligation as another spec, in another slot and file, at
	// another number, with its keys reordered, its prose folded, and its
	// guard quoted, carries the same stamp.
	write(t, dir, "b.yass.yaml", modeline+`---
description: b
version: v1
---
spec: Gamma
SIDE-EFFECT:
- MUST: unrelated
- USES: root@Policy
  MUST: >-
    accept a
    thing
  SEE: a@Alpha
  WHEN: "it is raining"
`)
	wantCite(t, dir, []string{citationMarker + " b@Gamma::SIDE-EFFECT[2]#" + thingStamp + "\tMUST\taccept a thing"}, "b@Gamma::SIDE-EFFECT[2]")
	// Moving the entry down a line leaves the stamp alone.
	write(t, dir, "c.yass.yaml", modeline+"---\ndescription: c\nversion: v1\n---\nspec: Delta\nRETURN:\n\n- MUST: accept a thing\n  WHEN: it is raining\n  SEE: a@Alpha\n  USES: root@Policy\n")
	wantCite(t, dir, []string{citationMarker + " c@Delta::RETURN[1]#" + thingStamp + "\tMUST\taccept a thing"}, "c@Delta::RETURN[1]")
}

// yass: 14-cite@CitationTarget::RETURN[1]#4dc3dc15
// yass: 14-cite@CitationTarget::RETURN[3]#1d72b5b1
func TestStampChangesWithEveryValueOfTheEntry(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: r\nversion: v1\n---\nspec: R\nINPUT:\n- MUST: x\n---\ndesign: Policy\ntype: constraint\ncontent: |\n  one\n---\ndesign: Other\ntype: constraint\ncontent: |\n  one\n")
	for _, tc := range []struct{ name, body, stamp string }{
		{"keyword", "- SHOULD: accept a thing\n  WHEN: it is raining\n  SEE: a@Alpha\n  USES: root@Policy\n", "cdac8e01"},
		{"prose", "- MUST: accept things\n  WHEN: it is raining\n  SEE: a@Alpha\n  USES: root@Policy\n", "db8c8391"},
		{"guard", "- MUST: accept a thing\n  WHEN: it is snowing\n  SEE: a@Alpha\n  USES: root@Policy\n", "a4a87e2d"},
		{"no guard", "- MUST: accept a thing\n  SEE: a@Alpha\n  USES: root@Policy\n", "c6cf7cc0"},
		{"relation target", "- MUST: accept a thing\n  WHEN: it is raining\n  SEE: a@Alpha\n  USES: root@Other\n", "8588ffc7"},
		{"relation dropped", "- MUST: accept a thing\n  WHEN: it is raining\n  SEE: a@Alpha\n", "5375025d"},
		{"relation added", "- MUST: accept a thing\n  WHEN: it is raining\n  SEE: a@Alpha\n  USES: root@Policy\n  CONFORMS: a@Beta::RETURN\n", "efc673e0"},
	} {
		// The reference-only entry takes no number, so the variant is [1].
		write(t, dir, "a.yass.yaml", modeline+"---\ndescription: a\nversion: v1\n---\nspec: V\nRETURN:\n- CONFORMS: Beta::RETURN\n"+tc.body+"---\nspec: Beta\nRETURN:\n- MUST: unrelated\n")
		got := run(dir, "cite", "a@V::RETURN[1]")
		wantStatus(t, got, ExitOK)
		if want := citationMarker + " a@V::RETURN[1]#" + tc.stamp + "\t"; !strings.HasPrefix(got.stdout, want) {
			t.Errorf("%s: stdout = %q, want it to open with %q", tc.name, got.stdout, want)
		}
	}
}

// yass: 14-cite@CitationTarget::INPUT[4]#ac38398e
// yass: 14-cite@Cite::INVARIANT[2]#e666acb2
func TestCiteNumbersOnlyEntriesThatCarryAKeyword(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", conformanceRoot)
	// A spec that would not validate still loads, and each of its entries
	// that carries no normativity keyword takes no number.
	write(t, dir, "n.yass.yaml", modeline+`---
description: n
version: v1
---
spec: N
RETURN:
- WHEN: it is raining
- SEE: root@R
- {}
- MUST: begin at start
`)
	wantCite(t, dir, []string{citationMarker + " n@N::RETURN[1]#85033fc8\tMUST\tbegin at start"}, "n@N::RETURN")
}

// yass: 14-cite@Cite::RETURN[3]#6584890f
// yass: 14-cite@Cite::RETURN[4]#16eeb9e9
// yass: 14-cite@Cite::RETURN[5]#5d62441d
// yass: 14-cite@Cite::RETURN[6]#10e8ebcc
// yass: 14-cite@Cite::INVARIANT[1]#90498f24
func TestCiteRecordForms(t *testing.T) {
	dir := citeProject(t)
	// Prose collapses to one line of single spaces, as every record field does.
	wantCite(t, dir, []string{citedSpaced}, "a@Format::RETURN[1]")
	got := run(dir, "cite", "--format", "json", "a@Alpha::RETURN[2]", "a@Format::RETURN[1]")
	want := `{"citation":"` + citationMarker + " " + copyToken + `","what":"MUST-NOT","text":"keep a copy"}` + "\n" +
		`{"citation":"` + citationMarker + " " + spacedToken + `","what":"MUST","text":"line one line two"}` + "\n"
	if got.status != ExitOK || got.stderr != "" || got.stdout != want {
		t.Fatalf("status = %d, stdout = %q, stderr = %q; want %q", got.status, got.stdout, got.stderr, want)
	}
	// A value that is not a string is an absent field.
	write(t, dir, "n.yass.yaml", modeline+"---\ndescription: n\nversion: v1\n---\nspec: N\nRETURN:\n- MUST: 42\n")
	got = run(dir, "cite", "--format=json", "n@N::RETURN[1]")
	want = `{"citation":"` + citationMarker + ` n@N::RETURN[1]#ab2ec26b","what":"MUST","text":null}` + "\n"
	if got.status != ExitOK || got.stderr != "" || got.stdout != want {
		t.Fatalf("status = %d, stdout = %q, stderr = %q; want %q", got.status, got.stdout, got.stderr, want)
	}
}

// yass: 14-cite@Cite::SIDE-EFFECT[1]#2c7ec007
// yass: 14-cite@Cite::SIDE-EFFECT[2]#8b37faf0
// yass: 14-cite@Cite::RETURN[12]#9c87bade
func TestCiteWritesRecordsToStdoutAloneAndTouchesNothing(t *testing.T) {
	dir := citeProject(t)
	before := treeSnapshot(t, dir)
	got := run(dir, "cite", "a@Alpha::RETURN[1]")
	wantStatus(t, got, ExitOK)
	if got.stderr != "" || got.stdout != citedThing+"\n" {
		t.Fatalf("stdout = %q, stderr = %q", got.stdout, got.stderr)
	}
	if after := treeSnapshot(t, dir); after != before {
		t.Fatal("cite changed the file tree")
	}
	// A diagnostic goes to standard error alone.
	got = run(dir, "cite", "a@Nowhere")
	wantStatus(t, got, ExitUnresolved)
	if got.stdout != "" || got.stderr == "" {
		t.Fatalf("stdout = %q, stderr = %q; want the diagnostic on standard error alone", got.stdout, got.stderr)
	}
}

// treeSnapshot renders every file of a tree with its content, to tell a tree
// that was left alone from one that was written to.
func treeSnapshot(t *testing.T, dir string) string {
	t.Helper()
	var out []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		out = append(out, rel+": "+string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(out, "\n")
}

// yass: 14-cite@Cite::INPUT[1]#31ad3666
// yass: 14-cite@Cite::INPUT[3]#1c2a8b7f
// yass: 14-cite@Cite::INPUT[4]#579fefc0
// yass: 14-cite@CitationTarget::INPUT[3]#ad828187
func TestCiteInvocation(t *testing.T) {
	dir := citeProject(t)
	for _, tc := range []struct {
		args []string
		code string
	}{
		{[]string{"cite"}, "yass.args.missing_operand"},
		{[]string{"cite", "--raw", "a@Alpha"}, "yass.args.unknown_option"},
		{[]string{"cite", "a@Alpha", "--format", "yaml"}, "yass.args.unknown_format"},
		{[]string{"cite", ""}, "yass.args.empty"},
	} {
		got := run(dir, tc.args...)
		wantStatus(t, got, ExitUsage)
		if got.stdout != "" || !strings.Contains(got.stderr, tc.code) {
			t.Errorf("%v: status = %d, stdout = %q, stderr = %q; want %s", tc.args, got.status, got.stdout, got.stderr, tc.code)
		}
	}
	// A path with no leading dot resolves from the project root, from any
	// directory below it, and a leading `./` from the starting directory.
	want := []string{citedCopy}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	wantCite(t, filepath.Join(dir, "sub"), want, "a@Alpha::RETURN[2]")
	// The token with its stamp and without it, and the ref target alone, all
	// name the same entry, so the three TARGETs yield three records.
	wantCite(t, dir, []string{citedCopy, citedCopy, citedCopy}, "./a@Alpha::RETURN[2]", "./a@Alpha::RETURN[2]#309207aa", "a@Alpha::RETURN[2]#309207aa")
}
