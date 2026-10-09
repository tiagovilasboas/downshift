# Beta exit — task map

This document tracks everything required to remove the **beta** label from
`harness-downshift`. Criteria match [README.md § Status](../README.md#status).
Update checkboxes as work lands; link PRs/commits in the **Done** column when
closing a task.

The current runtime, quota, cost and evidence queue is [gap-tasks.md](gap-tasks.md),
with acceptance criteria, dependencies and dated validation status. Close a
checkbox here only after its corresponding acceptance evidence is verified.

**Legend:** `[ ]` open · `[~]` in progress · `[x]` done · `—` blocked on external harness/vendor

---

## Pillar 1 — Real-spawn verification (rewrite **honored**)

Proof that the harness executor applied the hook’s model, not only that
`outcome: "rewrite_emitted"` was logged.

| ID | Task | Owner | Done |
|----|------|-------|------|
| P1.1 | Document “emission vs honored” in install guide (`docs/session-models.md` + matrix) | eng | [x] |
| P1.2 | Hook-layer E2E in CI (`TestHookE2E_RewriteEventStats`) | eng | [x] |
| P1.3 | **Manual protocol**: one paid Claude Code session, trivial Task, capture stderr + child model in UI/logs | dogfood | [x] `docs/evidence/claude-code-rewrite-honored-2026-10-06.md` (child ran on Haiku 4.5 per the API-reported model in its transcript, under a Sonnet 5.5 parent; billing cross-check still open under P3) |
| P1.4 | **Automated smoke** (CI gate): script that runs `downshift try` + adapter golden JSON payloads (`scripts/smoke-test.sh`) | eng | [x] |
| P1.5 | Codex `multi_agent_v2`: repeat P1.3 on a known-good build; record in `docs/harness-matrix.md` | dogfood | [x] `docs/evidence/codex-rewrite-honored-2026-10-02.md` |
| P1.6 | Cursor Pro/Ultra usage-based: repeat P1.3; update matrix row | dogfood | [ ] |
| P1.7 | Telemetry field `rewrite_honored: bool` (only when harness exposes post-spawn model) — schema + privacy review | eng | [x] |

**Exit:** at least **one** harness with P1.3 write-up + matrix row **Yes** with date; P1.7 optional until upstream exposes signal.

Codex P1.5 write-up landed 2026-10-02. Claude Code P1.3 landed 2026-10-06 after a schema-rejection bug (full id vs family name) was found and fixed; its evidence is the API-reported model in the child transcript, not billing.

---

## Pillar 2 — Multi-user evidence

Graduation needs independent operators, not only one `events.jsonl`.

| ID | Task | Owner | Done |
|----|------|-------|------|
| P2.1 | `downshift stats --export` JSON (no prompts) for sharing aggregates | eng | [x] |
| P2.2 | Contributor guide: how to paste export into issue/PR without leaking paths | docs | [x] |
| P2.3 | **2+ external contributors** (or teammates) run hooks 1 week; aggregate exports | community | [ ] |
| P2.4 | README “Real session data” table updated with multi-source note + date | docs | [ ] |
| P2.5 | Optional: anonymised `stats --export` CI artifact from maintainer machine (directional) | eng | [x] |

**Exit:** README cites **≥3 distinct session sources** OR **≥2 non-maintainer exports** in a release note.

**P2 status (2026-10-06):** open. It needs independent operators, not more data from one machine. If you run Downshift for a week, a `downshift stats --days=7 --export` is the contribution: see [stats-export.md](stats-export.md) and the stats-export issue template.

---

## Pillar 3 — Billing before/after (real dollars)

Replace directional normalised units with provider-grounded savings.

| ID | Task | Owner | Done |
|----|------|-------|------|
| P3.1 | Event schema + `CostUSD` / `FillRealCost` plumbing | eng | [x] |
| P3.2 | Claude Code **PostToolUse** hook + usage linkage | eng | [x] |
| P3.3 | Wire PostToolUse in maintainer `settings.json` + document in README quickstart | dogfood | [x] |
| P3.4 | **≥50** `outcome: "usage"` events linked to prior PreToolUse decisions | dogfood | [ ] |
| P3.5 | Non-zero token-derived savings, known baseline and native provenance: [2026-10-09 audit](evidence/billing-gap-audit.md), QA PASS; not an invoice or controlled-period comparison | dogfood | [x] |
| P3.6 | **Control group**: `DOWNSHIFT_NO_ROUTE=1` / `--no-route` baseline events | eng | [x] |
| P3.7 | Report template: same calendar week, Anthropic/OpenAI dashboard vs `stats --export` (`docs/billing-comparison-template.md`) | docs | [x] |
| P3.8 | PostToolUse for Codex/Cursor when harness exposes usage (track upstream) | eng | — |

**Exit:** P3.4 + P3.5 + P3.7 filled for **one** billing period (maintainer sign-off). Still open: 5/50 linked agents and no real control-period comparison. P3.5 alone does not close the pillar.

Historical P3.4 finding (2026-10-06; superseded counts and current provenance in the [2026-10-09 audit](evidence/billing-gap-audit.md)): the Claude Code PostToolUse payload for an async `Agent` launch carries no token usage (`tool_response` is launch metadata only, `duration_ms` is a few ms), which is why the log has no `usage` events. The subagent transcript at `tool_response.outputFile` does carry per-message `message.model` and `message.usage` (input, output, cache creation/read; the same message id repeats while streaming, so take the last per id). It is not complete at launch time, so usage is read when the subagent stops: the SubagentStop hook (`downshift claude-code-subagent-stop`, payload `agent_id` + `agent_transcript_path`) prices it from that transcript and links it to the decision through the hashed agent id. Verified 2026-10-06 against a real transcript (18 input, 311 output, 82,900 cache tokens). It is not enabled in the maintainer `settings.json` yet, and savings stay zero while the decision's `requested_model` is `unknown`. The launch payload does carry `resolvedModel`, now recorded as an `outcome: "resolved"` honor observation (P1.7).

---

## Pillar 4 — Benchmark & classifier quality

| ID | Task | Owner | Done |
|----|------|-------|------|
| P4.1 | Seed dataset health checks (`DatasetHealth`, no duplicates) | eng | [x] |
| P4.2 | Expand curated set **30 → 108** tasks | eng | [x] |
| P4.3 | `internal/graphify` library. A nil fetcher never escalates. `core.Route` does not escalate via the graph. | eng | [x] |
| P4.4 | Grow to **200** tasks (real prompts, rubric in `benchmark/README.md`) | curation | [x] |
| P4.5 | 500-task seed+holdout net existed (200+300); datasets moved to **downshift-labs** since v0.1.0-beta.7; public [benchmark/REPORT.md](../benchmark/REPORT.md). | curation | [x] |
| P4.6 | `downshift benchmark --report` + `--gate` exist and can pass on the in-tree files. They gate the burned net, not unseen traffic. | eng | [x] |
| P4.7 | Confidence intervals / bootstrap on holdout (document in benchmark README) | eng | [x] |
| P4.8 | Graphify **MCP fetcher** for KiroCrew (optional; measure FRONTIER→MID delta) | eng | [~] `DOWNSHIFT_GRAPHIFY_CMD` command fetcher, fail-open; native MCP socket still optional |
| P4.9 | **MiniLM semantic boost on by default** (local hash; external embed falls back to hash; `DOWNSHIFT_MINILM=0` opts out — `docs/minilm-semantic.md`) | eng | [x] |
| P4.10 | Retrain prototypes with `sentence-transformers` + measure tier accuracy delta on holdout (`benchmark/minilm-holdout.json`) | eng | [x] |

**Exit (not met on this dataset):** P4.5 regression files live in private **downshift-labs** (`ci-eval` gates). The public repo ships [benchmark/REPORT.md](../benchmark/REPORT.md) only. Historical note (2026-10-04, `eb116ed`): holdout was a burned regression net (template stems, post-holdout signal tuning to 100% tier accuracy), not proof of generalization. Do not remove the beta label on that dataset alone.

P4.3's checked box is the library, not graph-aware routing on the hook. `TestHint_NilFetcher_OfflineMode` shows a nil fetcher never escalates. See [graphify-integration.md](graphify-integration.md).

---

## Pillar 5 — Harness coverage (external + docs)

| ID | Task | Owner | Done |
|----|------|-------|------|
| P5.1 | `docs/harness-matrix.md` maintained per release | docs | [x] |
| P5.2 | Antigravity catalog entries (`flash_lite` / `flash` / `pro`) | eng | [x] |
| P5.3 | Revalidate Cursor free/legacy discard quarterly | dogfood | [ ] |
| P5.4 | Revalidate Codex `multi_agent_v2` schema quarterly | dogfood | [ ] |
| P5.5 | Antigravity real child observed on desktop 2.21.1, 2026-10-09; [native evidence](evidence/antigravity-executor-ack.md), QA PASS, build-specific scope | dogfood | [x] |
| P5.6 | Issue template: “plan compatibility” checklist for bug reports | docs | [x] |

**Exit:** matrix has **no stale TBD** for harnesses you ship support for; known “No” rows stay honest.

---

## Infrastructure & honesty (supports all pillars)

| ID | Task | Owner | Done |
|----|------|-------|------|
| I1 | Multi-process JSONL lock (`LockFile` in `AppendTo`) | eng | [x] |
| I2 | Log rotation wired on append (`RotationPolicy`) | eng | [x] |
| I3 | Commit `go.sum` + CI cache (silenced in CI since zero external modules in `go.mod`) | eng | [x] |
| I4 | README “What’s missing” table synced with this doc | docs | [x] |
| I5 | `ExportSummary` includes `baseline` + `usage_linked` counts | eng | [x] |
| I6 | Session route cache persistence audit (multi-process: stateless hooks + in-memory memo documented) | eng | [x] |

---

## Suggested execution order (sprints)

1. **Code done:** P4.9 default local hash boost, P4.10 neural holdout measurement, P3.3 PostToolUse command shipped, inferred honor in stats, Graphify command fetcher.
2. **Dogfood now:** P3.4 (5/50 genuinely linked agents) and P3.7 (fill the period comparison with real controls). P3.5 technical token-derived criterion passed on 2026-10-09; the period exit remains open.
3. **Community:** P2.3–P2.4 exports from other operators.
4. **Before label removal:** P1.6, P5.3–P5.4, all five pillar exit checks signed in a release note. P5.5 observed 2026-10-09 on desktop 2.21.1; Codex P1.5 remains inferred, recorded 2026-10-02.

---

## Release checklist (flip beta → stable)

- [ ] All five **Exit** lines above satisfied.
- [ ] README badge + Status section updated (remove beta; link here).
- [ ] CHANGELOG entry with evidence links (exports, benchmark numbers, matrix date).
- [ ] No open **P1** or **P3** blockers tagged `release-critical`.

When in doubt, keep **beta** — the label is a promise to new installers.
