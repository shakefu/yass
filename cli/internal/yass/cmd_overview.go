package yass

import (
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// overviewWidth is the column the block's fixed text is written within, and
	// the width a wrapped description is broken to fit.
	overviewWidth = 88
	// overviewLabel is the column the project section's values start at.
	overviewLabel = 11
	// overviewDescriptionLimit is the count of Unicode scalar values the root
	// preamble description is truncated to before it is wrapped.
	overviewDescriptionLimit = 240
)

// The fixed sections of 11-overview@OrientationBlock, in block order. Nothing is
// added to them per invocation; only the project section is computed.

const overviewWhat = `yass — Yet Another Spec Syntax. A spec file carries the behaviour a project must
implement, written as obligations that can be checked against code. This program is
read-only: it serves a spec set that already exists, and never writes one.`

const overviewNotation = `A spec file is a YAML document stream. The first document is the preamble, carrying
` + "`description`" + ` and ` + "`version`" + `; every later one is a ` + "`spec:`" + ` holding obligations under five
slots, or a ` + "`design:`" + ` holding normative freeform content under ` + "`type`" + ` and ` + "`content`" + `.

  slots        INPUT  RETURN  ERROR  SIDE-EFFECT  INVARIANT
  obligation   one of MUST / MUST-NOT / SHOULD / SHOULD-NOT / MAY with the prose, an
               optional WHEN guard, and at most one each of CONFORMS / USES / SEE
  relations    CONFORMS = must match the target, USES = binding design block,
               SEE = pointer that binds nothing
  target       path@Name::SLOT — ` + "`path@`" + ` and ` + "`::SLOT`" + ` are each optional; a bare name is
               the same file, and a path with no leading dot is from the project root`

const overviewReading = `read a spec set
  yass list                    every document of the project, one per line
  yass query TARGET            one document, with its references resolved
  yass find PATTERN            search obligation prose and design content
  yass refs TARGET             what a document points at, and what points at it
  yass validate                check the files against the language definition
  yass lint                    report graph hygiene`

const overviewWriting = `write or change a spec
  yass docs                    index the language documents carried in this program
  yass docs NAME               write one of those documents whole`

const overviewSpending = `Read a carried document when you are writing or changing a spec, not when you are
implementing one. Implementing against a spec set that already exists needs neither: the
notation above is all a spec file takes to read, and the behaviour to build is in the
spec files themselves.`

const overviewResidue = `  yass --help                  every global option, subcommand operand, and exit code`

// overviewProject is what the block's project section reports.
type overviewProject struct {
	found       bool
	root        string
	description string
	files       int
	documents   int
}

// cmdOverview writes the orientation block: what this program is, what the
// project under the starting directory is, the notation a spec file uses, the
// commands that read a spec set, and where the language corpus is. It is the
// answer to `yass overview` and equally to `yass` carrying no subcommand.
func (a *App) cmdOverview() (int, error) {
	start, err := resolveStart(a.Inv.StartDir, a.Inv.StartDir)
	if err != nil {
		return 0, err
	}
	a.StartDir = start

	p, err := a.surveyProject()
	if err != nil {
		return 0, err
	}
	for _, line := range overviewBlock(p) {
		a.out.line(line)
	}
	return ExitOK, nil
}

// surveyProject discovers the project root governing the starting directory and
// counts what it holds. The absence of a root is a fact the block reports rather
// than a failure to serve the request, so it never selects the no-root status.
func (a *App) surveyProject() (overviewProject, error) {
	var p overviewProject

	root, err := discoverRoot(a.StartDir, a.Inv.StartDir)
	if err != nil {
		if e, ok := err.(*cliError); ok && e.Status == ExitNoRoot {
			return p, nil
		}
		return p, err
	}
	a.Root = root
	a.Loader = newLoader(root)

	files, err := a.projectFileSet()
	if err != nil {
		return p, err
	}
	p.found = true
	p.root = root
	p.files = len(files)

	rootFile := filepath.Join(root, RootBasename)
	for _, abs := range files {
		f, err := a.Loader.Load(abs)
		if err != nil {
			return p, err
		}
		if !f.Parsed {
			// Counted among the files; its documents are unknown, and naming
			// the failure is validate's answer rather than this command's.
			continue
		}
		for _, d := range f.Docs {
			if d.Kind == KindSpec || d.Kind == KindDesign {
				p.documents++
			}
		}
		if abs == rootFile {
			if pre := a.preambleOf(f); pre != nil {
				p.description = pre.Description
			}
		}
	}
	return p, nil
}

// overviewBlock assembles the sections, separated by exactly one empty line,
// with no empty line before the first section or after the last.
func overviewBlock(p overviewProject) []string {
	sections := [][]string{
		splitLines(overviewWhat),
		overviewProjectSection(p),
		splitLines(overviewNotation),
		splitLines(overviewReading),
		splitLines(overviewWriting),
		splitLines(overviewSpending),
		splitLines(overviewResidue),
	}
	var out []string
	for i, sec := range sections {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, sec...)
	}
	return out
}

func overviewProjectSection(p overviewProject) []string {
	if !p.found {
		return []string{labelled("project", "none — no "+RootBasename+" at or above the starting directory")}
	}
	out := []string{labelled("project", p.root)}
	out = append(out, wrapDescription(p.description)...)
	return append(out, labelled("holds", plural(p.files, "spec file")+", "+plural(p.documents, "document")))
}

// labelled writes one project-section line: the field name, then padding to the
// label column, then the value.
func labelled(name, value string) string {
	return name + strings.Repeat(" ", overviewLabel-len(name)) + value
}

func plural(n int, noun string) string {
	if n == 1 {
		return strconv.Itoa(n) + " " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// wrapDescription collapses the root preamble description to one line under the
// field rules, truncates it, and breaks it at spaces onto lines that fit the
// block width, indenting each line to the label column so the text aligns under
// itself. A description that is empty after collapsing contributes no line.
func wrapDescription(desc string) []string {
	collapsed := field(desc)
	if collapsed == "-" {
		return nil
	}
	text := truncateScalars(collapsed, overviewDescriptionLimit)
	pad := strings.Repeat(" ", overviewLabel)
	limit := overviewWidth - overviewLabel

	var out []string
	line := ""
	for _, word := range strings.Split(text, " ") {
		switch {
		case line == "":
			line = word
		case len([]rune(line))+1+len([]rune(word)) <= limit:
			line += " " + word
		default:
			out = append(out, pad+line)
			line = word
		}
	}
	if line != "" {
		out = append(out, pad+line)
	}
	return out
}
