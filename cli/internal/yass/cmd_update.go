package yass

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// The release host and the platform this program was built for, per
// 12-update@ReleaseLayout. Vars rather than consts so the tests can point the
// command at a fake release host and a chosen platform.
var (
	updateReleases   = "https://github.com/shakefu/yass/releases"
	updateOS         = runtime.GOOS
	updateArch       = runtime.GOARCH
	updateExecutable = installedBinary
	updateClient     = &http.Client{Timeout: 5 * time.Minute}
)

// installedBinary is the file the running program was executed from, with
// symbolic links resolved, per 12-update@Update::INPUT.
func installedBinary() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// cmdUpdate replaces the installed binary with the newest published stable
// release. It needs no project root and loads nothing from the tree: the
// answer concerns the program itself.
func (a *App) cmdUpdate() (int, error) {
	latest, err := latestVersion()
	if err != nil {
		return 0, err
	}
	if latest == programVersion {
		a.out.line(record("current", programVersion, latest))
		return ExitOK, nil
	}

	exe, err := updateExecutable()
	if err != nil {
		return 0, internal("installed binary cannot be located: " + err.Error())
	}
	if brewManaged(exe) {
		a.out.line(record("homebrew", programVersion, latest))
		return ExitOK, nil
	}

	bin, err := fetchVerifiedBinary(latest)
	if err != nil {
		return 0, err
	}
	if err := replaceBinary(exe, bin); err != nil {
		return 0, fail(ExitEnvironment, "yass.update.denied", "", 0,
			"cannot replace "+exe+": "+err.Error())
	}
	a.out.line(record("updated", programVersion, latest))
	return ExitOK, nil
}

// latestVersion resolves the newest stable release without touching the
// rate-limited API: `releases/latest` redirects to `releases/tag/<tag>`, and
// the tag is `v` followed by the version.
func latestVersion() (string, error) {
	url := updateReleases + "/latest"
	client := *updateClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := client.Get(url)
	if err != nil {
		return "", unreachableURL(url)
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	if resp.StatusCode < 300 || resp.StatusCode >= 400 || loc == "" {
		return "", unreachableURL(url)
	}
	tag := loc[strings.LastIndexByte(loc, '/')+1:]
	if len(tag) < 2 || !strings.HasPrefix(tag, "v") {
		return "", unreachableURL(url)
	}
	return tag[1:], nil
}

// fetchVerifiedBinary downloads the archive matching the built-for platform
// and yields its binary, only ever after the archive's digest has matched the
// release's published checksum.
func fetchVerifiedBinary(ver string) ([]byte, error) {
	base := updateReleases + "/download/v" + ver + "/"
	sums, err := fetch(base + "checksums.txt")
	if err != nil {
		return nil, err
	}
	asset := assetName(ver)
	want, listed := checksumFor(string(sums), asset)
	if !listed {
		return nil, fail(ExitEnvironment, "yass.update.unsupported", "", 0,
			"no published archive for platform "+updateOS+"/"+updateArch)
	}
	archive, err := fetch(base + asset)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(archive)
	if hex.EncodeToString(digest[:]) != want {
		return nil, fail(ExitEnvironment, "yass.update.checksum", "", 0,
			"archive digest does not match the published checksum for "+asset)
	}
	return extractBinary(archive, asset)
}

func fetch(url string) ([]byte, error) {
	resp, err := updateClient.Get(url)
	if err != nil {
		return nil, unreachableURL(url)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, unreachableURL(url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, unreachableURL(url)
	}
	return body, nil
}

func unreachableURL(url string) error {
	return fail(ExitEnvironment, "yass.update.unreachable", "", 0, "cannot fetch "+url)
}

// assetName is the archive the release publishes for the platform this
// program was built for, per 12-update@ReleaseLayout.
func assetName(ver string) string {
	ext := ".tar.gz"
	if updateOS == "windows" {
		ext = ".zip"
	}
	return "yass_" + ver + "_" + updateOS + "_" + updateArch + ext
}

// checksumFor reads the `<sha256-hex>  <archive-name>` lines script/build
// writes and yields the digest published for one archive.
func checksumFor(sums, asset string) (string, bool) {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			return fields[0], true
		}
	}
	return "", false
}

// brewManaged reports whether the resolved binary path lives in a Homebrew
// cellar, per 12-update@HomebrewDetection.
func brewManaged(exe string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(exe), "/") {
		if seg == "Cellar" || seg == "linuxbrew" {
			return true
		}
	}
	return false
}

// extractBinary yields the single binary a release archive holds.
func extractBinary(archive []byte, asset string) ([]byte, error) {
	if strings.HasSuffix(asset, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, internal("archive cannot be read: " + err.Error())
		}
		for _, f := range zr.File {
			if f.Name == "yass.exe" {
				r, err := f.Open()
				if err != nil {
					return nil, internal("archive cannot be read: " + err.Error())
				}
				defer r.Close()
				return io.ReadAll(r)
			}
		}
		return nil, internal("archive holds no binary")
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, internal("archive cannot be read: " + err.Error())
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, internal("archive holds no binary")
		}
		if err != nil {
			return nil, internal("archive cannot be read: " + err.Error())
		}
		if filepath.Base(hdr.Name) == "yass" {
			return io.ReadAll(tr)
		}
	}
}

// replaceBinary stages the new binary in a temporary file beside the installed
// one and swaps it in with a rename, so any failure leaves the installed
// binary as it was.
func replaceBinary(exe string, bin []byte) error {
	mode := os.FileMode(0o755)
	if info, err := os.Stat(exe); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(exe), ".yass-update-*")
	if err != nil {
		return err
	}
	staged := tmp.Name()
	defer os.Remove(staged) // gone already when the swap succeeded
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return swap(staged, exe)
}

// swap renames the staged binary onto the installed one. Windows cannot rename
// over a running binary, so there the old one is moved aside first and moved
// back if the swap fails.
func swap(staged, exe string) error {
	if runtime.GOOS != "windows" {
		return os.Rename(staged, exe)
	}
	old := exe + ".old"
	os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	if err := os.Rename(staged, exe); err != nil {
		os.Rename(old, exe) // preserve the working installation
		return err
	}
	os.Remove(old) // best effort; the running binary may hold it open
	return nil
}
