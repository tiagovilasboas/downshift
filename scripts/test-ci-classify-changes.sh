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
check routing-weights "$GO1" internal/routingv2/classifier/weights/default.json
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

[ "$fail" -eq 0 ] && echo "ci-classify-changes: all cases passed"
exit "$fail"
