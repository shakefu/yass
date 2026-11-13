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

// --version does not mask a malformed subcommand token or its arguments; a
// bare --version, or one beside a well-formed vector, still prints the line.
func TestVersionDoesNotMaskMalformedVector(t *testing.T) {
	dir := project(t)
	cases := []struct {
		args []string
		code string
	}{
		{[]string{"-V", "bogus"}, "yass.args.unknown_subcommand"},
		{[]string{"--version", "root", "--bogus"}, "yass.args.unknown_option"},
		{[]string{"-V", "root", "a", "b"}, "yass.args.extra_operand"},
		{[]string{"-V", "query"}, "yass.args.missing_operand"},
		{[]string{"-V", "list", ""}, "yass.args.empty"},
		{[]string{"-V", "--", "bogus"}, "yass.args.unknown_subcommand"},
	}
	for _, c := range cases {
		got := run(dir, c.args...)
		wantStatus(t, got, ExitUsage)
		if !strings.Contains(got.stderr, c.code) {
			t.Errorf("%v: stderr = %q, want code %s", c.args, got.stderr, c.code)
		}
		if got.stdout != "" {
			t.Errorf("%v: stdout should be empty, got %q", c.args, got.stdout)
		}
	}
	want := programName + " " + programVersion + " " + languageVersion + "\n"
	for _, args := range [][]string{{"-V"}, {"--version", "root"}, {"-V", "list", "--format", "json"}} {
		got := run(dir, args...)
		wantStatus(t, got, ExitOK)
		if got.stdout != want {
			t.Errorf("%v: stdout = %q, want %q", args, got.stdout, want)
		}
	}
}
