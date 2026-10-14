# Installation and updates

Released binaries run without Go. Choose Homebrew, the release installer, or a
manual download. Releases support macOS and Linux on arm64 and amd64, and
Windows on amd64. Check the installed version with:

```sh
yass --version
```

## Homebrew

```sh
brew tap shakefu/yass https://github.com/shakefu/yass
brew install yass
```

Update a Homebrew installation through its package manager:

```sh
brew update
brew upgrade yass
```

## Release installer (macOS and Linux)

The installer needs `curl`, `tar`, and either `sha256sum` or `shasum`, plus
standard shell utilities. It detects the host OS and architecture, downloads
the newest stable release, and checks the archive against that release's
`checksums.txt` before installing the binary.

```sh
curl -fsSL https://raw.githubusercontent.com/shakefu/yass/main/install.sh | sh
```

By default, it uses `/usr/local/bin` when that directory is writable. Otherwise
it creates and uses `$HOME/.local/bin`. It reports the installed path and prints
a PATH setup suggestion if that directory is absent from PATH.

Pass options after `sh -s --` when piping the script:

```sh
curl -fsSL https://raw.githubusercontent.com/shakefu/yass/main/install.sh |
  sh -s -- --version v0.4.3 --bin-dir "$HOME/bin"
```

Both options accept a separate value or an equals sign:

| Option | Accepted spellings | Effect |
| --- | --- | --- |
| Version | `--version TAG`, `--version=TAG` | Select a release; `v0.4.3` and `0.4.3` select the same tag. |
| Directory | `--bin-dir DIR`, `--bin-dir=DIR` | Install `yass` into the specified directory, creating it if needed. |
| Help | `--help`, `-h` | Display the script header and exit. |

For example, the equivalent equals form is:

```sh
curl -fsSL https://raw.githubusercontent.com/shakefu/yass/main/install.sh |
  sh -s -- --version=0.4.3 --bin-dir="$HOME/bin"
```

The installer also accepts environment variables:

| Variable | Default | Effect |
| --- | --- | --- |
| `YASS_VERSION` | Newest stable release | Select a release, as with `--version`. |
| `YASS_BIN_DIR` | Writable `/usr/local/bin`, otherwise `$HOME/.local/bin` | Select the installation directory. |
| `YASS_REPO_URL` | `https://github.com/shakefu/yass` | Override the repository base URL for a mirror or tests. The mirror must provide the same release URL and asset layout. |

An option overrides its matching environment variable. If an option appears
more than once, the last value wins. An empty version selects the newest stable
release, and an empty directory selects the default directory. There is no
command-line counterpart to `YASS_REPO_URL`.

Apply environment variables to the `sh` process that runs the installer:

```sh
curl -fsSL https://raw.githubusercontent.com/shakefu/yass/main/install.sh |
  YASS_VERSION=0.4.3 YASS_BIN_DIR="$HOME/bin" sh
```

To add the default user directory to your current shell's PATH:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Put that line in your shell's startup configuration, such as `~/.zshrc` or
`~/.bashrc`, to keep it for future sessions. Substitute your chosen directory
when using `--bin-dir` or `YASS_BIN_DIR`.

## Manual release downloads

Open the [releases page](https://github.com/shakefu/yass/releases) and choose a
release. Download its `checksums.txt` and the archive matching your platform:

| Platform | Archive name |
| --- | --- |
| macOS, Apple Silicon | `yass_<version>_darwin_arm64.tar.gz` |
| macOS, Intel | `yass_<version>_darwin_amd64.tar.gz` |
| Linux, ARM64 | `yass_<version>_linux_arm64.tar.gz` |
| Linux, x86-64 | `yass_<version>_linux_amd64.tar.gz` |
| Windows, x86-64 | `yass_<version>_windows_amd64.zip` |

The filename's version omits the tag's leading `v`. Compare the archive's SHA256
digest with its entry in `checksums.txt` before extracting it. On macOS:

```sh
shasum -a 256 yass_0.4.3_darwin_arm64.tar.gz
```

On Linux, use `sha256sum` with your archive name. After confirming the matching
checksum, extract the binary and place it on PATH. For example, on Apple Silicon:

```sh
tar -xzf yass_0.4.3_darwin_arm64.tar.gz yass
mkdir -p "$HOME/.local/bin"
install -m 0755 yass "$HOME/.local/bin/yass"
export PATH="$HOME/.local/bin:$PATH"
```

On Windows, the shell installer is unsupported. Download the zip and check its
digest in PowerShell:

```powershell
Get-FileHash .\yass_0.4.3_windows_amd64.zip -Algorithm SHA256
```

Compare the reported hash with the corresponding `checksums.txt` entry, then
extract `yass.exe` into a directory such as `%USERPROFILE%\bin`. Add that directory
to your user PATH through Windows environment variable settings and open a new
terminal before running `yass --version`.

Use the version you selected in place of `0.4.3` in these examples.

## Updating an installed binary

For an installer-managed or manually placed binary:

```sh
yass update
```

This command takes no options or operands and works outside a Yass project. It
resolves the newest stable release, verifies the matching archive's checksum,
and replaces the running binary at its resolved path. It needs network access
and permission to write the binary's directory. A failed update preserves the
working installation; diagnostics go to standard error with exit status 5.

On success it exits with status 0 and writes one tab-separated record:

```text
updated	0.4.1	0.4.3
```

The fields are action, installed version, and newest stable version. The action
is `current` when already current, `updated` after replacement, or `homebrew`
when an older Homebrew-managed binary is detected. For `homebrew`, run
`brew update` and `brew upgrade yass`.

`yass update` selects the newest stable release, even when you previously
installed a pinned version. To keep or change a pin, rerun the installer with
`--version` or manually install the selected release instead.

## Building from source

A source build needs Go 1.24 or newer and network access on the first build to
fetch module dependencies. From a repository checkout:

```sh
cd cli
go build -o yass .
./yass --version
```

To build directly into Go's binary directory instead:

```sh
go build -o "$(go env GOPATH)/bin/yass" .
export PATH="$(go env GOPATH)/bin:$PATH"
```

Persist the PATH setting in your shell configuration if necessary. For more
build and test instructions, see
[`cli/HOWTORUN.txt`](https://github.com/shakefu/yass/blob/main/cli/HOWTORUN.txt).
