#!/usr/bin/env bash
# Exit 0 if README outcome numbers should be checked in CI.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mapfile -t files < <("$REPO_ROOT/scripts/ci-changed-files.sh")

for f in "${files[@]}"; do
  [ -z "$f" ] && continue
  case "$f" in
    README.md | benchmark/outcomes/* | internal/outcome/* | cmd/downshift/outcome*)
      exit 0
      ;;
  esac
done

exit 1
