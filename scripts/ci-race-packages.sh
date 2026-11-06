#!/usr/bin/env bash
# Filters the package list for the pull-request `race` job. Reads import paths
# (the output of ci-test-packages.sh, expanded from ./...) on stdin and prints
# the ones to race-test.
#
# internal/core takes ~5 min under -race, so it is dropped when it is only a
# dependent of the change, and kept when a file under internal/core/ itself
# changed: a PR that edits the core must not merge without its race run.
#
# Changed files come from CI_CHANGED_FILES (one per line) when set, otherwise
# from scripts/ci-changed-files.sh.
#
# RACE_SHARD splits the kept packages across the parallel `race` runners so
# the slow ones do not run back to back: `core` is internal/core alone (~5 min),
# `cmd` is cmd/downshift alone, `rest` is every other package. Unset prints all
# of them. Every kept package lands in exactly one shard.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

module=$(awk '$1 == "module" { print $2; exit }' go.mod)
core="$module/internal/core"
cmd="$module/cmd/downshift"
shard="${RACE_SHARD:-}"
case "$shard" in
	"" | core | cmd | rest) ;;
	*)
		echo "RACE_SHARD must be core, cmd or rest, got: $shard" >&2
		exit 2
		;;
esac

if [[ -n "${CI_CHANGED_FILES+x}" ]]; then
	files="$CI_CHANGED_FILES"
else
	files=$(bash scripts/ci-changed-files.sh)
fi

keep_core=""
if printf '%s\n' "$files" | grep -q '^internal/core/'; then
	keep_core=1
fi

while IFS= read -r pkg; do
	[[ -n "$pkg" ]] || continue
	if [[ "$pkg" == "$core" && -z "$keep_core" ]]; then
		continue
	fi
	case "$shard" in
		core) [[ "$pkg" == "$core" ]] || continue ;;
		cmd) [[ "$pkg" == "$cmd" ]] || continue ;;
		rest) [[ "$pkg" != "$core" && "$pkg" != "$cmd" ]] || continue ;;
	esac
	echo "$pkg"
done
