package yass

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// resolveStart turns a starting path into an absolute path with symbolic links
// resolved and `.` and `..` removed. label is how the path is named in a
// diagnostic.
func resolveStart(path, label string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", missingPath(label)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if os.IsPermission(err) {
			return "", unreadable(label)
		}
		return "", missingPath(label)
	}
	return real, nil
}

// discoverRoot yields the nearest directory at or above the starting path that
// holds a file named exactly root.yass.yaml. A project root is located by that
// name and by nothing else; the ascent never descends into a child directory
// and never moves sideways.
func discoverRoot(start, label string) (string, error) {
	real, err := resolveStart(start, label)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(real)
	if err != nil {
		if os.IsPermission(err) {
			return "", unreadable(label)
		}
		return "", missingPath(label)
	}
	dir := real
	if !info.IsDir() {
		dir = filepath.Dir(real)
	}
	for {
		candidate := filepath.Join(dir, RootBasename)
		st, err := os.Stat(candidate)
		switch {
		case err == nil:
			if st.Mode().IsRegular() {
				return dir, nil
			}
		case os.IsPermission(err):
			return "", unreadable(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", noRoot(label)
		}
		dir = parent
	}
}

// isSpecBasename reports whether a basename ends with the literal .yass.yaml
// suffix and carries at least one character before it. The comparison is
// byte-for-byte and case-sensitive on every platform.
func isSpecBasename(name string) bool {
	return strings.HasSuffix(name, SpecSuffix) && len(name) > len(SpecSuffix)
}

// collectSpecFiles yields every regular file at or below dir whose basename
// ends with .yass.yaml, ordered by byte-wise ascending path relative to
// relBase. It never descends through a symbolic link naming a directory, and
// never descends into a subdirectory that holds its own root.yass.yaml.
func collectSpecFiles(dir string, relBase string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return unreadable(relTo(relBase, path))
		}
		if entry.IsDir() {
			if path != dir && holdsRootFile(path) {
				return fs.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if !isSpecBasename(name) {
			return nil
		}
		switch {
		case entry.Type().IsRegular():
			out = append(out, path)
		case entry.Type()&fs.ModeSymlink != 0:
			// Read through a symbolic link that names a regular file; a link
			// naming a directory is never descended through.
			if st, serr := os.Stat(path); serr == nil && st.Mode().IsRegular() {
				out = append(out, path)
			}
		}
		return nil
	})
	if err != nil {
		if ce, ok := err.(*cliError); ok {
			return nil, ce
		}
		return nil, unreadable(relTo(relBase, dir))
	}
	sort.Slice(out, func(i, j int) bool {
		return relTo(relBase, out[i]) < relTo(relBase, out[j])
	})
	return out, nil
}

func holdsRootFile(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, RootBasename))
	return err == nil && st.Mode().IsRegular()
}

func relTo(base, path string) string {
	if base == "" {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
