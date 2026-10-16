<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Handoff 2026-10-16: open work after the hardening audit

Written for the next agent (Codex) to pick up. Each task is small, has a source of evidence and a "done when". Rules for all:
small deliveries (one commit, one push, one CI check), tests first for behaviour changes, no tuning on the five benchmark splits
(`tasks`, `holdout`, `fresh`, `heldout2`, `blind-vitrine`), benchmarks compared before and after with identical commands, no push
that touches release tags without the owner's approval.

State at filing: main has the audit fixes (beta.14 released), the language guard (`48c0dc4`), docs reorganised by audience, and the
tasks of [2026-10-16-multilang](../2026-10-16-multilang/README.md). Evidence pages: `docs/evidence/hardening-audit-2026-10-15.md`.

## State at the end of the Claude session (2026-10-16)

Landed on `main`, each checked with the full suite, the five benchmark splits (identical item by item, gates pass) and CI:

| Commit | What |
|---|---|
| `48c0dc4` | Language guard and Portuguese risk terms (M1 core, M2) |
| `82c2c1b` | Claude Code small tier is `claude-haiku-4-5`; cache writes priced at the write rate |
| `def8ac3` | Language guard reads the full prompt; reason code `UNSUPPORTED_LANGUAGE` in events, `stats`, `why` (M1 telemetry) |
| `3dd246a` | Unknown model prices stay unknown: no free spend, no savings against an unpriced target |
| H1 commit | Bounded delegation briefs do not default to COMPLEX by length (H1) |

Not on `main`: branch `codex/session-metadata-wip` (pushed). It holds the Codex session-metadata work (see H13). It is not reviewed
and was not run against current `main`.

Labs (private, pushed): M3 labels frozen (`653bfd9`), candidate runners and the M4 evidence (`b640e92`). Result of M4 on the frozen
M3 set (170 prompts), identical for the current classifier and the multilingual embedding: tier accuracy 0.341, confident coverage
0.253, unsafe confident recommendations 10, unsafe raw recommendations 74. On the five legacy splits the candidate is equal or
slightly worse (`tasks` 0.730 against 0.735). Recorded decision: reject. M5 (LLM reference) is blocked: the account had extra usage
enabled and no request was sent. Human adjudication of the M3 labels is still pending.

| ID | Priority | Task |
|----|----------|------|
| H1 | Done | Bounded-brief classifier change (landed; keep the note that `blind-vitrine` is not a clean held-out check for it) |
| H2 | High | Codex session model missing from the catalog: every Codex spawn is held |
| H3 | Done | Cursor: `inherit` uses parent `model_id` baseline (`3015fc9`) |
| H4 | Done | Hook failure events record harness (`f8acfbf`) |
| H5 | Medium | Six long briefs with fewer than two limits are still confidently TRIVIAL (go to Haiku) |
| H6 | High | Owner decisions with finished diffs: upshift to the needed tier; risky write with unknown baseline |
| H7 | Medium | Fresh routed-versus-control run after the Haiku fix, and the README evidence screenshots |
| H8 | Low | Outcome suite re-run on the model that actually runs (Haiku 4.5), 40 tasks, about $5 to $6 |
| H9 | Low | `hookctx` timing test is flaky under a full parallel run |
| H10 | Low | Local clean-up: shadowed binaries, three old log lines with prompt text |
| H11 | Medium | harness-core leftovers found during the renaming |
| H12 | High | Release beta.15 (needs the owner's approval) |
| H13 | High | Review and integrate the session-metadata work in `codex/session-metadata-wip` |
| H14 | High | Unsafe confident recommendations on the multilingual set: 10 of 170, with or without the embedding |

## H1: bounded-brief classifier change

**Problem.** In real use the router rarely leaves Opus. Of 196 agent-written delegation prompts (local Claude Code transcripts,
2026-10-06 to 10-16; 178 have 80 or more words), the +1 for long prompts tips the class to COMPLEX in 37 of 64 dev COMPLEX results;
the COMPLEX keywords that fire are context words (graph, rewrite, concurrently, architecture). Unconfident MEDIUM downshifts are
then held by R1.

**Do.** A "bounded brief" is a prompt of 80+ words with at least two of: a scope limit, a test command, a reply budget. Inside one:
drop the length +1, TRIVIAL keywords do not vote, a single COMPLEX keyword does not vote unless it is review or retry wording
(two different COMPLEX keywords still win), and the brief counts +2 toward MEDIUM. Non-brief long prompts keep the +1. The risk
floor is untouched. Source: `~/.downshift-eval/opus` (diff in `internal/core/classifier.go`, `bounded_brief_test.go`).

**Done when.** Held-out set (79 prompts, newest 40%): COMPLEX 25 to 13, confident MEDIUM 0 to 26, no risky prompt downshifted;
all five benchmark splits identical item by item. The final form was chosen after seeing `blind-vitrine`, so it is not a clean
held-out check for this change: say so in the commit.

## H2: Codex session model not in the catalog

**Problem.** The Codex adapter reads the session model correctly (config.toml, `models_cache.json`) but the catalog has no entry
for it (`gpt-6.1-sol` on the owner's machine), so every Codex spawn has an unknown baseline and is held. Since 2026-10-15, 7 of 8
Codex decisions were unknown. Test: `TestDefect_CodexSessionModelMissingFromCatalog` in the `opus` clone.

**Do.** Add the model to the Codex section with tier and price from a verifiable source (`models_cache.json`, the vendor price
page), not guessed. If the tier cannot be verified, say so and leave it out.

**Done when.** The defect test passes and a Codex spawn with that session model gets a known baseline.

## H3: Cursor explicit `inherit`

**Problem.** With no model in the call, the parent's `model_id` is the baseline (small-tier downshifts only). With
`model:"inherit"` the same parent model is ignored and the spawn logs unknown. `TestRequestedModel_Baseline` pins the current
behaviour. Test: `TestDefect_InheritIgnoresParentModel`.

**Do.** Treat `inherit` like a missing model, keeping the small-tier-only rule for a guessed baseline; update the pinning test
with the reason.

**Done when.** Both tests agree; the guessed-baseline rule still holds a mid downshift.

## H4: hook failures record the harness

**Problem.** `telemetry.Failure` has no harness field, so the two `INVALID_EVENT` rows of 2026-10-10 cannot be attributed. Test:
`TestDefect_HookFailureRecordsHarness`.

**Done when.** The runner passes the harness it was started for; the test passes; no payload content is recorded.

## H5: long briefs with fewer than two limits

**Problem.** After H1, six long briefs are still confidently TRIVIAL because of a word such as "format" or "git log", or a
constraint such as "no git commit".

**Do.** Analyse on the dev set only; propose the smallest rule (for example, a TRIVIAL keyword inside a negation or a command
quote does not vote). Judge on the held-out set. Reject if any benchmark split changes.

## H6: owner decisions with finished diffs

1. **Upshift to the needed tier** (`~/.downshift-eval/design`, uncommitted). Today a confident MEDIUM task starts on Haiku and is
   upshifted to Opus, but the same task starting on Opus is downshifted to Sonnet. The diff picks the lowest tier at or above both
   the needed tier and the current tier, keeps the strongest on a guessed baseline or with `explicit_upshift`, and adds
   `Decision.BaselineGuessed`. Roadmap P2.3 calls it an owner decision: ask before landing.
2. **Risky write with unknown baseline.** Guardrail R5 only fires on a DOWNSHIFT verdict, so with no baseline a risky task can be
   written to mid even if the child would inherit frontier. Suggested fix: hold a risky write below frontier when the baseline is
   unknown. It reverses `TestDecisionRewritePolicy` and `t1-deterministic-lift.md`; ask first.

## H7: fresh measurement and README screenshots

After the Haiku fix is released: one routed and one control run of `mech-rename` (about $1.1 at list price) with
`~/Github/downshift-labs/tools/harness-runs`, then screenshots of `downshift session`, `downshift session --why` and the
per-model cost from Claude Code's own report, generated from the real output. The runs recorded on 2026-10-15 priced Haiku at the
wrong rate (Downshift's own usage rows sum to $0.011 against $0.212 reported by Claude Code), so they are not usable as evidence.

## H8: outcome suite on the model that runs

The recorded small tier is a Qwen model through OpenRouter, so the -20 point gap is not about Haiku. A 9-task subset on Haiku 4.5
passed 6 of 6 routed-to-small tasks (too small for rates). A full 40-task run needs the owner's approval for the cost.

## H9: flaky timing test

`internal/hookctx` `TestRunCommand_BackgroundChildDoesNotHang` fails under a full parallel run (about 5 s) and passes alone.
Find the load sensitivity or loosen the bound with a reason.

## H10: local clean-up (ask before deleting)

- `~/.local/bin/downshift` (2026-10-10) and `/opt/homebrew/bin/downshift` (2026-10-06, no `context` command) are shadowed by
  `~/go/bin/downshift`; remove them or fix PATH order.
- `~/.downshift/agents.jsonl` has 3 old lines with prompt text and raw session ids (from the Kiro hook built before the privacy
  fix, since replaced). Redact them.

## H11: harness-core leftovers

`harnesses/cursor/install-cursor-cloud.sh` points at `adapters/cursor-cloud-overlay`, which does not exist; the installer copies
`team.md` to `~/.cursor/TEAM.md`, which other docs still name in uppercase; 20 broken relative links; `check-harness-extensions-pack.sh`
fails on three skills missing from `personal-skills-pack.txt`; `adapters/` mixes real scripts with wrappers that forward to `scripts/`.
The "model by task" rule belongs to the router (Downshift), not to the harness core: do not add it there.

## H12: release beta.15

Contents: performance prefilter, single transcript wait, `HELD_GUESSED_BASELINE`, Haiku 4.5 catalog fix and cache-write cost
split, language guard and its telemetry, H1 if landed. The CHANGELOG already has the entries under Unreleased. Tag and release only
with the owner's approval.

## H13: session metadata work (branch `codex/session-metadata-wip`)

**What it is.** Native or operator session metadata (identity, tier, supported effort) kept apart from price knowledge: new
`core.SessionModel`, `session_resolver.go`, `Metadata` and `Unordered` on `SessionList`, discovery ordering, adapter changes
(Claude Code, Codex, Cursor, Antigravity, KiroCrew), native-selection rules that treat a zero or missing price as unknown, docs
(`session-discovery.md`, `session-models.md`), `tools/multilang`, the evaluation status and m6 notes. Task text of the intent:
`docs/internal/tasks/session-metadata-price-safety-2026-10-16.md` (its price part already landed in `3dd246a`).

**Status, checked.** The tree compiles. `internal/core` passed after fixing five test fixtures that set only `OutputM`
(commit `9b79c2a`; the new rule treats a missing input or output rate as unknown). Not run: the full suite, the five benchmark
splits and `eval-outcome` on top of current `main`; nothing was reviewed line by line. The branch is based on `fb0a4b4`, so it must
be rebased onto `main` (the catalog, cost and telemetry files were rewritten on both sides).

**Do.** Rebase, run the full suite and the benchmark comparison, review the adapters one by one, split into small commits
(core session types, discovery, adapters, docs), and drop `tools/multilang` if it duplicates the labs runners.

**Done when.** Suite green, benchmarks identical item by item, CI green, each adapter's behaviour covered by a test at
`core.Route` or `runHookAdapter`.

## H14: unsafe confident recommendations on the multilingual set

**Evidence.** On the frozen M3 set the classifier gives 10 unsafe confident recommendations out of 170 and 74 unsafe raw
recommendations. It is not recorded whether the run included the language guard; check the run manifest against the commit used.

**Do.** List the 10 by category (mixed-language, risk, tier), find which rule made them confident, and fix with the smallest rule
that does not weaken any floor. Decide the human adjudication of the labels first, because the count depends on them. Do not use
the five legacy splits to tune.

**Done when.** Zero unsafe confident recommendations on the M3 set without a drop of coverage on the legacy splits, or the cases
are documented as label disagreements.
