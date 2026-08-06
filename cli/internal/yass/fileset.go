package yass

import (
	"os"
	"sort"
)

// projectFileSet yields every spec file the project root governs, ordered by
// byte-wise ascending project-root-relative path. It always holds at least the
// root file itself.
func (a *App) projectFileSet() ([]string, error) {
	return collectSpecFiles(a.Root, a.Root)
}

// addressedFiles turns PATH operands into an ordered set of absolute file
// paths, taking each file at most once even when two PATHs address it. With no
// PATH the project file set is taken. requireSpecBasename selects whether a
// PATH naming a file must itself end with the .yass.yaml suffix.
func (a *App) addressedFiles(paths []string, requireSpecBasename bool) ([]string, error) {
	if len(paths) == 0 {
		return a.projectFileSet()
	}
	var all []string
	for _, p := range paths {
		abs := a.absPath(p)
		st, err := os.Stat(abs)
		if err != nil {
			if os.IsPermission(err) {
				return nil, unreadable(p)
			}
			return nil, missingPath(p)
		}
		switch {
		case st.IsDir():
			files, err := collectSpecFiles(abs, a.Root)
			if err != nil {
				return nil, err
			}
			all = append(all, files...)
		case st.Mode().IsRegular():
			if requireSpecBasename && !isSpecBasename(baseName(abs)) {
				return nil, notASpecFile(p)
			}
			all = append(all, abs)
		default:
			return nil, notASpecFile(p)
		}
	}
	return a.orderUnique(all), nil
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
