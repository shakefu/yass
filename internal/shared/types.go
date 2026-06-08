package shared

import "fmt"

// PathError carries structured error information for path-related errors.
// It implements the error interface and carries enough context for callers
// to format ErrorLines.
type PathError struct {
	Code    string
	Message string
	Path    string
}

func (e *PathError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}
