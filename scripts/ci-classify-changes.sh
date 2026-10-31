#!/usr/bin/env bash
# Prints shell-assignable flags: docs_only, go, python, os_sensitive (0 or 1).
# Go is the default; see needs_go. os_sensitive gates the cross-OS test job.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# CI_CHANGED_FILES (newline separated) overrides the git lookup; used by
# scripts/test-ci-classify-changes.sh.
if [ -n "${CI_CHANGED_FILES+x}" ]; then
  changed="$CI_CHANGED_FILES"
else
  changed="$("$REPO_ROOT/scripts/ci-changed-files.sh")"
fi

# Allowlist: the only paths that provably need no Go tests are images under
# docs/ that still exist (a deleted image can break a link that
# internal/docscheck verifies). Every other path, including markdown (read by
# docscheck), JSON embedded in the binary and .goreleaser.yml, runs Go.
needs_go() {
  case "$1" in
    docs/*.png | docs/*.jpg | docs/*.jpeg | docs/*.gif | docs/*.webp | docs/*.svg)
      [ -e "$REPO_ROOT/$1" ] && return 1
      ;;
  esac
  return 0
}

# Paths whose behaviour differs by OS (paths, HOME, setup writer, exec wrapper,
# hooks wiring) or whose change must be proven on Windows and macOS.
needs_os() {
  case "$1" in
    internal/paths/* | internal/execwrap/* | internal/contextopt/* | internal/hookctx/* | internal/adapters/* | cmd/downshift/* | scripts/kirocrew-* | hooks/hooks.json | .github/workflows/*)
      return 0
      ;;
  esac
  return 1
}

go=0
python=0
os_sensitive=0
seen=0

while IFS= read -r f; do
  [ -z "$f" ] && continue
  seen=1
  if needs_go "$f"; then
    go=1
  fi
  if needs_os "$f"; then
    os_sensitive=1
  fi
  case "$f" in
    tests/ds04_observer_contract/*)
      python=1
      ;;
  esac
done <<<"$changed"

# An empty diff cannot be proven safe.
if [ "$seen" -eq 0 ]; then
  go=1
  python=1
  os_sensitive=1
fi

# Tags always run Go smoke path.
if [[ "${GITHUB_REF:-}" == refs/tags/* ]]; then
  go=1
fi

# main is always checked in full on every OS, whatever the diff touched.
if [[ "${GITHUB_REF:-}" == refs/heads/main ]] || [[ "${GITHUB_REF:-}" == refs/tags/* ]]; then
  os_sensitive=1
fi

# docs_only means "nothing needing Go or Node tests changed".
if [ "$go" -eq 1 ]; then docs_only=0; else docs_only=1; fi

echo "docs_only=$docs_only"
echo "go=$go"
echo "python=$python"
echo "os_sensitive=$os_sensitive"
