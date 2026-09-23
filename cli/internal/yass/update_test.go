package yass

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// targz builds a release archive holding one member, as script/build does.
func targz(t *testing.T, name string, contents []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(contents))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipArchive(t *testing.T, name string, contents []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// updateHost fakes the release layout of 12-update@ReleaseLayout for one
// version, counting archive downloads so a test can assert none happened.
type updateHost struct {
	srv       *httptest.Server
	downloads int
}

// serveUpdate starts the fake host. With tamper set, checksums.txt publishes
// digests that match no archive.
func serveUpdate(t *testing.T, version string, assets map[string][]byte, tamper bool) *updateHost {
	t.Helper()
	h := &updateHost{}
	var sums strings.Builder
	for name, data := range assets {
		d := sha256.Sum256(data)
		digest := hex.EncodeToString(d[:])
		if tamper {
			digest = strings.Repeat("0", 64)
		}
		sums.WriteString(digest + "  " + name + "\n")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/tag/v"+version, http.StatusFound)
	})
	mux.HandleFunc("/download/v"+version+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sums.String()))
	})
	for name, data := range assets {
		mux.HandleFunc("/download/v"+version+"/"+name, func(w http.ResponseWriter, r *http.Request) {
			h.downloads++
			_, _ = w.Write(data)
		})
	}
	h.srv = httptest.NewServer(mux)
	t.Cleanup(h.srv.Close)
	return h
}

// pointUpdateAt overrides the release host, the built-for platform, the
// running version, and the installed binary, restoring each after the test.
func pointUpdateAt(t *testing.T, url, goos, goarch, running string, exe func() (string, error)) {
	t.Helper()
	oldURL, oldOS, oldArch := updateReleases, updateOS, updateArch
	oldVer, oldExe := programVersion, updateExecutable
	updateReleases, updateOS, updateArch = url, goos, goarch
	programVersion, updateExecutable = running, exe
	t.Cleanup(func() {
		updateReleases, updateOS, updateArch = oldURL, oldOS, oldArch
		programVersion, updateExecutable = oldVer, oldExe
	})
}

func fixedExe(path string) func() (string, error) {
	return func() (string, error) { return path, nil }
}

func TestUpdateReplacesTheBinary(t *testing.T) {
	newBin := []byte("the new release binary")
	h := serveUpdate(t, "9.9.9", map[string][]byte{
		"yass_9.9.9_linux_amd64.tar.gz": targz(t, "yass", newBin),
	}, false)
	dir := t.TempDir()
	exe := filepath.Join(dir, "yass")
	if err := os.WriteFile(exe, []byte("the old binary"), 0o750); err != nil {
		t.Fatal(err)
	}
	pointUpdateAt(t, h.srv.URL, "linux", "amd64", "1.0.0", fixedExe(exe))

	// The starting directory holds no root.yass.yaml: update needs none.
	got := run(t.TempDir(), "update")
	wantStatus(t, got, ExitOK)
	if got.stdout != "updated\t1.0.0\t9.9.9\n" {
		t.Fatalf("update record = %q", got.stdout)
	}
	content, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, newBin) {
		t.Fatalf("installed binary was not replaced: %q", content)
	}
	info, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o750 {
		t.Fatalf("mode not preserved: %v", info.Mode())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("staging residue beside the binary: %v", entries)
	}
}

func TestUpdateAlreadyCurrent(t *testing.T) {
	h := serveUpdate(t, "9.9.9", map[string][]byte{
		"yass_9.9.9_linux_amd64.tar.gz": targz(t, "yass", []byte("release")),
	}, false)
	dir := t.TempDir()
	exe := filepath.Join(dir, "yass")
	if err := os.WriteFile(exe, []byte("installed"), 0o755); err != nil {
		t.Fatal(err)
	}
	pointUpdateAt(t, h.srv.URL, "linux", "amd64", "9.9.9", fixedExe(exe))

	got := run(t.TempDir(), "update")
	wantStatus(t, got, ExitOK)
	if got.stdout != "current\t9.9.9\t9.9.9\n" {
		t.Fatalf("update record = %q", got.stdout)
	}
	if h.downloads != 0 {
		t.Fatalf("a current installation downloaded %d archive(s)", h.downloads)
	}
	content, _ := os.ReadFile(exe)
	if string(content) != "installed" {
		t.Fatalf("a current installation was rewritten: %q", content)
	}
}

func TestUpdateHomebrewManagedIsLeftAlone(t *testing.T) {
	h := serveUpdate(t, "9.9.9", map[string][]byte{
		"yass_9.9.9_darwin_arm64.tar.gz": targz(t, "yass", []byte("release")),
	}, false)
	exe := filepath.Join(t.TempDir(), "Cellar", "yass", "1.0.0", "bin", "yass")
	pointUpdateAt(t, h.srv.URL, "darwin", "arm64", "1.0.0", fixedExe(exe))

	got := run(t.TempDir(), "update")
	wantStatus(t, got, ExitOK)
	if got.stdout != "homebrew\t1.0.0\t9.9.9\n" {
		t.Fatalf("update record = %q", got.stdout)
	}
	if h.downloads != 0 {
		t.Fatalf("a package-managed installation downloaded %d archive(s)", h.downloads)
	}
}

func TestUpdateChecksumMismatchInstallsNothing(t *testing.T) {
	h := serveUpdate(t, "9.9.9", map[string][]byte{
		"yass_9.9.9_linux_amd64.tar.gz": targz(t, "yass", []byte("release")),
	}, true)
	dir := t.TempDir()
	exe := filepath.Join(dir, "yass")
	if err := os.WriteFile(exe, []byte("installed"), 0o755); err != nil {
		t.Fatal(err)
	}
	pointUpdateAt(t, h.srv.URL, "linux", "amd64", "1.0.0", fixedExe(exe))

	got := run(t.TempDir(), "update")
	wantStatus(t, got, ExitEnvironment)
	if got.stdout != "" {
		t.Fatalf("stdout on a failed update: %q", got.stdout)
	}
	if !strings.Contains(got.stderr, "yass.update.checksum") {
		t.Fatalf("stderr = %q", got.stderr)
	}
	content, _ := os.ReadFile(exe)
	if string(content) != "installed" {
		t.Fatalf("an unverified archive replaced the binary: %q", content)
	}
}

func TestUpdateUnsupportedPlatform(t *testing.T) {
	h := serveUpdate(t, "9.9.9", map[string][]byte{
		"yass_9.9.9_linux_amd64.tar.gz": targz(t, "yass", []byte("release")),
	}, false)
	pointUpdateAt(t, h.srv.URL, "linux", "mips64", "1.0.0", fixedExe("/nonexistent/yass"))

	got := run(t.TempDir(), "update")
	wantStatus(t, got, ExitEnvironment)
	if !strings.Contains(got.stderr, "yass.update.unsupported") ||
		!strings.Contains(got.stderr, "linux/mips64") {
		t.Fatalf("stderr = %q", got.stderr)
	}
	if h.downloads != 0 {
		t.Fatalf("an unsupported platform downloaded %d archive(s)", h.downloads)
	}
}

func TestUpdateUnreachableHost(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	pointUpdateAt(t, srv.URL, "linux", "amd64", "1.0.0", fixedExe("/nonexistent/yass"))

	got := run(t.TempDir(), "update")
	wantStatus(t, got, ExitEnvironment)
	if !strings.Contains(got.stderr, "yass.update.unreachable") {
		t.Fatalf("stderr = %q", got.stderr)
	}
}

func TestUpdateDeniedPreservesTheBinary(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not bind root")
	}
	h := serveUpdate(t, "9.9.9", map[string][]byte{
		"yass_9.9.9_linux_amd64.tar.gz": targz(t, "yass", []byte("release")),
	}, false)
	dir := t.TempDir()
	exe := filepath.Join(dir, "yass")
	if err := os.WriteFile(exe, []byte("installed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	pointUpdateAt(t, h.srv.URL, "linux", "amd64", "1.0.0", fixedExe(exe))

	got := run(t.TempDir(), "update")
	wantStatus(t, got, ExitEnvironment)
	if !strings.Contains(got.stderr, "yass.update.denied") ||
		!strings.Contains(got.stderr, exe) {
		t.Fatalf("stderr = %q", got.stderr)
	}
	content, _ := os.ReadFile(exe)
	if string(content) != "installed" {
		t.Fatalf("a denied update changed the binary: %q", content)
	}
}

func TestUpdateWindowsAssetIsAZip(t *testing.T) {
	newBin := []byte("the windows binary")
	h := serveUpdate(t, "9.9.9", map[string][]byte{
		"yass_9.9.9_windows_amd64.zip": zipArchive(t, "yass.exe", newBin),
	}, false)
	dir := t.TempDir()
	exe := filepath.Join(dir, "yass.exe")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	pointUpdateAt(t, h.srv.URL, "windows", "amd64", "1.0.0", fixedExe(exe))

	got := run(t.TempDir(), "update")
	wantStatus(t, got, ExitOK)
	content, _ := os.ReadFile(exe)
	if !bytes.Equal(content, newBin) {
		t.Fatalf("zip binary was not installed: %q", content)
	}
}

func TestUpdateTakesNoOperandAndNoOption(t *testing.T) {
	dir := t.TempDir()
	got := run(dir, "update", "extra")
	wantStatus(t, got, ExitUsage)
	if !strings.Contains(got.stderr, "yass.args.extra_operand") {
		t.Fatalf("stderr = %q", got.stderr)
	}
	got = run(dir, "update", "--raw")
	wantStatus(t, got, ExitUsage)
	if !strings.Contains(got.stderr, "yass.args.unknown_option") {
		t.Fatalf("stderr = %q", got.stderr)
	}
}
