#!/usr/bin/env bash
# Regression test for scripts/ci-classify-changes.sh (run in CI, no Go needed).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

fail=0
check() { # name, expected "go=X docs_only=Y", files...
  local name="$1" want="$2" out got
  shift 2
  out="$(CI_CHANGED_FILES="$(printf '%s\n' "$@")" GITHUB_REF=refs/heads/x bash scripts/ci-classify-changes.sh)"
  got="$(printf '%s\n' "$out" | grep -E '^(go|docs_only)=' | sort | tr '\n' ' ' | sed 's/ $//')"
  if [ "$got" != "$want" ]; then
    echo "FAIL $name: got [$got] want [$want]"
    fail=1
  fi
}

GO1="docs_only=0 go=1"
GO0="docs_only=1 go=0"

check catalog-json "$GO1" internal/catalog/catalog.json
check nbtier-train "$GO1" internal/nbtier/train.json
check corpus-json "$GO1" internal/core/testdata/corpus/a.json
check goreleaser "$GO1" .goreleaser.yml
check readme "$GO1" README.md
check docs-md "$GO1" docs/some-page.md
check changelog "$GO1" CHANGELOG.md
check go-file "$GO1" internal/x/y.go
check web "$GO1" web/app.js
check mixed "$GO1" docs/architecture.png internal/catalog/catalog.json
check empty "$GO1"
check deleted-image "$GO1" docs/does-not-exist-anywhere.png

img="$(git ls-files 'docs/*.png' | head -1)"
if [ -n "$img" ]; then
  check existing-image "$GO0" "$img"
fi

# check_os name want-os ref files...: asserts os_sensitive only.
check_os() {
  local name="$1" want="$2" ref="$3" out got
  shift 3
  out="$(CI_CHANGED_FILES="$(printf '%s\n' "$@")" GITHUB_REF="$ref" bash scripts/ci-classify-changes.sh)"
  got="$(printf '%s\n' "$out" | grep -E '^os_sensitive=' || true)"
  if [ "$got" != "os_sensitive=$want" ]; then
    echo "FAIL $name: got [$got] want [os_sensitive=$want]"
    fail=1
  fi
}

PR=refs/pull/1/merge
check_os os-paths 1 "$PR" internal/paths/paths.go
check_os os-execwrap 1 "$PR" internal/execwrap/exec.go
check_os os-contextopt 1 "$PR" internal/contextopt/x.go
check_os os-hookctx 1 "$PR" internal/hookctx/x.go
check_os os-adapters 1 "$PR" internal/adapters/claudecode/adapter.go
check_os os-cmd 1 "$PR" cmd/downshift/setup.go
check_os os-kirocrew-wrapper 1 "$PR" scripts/kirocrew-pre-tool-use.sh
check_os os-hooks-json 1 "$PR" hooks/hooks.json
check_os os-workflow 1 "$PR" .github/workflows/ci.yml
check_os os-mixed-with-docs 1 "$PR" docs/x.md internal/paths/paths.go
check_os os-empty-diff 1 "$PR"
check_os not-os-catalog 0 "$PR" internal/catalog/catalog.json
check_os not-os-core 0 "$PR" internal/core/route.go
check_os not-os-prefix-sibling 0 "$PR" internal/pathsx/x.go
check_os not-os-docs 0 "$PR" docs/some-page.md
check_os not-os-readme 0 "$PR" README.md
check_os not-os-other-script 0 "$PR" scripts/ci-test-packages.sh
check_os main-push-docs-only 1 refs/heads/main docs/some-page.md
check_os tag-push-docs-only 1 refs/tags/v1.2.3 docs/some-page.md

[ "$fail" -eq 0 ] && echo "ci-classify-changes: all cases passed"
exit "$fail"
