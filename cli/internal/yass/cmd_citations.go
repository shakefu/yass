package yass

import (
	"os"
	"strconv"
)

// coverageFields are the fields of one coverage record: REF, MUST,
// MUST_CITED, SHOULD, SHOULD_CITED.
var coverageFields = []string{"ref", "must", "must_cited", "should", "should_cited"}

// cmdCitations checks the citations a project's tests carry against the spec
// set, or reports what the citations leave uncovered.
func (a *App) cmdCitations() (int, error) {
	if err := a.prepare(); err != nil {
		return 0, err
	}
	specs, err := a.projectFileSet()
	if err != nil {
		return 0, err
	}
	// The scope is settled before anything is written, so a PATH the project
	// does not govern ends the request with nothing on either stream.
	scope, err := a.selectedFiles(a.Inv.Operands, a.citationSelect(), false)
	if err != nil {
		return 0, err
	}

	status := ExitOK
	var files []*File
	for _, abs := range specs {
		f, lerr := a.Loader.Load(abs)
		if lerr != nil {
			return 0, lerr
		}
		if !f.Parsed {
			// An obligation of that file can be neither resolved nor counted,
			// so the reader must know the model was partial.
			status = ExitFindings
			a.err.line(Diag{Severity: "error", Code: f.FailCode, Location: f.Ref,
				Line: f.FailLine, Message: failMessage(f.FailCode)}.Record())
			continue
		}
		files = append(files, f)
	}

	// A report counts the citations that carry a matching stamp and checks
	// nothing; the default run checks every citation and counts nothing.
	report := a.Inv.Missing || a.Inv.Coverage
	covered := map[*Obligation]bool{}
	for _, abs := range scope {
		data, rerr := os.ReadFile(abs)
		if rerr != nil {
			return 0, unreadable(a.rel(abs))
		}
		for _, c := range scanCitations(data, a.rel(abs)) {
			// A token that fails the citation grammar and one whose ref target
			// the grammar or the slot vocabulary rejects are both malformed.
			// Every other outcome has a code of its own, and each is written.
			var o *Obligation
			code := citationSyntax
			if ct, ok := parseCitationToken(c.token); ok {
				var cerr error
				if o, code, cerr = a.checkCitation(ct); cerr != nil {
					return 0, cerr
				}
			}
			if o != nil {
				covered[o] = true
			}
			if report {
				continue
			}
			switch code {
			case citationSyntax:
				status = ExitFindings
				a.citationDiag("error", code, c, "citation is malformed")
			case citationUnresolved:
				status = ExitFindings
				a.citationDiag("error", code, c, "citation "+c.token+" names no obligation")
			case citationStale:
				status = ExitFindings
				a.citationDiag("warn", code, c, "citation "+c.token+" carries a stamp the obligation at that number does not")
			}
		}
	}

	if !report {
		return status, nil
	}
	if a.Inv.Missing {
		a.reportMissing(files, covered)
	} else {
		a.reportCoverage(files, covered)
	}
	return status, nil
}

// citationDiag writes one citation finding to standard output.
func (a *App) citationDiag(severity, code string, c citationFound, message string) {
	a.out.line(a.diag(Diag{Severity: severity, Code: code, Location: c.file, Line: c.line, Message: message}))
}

// reportMissing lists every obligation of the project that no citation covers,
// one record in the form `yass cite` writes.
func (a *App) reportMissing(files []*File, covered map[*Obligation]bool) {
	for _, f := range files {
		for _, d := range f.Docs {
			if d.Kind != KindSpec || !d.Addressable() {
				continue
			}
			for _, s := range canonicalSlots(d) {
				for i, o := range numbered(s) {
					if isObligation(o) && !covered[o] {
						c := cite(s, i+1, o)
						a.out.line(a.record(citeFields, citationMarker+" "+c.Token, o.Keyword, obligationProse(o)))
					}
				}
			}
		}
	}
}

// reportCoverage counts, per spec document, the obligations that MUST, SHOULD,
// and their negatives carry and how many are covered, then the whole project.
func (a *App) reportCoverage(files []*File, covered map[*Obligation]bool) {
	var total [4]int
	for _, f := range files {
		for _, d := range f.Docs {
			if d.Kind != KindSpec || !d.Addressable() {
				continue
			}
			var row [4]int
			for _, s := range canonicalSlots(d) {
				for _, o := range numbered(s) {
					switch o.Keyword {
					case "MUST", "MUST-NOT":
						row[0]++
						if covered[o] {
							row[1]++
						}
					case "SHOULD", "SHOULD-NOT":
						row[2]++
						if covered[o] {
							row[3]++
						}
					}
				}
			}
			if row[0]+row[2] == 0 {
				continue
			}
			a.coverageRow(d.Target(), row)
			for i := range total {
				total[i] += row[i]
			}
		}
	}
	if total[0]+total[2] == 0 {
		// A project holding no obligation has nothing to count.
		return
	}
	a.coverageRow("-", total)
}

func (a *App) coverageRow(ref string, row [4]int) {
	a.out.line(a.record(coverageFields, ref, strconv.Itoa(row[0]), strconv.Itoa(row[1]),
		strconv.Itoa(row[2]), strconv.Itoa(row[3])))
}

// isObligation reports whether an entry counts toward coverage: it carries
// MUST, SHOULD, or a negative of either. MAY is a permission that no test is
// written for, so an entry carrying it is neither listed nor counted.
func isObligation(o *Obligation) bool {
	return o.Keyword == "MUST" || o.Keyword == "MUST-NOT" || o.Keyword == "SHOULD" || o.Keyword == "SHOULD-NOT"
}
