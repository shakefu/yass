package yass

import (
	"bufio"
	"bytes"
	"encoding/json"
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

// Field names of each command's records, in field order: the lowercase names
// root@RecordFormat and root@DiagnosticFormat give them, and the keys of the
// JSON form root@JsonRecordFormat fixes.
var (
	rootFields   = []string{"path"}
	listFields   = []string{"ref", "kind", "detail", "description"}
	findFields   = []string{"ref", "what", "text"}
	refsFields   = []string{"dir", "relation", "ref"}
	updateFields = []string{"action", "from", "to"}
	diagFields   = []string{"severity", "code", "location", "line", "message"}
)

// record renders one standard-output record in the format the invocation
// selected: the text form of root@RecordFormat, or one JSON object.
func (a *App) record(names []string, fields ...string) string {
	if a.Inv.Format != FormatJSON {
		return record(fields...)
	}
	return jsonRecord(names, fields, -1)
}

// diag renders one diagnostic written to standard output in the selected
// format. Standard error stays text, so it calls Diag.Record directly.
func (a *App) diag(d Diag) string {
	if a.Inv.Format != FormatJSON {
		return d.Record()
	}
	return jsonRecord(diagFields, []string{d.Severity, d.Code, d.Location, strconv.Itoa(d.Line), d.Message}, 3)
}

// jsonRecord writes one compact JSON object whose keys are names in order and
// whose values are the text form's fields: a string, or null where the field
// is empty after collapsing, so a real "-" stays a string. The field at index lineAt is the LINE field, written as an
// integer, or null when it is not positive.
func jsonRecord(names, fields []string, lineAt int) string {
	var b strings.Builder
	b.WriteByte('{')
	for i, name := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(jsonString(name))
		b.WriteByte(':')
		if i == lineAt {
			if n, err := strconv.Atoi(fields[i]); err == nil && n > 0 {
				b.WriteString(strconv.Itoa(n))
			} else {
				b.WriteString("null")
			}
			continue
		}
		if strings.Join(strings.Fields(fields[i]), "") == "" {
			b.WriteString("null")
		} else {
			b.WriteString(jsonString(field(fields[i])))
		}
	}
	b.WriteByte('}')
	return b.String()
}

// jsonString encodes s as a JSON string with no HTML escaping, leaving
// non-ASCII text as raw UTF-8.
func jsonString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return `""`
	}
	return strings.TrimSuffix(buf.String(), "\n")
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

// raw writes text byte-for-byte, adding nothing and terminating nothing, then
// flushes. It is how a served document reaches standard output: a document is
// prose rather than a sequence of fields, so the record grammar does not govern
// it and no terminator is imposed on it.
func (s *stream) raw(text string) {
	if s.broken || s.failed {
		return
	}
	if _, err := s.w.WriteString(text); err != nil {
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
