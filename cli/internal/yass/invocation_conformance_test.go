package yass

import (
	"bytes"
	"strings"
	"testing"
)

// mainIn runs the program with exactly args, adding no -C of its own.
func mainIn(args ...string) result {
	var o, e bytes.Buffer
	st := Main(args, &o, &e)
	return result{o.String(), e.String(), st}
}

// An empty -C value is reported like an empty operand, never read as the
// process working directory.
func TestEmptyStartDirIsUsageError(t *testing.T) {
	dir := project(t)
	for _, args := range [][]string{
		{"-C", "", "root"},
		{"-C=", "root"},
		{"-C", dir, "-C", "", "root"},
	} {
		got := mainIn(args...)
		wantStatus(t, got, ExitUsage)
		if !strings.Contains(got.stderr, "yass.args.empty") {
			t.Errorf("%q: stderr = %q, want yass.args.empty", args, got.stderr)
		}
		if got.stdout != "" {
			t.Errorf("%q: stdout should be empty, got %q", args, got.stdout)
		}
	}
	// The last occurrence still wins, so a later non-empty value is accepted.
	if got := mainIn("-C", "", "-C", dir, "root"); got.status != ExitOK {
		t.Fatalf("later -C did not override the empty one: %d %q", got.status, got.stderr)
	}
}
