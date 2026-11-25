package yass

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"math"
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

	var slots []*Slot
	switch {
	case b.WholeFile:
		for _, d := range b.File.Docs {
			if d.Kind == KindSpec && d.Addressable() {
				slots = append(slots, canonicalSlots(d)...)
			}
		}
	case b.Doc.Kind == KindDesign:
		// A design holds no obligation to cite.
	case b.Slot != "":
		for _, s := range b.Doc.Slots {
			if s.Name == b.Slot {
				slots = append(slots, s)
			}
		}
	default:
		slots = canonicalSlots(b.Doc)
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

// canonicalSlots yields a spec's declared slots in canonical order.
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
