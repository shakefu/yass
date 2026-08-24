package yass

import (
	"strconv"
	"strings"
)

// descriptionLimit is the number of Unicode scalar values a file description is
// truncated to in a list record.
const descriptionLimit = 100

// cmdList reports every file, spec, and design of a tree as one record per
// line: a ref target, a kind, a one-field detail, and a truncated file
// description.
func (a *App) cmdList() (int, error) {
	if err := a.prepare(); err != nil {
		return 0, err
	}
	files, err := a.addressedFiles(a.Inv.Operands, true)
	if err != nil {
		return 0, err
	}

	status := ExitOK
	for _, abs := range files {
		f, err := a.Loader.Load(abs)
		if err != nil {
			return 0, err
		}
		fileRef := f.Ref

		if !f.Parsed {
			// A file that yielded no documents is a finding about the spec
			// set; which rule it breaks is validation's to name, not this
			// command's.
			status = ExitFindings
			if matchesAny(a.Inv.Filters, fileRef) {
				a.out.line(record(fileRef, "file", "", ""))
			}
			continue
		}

		description := ""
		if pre := a.preambleOf(f); pre != nil {
			description = truncateScalars(field(pre.Description), descriptionLimit)
		} else {
			status = ExitFindings
		}

		type docRecord struct{ ref, kind, detail string }
		var docs []docRecord
		count := 0
		for _, d := range f.Docs {
			if d.Kind != KindSpec && d.Kind != KindDesign {
				continue
			}
			count++
			if !d.Addressable() {
				continue
			}
			detail := ""
			if d.Kind == KindSpec {
				detail = declaredSlotList(d)
			} else {
				detail = d.Type
			}
			docs = append(docs, docRecord{ref: d.Target(), kind: d.Kind.String(), detail: detail})
		}

		var selected []docRecord
		for _, dr := range docs {
			if matchesAny(a.Inv.Filters, dr.ref) {
				selected = append(selected, dr)
			}
		}
		// A document record is never orphaned from the file that holds it.
		if !matchesAny(a.Inv.Filters, fileRef) && len(selected) == 0 {
			continue
		}
		a.out.line(record(fileRef, "file", strconv.Itoa(count), description))
		for _, dr := range selected {
			a.out.line(record(dr.ref, dr.kind, dr.detail, ""))
		}
	}
	return status, nil
}

// declaredSlotList joins a spec's declared slot names in canonical order.
func declaredSlotList(d *Doc) string {
	var names []string
	for _, want := range SlotNames {
		for _, s := range d.Slots {
			if s.Name == want && !contains(names, want) {
				names = append(names, want)
			}
		}
	}
	return strings.Join(names, ",")
}

// truncateScalars truncates to at most limit Unicode scalar values, making the
// last emitted scalar a single U+2026 when truncation dropped at least one.
func truncateScalars(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit-1]) + "…"
}
