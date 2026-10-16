<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Multilingual semantic intelligence — diagnosis and execution contract

harness-downshift by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/downshift

**Status:** Phase A complete; **Phase D (E5 shadow) shipped** on `main` (`831829a`+). **Phase E contract frozen** (this doc §15): opt-in encoder fuse, default off. Implementation not started; no LLM in `Route()`/`Classify()`.  
**Owner decision:** This initiative supersedes non-essential handoff work (H7, H8, H10, H11) until Phase B baseline is frozen; it does **not** pause guardrail integrity work or H12 release when approved.

---

## 1. Output contract (investigation vs implementation)

| Phase | Deliverable | Accept when |
|-------|-------------|-------------|
| **A — Diagnosis** | This document + frozen repro commands + gap map | Signed off on root causes and paused-task list |
| **B — Benchmark** | Versioned eval harness in labs; per-language metrics; equivalence sets | Baseline numbers reproduced on current `main`; train/val/test split documented |
| **C — Local prototype** | Isolated encoder + offline scorer; p50/p95, RSS, artifact size | Prototype runs without network; fails closed under hook budget in synthetic hook test |
| **D — Shadow** | Parallel observations only (`routingv2/shadow` pattern); no `Route()` change | Shadow report on M3 + legacy splits; zero production tier writes from candidate |
| **E — Integration** | Opt-in `DOWNSHIFT_E5_FUSE=1`; language-aware abstention + constrained raise; kill switch | Merge: default off + unit gates. Enable: H14 unsafe confident = 0 (fuse on and off), legacy five splits non-regression vs current `main`, labs mover ≠ MiniLM raise-only. Owner enables flag. |

**Hard rules (AGENTS.md layering):** behaviour in `internal/core` and shared packages; adapters unchanged except exposing facts; no harness-specific branches in `core`; shadow never auto-activates routing; task text not persisted in telemetry.

---

## 2. Repository state (2026-10-16)

| Area | State |
|------|--------|
| Branch | `main` @ `18abffc` (H14 + E5 shadow + BDI docs); CI green |
| Open PRs | None |
| Handoff H1–H6, H9, H13, **H14** | Done on `main` |
| H14 | **M3 unsafe confident = 0** (regex/guard; see [inventory](../2026-10-16-handoff/h14-unsafe-confident-inventory.md)) |
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
  → applyRiskFloor (lift Trivial/Simple; H14 RiskAbstain on bounded/high-stakes risk)
  → ClassifyWithSemantic (optional): semantic.MaybeAugment (MiniLM/hash centroids); monotonic raise only; never Complex without regex Complex score
  → core.Route → compareToCurrentModel → Decision (+ optional E5 shadow observation)
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
| E5 shadow | `internal/semantic/e5shadow` + `applyE5Shadow` in `Route` | `DOWNSHIFT_E5_SHADOW=1`; observation only; no tier change |
| Embedder | `internal/semantic/` (`CmdEmbedder`, `HashEmbedder`, prototypes) | `DOWNSHIFT_MINILM_EMBED`; 1s cmd timeout; hook budget 1500ms |
| Hook runner | `internal/hookctx` | Bounds external helpers |
| Shadow v2 | `internal/routingv2/shadow` | Local predictors; no prompt persistence in observations |
| Score routing | `DOWNSHIFT_SCORE_ROUTING=1` | Separate opt-in path; not semantic encoder |

**Linguistic gap (evidence-based, not hypothetical):**

1. **Tables are EN/PT-centric** — ES/FR/DE/JA parallel rows often abstain (`confident=false`) or land on Medium default; tier accuracy on M3 is **34.1%** vs authored labels (labels may be wrong until adjudication).
2. **Mixed EN + foreign risk** — Fixed on `main` (`6f11283`): mixed English helper briefs set `UnsupportedLanguage` (M3 mixed bucket **0** unsafe).
3. **Risk labelled but class MEDIUM** — Fixed (`1ca2b2c`, `a438604`): bounded delegation briefs + parallel high-stakes patterns clear confidence via `RiskAbstain` without legacy-split regression.
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
- [x] H14 failure buckets linked and **closed** on M3 (unsafe confident 0)
- [x] Reproduce M3 metrics: `go run ./tools/multilang --input …/m3-v1.json` (labs fixture)
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

- [x] **Encoder:** `intfloat/multilingual-e5-small` via **ONNX Runtime INT8** (`internal/semantic/e5shadow`); observation only on `main`.
- Env contract: [§14 Phase D env contract](#14-phase-d-env-contract) (`DOWNSHIFT_E5_SHADOW*`). **No** `Route()` tier change; telemetry field `e5_shadow`.
- [ ] Compare `would_label` / `would_lift` vs production tier on M3 + legacy splits (`downshift-labs` + `events.jsonl` aggregates) — ongoing analysis, not a routing gate.

### Phase E — Integration

Orchestrator verdict **REVISE** (2026-10-16, força total). Frozen contract: [§15](#15-phase-e-env-contract).

- **Not SHIP** of MiniLM-identical raise-only (`AugmentVector` → `would_label`). That is M4; it cannot add linguistic intelligence.
- **Not an LLM.** Encoder only: `intfloat/multilingual-e5-small` ONNX INT8.
- Env: `DOWNSHIFT_E5_FUSE` (exact `1`, default off). Orthogonal to `DOWNSHIFT_E5_SHADOW` and `DOWNSHIFT_MINILM`.
- Fuse may **abstain / lower confidence** on language mismatch; may **raise** only under MiniLM constraints. Must not lower complexity, create COMPLEX alone, or bypass Check R5–R9.
- Rollback: unset `DOWNSHIFT_E5_FUSE` → bit-identical `Route()` to current `main`.

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

**Continue in parallel:** M3 label adjudication (absolute tier accuracy); Phase B encoder bench (e5 vs MiniLM under hook-realistic latency).

---

## 9. Success criteria (numbers TBD after Phase B baseline)

Measure before setting targets:

- Unsafe confident on M3 → **0** ✓ (2026-10-16 on `18abffc`; label disagreements may still exist for non-confident rows)
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

1. **Fullstack (authorized):** implement §15 in `internal/core` + `internal/semantic` (not adapters). Flag default off.
2. **Labs:** ship E5 prototype JSON with reserved `ABSTAIN`/`FOREIGN` centroids (complexity-only store cannot abstain). Warm ONNX p95 under hook budget.
3. **Human adjudication** sample of M3 labels — still blocks interpreting 34% tier accuracy as encoder failure.
4. **Phase D analysis** continues (shadow `would_lift` vs production) — does not gate the fuse *plumbing* PR; **does** gate owner `DOWNSHIFT_E5_FUSE=1`.
5. Resume H12/H7 when owner prioritizes release/evidence over semantic enable.

---

## 12. Files touched by this initiative (expected)

| Area | Likely touch |
|------|----------------|
| `internal/semantic/` (`e5fuse` + `e5shadow`) | Encoder backends, ONNX, Phase E fuse (new), shadow |
| `internal/core/semantic_fuse.go` | MiniLM + E5 fuse; `lang_guard.go` stays language-regex |
| `internal/routingv2/shadow/` | Observations |
| `docs/reference/complexity-weights.md`, harness-matrix | Behaviour docs |
| `downshift-labs/eval/multilang/` | Benchmarks, manifests (not embedded prompts in public repo) |

No adapter routing logic; no catalog changes for semantic work.

---

## 13. Risks still open

- M3 labels may over-state frontier expectation → false “unsafe” counts.
- Embedding may solve **variant** understanding but not **tier** if fuse stays MiniLM raise-only (Phase E forbids shipping that path; abstain is the linguistic lever).
- Hook budget may require **always-warm** sidecar — operational complexity.
- Overfitting M3 while breaking legacy splits (forbidden by project rules).

---

## 14. Phase D env contract

harness-downshift by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/downshift

**Owner decision (frozen for Phase D):** observe **multilingual E5-small** embeddings through an **ONNX Runtime INT8** helper. Shadow is **opt-in** and **fail-open**. Production **`Route()`**, **`Classify()`**, **`ClassifyWithSemantic()`**, hook rewrites, and **`DOWNSHIFT_MINILM`** behaviour stay unchanged until Phase E.

Implementation surface: `internal/semantic/e5shadow`, wired from `core.Route` via `applyE5Shadow` (telemetry only).

### 14.1 Environment variables

| Variable | Default | When set | Effect |
|----------|---------|----------|--------|
| `DOWNSHIFT_E5_SHADOW` | off (unset or any value except `1`) | `1` | Run the E5 shadow path after production classification. Any other spelling (`true`, `yes`, `on`) is treated as **off**. |
| `DOWNSHIFT_E5_SHADOW_CMD` | empty | Non-empty shell command | **Required** when shadow is on. Invokes the Phase C ONNX embed helper: read task text on **stdin**, write one JSON array of floats on **stdout** (same wire shape as `DOWNSHIFT_MINILM_EMBED`). Expected backend: **intfloat/multilingual-e5-small** exported to ONNX, executed with **ONNX Runtime INT8** on CPU. |
| `DOWNSHIFT_E5_PROTOTYPES` | empty | Absolute path to a regular file | **Required** when shadow is on. JSON **`semantic.PrototypeStore`** (development centroids in the **same embedding space** as the helper). Max **256 KiB**; symlinks, FIFOs, and oversize files are rejected. |

**Orthogonality:** `DOWNSHIFT_SHADOW_WEIGHTS` (softmax classifier v2) and `DOWNSHIFT_SCORE_ROUTING` are independent. E5 shadow does not load candidate weights or change score routing.

### 14.2 Fail-open behaviour

| Condition | Hook / routing | Shadow / telemetry |
|-----------|----------------|-------------------|
| `DOWNSHIFT_E5_SHADOW` not exactly `1` | Unchanged | No E5 work; `e5_shadow` omitted from `events.jsonl` |
| Shadow on, `DOWNSHIFT_E5_SHADOW_CMD` empty | Unchanged | `error_code`: `missing_cmd` |
| Shadow on, prototypes missing / invalid / too large | Unchanged | `error_code`: `load_prototypes` |
| Embed helper missing, slow, non-JSON, or empty vector | Unchanged | `error_code`: `embed_error` (helper stderr is **not** persisted) |
| Shadow on, full success | Unchanged tier, complexity, confidence, model rewrite | `e5_shadow` populated (see §14.3) |

Boundaries match other hook helpers: each embed call is capped at **1 s** and the hook’s **1500 ms** global budget (`hookctx`). A hung helper is killed; routing never waits indefinitely. Unlike production MiniLM, the E5 path **does not** fall back to the hash embedder—errors stay in `error_code` only.

**Invariant (Phase D gate):** with shadow enabled vs disabled, `Route()` must produce identical `Tier`, `Complexity`, and `Confident` for the same prompt (`TestE5Shadow_OnRecordsWithoutChangingRoute`, `TestE5Shadow_EnabledFailOpenDoesNotChangeRoute`).

### 14.3 Telemetry (`events.jsonl`)

Decision events may include a prompt-free object **`e5_shadow`** (see `telemetry.E5ShadowRecord`). Allowed fields:

| Field | Meaning |
|-------|---------|
| `model_id` | Allowlisted artifact id (SHA-256 of prototypes file, optionally prefixed with `store.model`, e.g. `e5:intfloat/multilingual-e5-small:…`) |
| `nearest_label` | Nearest centroid label in prototype space |
| `similarity` | Cosine similarity to that centroid (rounded for JSON) |
| `would_label` | Hypothetical complexity after the **same monotonic fuse** as production (`semantic.AugmentWith`), not an executed tier |
| `would_lift` | Whether that fuse would have raised complexity vs the production snapshot |
| `regex_label` | Production complexity string at observation time (after regex, risk, MiniLM fuse, graph, NB—field name is historical) |
| `regex_confident` | Production `Confident` at observation time |
| `latency_ms` | Wall time for the shadow path on this hook |
| `error_code` | `missing_cmd`, `untrusted_cmd`, `load_prototypes`, `embed_error`, or `observe_panic` when observation failed |

**Never persisted:** raw prompt, stdin/stderr, embedding vectors, file paths from env, or provider errors that might echo task text. No new **`SEMANTIC_BOOST`** or reason-code changes from E5 in Phase D.

### 14.4 Operator example (labs / local only)

```bash
export DOWNSHIFT_E5_SHADOW=1
export DOWNSHIFT_E5_SHADOW_CMD='/absolute/path/downshift-e5-onnx-int8'   # Phase C artifact
export DOWNSHIFT_E5_PROTOTYPES='/absolute/path/e5-centroids.json'       # labs-built PrototypeStore

# Run harness hooks as usual; compare aggregates offline (labs / stats), not live tier changes.
unset DOWNSHIFT_E5_SHADOW   # disable — zero shadow overhead
```

Centroids and ONNX artifacts are built and versioned in **downshift-labs**; do not commit prompt-bearing fixtures into the public repo.

### 14.5 Phase D acceptance criteria

1. **Routing invariant:** `DOWNSHIFT_E5_SHADOW=1` with valid cmd + prototypes does not change `Tier`, `Complexity`, `Confident`, or hook model rewrite vs shadow off (automated: `internal/core/e5_shadow_test.go`).
2. **Privacy:** No decision or training JSONL record contains prompt text, full embedding vectors, or helper stderr; only §14.3 fields appear under `e5_shadow`.
3. **Fail-open:** Every misconfiguration and embed failure yields an `error_code` and leaves production routing identical (automated: `internal/semantic/e5shadow/e5shadow_test.go`).
4. **Eval report:** Labs (or `downshift-labs`) produces a shadow comparison on **M3** and **legacy five splits**: distribution of `would_lift`, disagreement rate vs production complexity, and p50/p95 `latency_ms` under hook-realistic warm ONNX—without promoting Phase E.
5. **Promotion gate:** Do **not** wire shadow `would_label` / `AugmentVector` into `ClassifyWithSemantic` (that is M4). Phase E plumbing is authorized as a **distinct** fuse (§15). Owner **enable** (`DOWNSHIFT_E5_FUSE=1`) still waits on item 4 plus §15.6 gates.

---

## 15. Phase E env contract (opt-in encoder fuse)

harness-downshift by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/downshift

**Owner decision (frozen 2026-10-16):** Phase E is an **opt-in** fuse of **intfloat/multilingual-e5-small** (ONNX Runtime INT8) into `ClassifyWithSemantic`. It is **not** a chat/completion LLM, **not** a second `Route()`, and **not** MiniLM raise-only. Default **off**. Fail-open. Kill switch = unset.

Orchestrator verdict: **REVISE** (reject SHIP of `would_label`; reject waiting for an LLM ceiling). Fallback if the PR exceeds one increment: **SPLIT** E1 abstain-only then E2 constrained raise, same env flag.

### 15.1 Environment variables

| Variable | Default | When set | Effect |
|----------|---------|----------|--------|
| `DOWNSHIFT_E5_FUSE` | off (unset or any value except `1`) | `1` | Run the E5 fuse **after** regex `Classify` + optional MiniLM `MaybeAugment`, **before** graphify / NB / `Check`. Any other spelling (`true`, `yes`, `on`) is **off**. |
| `DOWNSHIFT_E5_CMD` | empty | Non-empty shell command | **Required** when fuse is on. Same wire as `DOWNSHIFT_E5_SHADOW_CMD` / `DOWNSHIFT_MINILM_EMBED`: task text on **stdin**, one JSON float array on **stdout**. Backend: **intfloat/multilingual-e5-small** ONNX INT8 CPU. Must pass `e5shadow.CmdTrusted`. |
| `DOWNSHIFT_E5_PROTOTYPES` | empty | Absolute path to a regular file | **Shared with Phase D.** **Required** when fuse is on. JSON `semantic.PrototypeStore`. Max **256 KiB**; symlinks, FIFOs, oversize rejected. Enable-gate centroids **must** include reserved labels `ABSTAIN` and/or `FOREIGN` (complexity-only maps cannot abstain). |

Do **not** introduce `DOWNSHIFT_SEMANTIC_*`. MiniLM already owns that role. Do **not** retune thresholds via env (`MinSimilarity` stays in code).

### 15.2 Orthogonality

| `E5_FUSE` | `E5_SHADOW` | `MINILM` | Behaviour |
|-----------|-------------|----------|-----------|
| off | off | on (default) | Current `main` |
| off | `1` | any | Phase D: observation only; `Route()` identity |
| `1` | off | on or `0` | Fuse **success** skips MiniLM `MaybeAugment` (one semantic slot). Fuse **fail-open** restores MiniLM+regex (current `main`). |
| `1` | `1` | any | Fuse applies **and** shadow may record. **At most one embed** per `Route()` (reuse the vector; do not pay 2× 1 s) |
| off | off | `0` | Regex + guards only (existing tests) |

`DOWNSHIFT_SHADOW_WEIGHTS` and `DOWNSHIFT_SCORE_ROUTING` stay independent. Fuse does not load v2 candidate weights.

MiniLM polarity stays **opt-out** (`0`/`false`/`off`/`no`) and is **unchanged whenever `E5_FUSE` is unset**. E5 fuse polarity is **opt-in exact `1`**, like shadow. When fuse is on and the helper succeeds, **skip** MiniLM hash **and** `DOWNSHIFT_MINILM_EMBED` for that hook (AppSec: two helpers vs 1500 ms lets MiniLM hash-fallback **change** `Route` while E5 eats the budget). Do **not** rewrite the MiniLM env; fail-open must still run MiniLM.

Do **not** silently reuse `DOWNSHIFT_E5_SHADOW_CMD` when `DOWNSHIFT_E5_CMD` is empty (`missing_cmd`, fail-open). Operators who want both copy the same absolute path into both variables.

### 15.3 Fail-open and kill switch

| Condition | `Route()` / classify | Telemetry |
|-----------|----------------------|-----------|
| `DOWNSHIFT_E5_FUSE` not exactly `1` | Unchanged vs current `main` | No fuse work; no `e5_fuse` object |
| Fuse on, `DOWNSHIFT_E5_CMD` empty | Unchanged | `error_code`: `missing_cmd` |
| Fuse on, cmd not `CmdTrusted` | Unchanged | `error_code`: `untrusted_cmd` |
| Fuse on, prototypes missing / invalid / too large | Unchanged | `error_code`: `load_prototypes` |
| Helper missing, slow (>1 s or hook 1500 ms), non-JSON, empty vector | Unchanged | `error_code`: `embed_error` (stderr **not** persisted) |
| Fuse on, full success | Complexity/confidence only as §15.4 allows | `e5_fuse` populated (prompt-free) |

Unlike MiniLM, E5 fuse **does not** fall back to the hash embedder (spaces must not mix). Hung helper is killed; routing never waits indefinitely.

**Kill switch:** `unset DOWNSHIFT_E5_FUSE` (or any value except `1`) → `Tier`, `Complexity`, `Confident`, `SafeVerdict`, and hook rewrite **identical** to current `main` for the same prompt (`TestE5Fuse_OffMatchesCurrentRoute`).

### 15.4 Fuse policy (what E5 may vs must not)

Insertion point: `classifyWithSemantic` after MiniLM `MaybeAugment`, still inside `classifyTask`, **before** graphify, NB, `compareToCurrentModel`, and **`Check`**. Adapters unchanged.

**MAY**

1. **Abstain / lower confidence** when nearest centroid label is `ABSTAIN` or `FOREIGN` and cosine ≥ `semantic.MinSimilarity` (0.45). Set `Confident=false`. Optionally set `E5Abstain=true` for telemetry. **Do not** change `Complexity`. **Do not** set `UnsupportedLanguage` (that flag stays `lang_guard`). **Do not** set `RiskAbstain` (that flag stays `risk.go`).
2. **Raise complexity** only if **all** of: not `NoSignal`; regex `Confident==false`; nearest label is TRIVIAL\|SIMPLE\|MEDIUM\|COMPLEX; `labelRank(nearest) > labelRank(base)`; cosine ≥ 0.45; **not** (`nearest==COMPLEX` && `base.Scores[Complex]==0`). After a raise: `Confident=false`, `SemanticLift` if rank increased, then `applyRiskFloor`. Reason code **`E5_BOOST`** (do not reuse `SEMANTIC_BOOST`).
3. Log abstention as **`E5_ABSTAIN`**.

**MUST NOT**

- Call a chat/completion LLM from `Route()` / `Classify()` / fuse / shadow.
- Lower `Complexity` or emit a sole-source downshift (risk-labelled M3 stays on regex + R5).
- Create COMPLEX with no regex Complex score (same MiniLM guard).
- Boost a `NoSignal` prompt.
- Set `Confident=true`.
- Enter `deterministicLift` (`RiskFloor` / `RiskAbstain` / `GraphEscalated` / `NBUpshift` / `PolicyFloor` only). An E5 raise or abstain **must not** exempt R7/R8/R9.
- Bypass or rewrite `Check` R5–R9 (or R1/R2/R4). Fuse runs **before** Check; guardrails still win.
- Persist prompt text, stdin, stderr, embedding vectors, or env file paths.
- Auto-enable (no default-on, no “shadow was healthy so fuse on”).
- Put policy in adapters or hardcode model IDs in Go (artifact id via prototypes `model` + SHA-256, as Phase D).

### 15.5 Why M4 was null, and what would move M3

Evidence: `downshift-labs/eval/multilang/m4/metrics.json` (tier accuracy **34.1% = 34.1%**, coverage **25.3% = 25.3%**, unsafe confident **10 = 10**); `helper-probe.json` (CmdEmbedder **1103 ms**, `success: false` at 1 s); candidate p95 **2.34 ms** (not neural). Post-H14 on `main`, unsafe confident is **0** via regex/`lang_guard`/`RiskAbstain`, not MiniLM.

Ranked causes:

1. **Raise-only fuse cannot fix the H14 buckets.** Mixed EN+foreign risk and bounded high-stakes briefs needed **abstention**, not a higher class. `AugmentVector` never clears confidence on a language mismatch.
2. **The encoder likely never fused.** Cold helper missed the 1 s budget; MiniLM **hash-falls-back**. Candidate p95 (~2 ms) is not E5/L12. Identical metrics ⇒ **zero surviving lifts** on 170 rows.
3. **Complexity-only centroids have no language axis.** `PrototypeStore.Centroids` is `label → vector`. `labelRank` maps unknown labels to SIMPLE (1), so a reserved abstain class must **not** go through `AugmentVector`.
4. **M3 34.1% is not an encoder score until labels are adjudicated.** Many “misses” are authored `expected_tier` vs EN/PT regex. Abstention will **not** raise tier accuracy (class unchanged). Constrained raise **can** move accuracy only on unconfident rows with regex Complex evidence; it **cannot** invent COMPLEX.

**What would make E5 actually move M3 (linguistic intelligence, not a second classifier):**

| Lever | Moves | Does not move |
|-------|--------|----------------|
| Language-aware **abstain** (`FOREIGN`/`ABSTAIN`) | `confident_coverage` down on lang-guard misses; keeps **unsafe confident = 0**; adds intelligence H14 regex cannot see (slang / low-cue foreign) | `tier_accuracy` (class unchanged) |
| Constrained **raise** (MiniLM rules, E5 space, warm ONNX) | `tier_accuracy` on unconfident multilingual rows where regex under-ranked **and** Complex score already > 0 | H14 mixed/risk buckets; NoSignal; COMPLEX-from-nothing |
| Copy `would_label` into production | Nothing new (M4) | Anything |

Enable-bar for “better”: labs slice where `lang_guard` did **not** fire, MiniLM raise-only did **not** fire, and E5 **did** abstain (count > 0) **without** increasing unsafe confident. Do not sell 34.1% → N% as the Phase E KPI until adjudication.

### 15.6 Success gates

**Merge (flag default off, CI):**

1. `DOWNSHIFT_E5_FUSE` unset / not `1` → identical `Route()` to `Classify`+MiniLM current behaviour (kill switch).
2. Fuse on + each fail-open condition in §15.3 → identical `Tier`/`Complexity`/`Confident` to fuse off.
3. Policy tests: abstain clears confidence and does not change class; raise cannot create COMPLEX when `Scores[Complex]==0`; `NoSignal` untouched; `Check` still holds R5–R9 on fused decisions (`E5Abstain` / `E5_BOOST` ∉ `deterministicLift`).
4. Shadow orthogonality: `E5_FUSE` unset + `E5_SHADOW=1` still observation-only (existing `e5_shadow_test.go`).
5. MiniLM orthogonality: `E5_FUSE` unset + `DOWNSHIFT_MINILM=0` unchanged.
6. H14 acceptance tests pass with fuse **mocked on** (unsafe confident writes still 0).
7. Privacy: `e5_fuse` fields ⊆ `{model_id, nearest_label, similarity, action (abstain\|raise\|none), latency_ms, error_code}`. No prompt/vector/stderr/env paths.
8. Default-off path keeps `TestBenchmarkSplitsMatchClsAfterBaseline` green (`cls-after.jsonl`).

**Enable (owner sets `DOWNSHIFT_E5_FUSE=1`, labs report):**

1. **H14:** M3 unsafe confident = **0** with fuse on **and** off (`go run ./tools/multilang --input …/m3-v1.json` plus H14 inventory).
2. **Legacy five splits** vs **current `main` fuse-off** (not vs pre-H14 M4 JSON): `tasks`, `holdout`, `fresh`, `heldout2`, `blind-vitrine`. BLOCK if any split increases `unsafe_confident` or `unsafe_raw`, or drops `tier_accuracy` (M4 `tasks` 73.5% → 73.0% is the forbidden pattern).
3. **Kill switch** re-verified on the enable candidate binary.
4. Warm ONNX p95 embed+classify inside hook budget leave-headroom (Phase C: ≤ **500 ms** of the 1500 ms budget). Cold 6.5 s Python is BLOCK for enable.
5. Prototypes include `ABSTAIN`/`FOREIGN`; mover slice in §15.5 is non-zero **or** owner explicitly accepts raise-only (not recommended).
6. Dual `FUSE=1`+`SHADOW=1` does not double-embed or blow the 1500 ms budget.

Any regression in a critical language or risk class **blocks enable**, not necessarily the default-off merge.

### 15.7 Non-goals

- LLM (chat/completion) in `Route()` / `Classify()` / fuse / `Check`.
- Adapter routing policy or harness branches in `core`.
- Auto Phase E default-on; shadow health does not promote the flag.
- Wiring Phase D `would_label` into production.
- Changing MiniLM default, hash fallback, or `DOWNSHIFT_MINILM` polarity.
- Catalog ID hardcoding; prompt persistence; Phase F fine-tune.
- Using E5 to sole-source downshift risk-labelled rows.
- In-process ONNX Runtime in the hook binary (new native surface). Phase E keeps the **out-of-process** helper, same as Phase D.

### 15.8 Implementation notes (fullstack; not done in this turn)

- New `internal/semantic/e5fuse` (or equivalent) **plus** a few lines in `internal/core/semantic_fuse.go`. Reuse `e5shadow.CmdTrusted` and prototype load rules. **Before the helper can change `Route`:** `CmdTrusted` uses **`Lstat`** (not `Stat` — prototypes already reject symlinks) and rejects glued `-c`/`-e` (today only ` -c ` with spaces is banned). Do not put fuse logic in adapters. Out-of-process helper only.
- Reserved labels `ABSTAIN`/`FOREIGN` must short-circuit **before** `AugmentVector` / `labelRank` (unknown labels currently rank as SIMPLE).
- Tests at the shared layer (`classifyWithSemantic` / `Route` / `Check`). Table-driven cases in `semantic_fuse_test.go` (or adjacent `e5_fuse_test.go`).
- Copyright header on every new `.go` file. No production Go in this orchestrator turn.

### 15.9 Operator example (labs / local only)

```bash
export DOWNSHIFT_E5_FUSE=1
export DOWNSHIFT_E5_CMD='/absolute/path/downshift-e5-onnx-int8'
export DOWNSHIFT_E5_PROTOTYPES='/absolute/path/e5-centroids.json'   # must include ABSTAIN/FOREIGN

# Optional: keep shadow observation without a second embed
export DOWNSHIFT_E5_SHADOW=1
export DOWNSHIFT_E5_SHADOW_CMD="$DOWNSHIFT_E5_CMD"

unset DOWNSHIFT_E5_FUSE   # kill switch — current main behaviour
```

