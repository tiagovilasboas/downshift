#!/usr/bin/env bash
# Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
# SPDX-License-Identifier: BUSL-1.1
#
# fresh-guard.sh: benchmark/fresh.json is evaluation-only. A change set that
# edits it together with anything that tunes the router (signals, risk
# table, classifier, semantic prototypes, learned weights, prototype tools)
# is rejected: fresh numbers stop meaning anything once rules are fitted to
# them. Reads changed paths (one per line) on stdin; exit 1 on violation.
set -euo pipefail

changed=$(cat)
grep -qx 'benchmark/fresh.json' <<<"$changed" || exit 0

tuning=$(grep -E '^(internal/core/(signals|risk|classifier)\.go|internal/semantic/data/|internal/routingv2/classifier/weights/|tools/minilm/)' <<<"$changed" || true)
if [ -n "$tuning" ]; then
  echo "fresh-guard: benchmark/fresh.json changed together with router tuning files:" >&2
  echo "$tuning" | sed 's/^/  /' >&2
  echo "Split the change: fresh prompts and classifier rules never move in the same PR (benchmark/README.md)." >&2
  exit 1
fi
