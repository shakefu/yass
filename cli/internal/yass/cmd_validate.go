package yass

import (
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// preambleKeys is the closed set of keys a preamble may carry.
var preambleKeys = []string{"description", "version", "related"}

// designKeys is the closed set of keys a design may carry.
var designKeys = []string{"design", "type", "content"}

// cmdValidate checks addressed files against every construct of the yass
// language definition and emits one coded diagnostic per rule violation. It is
// the only place this program names which rule a file violates.
func (a *App) cmdValidate() (int, error) {
	if err := a.prepare(); err != nil {
		return 0, err
	}
	files, err := a.addressedFiles(a.Inv.Operands, false)
	if err != nil {
		return 0, err
	}

	var diags []Diag
	for _, abs := range files {
		f, lerr := a.Loader.Load(abs)
		if lerr != nil {
			return 0, lerr
		}
		diags = append(diags, a.checkFile(f)...)
	}

	sortDiags(diags)
	for _, d := range diags {
		a.out.line(d.Record())
	}
	if len(diags) > 0 {
		return ExitFindings, nil
	}
	return ExitOK, nil
}

// sortDiags orders by byte-wise ascending location path, then ascending line,
// then code in byte order.
func sortDiags(diags []Diag) {
	sort.SliceStable(diags, func(i, j int) bool {
		pi, pj := locationPath(diags[i].Location), locationPath(diags[j].Location)
		if pi != pj {
			return pi < pj
		}
		if diags[i].Line != diags[j].Line {
			return diags[i].Line < diags[j].Line
		}
		return diags[i].Code < diags[j].Code
	})
}

func locationPath(loc string) string {
	for i := 0; i < len(loc); i++ {
		if loc[i] == '@' {
			return loc[:i]
		}
	}
	return loc
}

func diag(code, location string, line int, message string) Diag {
	return Diag{Severity: "error", Code: code, Location: location, Line: line, Message: message}
}

// checkFile runs every check against one addressed file.
func (a *App) checkFile(f *File) []Diag {
	var out []Diag

	if !f.Parsed {
		// The file's documents cannot be read, so no further diagnostic for it.
		return []Diag{diag(f.FailCode, f.Ref, f.FailLine, failMessage(f.FailCode))}
	}

	if _, err := discoverRoot(f.Abs, f.Ref); err != nil {
		if ce, ok := err.(*cliError); ok && ce.Diag.Code == "yass.root.not_found" {
			out = append(out, diag("yass.root.not_found", f.Ref, 0,
				"no "+RootBasename+" at or above path"))
		}
	}
	if baseName(f.Abs) == RootBasename && len(f.specDocs()) != 1 {
		out = append(out, diag("yass.root.spec_count", f.Ref, 0,
			"root file does not carry exactly one spec"))
	}

	names := map[string]bool{}
	for _, d := range f.Docs {
		out = append(out, a.checkDocumentShape(d)...)
		if d.Node != nil {
			out = append(out, checkNodeFeatures(d, d.Node)...)
		}
		if d.Addressable() {
			if names[d.Name] {
				out = append(out, diag("yass.document.name_duplicate", d.Target(), d.Line,
					"document name is already used in this file"))
			}
			names[d.Name] = true
		}
		switch d.Kind {
		case KindPreamble:
			if d.Index == 0 {
				out = append(out, checkPreamble(d)...)
			} else {
				out = append(out, diag("yass.document.unknown_kind", f.Ref, d.Line,
					"document carries neither a spec key nor a design key"))
			}
		case KindSpec:
			if d.Index == 0 {
				out = append(out, diag("yass.preamble.missing", f.Ref, d.Line,
					"first document is not a preamble"))
			}
			out = append(out, a.checkSpec(d)...)
		case KindDesign:
			if d.Index == 0 {
				out = append(out, diag("yass.preamble.missing", f.Ref, d.Line,
					"first document is not a preamble"))
			}
			out = append(out, checkDesign(d)...)
		}
	}
	return out
}

// checkDocumentShape reports duplicate keys anywhere in the document.
func (a *App) checkDocumentShape(d *Doc) []Diag {
	var out []Diag
	loc := d.File.Ref
	if d.Addressable() {
		loc = d.Target()
	}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n == nil {
			return
		}
		if n.Kind == yaml.MappingNode {
			seen := map[string]bool{}
			for i := 0; i+1 < len(n.Content); i += 2 {
				k := n.Content[i]
				if seen[k.Value] {
					out = append(out, diag("yass.document.duplicate_key", loc, k.Line,
						"mapping carries a duplicate key"))
				}
				seen[k.Value] = true
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(d.Node)
	return out
}

// checkNodeFeatures reports YAML anchors, aliases, and explicit tags.
func checkNodeFeatures(d *Doc, n *yaml.Node) []Diag {
	var out []Diag
	loc := d.File.Ref
	if d.Addressable() {
		loc = d.Target()
	}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n == nil {
			return
		}
		switch {
		case n.Kind == yaml.AliasNode:
			out = append(out, diag("yass.document.yaml_feature", loc, n.Line, "uses a yaml alias"))
		case n.Anchor != "":
			out = append(out, diag("yass.document.yaml_feature", loc, n.Line, "uses a yaml anchor"))
		case n.Style&yaml.TaggedStyle != 0:
			out = append(out, diag("yass.document.yaml_feature", loc, n.Line, "uses an explicit yaml tag"))
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(n)
	return out
}

func checkPreamble(d *Doc) []Diag {
	var out []Diag
	loc := d.File.Ref
	if !d.HasDescription || !d.HasVersion {
		out = append(out, diag("yass.preamble.incomplete", loc, d.Line,
			"preamble omits description or version"))
	}
	if d.HasVersion && d.Version != "v1" {
		out = append(out, diag("yass.preamble.version", loc, d.Line, "preamble version is not v1"))
	}
	for _, e := range d.Entries {
		if !contains(preambleKeys, e.Key) {
			out = append(out, diag("yass.preamble.unknown_key", loc, e.Kn.Line,
				"preamble carries an unrecognized key"))
		}
	}
	if d.RelatedNode != nil && !isStringSequence(d.RelatedNode) {
		out = append(out, diag("yass.preamble.related_shape", loc, d.RelatedNode.Line,
			"preamble related is not a sequence of strings"))
	}
	return out
}

func isStringSequence(n *yaml.Node) bool {
	if n.Kind != yaml.SequenceNode {
		return false
	}
	for _, item := range n.Content {
		if _, ok := stringOf(item); !ok {
			return false
		}
	}
	return true
}

func (a *App) checkSpec(d *Doc) []Diag {
	var out []Diag
	loc := d.File.Ref
	if d.Addressable() {
		loc = d.Target()
	}
	if !d.HasName || d.Name == "" {
		out = append(out, diag("yass.spec.unnamed", loc, d.Line, "spec key has no string name"))
	} else {
		if !nameGrammar.MatchString(d.Name) {
			out = append(out, diag("yass.spec.name_grammar", loc, d.Line,
				"spec name does not match the name grammar"))
		}
		if isReservedName(d.Name) {
			out = append(out, diag("yass.spec.name_keyword", loc, d.Line,
				"spec name matches a reserved keyword"))
		}
	}
	for _, e := range d.Entries {
		if e.Key == "spec" || isSlotName(e.Key) {
			continue
		}
		out = append(out, diag("yass.slot.unknown", loc, e.Kn.Line, "unrecognized slot key"))
	}
	for _, s := range d.Slots {
		if s.Vn == nil || s.Vn.Kind != yaml.SequenceNode {
			out = append(out, diag("yass.slot.not_a_list", s.Target(), s.Line,
				"slot value is not a sequence"))
			continue
		}
		for _, o := range s.Obligations {
			out = append(out, a.checkObligation(s, o)...)
		}
	}
	return out
}

func (a *App) checkObligation(s *Slot, o *Obligation) []Diag {
	var out []Diag
	loc := s.Target()
	if !o.Mapping {
		return []Diag{diag("yass.obligation.not_a_mapping", loc, o.Line, "obligation is not a mapping")}
	}
	hasRelation := false
	for _, e := range o.Entries {
		switch {
		case isNormativity(e.Key), e.Key == "WHEN":
			if !isScalarValue(e.Vn) {
				out = append(out, diag("yass.obligation.not_scalar", loc, e.Kn.Line,
					"obligation value is not a scalar"))
			}
		case isRelation(e.Key):
			hasRelation = true
		default:
			out = append(out, diag("yass.obligation.unknown_key", loc, e.Kn.Line,
				"obligation carries an unrecognized key"))
		}
	}
	if len(o.Keywords) > 1 {
		out = append(out, diag("yass.obligation.multiple_normativity", loc, o.Line,
			"obligation carries more than one normativity keyword"))
	}
	if o.Keyword == "" && !hasRelation {
		out = append(out, diag("yass.obligation.empty", loc, o.Line,
			"obligation carries neither a normativity keyword nor a reference"))
	}
	if o.HasGuard && o.Keyword == "" {
		out = append(out, diag("yass.obligation.guard_alone", loc, o.Line,
			"obligation carries a guard with no normativity keyword"))
	}
	for _, r := range o.Refs {
		out = append(out, a.checkRef(s, r)...)
	}
	return out
}

// isScalarValue reports whether a node sits at a scalar position: a mapping, a
// sequence, or null is a violation.
func isScalarValue(n *yaml.Node) bool {
	return n != nil && n.Kind == yaml.ScalarNode && n.Tag != "!!null"
}

// checkRef checks one reference against the ref-target grammar, then against
// the project.
func (a *App) checkRef(s *Slot, r *Ref) []Diag {
	loc := s.Target()
	if !r.IsString {
		return []Diag{diag("yass.ref.not_a_string", loc, r.Line, "reference value is not a string")}
	}
	t, ok := parseTarget(r.Target, false)
	if !ok {
		return []Diag{diag("yass.ref.syntax", loc, r.Line, "reference target is malformed")}
	}
	var out []Diag
	slotKnown := true
	if t.HasSlot && !isSlotName(t.Slot) {
		slotKnown = false
		out = append(out, diag("yass.ref.slot_unknown", loc, r.Line,
			"slot token is not a recognized slot"))
	}

	carrier := s.Doc.File
	abs := carrier.Abs
	if t.HasPath {
		abs = filePathFor(a.Root, filepath.Dir(carrier.Abs), t.Path)
	}
	target := carrier
	if abs != carrier.Abs {
		st, err := os.Stat(abs)
		if err != nil || !st.Mode().IsRegular() {
			return append(out, diag("yass.ref.unresolved", loc, r.Line, "reference target does not resolve"))
		}
		loaded, lerr := a.Loader.Load(abs)
		if lerr != nil || !loaded.Parsed {
			return append(out, diag("yass.ref.unresolved", loc, r.Line, "reference target does not resolve"))
		}
		target = loaded
	}
	if !target.Parsed {
		return append(out, diag("yass.ref.unresolved", loc, r.Line, "reference target does not resolve"))
	}

	var doc *Doc
	for _, d := range target.Docs {
		if d.Addressable() && d.Name == t.Name {
			doc = d
			break
		}
	}
	if doc == nil {
		return append(out, diag("yass.ref.unresolved", loc, r.Line, "reference target does not resolve"))
	}
	if t.HasSlot {
		if doc.Kind == KindDesign {
			out = append(out, diag("yass.ref.slot_on_design", loc, r.Line, "slot token addresses a design"))
		} else if slotKnown && !declaresSlot(doc, t.Slot) {
			out = append(out, diag("yass.ref.slot_undeclared", loc, r.Line,
				"referenced spec does not declare that slot"))
		}
	}
	switch r.Relation {
	case "CONFORMS":
		if doc.Kind == KindDesign {
			out = append(out, diag("yass.ref.conforms_design", loc, r.Line,
				"conforms target resolves to a design"))
		}
	case "USES":
		if doc.Kind != KindDesign {
			out = append(out, diag("yass.ref.uses_spec", loc, r.Line,
				"uses target resolves to a spec or slot"))
		}
	}
	return out
}

func checkDesign(d *Doc) []Diag {
	var out []Diag
	loc := d.File.Ref
	if d.Addressable() {
		loc = d.Target()
	}
	if !d.HasName || d.Name == "" {
		out = append(out, diag("yass.design.unnamed", loc, d.Line, "design key has no string name"))
	} else {
		if !nameGrammar.MatchString(d.Name) {
			out = append(out, diag("yass.design.name_grammar", loc, d.Line,
				"design name does not match the name grammar"))
		}
		if isReservedName(d.Name) {
			out = append(out, diag("yass.design.name_keyword", loc, d.Line,
				"design name matches a reserved keyword"))
		}
	}
	typeNode, contentNode := d.value("type"), d.value("content")
	if typeNode == nil || contentNode == nil {
		out = append(out, diag("yass.design.incomplete", loc, d.Line, "design omits type or content"))
	}
	if (typeNode != nil && !d.HasType) || (contentNode != nil && !d.HasContent) {
		out = append(out, diag("yass.design.value_type", loc, d.Line,
			"design type or content is not a string"))
	}
	for _, e := range d.Entries {
		if !contains(designKeys, e.Key) {
			out = append(out, diag("yass.design.unknown_key", loc, e.Kn.Line,
				"design carries an unrecognized key"))
		}
	}
	return out
}
