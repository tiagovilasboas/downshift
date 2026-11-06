<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Evidence roadmap (2026-10-20): what is missing before any new number

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Goal: a savings figure we can publish, with its method, sample and interval. Today the only published
figure is the 2026-10-15 A/B (about 12% at list price, one operator, three runs per cell, 12/12 checks on
both arms). Nothing in the site or the repo may be raised until the items below are closed.

Each item is one small PR. Items marked **decision** need the owner first.

## Status of what is already done

- Compactor is the product name; `downshift compact` and `compactor` (alias) and `exec` (legacy) all run.
- `session --panel` has an optional `compactor` block per session (bytes measured, tokens estimated, never summed).
- The savvy-progress mod is 1.2.6: "Downshifted" tile, "Compactor" tile with a down arrow.
- Site: home, guides and SEO meta use Compactor; RTK is gone from published pages and the repo docs.

## A. Correctness of the numbers we already show (blocking)

1. **Audit the direction of `haiku → haiku` spawns.** Spawns with the same model on both sides and the
   label "effort changed" count as "saved from haiku" in `TokenSavings`. Take the real events of one
   session, trace `tokenDirection` and `tokenModels`, and fix the classification (same tier must read
   unchanged, whatever the effort). Add a regression test in `internal/telemetry`. Until this ships, the
   "Downshifted" figure is not evidence.
2. **Check the `Downshifted` total against the child records.** `session --panel` sums work and cache
   saved; confirm that each child is counted once (resumed children fold into one, per the code) and that
   control, held and kept children add nothing.
3. **Attribute the Compactor to a session end to end.** The hook adds the hashed session to the wrapped
   command. Verify live that a Claude Code session with `Bash(go test:*)` in its allow list produces
   `session` on `exec-events.jsonl`. Not yet observed on a live session (the harness matrix says so).

## B. Benchmark protocol (`docs/guides/benchmark-protocol.md`)

4. **Add the Compactor as an arm variable.** Each arm states whether the Compactor is on
   (`"exec": true`) or off (`"exec": false` or `DOWNSHIFT_EXEC=0`). Without it the arms mix two effects.
5. **Require the permission rule.** The Compactor only rewrites a command the user's own allow rule
   covers (`Bash(go test:*)`, `Bash(git status)`). The protocol must check the rule exists in each arm's
   `settings.json` and stop the run if it does not; otherwise the arm measures nothing and does not say so.
6. **Record bytes per run.** Add the Compactor fields to the per-run table (`raw_bytes`, `saved_bytes`,
   `compacted`, from `exec-events.jsonl` for the run's session). Keep them out of the token totals.
7. **Rule for `haiku → haiku`.** Write the classification rule from A1 into the analysis section, so the
   protocol and the telemetry agree.
8. **Vocabulary.** Use Downshifted (tokens on a smaller model, a price-table estimate) and Compactor
   (bytes measured, tokens estimated as bytes / 4) as separate metrics, never summed.

## C. Output-cutting protocol (`docs/guides/benchmark-output-cutting.md`)

9. **Update the status line.** It still reads "Protocol, not yet run" and "arm c ... once released".
   The Compactor is released and on by default; say what is measured and what is not.
10. **Keep the third-party arm unnamed.** Already done in the repo docs (PR #318); keep it that way in any
    new text.

## D. Evidence and publication

11. **Pre-register the tasks.** Freeze the task list and the hidden checks before the run, and publish
    the list of categories (the protocol already names them).
12. **Run the protocol** on at least the minimum the report needs (20 completed tasks per group, per
    `downshift report`), with the arms from B. Publish the per-run table with opaque ids and the exports.
13. **Report intervals.** Wilson intervals for acceptance; bootstrap for cost per accepted run. An
    interval that includes zero is a valid result and is published as such (as the protocol already says).
14. **Only then change the site.** Replace the 12% in the home, the guides (EN and PT), `llms.txt` and the
    evidence page with the new figure, its sample size and its interval. Until then the site keeps the
    2026-10-15 figure and says it is one trial.

## E. Site claims to keep honest now (no new run needed)

15. **Pages that cite the 12% without the sample.** Home, both usage-limit guides and the evidence page
    all say "about 12%"; each must also say "one operator, one controlled trial, list price". Check EN and
    PT match.
16. **The "Downshifted 5.6M" figure must not appear on the site** until item 1 is closed.

## Decisions for the owner

- **D1.** Which arm is the default for the public number: opusplan (B vs C, as the protocol says) or Opus
  fixed (A vs C)? Pick one before the run.
- **D2.** Is the 1-in-10 control group kept during the run? The protocol uses `DOWNSHIFT_CONTROL_SHARE=0`
  in arm C; confirm.
- **D3.** Whether to publish the per-run table at all, or only the exports. The privacy section of the
  protocol allows the table with opaque ids.

## Order

A1–A3 first (they change numbers that already exist), then B4–B8 and C9–C10 (documents), then D11–D13 (the
run), then E14–E16 (site). A and E15 can go in parallel with B.
