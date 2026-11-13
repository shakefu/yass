package yass

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// assertScalarReadsBack emits v as a mapping value and as a sequence item, at
// the top level and nested, and fails unless each reads back as v.
func assertScalarReadsBack(t *testing.T, v string) {
	t.Helper()
	for _, indent := range []string{"", "  "} {
		var vals []string
		emitScalarValue(&vals, indent, "k", v)
		doc := strings.Join(vals, "\n") + "\n"
		if indent != "" {
			doc = "m:\n" + doc
		}
		var m map[string]any
		if err := yaml.Unmarshal([]byte(doc), &m); err != nil {
			t.Fatalf("value %q does not parse: %v\n%s", v, err, doc)
		}
		got := m["k"]
		if indent != "" {
			inner, _ := m["m"].(map[string]any)
			got = inner["k"]
		}
		if got != v {
			t.Fatalf("value %q read back as %q:\n%s", v, got, doc)
		}

		var items []string
		emitScalarItem(&items, indent, v)
		doc = strings.Join(items, "\n") + "\n"
		if indent != "" {
			doc = "m:\n" + doc
		}
		var back any
		if err := yaml.Unmarshal([]byte(doc), &back); err != nil {
			t.Fatalf("item %q does not parse: %v\n%s", v, err, doc)
		}
		seq := back
		if indent != "" {
			seq = back.(map[string]any)["m"]
		}
		if s, _ := seq.([]any); len(s) != 1 || s[0] != v {
			t.Fatalf("item %q read back as %#v:\n%s", v, seq, doc)
		}
	}
}

// queryReadsBack queries root@A from a root file holding obligations, and
// fails unless the output reads back as the wanted obligations and re-emits
// identically once saved as a project root.
func queryReadsBack(t *testing.T, obligations string, want []map[string]string) {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", modeline+"---\ndescription: root\nversion: v1\n---\nspec: A\nRETURN:\n"+obligations)
	if got := run(dir, "validate"); got.status != ExitOK {
		t.Fatalf("fixture does not validate: %d\n%s%s", got.status, got.stdout, got.stderr)
	}
	got := run(dir, "query", "root@A")
	wantStatus(t, got, ExitOK)
	var spec struct {
		RETURN []map[string]string `yaml:"RETURN"`
	}
	if err := yaml.NewDecoder(strings.NewReader(got.stdout)).Decode(&spec); err != nil {
		t.Fatalf("query output does not read back: %v\n%s", err, got.stdout)
	}
	if len(spec.RETURN) != len(want) {
		t.Fatalf("read back %d obligations, want %d:\n%s", len(spec.RETURN), len(want), got.stdout)
	}
	for i, w := range want {
		for k, v := range w {
			if spec.RETURN[i][k] != v {
				t.Fatalf("obligation %d %s = %q, want %q:\n%s", i, k, spec.RETURN[i][k], v, got.stdout)
			}
		}
	}
	again := t.TempDir()
	write(t, again, "root.yass.yaml", modeline+"---\ndescription: root\nversion: v1\n"+got.stdout)
	if second := run(again, "query", "root@A"); second.status != ExitOK || second.stdout != got.stdout {
		t.Fatalf("saved output does not re-emit identically (%d):\n%s%s\nwant:\n%s",
			second.status, second.stdout, second.stderr, got.stdout)
	}
}

// A multi-line value with an indented line after an empty or whitespace-only
// first line pins its indentation in the block header, so the leading spaces
// survive and the stream stays well-formed.
func TestBlockScalarPinsIndentationOfAnyIndentedLine(t *testing.T) {
	for _, v := range []string{
		"\n  indented second line",
		"\n  \nz",
		"  \nz",
		"a\n  b",
		"a\n  ",
		"\n\n   deep\nshallow\n",
		" leading\nnext",
	} {
		assertScalarReadsBack(t, v)
	}
	queryReadsBack(t, `- MUST: "\n  indented second line"
- MUST: p
  WHEN: "\n  \nz"
`, []map[string]string{
		{"MUST": "\n  indented second line"},
		{"MUST": "p", "WHEN": "\n  \nz"},
	})
}

// A character outside YAML's printable set, or one a parser reads as a line
// break, is escaped in a double-quoted scalar rather than written raw, so the
// output stays a well-formed stream that reads back as the authored string.
func TestUnprintableCharactersAreEscaped(t *testing.T) {
	for _, v := range []string{
		"a\u0080b",
		"a\u0085b",
		"a\u009fb",
		"a\u2028b",
		"a\u2029b",
		"a\ufeffb",
		"a\ufffeb",
		"a\u009fb\nsecond",
		"line\u2028\nnext",
		"\ufeff",
		"\u0085",
	} {
		assertScalarReadsBack(t, v)
	}
	queryReadsBack(t, `- MUST: "a\x80b"
- MUST: "a\u2028b"
- MUST: "a\x85b"
- MUST: "a\ufeffb"
`, []map[string]string{
		{"MUST": "a\u0080b"},
		{"MUST": "a\u2028b"},
		{"MUST": "a\u0085b"},
		{"MUST": "a\ufeffb"},
	})
}

// A root description that is the string "-" is a value, not an absent field:
// overview writes it under the project line. A whitespace-only description
// still contributes no line.
func TestOverviewWritesDashDescription(t *testing.T) {
	pad := strings.Repeat(" ", overviewLabel)
	for _, tc := range []struct{ desc, want string }{
		{`"-"`, "\n" + pad + "-\nholds"},
		{`"  "`, "\nholds"},
	} {
		dir := t.TempDir()
		write(t, dir, "root.yass.yaml", modeline+"---\ndescription: "+tc.desc+"\nversion: v1\n---\nspec: A\nRETURN:\n- MUST: x\n")
		got := run(dir, "overview")
		wantStatus(t, got, ExitOK)
		project := strings.Index(got.stdout, "\nproject ")
		if project < 0 {
			t.Fatalf("no project line:\n%s", got.stdout)
		}
		rest := got.stdout[project+1:]
		rest = rest[strings.Index(rest, "\n"):]
		if !strings.HasPrefix(rest, tc.want) {
			t.Fatalf("description %s: after the project line got %q, want prefix %q", tc.desc, rest, tc.want)
		}
	}
}
