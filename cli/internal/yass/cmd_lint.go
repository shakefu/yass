package yass

import (
	"path/filepath"
	"strings"
)

// cmdLint reports what is well formed but wrong about a spec set as a whole. It
// never runs validation first, so a spec set that is being authored can still
// be linted.
func (a *App) cmdLint() (int, error) {
	if err := a.prepare(); err != nil {
		return 0, err
	}

	// Reachability and reference counting are properties of the whole graph,
	// so the model is always built from the whole project file set.
	all, err := a.projectFileSet()
	if err != nil {
		return 0, err
	}
	files := make([]*File, 0, len(all))
	for _, abs := range all {
		f, lerr := a.Loader.Load(abs)
		if lerr != nil {
			return 0, lerr
		}
		files = append(files, f)
	}

	// PATH selects which files get advisories reported.
	selected := map[string]bool{}
	if len(a.Inv.Operands) == 0 {
		for _, f := range files {
			selected[f.Abs] = true
		}
	} else {
		picked, aerr := a.addressedFiles(a.Inv.Operands, false)
		if aerr != nil {
			return 0, aerr
		}
		for _, abs := range picked {
			selected[abs] = true
		}
	}

	usedDesigns := map[*Doc]bool{}
	edges := map[*Doc][]*Doc{}
	for _, f := range files {
		if !f.Parsed {
			continue
		}
		for _, d := range f.Docs {
			if d.Kind != KindSpec || !d.Addressable() {
				continue
			}
			for _, s := range d.Slots {
				for _, o := range s.Obligations {
					for _, r := range o.Refs {
						rr, code := a.resolveRefFrom(f, r)
						if code != "" {
							continue
						}
						if r.Relation == "USES" && rr.Doc.Kind == KindDesign {
							usedDesigns[rr.Doc] = true
						}
						// CONFORMS, USES, and SEE weigh the same as edges.
						edges[d] = append(edges[d], rr.Doc)
					}
				}
			}
		}
	}

	// The origin of every reachability chain is the root file's own spec.
	rootFile := a.findRootFile(files)
	var origin *Doc
	rootSpecCount := 0
	if rootFile != nil && rootFile.Parsed {
		specs := rootFile.specDocs()
		rootSpecCount = len(specs)
		if len(specs) == 1 && specs[0].Addressable() {
			origin = specs[0]
		}
	}

	reached := map[*Doc]bool{}
	if origin != nil {
		queue := []*Doc{origin}
		reached[origin] = true
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, next := range edges[cur] {
				if reached[next] || !a.underRoot(next.File) {
					continue
				}
				reached[next] = true
				queue = append(queue, next)
			}
		}
	}

	var advisories []Diag
	warn := func(code, loc string, line int, msg string) {
		advisories = append(advisories, Diag{Severity: "warn", Code: code, Location: loc, Line: line, Message: msg})
	}

	for _, f := range files {
		if !selected[f.Abs] {
			continue
		}
		if !f.Parsed {
			// A partial graph can yield a false reachability advisory, so the
			// reader must know the graph was partial.
			warn("yass.lint.unparsed", f.Ref, f.FailLine, "file yielded no documents")
			continue
		}
		if f.FirstLine != Modeline {
			warn("yass.lint.modeline", f.Ref, 1, "first line is not the schema modeline")
		}
		for _, d := range f.Docs {
			if !d.Addressable() {
				continue
			}
			if d.Kind == KindDesign && !usedDesigns[d] {
				warn("yass.lint.design_unreferenced", d.Target(), d.Line,
					"design is the target of no uses reference")
			}
			if origin != nil && d != origin && !reached[d] {
				warn("yass.lint.unreachable", d.Target(), d.Line,
					"document is reached by no reference chain from the root spec")
			}
			if d.Kind != KindSpec {
				continue
			}
			if len(d.Slots) == 0 {
				warn("yass.lint.spec_empty", d.Target(), d.Line, "spec declares no slot")
			}
			for _, s := range d.Slots {
				if len(s.Obligations) == 0 {
					warn("yass.lint.slot_empty", s.Target(), s.Line, "slot declares no obligation")
				}
			}
		}
	}

	sortDiags(advisories)
	for _, d := range advisories {
		a.out.line(d.Record())
	}

	if rootSpecCount != 1 {
		// Reachability has no single origin; every advisory that does not
		// depend on it has still been written.
		a.err.line(Diag{
			Severity: "error", Code: "yass.root.spec_count", Location: refOf(RootBasename),
			Message: "root file does not carry exactly one spec",
		}.Record())
		return ExitFindings, nil
	}
	if len(advisories) > 0 {
		return ExitFindings, nil
	}
	return ExitOK, nil
}

func (a *App) findRootFile(files []*File) *File {
	want := filepath.Join(a.Root, RootBasename)
	for _, f := range files {
		if f.Abs == want {
			return f
		}
	}
	return nil
}

// underRoot reports whether a file lies at or below the project root, so
// reachability never crosses a project root boundary.
func (a *App) underRoot(f *File) bool {
	rel := a.rel(f.Abs)
	return rel != "" && rel != ".." && !strings.HasPrefix(rel, "/") &&
		!strings.HasPrefix(rel, "../")
}
