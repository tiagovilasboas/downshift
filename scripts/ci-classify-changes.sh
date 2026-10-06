#!/usr/bin/env bash
# Prints shell-assignable flags: docs_only, go, python (0 or 1).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mapfile -t files < <("$REPO_ROOT/scripts/ci-changed-files.sh")

docs_only=1
go=0
python=0

if [ "${#files[@]}" -eq 0 ]; then
  docs_only=0
  go=1
  python=1
else
  for f in "${files[@]}"; do
    [ -z "$f" ] && continue
    case "$f" in
      README.md)
        go=1
        docs_only=0
        ;;
      *.md | docs/*)
        ;;
      *)
        docs_only=0
        ;;
    esac
    case "$f" in
      *.go | go.mod | go.sum | benchmark/* | scripts/* | .github/workflows/* | catalog.sample.json | install.sh | README.md)
        go=1
        ;;
    esac
    case "$f" in
      orchestration/* | tests/ds04_observer_contract/*)
        python=1
        ;;
    esac
  done
fi

# Tags always run Go smoke path.
if [[ "${GITHUB_REF:-}" == refs/tags/* ]]; then
  docs_only=0
  go=1
fi

echo "docs_only=$docs_only"
echo "go=$go"
echo "python=$python"
