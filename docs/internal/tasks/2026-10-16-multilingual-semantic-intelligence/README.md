<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Multilingual semantic intelligence — diagnosis and execution contract

harness-downshift by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/downshift

**Status:** Phase A (diagnosis). No runtime integration until Phase D/E gates pass.  
**Owner decision:** This initiative supersedes non-essential handoff work (H7, H8, H10, H11) until Phase B baseline is frozen; it does **not** pause H14 safety fixes, H12 release when approved, or guardrail integrity work.

---

## 1. Output contract (investigation vs implementation)

| Phase | Deliverable | Accept when |
|-------|-------------|-------------|
| **A — Diagnosis** | This document + frozen repro commands + gap map | Signed off on root causes and paused-task list |
| **B — Benchmark** | Versioned eval harness in labs; per-language metrics; equivalence sets | Baseline numbers reproduced on current `main`; train/val/test split documented |
| **C — Local prototype** | Isolated encoder + offline scorer; p50/p95, RSS, artifact size | Prototype runs without network; fails closed under hook budget in synthetic hook test |
| **D — Shadow** | Parallel observations only (`routingv2/shadow` pattern); no `Route()` change | Shadow report on M3 + legacy splits; zero production tier writes from candidate |
| **E — Integration** | Opt-in env flag; fuses with regex + guardrails; kill switch | M3: unsafe confident → 0 without legacy-split regression; owner enables flag |

**Hard rules (AGENTS.md layering):** behaviour in `internal/core` and shared packages; adapters unchanged except exposing facts; no harness-specific branches in `core`; shadow never auto-activates routing; task text not persisted in telemetry.

---

## 2. Repository state (2026-10-16)

| Area | State |
|------|--------|
| Branch | `main` @ `794b7e0`; clean except untracked `.claude/launch.json` |
| Open PRs | None |
| Remote branches | `fix/latency-test-skip-race`, `fix/session-latency-headroom`, stale `feat/h6-upshift-needed-tier` (merged via #134) |
| Handoff H1–H6, H9, H13 | Done on `main` |
| Handoff H14 | Inventory on `main` (`h14-unsafe-confident-inventory.md`); fixes not started |
| Handoff H7, H8, H10, H11, H12 | **Paused** for this initiative (see §8) |
| Multilang M1–M2 | Done (language guard + PT risk terms) |
| M3 | 170-row fixture frozen in labs; **human label adjudication pending** |
| M4 | **Rejected** for adoption (see §5) |
| M5 | Blocked (billing / extra usage) |
| M6 | Partially addressed by H1/H5 delegation rules |
| M7 | Docs partial; finish after H14 / semantic policy stable |

---

## 3. Current routing and classification flow

```
Prompt
  → Classify (regex signals, bounded-brief rules, 32 KiB scoring cap; full text for risk + lang)
  → unsupportedLanguage? → Confident=false (lang guard)
  → applyRiskFloor
  → ClassifyWithSemantic (optional): semantic.MaybeAugment (MiniLM/hash centroids); monotonic raise only; never Complex without regex Complex score
  → core.Route → compareToCurrentModel → Decision
  → core.Check (R1–R9, risk downshift, unknown-model holds)
  → Plan / PlanForSession (session + catalog; guardrails win)
```

**Existing semantic stack (do not replace blindly):**

| Piece | Location | Role |
|-------|----------|------|
| Regex classifier | `internal/core/classifier.go`, `signals.go` | Primary tier/complexity; EN/PT-oriented |
| Language guard | `internal/core/lang_guard.go` | Abstention for unsupported/mixed script; not full variant detection |
| Risk floor | `internal/core/risk.go` | Lifts off small; categories EN + PT extensions |
| Semantic fuse | `internal/core/semantic_fuse.go` | Optional boost via `ClassifyWithSemantic` |
| Embedder | `internal/semantic/` (`CmdEmbedder`, `HashEmbedder`, prototypes) | `DOWNSHIFT_MINILM_EMBED`; 1s cmd timeout; hook budget 1500ms |
| Hook runner | `internal/hookctx` | Bounds external helpers |
| Shadow v2 | `internal/routingv2/shadow` | Local predictors; no prompt persistence in observations |
| Score routing | `DOWNSHIFT_SCORE_ROUTING=1` | Separate opt-in path; not semantic encoder |

**Linguistic gap (evidence-based, not hypothetical):**

1. **Tables are EN/PT-centric** — ES/FR/DE/JA parallel rows often abstain (`confident=false`) or land on Medium default; tier accuracy on M3 is **34.1%** vs authored labels (labels may be wrong until adjudication).
2. **Mixed EN + foreign risk** — Three M3 cases (`mixed-01`, `11`, `27`) are **confident TRIVIAL → small** while labels expect frontier; `unsupported_language=false` so guard does not fire ([H14 inventory](../2026-10-16-handoff/h14-unsafe-confident-inventory.md)).
3. **Risk labelled but class MEDIUM** — Five delegation + two parallel EN/PT-BR cases: confident mid below labelled frontier floor (regex/classifier, not embedding).
4. **Informal / slang / regional EN** — Not systematically covered; no dedicated eval slice in repo (only M3 + legacy splits).
5. **M4 embedding path** — Did not improve M3 metrics; cold helper **failed** hook-style timeout (see §5).

Semantic intelligence is therefore **not greenfield**: extend or replace the **fuse + embedder** path and/or **lang/risk signals**, with shadow proof, rather than a parallel classifier silo.

---

## 4. Proposed architecture (mapped to Downshift)

Conceptual layers from the initiative brief, mapped to existing or planned surfaces:

| Layer | Downshift home | Notes |
|-------|----------------|-------|
| Semantic encoder | `internal/semantic` (+ optional `downshift serve` warm worker) | Pluggable `Embedder`; ONNX candidate behind same interface |
| Task / complexity signals | Still `Classify` + optional fuse | Encoder outputs **features**, not final tier |
| Language / variant | Extend `lang_guard` + shadow metadata | Variant detection is **abstention/confidence**, not tier by itself |
| Risk / uncertainty | `Check`, `Confident`, `UnsupportedLanguage`, shadow margin | Semantic confidence must not bypass R5/R6/R7/R9 |
| Routing policy | Unchanged `Route` + `Plan` | Combine via existing fuse pattern or shadow-only until E |

**Anti-patterns to avoid:** second `Route()`; per-harness semantic logic; auto-training from production; storing prompt text in shadow JSONL.

---

## 5. M3 / M4 experiment — why no improvement (evidence)

Sources: `downshift-labs/eval/multilang/m4/metrics.json`, `evaluation-status.md`, `m4/helper-probe.json`.

| Metric (M3, n=170) | Regex baseline | M4 candidate (MiniLM L12 centroids) |
|--------------------|----------------|-------------------------------------|
| Tier accuracy | 34.1% | 34.1% (identical) |
| Confident coverage | 25.3% | 25.3% |
| Unsafe confident | 10 | 10 |
| Unsafe raw | 74 | 74 |
| Classifier p95 | ~1.26 ms | ~2.34 ms |

Legacy five splits: candidate **equal or slightly worse** on `tasks` (73.0% vs 73.5% tier accuracy); one extra raw below-floor on `tasks`.

**Interpretation (hypotheses ranked by evidence):**

1. **Label noise / adjudication** — Low absolute accuracy on M3; unsafe counts depend on authored `expected_tier`; not validated by humans yet.
2. **Wrong integration point** — M4 used `ClassifyWithSemantic` centroid augmentation, not a multilingual **language-aware abstention** or **risk-visible** fuse; same 10 failures → embedding did not fix the known H14 buckets.
3. **Runtime infeasibility as wired** — Existing `CmdEmbedder` one-shot: **1103 ms**, success **false** at 1s timeout (`helper-probe.json`); cold Python load **~6.5 s** (evaluation-status). Hook budget **1500 ms** total for all helpers.
4. **Not a representation A/B** — Same metric on baseline and candidate implies centroids did not change decisions on this fixture (or changed and reverted via fuse rules).

**Conclusion:** M4 rejected correctly for **adoption**; it still valid as **baseline**. Next work must fix **identified failure modes** (H14 buckets) *and* measure encoder candidates under **hook-realistic** latency (warm service or ONNX in-process), not warm Python-only timings.

---

## 6. Model candidates (Phase B — not chosen yet)

| Candidate | Role | Initial hypothesis |
|-----------|------|--------------------|
| `intfloat/multilingual-e5-small` | Primary Phase B bench | Stronger multilingual retrieval; size/latency TBD on Mac CPU |
| `sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2` | Repro baseline (M4) | Known null result on M3; compare fairly with e5 under same fuse |
| Compact newer multilingual | TBD after literature + license scan | Only if beats e5 on equivalence sets **and** p95 within budget |

Evaluation dimensions: equivalence consistency (formal/informal/slang PT/EN), M3 per-language macro-F1, **unsafe confident**, legacy-split non-regression, artifact MB, cold/warm p50/p95, ONNX vs PyTorch.

**Not in scope for Phase B:** cloud APIs during hook path; auto fine-tune before Phase B scores.

---

## 7. Phased plan (aligned to brief §5)

### Phase A — Diagnosis (this doc)

- [x] Flow map and existing components
- [x] M3/M4 evidence summary
- [x] H14 failure buckets linked
- [ ] Reproduce M3 metrics from `main` via documented command (labs runner or `tools/multilang`)
- [ ] Add **equivalence set spec** (synthetic + anonymized real) in labs only

### Phase B — Benchmark

- Build stratified sets: EN formal/informal/slang, PT variants, ES/FR/DE/JA parallels, mixed, “simple posh / complex terse”.
- Leakage control: group by semantic equivalence ID across languages.
- Metrics: per-language macro-F1, confusion by tier, unsafe confident/raw, equivalence consistency rate.

### Phase C — Local prototype

- Implement `Embedder` backend (ONNX Runtime CPU int8 trial) behind existing interface.
- Measure init once vs amortized; optional `downshift serve` warm path (already used elsewhere in project docs).
- Gate: p95 embed + classify ≤ **500 ms** of hook budget leave headroom for graphify/other helpers.

### Phase D — Shadow

- Register candidate as `routingv2/shadow.Predictor` or extend semantic shadow logging (no prompt text; hash + id only).
- Compare tier distributions vs production `Classify` on live hooks (opt-in env).

### Phase E — Integration

- Env: `DOWNSHIFT_SEMANTIC_*` (names TBD); default off.
- Fuse rules: semantic may **lower confidence** or **abstain** on language mismatch; may **not** sole-source downshift on risk-labelled M3 cases until proven.
- Rollback: unset env → current behaviour.

---

## 8. Paused handoff tasks (how to resume)

| Task | Pause reason | Resume when |
|------|--------------|-------------|
| H7 README screenshots / mech-rename runs | Cost; not linguistic core | After beta.15 or explicit budget |
| H8 outcome 40-task Haiku run | ~$5–6 | Owner approval |
| H10 local PATH / agents.jsonl redaction | Hygiene | Anytime; 15 min |
| H11 harness-core cleanup | Different repo | Parallel OK |
| H12 beta.15 tag | Release gate | Owner OK; CHANGELOG ready |
| H6 part 2 risky write unknown baseline | Guardrail contract change | After H14 + semantic policy |
| M5 LLM reference ceiling | Billing | Subscription-only run authorized |

**Continue in parallel (security / correctness):** H14 regex/guard fixes (10 unsafe confident); M3 label adjudication.

---

## 9. Success criteria (numbers TBD after Phase B baseline)

Measure before setting targets:

- Unsafe confident on M3 → **0** (or documented label disagreement)
- Equivalence consistency ≥ **TBD** % across language variants at same intended tier
- No increase in unsafe raw on legacy splits
- p95 hook classify path ≤ **TBD** ms with semantic enabled
- No regression in confident coverage without safety trade study

Regression worse in **any** critical language or risk class blocks Phase E.

---

## 10. Fine-tuning (Phase F, conditional)

Proceed only if Phase B shows representation limit (e.g. e5 fixes equivalence but not tier mapping) **and** labels adjudicated.

- Versioned dataset in labs; no production log ingestion without review.
- Compare fine-tuned vs base on held-out equivalence groups.

---

## 11. Next actions (priority order)

1. **Human adjudication** sample of M3 labels (owner/labs) — blocks interpreting 34% accuracy.
2. **H14 quick wins** on regex/guard (mixed-01 bucket, delegation risk floor) — may remove 10 unsafe without embedding; re-run M3.
3. **Phase B harness** in `downshift-labs`: e5-small vs MiniLM, same fuse contract, hook-realistic latency.
4. **Phase C prototype** ONNX embedder behind `semantic.Embedder`.
5. **Phase D shadow** wiring; no production tier change.
6. Resume H12/H7 when owner prioritizes release/evidence over semantic bench.

---

## 12. Files touched by this initiative (expected)

| Area | Likely touch |
|------|----------------|
| `internal/semantic/` | Encoder backends, ONNX, benchmarks |
| `internal/core/semantic_fuse.go`, `lang_guard.go` | Fuse and abstention policy |
| `internal/routingv2/shadow/` | Observations |
| `docs/reference/complexity-weights.md`, harness-matrix | Behaviour docs |
| `downshift-labs/eval/multilang/` | Benchmarks, manifests (not embedded prompts in public repo) |

No adapter routing logic; no catalog changes for semantic work.

---

## 13. Risks still open

- M3 labels may over-state frontier expectation → false “unsafe” counts.
- Embedding may solve **variant** understanding but not **tier** if fuse stays monotonic raise-only.
- Hook budget may require **always-warm** sidecar — operational complexity.
- Overfitting M3 while breaking legacy splits (forbidden by project rules).
