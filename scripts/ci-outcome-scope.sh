#!/usr/bin/env bash
# Exit 0 if offline outcome eval (--verify) should run for this change set.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mapfile -t files < <("$REPO_ROOT/scripts/ci-changed-files.sh")

pattern='^(benchmark/outcomes/|internal/outcome/|cmd/downshift/outcome|internal/core/|internal/catalog/|internal/routingv2/|benchmark/tasks\.json|benchmark/holdout\.json|\.github/workflows/ci(-full)?\.yml|scripts/ci-)'

for f in "${files[@]}"; do
  [ -z "$f" ] && continue
  if echo "$f" | grep -Eq "$pattern"; then
    exit 0
  fi
done

exit 1
