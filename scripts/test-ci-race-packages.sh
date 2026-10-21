#!/usr/bin/env bash
# Regression test for scripts/ci-race-packages.sh (run in CI, no Go needed).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

module=$(awk '$1 == "module" { print $2; exit }' go.mod)
pkgs="$module/internal/catalog
$module/internal/core
$module/cmd/downshift"

fail=0
check() { # name, expected package count, core expected (0/1), files...
	local name="$1" want_n="$2" want_core="$3" out n has_core=0
	shift 3
	out="$(printf '%s\n' "$pkgs" | CI_CHANGED_FILES="$(printf '%s\n' "$@")" bash scripts/ci-race-packages.sh)"
	n=$(printf '%s\n' "$out" | grep -c . || true)
	if printf '%s\n' "$out" | grep -qx "$module/internal/core"; then has_core=1; fi
	if [[ "$n" != "$want_n" || "$has_core" != "$want_core" ]]; then
		echo "FAIL $name: got $n package(s) core=$has_core, want $want_n core=$want_core"
		fail=1
	fi
}

check core-file-changed 3 1 internal/core/classifier.go
check core-test-changed 3 1 internal/core/semantic_adversarial_test.go
check core-testdata-changed 3 1 internal/core/testdata/corpus/a.json
check core-only-dependent 2 0 internal/catalog/catalog.go
check core-prefix-lookalike 2 0 internal/corefoo/x.go
check mixed 3 1 internal/catalog/catalog.go internal/core/route.go

if [[ "$fail" != 0 ]]; then exit 1; fi
echo "ci-race-packages: all cases pass"
