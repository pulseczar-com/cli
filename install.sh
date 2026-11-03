#!/bin/sh
# Installs the pulseczar program from a GitHub release. It checks the checksum
# and needs no sudo.
#
#   curl -fsSL https://pulseczar.com/install.sh | sh
#
# PULSECZAR_VERSION  install this version, such as 1.2.3 (default: the latest)
# PULSECZAR_BIN_DIR  install into this folder (default: ~/.local/bin)
set -eu

repo="pulseczar-com/cli"
bin_dir="${PULSECZAR_BIN_DIR:-$HOME/.local/bin}"

fail() {
  echo "install.sh: $1" >&2
  exit 1
}

detect_os() {
  case "$(uname -s)" in
    Linux) echo linux ;;
    Darwin) echo darwin ;;
    *) fail "unsupported system $(uname -s). On Windows, download the zip from https://github.com/$repo/releases" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) echo amd64 ;;
    arm64 | aarch64) echo arm64 ;;
    *) fail "unsupported processor $(uname -m)" ;;
  esac
}

# latest_version follows the redirect of /releases/latest to find the tag.
latest_version() {
  url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest") ||
    fail "cannot find the latest release"
  echo "${url##*/v}"
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d ' ' -f 1
  else
    shasum -a 256 "$1" | cut -d ' ' -f 1
  fi
}

main() {
  os=$(detect_os)
  arch=$(detect_arch)
  version="${PULSECZAR_VERSION:-$(latest_version)}"
  version="${version#v}"

  archive="pulseczar_${version}_${os}_${arch}.tar.gz"
  base="https://github.com/$repo/releases/download/v${version}"

  work=$(mktemp -d)
  trap 'rm -rf "$work"' EXIT

  echo "Downloading pulseczar $version for $os/$arch"
  curl -fsSL -o "$work/$archive" "$base/$archive" || fail "cannot download $base/$archive"
  curl -fsSL -o "$work/checksums.txt" "$base/pulseczar_${version}_checksums.txt" ||
    fail "cannot download the checksum file"

  expected=$(grep " $archive\$" "$work/checksums.txt" | cut -d ' ' -f 1)
  [ -n "$expected" ] || fail "no checksum for $archive"
  [ "$expected" = "$(sha256_of "$work/$archive")" ] || fail "checksum does not match for $archive"

  tar -xzf "$work/$archive" -C "$work" pulseczar
  mkdir -p "$bin_dir"
  mv "$work/pulseczar" "$bin_dir/pulseczar"

  echo "Installed $bin_dir/pulseczar"
  case ":$PATH:" in
    *":$bin_dir:"*) ;;
    *) echo "Add $bin_dir to your PATH to run it." ;;
  esac
}

main
