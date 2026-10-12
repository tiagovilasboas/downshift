# Prompt corpus and refinement protocol

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

How classifier signals are refined without fitting them to the data they are
judged on.

## Data
- `internal/core/testdata/corpus/*.json` — 200 hand-labelled prompts (tier, optional
  `want_hold`, `known_gap`). Labels were written by agents that could not run the
  router. `TestPromptCorpusFiles` fails on a **wrong write** (the model would be
  rewritten to a tier other than the label); a held spawn only loses a saving and is
  reported as a metric. Remove `known_gap` when a gap is fixed.
- `downshift-labs/research/prompts-1000/` — 1000 synthetic developer prompts with the
  generator's own `dev_difficulty`. That label is a guess, not ground truth; the set
  is for finding error families, not for measuring accuracy.
- `TestPromptReport` (env `DOWNSHIFT_PROMPT_REPORT_IN` / `_OUT`) scores a directory of
  prompt files and writes one JSON row per prompt. No new command: it is a test.

## Protocol for a signal change
1. Split by id parity: **odd = development**, **even = control**.
2. Mine wording and read failures on the development half only.
3. Change `RawSignals`; add a table-driven case with look-alikes that must NOT match.
4. Re-run the report. Accept only if the control half improves **and** the harm
   count (medium/hard prompts reaching small) does not rise.
5. Run `go test ./...` and `downshift benchmark` against a baseline built from the
   commit **before** the change, never an old binary.

## Result of the first round (mechanical edits and commands, EN + PT-BR)
Trivial prompts reaching small: development 27 → 60 of 94, control 16 → 34 of 106.
Harm (medium/hard to small): 4 and 7 before, 4 and 7 after. Benchmark tier accuracy
against the previous commit: tasks 66.5 → 70.5, holdout 100 → 100, heldout2
35.8 → 39.2, fresh 47.0 → 50.0. Two signals were first too broad (a long "explique"
analysis and "run X and summarise by root cause") and were limited to short prompts.

## Still open
656 of 1000 prompts match no signal (default MEDIUM). The remaining families are
topic words that override scope ("migration" in a file name), hard work without
vocabulary (CRDT, SSA, JIT), and PII outside the risk floor. See `known_gap` cases.

## Tried and rejected: migration as Medium unless scale words appear
Hypothesis: `migrat*` alone is not architecture (a migration file, a library swap).
Making it Medium and Complex only with scale words (several services, production
data, zero downtime, backfill, dual write, cutover, planning) halved the unnecessary
frontier writes (development 45 → 22, control 35 → 19) but lost real hard work
(hard prompts reaching frontier 43 → 30 and 51 → 34) and broke three tests that fix
the owner's intent: "rename + migration" ties to Complex, "migrate the orders table
… with zero downtime" is frontier, and a hard corpus case. A narrower damper for
"a migration file" context catches about 11 development prompts and also hit 2 hard
ones. Not adopted. Note a latent bug found on the way: the PT-BR pattern
`\bmigr[aeo]\w*` also matches English "migration/migrate", so English migration words
score 3 + 3. Removing the double count alone worsened `hard → frontier` (43 → 42, 51 →
50) and broke `HARD-04`, so several hard cases rely on it today. Deciding what a bare
migration word should mean is a design choice for the owner.

