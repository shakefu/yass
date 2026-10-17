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

	// Every addressed file is loaded before any record is written: a status
	// of 2 through 5 means the program did not get far enough to answer, so
	// standard output must stay empty when one of them fails.
	loaded := make([]*File, len(files))
	for i, abs := range files {
		f, err := a.Loader.Load(abs)
		if err != nil {
			return 0, err
		}
		loaded[i] = f
	}

	status := ExitOK
	for _, f := range loaded {
		fileRef := f.Ref

		if !f.Parsed {
			// A file that yielded no documents is a finding about the spec
			// set; which rule it breaks is validation's to name, not this
			// command's. The finding counts only when the file's record is
			// part of the answer the filter selected.
			if matchesAny(a.Inv.Filters, fileRef) {
				status = ExitFindings
				a.out.line(record(fileRef, "file", "", ""))
			}
			continue
		}

		pre := a.preambleOf(f)
		description := ""
		if pre != nil {
			description = truncateScalars(field(pre.Description), descriptionLimit)
		}

		type docRecord struct{ ref, kind, detail string }
		var selected []docRecord
		count := 0
		for _, d := range f.Docs {
			if !d.Addressable() {
				continue
			}
			count++
			detail := ""
			if d.Kind == KindSpec {
				detail = declaredSlotList(d)
			} else {
				detail = d.Type
			}
			// A document record is never orphaned from the file that holds it.
			if matchesAny(a.Inv.Filters, d.Target()) {
				selected = append(selected, docRecord{ref: d.Target(), kind: d.Kind.String(), detail: detail})
			}
		}
		if !matchesAny(a.Inv.Filters, fileRef) && len(selected) == 0 {
			continue
		}
		if pre == nil {
			status = ExitFindings
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
