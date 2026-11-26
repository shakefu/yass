package yass

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// citationMarker opens a citation: the body of the comment a test carries is
// this word, whitespace, and one citation token (14-cite@CitationSyntax).
const citationMarker = "yass:"

// citationTail is the index and optional stamp that follow a ref target.
var citationTail = regexp.MustCompile(`^\[([1-9][0-9]*)\](#[0-9a-f]{8})?$`)

// citationTarget is a ref target with an optional obligation index and an
// optional stamp. An index is 1-based within the addressed slot and 0 when
// absent; a stamp never stands without an index.
type citationTarget struct {
	Raw   string
	Base  string // the ref target
	Index int
	Stamp string
}

// parseCitation splits a command-line citation target into its ref target,
// index, and stamp. It reports false when an index or a stamp does not fit
// the citation grammar, or when an index stands on a target naming no slot,
// since the index counts the entries of one slot. The grammar of the ref
// target itself is left to target resolution.
func parseCitation(raw string) (citationTarget, bool) {
	ct := citationTarget{Raw: raw}
	i := strings.LastIndex(raw, "[")
	if i < 0 {
		ct.Base = raw
		return ct, true
	}
	m := citationTail.FindStringSubmatch(raw[i:])
	if m == nil {
		return ct, false
	}
	ct.Base = raw[:i]
	if !strings.Contains(ct.Base, "::") {
		return ct, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		// A number too large to hold is well formed and names no entry.
		n = math.MaxInt
	}
	ct.Index = n
	ct.Stamp = strings.TrimPrefix(m[2], "#")
	return ct, true
}

// numbered yields the entries of a slot that carry a normativity keyword, in
// authored order. Position plus one is the entry's citation number; an entry
// holding only references is absent and takes no number.
func numbered(s *Slot) []*Obligation {
	var out []*Obligation
	for _, o := range s.Obligations {
		if o.Keyword != "" {
			out = append(out, o)
		}
	}
	return out
}

// obligationStamp is the stamp of one entry: the first four bytes of a sha256
// digest over the entry's own keys and values, so quoting, layout, and the
// entries around it change nothing (14-cite@ObligationStamp).
func obligationStamp(o *Obligation) string {
	h := sha256.New()
	stampField(h, o.Keyword)
	var prose, guard string
	if o.HasProse {
		prose = o.Prose
	}
	if o.HasGuard && o.GuardString {
		guard = o.Guard
	}
	stampField(h, prose)
	stampField(h, guard)
	for _, r := range sortedRefs(o) {
		var target string
		if r.IsString {
			target = r.Target
		}
		stampField(h, r.Relation)
		stampField(h, target)
	}
	return hex.EncodeToString(h.Sum(nil)[:4])
}

// stampField feeds one length-prefixed field to the digest, so no value can be
// read as two.
func stampField(h hash.Hash, text string) {
	h.Write([]byte(strconv.Itoa(len(text))))
	h.Write([]byte{':'})
	h.Write([]byte(text))
}

// cited is one obligation a citation target resolved to.
type cited struct {
	Token string // path@Name::SLOT[n]#stamp
	Obl   *Obligation
}

// resolveCitation binds a command-line citation target against the loaded
// model. A target with no index yields every numbered entry the ref target
// addresses. With an index and no stamp it yields the entry holding that
// number. With a stamp it yields the entry holding that number when its stamp
// matches, and otherwise the one entry of the slot carrying that stamp, since
// an obligation that moved takes its stamp with it; a stamp no entry carries,
// as after a rewording, resolves to nothing. An error is the diagnostic target
// resolution or this resolution selects.
func (a *App) resolveCitation(ct citationTarget) ([]cited, error) {
	b, err := a.ResolveTarget(ct.Base)
	if err != nil {
		return nil, asGiven(err, ct)
	}
	// A citation names an obligation of this project alone. A file outside the
	// project file set addresses no entry, as a design addresses none.
	member, err := a.inCitationFileSet(b.File.Abs)
	if err != nil {
		return nil, err
	}
	if !member {
		return noEntry(ct)
	}

	var slots []*Slot
	switch {
	case b.WholeFile:
		for _, d := range b.File.Docs {
			if addressableByTarget(d) {
				slots = append(slots, uniqueSlots(d)...)
			}
		}
	case b.Doc.Kind == KindDesign:
		// A design holds no obligation to cite.
	case b.Slot != "":
		if s := singleSlot(b.Doc, b.Slot); s != nil {
			slots = append(slots, s)
		}
	default:
		slots = uniqueSlots(b.Doc)
	}
	if len(slots) == 0 {
		// Nothing a citation can name: a design, or a slot declared twice.
		return noEntry(ct)
	}

	if ct.Index == 0 {
		var out []cited
		for _, s := range slots {
			for i, o := range numbered(s) {
				out = append(out, cite(s, i+1, o))
			}
		}
		return out, nil
	}

	entries := numbered(slots[0])
	if ct.Stamp == "" {
		if ct.Index <= len(entries) {
			return []cited{cite(slots[0], ct.Index, entries[ct.Index-1])}, nil
		}
		return nil, fail(ExitUnresolved, "yass.ref.unresolved", ct.Raw, 0, "reference target does not resolve")
	}
	if ct.Index <= len(entries) && obligationStamp(entries[ct.Index-1]) == ct.Stamp {
		return []cited{cite(slots[0], ct.Index, entries[ct.Index-1])}, nil
	}
	found := -1
	for i, o := range entries {
		if obligationStamp(o) == ct.Stamp {
			if found >= 0 {
				// Two entries share a stamp: it names neither alone.
				found = -2
				break
			}
			found = i
		}
	}
	if found >= 0 {
		return []cited{cite(slots[0], found+1, entries[found])}, nil
	}
	return nil, fail(ExitUnresolved, "yass.ref.unresolved", ct.Raw, 0,
		"citation stamp names no obligation of that slot")
}

// noEntry is the outcome of a citation target that addresses no entry: nothing
// to list, and an unresolved diagnostic for one that named a number.
func noEntry(ct citationTarget) ([]cited, error) {
	if ct.Index == 0 {
		return nil, nil
	}
	return nil, fail(ExitUnresolved, "yass.ref.unresolved", ct.Raw, 0, "reference target does not resolve")
}

// inCitationFileSet reports whether abs names a file of the project file set,
// the only spec files a citation may name or be checked against. The set is
// walked once per invocation and judged on cleaned paths.
func (a *App) inCitationFileSet(abs string) (bool, error) {
	if a.citedFiles == nil {
		files, err := a.projectFileSet()
		if err != nil {
			return false, err
		}
		a.citedFiles = make(map[string]bool, len(files))
		for _, f := range files {
			a.citedFiles[filepath.Clean(f)] = true
		}
	}
	return a.citedFiles[filepath.Clean(abs)], nil
}

// addressableByTarget reports whether a command-line ref target can name a
// spec document: it is addressable, `path@Name` parses as a target naming
// exactly that document and no slot, and no other document of its file carries
// its name. A citation is written only for an obligation of such a document,
// since one written for any other would not resolve once it is pasted into a
// test.
func addressableByTarget(d *Doc) bool {
	if d.Kind != KindSpec || !d.Addressable() {
		return false
	}
	t, ok := parseTarget(d.File.Ref+"@"+d.Name, true)
	if !ok || t.Name != d.Name || t.HasSlot {
		return false
	}
	same := 0
	for _, o := range d.File.Docs {
		if o.Addressable() && o.Name == d.Name {
			same++
		}
	}
	return same == 1
}

// asGiven lets a diagnostic target resolution emits name the citation target
// as it was given, index and stamp included, rather than the ref target that
// was resolved alone. Any other failure passes through untouched.
func asGiven(err error, ct citationTarget) error {
	e, ok := err.(*cliError)
	if !ok || e.Diag.Location != ct.Base {
		return err
	}
	e.Diag.Location = ct.Raw
	return e
}

// canonicalSlots yields a spec's declared slots in canonical order, a slot
// the spec declares twice included twice.
func canonicalSlots(d *Doc) []*Slot {
	var out []*Slot
	for _, name := range SlotNames {
		for _, s := range d.Slots {
			if s.Name == name {
				out = append(out, s)
			}
		}
	}
	return out
}

// uniqueSlots yields, in canonical order, the slots of a spec that a citation
// can name: each declared slot whose name the spec declares once. A spec that
// fails validation may declare a slot twice, and two slots of one name leave a
// citation's number ambiguous, so such a slot is left out.
func uniqueSlots(d *Doc) []*Slot {
	var out []*Slot
	for _, name := range SlotNames {
		if s := singleSlot(d, name); s != nil {
			out = append(out, s)
		}
	}
	return out
}

// singleSlot yields the slot a spec declares under name, and nil when it
// declares no slot of that name or declares more than one.
func singleSlot(d *Doc, name string) *Slot {
	var found *Slot
	for _, s := range d.Slots {
		if s.Name != name {
			continue
		}
		if found != nil {
			return nil
		}
		found = s
	}
	return found
}

// cite renders the citation of one numbered entry of a slot.
func cite(s *Slot, n int, o *Obligation) cited {
	return cited{
		Token: s.Target() + "[" + strconv.Itoa(n) + "]#" + obligationStamp(o),
		Obl:   o,
	}
}

// obligationProse is the value of an entry's normativity keyword, or the empty
// text when that value is not a string.
func obligationProse(o *Obligation) string {
	if o.HasProse {
		return o.Prose
	}
	return ""
}

// marker is the bytes that open a citation.
var marker = []byte(citationMarker)

// citationTokenTail is the index and stamp that end a citation read from a
// file, where the stamp is required.
var citationTokenTail = regexp.MustCompile(`^\[([1-9][0-9]*)\]#([0-9a-f]{8})$`)

// parseCitationToken reads one citation token as CitationSyntax states it: a
// ref target in its full form, an index, and a stamp, all required. The
// grammar of the ref target's own tokens is left to target resolution.
func parseCitationToken(token string) (citationTarget, bool) {
	ct := citationTarget{Raw: token}
	i := strings.LastIndex(token, "[")
	if i < 0 {
		return ct, false
	}
	m := citationTokenTail.FindStringSubmatch(token[i:])
	if m == nil {
		return ct, false
	}
	ct.Base = token[:i]
	if !strings.Contains(ct.Base, "@") || !strings.Contains(ct.Base, "::") {
		return ct, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		// A number too large to hold is well formed, and names no entry.
		n = math.MaxInt
	}
	ct.Index, ct.Stamp = n, m[2]
	return ct, true
}

// citationFound is one citation recognized in a file of the citation scope.
type citationFound struct {
	file  string // project-root-relative, `/` separators
	line  int    // 1-based
	token string // the bytes after the marker, empty when nothing follows
}

// scanCitations recognizes every citation in data, the bytes of one file, as
// CitationSyntax states: the marker as a whole word, and the token from the
// marker's whitespace to the next space, tab, or carriage return. Nothing is
// parsed and no encoding is checked, so the bytes of any file are read alike.
func scanCitations(data []byte, file string) []citationFound {
	var out []citationFound
	for line := 1; len(data) > 0; line++ {
		var l []byte
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			l, data = data[:i], data[i+1:]
		} else {
			l, data = data, nil
		}
		out = append(out, lineCitations(l, file, line)...)
	}
	return out
}

// lineCitations recognizes every citation of one line, left to right.
func lineCitations(l []byte, file string, line int) []citationFound {
	var out []citationFound
	for i := 0; ; {
		j := bytes.Index(l[i:], marker)
		if j < 0 {
			return out
		}
		j += i
		i = j + len(marker)
		if j > 0 && isWordByte(l[j-1]) {
			// A longer name merely ends with the marker: not a whole word.
			continue
		}
		for i < len(l) && (l[i] == ' ' || l[i] == '\t') {
			i++
		}
		end := i
		for end < len(l) && l[end] != ' ' && l[end] != '\t' && l[end] != '\r' {
			end++
		}
		out = append(out, citationFound{file, line, string(l[i:end])})
		i = end
	}
}

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

// isTestFileName reports whether a basename is named as a test
// (15-citations@TestFileNames).
func isTestFileName(name string) bool {
	for _, part := range testNameParts(name) {
		switch {
		case strings.EqualFold(part, "test"), strings.EqualFold(part, "tests"),
			strings.EqualFold(part, "spec"), strings.EqualFold(part, "specs"):
			return true
		}
	}
	return false
}

// testNameParts splits a basename at every `.`, `_`, and `-` and before each
// capital letter that opens a word: one that follows a lower-case letter or a
// digit, or that a lower-case letter follows. A run of capitals such as SPEC
// or HTTP stays one part.
func testNameParts(name string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c == '.' || c == '_' || c == '-':
			parts = append(parts, name[start:i])
			start = i + 1
		case c >= 'A' && c <= 'Z' && i > start && (isLower(name[i-1]) || i+1 < len(name) && isLower(name[i+1])):
			parts = append(parts, name[start:i])
			start = i
		}
	}
	return append(parts, name[start:])
}

// isLower reports whether c is a lower-case ASCII letter or a digit.
func isLower(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}

// citationSelect yields the predicate that selects the files of the citation
// scope: the files whose whole project-root-relative path matches the --tests
// glob when one is given, and the files named as tests otherwise.
func (a *App) citationSelect() func(rel, name string) bool {
	if a.Inv.HasTests {
		return func(rel, _ string) bool { return globMatch(a.Inv.Tests, rel) }
	}
	return func(_, name string) bool { return isTestFileName(name) }
}

// The codes of a citation finding.
const (
	citationSyntax     = "yass.citation.syntax"
	citationUnresolved = "yass.citation.unresolved"
	citationStale      = "yass.citation.stale"
)

// checkCitation resolves one citation read from a file against the loaded
// model. Its ref target resolves from the project root, and only a file of the
// project file set may be read to resolve it. The result is the obligation the
// citation covers, or nil together with the code naming what is wrong with the
// citation. An error is a failure to serve the request, such as a cited file
// that cannot be read.
func (a *App) checkCitation(ct citationTarget) (*Obligation, string, error) {
	// The grammar and the slot vocabulary are judged first and read nothing,
	// so a malformed citation stays malformed whatever file it names.
	t, ok := parseTarget(ct.Base, true)
	if !ok || t.HasSlot && !isSlotName(t.Slot) {
		return nil, citationSyntax, nil
	}
	member, err := a.inCitationFileSet(filePathFor(a.Root, a.Root, t.Path))
	if err != nil {
		return nil, "", err
	}
	if !member {
		// Another project's spec file: never read, and never an obligation of
		// this one.
		return nil, citationUnresolved, nil
	}
	b, err := a.resolveTargetFrom(ct.Base, a.Root)
	if err != nil {
		if e, ok := err.(*cliError); ok {
			// A target the grammar or the slot vocabulary rejects is a
			// malformed citation; one that addresses nothing is unresolved.
			switch e.Status {
			case ExitUsage:
				return nil, citationSyntax, nil
			case ExitUnresolved:
				return nil, citationUnresolved, nil
			}
		}
		return nil, "", err
	}
	// A spec that fails validation may declare the slot twice. Two slots of
	// one name leave the number ambiguous, so the citation names no entry,
	// exactly as `yass cite` writes none for it.
	slot := singleSlot(b.Doc, b.Slot)
	if slot == nil {
		return nil, citationUnresolved, nil
	}
	entries := numbered(slot)
	if ct.Index > len(entries) {
		return nil, citationUnresolved, nil
	}
	o := entries[ct.Index-1]
	if obligationStamp(o) != ct.Stamp {
		return nil, citationStale, nil
	}
	return o, "", nil
}
