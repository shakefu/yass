package yass

import "strconv"

// cmdDocs serves the language documents the program carries: with no operand it
// indexes the corpus, and with a NAME it writes that document whole. It loads no
// spec file — no answer of this command comes from the file tree — so it needs
// neither a project root nor a loader.
func (a *App) cmdDocs() (int, error) {
	if len(a.Inv.Operands) == 0 {
		return a.docsIndex()
	}
	return a.docsWrite(a.Inv.Operands[0])
}

// docsIndex writes one record per carried document, in corpus order: the name
// that addresses it, the lines it holds so a reader can budget for it, the task
// that earns the cost of reading it, and its title.
func (a *App) docsIndex() (int, error) {
	for _, d := range Corpus {
		text, err := d.text()
		if err != nil {
			return 0, err
		}
		a.out.line(record(d.Name, strconv.Itoa(countLines(text)), d.When, d.Title))
	}
	return ExitOK, nil
}

// docsWrite writes one carried document byte-for-byte, adding no header, no
// footer, no provenance comment, and no trailing summary. The document is prose
// rather than a sequence of fields, so the record grammar does not govern it.
func (a *App) docsWrite(name string) (int, error) {
	d, ok := corpusDoc(name)
	if !ok {
		// The reader recovers from `yass docs`, which is the index, so the
		// diagnostic does not name the recognized documents.
		return 0, fail(ExitUnresolved, "yass.docs.unknown", "", 0,
			"no carried document named "+name)
	}
	text, err := d.text()
	if err != nil {
		return 0, err
	}
	a.out.raw(text)
	return ExitOK, nil
}
