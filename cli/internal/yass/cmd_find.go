package yass

import "strings"

// findRow is one emitted find record: REF, WHAT, TEXT.
type findRow struct{ ref, what, text string }

// cmdFind matches a literal substring against obligation prose, guard
// conditions, and design content, answering with the slot-level ref target of
// every hit plus the matched text.
func (a *App) cmdFind() (int, error) {
	if err := a.prepare(); err != nil {
		return 0, err
	}
	pattern := a.Inv.Operands[0]
	if a.Inv.HasSlot && !isSlotName(a.Inv.SlotOpt) {
		return 0, fail(ExitUsage, "yass.ref.slot_unknown", a.Inv.SlotOpt, 0,
			"slot token is not a recognized slot")
	}

	files, err := a.projectFileSet()
	if err != nil {
		return 0, err
	}

	status := ExitOK
	var unparsed []Diag
	var rows []findRow

	for _, abs := range files {
		f, err := a.Loader.Load(abs)
		if err != nil {
			return 0, err
		}
		if !f.Parsed {
			status = ExitFindings
			unparsed = append(unparsed, Diag{
				Severity: "error", Code: f.FailCode, Location: f.Ref,
				Line: f.FailLine, Message: failMessage(f.FailCode),
			})
			continue
		}
		for _, d := range f.Docs {
			if !d.Addressable() {
				continue
			}
			if d.Kind == KindDesign {
				// A design declares no slot, so --slot never restricts it.
				if !d.HasContent {
					continue
				}
				for _, line := range strings.Split(strings.TrimSuffix(d.Content, "\n"), "\n") {
					if foldContains(line, pattern) {
						rows = append(rows, findRow{d.Target(), "content", line})
					}
				}
				continue
			}
			for _, want := range SlotNames {
				for _, s := range d.Slots {
					if s.Name != want {
						continue
					}
					if a.Inv.HasSlot && s.Name != a.Inv.SlotOpt {
						continue
					}
					for _, o := range s.Obligations {
						rows = append(rows, matchObligation(s, o, pattern)...)
					}
				}
			}
		}
	}

	for _, r := range rows {
		a.out.line(record(r.ref, r.what, r.text))
	}
	for _, d := range unparsed {
		a.err.line(d.Record())
	}
	return status, nil
}

// matchObligation emits one record per matching field of an obligation, in the
// authored order of its keys. A reference-only obligation holds no text to
// match, so it yields nothing.
func matchObligation(s *Slot, o *Obligation, pattern string) []findRow {
	var out []findRow
	ref := s.Target()
	for _, e := range o.Entries {
		switch {
		case e.Key == "WHEN" && o.HasGuard && o.GuardScalar:
			if foldContains(o.Guard, pattern) {
				out = append(out, findRow{ref, "WHEN", o.Guard})
			}
		case isNormativity(e.Key) && e.Key == o.Keyword && o.HasProse:
			if foldContains(o.Prose, pattern) {
				out = append(out, findRow{ref, o.Keyword, o.Prose})
			}
		}
	}
	return out
}
