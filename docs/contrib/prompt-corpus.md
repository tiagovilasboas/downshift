# Prompt corpus and refinement protocol

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

How classifier signals are refined without fitting them to the data they are
judged on.

## Data
- `internal/core/testdata/corpus/*.json` — 200 hand-labelled prompts (tier, optional
  `want_hold`, `known_gap`). Labels were written by agents that could not run the
  router. `TestPromptCorpusFiles` fails on a **wrong write** (the model would be
  rewritten to a tier other than the label); a held spawn only loses a saving and is
  reported as a metric. Remove `known_gap` when a gap is fixed.
- `internal/core/testdata/stability/golden.json` — what the router decides (complexity,
  confidence, tier) for each of those 200 prompts. `TestClassifierStability` fails on any
  drift, including drift inside the label. An intended classifier change regenerates it with
  `go test ./internal/core -run TestClassifierStability -update-stability`; review the diff
  next to the before/after evidence. It replaces the old check that read a private
  directory outside the repository.
- `internal/core/testdata/quality/baseline.json` — the recorded floor for offline routing
  quality: tier agreement and under-routed count of the classifier on this corpus and on the
  90-prompt domain precision set, with the commit and command that measured them, a
  tolerance in percentage points and a digest of the prompts and labels.
  `TestRoutingQualityBaseline` fails when agreement falls, or under-routing rises, by more
  than the tolerance, or when a prompt or label changed. Regenerating the stability golden
  does not move it. Raising the values after an improvement, or accepting a loss, is a
  reviewed edit of that file with the evidence; never edit labels to raise a score. It gates
  label agreement, not task success; live routed-versus-control quality is measured in
  downshift-labs and is not gated here.
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
5. Regenerate the report with `go test -count=1` (Go caches a test whose inputs are unchanged and then does not rewrite the output file).
6. Run `go test ./...` against a baseline built from the
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
ones. Not adopted. A latent bug found on the way: the PT-BR pattern
`\bmigr[aeo]\w*` also matched English "migration/migrate", so English migration words
scored 3 + 3. At the time, removing the double count alone worsened `hard → frontier`
(43 → 42, 51 → 50) and broke `HARD-04`.

## Fixed: one mention of a stem scores once
- `migrat` / `migr[aeo]` and `architect` / `re-architect`: one row each since commit
  `6c57719`, which in the same change added system-design and distributed-data
  vocabulary. `TestClassify_SingleMentionScoresOnce` pins it.
- `distributed`: a bare-word row (2) and a phrase row (`distributed lock|consensus|
  transaction|sharding|cache`, 3) both matched one mention, so "distributed lock"
  scored 5. The phrase alternatives were dropped; the bare word still covers them and
  `etcd|raft|consensus protocol` keep their own row. Measured against the commit
  before: no tier change on the 200-case corpus, the 1000-prompt set, or any
  benchmark set (tasks 73.5, holdout 100, fresh 53.0, heldout2 41.7, blind-vitrine
  46.7 tier accuracy; both gates pass). These sets are in-sample for the signal
  table, so "no change" shows no regression, not a gain. Two-word phrases that name
  two concepts ("distributed consensus" also hits `consensus`) still score twice.

## Tried and rejected: ignore Complex keywords inside quoted text
Hypothesis: a keyword inside a quoted error or log line names text, not work
(`fix the typo in the error message "non-replication"`). Dropping Complex hits found
only inside a `"..."` span (with or without backticks) changed 1 of 1000 prompts
(`mobile-023`, "Undefined symbols for architecture arm64", frontier → mid, correctly)
and no benchmark set. Not adopted: quoted errors often carry the real difficulty
(`fix "ERROR: deadlock detected" in the orders worker` would drop from frontier to
mid, the unsafe direction), and the case that motivated it, `database-026`, is an
unquoted log line ("reserved for non-replication superuser connections"), which
needs log-line detection rather than a small quoting rule.

## Tried and rejected: swapping the semantic embedder for character trigrams
The embedder in `internal/semantic` hashes whole words, so it knows only vocabulary it
has seen. A character-trigram hash (no model, no download) separated generator
difficulty about 7 points better in `scripts/embed_eval.py` (4-way centroid accuracy
0.47 against 0.40, held in Portuguese). Wired in as an option with centroids built from
the same training set, it changed the tier of **3 of 1000 prompts** and left the
benchmark sets equal (tasks 70.5 / 70.0, holdout 100, heldout2 39.2, fresh 50.0). The
semantic layer can only raise a class, fires only when the regex is unconfident, and
needs cosine 0.45, so it barely moves routing whichever embedder feeds it. Not adopted:
the lever for multi-language coverage is the signal table and structural signals, not
the embedder. A multilingual static model (potion-multilingual-128M) is untested here
(huggingface.co is blocked from the build container).

