#!/usr/bin/env bash

# Cut a release: tag the version in internal/version/version.go, build all
# platform archives (via build-binaries.sh), publish a GitHub release with
# them attached, and update the Homebrew tap formula automatically.
#
# Usage: ./release.sh [--dry-run] [-y|--yes]
#   --dry-run   Print what would happen; don't build archives, tag, push,
#               create a release, or touch the tap. Pre-flight checks and
#               the build gate (go build/test) still run for real.
#   -y, --yes   Skip the confirmation prompt (for scripted use).

set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

DRY_RUN=false
ASSUME_YES=false

for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    -y | --yes) ASSUME_YES=true ;;
    -h | --help)
      cat <<'EOF'
Usage: ./release.sh [--dry-run] [-y|--yes]

  --dry-run   Print what would happen; don't build archives, tag, push,
              create a release, or touch the tap. Pre-flight checks and
              the build gate (go build/test) still run for real.
  -y, --yes   Skip the confirmation prompt (for scripted use).
EOF
      exit 0
      ;;
    *)
      echo "error: unknown argument: $arg" >&2
      exit 1
      ;;
  esac
done

VERSION_FILE="internal/version/version.go"
BUILD_SCRIPT="./build-binaries.sh"
TAP_REPO="melvinsembrano/homebrew-terman"
TAP_FORMULA="Formula/terman.rb"

# --- Resolve version/tag -----------------------------------------------------

if [[ ! -f "$VERSION_FILE" ]]; then
  echo "error: $VERSION_FILE not found — run this from the repo root." >&2
  exit 1
fi
if [[ ! -x "$BUILD_SCRIPT" ]]; then
  echo "error: $BUILD_SCRIPT not found or not executable." >&2
  exit 1
fi

VERSION=$(sed -nE 's/.*Version = "([^"]+)".*/\1/p' "$VERSION_FILE")
if [[ -z "$VERSION" ]]; then
  echo "error: could not extract Version from $VERSION_FILE" >&2
  exit 1
fi
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "error: Version \"$VERSION\" (from $VERSION_FILE) doesn't look like a semantic version (X.Y.Z)" >&2
  exit 1
fi
TAG="v${VERSION}"

# Archive filenames produced by build-binaries.sh.
ARCHIVES=(
  "terman-${VERSION}-linux-amd64.tar.gz"
  "terman-${VERSION}-linux-arm64.tar.gz"
  "terman-${VERSION}-darwin-amd64.tar.gz"
  "terman-${VERSION}-darwin-arm64.tar.gz"
  "terman-${VERSION}-windows-amd64.zip"
  "checksums.txt"
)

# Capture the previous tag now, before anything below creates $TAG —
# otherwise "git describe" could resolve to the tag we're about to
# create instead of the real previous one. Empty on a first-ever release.
PREV_TAG=$(git describe --tags --abbrev=0 2>/dev/null || true)

# A plain commit list to prepend ahead of GitHub's own --generate-notes
# section (which only lists merged PRs — this repo has none, so on its
# own it produces just a bare compare-link with no actual changelog).
if [[ -n "$PREV_TAG" ]]; then
  COMMITS=$(git log "${PREV_TAG}..HEAD" --pretty='format:- %s (%h)')
  NOTES_PREFIX=$'## Commits since '"$PREV_TAG"$'\n\n'"$COMMITS"$'\n'
else
  COMMITS=$(git log --pretty='format:- %s (%h)')
  NOTES_PREFIX=$'## Commits\n\n'"$COMMITS"$'\n'
fi

# --- Pre-flight checks (always run, even under --dry-run) -------------------

echo "== Pre-flight checks =="

if [[ -n "$(git status --porcelain)" ]]; then
  echo "error: working tree is not clean. Commit or stash changes before releasing." >&2
  exit 1
fi

if ! git rev-parse --abbrev-ref --symbolic-full-name '@{u}' >/dev/null 2>&1; then
  echo "error: current branch has no upstream tracking branch." >&2
  exit 1
fi

LOCAL_HEAD=$(git rev-parse HEAD)
UPSTREAM_HEAD=$(git rev-parse '@{u}')
if [[ "$LOCAL_HEAD" != "$UPSTREAM_HEAD" ]]; then
  echo "error: HEAD ($LOCAL_HEAD) does not match upstream ($UPSTREAM_HEAD)." >&2
  echo "       Push or pull to sync before releasing." >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
  echo "error: tag $TAG already exists locally." >&2
  exit 1
fi

if [[ -n "$(git ls-remote --tags origin "$TAG")" ]]; then
  echo "error: tag $TAG already exists on origin." >&2
  exit 1
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "error: gh CLI is not installed. See https://cli.github.com/." >&2
  exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
  echo "error: gh CLI is not authenticated. Run 'gh auth login'." >&2
  exit 1
fi

echo "Pre-flight checks passed."
echo "  Version: $VERSION"
echo "  Tag:     $TAG"

# --- Build gate (always run, even under --dry-run) --------------------------

echo
echo "== Build gate =="
go build ./...
go test ./...
echo "Build and tests passed."

# --- Confirm ----------------------------------------------------------------

if ! $DRY_RUN && ! $ASSUME_YES; then
  echo
  echo "About to:"
  echo "  1. Build all platform archives via $BUILD_SCRIPT"
  echo "  2. Create and push tag $TAG"
  if [[ -n "$PREV_TAG" ]]; then
    echo "  3. Publish a GitHub release $TAG with archives attached and a commit list since $PREV_TAG"
  else
    echo "  3. Publish a GitHub release $TAG with archives attached and a commit list"
  fi
  echo "  4. Update Homebrew tap ($TAP_REPO)"
  echo
  read -r -p "Continue? [y/N] " REPLY
  case "$REPLY" in
    [yY] | [yY][eE][sS]) ;;
    *)
      echo "Aborted."
      exit 1
      ;;
  esac
fi

# --- Build all platform archives --------------------------------------------

echo
echo "== Build archives =="
if $DRY_RUN; then
  echo "[dry-run] would run: $BUILD_SCRIPT $VERSION"
else
  "$BUILD_SCRIPT" "$VERSION"

  MISSING=()
  for f in "${ARCHIVES[@]}"; do
    if [[ ! -f "$f" ]]; then
      MISSING+=("$f")
    fi
  done
  if [[ ${#MISSING[@]} -gt 0 ]]; then
    echo "error: expected files missing after build: ${MISSING[*]}" >&2
    exit 1
  fi
fi

# --- Tag --------------------------------------------------------------------

echo
echo "== Tag =="
if $DRY_RUN; then
  echo "[dry-run] would run: git tag -a $TAG -m $TAG"
  echo "[dry-run] would run: git push origin $TAG"
else
  git tag -a "$TAG" -m "$TAG"
  git push origin "$TAG"
fi

# --- GitHub release ---------------------------------------------------------

echo
echo "== GitHub release =="
echo "Release notes will prepend this commit list ahead of GitHub's own auto-generated section:"
echo "---"
echo "$NOTES_PREFIX"
echo "---"

if $DRY_RUN; then
  echo "[dry-run] would run: gh release create $TAG --verify-tag --generate-notes --notes <above> --title $TAG ${ARCHIVES[*]}"
else
  if [[ -n "$PREV_TAG" ]]; then
    gh release create "$TAG" \
      --verify-tag \
      --generate-notes \
      --notes "$NOTES_PREFIX" \
      --notes-start-tag "$PREV_TAG" \
      --title "$TAG" \
      "${ARCHIVES[@]}"
  else
    gh release create "$TAG" \
      --verify-tag \
      --generate-notes \
      --notes "$NOTES_PREFIX" \
      --title "$TAG" \
      "${ARCHIVES[@]}"
  fi
  echo
  echo "Release $TAG published: $(gh release view "$TAG" --json url -q .url)"
fi

# --- Update Homebrew tap ----------------------------------------------------

echo
echo "== Update Homebrew tap =="

# Read SHA256 values from checksums.txt (format: "<sha256>  <filename>")
read_sha() {
  local filename="$1"
  grep "$filename" checksums.txt | awk '{print $1}'
}

SHA_DARWIN_AMD64=$(read_sha "terman-${VERSION}-darwin-amd64.tar.gz")
SHA_DARWIN_ARM64=$(read_sha "terman-${VERSION}-darwin-arm64.tar.gz")
SHA_LINUX_AMD64=$(read_sha  "terman-${VERSION}-linux-amd64.tar.gz")
SHA_LINUX_ARM64=$(read_sha  "terman-${VERSION}-linux-arm64.tar.gz")

if $DRY_RUN; then
  echo "[dry-run] would ensure $TAP_REPO exists (create if not)"
  echo "[dry-run] would clone $TAP_REPO, patch $TAP_FORMULA with:"
  echo "  version:          $VERSION"
  echo "  darwin/amd64 sha: $SHA_DARWIN_AMD64"
  echo "  darwin/arm64 sha: $SHA_DARWIN_ARM64"
  echo "  linux/amd64  sha: $SHA_LINUX_AMD64"
  echo "  linux/arm64  sha: $SHA_LINUX_ARM64"
  echo "[dry-run] would commit 'chore: update terman to $TAG' and push"
else
  TAP_TMPDIR=$(mktemp -d)
  trap 'rm -rf "$TAP_TMPDIR"' EXIT

  # Create the tap repo if it doesn't exist yet.
  if ! gh repo view "$TAP_REPO" >/dev/null 2>&1; then
    echo "Creating tap repository $TAP_REPO..."
    gh repo create "$TAP_REPO" \
      --public \
      --description "Homebrew tap for terman — Terminal API Client"

    # Bootstrap it with a README so the clone has an initial commit.
    BOOTSTRAP_TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TAP_TMPDIR" "$BOOTSTRAP_TMPDIR"' EXIT
    cd "$BOOTSTRAP_TMPDIR"
    git init -q
    git remote add origin "https://github.com/${TAP_REPO}.git"
    mkdir -p Formula
    cp "$OLDPWD/Formula/terman.rb" Formula/terman.rb
    cat > README.md <<'TAPREADME'
# homebrew-terman

Homebrew tap for [terman](https://github.com/melvinsembrano/terman) — Terminal API Client.

## Install

```sh
brew tap melvinsembrano/terman
brew install terman
```
TAPREADME
    git add .
    git commit -q -m "chore: initial tap setup"
    git branch -M main
    git push -q -u origin main
    cd "$OLDPWD"
    rm -rf "$BOOTSTRAP_TMPDIR"
  fi

  echo "Cloning $TAP_REPO..."
  gh repo clone "$TAP_REPO" "$TAP_TMPDIR/tap"

  FORMULA_PATH="$TAP_TMPDIR/tap/$TAP_FORMULA"

  if [[ ! -f "$FORMULA_PATH" ]]; then
    echo "error: $TAP_FORMULA not found in cloned tap repo." >&2
    exit 1
  fi

  # Patch version and sha256 values in-place.
  # The formula uses distinct placeholder-or-previous sha256 lines per platform,
  # identified by the surrounding on_* context. We use perl for reliable
  # multi-line-aware sed-style replacements.
  perl -i -pe "
    s|^(  version \").*(\")|\${1}${VERSION}\${2}|;
    s|PLACEHOLDER_DARWIN_AMD64|${SHA_DARWIN_AMD64}|;
    s|PLACEHOLDER_DARWIN_ARM64|${SHA_DARWIN_ARM64}|;
    s|PLACEHOLDER_LINUX_AMD64|${SHA_LINUX_AMD64}|;
    s|PLACEHOLDER_LINUX_ARM64|${SHA_LINUX_ARM64}|;
  " "$FORMULA_PATH"

  # Replace previous real SHA values too (for subsequent releases).
  # Strategy: rewrite every sha256 line by matching the URL on the preceding line.
  python3 - "$FORMULA_PATH" "$VERSION" \
    "$SHA_DARWIN_AMD64" "$SHA_DARWIN_ARM64" \
    "$SHA_LINUX_AMD64"  "$SHA_LINUX_ARM64"  <<'PYSCRIPT'
import sys, re

path, ver, sha_da, sha_dr, sha_la, sha_lr = sys.argv[1:]

platform_map = {
    f"terman-{ver}-darwin-amd64.tar.gz": sha_da,
    f"terman-{ver}-darwin-arm64.tar.gz": sha_dr,
    f"terman-{ver}-linux-amd64.tar.gz":  sha_la,
    f"terman-{ver}-linux-arm64.tar.gz":  sha_lr,
}

with open(path) as f:
    lines = f.readlines()

out = []
i = 0
while i < len(lines):
    line = lines[i]
    # If this line is a url line, look ahead for the sha256 line and update it.
    m = re.match(r'(\s+url\s+"[^"]*terman-[^"]+/([^"]+)")', line)
    if m:
        fname = m.group(2)
        out.append(line)
        i += 1
        if i < len(lines) and re.match(r'\s+sha256\s+"', lines[i]):
            if fname in platform_map:
                indent = re.match(r'(\s+)', lines[i]).group(1)
                out.append(f'{indent}sha256 "{platform_map[fname]}"\n')
            else:
                out.append(lines[i])
            i += 1
        continue
    out.append(line)
    i += 1

with open(path, 'w') as f:
    f.writelines(out)
PYSCRIPT

  # Also update the version line.
  perl -i -pe "s|^(  version \").*(\")|\${1}${VERSION}\${2}|" "$FORMULA_PATH"

  cd "$TAP_TMPDIR/tap"
  git add "$TAP_FORMULA"
  if git diff --cached --quiet; then
    echo "Homebrew formula already up to date — nothing to commit."
  else
    git commit -m "chore: update terman to $TAG"
    git push
    echo "Homebrew tap updated: https://github.com/${TAP_REPO}/blob/main/${TAP_FORMULA}"
  fi
  cd "$OLDPWD"
fi

echo
echo "Done. Users can install with:"
echo "  brew tap melvinsembrano/terman"
echo "  brew install terman"
