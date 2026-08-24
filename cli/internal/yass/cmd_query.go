package yass

import "gopkg.in/yaml.v3"

// cmdQuery emits each addressed document as a self-contained YAML fragment,
// with its references resolved unless --raw was given.
func (a *App) cmdQuery() (int, error) {
	if err := a.prepare(); err != nil {
		return 0, err
	}

	// Every TARGET is resolved before anything is emitted, so a failure on any
	// one leaves standard output empty.
	bounds := make([]*Bound, 0, len(a.Inv.Operands))
	for _, raw := range a.Inv.Operands {
		b, err := a.ResolveTarget(raw)
		if err != nil {
			return 0, err
		}
		bounds = append(bounds, b)
	}

	fragments := make([][]string, 0, len(bounds))
	for _, b := range bounds {
		lines, err := a.fragment(b)
		if err != nil {
			return 0, err
		}
		fragments = append(fragments, lines)
	}

	for _, lines := range fragments {
		for _, l := range lines {
			a.out.line(l)
		}
	}
	return ExitOK, nil
}

// fragment renders one TARGET, together with any design documents resolution
// appends after it.
func (a *App) fragment(b *Bound) ([]string, error) {
	var out []string
	switch {
	case b.WholeFile:
		// A file target asks what the file is, so the preamble alone is the
		// fragment.
		pre := a.preambleOf(b.File)
		out = append(out, "---")
		if pre != nil {
			emitDocumentEntries(&out, pre)
		}
		return out, nil

	case b.Doc.Kind == KindDesign:
		out = append(out, "---")
		emitDocumentEntries(&out, b.Doc)
		return out, nil
	}

	slots := b.Doc.Slots
	if b.Slot != "" {
		slots = nil
		for _, s := range b.Doc.Slots {
			if s.Name == b.Slot {
				slots = append(slots, s)
			}
		}
	}

	if a.Inv.Raw {
		out = append(out, "---")
		out = append(out, "spec: "+scalarInline(b.Doc.Name))
		for _, s := range slots {
			emitSlot(&out, s, nil)
		}
		return out, nil
	}

	res, err := a.resolveSlots(b.Doc, slots)
	if err != nil {
		return nil, err
	}
	out = append(out, "---")
	out = append(out, "spec: "+scalarInline(b.Doc.Name))
	for i, s := range slots {
		emitSlot(&out, s, res.perSlot[i])
	}
	for _, ap := range res.appends {
		out = append(out, "---")
		out = append(out, "# USES: "+ap.source)
		emitDocumentEntries(&out, ap.doc)
	}
	return out, nil
}

func scalarInline(v string) string {
	var lines []string
	emitScalarValue(&lines, "", "k", v)
	if len(lines) == 1 {
		return lines[0][len("k: "):]
	}
	return quoteDouble(v)
}

func (a *App) preambleOf(f *File) *Doc {
	if len(f.Docs) > 0 && f.Docs[0].Kind == KindPreamble {
		return f.Docs[0]
	}
	return nil
}

// insertion is one obligation spliced in front of a carrier by a slot-targeted
// CONFORMS, with the source ref target the provenance comment names.
type insertion struct {
	before *Obligation // the carrier this insertion precedes
	source string
	obl    *Obligation
	guard  string // conjoined guard, "" when the obligation keeps its own
	drop   bool   // the carrier is reference-only and adds no obligation
}

type designAppend struct {
	source string
	doc    *Doc
}

type resolution struct {
	perSlot [][]insertion
	appends []designAppend
}

// resolveSlots applies reference resolution to the emitted slots of a spec
// document. Resolution is one level only: a reference carried by an obligation
// it inserted, or by a design it appended, stays unresolved.
func (a *App) resolveSlots(doc *Doc, slots []*Slot) (*resolution, error) {
	res := &resolution{perSlot: make([][]insertion, len(slots))}
	seen := map[string]bool{}

	for si, s := range slots {
		for _, o := range s.Obligations {
			ins := insertion{before: o, drop: false}
			transcluded := false
			for _, r := range o.Refs {
				rr, code := a.resolveRefFrom(doc.File, r)
				if code != "" {
					return nil, fail(ExitUnresolved, code, doc.Target(), r.Line, refMessage(code))
				}
				switch r.Relation {
				case "CONFORMS":
					if rr.Slot == "" {
						// A whole-spec CONFORMS is a conformance reference
						// rather than a transclusion.
						continue
					}
					transcluded = true
					for _, src := range slotOf(rr.Doc, rr.Slot).Obligations {
						guard := ""
						if src.HasGuard && o.HasGuard {
							guard = o.Guard + " and " + src.Guard
						}
						res.perSlot[si] = append(res.perSlot[si], insertion{
							before: o, source: r.Target, obl: src, guard: guard,
						})
					}
				case "USES":
					transcluded = true
					key := rr.Doc.File.Abs + "\x00" + rr.Doc.Name
					if !seen[key] {
						seen[key] = true
						res.appends = append(res.appends, designAppend{source: r.Target, doc: rr.Doc})
					}
				}
			}
			if o.RefOnly() && transcluded {
				ins.drop = true
			}
			if ins.drop {
				res.perSlot[si] = append(res.perSlot[si], insertion{before: o, drop: true})
			}
		}
	}
	return res, nil
}

func slotOf(d *Doc, name string) *Slot {
	for _, s := range d.Slots {
		if s.Name == name {
			return s
		}
	}
	return &Slot{}
}

// emitSlot writes one slot and the obligations inserted into it.
func emitSlot(out *[]string, s *Slot, ins []insertion) {
	if s.Vn != nil && s.Vn.Kind != yaml.SequenceNode {
		emitNode(out, "", s.Name, s.Vn)
		return
	}
	if len(s.Obligations) == 0 {
		*out = append(*out, s.Name+": []")
		return
	}
	*out = append(*out, s.Name+":")
	for _, o := range s.Obligations {
		dropped := false
		for _, in := range ins {
			if in.before != o {
				continue
			}
			if in.drop {
				dropped = true
				continue
			}
			*out = append(*out, "# CONFORMS: "+in.source)
			emitObligation(out, in.obl, in.guard)
		}
		if !dropped {
			emitObligation(out, o, "")
		}
	}
}

func refMessage(code string) string {
	switch code {
	case "yass.ref.syntax":
		return "reference target is malformed"
	case "yass.ref.conforms_design":
		return "conforms target resolves to a design"
	case "yass.ref.uses_spec":
		return "uses target resolves to a spec or slot"
	}
	return "reference target does not resolve"
}
