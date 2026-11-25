package yass

import (
	"path/filepath"
	"sort"
)

// projectFileSet yields every spec file the project root governs, ordered by
// byte-wise ascending project-root-relative path. It always holds at least the
// root file itself.
func (a *App) projectFileSet() ([]string, error) {
	return collectSpecFiles(a.Root, a.Root, a.names)
}

// addressedFiles turns PATH operands into an ordered set of absolute file
// paths, taking each file at most once even when two PATHs address it. With no
// PATH the project file set is taken. requireSpecBasename selects whether a
// PATH naming a file must itself end with the .yass.yaml suffix. Every PATH
// must lie in the project the root governs: one outside it, or under a nearer
// project root, ends the request, since a second tree's files would be read
// against the wrong root's model or skipped without a word.
func (a *App) addressedFiles(paths []string, requireSpecBasename bool) ([]string, error) {
	if len(paths) == 0 {
		return a.projectFileSet()
	}
	var all []string
	for _, p := range paths {
		resolved, st, err := resolvePath(a.absPath(p), p)
		if err != nil {
			return nil, err
		}
		switch {
		case st.IsDir():
			if err := a.governed(resolved, true, p); err != nil {
				return nil, err
			}
			// The operand names a directory, through a symbolic link when it
			// uses one; the walk begins at the directory it names, which
			// walking the link itself would not enter.
			files, err := collectSpecFiles(resolved, a.Root, a.names)
			if err != nil {
				return nil, err
			}
			all = append(all, files...)
		case st.Mode().IsRegular():
			if err := a.governed(resolved, false, p); err != nil {
				return nil, err
			}
			if requireSpecBasename && !isSpecBasename(baseName(resolved)) {
				return nil, notASpecFile(p)
			}
			// Resolved as the Loader names it, so orderUnique takes one
			// physical file once however the PATHs spell it.
			all = append(all, resolved)
		default:
			return nil, notASpecFile(p)
		}
	}
	return a.orderUnique(all), nil
}

// governed reports whether the project root governs an addressed path, which is
// a directory when dir is true and a file otherwise: the path lies at or below
// the root, and no directory from the root down to the one holding it holds a
// root.yass.yaml of its own, because that nearer project root governs its
// subtree and the project file set excludes it. Every directory on the way down
// must be one that can be read, whatever it holds, as the ascent requires of
// every directory it climbs. label names the operand in a diagnostic.
func (a *App) governed(abs string, dir bool, label string) error {
	segs, ok := a.relUnder(abs)
	if !ok {
		return outsideRoot(label)
	}
	if !dir && len(segs) > 0 {
		segs = segs[:len(segs)-1]
	}
	at := a.Root
	for _, seg := range segs {
		at = filepath.Join(at, seg)
		held, readable := rootFileIn(at, a.names)
		if !readable || a.names.listing(at) == nil {
			return unreadable(relTo(a.Root, at))
		}
		if held {
			return outsideRoot(label)
		}
	}
	return nil
}

// orderUnique deduplicates absolute paths and orders them by byte-wise
// ascending project-root-relative path.
func (a *App) orderUnique(paths []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return a.rel(out[i]) < a.rel(out[j]) })
	return out
}

func baseName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[i+1:]
		}
	}
	return p
}
