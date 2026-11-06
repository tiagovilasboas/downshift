#!/usr/bin/env bash
# Splits the package list from scripts/ci-test-packages.sh into one shard of a
# deterministic, complete partition, so the `test` go test run can fan out
# across parallel jobs without losing or duplicating a package.
#
# Usage:
#   SHARD_INDEX=<0..N-1> SHARD_TOTAL=<N> bash scripts/ci-shard-packages.sh
#       Prints the packages assigned to shard SHARD_INDEX, one per line.
#   bash scripts/ci-shard-packages.sh --verify
#       Prints nothing and exits non-zero unless the union of all shards
#       equals the full package list exactly (no package dropped or doubled).
#
# The partition rule is fixed and data-driven (see .github/workflows/ci.yml):
#   - cmd/downshift is the single slowest package (~63 s in CI; the whole test
#     run's floor). It is pinned ALONE to shard 0 so it runs on its own runner.
#   - Every other package is spread across shards 1..N-1 round-robin by its
#     position in the sorted, de-duplicated list; with only one non-zero shard
#     (N=2) that is simply "all the rest".
# The rule depends only on the sorted package list, so it is deterministic and
# every package lands in exactly one shard.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# The one package that dominates the test wall; pinned alone to shard 0.
HEAVY_PKG_SUFFIX="/cmd/downshift"

# Full, concrete, sorted, de-duplicated package list the Test step would run.
# ci-test-packages.sh prints either "./..." or a list of import paths; expand
# "./..." to the concrete list so the partition is over real packages.
full_package_list() {
  local pkgs
  pkgs="$(bash "$SCRIPT_DIR/ci-test-packages.sh")"
  if [ "$pkgs" = "./..." ]; then
    go list ./...
  else
    printf '%s\n' "$pkgs"
  fi | sed '/^[[:space:]]*$/d' | sort -u
}

# Emit "<shard-index>\t<package>" for every package, applying the fixed rule.
assign_shards() {
  local total="$1"
  local heavy="" rest=() line
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    case "$line" in
      *"$HEAVY_PKG_SUFFIX") heavy="$line" ;;
      *) rest+=("$line") ;;
    esac
  done < <(full_package_list)

  # cmd/downshift (if present in this run) goes alone to shard 0.
  if [ -n "$heavy" ]; then
    printf '0\t%s\n' "$heavy"
  fi

  # Round-robin the rest across shards 1..total-1. With total=2 that is shard 1
  # for all of them. If there is only one shard (total=1), everything is shard 0.
  local other_shards=$(( total > 1 ? total - 1 : 1 ))
  local base=$(( total > 1 ? 1 : 0 ))
  local i=0 pkg
  for pkg in "${rest[@]:-}"; do
    [ -n "$pkg" ] || continue
    printf '%d\t%s\n' "$(( base + i % other_shards ))" "$pkg"
    i=$(( i + 1 ))
  done
}

if [ "${1:-}" = "--verify" ]; then
  total="${SHARD_TOTAL:-2}"
  full="$(full_package_list)"
  union="$(assign_shards "$total" | cut -f2- | sort -u)"
  if [ "$full" = "$union" ]; then
    echo "OK: union of $total shards == full package list ($(printf '%s\n' "$full" | grep -c . ) packages)"
    exit 0
  fi
  echo "MISMATCH between full list and union of shards:" >&2
  diff <(printf '%s\n' "$full") <(printf '%s\n' "$union") >&2 || true
  exit 1
fi

SHARD_INDEX="${SHARD_INDEX:?set SHARD_INDEX (0..SHARD_TOTAL-1)}"
SHARD_TOTAL="${SHARD_TOTAL:?set SHARD_TOTAL (number of shards)}"
assign_shards "$SHARD_TOTAL" | awk -F'\t' -v s="$SHARD_INDEX" '$1 == s { print $2 }'
