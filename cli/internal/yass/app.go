package yass

import (
	"io"
	"path/filepath"
)

// synopsis is the option table Invocation parses against and the exact text
// --help writes: a synopsis line, one line per subcommand, one line per option,
// and the exit-code table.
const synopsis = `usage: yass [-C DIR] [--version] [--help] SUBCOMMAND [ARGS...]

  root [PATH]                    report the project root governing PATH
  query [--raw] TARGET...        emit each addressed document, references resolved
  list [--filter GLOB] [PATH...] index the documents of a tree
  find [--slot SLOT] PATTERN     search obligation prose and design content
  refs [--in] [--out] TARGET     report the reference edges of one document
  validate [PATH...]             check files against the yass language definition
  lint [PATH...]                 report graph hygiene of the project

  -C DIR                         start path resolution and root discovery at DIR
  --version, -V                  write the version and exit
  --help, -h                     write this text and exit

  --raw                          emit the document as authored, resolving nothing
  --filter GLOB                  emit only records whose REF matches GLOB
  --slot SLOT                    restrict matches to obligations of that slot
  --in                           report incoming edges only
  --out                          report outgoing edges only

  0 ok   1 findings   2 usage   3 unresolved   4 no-root   5 environment`

// App carries the one project root computed per invocation and the streams
// every command writes through.
type App struct {
	Inv      *Invocation
	StartDir string // absolute starting directory
	Root     string // absolute project root
	Loader   *Loader
	out      *stream
	err      *stream
}

// Main runs one invocation and returns the single exit status it selects.
func Main(args []string, stdout, stderr io.Writer) int {
	out := newStream(stdout)
	errs := newStream(stderr)

	inv, err := ParseArgs(args)
	if err != nil {
		return report(errs, out, err)
	}
	if inv.Help {
		for _, line := range splitLines(synopsis) {
			out.line(line)
		}
		return finish(out, errs, ExitOK)
	}
	if inv.Version {
		out.line(programName + " " + programVersion + " " + languageVersion)
		return finish(out, errs, ExitOK)
	}

	a := &App{Inv: inv, out: out, err: errs}
	status := a.dispatch()
	return finish(out, errs, status)
}

// dispatch routes the recognized subcommand token to the command that owns it.
// Parsing guarantees the token is a member of the recognized set, so the
// residual arm can only be reached by an internal fault.
func (a *App) dispatch() int {
	switch a.Inv.Subcommand {
	case "root":
		return a.run(a.cmdRoot)
	case "query":
		return a.run(a.cmdQuery)
	case "list":
		return a.run(a.cmdList)
	case "find":
		return a.run(a.cmdFind)
	case "refs":
		return a.run(a.cmdRefs)
	case "validate":
		return a.run(a.cmdValidate)
	case "lint":
		return a.run(a.cmdLint)
	}
	return report(a.err, a.out, internal("subcommand outside the recognized set"))
}

func (a *App) run(cmd func() (int, error)) int {
	status, err := cmd()
	if err != nil {
		return report(a.err, a.out, err)
	}
	return status
}

// prepare computes the starting directory and the one project root this
// invocation uses, and hands the same value to every spec that needs it.
func (a *App) prepare() error {
	start, err := resolveStart(a.Inv.StartDir, a.Inv.StartDir)
	if err != nil {
		return err
	}
	a.StartDir = start
	root, err := discoverRoot(start, a.Inv.StartDir)
	if err != nil {
		return err
	}
	a.Root = root
	a.Loader = newLoader(root)
	return nil
}

// rel renders an absolute path project-root-relative with `/` separators.
func (a *App) rel(abs string) string { return relTo(a.Root, abs) }

// ref renders an absolute spec-file path as the ref target addressing the file.
func (a *App) ref(abs string) string { return refOf(a.rel(abs)) }

func report(errs, out *stream, err error) int {
	e := asCLIError(err)
	errs.line(e.Diag.Record())
	return finish(out, errs, e.Status)
}

// finish selects the final status, honouring a stream failure and the closed
// reading end of standard output.
func finish(out, errs *stream, status int) int {
	out.flush()
	errs.flush()
	if out.broken {
		// The reading end of the pipe is closed: stop writing and exit 0.
		return ExitOK
	}
	if out.failed || errs.failed {
		return ExitEnvironment
	}
	return status
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// absPath resolves an operand path against the starting directory.
func (a *App) absPath(p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(a.StartDir, p)
}
