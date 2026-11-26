//go:build !windows

package yass

import (
	"syscall"
	"testing"
)

// makeFIFO creates a named pipe at path: a file that is neither a directory
// nor a regular file.
func makeFIFO(t *testing.T, path string) {
	t.Helper()
	if err := syscall.Mkfifo(path, 0o644); err != nil {
		t.Skipf("a named pipe cannot be created here: %v", err)
	}
}
