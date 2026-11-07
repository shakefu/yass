package yass

import (
	"embed"
	"strconv"
	"strings"
)

// corpusFS carries the language documents this program serves. They are
// embedded rather than read from the file tree because a reader holding this
// binary does not hold the checkout they live in, and because
// root@YassCli::SIDE-EFFECT forbids reading a file that is not a spec file.
//
// script/sync-docs is the only writer of this directory, and
// TestCorpusInSyncWithCheckout fails when a copy has drifted from its source.
//
//go:embed docs
var corpusFS embed.FS

// CorpusDoc is one carried language document, per 10-docs@Corpus.
type CorpusDoc struct {
	Name   string // the token `yass docs NAME` takes
	File   string // basename under docs/
	Source string // the checkout path script/sync-docs copies from
	When   string // the task that earns the cost of reading it
	Title  string // the document's own title
}

// Corpus is the carried set, in index order: exactly the files that are
// authoritative for the yass language and for writing specs in it, and nothing
// else.
//
// An embedded basename never ends in .yass.yaml. Every file with that suffix
// under a project root is collected as part of that project, so a copy carrying
// it would be indexed, validated, and linted as one of this program's own
// documents.
var Corpus = []CorpusDoc{
	{
		Name:   "brainstorming",
		File:   "brainstorming.md",
		Source: "context/BRAINSTORMING.md",
		When:   "eliciting a spec set with a user: elicit path, then the write path",
		Title:  "yass — Brainstorming a Spec",
	},
	{
		Name:   "authoring",
		File:   "authoring.md",
		Source: "context/AUTHORING.md",
		When:   "writing or changing a spec: write path 1 of 5",
		Title:  "yass — Authoring",
	},
	{
		Name:   "documents",
		File:   "documents.md",
		Source: "context/DOCUMENTS.md",
		When:   "writing or changing a spec: write path 2 of 5",
		Title:  "yass — Files and Documents",
	},
	{
		Name:   "obligations",
		File:   "obligations.md",
		Source: "context/OBLIGATIONS.md",
		When:   "writing or changing a spec: write path 3 of 5",
		Title:  "yass — Slots and Obligations",
	},
	{
		Name:   "references",
		File:   "references.md",
		Source: "context/REFERENCES.md",
		When:   "writing or changing a spec: write path 4 of 5",
		Title:  "yass — References",
	},
	{
		Name:   "coverage",
		File:   "coverage.md",
		Source: "context/COVERAGE.md",
		When:   "writing or changing a spec: write path 5 of 5",
		Title:  "yass — Case Coverage and Review",
	},
	{
		Name:   "implementing",
		File:   "implementing.md",
		Source: "context/IMPLEMENTING.md",
		When:   "implementing code and tests against an approved spec set: implement path",
		Title:  "yass — Implementing and Testing a Spec",
	},
	{
		Name:   "language",
		File:   "language.yaml",
		Source: "yass.yass.yaml",
		When:   "changing the yass language itself, then the write path",
		Title:  "the yass language defined in yass",
	},
}

// corpusDoc finds a carried document by exact name. A name never resolves by
// abbreviation, by prefix, or in another casing.
func corpusDoc(name string) (CorpusDoc, bool) {
	for _, d := range Corpus {
		if d.Name == name {
			return d, true
		}
	}
	return CorpusDoc{}, false
}

// text returns the document's text exactly as the corpus carries it.
func (d CorpusDoc) text() (string, error) {
	b, err := corpusFS.ReadFile("docs/" + d.File)
	if err != nil {
		return "", internal("carried document " + d.File + " is missing from the build")
	}
	return string(b), nil
}

// LineBudget is the most lines a document-form answer holds, per root@Brevity:
// a reader commonly cuts its read there.
const LineBudget = 200

// lengthLine is the line written before a carried document longer than the
// line budget, per 10-docs@Docs::RETURN, so a cut read is a known one. The
// count is computed from the carried text, so it cannot drift from it.
func lengthLine(name, text string) string {
	return "# yass docs " + name + ": " + strconv.Itoa(countLines(text)) +
		" lines total. Read every line; a read cut short drops rules.\n"
}

// countLines counts the lines a text holds: one per LF, plus a final unterminated
// line when the text does not end in one.
func countLines(text string) int {
	if text == "" {
		return 0
	}
	n := strings.Count(text, "\n")
	if !strings.HasSuffix(text, "\n") {
		n++
	}
	return n
}
