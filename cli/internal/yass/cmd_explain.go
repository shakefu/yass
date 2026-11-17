package yass

import "strings"

// cmdExplain serves built-in lint guidance without discovering a project or
// reading a spec file. The no-operand index is the recovery path for unknown IDs.
func (a *App) cmdExplain() (int, error) {
	if len(a.Inv.Operands) == 0 {
		for _, rule := range lintExplanations {
			a.out.line(record(rule.ID))
		}
		return ExitOK, nil
	}
	id := a.Inv.Operands[0]
	for _, rule := range lintExplanations {
		if rule.ID == id {
			a.out.raw(rule.text())
			return ExitOK, nil
		}
	}
	return 0, fail(ExitUnresolved, "yass.explain.unknown", "", 0,
		"no lint rule named "+id)
}

// text renders the requested explanation, keeping example YAML as a stream
// rather than collapsing it into record fields.
func (r lintExplanation) text() string {
	return r.ID + "\n\nDetects: " + r.Detects + "\nWhy: " + r.Why +
		"\nResolve: " + r.Remedy + "\n\nBad:\n```yaml\n" + strings.TrimRight(r.Bad, "\n") +
		"\n```\n\nGood:\n```yaml\n" + strings.TrimRight(r.Good, "\n") + "\n```\n"
}
