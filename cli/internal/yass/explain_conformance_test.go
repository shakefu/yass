package yass

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// yass: 13-explain@Explain::RETURN
func TestExplainIndex(t *testing.T) {
	want := strings.Join(explanationRuleIDs(t), "\n") + "\n"
	got := run(t.TempDir(), "explain")
	wantStatus(t, got, ExitOK)
	if got.stdout != want || got.stderr != "" {
		t.Fatalf("index = %q, stderr = %q; want %q", got.stdout, got.stderr, want)
	}
}

// yass: 13-explain@Explain::INPUT
// yass: 13-explain@Explain::ERROR
func TestExplainUnknownIDIsExactAndDoesNotEnumerate(t *testing.T) {
	for _, id := range []string{"nope", "open_question", "yass.lint.open", "YASS.LINT.OPEN_QUESTION", "yass.lint.open_question ", "yass.docs.unknown", "yass.validate.invalid"} {
		got := run(t.TempDir(), "explain", id)
		wantStatus(t, got, ExitUnresolved)
		want := record("error", "yass.explain.unknown", "", "", "no lint rule named "+id) + "\n"
		if got.stdout != "" || got.stderr != want {
			t.Errorf("%q: stdout = %q, stderr = %q; want empty stdout and %q", id, got.stdout, got.stderr, want)
		}
	}
}

// yass: 13-explain@Explain::INPUT
func TestExplainInvocation(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code string
	}{
		{[]string{"explain", "a", "b"}, "yass.args.extra_operand"},
		{[]string{"explain", "--format", "json"}, "yass.args.unknown_option"},
		{[]string{"explain", "--raw"}, "yass.args.unknown_option"},
		{[]string{"explain", ""}, "yass.args.empty"},
	} {
		got := run(t.TempDir(), tc.args...)
		wantStatus(t, got, ExitUsage)
		if got.stdout != "" || !strings.Contains(got.stderr, tc.code) {
			t.Errorf("%v: stdout = %q, stderr = %q; want %s", tc.args, got.stdout, got.stderr, tc.code)
		}
	}
}

// yass: 13-explain@Explain::SIDE-EFFECT
// yass: 13-explain@Explain::INVARIANT
func TestExplainReadsNoTree(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "root.yass.yaml", "not: [valid yaml")
	starts := []string{dir, filepath.Join(dir, "missing")}
	for _, start := range starts {
		for _, args := range [][]string{{"explain"}, {"explain", "yass.lint.open_question"}} {
			got := run(start, args...)
			wantStatus(t, got, ExitOK)
			if got.stderr != "" || got.stdout == "" {
				t.Fatalf("%s %v: stdout = %q, stderr = %q", start, args, got.stdout, got.stderr)
			}
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "root.yass.yaml"))
	if err != nil || string(data) != "not: [valid yaml" {
		t.Fatalf("file changed: %q, %v", data, err)
	}
}

// yass: 13-explain@Explain::RETURN
func TestExplainServesEveryEntryWithinBudget(t *testing.T) {
	for _, rule := range lintExplanations {
		for name, value := range map[string]string{"Detects": rule.Detects, "Why": rule.Why, "Resolve": rule.Remedy} {
			if strings.ContainsAny(value, "\r\n") {
				t.Errorf("%s %s must occupy one line", rule.ID, name)
			}
		}
		got := run(t.TempDir(), "explain", rule.ID)
		wantStatus(t, got, ExitOK)
		if got.stderr != "" || got.stdout != rule.text() {
			t.Fatalf("%s: stdout = %q, stderr = %q", rule.ID, got.stdout, got.stderr)
		}
		if countLines(got.stdout) > LineBudget {
			t.Errorf("%s exceeds %d lines", rule.ID, LineBudget)
		}
		for _, required := range []string{rule.ID + "\n\n", "Detects: ", "Why: ", "Resolve: ", "Bad:\n```yaml\n", "Good:\n```yaml\n"} {
			if !strings.Contains(got.stdout, required) {
				t.Errorf("%s lacks %q", rule.ID, required)
			}
		}
	}
}

// yass: root@ExitPolicy
func TestExplainOutputFailure(t *testing.T) {
	for _, args := range [][]string{{"explain"}, {"explain", "yass.lint.open_question"}} {
		var stderr bytes.Buffer
		if status := Main(args, failingExplainWriter{}, &stderr); status != ExitEnvironment {
			t.Errorf("%v: status = %d, want %d", args, status, ExitEnvironment)
		}
	}
}

type failingExplainWriter struct{}

func (failingExplainWriter) Write([]byte) (int, error) { return 0, os.ErrPermission }
