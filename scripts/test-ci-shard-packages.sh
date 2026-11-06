#!/usr/bin/env bash
# Tests scripts/ci-shard-packages.sh: the partition must be deterministic,
# complete (union == full list), and disjoint (no package in two shards), and
# cmd/downshift must be pinned alone to shard 0.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

# Run against the full package list (push context forces ./...).
export GITHUB_EVENT_NAME=push

fail=0
check() { # check <label> <condition-exit-code>
  if [ "$2" -eq 0 ]; then
    echo "PASS: $1"
  else
    echo "FAIL: $1" >&2
    fail=1
  fi
}

full="$(go list ./... | sort -u)"
full_count="$(printf '%s\n' "$full" | grep -c .)"

for total in 2 3; do
  union=""
  prev=""
  overlap=0
  for i in $(seq 0 $(( total - 1 ))); do
    shard="$(SHARD_INDEX=$i SHARD_TOTAL=$total bash scripts/ci-shard-packages.sh | sort)"
    # Disjointness: this shard must not intersect any earlier shard.
    if [ -n "$prev" ] && [ -n "$shard" ]; then
      if [ -n "$(comm -12 <(printf '%s\n' "$prev") <(printf '%s\n' "$shard"))" ]; then
        overlap=1
      fi
    fi
    prev="$(printf '%s\n%s\n' "$prev" "$shard" | sed '/^$/d' | sort -u)"
    union="$(printf '%s\n%s\n' "$union" "$shard" | sed '/^$/d' | sort -u)"
  done

  check "N=$total: union == full package list" "$([ "$union" = "$full" ] && echo 0 || echo 1)"
  check "N=$total: shards are disjoint" "$overlap"
  check "N=$total: --verify agrees" "$(SHARD_TOTAL=$total bash scripts/ci-shard-packages.sh --verify >/dev/null 2>&1; echo $?)"

  shard0="$(SHARD_INDEX=0 SHARD_TOTAL=$total bash scripts/ci-shard-packages.sh)"
  check "N=$total: cmd/downshift pinned to shard 0" \
    "$(printf '%s\n' "$shard0" | grep -qx 'github.com/tiagovilasboas/downshift/cmd/downshift' && echo 0 || echo 1)"
  check "N=$total: shard 0 holds only cmd/downshift" \
    "$([ "$(printf '%s\n' "$shard0" | grep -c .)" = 1 ] && echo 0 || echo 1)"
done

# Determinism: two runs produce identical output.
a="$(SHARD_INDEX=1 SHARD_TOTAL=2 bash scripts/ci-shard-packages.sh)"
b="$(SHARD_INDEX=1 SHARD_TOTAL=2 bash scripts/ci-shard-packages.sh)"
check "deterministic across runs" "$([ "$a" = "$b" ] && echo 0 || echo 1)"

echo "Full package count: $full_count"
exit "$fail"
