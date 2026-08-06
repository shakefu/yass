package yass

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// bom is the UTF-8 byte order mark, which a yass file must not begin with.
var bom = []byte{0xEF, 0xBB, 0xBF}

// yamlErrLine pulls the 1-based line out of a go-yaml parse error.
var yamlErrLine = regexp.MustCompile(`line (\d+):`)

// norwayWords are the plain scalars that must stay strings rather than become
// booleans.
var norwayWords = map[string]bool{
	"yes": true, "Yes": true, "YES": true,
	"no": true, "No": true, "NO": true,
	"on": true, "On": true, "ON": true,
	"off": true, "Off": true, "OFF": true,
}

// Loader reads each file at most once per invocation and hands back the model
// every command reads. It is the only place this program reads a spec file.
type Loader struct {
	root  string // absolute project root, "" when none has been discovered
	files map[string]*File
}

func newLoader(root string) *Loader {
	return &Loader{root: root, files: map[string]*File{}}
}

// relOf renders an absolute path project-root-relative with `/` separators.
func (l *Loader) relOf(abs string) string {
	if l.root == "" {
		return filepath.ToSlash(abs)
	}
	rel, err := filepath.Rel(l.root, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

// Load reads and parses one file, returning the cached model when the file has
// already been read this invocation. A read failure is an environment error.
func (l *Loader) Load(abs string) (*File, error) {
	if f, ok := l.files[abs]; ok {
		return f, nil
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, unreadable(l.relOf(abs))
	}
	f := parseBytes(abs, l.relOf(abs), data)
	l.files[abs] = f
	return f, nil
}

// Loaded reports the already-loaded model for a path, if any.
func (l *Loader) Loaded(abs string) (*File, bool) {
	f, ok := l.files[abs]
	return f, ok
}

func refOf(rel string) string { return strings.TrimSuffix(rel, SpecSuffix) }

// parseBytes turns a file's bytes into documents, or into a parse failure.
func parseBytes(abs, rel string, data []byte) *File {
	f := &File{Abs: abs, Rel: rel, Ref: refOf(rel), FirstLine: firstLine(data)}

	if bytes.HasPrefix(data, bom) {
		f.FailCode = "yass.document.encoding"
		f.FailLine = 1
		return f
	}
	if !utf8.Valid(data) {
		f.FailCode = "yass.document.encoding"
		f.FailLine = invalidUTF8Line(data)
		return f
	}

	var nodes []*yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var n yaml.Node
		err := dec.Decode(&n)
		if err == io.EOF {
			break
		}
		if err != nil {
			f.FailCode = "yass.document.not_yaml"
			f.FailLine = errLine(err)
			return f
		}
		nodes = append(nodes, &n)
	}

	f.Parsed = true
	for i, doc := range nodes {
		content := doc
		if doc.Kind == yaml.DocumentNode {
			if len(doc.Content) == 0 {
				content = nil
			} else {
				content = doc.Content[0]
			}
		}
		if content != nil {
			retagNorway(content)
		}
		f.Docs = append(f.Docs, buildDoc(f, i, doc, content))
	}
	return f
}

func firstLine(data []byte) string {
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		data = data[:i]
	}
	return strings.TrimSuffix(string(data), "\r")
}

// invalidUTF8Line reports the 1-based line holding the first invalid byte.
func invalidUTF8Line(data []byte) int {
	line := 1
	for i := 0; i < len(data); {
		if data[i] == '\n' {
			line++
			i++
			continue
		}
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			return line
		}
		i += size
	}
	return line
}

func errLine(err error) int {
	if m := yamlErrLine.FindStringSubmatch(err.Error()); m != nil {
		if n, e := strconv.Atoi(m[1]); e == nil {
			return n
		}
	}
	return 0
}

// retagNorway keeps `yes`, `no`, `on`, and `off` plain strings. go-yaml's YAML
// 1.2 core schema already resolves them as strings; this walk makes the
// property of the model rather than of the library.
func retagNorway(n *yaml.Node) {
	if n == nil {
		return
	}
	if n.Kind == yaml.ScalarNode && n.Tag == "!!bool" && n.Style == 0 && norwayWords[n.Value] {
		n.Tag = "!!str"
	}
	for _, c := range n.Content {
		retagNorway(c)
	}
}

func buildDoc(f *File, index int, docNode, content *yaml.Node) *Doc {
	d := &Doc{File: f, Index: index, Kind: KindPreamble, Node: content}
	if content == nil {
		d.Line = docNode.Line
		return d
	}
	d.Line = content.Line
	if content.Kind != yaml.MappingNode {
		return d
	}
	for i := 0; i+1 < len(content.Content); i += 2 {
		kn, vn := content.Content[i], content.Content[i+1]
		d.Entries = append(d.Entries, Entry{Key: kn.Value, Kn: kn, Vn: vn})
	}

	switch {
	case d.hasKey("spec"):
		d.Kind = KindSpec
		d.Name, d.HasName = stringOf(d.value("spec"))
	case d.hasKey("design"):
		d.Kind = KindDesign
		d.Name, d.HasName = stringOf(d.value("design"))
	}

	switch d.Kind {
	case KindPreamble:
		d.Description, d.HasDescription = stringOf(d.value("description"))
		d.Version, d.HasVersion = stringOf(d.value("version"))
		d.RelatedNode = d.value("related")
	case KindDesign:
		d.Type, d.HasType = stringOf(d.value("type"))
		d.Content, d.HasContent = stringOf(d.value("content"))
	case KindSpec:
		for _, e := range d.Entries {
			if !isSlotName(e.Key) {
				continue
			}
			s := &Slot{Doc: d, Name: e.Key, Index: len(d.Slots), Line: e.Kn.Line, Kn: e.Kn, Vn: e.Vn}
			if e.Vn != nil && e.Vn.Kind == yaml.SequenceNode {
				for _, item := range e.Vn.Content {
					s.Obligations = append(s.Obligations, buildObligation(s, len(s.Obligations), item))
				}
			}
			d.Slots = append(d.Slots, s)
		}
	}
	return d
}

func buildObligation(s *Slot, index int, item *yaml.Node) *Obligation {
	o := &Obligation{Slot: s, Index: index, Line: item.Line, Node: item}
	if item.Kind != yaml.MappingNode {
		return o
	}
	o.Mapping = true
	for i := 0; i+1 < len(item.Content); i += 2 {
		kn, vn := item.Content[i], item.Content[i+1]
		key := kn.Value
		o.Entries = append(o.Entries, Entry{Key: key, Kn: kn, Vn: vn})
		switch {
		case isNormativity(key):
			if !contains(o.Keywords, key) {
				o.Keywords = append(o.Keywords, key)
			}
			if o.Keyword == "" {
				o.Keyword = key
				o.Prose, o.HasProse = stringOf(vn)
			}
		case key == "WHEN":
			if !o.HasGuard {
				o.HasGuard = true
				o.Guard, o.GuardScalar = stringOf(vn)
			}
		case isRelation(key):
			target, ok := stringOf(vn)
			o.Refs = append(o.Refs, &Ref{Obl: o, Relation: key, Target: target, IsString: ok, Line: vn.Line, Node: vn})
		}
	}
	return o
}

func (d *Doc) hasKey(key string) bool { return d.value(key) != nil }

func (d *Doc) value(key string) *yaml.Node {
	for _, e := range d.Entries {
		if e.Key == key {
			return e.Vn
		}
	}
	return nil
}

// stringOf reports a node's value when it is a plain string scalar.
func stringOf(n *yaml.Node) (string, bool) {
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag != "!!str" {
		return "", false
	}
	return n.Value, true
}
