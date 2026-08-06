package yass

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Kind tags a document as a preamble, a spec, or a design. A document is a
// spec when it carries a `spec` key, a design when it carries a `design` key,
// and a preamble when it carries neither; whether the preamble sits first is a
// question for validation, not for loading.
type Kind int

const (
	KindPreamble Kind = iota
	KindSpec
	KindDesign
)

func (k Kind) String() string {
	switch k {
	case KindSpec:
		return "spec"
	case KindDesign:
		return "design"
	}
	return "preamble"
}

// SlotNames is the canonical slot order, used wherever a slot sequence is
// emitted.
var SlotNames = []string{"INPUT", "RETURN", "ERROR", "SIDE-EFFECT", "INVARIANT"}

// RelationNames is the canonical relation order.
var RelationNames = []string{"CONFORMS", "USES", "SEE"}

// NormativityNames is the recognized normativity vocabulary.
var NormativityNames = []string{"MUST", "MUST-NOT", "SHOULD", "SHOULD-NOT", "MAY"}

func isSlotName(s string) bool { return contains(SlotNames, s) }

func isRelation(s string) bool { return contains(RelationNames, s) }

func isNormativity(s string) bool { return contains(NormativityNames, s) }

func contains(set []string, s string) bool {
	for _, v := range set {
		if v == s {
			return true
		}
	}
	return false
}

// slotRank orders a slot name canonically; an unrecognized name sorts last.
func slotRank(s string) int {
	for i, v := range SlotNames {
		if v == s {
			return i
		}
	}
	return len(SlotNames)
}

func relationRank(s string) int {
	for i, v := range RelationNames {
		if v == s {
			return i
		}
	}
	return len(RelationNames)
}

// nameGrammar is the spec and design name regex the language definition fixes.
var nameGrammar = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)

// isReservedName reports whether a document name case-insensitively matches a
// slot keyword or a normativity keyword.
func isReservedName(name string) bool {
	for _, k := range SlotNames {
		if strings.EqualFold(name, k) {
			return true
		}
	}
	for _, k := range NormativityNames {
		if strings.EqualFold(name, k) {
			return true
		}
	}
	return false
}

// SpecSuffix is the literal basename suffix of a yass spec file.
const SpecSuffix = ".yass.yaml"

// RootBasename is the file whose presence defines a project root.
const RootBasename = "root.yass.yaml"

// Modeline is the schema modeline lint expects as a file's first line.
const Modeline = "# yaml-language-server: $schema=https://textla.dev/yass/v1.schema.json"

// File is one loaded spec file: either its parsed documents or a parse
// failure, never both and never neither.
type File struct {
	Abs       string // absolute path on disk
	Rel       string // project-root-relative, `/` separators, suffix kept
	Ref       string // Rel with the .yass.yaml suffix removed
	FirstLine string // the file's first line, for the modeline check
	Parsed    bool
	FailCode  string // yass.document.not_yaml or yass.document.encoding
	FailLine  int
	Docs      []*Doc
}

// Doc is one document of a file.
type Doc struct {
	File    *File
	Index   int // position within the file's stream
	Kind    Kind
	Name    string
	HasName bool
	Line    int
	Node    *yaml.Node // the document's content node
	Entries []Entry    // top-level mapping pairs, in authored order

	// Preamble
	Description    string
	HasDescription bool
	Version        string
	HasVersion     bool
	RelatedNode    *yaml.Node

	// Design
	Type       string
	HasType    bool
	Content    string
	HasContent bool

	// Spec
	Slots []*Slot
}

// Entry is one key/value pair of a document's top-level mapping.
type Entry struct {
	Key string
	Kn  *yaml.Node
	Vn  *yaml.Node
}

// Target renders `path@Name`, the ref target that addresses this document.
func (d *Doc) Target() string { return d.File.Ref + "@" + d.Name }

// Addressable reports whether the document can be named by a ref target.
func (d *Doc) Addressable() bool {
	return d.HasName && d.Name != "" && (d.Kind == KindSpec || d.Kind == KindDesign)
}

// Slot is one declared slot of a spec document.
type Slot struct {
	Doc         *Doc
	Name        string
	Index       int // authored position among the document's slots
	Line        int
	Kn          *yaml.Node
	Vn          *yaml.Node
	Obligations []*Obligation
}

// Target renders `path@Name::SLOT`.
func (s *Slot) Target() string { return s.Doc.Target() + "::" + s.Name }

// Obligation is one item of a slot's sequence.
type Obligation struct {
	Slot    *Slot
	Index   int
	Line    int
	Node    *yaml.Node
	Mapping bool // the item is a YAML mapping
	Entries []Entry

	Keyword     string // the normativity keyword, "" when reference-only
	Keywords    []string
	Prose       string
	HasProse    bool
	Guard       string
	HasGuard    bool
	GuardScalar bool
	Refs        []*Ref
}

// RefOnly reports whether the obligation carries at least one relation key, no
// normativity keyword, and no WHEN guard.
func (o *Obligation) RefOnly() bool {
	return len(o.Refs) > 0 && o.Keyword == "" && !o.HasGuard
}

// Ref is one reference an obligation carries.
type Ref struct {
	Obl      *Obligation
	Relation string
	Target   string
	IsString bool
	Line     int
	Node     *yaml.Node
}

// Slot returns the slot the reference sits in.
func (r *Ref) Slot() *Slot { return r.Obl.Slot }

// specDocs returns the file's spec documents.
func (f *File) specDocs() []*Doc {
	var out []*Doc
	for _, d := range f.Docs {
		if d.Kind == KindSpec {
			out = append(out, d)
		}
	}
	return out
}
