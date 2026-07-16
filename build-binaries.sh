#!/usr/bin/env bash

# Build versioned release archives for all supported platforms.
#
# Usage: ./build-binaries.sh [VERSION]
#
#   VERSION   Semver string without the leading "v", e.g. "0.7.0".
#             If omitted, the version is read from internal/version/version.go.
#
# Outputs (in the repo root):
#   terman-<version>-linux-amd64.tar.gz
#   terman-<version>-linux-arm64.tar.gz
#   terman-<version>-darwin-amd64.tar.gz
#   terman-<version>-darwin-arm64.tar.gz
#   terman-<version>-windows-amd64.zip
#   checksums.txt   (SHA-256 for each archive)
#
# Also removes stale bare-binary build artifacts from earlier releases
# (terman-linux, terman-linux-arm, terman-mac, terman-mac-arm, terman.exe).

set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# --- Resolve version --------------------------------------------------------

VERSION_FILE="internal/version/version.go"

if [[ $# -ge 1 && -n "$1" ]]; then
  VERSION="$1"
else
  if [[ ! -f "$VERSION_FILE" ]]; then
    echo "error: $VERSION_FILE not found — run this from the repo root." >&2
    exit 1
  fi
  VERSION=$(sed -nE 's/.*Version = "([^"]+)".*/\1/p' "$VERSION_FILE")
  if [[ -z "$VERSION" ]]; then
    echo "error: could not extract Version from $VERSION_FILE" >&2
    exit 1
  fi
fi

echo "Building terman v${VERSION} for all platforms..."

# --- SHA-256 helper (macOS uses shasum, Linux uses sha256sum) ---------------

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1"
  else
    shasum -a 256 "$1"
  fi
}

# --- Build function ---------------------------------------------------------

TMPDIR_BUILD=$(mktemp -d)
trap 'rm -rf "$TMPDIR_BUILD"' EXIT

ARCHIVES=()

build_archive() {
  local goos="$1"
  local goarch="$2"
  local bin_name="${3:-terman}"   # terman.exe for windows
  local archive_name="$4"         # full archive filename

  echo "  Building ${goos}/${goarch}..."
  local bin_path="$TMPDIR_BUILD/$bin_name"
  GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags="-s -w" -o "$bin_path" .

  if [[ "$archive_name" == *.zip ]]; then
    # zip for Windows — entry at root of archive
    (cd "$TMPDIR_BUILD" && zip -q "$OLDPWD/$archive_name" "$bin_name")
  else
    # tar.gz for Unix — entry at root of archive
    tar -czf "$archive_name" -C "$TMPDIR_BUILD" "$bin_name"
  fi

  rm -f "$bin_path"
  ARCHIVES+=("$archive_name")
  echo "    → $archive_name"
}

# --- Build all targets ------------------------------------------------------

build_archive linux  amd64 terman "terman-${VERSION}-linux-amd64.tar.gz"
build_archive linux  arm64 terman "terman-${VERSION}-linux-arm64.tar.gz"
build_archive darwin amd64 terman "terman-${VERSION}-darwin-amd64.tar.gz"
build_archive darwin arm64 terman "terman-${VERSION}-darwin-arm64.tar.gz"
build_archive windows amd64 terman.exe "terman-${VERSION}-windows-amd64.zip"

# --- Checksums --------------------------------------------------------------

echo "Writing checksums.txt..."
rm -f checksums.txt
for archive in "${ARCHIVES[@]}"; do
  sha256_file "$archive" >> checksums.txt
done
echo "  → checksums.txt"

# --- Clean up stale bare-binary artifacts -----------------------------------

STALE=(terman-linux terman-linux-arm terman-mac terman-mac-arm terman.exe)
for f in "${STALE[@]}"; do
  if [[ -f "$f" ]]; then
    echo "  Removing stale artifact: $f"
    rm -f "$f"
  fi
done

echo "All archives built successfully."
