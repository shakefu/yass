package yass

import (
	"os"
	"path/filepath"
	"strings"
)

// parsedTarget is a ref target split into its three tokens.
type parsedTarget struct {
	Path    string
	HasPath bool
	Name    string
	Slot    string
	HasSlot bool
}

func inClass(s string, allow func(byte) bool) bool {
	for i := 0; i < len(s); i++ {
		if !allow(s[i]) {
			return false
		}
	}
	return true
}

func pathByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
		c == '.' || c == '_' || c == '/' || c == '-'
}

func nameByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
		c == '.' || c == '_' || c == '-'
}

func slotByte(c byte) bool { return c >= 'A' && c <= 'Z' || c == '-' }

// parseTarget splits `path@Name::SLOT` into its tokens. bareIsPath selects the
// command-line extension, in which a target with no `@` addresses a whole file;
// inside a spec file a bare name still addresses a document of the same file.
func parseTarget(target string, bareIsPath bool) (parsedTarget, bool) {
	var t parsedTarget
	s := target
	if i := strings.Index(s, "::"); i >= 0 {
		t.Slot = s[i+2:]
		t.HasSlot = true
		s = s[:i]
	}
	hasAt := false
	if i := strings.Index(s, "@"); i >= 0 {
		hasAt = true
		t.Path = s[:i]
		t.HasPath = true
		t.Name = s[i+1:]
	} else if bareIsPath {
		t.Path = s
		t.HasPath = true
	} else {
		t.Name = s
	}

	if !inClass(t.Path, pathByte) || !inClass(t.Name, nameByte) || !inClass(t.Slot, slotByte) {
		return t, false
	}
	if t.HasPath && (t.Path == "" || t.Path[0] == '/') {
		// An empty path token names no file, and a leading `/` would read as a
		// POSIX absolute path when yass paths are file-relative or root-relative.
		return t, false
	}
	if t.HasSlot && t.Slot == "" {
		return t, false
	}
	if t.Name == "" && (hasAt || !t.HasPath) {
		// `a@` names no document, and a bare empty token names nothing at all.
		return t, false
	}
	return t, true
}

// Bound is a resolved command-line target.
type Bound struct {
	Raw       string // the target exactly as given
	Abs       string // absolute path of the addressed file
	File      *File
	Doc       *Doc   // nil when the target named a whole file
	Slot      string // "" when no slot was named
	Normal    string // normalized root-relative rendering of the target
	WholeFile bool
}

// filePathFor turns a path token into a file path. A token beginning with `./`
// or `../` resolves against base; any other token resolves from the project
// root. The suffix is appended to the token exactly as written, and the join
// cleans nothing, so `.` and `..` segments stay part of the name spelled and
// the kernel alone resolves what they climb.
func filePathFor(root, base, token string) string {
	prefix := root
	if strings.HasPrefix(token, "./") || strings.HasPrefix(token, "../") {
		prefix = base
	}
	return prefix + string(filepath.Separator) + filepath.FromSlash(token) + SpecSuffix
}

// ResolveTarget binds one command-line target against the loaded model. A
// target the grammar or the slot vocabulary rejects exits with the usage
// status; a well-formed target that addresses nothing exits with the
// unresolved status.
func (a *App) ResolveTarget(raw string) (*Bound, error) {
	t, ok := parseTarget(raw, true)
	if !ok {
		return nil, fail(ExitUsage, "yass.ref.syntax", raw, 0, "reference target is malformed")
	}
	if t.HasSlot && !isSlotName(t.Slot) {
		return nil, fail(ExitUsage, "yass.ref.slot_unknown", raw, 0, "slot token is not a recognized slot")
	}

	abs := filePathFor(a.Root, a.StartDir, t.Path)
	st, err := os.Stat(abs)
	if err != nil || !st.Mode().IsRegular() {
		if err != nil && os.IsPermission(err) {
			return nil, unreadable(a.rel(abs))
		}
		return nil, fail(ExitUnresolved, "yass.ref.unresolved", raw, 0, "reference target does not resolve")
	}

	f, err := a.Loader.Load(abs)
	if err != nil {
		return nil, err
	}
	if !f.Parsed {
		return nil, fail(ExitUnresolved, f.FailCode, f.Ref, f.FailLine, failMessage(f.FailCode))
	}

	b := &Bound{Raw: raw, Abs: abs, File: f, Slot: t.Slot}
	if !t.HasPath || t.Name == "" {
		b.WholeFile = true
		b.Normal = f.Ref
		if t.HasSlot {
			// A whole-file target carries no document to hang a slot on.
			return nil, fail(ExitUnresolved, "yass.ref.unresolved", raw, 0, "reference target does not resolve")
		}
		return b, nil
	}

	var found []*Doc
	for _, d := range f.Docs {
		if d.Addressable() && d.Name == t.Name {
			found = append(found, d)
		}
	}
	switch len(found) {
	case 0:
		return nil, fail(ExitUnresolved, "yass.ref.unresolved", raw, 0, "reference target does not resolve")
	case 1:
	default:
		return nil, fail(ExitUnresolved, "yass.ref.unresolved", raw, 0,
			"two documents of "+f.Ref+SpecSuffix+" carry that name")
	}
	b.Doc = found[0]
	b.Normal = f.Ref + "@" + b.Doc.Name

	if t.HasSlot {
		if b.Doc.Kind == KindDesign {
			return nil, fail(ExitUnresolved, "yass.ref.slot_on_design", raw, 0, "slot token addresses a design")
		}
		if !declaresSlot(b.Doc, t.Slot) {
			return nil, fail(ExitUnresolved, "yass.ref.slot_undeclared", raw, 0,
				"referenced spec does not declare that slot")
		}
		b.Normal += "::" + t.Slot
	}
	return b, nil
}

func declaresSlot(d *Doc, slot string) bool {
	for _, s := range d.Slots {
		if s.Name == slot {
			return true
		}
	}
	return false
}

func failMessage(code string) string {
	if code == "yass.document.encoding" {
		return "file is not valid utf-8"
	}
	return "stream is not well-formed yaml"
}

// refResolution is the outcome of resolving one in-file reference.
type refResolution struct {
	File *File
	Doc  *Doc
	Slot string
}

// resolveRefFrom resolves one reference carried by a document of carrier. It
// returns the resolution, or an error that is either the diagnostic the
// resolution rules select — naming the carrying document and the target — or
// an environment failure a referenced path turned into an unreadable file.
// The codes are the ones 04-query@Resolution::ERROR enumerates.
func (a *App) resolveRefFrom(carrier *Doc, r *Ref) (*refResolution, error) {
	bad := func(code string) error {
		return fail(ExitUnresolved, code, carrier.Target(), r.Line, refMessage(code, r.Target))
	}
	if !r.IsString {
		return nil, bad("yass.ref.syntax")
	}
	t, ok := parseTarget(r.Target, false)
	if !ok {
		return nil, bad("yass.ref.syntax")
	}
	if t.HasSlot && !isSlotName(t.Slot) {
		return nil, bad("yass.ref.unresolved")
	}

	abs := carrier.File.Abs
	if t.HasPath {
		abs = filePathFor(a.Root, filepath.Dir(carrier.File.Abs), t.Path)
	}
	var f *File
	if abs == carrier.File.Abs {
		f = carrier.File
	} else {
		st, err := os.Stat(abs)
		if err != nil || !st.Mode().IsRegular() {
			return nil, bad("yass.ref.unresolved")
		}
		// A referenced file that exists but cannot be read is an environment
		// failure, not an unresolved reference (root@YassCli::ERROR).
		loaded, lerr := a.Loader.Load(abs)
		if lerr != nil {
			return nil, lerr
		}
		f = loaded
	}
	if !f.Parsed {
		return nil, bad("yass.ref.unresolved")
	}

	var found []*Doc
	for _, d := range f.Docs {
		if d.Addressable() && d.Name == t.Name {
			found = append(found, d)
		}
	}
	// An absent name and a duplicated name alike resolve to neither, the rule
	// target resolution already states for a command-line target.
	if len(found) != 1 {
		return nil, bad("yass.ref.unresolved")
	}
	doc := found[0]

	if t.HasSlot {
		if doc.Kind == KindDesign {
			return nil, bad("yass.ref.unresolved")
		}
		if !declaresSlot(doc, t.Slot) {
			return nil, bad("yass.ref.unresolved")
		}
	}
	switch r.Relation {
	case "CONFORMS":
		if doc.Kind == KindDesign {
			return nil, bad("yass.ref.conforms_design")
		}
	case "USES":
		if doc.Kind != KindDesign {
			return nil, bad("yass.ref.uses_spec")
		}
	}
	return &refResolution{File: f, Doc: doc, Slot: t.Slot}, nil
}
