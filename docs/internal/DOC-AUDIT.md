# Documentation audit — use vs inside vs competitive exposure

Companion to [CONTENT-CLASSIFICATION.md](../CONTENT-CLASSIFICATION.md) (A/B/C/D).
**Audience:** maintainers. Not linked from the public doc hero.

**Legend**

| Tag | Meaning |
|-----|---------|
| **USE** | How to install, configure, integrate, trust |
| **INSIDE** | How the router works internally (implementation / policy detail) |
| **MOAT** | Teaches competitive differentiation beyond what adopters need — review trim or move |
| **OK-B** | Inside detail appropriate for **B** (contributors / reproducible OSS) |

---

## Summary

| Area | Dominant mode | MOAT concern |
|------|---------------|--------------|
| `README.md` | Mixed USE + long **INSIDE** | **High** — single file teaches full algorithm |
| `docs/` (indexed) | USE + B | **Medium** — design + generalization docs |
| `docs/internal/` | Maintainer / archive | Expected **C** (still public git) |
| `docs/pt/` | USE product + **INSIDE** engineering blocks | **Medium** in `03-marcha`, `04–08` |
| ADRs | No `docs/adr/` — use `pt/09-decisoes.md`, `DECISION-INTELLIGENCE.md` | Low |
| Diagrams | USE (positioning) | Low |
| `benchmark/` + `design/` | **OK-B** / eval honesty | **Medium** — detailed numbers in `router-generalization.md` |
| `ROADMAP.md` | USE direction | Low |
| Planning / TODO | `docs/internal/LAUNCH-WAVES.md` | **C** — not moat doc, ops |
| `examples/` | **USE** | None |

**Formal ADRs:** none. Decision-style docs: `docs/pt/09-decisoes.md`, `docs/DECISION-INTELLIGENCE.md`.

---

## README.md (~1.2k lines)

| Section | Mode | MOAT? | Note |
|---------|------|-------|------|
| Hero, quickstart, install per harness | USE | No | Keep |
| Testing locally, plan compatibility | USE | No | Keep |
| Cost problem / session savings | USE | No | Adoption story |
| Tier model, what it does | USE | Borderline | High-level OK |
| **How the deterministic switch works** | **INSIDE** | **Yes** | Step-by-step signals, scores, example weights — **trim or link to `contrib/classifier.md`** |
| **Capability Router v2** (in README) | INSIDE | **Yes** | Duplicate of `docs/CAPABILITY-ROUTER-V2.md` — shorten to 1 paragraph + link |
| Architecture (in README) | INSIDE | Mild | Prefer single canonical `docs/ARCHITECTURE.md` |
| **Classifier benchmark** / outcome metrics | INSIDE + **OK-B** | Mild | Numbers for trust; avoid duplicating `design/router-generalization.md` |
| Design principles, financial impact | USE + narrative | Low | |
| dsmon, troubleshooting, contributing | USE | No | |

**Recommendation:** Treat README as **USE funnel**; move deep **INSIDE** blocks to `docs/contrib/classifier.md` + `docs/ARCHITECTURE.md` (follow-up PR).

---

## `docs/` — public index

### A — Adoption (USE)

| Document | MOAT |
|----------|------|
| `ARCHITECTURE.md` | No — boundary diagram, packages (appropriate high-level **INSIDE**) |
| `WHEN-TO-USE.md`, `CONFIG.md`, `session-models.md` | No |
| `HARNESS-MATRIX.md`, `BETA-EXIT.md`, `stats-export.md` | No |
| `MINILM-SEMANTIC.md` | Mild — paths + flags; not full scoring recipe |
| `evidence/*` | No — honor protocol |
| `pt/01-entrada.md`, `pt/README.md` | No |
| `brand/*` (assets) | No |

### B — Community (USE + INSIDE, OK-B)

| Document | MOAT |
|----------|------|
| `contrib/classifier.md` | **OK-B** — margin, signals, tuning (intended for contributors) |
| `CLASSIFIER-SHADOW.md` | **OK-B** |
| `ENGINEERING-LOOP.md` (short) | No |
| `CAPABILITY-ROUTER-V2.md` (short) | Mild — offline pipeline overview |
| `DECISION-INTELLIGENCE.md` | **OK-B** — advisory contract |
| `LANGGRAPH-ORCHESTRATION.md` | No |
| `GRAPHIFY-INTEGRATION.md` | Mild — god-node / escalation **library** detail |
| `DS-04-CODEX-OBSERVER-CONTRACT.md` | No |
| `design/router-generalization.md` | **MOAT** — full split metrics, NB baseline, heldout2 study — **OK-B for eval culture**; consider executive summary in public + move raw tables to `downshift-labs` if you want less competitive teaching |
| `pt/09-decisoes.md` | **OK-B** — decision matrix |
| `pt/02-classificador.md` | No — points to `contrib/classifier.md` for engineering |
| `pt/10-candidato-shadow.md` | No |

### INSIDE + MOAT (teaches differentiation)

| Document | MOAT | Action |
|----------|------|--------|
| `docs/pt/03-marcha.md` | **Yes** — `policy.go`, `ShouldRewriteModel`, ≥2 margin, `explicit_only` | Trim “Detalhes de Engenharia” to link EN doc; keep product table |
| `docs/pt/04-fallbacks.md` | Mild | Review escalation narrative |
| `docs/pt/05-catalogo-e-sessao.md` | Mild | Catalog/session merge rules |
| `docs/pt/07-minilm.md` | Mild | Fuse behavior |
| `docs/pt/08-telemetria.md` | Low | Event schema overview OK for operators |

### C — Maintainer (expected, not adoption path)

| Document | MOAT |
|----------|------|
| `internal/CAPABILITY-ROUTER-V2-FULL.md` | **High** — full v2 blueprint (675 lines) — correct in `internal/` |
| `internal/ENGINEERING-LOOP-MAINTAINER.md` | **Yes** — promotion / eval |
| `internal/LAUNCH-WAVES.md` | Ops, not router moat |
| `internal/TEST-COVERAGE.md` | Low |

Stubs: `BACKLOG.md`, `NEXT-STEPS.md` (root + internal) — no moat leak.

---

## Diagrams

| Asset | Mode | MOAT |
|-------|------|------|
| `docs/img/architecture.svg` | USE | No |
| `docs/brand/downshift-routing-*.svg` | USE | No |
| `docs/brand/downshift-capability-router-v2.png` | INSIDE | Mild — v2 marketing |

---

## Benchmarks & planning

| Path | Mode | MOAT |
|------|------|------|
| `benchmark/README.md`, `tasks.json` rubric | **OK-B** | No |
| `benchmark/fresh.json`, `heldout2.json` | **OK-B** | Public eval splits by design |
| `benchmark/outcomes/` | **OK-B** | Executable tasks — not prod dumps |
| `ROADMAP.md` | USE | No |
| `docs/internal/LAUNCH-WAVES.md` | C / planning | No router secret |
| `CHANGELOG.md` | USE + history | No |

**TODO in repo:** no root `TODO.md`; wave checklists in `LAUNCH-WAVES` only.

---

## Examples

| Path | Mode | MOAT |
|------|------|------|
| `examples/` | USE | No |
| `hook-input.json` (root) | USE | No |
| `docs/examples/session-models.example.json` | USE | No |

---

## Root / meta (not in `docs/`)

| File | Mode | MOAT |
|------|------|------|
| `CONTRIBUTING.md`, `AGENTS.md` | **OK-B** | Package map — expected |
| `COMMERCIAL.md`, `MONETIZATION.md` | USE | No |
| `GOVERNANCE.md`, `SECURITY.md` | USE | No |

---

## Priority follow-ups (moat trim)

1. **README:** Collapse “How the deterministic switch works” + v2 section → links (biggest win).
2. **`router-generalization.md`:** Add 1-page “findings summary”; optional move detailed tables to private eval notebook.
3. **`pt/03-marcha.md`:** Shorten engineering block; link `contrib/classifier.md` / `ARCHITECTURE.md`.
4. **Keep** `contrib/classifier.md`, `benchmark/`, and Go source as **OK-B** (Apache fork surface).

---

## Audit program status

| Step | Done |
|------|------|
| Inventory README + `docs/` + benchmarks + examples | Yes (this file) |
| MOAT flags | Yes |
| Automated enforcement | No — manual review + `PUBLISHING-BOUNDARIES` checklist |
| README trim PR | **Pending** |

Last review: 2026-10-06.
