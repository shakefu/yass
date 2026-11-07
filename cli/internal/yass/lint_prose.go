package yass

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
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
	// proseRef matches a token of the ref-target grammar's other-file form.
	proseRef = regexp.MustCompile(`[\w./-]*@[A-Za-z_]`)

	// shapeClauses matches a clause break: a semicolon, or a sentence end
	// followed by a capitalized word.
	shapeClauses = regexp.MustCompile(`; |[\w)\]"'][.!] +[A-Z]`)
	// shapeAbbrev matches the abbreviations whose period ends no sentence.
	shapeAbbrev = regexp.MustCompile(`\b(e\.g|i\.e|etc|vs|cf)\.`)
	// shapeProcedure matches a connective that sequences steps.
	shapeProcedure = regexp.MustCompile(`(?i)\b(and then|which then|after that|afterwards)\b`)
	// shapeAside matches a parenthetical that opens with a reason.
	shapeAside = regexp.MustCompile(`(?i)\((so|because|since) `)
	// shapeCode matches code written into prose: an assignment, an interface
	// literal, or a struct literal.
	shapeCode = regexp.MustCompile(`:=|\binterface ?\{|\{\s*[A-Z]\w*: `)
	// shapeErrorMap matches a mapping of errors, which belongs in ERROR.
	shapeErrorMap = regexp.MustCompile(`(?i)\bmap(s|ping)? (its|the|every|each|all|any|other) (\w+ )?errors?\b`)
)

// ShapeLength is the length, in characters, beyond which obligation prose is
// reported as more than one requirement.
const ShapeLength = 400

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
				case proseRef.MatchString(prose) || proseRef.MatchString(guard):
					warn("yass.lint.prose_keyword", s.Target(), o.Line,
						"reference written into prose; state it under its relation key")
				}
				if msg := shapeProblem(o, s.Name, prose, guard); msg != "" {
					warn("yass.lint.obligation_shape", s.Target(), o.Line, msg)
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

// shapeProblem names the first way an obligation's prose or guard, with code
// spans blanked, holds more than one requirement, or returns "".
func shapeProblem(o *Obligation, slot, prose, guard string) string {
	both := prose + "\n" + guard
	switch {
	case utf8.RuneCountInString(o.Prose) > ShapeLength:
		return "obligation prose runs " + strconv.Itoa(utf8.RuneCountInString(o.Prose)) +
			" characters; state one requirement per obligation and move the mechanism to a design"
	case shapeCode.MatchString(both):
		return "code written into prose; name the symbol and move the mechanism to a design"
	case shapeProcedure.MatchString(both):
		return "procedure written into prose; state one requirement per obligation and move the steps to a design"
	case shapeClauses.MatchString(shapeAbbrev.ReplaceAllString(both, "")):
		return "several clauses in one obligation; split each requirement into its own obligation"
	case shapeAside.MatchString(both):
		return "rationale aside in prose; state the requirement alone"
	case slot != "ERROR" && shapeErrorMap.MatchString(prose):
		return "error mapping outside ERROR; state each failure as an ERROR obligation"
	}
	return ""
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
