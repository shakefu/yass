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
func discoverRoot(start, label string, names *dirNames) (string, error) {
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
			if isRootFile(dir, st, names) {
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
func collectSpecFiles(dir string, root string, names *dirNames) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return unreadable(relTo(root, path))
		}
		if entry.IsDir() {
			if path != root && holdsRootFile(path, names) {
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

// dirNames caches directory listings so a name can be compared with a
// directory's entries byte-for-byte: a case-insensitive filesystem lets os.Stat
// answer for a differently cased spelling, while the root file's name and each
// component of a ref target's path token are case-sensitive on every platform.
// A nil set records a directory that cannot be listed.
type dirNames struct{ m map[string]map[string]bool }

func newDirNames() *dirNames { return &dirNames{m: map[string]map[string]bool{}} }

// has reports whether dir holds an entry named exactly name, reading each
// directory's listing at most once; a nil receiver reads it uncached. Every
// caller has already found the path by os.Stat, so a directory that cannot be
// listed fails open and reports true: failing closed would turn a correctly
// spelled, readable path into an unresolved one, at the cost of accepting a
// misspelling only where the listing is denied.
func (c *dirNames) has(dir, name string) bool {
	var names map[string]bool
	if c == nil {
		names = entryNames(dir)
	} else {
		var seen bool
		if names, seen = c.m[dir]; !seen {
			names = entryNames(dir)
			c.m[dir] = names
		}
	}
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

// isRootFile reports whether st, the stat of dir's root.yass.yaml, is a
// regular file listed under exactly that name.
func isRootFile(dir string, st fs.FileInfo, names *dirNames) bool {
	return st.Mode().IsRegular() && names.has(dir, RootBasename)
}

func holdsRootFile(dir string, names *dirNames) bool {
	st, err := os.Stat(filepath.Join(dir, RootBasename))
	return err == nil && isRootFile(dir, st, names)
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
