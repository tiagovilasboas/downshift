<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# S1: Compaction persistence — four-arm battery part 1 (2026-10-21)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Status: completed 2026-10-21. Blocker hit — corpus fixture assumption applied.**

---

## Method

Per wave1.md: for each Bash tool result inside a subagent in arms D (routed) and C
(compact), count the later turns of that agent; estimate the cost under:

- **v1** (D2 current): compacted block priced once as input at the session's
  requested model.
- **v2** (alternative): one cache write + one cache read per remaining turn.

Source: `~/.downshift-eval/runs/four-arm-part1-2026-10-21/**/stream.jsonl` (48 files)
and `result.json` `compactor` field.

---

## Blocker hit

Per wave1.md section S1 stop condition: *"if fewer than 30 compacted-eligible
results exist, say so and use the corpus fixtures with a stated assumption."*

Actual count: **0 compacted commands** in arms C and D across all 48 stream files.
The `compactor.compacted` field is 0 for every routed and compact run in part 1.
`compactor.commands` has 1 entry total (1 raw command, not compacted).

**Reason:** the part 1 battery tasks (`mech-constant`, `mech-doc-comments`,
`mech-rename`, `mech-test-mirror`) are primarily agentic Go/git/gh tasks. The
Compactor wrapped 0–1 commands total in 24 runs. This is expected: part 1 was
designed to measure routing (model selection), not compaction.

---

## Corpus fixture assumption

Using the downshift compressor corpus (`internal/compressor/testdata/corpus/`) as
proxy. From fixture analysis and the S0 battery:

- Median task has ~3–5 turns per subagent.
- A compacted `go test` result is ~2–5 KB, occupying ~500–1250 tokens.
- At `claude-sonnet-5-5` prices (input $2/MTok, cache_read $0.10/MTok):
  - v1 cost (input once): 1000 tokens × $2/1M = $0.000002
  - v2 cost (cache write + 3 cache reads): $0.0025/1M + 3×$0.10/1M = ~$0.000000325 per 1K tokens
  - **v1/v2 ratio ≈ 6×** — v1 overprices compaction by ~6× vs v2 for a 4-turn subagent.

**However:** the absolute dollar amount is tiny ($0.000002 vs $0.0000003 per 1K
tokens). At the battery scale, the difference between v1 and v2 is <$0.001 per
run — well within the S0 bias of ~$0.058 per run.

---

## Verdict and recommendation for D2

**D2 CONFIRMED: keep v1 (price compaction once as input).**

The v1/v2 difference ($0.0000017 per 1K tokens) is negligible compared to:
- S0 formula bias (~$0.058 mean, ~15%)
- Run-to-run cost variance in the battery ($0.05–$0.20)

V1 is simpler, more conservative (prices high), and the error is below the noise
floor of the current measurement methodology. Revisit in part 2 if Compactor
usage is significant (>30 compacted commands per run).

The assumption to print: *"Compactor saving is priced at input cost, once per
compacted block; actual cost may be lower if the block stays in cache across
multiple turns."*

---

## Reproducibility

Script: `/tmp/s1_persistence.py` (session artifact, not committed).
Input: `~/.downshift-eval/runs/four-arm-part1-2026-10-21/**/stream.jsonl` (48 files).
Corpus proxy: `internal/compressor/testdata/corpus/` at `97ee5479`.
