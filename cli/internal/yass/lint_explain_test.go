package yass

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The registry must document exactly the codes both the normative lint
// contract defines and production emission sites use. Reading Go syntax
// rather than grepping strings excludes IDs mentioned only in documentation.
func TestLintExplanationCompleteness(t *testing.T) {
	registered := map[string]bool{}
	previous := ""
	for _, e := range lintExplanations {
		if e.ID <= previous {
			t.Errorf("registry ID %q duplicates or precedes %q", e.ID, previous)
		}
		previous = e.ID
		registered[e.ID] = true
		for name, text := range map[string]string{"detects": e.Detects, "why": e.Why, "remedy": e.Remedy, "bad": e.Bad, "good": e.Good} {
			if strings.TrimSpace(text) == "" {
				t.Errorf("%s has no %s", e.ID, name)
			}
		}
	}

	contract, err := os.ReadFile("../../09-lint.yass.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defined := map[string]bool{}
	for _, match := range regexp.MustCompile(`(?m)^\s+MUST:.*emit code (yass\.lint\.[a-z_]+)`).FindAllStringSubmatch(string(contract), -1) {
		defined[match[1]] = true
	}
	if !reflect.DeepEqual(registered, defined) {
		t.Errorf("registry %v differs from lint contract %v", sortedExplanationIDs(registered), sortedExplanationIDs(defined))
	}

	emitted := map[string]bool{}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			var code ast.Expr
			switch node := n.(type) {
			case *ast.CallExpr:
				if fn, ok := node.Fun.(*ast.Ident); ok && fn.Name == "warn" && len(node.Args) > 0 {
					code = node.Args[0]
				}
			case *ast.CompositeLit:
				if typ, ok := node.Type.(*ast.Ident); ok && typ.Name == "Diag" {
					for _, element := range node.Elts {
						if field, ok := element.(*ast.KeyValueExpr); ok {
							if key, ok := field.Key.(*ast.Ident); ok && key.Name == "Code" {
								code = field.Value
							}
						}
					}
				}
			}
			if literal, ok := code.(*ast.BasicLit); ok && literal.Kind == token.STRING {
				value, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(value, "yass.lint.") {
					emitted[value] = true
				}
			}
			return true
		})
	}
	if !reflect.DeepEqual(registered, emitted) {
		t.Errorf("registry %v differs from emitting code %v", sortedExplanationIDs(registered), sortedExplanationIDs(emitted))
	}
}

func sortedExplanationIDs(set map[string]bool) []string {
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Every published example runs as root.yass.yaml in its own project. This
// exercises graph rules as well as text patterns without wrapping the example
// in additional obligations that could change its meaning.
func TestLintExplanationExamples(t *testing.T) {
	for _, e := range lintExplanations {
		t.Run(e.ID, func(t *testing.T) {
			for _, example := range []struct {
				name, text string
				finding    bool
			}{{"bad", e.Bad, true}, {"good", e.Good, false}} {
				t.Run(example.name, func(t *testing.T) {
					dir := t.TempDir()
					write(t, dir, "root.yass.yaml", example.text)
					got := run(dir, "lint")
					if got.stderr != "" || (got.status != ExitOK && got.status != ExitFindings) {
						t.Fatalf("lint failed: %+v", got)
					}
					found := false
					for _, line := range lines(got.stdout) {
						fields := strings.Split(line, "\t")
						if len(fields) == 5 && fields[1] == e.ID {
							found = true
						}
					}
					if found != example.finding {
						t.Errorf("finding %s present = %v, want %v; output:\n%s", e.ID, found, example.finding, got.stdout)
					}
				})
			}
		})
	}
}
