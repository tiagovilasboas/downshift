<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Open issues (2026-10-20): what the live panel exposed

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Found while using the agents panel in a live Claude Code session (session cbf6de37). Each item is
one small PR. Status is what we have observed, not what we hope. Items marked **decision** need the
owner first. Companion to [the evidence roadmap](../2026-10-20-evidence-roadmap/README.md).

## 1. Compactor does not appear in a live session (investigating)

- Observed: `session --panel` has no `compactor` block for cbf6de37. The exec log has compacted
  `go-test` records, and the newest records (14:18 onward) carry no `session` key.
- Shipped: #320 (session on the wrapped command and in the log), #321/#322 (Compactor tile, 1.2.6).
- Open: is the session lost on the hook path, or is the live hook running an older binary?
- Next: root cause with file:line; then a live check with `Bash(go test:*)` in the allow list.

## 2. A subagent labelled Haiku 5.5 ran as claude-haiku-4-5 (investigating)

- Observed: the panel says "Haiku 5.5"; the decision record says `to_model: claude-haiku-4-5`.
- Open: which ID the `haiku` alias resolves to, and whether the record or the label is wrong.
- Impact: any Haiku arm in the experiment must pin the exact ID (`claude-haiku-5-5`) and verify it.

## 3. "Downshifted 0" with five switches (investigating)

- Observed: `switched 5`, `saved_children 0`, `work_saved 0`, `cache_saved 0`.
- A switch is a decision; "Downshifted" counts only usage records whose direction came out cheaper.
  The one Sonnet → Haiku child (216k tokens) was not counted as cheaper.
- Open: is the cause the ID in item 2 not matching the price tier, or the usage record naming another
  model? This is the same question as item 2, so they are answered together.

## 4. Review subagents are classed COMPLEX and raised to frontier (investigating)

- Observed: "QA: validity of the experiment design" ran as `effort changed (frontier)`, sonnet kept,
  with `CLASSIFIER_CONFIDENT`. Its cost is higher than a mechanical task.
- Open: which signals fired, and whether review-type prompts are over-classified.
- Decision: whether a fix lands in `internal/classify` with a table-driven test (repo rule), or
  whether the label is accepted as correct.

## 5. Privacy of the session attribution (investigating)

- Question: does the new `--session=<hash>` put anything other than a hashed id into the command text
  or the log?

## 6. DeepSeek is not in the agents panel (by design)

- The panel tracks Claude Code subagents only. Downshift routes Claude models; a DeepSeek run would be
  a separate experiment, not a subagent. It must not be shown as Downshift evidence.

## 7. Experiment on hold

- Waiting on: the owner's spend cap (recommended US$ 15 hard, review at US$ 7.50), the Haiku ID fix
  (item 2), and the Haiku 5.5 price (not in our catalog).
- AppSec conditions: public or synthetic repo only; `zdr: true`, `data_collection: deny`, no
  fallbacks; key from env or keychain only.

## Order

1. Items 1 and 2 first: they decide whether any number we show is attributed to the right model and
   session.
2. Item 3 follows from item 2.
3. Item 4 is a product decision, then a test.
4. Item 7 waits for 1 to 3.
