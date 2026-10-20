#!/usr/bin/env bash
# Prints the Go packages the Test step runs on a pull request: the packages
# whose files changed, plus every package that depends on one of them, directly
# or through its tests. Prints ./... whenever the change is not confined to Go
# package directories, and on every push (main is always tested in full).
set -euo pipefail

full() { echo "./..."; exit 0; }

[[ "${GITHUB_EVENT_NAME:-}" == "pull_request" && -n "${GITHUB_BASE_SHA:-}" ]] || full

head="${GITHUB_HEAD_SHA:-HEAD}"
files=$(git diff --name-only "$GITHUB_BASE_SHA" "$head") || full
module=$(go list -m) || full

changed=""
while IFS= read -r f; do
	[[ -n "$f" ]] || continue
	case "$f" in
	docs/*|*.md) continue ;;
	go.mod|go.sum|scripts/*|.github/*|web/*) full ;;
	esac
	dir=$(dirname "$f")
	# A file outside a Go package (root files, unknown trees) is not safe to narrow.
	go list "./$dir" >/dev/null 2>&1 || full
	changed="$changed $module/$dir"
done <<< "$files"

[[ -n "${changed// /}" ]] || full

go list -f '{{.ImportPath}} {{join .Deps " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... |
	while read -r pkg rest; do
		hit=""
		for dep in $pkg $rest; do
			case " $changed " in *" $dep "*) hit=1; break ;; esac
		done
		if [[ -n "$hit" ]]; then echo "$pkg"; fi
	done | sort -u
