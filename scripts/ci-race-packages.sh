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
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

module=$(awk '$1 == "module" { print $2; exit }' go.mod)
core="$module/internal/core"

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
	echo "$pkg"
done
