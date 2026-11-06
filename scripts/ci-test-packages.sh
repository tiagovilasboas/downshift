#!/usr/bin/env bash
# Prints the Go packages a change needs tested, one import path per line. CI
# (test shards, race) and the git hooks (make test-changed, make vet-changed)
# all read this one script, so they cannot diverge.
#
#   ./...    on every push (main is always tested in full), and whenever a
#            changed file cannot be narrowed safely: go.mod, go.sum,
#            scripts/*, .github/*, web/*, and files at the repository root.
#   empty    the change needs no Go test (for example examples/*.json).
#   a list   otherwise, built per changed file:
#     *.md, docs/*            internal/docscheck (it reads every markdown file
#                             and checks links); docs/reference/harness-matrix.md
#                             also internal/execwrap (its inventory test).
#     mods/** (not .go)       the mods package alone: the mod's TypeScript is
#                             embedded bytes, its own tests run with
#                             `make test-mods`, not with go test.
#     examples/** (not .md)   nothing: no Go code reads examples/.
#     install.sh              internal/installer (its tests run the script).
#     anything else           the nearest Go package directory holding the file
#                             (a .go file, testdata, an embedded asset) plus every
#                             package that depends on it, directly or through its
#                             tests. No Go package above the file: ./...
#   Any .go file also selects internal/archtest, internal/docscheck and
#   internal/limits: they guard the whole tree, not one package.
#   A markdown file inside a Go package (an embedded skill or agent) counts both
#   ways: docscheck, and its package with dependents.
#
# CI_CHANGED_FILES (newline separated) replaces the git diff; used by
# scripts/test-ci-test-packages.sh.
set -euo pipefail

full() { echo "./..."; exit 0; }

if [ -n "${CI_CHANGED_FILES+x}" ]; then
	files="$CI_CHANGED_FILES"
else
	[[ "${GITHUB_EVENT_NAME:-}" == "pull_request" && -n "${GITHUB_BASE_SHA:-}" ]] || full
	head="${GITHUB_HEAD_SHA:-HEAD}"
	files=$(git diff --name-only "$GITHUB_BASE_SHA" "$head") || full
fi

module=$(go list -m) || full
pkgs=$(go list ./...) || full

is_pkg() { case $'\n'"$pkgs"$'\n' in *$'\n'"$module/$1"$'\n'*) return 0 ;; esac; return 1; }

# nearest_pkg <dir>: the closest directory at or above dir that is a Go package.
nearest_pkg() {
	local d="$1"
	while [[ "$d" != "." && "$d" != "/" && -n "$d" ]]; do
		if is_pkg "$d"; then echo "$d"; return 0; fi
		d=$(dirname "$d")
	done
	return 1
}

direct=""  # tested as is
seeds=""   # tested with every dependent
seen=""
while IFS= read -r f; do
	[[ -n "$f" ]] || continue
	seen=1
	case "$f" in
	go.mod|go.sum|scripts/*|.github/*|web/*) full ;;
	esac
	case "$f" in
	*.md|docs/*) direct="$direct $module/internal/docscheck" ;;
	esac
	case "$f" in
	docs/reference/harness-matrix.md) direct="$direct $module/internal/execwrap" ;;
	esac
	case "$f" in
	install.sh) direct="$direct $module/internal/installer"; continue ;;
	esac
	case "$f" in
	*.go)
		# Architecture, docs and limits guards read the whole tree: any Go change runs them.
		direct="$direct $module/internal/archtest $module/internal/docscheck $module/internal/limits"
		;;
	docs/*|examples/*) continue ;;
	mods/*) direct="$direct $module/mods"; continue ;;
	esac
	if [[ "$f" != */* ]]; then
		# Root markdown (README, AGENTS) is docscheck's alone; any other root
		# file (Makefile, lefthook.yml, .goreleaser.yml) is not safe to narrow.
		case "$f" in *.md) continue ;; esac
		full
	fi
	dir=$(nearest_pkg "$(dirname "$f")") || full
	seeds="$seeds $module/$dir"
done <<< "$files"

# An empty diff cannot be proven safe.
[[ -n "$seen" ]] || full

{
	for p in $direct; do echo "$p"; done
	if [[ -n "${seeds// /}" ]]; then
		go list -f '{{.ImportPath}} {{join .Deps " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... |
			while read -r pkg rest; do
				for dep in $pkg $rest; do
					case " $seeds " in *" $dep "*) echo "$pkg"; break ;; esac
				done
			done
	fi
} | sort -u
