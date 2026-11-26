//go:build windows

package yass

import "testing"

// makeFIFO creates a named pipe at path. Windows has none in its file tree,
// so a test that needs one is skipped there.
func makeFIFO(t *testing.T, path string) {
	t.Helper()
	t.Skip("no named pipe in the file tree on windows")
}
