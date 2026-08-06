package yass

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
	"syscall"
	"unicode"
)

// Diag is the five-field record form of root@DiagnosticFormat:
// SEVERITY, CODE, LOCATION, LINE, MESSAGE.
type Diag struct {
	Severity string // "error" or "warn"
	Code     string
	Location string // most precise ref target known, or "" when none
	Line     int    // 1-based, or 0 when no line is known
	Message  string
}

// Record renders the diagnostic under the segmentation rules of
// root@RecordFormat: TAB-separated fields, "-" for an absent field.
func (d Diag) Record() string {
	line := ""
	if d.Line > 0 {
		line = strconv.Itoa(d.Line)
	}
	return record(d.Severity, d.Code, d.Location, line, d.Message)
}

// record joins fields with one TAB, collapsing whitespace within each field
// and writing "-" for a field that is empty after collapsing.
func record(fields ...string) string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = field(f)
	}
	return strings.Join(out, "\t")
}

// field collapses each run of whitespace to one ASCII space, trims, and
// substitutes "-" for an empty result, so "-" is the only spelling of an
// absent field.
func field(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	started := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = started
			continue
		}
		if space {
			b.WriteByte(' ')
		}
		space = false
		started = true
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return "-"
	}
	return b.String()
}

// stream wraps one output stream, remembering the first write failure so a
// command can select an exit status once rather than at every write.
type stream struct {
	w      *bufio.Writer
	broken bool // the reading end of the pipe is closed
	failed bool // any other write failure
}

func newStream(w io.Writer) *stream {
	return &stream{w: bufio.NewWriter(w)}
}

// line writes one line terminated by exactly one LF, then flushes, so a
// consumer reading incrementally sees whole records.
func (s *stream) line(text string) {
	if s.broken || s.failed {
		return
	}
	if _, err := s.w.WriteString(text); err != nil {
		s.note(err)
		return
	}
	if err := s.w.WriteByte('\n'); err != nil {
		s.note(err)
		return
	}
	if err := s.w.Flush(); err != nil {
		s.note(err)
	}
}

func (s *stream) note(err error) {
	if errors.Is(err, syscall.EPIPE) {
		s.broken = true
		return
	}
	s.failed = true
}

func (s *stream) flush() {
	if s.broken || s.failed {
		return
	}
	if err := s.w.Flush(); err != nil {
		s.note(err)
	}
}
