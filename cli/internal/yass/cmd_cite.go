package yass

// cmdCite writes the citation line of each obligation its TARGETs resolve to,
// so an implementer pastes a citation instead of counting obligations by hand.
// Every TARGET resolves before any record is written.
func (a *App) cmdCite() (int, error) {
	if err := a.prepare(); err != nil {
		return 0, err
	}
	var rows []string
	for _, raw := range a.Inv.Operands {
		ct, ok := parseCitation(raw)
		if !ok {
			return 0, fail(ExitUsage, "yass.args.target_syntax", raw, 0, "reference target is malformed")
		}
		cites, err := a.resolveCitation(ct)
		if err != nil {
			return 0, err
		}
		for _, c := range cites {
			rows = append(rows, a.record(citeFields, citationMarker+" "+c.Token, c.Obl.Keyword, obligationProse(c.Obl)))
		}
	}
	for _, r := range rows {
		a.out.line(r)
	}
	return ExitOK, nil
}
