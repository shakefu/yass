package yass

import "sort"

// cmdRefs reports the edges of one document: what it points at, and what points
// at it.
func (a *App) cmdRefs() (int, error) {
	if err := a.prepare(); err != nil {
		return 0, err
	}
	b, err := a.ResolveTarget(a.Inv.Operands[0])
	if err != nil {
		return 0, err
	}
	if b.WholeFile {
		// An edge belongs to a document, and a file is not one.
		return 0, fail(ExitUsage, "yass.args.missing_operand", b.Raw, 0,
			"target names a file rather than a document")
	}

	// Neither flag, or both, mean both directions.
	wantOut := a.Inv.Out || !a.Inv.In
	wantIn := a.Inv.In || !a.Inv.Out

	status := ExitOK
	var outgoing, incoming []string

	if wantOut && b.Doc.Kind == KindSpec {
		for _, s := range b.Doc.Slots {
			if b.Slot != "" && s.Name != b.Slot {
				continue
			}
			for _, o := range s.Obligations {
				for _, r := range sortedRefs(o) {
					rr, code := a.resolveRefFrom(b.File, r)
					if code != "" {
						// An unresolved reference is a validation finding
						// rather than an edge.
						continue
					}
					outgoing = append(outgoing, record("out", r.Relation, a.edgeTarget(rr)))
				}
			}
		}
	}

	if wantIn {
		files, err := a.projectFileSet()
		if err != nil {
			return 0, err
		}
		for _, abs := range files {
			f, lerr := a.Loader.Load(abs)
			if lerr != nil {
				return 0, lerr
			}
			if !f.Parsed {
				status = ExitFindings
				a.err.line(Diag{
					Severity: "error", Code: f.FailCode, Location: f.Ref,
					Line: f.FailLine, Message: failMessage(f.FailCode),
				}.Record())
				continue
			}
			for _, d := range f.Docs {
				if d.Kind != KindSpec || !d.Addressable() {
					continue
				}
				for _, s := range d.Slots {
					for _, o := range s.Obligations {
						for _, r := range sortedRefs(o) {
							rr, code := a.resolveRefFrom(f, r)
							if code != "" || rr.Doc != b.Doc {
								continue
							}
							if b.Slot != "" && rr.Slot != b.Slot {
								continue
							}
							incoming = append(incoming, record("in", r.Relation, d.Target()))
						}
					}
				}
			}
		}
	}

	for _, l := range outgoing {
		a.out.line(l)
	}
	for _, l := range incoming {
		a.out.line(l)
	}
	return status, nil
}

// edgeTarget renders the far end of an outgoing edge project-root-relative,
// keeping any ::SLOT the reference stated.
func (a *App) edgeTarget(rr *refResolution) string {
	t := rr.Doc.Target()
	if rr.Slot != "" {
		t += "::" + rr.Slot
	}
	return t
}

// sortedRefs orders the references of one obligation canonically: CONFORMS,
// USES, SEE.
func sortedRefs(o *Obligation) []*Ref {
	refs := make([]*Ref, len(o.Refs))
	copy(refs, o.Refs)
	sort.SliceStable(refs, func(i, j int) bool {
		return relationRank(refs[i].Relation) < relationRank(refs[j].Relation)
	})
	return refs
}
