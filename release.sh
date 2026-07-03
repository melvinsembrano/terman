#!/usr/bin/env bash

# Cut a release: tag the version in internal/version/version.go, build all
# platform binaries (via build-binaries.sh), and publish a GitHub release
# with them attached.
#
# Usage: ./release.sh [--dry-run] [-y|--yes]
#   --dry-run   Print what would happen; don't build binaries, tag, push,
#               or create a release. Pre-flight checks and the build gate
#               (go build/test) still run for real.
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

  --dry-run   Print what would happen; don't build binaries, tag, push,
              or create a release. Pre-flight checks and the build gate
              (go build/test) still run for real.
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

# file#label pairs — the label is just a friendly display name on the
# GitHub release page; the uploaded file itself keeps its real name.
BINARIES=(
  "terman-linux#Linux (amd64)"
  "terman-linux-arm#Linux (arm64)"
  "terman.exe#Windows (amd64)"
  "terman-mac#macOS (amd64)"
  "terman-mac-arm#macOS (arm64)"
)

# --- Resolve version/tag ---------------------------------------------

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

# --- Pre-flight checks (always run, even under --dry-run) -------------

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

# --- Build gate (always run, even under --dry-run) ---------------------

echo
echo "== Build gate =="
go build ./...
go test ./...
echo "Build and tests passed."

# --- Confirm ------------------------------------------------------------

if ! $DRY_RUN && ! $ASSUME_YES; then
  echo
  echo "About to:"
  echo "  1. Build all platform binaries via $BUILD_SCRIPT"
  echo "  2. Create and push tag $TAG"
  if [[ -n "$PREV_TAG" ]]; then
    echo "  3. Publish a GitHub release $TAG with the binaries attached and a commit list since $PREV_TAG"
  else
    echo "  3. Publish a GitHub release $TAG with the binaries attached and a commit list"
  fi
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

# --- Build all platform binaries ----------------------------------------

echo
echo "== Build binaries =="
if $DRY_RUN; then
  echo "[dry-run] would run: $BUILD_SCRIPT"
else
  "$BUILD_SCRIPT"

  MISSING=()
  for entry in "${BINARIES[@]}"; do
    file="${entry%%#*}"
    if [[ ! -f "$file" ]]; then
      MISSING+=("$file")
    fi
  done
  if [[ ${#MISSING[@]} -gt 0 ]]; then
    echo "error: expected binaries missing after build: ${MISSING[*]}" >&2
    exit 1
  fi
fi

# --- Tag ------------------------------------------------------------------

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
  if [[ -n "$PREV_TAG" ]]; then
    echo "[dry-run] would run: gh release create $TAG --verify-tag --generate-notes --notes <above> --notes-start-tag $PREV_TAG --title $TAG ${BINARIES[*]}"
  else
    echo "[dry-run] would run: gh release create $TAG --verify-tag --generate-notes --notes <above> --title $TAG ${BINARIES[*]}"
  fi
else
  # Branched (rather than building an optional-flag array) because an
  # empty array expanded with "${arr[@]}" under `set -u` is a portability
  # trap on bash 3.2 (macOS's default /bin/bash) — it errors as an
  # unbound variable instead of expanding to nothing.
  if [[ -n "$PREV_TAG" ]]; then
    gh release create "$TAG" \
      --verify-tag \
      --generate-notes \
      --notes "$NOTES_PREFIX" \
      --notes-start-tag "$PREV_TAG" \
      --title "$TAG" \
      "${BINARIES[@]}"
  else
    gh release create "$TAG" \
      --verify-tag \
      --generate-notes \
      --notes "$NOTES_PREFIX" \
      --title "$TAG" \
      "${BINARIES[@]}"
  fi
  echo
  echo "Release $TAG published: $(gh release view "$TAG" --json url -q .url)"
fi
