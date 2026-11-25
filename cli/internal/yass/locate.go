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

// resolvePath turns a PATH operand into the path root discovery anchors on and
// membership in the project is judged by: a directory through every symbolic
// link, and a file by its own name in the directory that physically holds it,
// which is how a loaded file is named, so a symbolic link naming a regular
// file belongs to the project holding the link as the project file set holds
// it. The entry's own info is returned with it. label is how the path is
// named in a diagnostic.
func resolvePath(path, label string) (string, fs.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsPermission(err) {
			return "", nil, unreadable(label)
		}
		return "", nil, missingPath(label)
	}
	resolved, err := path, error(nil)
	if info.IsDir() {
		resolved, err = filepath.EvalSymlinks(path)
	} else if dir, derr := filepath.EvalSymlinks(filepath.Dir(path)); derr != nil {
		resolved, err = "", derr
	} else {
		resolved = filepath.Join(dir, filepath.Base(path))
	}
	if err != nil {
		if os.IsPermission(err) {
			return "", nil, unreadable(label)
		}
		return "", nil, missingPath(label)
	}
	return resolved, info, nil
}

// discoverRoot yields the nearest directory at or above an already-resolved
// absolute starting path that holds a file named exactly root.yass.yaml. A
// project root is located by that name and by nothing else; the ascent never
// descends into a child directory and never moves sideways. Every directory it
// examines must be one that can be read, whatever it holds, so a directory
// that cannot be listed is never yielded as a project root and never climbed
// past. label is how the path is named in a diagnostic.
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
		// The traversal learns that a directory cannot be listed from its own
		// read of it; the ascent reads no directory otherwise, so it asks.
		if names.listing(dir) == nil {
			return "", unreadable(dir)
		}
		held, readable := rootFileIn(dir, names)
		if !readable {
			return "", unreadable(dir)
		}
		if held {
			return dir, nil
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
// project root.
func collectSpecFiles(dir string, root string, names *dirNames) ([]string, error) {
	return collectFiles(dir, root, names, func(_, name string) bool { return isSpecBasename(name) })
}

// collectFiles yields every regular file at or below dir that keep selects,
// judged on the file's project-root-relative path and basename, ordered by
// byte-wise ascending path relative to the project root. It never descends through a symbolic link naming a directory,
// and never descends into a subdirectory that holds its own root.yass.yaml —
// including dir itself when dir is not the project root, so a directory
// operand naming a nested project root yields nothing rather than the files
// that nearer root governs. A directory that cannot be read ends the traversal
// with a diagnostic naming that directory: the walk reports one it cannot
// list, and the root-file probe reports one whose entries cannot be examined,
// before any file inside it is taken and before it is skipped as a nested root.
func collectFiles(dir string, root string, names *dirNames, keep func(rel, name string) bool) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return unreadable(relTo(root, path))
		}
		if entry.IsDir() {
			held, readable := rootFileIn(path, names)
			if !readable {
				return unreadable(relTo(root, path))
			}
			if held && path != root {
				return fs.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if !keep(relTo(root, path), name) {
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
	names := c.listing(dir)
	return names == nil || names[name]
}

// listing returns the set of entry names dir holds, or nil when dir cannot be
// listed, reading each directory's listing at most once; a nil receiver reads
// it uncached. Root discovery and traversal ask it directly, because a
// directory that cannot be listed is one they report rather than accept.
func (c *dirNames) listing(dir string) map[string]bool {
	if c == nil {
		return entryNames(dir)
	}
	names, seen := c.m[dir]
	if !seen {
		names = entryNames(dir)
		c.m[dir] = names
	}
	return names
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

// rootFileIn reports whether dir holds a regular file listed under exactly the
// name root.yass.yaml, and whether dir could be read far enough to tell. A
// directory cannot be read when a named entry in it cannot be examined or its
// entries cannot be listed; both are judged by the outcome of the operation,
// never by mode bits. Examining the root file's name probes the first, whether
// or not the file exists. The listing is consulted only where that name
// answers, since only there does the exact spelling need confirming, so a
// caller that requires every directory to be listable asks for the listing
// itself.
func rootFileIn(dir string, names *dirNames) (held, readable bool) {
	st, err := os.Stat(filepath.Join(dir, RootBasename))
	if err != nil {
		return false, !os.IsPermission(err)
	}
	if !st.Mode().IsRegular() {
		return false, true
	}
	listed := names.listing(dir)
	return listed[RootBasename], listed != nil
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
