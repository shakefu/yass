#!/bin/sh
#
# install.sh — install a released yass binary without a Go toolchain.
#
#   curl -fsSL https://raw.githubusercontent.com/shakefu/yass/main/install.sh | sh
#
# Options (after `sh -s --` when piped):
#   --version TAG   install that release instead of the latest (v0.2.0 or 0.2.0)
#   --bin-dir DIR   install into DIR instead of the default
#   --help          write this header and exit
#
# Environment (each is overridden by the matching option):
#   YASS_VERSION    same as --version
#   YASS_BIN_DIR    same as --bin-dir
#   YASS_REPO_URL   repository base URL, for mirrors and tests
#                   (default https://github.com/shakefu/yass)
#
# The default install directory is /usr/local/bin when it is writable, and
# $HOME/.local/bin (created if absent) otherwise.
#
# The matching release archive for the host OS and architecture is downloaded,
# verified against the release's published sha256 checksums, and installed.
# Nothing is installed unless verification succeeds.

set -eu

say() { printf '%s\n' "$*"; }
fail() {
	printf 'install.sh: %s\n' "$*" >&2
	exit 1
}

repo_url="${YASS_REPO_URL:-https://github.com/shakefu/yass}"
version="${YASS_VERSION:-}"
bin_dir="${YASS_BIN_DIR:-}"

while [ $# -gt 0 ]; do
	case "$1" in
	--version)
		[ $# -ge 2 ] || fail "missing value for option --version"
		version="$2"
		shift 2
		;;
	--version=*)
		version="${1#--version=}"
		shift
		;;
	--bin-dir)
		[ $# -ge 2 ] || fail "missing value for option --bin-dir"
		bin_dir="$2"
		shift 2
		;;
	--bin-dir=*)
		bin_dir="${1#--bin-dir=}"
		shift
		;;
	--help | -h)
		sed -n '2,24p' "$0" 2>/dev/null || say "see the header of install.sh"
		exit 0
		;;
	*)
		fail "unknown argument $1 (recognized: --version TAG, --bin-dir DIR, --help)"
		;;
	esac
done

command -v curl >/dev/null 2>&1 || fail "curl is required to download a release"
command -v tar >/dev/null 2>&1 || fail "tar is required to unpack a release"

# sha256 FILE — write the hex digest of FILE.
if command -v sha256sum >/dev/null 2>&1; then
	sha256() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
	sha256() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
	fail "sha256sum or shasum is required to verify a release"
fi

# --- host platform ------------------------------------------------------------

os=$(uname -s)
case "$os" in
Linux) os=linux ;;
Darwin) os=darwin ;;
MINGW* | MSYS* | CYGWIN* | Windows_NT)
	fail "Windows is not supported by this installer; download the windows_amd64 .zip from ${repo_url}/releases"
	;;
*)
	fail "unsupported operating system $os (supported: Linux, macOS)"
	;;
esac

arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*)
	fail "unsupported architecture $arch (supported: x86_64/amd64, aarch64/arm64)"
	;;
esac

# --- release selection ----------------------------------------------------------

if [ -z "$version" ]; then
	# The /releases/latest URL redirects to /releases/tag/<tag>; the tag names
	# the latest stable release without touching the rate-limited API.
	effective=$(curl -fsSL -o /dev/null -w '%{url_effective}' "${repo_url}/releases/latest") ||
		fail "cannot resolve the latest release from ${repo_url}/releases/latest"
	version="${effective##*/}"
	case "$version" in
	v[0-9]*) ;;
	*) fail "cannot parse a release tag from $effective" ;;
	esac
fi
case "$version" in
v*) tag="$version" ;;
*) tag="v$version" ;;
esac
ver="${tag#v}"

asset="yass_${ver}_${os}_${arch}.tar.gz"
base="${repo_url}/releases/download/${tag}"

# --- download and verify --------------------------------------------------------

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

say "downloading ${base}/${asset}"
curl -fsSL -o "${tmp}/${asset}" "${base}/${asset}" ||
	fail "cannot download ${base}/${asset} (is ${tag} a published release?)"
curl -fsSL -o "${tmp}/checksums.txt" "${base}/checksums.txt" ||
	fail "cannot download ${base}/checksums.txt"

want=$(awk -v a="$asset" '$2 == a { print $1 }' "${tmp}/checksums.txt")
[ -n "$want" ] || fail "checksums.txt of ${tag} does not list ${asset}"
got=$(sha256 "${tmp}/${asset}")
[ "$got" = "$want" ] ||
	fail "checksum mismatch for ${asset}: want ${want}, got ${got}; nothing was installed"

tar -xzf "${tmp}/${asset}" -C "$tmp" yass ||
	fail "cannot unpack ${asset}"

# --- install --------------------------------------------------------------------

if [ -z "$bin_dir" ]; then
	if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
		bin_dir=/usr/local/bin
	else
		bin_dir="${HOME}/.local/bin"
	fi
fi
mkdir -p "$bin_dir" || fail "cannot create ${bin_dir}"
[ -d "$bin_dir" ] && [ -w "$bin_dir" ] ||
	fail "cannot write to ${bin_dir}; re-run with --bin-dir DIR or as a user who can"

if command -v install >/dev/null 2>&1; then
	install -m 0755 "${tmp}/yass" "${bin_dir}/yass" || fail "cannot install into ${bin_dir}"
else
	cp "${tmp}/yass" "${bin_dir}/yass" && chmod 0755 "${bin_dir}/yass" ||
		fail "cannot install into ${bin_dir}"
fi

say "installed ${bin_dir}/yass ($("${bin_dir}/yass" --version))"

case ":${PATH}:" in
*":${bin_dir}:"*) ;;
*)
	say "note: ${bin_dir} is not on PATH; add it, e.g.:"
	say "  export PATH=\"${bin_dir}:\$PATH\""
	;;
esac
say "verify with: yass --version"
