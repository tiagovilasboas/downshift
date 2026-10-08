#!/usr/bin/env bash
set -euo pipefail

if ! command -v rtk >/dev/null 2>&1; then
  echo "RTK is optional. Install it to run this target: https://github.com/rtk-ai/rtk" >&2
  exit 127
fi

expect_rewrite() {
  local input="$1"
  local expected="$2"
  local actual
  actual="$(rtk rewrite "$input")"
  if [[ "$actual" != "$expected" ]]; then
    printf 'RTK rewrite mismatch\n  input:    %s\n  expected: %s\n  actual:   %s\n' \
      "$input" "$expected" "$actual" >&2
    exit 1
  fi
}

expect_rewrite 'git status --short' 'rtk git status --short'
expect_rewrite 'go test -race -cover ./...' 'rtk go test -race -cover ./...'

rtk --version
echo 'RTK rewrite checks passed; running Downshift tests through RTK.'
rtk go test -race -cover ./...
echo
echo 'Project-scoped RTK savings (estimated terminal-output reduction):'
rtk gain --project
