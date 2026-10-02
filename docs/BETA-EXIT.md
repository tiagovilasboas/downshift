# Beta exit — task map

This document tracks everything required to remove the **beta** label from
`harness-downshift`. Criteria match [README.md § Status](../README.md#status).
Update checkboxes as work lands; link PRs/commits in the **Done** column when
closing a task.

**Legend:** `[ ]` open · `[~]` in progress · `[x]` done · `—` blocked on external harness/vendor

---

## Pillar 1 — Real-spawn verification (rewrite **honored**)

Proof that the harness executor applied the hook’s model, not only that
`outcome: "rewrite_emitted"` was logged.

| ID | Task | Owner | Done |
|----|------|-------|------|
| P1.1 | Document “emission vs honored” in install guide (`docs/session-models.md` + matrix) | eng | [x] |
| P1.2 | Hook-layer E2E in CI (`TestHookE2E_RewriteEventStats`) | eng | [x] |
| P1.3 | **Manual protocol**: one paid Claude Code session, trivial Task, capture stderr + child model in UI/logs | dogfood | [ ] |
| P1.4 | **Automated smoke** (CI gate): script that runs `downshift try` + adapter golden JSON payloads (`scripts/smoke-test.sh`) | eng | [x] |
| P1.5 | Codex `multi_agent_v2`: repeat P1.3 on a known-good build; record in `docs/HARNESS-MATRIX.md` | dogfood | [ ] |
| P1.6 | Cursor Pro/Ultra usage-based: repeat P1.3; update matrix row | dogfood | [ ] |
| P1.7 | Telemetry field `rewrite_honored: bool` (only when harness exposes post-spawn model) — schema + privacy review | eng | [x] |

**Exit:** at least **one** harness with P1.3 write-up + matrix row **Yes** with date; P1.7 optional until upstream exposes signal.

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

---

## Pillar 3 — Billing before/after (real dollars)

Replace directional normalised units with provider-grounded savings.

| ID | Task | Owner | Done |
|----|------|-------|------|
| P3.1 | Event schema + `CostUSD` / `FillRealCost` plumbing | eng | [x] |
| P3.2 | Claude Code **PostToolUse** hook + usage linkage | eng | [x] |
| P3.3 | Wire PostToolUse in maintainer `settings.json` + document in README quickstart | dogfood | [x] |
| P3.4 | **≥50** `outcome: "usage"` events linked to prior PreToolUse decisions | dogfood | [ ] |
| P3.5 | `downshift stats` shows `Real provider cost` section with non-zero saved USD | dogfood | [ ] |
| P3.6 | **Control group**: `DOWNSHIFT_NO_ROUTE=1` / `--no-route` baseline events | eng | [x] |
| P3.7 | Report template: same calendar week, Anthropic/OpenAI dashboard vs `stats --export` (`docs/billing-comparison-template.md`) | docs | [x] |
| P3.8 | PostToolUse for Codex/Cursor when harness exposes usage (track upstream) | eng | — |

**Exit:** P3.4 + P3.5 + P3.7 filled for **one** billing period (maintainer sign-off).

---

## Pillar 4 — Benchmark & classifier quality

| ID | Task | Owner | Done |
|----|------|-------|------|
| P4.1 | Seed dataset health checks (`DatasetHealth`, no duplicates) | eng | [x] |
| P4.2 | Expand curated set **30 → 108** tasks | eng | [x] |
| P4.3 | Graphify offline escalation in `core.Route` | eng | [x] |
| P4.4 | Grow to **200** tasks (real prompts, rubric in `benchmark/README.md`) | curation | [x] |
| P4.5 | Grow to **500+** tasks with held-out split (`benchmark/holdout.json`, never tuned against) | curation | [x] |
| P4.6 | `downshift benchmark --report` + `--gate` (tier accuracy, FRONTIER→MID rate, CI regression) | eng | [x] |
| P4.7 | Confidence intervals / bootstrap on holdout (document in benchmark README) | eng | [x] |
| P4.8 | Graphify **MCP fetcher** for KiroCrew (optional; measure FRONTIER→MID delta) | eng | [ ] |
| P4.9 | **MiniLM semantic boost on by default** (local hash; external embed falls back to hash; `DOWNSHIFT_MINILM=0` opts out — `docs/MINILM-SEMANTIC.md`) | eng | [x] |
| P4.10 | Retrain prototypes with `sentence-transformers` + measure tier accuracy delta on holdout (`benchmark/minilm-holdout.json`) | eng | [x] |

**Exit:** P4.5 + P4.6 green in CI; holdout tier accuracy documented with CI.

---

## Pillar 5 — Harness coverage (external + docs)

| ID | Task | Owner | Done |
|----|------|-------|------|
| P5.1 | `docs/HARNESS-MATRIX.md` maintained per release | docs | [x] |
| P5.2 | Antigravity catalog entries (`flash_lite` / `flash` / `pro`) | eng | [x] |
| P5.3 | Revalidate Cursor free/legacy discard quarterly | dogfood | [ ] |
| P5.4 | Revalidate Codex `multi_agent_v2` schema quarterly | dogfood | [ ] |
| P5.5 | Antigravity rewrite honored on real `invoke_subagent` session | dogfood | [ ] |
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

1. **Code done:** P4.9 default local hash boost, P4.10 neural holdout measurement, P3.3 PostToolUse command shipped.
2. **Dogfood now:** P3.4–P3.5 (usage events with tokens), P1.3 (one honored rewrite), P3.7 (fill the billing template).
3. **Community:** P2.3–P2.4 exports from other operators.
4. **Before label removal:** P1.5/P1.6, P5.3–P5.5, all five pillar exit checks signed in a release note.

---

## Release checklist (flip beta → stable)

- [ ] All five **Exit** lines above satisfied.
- [ ] README badge + Status section updated (remove beta; link here).
- [ ] CHANGELOG entry with evidence links (exports, benchmark numbers, matrix date).
- [ ] No open **P1** or **P3** blockers tagged `release-critical`.

When in doubt, keep **beta** — the label is a promise to new installers.
