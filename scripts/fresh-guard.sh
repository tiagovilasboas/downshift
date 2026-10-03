#!/usr/bin/env bash
# Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
# SPDX-License-Identifier: BUSL-1.1
#
# fresh-guard.sh: benchmark/fresh.json, benchmark/heldout2.json and
# benchmark/blind-vitrine.json are evaluation-only. A change set that edits
# any of them together with anything that tunes the router (signals, risk
# table, classifier, semantic prototypes, learned weights, prototype tools)
# is rejected: fresh numbers stop meaning anything once rules are fitted to
# them. Reads changed paths (one per line) on stdin; exit 1 on violation.
set -euo pipefail

changed=$(cat)
split=$(grep -xE 'benchmark/(fresh|heldout2|blind-vitrine)\.json' <<<"$changed" || true)
[ -n "$split" ] || exit 0

tuning=$(grep -E '^(internal/core/(signals|risk|classifier)\.go|internal/semantic/data/|internal/routingv2/classifier/weights/|tools/minilm/|tools/baseline/)' <<<"$changed" || true)
if [ -n "$tuning" ]; then
  echo "fresh-guard: $(echo $split) changed together with router tuning files:" >&2
  echo "$tuning" | sed 's/^/  /' >&2
  echo "Split the change: evaluation-only prompts and classifier rules never move in the same PR (benchmark/README.md)." >&2
  exit 1
fi
