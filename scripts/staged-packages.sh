#!/usr/bin/env bash
# Prints the Go packages that hold a staged .go file, one relative path per
# line (./internal/core). Files outside a Go package (testdata fixtures, a
# deleted package) are dropped. Read by the pre-commit hook (make test-staged,
# make vet-staged, make staticcheck-staged).
set -euo pipefail

module=$(go list -m)
pkgs=$(go list ./...)
git diff --cached --name-only --diff-filter=ACMR -- '*.go' |
	while IFS= read -r f; do
		d=$(dirname "$f")
		case $'\n'"$pkgs"$'\n' in *$'\n'"$module/$d"$'\n'*) echo "./$d" ;; esac
	done | sort -u
