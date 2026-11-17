package yass

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The registry must document exactly the codes both the normative lint
// contract defines and production emission sites use. Reading Go syntax
// rather than grepping strings excludes IDs mentioned only in documentation.
func TestLintExplanationCompleteness(t *testing.T) {
	registered := map[string]bool{}
	var ordered []string
	previous := ""
	for _, e := range lintExplanations {
		if e.ID <= previous {
			t.Errorf("registry ID %q duplicates or precedes %q", e.ID, previous)
		}
		previous = e.ID
		registered[e.ID] = true
		ordered = append(ordered, e.ID)
		for name, text := range map[string]string{"detects": e.Detects, "why": e.Why, "remedy": e.Remedy, "bad": e.Bad, "good": e.Good} {
			if strings.TrimSpace(text) == "" {
				t.Errorf("%s has no %s", e.ID, name)
			}
		}
	}

	if declared := explanationRuleIDs(t); !reflect.DeepEqual(ordered, declared) {
		t.Errorf("registry order %v differs from RuleIds %v", ordered, declared)
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
		codes, err := lintEmissionCodes(name, file)
		if err != nil {
			t.Fatal(err)
		}
		for code := range codes {
			emitted[code] = true
		}
	}
	if !reflect.DeepEqual(registered, emitted) {
		t.Errorf("registry %v differs from emitting code %v", sortedExplanationIDs(registered), sortedExplanationIDs(emitted))
	}
}

// RuleIds owns recognition and index order; the numeric order fields are part
// of the contract, rather than a second hardcoded expectation in a test.
func explanationRuleIDs(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile("../../13-explain.yass.yaml")
	if err != nil {
		t.Fatal(err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var node yaml.Node
		if err := dec.Decode(&node); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		var header struct{ Design string }
		if err := node.Decode(&header); err != nil {
			t.Fatal(err)
		}
		if header.Design != "RuleIds" {
			continue
		}
		var catalog struct {
			Content map[string]struct{ Order int }
		}
		if err := node.Decode(&catalog); err != nil {
			t.Fatal(err)
		}
		ids := make([]string, len(catalog.Content))
		for id, rule := range catalog.Content {
			if rule.Order < 1 || rule.Order > len(ids) || ids[rule.Order-1] != "" {
				t.Fatalf("invalid or duplicate RuleIds order %d for %s", rule.Order, id)
			}
			ids[rule.Order-1] = id
		}
		return ids
	}
	t.Fatal("13-explain.yass.yaml holds no RuleIds catalog")
	return nil
}

// Only the three local warn closures may pass a code variable into a Diag:
// all their callers are checked below. A different dynamic code expression
// cannot silently disappear from the completeness comparison.
func lintEmissionCodes(name string, file *ast.File) (map[string]bool, error) {
	passThrough := map[ast.Expr]bool{}
	switch name {
	case "cmd_lint.go", "lint_prose.go", "lint_design.go":
		ast.Inspect(file, func(n ast.Node) bool {
			assignment, ok := n.(*ast.AssignStmt)
			if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 {
				return true
			}
			fn, ok := assignment.Lhs[0].(*ast.Ident)
			body, literal := assignment.Rhs[0].(*ast.FuncLit)
			if !ok || fn.Name != "warn" || !literal || len(body.Type.Params.List) == 0 {
				return true
			}
			params := body.Type.Params.List[0].Names
			if len(params) == 0 || params[0].Name != "code" {
				return true
			}
			ast.Inspect(body.Body, func(n ast.Node) bool {
				if field, ok := n.(*ast.KeyValueExpr); ok {
					key, isKey := field.Key.(*ast.Ident)
					value, isValue := field.Value.(*ast.Ident)
					if isKey && key.Name == "Code" && isValue && value.Name == "code" {
						passThrough[field.Value] = true
					}
				}
				return true
			})
			return true
		})
	}
	codes := map[string]bool{}
	lintSource := name == "cmd_lint.go" || strings.HasPrefix(name, "lint_")
	var scanErr error
	ast.Inspect(file, func(n ast.Node) bool {
		var code ast.Expr
		switch node := n.(type) {
		case *ast.CallExpr:
			if fn, ok := node.Fun.(*ast.Ident); ok && fn.Name == "warn" && len(node.Args) > 0 {
				code = node.Args[0]
			}
		case *ast.CompositeLit:
			if typ, ok := node.Type.(*ast.Ident); ok && typ.Name == "Diag" {
				warning := false
				for _, element := range node.Elts {
					if field, ok := element.(*ast.KeyValueExpr); ok {
						if key, ok := field.Key.(*ast.Ident); ok {
							switch key.Name {
							case "Code":
								code = field.Value
							case "Severity":
								if lit, ok := field.Value.(*ast.BasicLit); ok {
									value, _ := strconv.Unquote(lit.Value)
									warning = value == "warn"
								}
							}
						}
					}
				}
				if !warning && !lintSource {
					code = nil // request failures and validation are not lint emissions
				}
			}
		}
		if code == nil || passThrough[code] {
			return true
		}
		literal, ok := code.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			scanErr = fmt.Errorf("%s: non-literal lint emission code at position %d", name, code.Pos())
			return false
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			scanErr = err
		} else if strings.HasPrefix(value, "yass.lint.") {
			codes[value] = true
		}
		return true
	})
	return codes, scanErr
}

func TestLintEmissionScanRejectsDynamicCodes(t *testing.T) {
	for _, source := range []string{
		`package yass; func f(code string) { warn(code, "", 1, "") }`,
		`package yass; var d = Diag{Severity: "warn", Code: "yass.lint." + suffix}`,
	} {
		file, err := parser.ParseFile(token.NewFileSet(), "lint_extra.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := lintEmissionCodes("lint_extra.go", file); err == nil {
			t.Errorf("accepted dynamic code in %s", source)
		}
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
					if !example.finding {
						if got.status != ExitOK || got.stdout != "" {
							t.Errorf("good example has lint findings: %+v", got)
						}
						validated := run(dir, "validate")
						if validated.status != ExitOK || validated.stdout != "" || validated.stderr != "" {
							t.Errorf("good example has validation findings: %+v", validated)
						}
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
