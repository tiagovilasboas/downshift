#!/usr/bin/env bash
# Tests scripts/ci-test-packages.sh: which Go packages each kind of change selects.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
m=$(go list -m)
fail=0

# expect <label> <changed files> <expected output, newline separated>
expect() {
	local got
	got=$(CI_CHANGED_FILES="$2" bash scripts/ci-test-packages.sh)
	if [ "$got" = "$3" ]; then
		echo "PASS: $1"
	else
		printf 'FAIL: %s\n  want: %s\n  got:  %s\n' "$1" "$(echo $3)" "$(echo $got)" >&2
		fail=1
	fi
}
# has <label> <changed files> <package that must be selected>
has() {
	if CI_CHANGED_FILES="$2" bash scripts/ci-test-packages.sh | grep -qx "$3"; then
		echo "PASS: $1"
	else
		echo "FAIL: $1: $3 not selected" >&2
		fail=1
	fi
}

expect "docs-only change runs docscheck" "docs/guides/install.md" "$m/internal/docscheck"
expect "root README runs docscheck" "README.md" "$m/internal/docscheck"
expect "docs image runs docscheck" "docs/img/x.png" "$m/internal/docscheck"
expect "harness matrix also runs execwrap" "docs/reference/harness-matrix.md" "$m/internal/docscheck
$m/internal/execwrap"
expect "mods TypeScript runs only the mods package" "mods/downshift-agents/hooks/register.tsx" "$m/mods"
expect "examples JSON runs no Go test" "examples/catalog.sample.json" ""
expect "go.mod runs everything" "go.mod" "./..."
expect "scripts run everything" "scripts/smoke-test.sh" "./..."
expect "workflows run everything" ".github/workflows/ci.yml" "./..."
expect "root non-markdown runs everything" "Makefile" "./..."
expect "install.sh runs the installer tests" "install.sh" "$m/internal/installer"
expect "empty diff runs everything" "" "./..."
has "Go change selects its package" "internal/testenv/timing.go" "$m/internal/testenv"
has "Go change always runs archtest" "internal/testenv/timing.go" "$m/internal/archtest"
has "Go change always runs docscheck" "internal/testenv/timing.go" "$m/internal/docscheck"
has "Go change always runs limits" "internal/testenv/timing.go" "$m/internal/limits"
has "Go change selects dependents" "internal/core/route.go" "$m/cmd/downshift"
has "embedded agent markdown selects its package" "agents/coder.md" "$m/agents"
has "embedded agent markdown selects dependents" "agents/coder.md" "$m/internal/routing/agenttier"
has "embedded agent markdown also runs docscheck" "agents/coder.md" "$m/internal/docscheck"
has "skill file selects the skills package" "skills/ponytail/SKILL.md" "$m/skills"
has "testdata selects its package" "internal/outcome/testdata/stub.go" "$m/internal/outcome"

exit "$fail"
