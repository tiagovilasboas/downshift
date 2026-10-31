#!/usr/bin/env sh
# Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0
#
# Downshift installer
# Downloads the latest (or a specific) release binary for your OS and arch.
#
# Usage:
#   # Latest release:
#   curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh
#
#   # Specific version:
#   curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh -s v0.1.0-beta.1
#
# The binary is installed to /usr/local/bin/downshift (or ~/.local/bin or
# ~/bin if /usr/local/bin is not writable without sudo). Set
# DOWNSHIFT_INSTALL_DIR to choose another directory.

set -e

REPO="tiagovilasboas/downshift"
BINARY="downshift"

# Every download is https-only (also after a redirect) and at least TLS 1.2.
fetch() { curl --proto '=https' --tlsv1.2 "$@"; }

# ── Resolve version ───────────────────────────────────────────────────────────
# No GitHub API: anonymous api.github.com calls are rate-limited (60/h per IP),
# which broke the one-liner on shared networks and CI. Instead:
#   1. https://github.com/<repo>/releases/latest redirects to
#      /releases/tag/<tag> for the newest stable release; read the tag from
#      the final URL.
#   2. While only prereleases exist, that URL redirects to /releases instead.
#      Fall back to the releases Atom feed, which lists every release
#      (prereleases included), newest first.
# An explicit version argument skips both lookups.
GH="https://github.com/${REPO}"
latest_tag() {
  final_url=$(fetch -fsSLI -o /dev/null -w '%{url_effective}' "${GH}/releases/latest" 2>/dev/null || true)
  case "$final_url" in
    */releases/tag/?*)
      echo "${final_url##*/releases/tag/}"
      return 0
      ;;
  esac
  fetch -fsSL "${GH}/releases.atom" 2>/dev/null \
    | grep -o '/releases/tag/[^"<]*' \
    | head -n 1 \
    | sed 's#.*/releases/tag/##' || true
}
# Flags do not consume the version argument. An unknown flag is an error so a
# typo is not installed as a version tag.
VERSION=""
for arg in "$@"; do
  case "$arg" in
    -*)
      echo "error: unknown option: $arg" >&2
      echo "Usage: sh install.sh [version]" >&2
      exit 1
      ;;
    *)
      if [ -n "$VERSION" ]; then
        echo "error: unexpected argument: $arg" >&2
        exit 1
      fi
      VERSION="$arg"
      ;;
  esac
done
if [ -z "$VERSION" ]; then
  VERSION=$(latest_tag)
fi
if [ -z "$VERSION" ]; then
  echo "error: could not determine latest version (no release found, or github.com unreachable)." >&2
  echo "Pass a version explicitly:" >&2
  echo "  sh install.sh v0.1.0-beta.1" >&2
  exit 1
fi

# The version goes into URLs and a grep pattern: accept nothing but a strict
# vMAJOR.MINOR.PATCH[-prerelease] tag.
# The character test comes first: grep matches line by line, so a value with an
# embedded newline could otherwise pass on its first line.
case "$VERSION" in
  *[!0-9A-Za-z.-]*) VERSION_OK=0 ;;
  *) VERSION_OK=1 ;;
esac
if [ "$VERSION_OK" = 0 ] || ! printf '%s\n' "$VERSION" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$'; then
  echo "error: invalid version '$VERSION' (expected something like v1.2.3 or v1.2.3-beta.1)" >&2
  exit 1
fi

# ── Detect OS and arch ────────────────────────────────────────────────────────
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  linux)  OS="linux"  ;;
  darwin) OS="darwin" ;;
  *)
    echo "error: unsupported OS: $OS" >&2
    echo "Install via Go toolchain instead:" >&2
    echo "  go install github.com/${REPO}/cmd/downshift@${VERSION}" >&2
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64 | amd64)  ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *)
    echo "error: unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

# ── Build download URL ────────────────────────────────────────────────────────
# Strip leading 'v' from version for the archive name
VER="${VERSION#v}"
ARCHIVE="downshift_${VER}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

# Probe the archive first: a release without this platform's asset (for example
# one created by hand, before the build finished) must read as "this tag has no
# archive", not as a bare curl 404. No fallback to another version.
if ! fetch -fsSLI -o /dev/null "$URL" 2>/dev/null; then
  {
    echo "error: release ${VERSION} has no ${OS}/${ARCH} archive yet (${ARCHIVE} was not found)."
    echo "This is a missing release asset, not a problem with your machine. Options:"
    echo "  1. Install another tag (list: https://github.com/${REPO}/releases):"
    echo "       curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sh -s <tag>"
    echo "  2. With Go installed, build it from source (lands in ~/go/bin):"
    echo "       go install github.com/${REPO}/cmd/downshift@${VERSION}"
    echo "  3. Report it: https://github.com/${REPO}/issues"
  } >&2
  exit 1
fi

# ── Choose install dir ────────────────────────────────────────────────────────
if [ -n "${DOWNSHIFT_INSTALL_DIR:-}" ]; then
  INSTALL_DIR="$DOWNSHIFT_INSTALL_DIR"
  mkdir -p "$INSTALL_DIR"
elif [ -w "/usr/local/bin" ]; then
  INSTALL_DIR="/usr/local/bin"
elif [ -d "$HOME/.local/bin" ]; then
  INSTALL_DIR="$HOME/.local/bin"
elif [ -d "$HOME/bin" ]; then
  INSTALL_DIR="$HOME/bin"
else
  INSTALL_DIR="$HOME/.local/bin"
  mkdir -p "$INSTALL_DIR"
fi

# ── Download and install ──────────────────────────────────────────────────────
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "Downloading downshift ${VERSION} (${OS}/${ARCH})..."
fetch -fsSL "$URL" -o "$TMP/$ARCHIVE"

# ── Verify checksum ───────────────────────────────────────────────────────────
# goreleaser publishes checksums.txt (sha256) next to every archive. A hook
# binary runs on every spawn, so an unverified download is not installed
# unless the user says so with DOWNSHIFT_SKIP_CHECKSUM=1.
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
if [ "${DOWNSHIFT_SKIP_CHECKSUM:-0}" = 1 ]; then
  echo "warning: checksum verification skipped (DOWNSHIFT_SKIP_CHECKSUM=1)" >&2
else
  if ! fetch -fsSL "$CHECKSUMS_URL" -o "$TMP/checksums.txt"; then
    echo "error: could not download checksums.txt for ${VERSION}; set DOWNSHIFT_SKIP_CHECKSUM=1 to install unverified" >&2
    exit 1
  fi
  expected=$(awk -v a="$ARCHIVE" '$2 == a {print $1; exit}' "$TMP/checksums.txt")
  if [ -z "$expected" ]; then
    echo "error: ${ARCHIVE} is not listed in checksums.txt" >&2
    exit 1
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$TMP/$ARCHIVE" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then
    actual=$(shasum -a 256 "$TMP/$ARCHIVE" | awk '{print $1}')
  else
    echo "error: neither sha256sum nor shasum is available; set DOWNSHIFT_SKIP_CHECKSUM=1 to install unverified" >&2
    exit 1
  fi
  if [ "$actual" != "$expected" ]; then
    echo "error: checksum mismatch for ${ARCHIVE} (expected ${expected}, got ${actual}); not installed" >&2
    exit 1
  fi
  echo "Checksum verified (sha256)."
fi

PROVENANCE=0
# ── Verify build provenance (optional) ────────────────────────────────────────
# Releases carry a GitHub build-provenance attestation. When the gh CLI is
# installed, check it; when gh is missing, not signed in, or the release
# predates attestations, say so and carry on: the sha256 above has already
# passed. DOWNSHIFT_REQUIRE_ATTESTATION=1 turns every non-verified outcome into
# an error; DOWNSHIFT_SKIP_ATTESTATION=1 skips the check.
attestation_failed() {
  if [ "${DOWNSHIFT_REQUIRE_ATTESTATION:-0}" = 1 ]; then
    echo "error: $1; not installed (DOWNSHIFT_REQUIRE_ATTESTATION=1)" >&2
    exit 1
  fi
  echo "warning: $1; continuing with the sha256 check only" >&2
}
if [ "${DOWNSHIFT_SKIP_ATTESTATION:-0}" = 1 ]; then
  echo "warning: attestation check skipped (DOWNSHIFT_SKIP_ATTESTATION=1)" >&2
elif command -v gh >/dev/null 2>&1; then
  if gh attestation verify "$TMP/$ARCHIVE" --repo "$REPO" >/dev/null 2>&1; then
    echo "Build provenance verified (GitHub attestation)."
    PROVENANCE=1
  else
    attestation_failed "build provenance for ${ARCHIVE} could not be verified (no attestation for this release, gh not signed in, or a mismatch: run 'gh attestation verify <file> --repo ${REPO}' to see which)"
  fi
else
  attestation_failed "build provenance not checked (the gh CLI is not installed; see docs/guides/install.md)"
fi

echo "Extracting..."
tar -xzf "$TMP/$ARCHIVE" -C "$TMP"

echo "Installing to ${INSTALL_DIR}/downshift ..."
# Copy next to the destination under a temporary name, then rename: a partial
# copy is never the file a hook would execute.
STAGED="${INSTALL_DIR}/.${BINARY}.new.$$"
cp "$TMP/$BINARY" "$STAGED"
chmod +x "$STAGED"
mv -f "$STAGED" "${INSTALL_DIR}/${BINARY}"

echo ""
echo "✓ downshift ${VERSION} installed to ${INSTALL_DIR}/downshift"
if [ "$PROVENANCE" = 1 ]; then
  echo "  Verified: sha256 and GitHub build provenance (who built it)."
elif [ "${DOWNSHIFT_SKIP_CHECKSUM:-0}" = 1 ]; then
  echo "  Verified: nothing (checksum skipped)."
else
  echo "  Verified: sha256 only. That proves the download is intact, not who published it:"
  echo "  gh attestation verify <archive> --repo ${REPO}"
fi
echo ""

# Session allowlist: the hook only writes model ids listed here. Without the
# file it never rewrites anything, so say so instead of installing a silent
# no-op. The list is operator-curated (it depends on your plan), so it is not
# created automatically.
SESSION_FILE="$HOME/.downshift/session-models.json"
if [ ! -f "$SESSION_FILE" ] && [ -f "$HOME/.harness-downshift/session-models.json" ]; then
  SESSION_FILE="$HOME/.harness-downshift/session-models.json"
fi
session_existed=0
if [ -f "$SESSION_FILE" ]; then
  session_existed=1
fi
if [ "$session_existed" = 0 ]; then
  echo "Optional: list the models your session can use (Claude Code infers them from the current model; Cursor, Codex and others need the list or 'downshift models discover'):"
  echo "  mkdir -m 700 -p \"$HOME/.downshift\""
  echo "  echo '{ \"claude-code\": [\"claude-haiku-4-5\", \"claude-sonnet-5-5\", \"claude-opus-5-5\"] }' > \"$SESSION_FILE\""
  echo "  Models marked explicit_only in the catalog (Cursor claude-fable-5-1-thinking-high, Codex gpt-6-astra) are never selected by routing: list them only if you run on them deliberately."
fi

# Verify
if command -v downshift >/dev/null 2>&1; then
  downshift --help 2>/dev/null | head -3 || true
else
  echo "⚠️  '${INSTALL_DIR}' is not in your PATH."
  echo ""
  echo "Add it now (current session):"
  echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
  echo ""
  echo "Add it permanently (zsh):"
  echo "  echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ~/.zshrc && source ~/.zshrc"
  echo ""
  echo "Or move the binary to a directory already in PATH:"
  echo "  sudo mv ${INSTALL_DIR}/downshift /usr/local/bin/downshift"
fi
