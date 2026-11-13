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

// The YAML 1.2 core schema's resolution of a plain scalar: null, bool, int, and
// float by these patterns, and str for everything else.
var (
	coreNull  = regexp.MustCompile(`^(?:~|null|Null|NULL|)$`)
	coreBool  = regexp.MustCompile(`^(?:true|True|TRUE|false|False|FALSE)$`)
	coreInt   = regexp.MustCompile(`^(?:[-+]?[0-9]+|0o[0-7]+|0x[0-9a-fA-F]+)$`)
	coreFloat = regexp.MustCompile(`^(?:[-+]?(?:\.[0-9]+|[0-9]+(?:\.[0-9]*)?)(?:[eE][-+]?[0-9]+)?|[-+]?\.(?:inf|Inf|INF)|\.(?:nan|NaN|NAN))$`)
)

// Loader reads each file at most once per invocation and hands back the model
// every command reads. It is the only place this program reads a spec file.
type Loader struct {
	root  string // absolute project root, "" when none has been discovered
	files map[string]*File
}

func newLoader(root string) *Loader {
	return &Loader{root: root, files: map[string]*File{}}
}

// Load reads and parses one file, returning the cached model when the file has
// already been read this invocation. A read failure is an environment error.
// The file is keyed and named by its canonical path, so every spelling of one
// file — `./`, `a/./b`, `x/../`, a symbolically linked directory — yields the
// same *File, and its Rel and Ref are computed from that canonical path.
func (l *Loader) Load(abs string) (*File, error) {
	abs = canonicalPath(abs)
	if f, ok := l.files[abs]; ok {
		return f, nil
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, unreadable(relTo(l.root, abs))
	}
	f := parseBytes(abs, relTo(l.root, abs), data)
	l.files[abs] = f
	return f, nil
}

// canonicalPath names a file by the directory that physically holds it: the
// directory part has its symbolic links, `.`, and `..` resolved the way the
// kernel resolves them, while the basename is kept, so a symbolic link naming a
// regular file stays a file of its own name. When the directory cannot be
// resolved the path is cleaned lexically instead.
func canonicalPath(abs string) string {
	if dir, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		return filepath.Join(dir, filepath.Base(abs))
	}
	return filepath.Clean(abs)
}

func refOf(rel string) string { return strings.TrimSuffix(rel, SpecSuffix) }

// parseBytes turns a file's bytes into documents, or into a parse failure.
func parseBytes(abs, rel string, data []byte) *File {
	f := &File{Abs: abs, Rel: rel, Ref: refOf(rel), FirstLine: firstLine(data), Lines: fileLines(data)}

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
			f.FailLine = errLine(err, data)
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
			retagCore(content)
		}
		f.Docs = append(f.Docs, buildDoc(f, i, doc, content))
	}
	return f
}

// fileLines returns the file's lines without their terminators.
func fileLines(data []byte) []string {
	s := strings.TrimSuffix(string(data), "\n")
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	return lines
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

// yamlUnknownAnchor pulls the alias name out of go-yaml's undefined-alias error.
var yamlUnknownAnchor = regexp.MustCompile(`unknown anchor '([^']*)' referenced`)

// errLine yields the 1-based line a parse failure stopped on. go-yaml names the
// line for most failures; for one it does not, the line is found in data: the
// first character YAML disallows, the first use of an undefined alias, and
// otherwise the stream's last line. It is never 0.
func errLine(err error, data []byte) int {
	msg := err.Error()
	if m := yamlErrLine.FindStringSubmatch(msg); m != nil {
		if n, e := strconv.Atoi(m[1]); e == nil && n > 0 {
			return n
		}
	}
	if strings.Contains(msg, "control characters are not allowed") {
		if n := disallowedCharLine(data); n > 0 {
			return n
		}
	}
	if m := yamlUnknownAnchor.FindStringSubmatch(msg); m != nil {
		alias := regexp.MustCompile(`\*` + regexp.QuoteMeta(m[1]) + `(?:$|[\s,\[\]{}])`)
		if loc := alias.FindIndex(data); loc != nil {
			return bytes.Count(data[:loc[0]], []byte{'\n'}) + 1
		}
	}
	if n := len(fileLines(data)); n > 0 {
		return n
	}
	return 1
}

// disallowedCharLine reports the 1-based line holding the first character
// outside YAML's printable set, or 0 when there is none.
func disallowedCharLine(data []byte) int {
	line := 1
	for _, r := range string(data) {
		switch {
		case r == '\n':
			line++
		case r == '\t' || r == '\r' || r == 0x85:
		case r < 0x20 || r == 0x7F || r >= 0x80 && r < 0xA0 || r == 0xFFFE || r == 0xFFFF:
			return line
		}
	}
	return 0
}

// retagCore resolves every untagged plain scalar under the YAML 1.2 core
// schema. go-yaml applies YAML 1.1 resolutions the core schema lacks, such as
// timestamps, `0b` binary, `_`-separated numbers, and the `<<` merge key; under
// the core schema each of those, like `yes`, `no`, `on`, and `off`, is a
// string. A scalar carrying an explicit tag, or written quoted or as a block,
// keeps the tag go-yaml gave it.
func retagCore(n *yaml.Node) {
	if n == nil {
		return
	}
	if n.Kind == yaml.ScalarNode && n.Style == 0 {
		n.Tag = coreTag(n.Value)
	}
	for _, c := range n.Content {
		retagCore(c)
	}
}

// coreTag yields the tag the YAML 1.2 core schema resolves a plain scalar to.
func coreTag(v string) string {
	switch {
	case coreNull.MatchString(v):
		return "!!null"
	case coreBool.MatchString(v):
		return "!!bool"
	case coreInt.MatchString(v):
		return "!!int"
	case coreFloat.MatchString(v):
		return "!!float"
	}
	return "!!str"
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
		d.Format, d.HasFormat = stringOf(d.value("format"))
		d.Content, d.HasContent = stringOf(d.value("content"))
		if c := d.value("content"); c != nil && c.Kind == yaml.MappingNode {
			d.IsCatalog = true
			d.Catalog = buildRecords(c)
		}
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

// buildRecords reads a design's mapping content as catalogue records, keeping
// every authored pair, including a repeated identifier, in authored order.
func buildRecords(c *yaml.Node) []*Record {
	var out []*Record
	for i := 0; i+1 < len(c.Content); i += 2 {
		kn, vn := c.Content[i], c.Content[i+1]
		r := &Record{Key: kn.Value, Kn: kn, Vn: vn}
		if vn.Kind == yaml.MappingNode {
			for j := 0; j+1 < len(vn.Content); j += 2 {
				r.Fields = append(r.Fields, Field{Name: vn.Content[j].Value, Kn: vn.Content[j], Vn: vn.Content[j+1]})
			}
		}
		out = append(out, r)
	}
	return out
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
				o.Guard, o.GuardString = stringOf(vn)
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
