package yass

// cliError couples a diagnostic with the exit status it selects. Every failure
// path in this program carries one, so a command selects its status once.
type cliError struct {
	Diag   Diag
	Status int
}

func (e *cliError) Error() string { return e.Diag.Message }

func fail(status int, code, location string, line int, message string) error {
	return &cliError{
		Diag:   Diag{Severity: "error", Code: code, Location: location, Line: line, Message: message},
		Status: status,
	}
}

func internal(detail string) error {
	return fail(ExitEnvironment, "yass.internal", "", 0, detail)
}

func unreadable(location string) error {
	return fail(ExitEnvironment, "yass.io.unreadable", location, 0, "path cannot be read")
}

func missingPath(location string) error {
	return fail(ExitUnresolved, "yass.io.missing", location, 0, "path does not exist")
}

func notASpecFile(location string) error {
	return fail(ExitUsage, "yass.io.not_a_spec_file", location, 0, "path is not a directory or an addressable file")
}

func noRoot(location string) error {
	return fail(ExitNoRoot, "yass.root.not_found", location, 0, "no "+RootBasename+" at or above path")
}

// asCLIError narrows an error to its diagnostic, falling back to the internal
// residual for anything a guarded obligation did not match.
func asCLIError(err error) *cliError {
	if e, ok := err.(*cliError); ok {
		return e
	}
	return internal(err.Error()).(*cliError)
}
