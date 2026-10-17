package yass

// cmdRoot reports the absolute path of the project root that governs a path,
// and nothing else. It loads no file: the answer is a directory path and no
// document is read.
func (a *App) cmdRoot() (int, error) {
	startDir, err := resolveStart(a.Inv.StartDir, a.Inv.StartDir)
	if err != nil {
		return 0, err
	}
	a.StartDir = startDirectory(startDir)

	target, label := startDir, a.Inv.StartDir
	if len(a.Inv.Operands) == 1 {
		label = a.Inv.Operands[0]
		target, err = resolveStart(a.absPath(label), label)
		if err != nil {
			return 0, err
		}
	}
	root, err := discoverRoot(target, label)
	if err != nil {
		return 0, err
	}
	a.out.line(record(root))
	return ExitOK, nil
}
