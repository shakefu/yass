package yass

import (
	"regexp"
	"strconv"
	"strings"
)

// The open-question and prose-keyword advisories of 09-lint@Lint::RETURN match
// fixed character patterns. They weigh nothing they match: a keyword or marker
// a spec mentions rather than uses is written inside a backtick code span,
// which every pattern skips.

var (
	// openMarker matches a placeholder or open-question marker on any line.
	openMarker = regexp.MustCompile(`\b(TBD|TODO|FIXME|XXX)\b|(?i:open question)|\?\?`)
	// openQuestionMark matches a `?` that ends the value or precedes whitespace.
	openQuestionMark = regexp.MustCompile(`\?(\s|$)`)
	// leadingCondition matches prose that opens with a condition.
	leadingCondition = regexp.MustCompile(`^(?i:when|if|unless|whenever)\b`)
	// trailingCondition matches a condition clause set off inside prose.
	trailingCondition = regexp.MustCompile(`[,;]\s+(when|if|unless|whenever)\b`)
	// proseNormativity matches a normativity or guard keyword inside prose.
	proseNormativity = regexp.MustCompile(`\b(MUST|SHOULD|MAY|WHEN)\b`)
	// proseRelation matches a relation keyword followed by a ref target.
	proseRelation = regexp.MustCompile(`\b(CONFORMS|USES|SEE)\b:?\s+\S*@`)
)

// outsideCode blanks every backtick code span of each line of s: the text
// between a backtick and the next backtick on the same line. An unpaired
// backtick opens no span.
func outsideCode(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		var b strings.Builder
		for {
			open := strings.IndexByte(l, '`')
			if open < 0 {
				break
			}
			end := strings.IndexByte(l[open+1:], '`')
			if end < 0 {
				break
			}
			b.WriteString(l[:open])
			b.WriteByte(' ')
			l = l[open+1+end+1:]
		}
		b.WriteString(l)
		lines[i] = b.String()
	}
	return strings.Join(lines, "\n")
}

// proseAdvisories returns the open-question and prose-keyword advisories of one
// parsed file, at most one of a given code at a given line.
func proseAdvisories(f *File) []Diag {
	var out []Diag
	seen := map[string]bool{}
	warn := func(code, loc string, line int, msg string) {
		key := code + "\x00" + strconv.Itoa(line)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, Diag{Severity: "warn", Code: code, Location: loc, Line: line, Message: msg})
	}

	// Obligations and the preamble description come first, so an advisory on
	// an obligation's line names its slot rather than its document.
	for _, d := range f.Docs {
		if d.Kind == KindPreamble && d.HasDescription && openQuestionMark.MatchString(outsideCode(d.Description)) {
			line := d.Line
			if vn := d.value("description"); vn != nil {
				line = vn.Line
			}
			warn("yass.lint.open_question", f.Ref, line, "description asks a question; decide it before the spec is handed off")
		}
		if d.Kind != KindSpec || !d.Addressable() {
			continue
		}
		for _, s := range d.Slots {
			for _, o := range s.Obligations {
				prose, guard := "", ""
				if o.HasProse {
					prose = outsideCode(o.Prose)
				}
				if o.HasGuard && o.GuardString {
					guard = outsideCode(o.Guard)
				}
				if openQuestionMark.MatchString(prose) || openQuestionMark.MatchString(guard) {
					warn("yass.lint.open_question", s.Target(), o.Line,
						"obligation asks a question; decide it before the spec is handed off")
				}
				switch {
				case leadingCondition.MatchString(strings.TrimSpace(prose)) || trailingCondition.MatchString(prose):
					warn("yass.lint.prose_keyword", s.Target(), o.Line,
						"condition written into prose; state it as a WHEN guard")
				case proseNormativity.MatchString(prose) || proseNormativity.MatchString(guard):
					warn("yass.lint.prose_keyword", s.Target(), o.Line,
						"keyword written into prose; split it into its own obligation or guard")
				case proseRelation.MatchString(prose) || proseRelation.MatchString(guard):
					warn("yass.lint.prose_keyword", s.Target(), o.Line,
						"reference written into prose; state it under its relation key")
				}
			}
		}
	}

	// Markers on any line: a key, a value, or a comment.
	for i, l := range f.Lines {
		if openMarker.MatchString(outsideCode(l)) {
			warn("yass.lint.open_question", f.locationOf(i+1), i+1,
				"open question or placeholder; resolve it with the user before the spec is handed off")
		}
	}
	return out
}

// locationOf names the slot holding a line, or else the addressable document
// holding it, or else the file. A slot holds the lines from its key to the next
// slot or document.
func (f *File) locationOf(line int) string {
	loc := f.Ref
	for _, d := range f.Docs {
		if d.Line > line {
			break
		}
		loc = f.Ref
		if !d.Addressable() {
			continue
		}
		loc = d.Target()
		if d.Kind != KindSpec {
			continue
		}
		for _, s := range d.Slots {
			if s.Line > line {
				break
			}
			loc = s.Target()
		}
	}
	return loc
}
