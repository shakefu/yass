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

// discoverRoot yields the nearest directory at or above an already-resolved
// absolute starting path that holds a file named exactly root.yass.yaml. A
// project root is located by that name and by nothing else; the ascent never
// descends into a child directory and never moves sideways. label is how the
// path is named in a diagnostic.
func discoverRoot(start, label string) (string, error) {
	info, err := os.Stat(start)
	if err != nil {
		if os.IsPermission(err) {
			return "", unreadable(label)
		}
		return "", missingPath(label)
	}
	dir := start
	if !info.IsDir() {
		dir = filepath.Dir(start)
	}
	for {
		candidate := filepath.Join(dir, RootBasename)
		st, err := os.Stat(candidate)
		switch {
		case err == nil:
			// The name compares byte-for-byte, so a differently cased file a
			// case-insensitive filesystem stats as the same is no root.
			if st.Mode().IsRegular() && hasEntryNamed(dir, RootBasename) {
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

// startDirectory turns a resolved starting path into the directory path
// resolution begins from: the path itself when it names a directory, and the
// directory holding it when it names a file.
func startDirectory(resolved string) string {
	if st, err := os.Stat(resolved); err == nil && !st.IsDir() {
		return filepath.Dir(resolved)
	}
	return resolved
}

// isSpecBasename reports whether a basename ends with the literal .yass.yaml
// suffix and carries at least one character before it. The comparison is
// byte-for-byte and case-sensitive on every platform.
func isSpecBasename(name string) bool {
	return strings.HasSuffix(name, SpecSuffix) && len(name) > len(SpecSuffix)
}

// collectSpecFiles yields every regular file at or below dir whose basename
// ends with .yass.yaml, ordered by byte-wise ascending path relative to the
// project root. It never descends through a symbolic link naming a directory,
// and never descends into a subdirectory that holds its own root.yass.yaml —
// including dir itself when dir is not the project root, so a directory
// operand naming a nested project root yields nothing rather than the files
// that nearer root governs.
func collectSpecFiles(dir string, root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return unreadable(relTo(root, path))
		}
		if entry.IsDir() {
			if path != root && holdsRootFile(path) {
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
		return nil, unreadable(relTo(root, dir))
	}
	sort.Slice(out, func(i, j int) bool {
		return relTo(root, out[i]) < relTo(root, out[j])
	})
	return out, nil
}

// hasEntryNamed reports whether dir holds an entry whose name is exactly name,
// compared byte-for-byte, so that a case-insensitive filesystem cannot answer
// for a differently cased spelling. It serves two checks: that a directory's
// root file is spelled `root.yass.yaml` (root discovery and the nested-root
// check, which run without an App and so read the listing uncached), and, via
// App.hasEntryNamed, that each component of a ref target's path token is
// spelled as written. Every caller has already found the path by os.Stat, so
// a directory that cannot be listed fails open and reports true: failing
// closed would turn a correctly spelled, readable path into an unresolved one,
// at the cost of accepting a misspelling only where the listing is denied.
func hasEntryNamed(dir, name string) bool {
	names := entryNames(dir)
	return names == nil || names[name]
}

// entryNames returns the set of entry names dir holds, or nil when dir cannot
// be opened or listed.
func entryNames(dir string) map[string]bool {
	d, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer d.Close() //nolint:errcheck
	list, err := d.Readdirnames(-1)
	if err != nil {
		return nil
	}
	names := make(map[string]bool, len(list))
	for _, n := range list {
		names[n] = true
	}
	return names
}

func holdsRootFile(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, RootBasename))
	return err == nil && st.Mode().IsRegular() && hasEntryNamed(dir, RootBasename)
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
