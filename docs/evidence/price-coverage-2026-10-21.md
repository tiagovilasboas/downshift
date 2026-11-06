<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# S2: Price coverage — exec event log and battery (2026-10-21)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Status: completed 2026-10-21.**

---

## Method

Inputs (read-only):
- `~/.downshift/events.jsonl` (1 570 exec event rows)
- `~/.downshift-eval/runs/four-arm-part1-2026-10-21/**/result.json` (46 runs,
  model ids from `models` field)
- `internal/catalog/catalog.json` at commit `97ee5479`
  (`prices_as_of: 2026-10-19`, 42 priced model entries)

For each model id seen in the battery and in exec events: check whether
`input_cost_per_1m`, `output_cost_per_1m`, `cache_read_cost_per_1m`,
`cache_write_cost_per_1m` are all non-null in the catalog.

---

## Battery model coverage

| Model id | Quality | Cost in battery | % of total | Arms |
|---|---|---|---|---|
| claude-sonnet-5-5 | exact | $20.88 | 52.4% | compact, control, routed |
| claude-opus-5-5 | exact | $18.71 | 46.9% | opus |
| claude-haiku-5-5 | exact | $0.26 | 0.6% | compact, routed |

**Coverage: 3/3 exact (100% of battery cost).** All four price fields present for
each model. No `unpriced` or `upper_bound` models in the battery.

Total battery cost: $39.84. Priced models: 100%.

---

## Exec event model id coverage

The exec events (`~/.downshift/events.jsonl`) record `requested_model` and
`final_model` fields for routing decisions, not for exec/compactor events
(exec events track `family`, not model). The model ids seen come from the
telemetry routing layer.

| Model id | Quality | Count | Notes |
|---|---|---|---|
| claude-sonnet-5-5 | exact | 64 | Full price in catalog |
| claude-opus-5-5 | exact | 47 | Full price in catalog |
| claude-haiku-5-5 | exact | 36 | Full price in catalog |
| claude-fable-5-1 | exact | 14 | Full price in catalog |
| claude-haiku-4-5 | exact | 2 | Full price in catalog |
| unknown | **unpriced** | 90 | Gateway slug — no canonical id |
| haiku | **unpriced** | 90 | Gateway slug — maps to haiku family, no version |
| sonnet | **unpriced** | 83 | Gateway slug — maps to sonnet family, no version |

**Gateway slugs (`unknown`, `haiku`, `sonnet`):** 263 of 426 model id occurrences
(62%) are unresolvable gateway slugs with no price in the catalog. These come from
the KiroCrew adapter, which reports the gateway's display name instead of the
canonical model id. Exec events from Claude Code report the full id and are
priced exactly.

---

## Proposed `quality` vocabulary (T2 input)

| Quality value | Meaning | When to use |
|---|---|---|
| `exact` | All four price fields present and used | claude-sonnet-5-5, claude-opus-5-5, etc. |
| `upper_bound` | Only input+output known; cache prices assumed zero or max | Models with partial catalog entries |
| `inferred` | Model id is a slug; priced at the most likely family member | `haiku` → `claude-haiku-5-5`, `sonnet` → `claude-sonnet-5-5` |
| `unpriced` | No resolvable id; excluded from `Saved` | `unknown`, gateway ids with no family match |

**Proposed resolution for the slugs:**
- `haiku` → infer `claude-haiku-5-5` (current default family member)
- `sonnet` → infer `claude-sonnet-5-5`
- `unknown` → `unpriced`; excluded from `Saved`; counted separately

---

## Impact of unpriced models on `Saved`

The 263 gateway slug events are **exec routing decisions**, not cost-bearing
children. They affect the `decisions` count in `routing` but not `Saved`
(which is computed from child `modelUsage`, not routing decisions). Their
direct impact on the `net_counterfactual_cost_v1` formula is zero in the
current implementation.

Future per-harness reporting (T4b) will need to handle KiroCrew slugs when
attributing Compactor events by harness; `inferred` quality is acceptable there.

---

## Verdict

**D2 and D4 unaffected.** The battery is 100% priced. Gateway slugs from KiroCrew
affect exec event model attribution but not formula accuracy. The `quality`
vocabulary above is ready for T2 (`PriceBook`) and the spec (T0).

Catalog `prices_as_of: 2026-10-19` is current. No tier threshold models
(haiku-5-5 above 100k, grok above 200k) were seen in the battery.

---

## Reproducibility

Script: inline Python, session artifact.
Catalog: `internal/catalog/catalog.json` at `97ee5479`.
Exec log: `~/.downshift/events.jsonl` (1 570 rows, 2026-10-21).
Battery: `~/.downshift-eval/runs/four-arm-part1-2026-10-21/` (46 result.json).
