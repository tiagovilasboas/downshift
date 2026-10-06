# Content classification (A / B / C / D)

Canonical map for **what belongs in the public Downshift repo**, what is
**maintainer-only but still cloned**, and what must live **outside git**.
Operational rules for adding docs: [publishing-boundaries.md](publishing-boundaries.md).

---

## Categories

| Class | Name | Purpose | Typical location |
|-------|------|---------|------------------|
| **A** | PUBLIC / ADOPTION | Install, integrate, trust, use the product | `README`, indexed `docs/`, `examples/`, `install.sh` |
| **B** | PUBLIC / COMMUNITY | Contribute, extend, understand non-secret design | `CONTRIBUTING.md`, `docs/contrib/`, `docs/design/`, `benchmark/`, tests |
| **C** | INTERNAL / COMPETITIVE | Ops, strategy, deep eval notes, data you do not want to gift | Private `downshift-labs`; **stubs** or **short** copies in `docs/internal/` |
| **D** | SECRET / SECURITY | Credentials, PII, production dumps | **Never** in this repo; user state under `~/.downshift/` |

---

## Policy: category C in **code** vs **docs/data**

Apache 2.0 Downshift **forks the router implementation**. These stay **public**
and are classified **B** (community / reproducibility), not hidden as C:

- Deterministic signals and scoring (`internal/core/`, `internal/routingv2/`)
- Embedded catalog (`internal/catalog/catalog.json`)
- Adapter hook shapes (`internal/adapters/`)
- Tiny **test fixtures** only (`internal/benchmark/testdata/sample.json`, `internal/outcome/testdata/`)

**C applies to documentation and data you choose not to publish**, not to
pretending the OSS binary is opaque. Competitive moat = harness coverage, eval
discipline, velocity, and **ops you keep private** — see
[publishing-boundaries.md](publishing-boundaries.md).

| Material | Class | Where |
|----------|-------|--------|
| Heuristics / thresholds in Go | **B** (OSS source) | This repo |
| Playbooks, squad handoffs, live prod labels | **C** | `downshift-labs` |
| Benchmark JSON, outcome suite, eval-only splits | **C** | `downshift-labs` (`eval/`) |
| Training scripts (`tools/minilm`, `tools/baseline`), NB golden parity | **C** | `downshift-labs` |
| Shadow/train exports, promotion thresholds (full) | **C** | `downshift-labs` |
| Unredacted `stats --export` from real sessions | **C** / **D** | Never commit |
| SKU pricing, enterprise pipeline | **C** | Private only |

---

## Path inventory (maintained summary)

### A — Adoption

| Path | Notes |
|------|--------|
| `README.md`, `llms.txt` | Hero, install, hooks |
| `install.sh`, `catalog.sample.json` | Distribution |
| `examples/` | Hook JSON fixtures |
| `docs/architecture.md`, `docs/when-to-use.md`, `docs/config.md` | Core user docs |
| `docs/session-models.md`, `docs/harness-matrix.md` | Integration |
| `docs/beta-exit.md`, `docs/stats-export.md`, `docs/billing-comparison-template.md` | Trust / beta |
| `docs/minilm-semantic.md`, `docs/pt/` (user track) | Guides |
| `docs/evidence/` | Honest harness honor protocol (no secrets) |
| `LICENSE`, `NOTICE`, `SECURITY.md`, `COMMERCIAL.md`, `MONETIZATION.md` | Legal / trust (high level) |
| `ROADMAP.md`, `GOVERNANCE.md` | Direction (public, not sales deck) |
| `cmd/downshift/` (CLI surface) | User commands |
| `internal/adapters/` | Required for integration behavior |

### B — Community

| Path | Notes |
|------|--------|
| `CONTRIBUTING.md`, `AGENTS.md`, `CODE_OF_CONDUCT.md` (if present) | Contribute |
| `docs/contrib/classifier.md` | Tuning v1 classifier |
| `docs/design/router-generalization.md` | Public summary (full study in labs) |
| `docs/classifier-shadow.md`, `docs/engineering-loop.md` | Offline improvement (short); full ops in labs |
| `docs/capability-router-v2.md` | Short experimental status |
| `docs/decision-intelligence.md`, `docs/langgraph-orchestration.md` | Extension points |
| `docs/ds-04-codex-observer-contract.md` | Privacy contract |
| `benchmark/REPORT.md`, `benchmark/EVAL-PRIVATE.md` | Published metrics + policy only |
| `tools/README.md` | Pointer to labs training scripts |
| `internal/**` (implementation) | Forkable OSS |
| `.github/ISSUE_TEMPLATE/` | Misroute, stats export, harness |
| `orchestration/` | Optional planner (LangGraph) |
| `tests/ds04_observer_contract/` | Privacy tests |

### C — Competitive / internal (minimize in public clone)

| Path | Notes |
|------|--------|
| `docs/internal/launch-waves.md` | Launch execution |
| `docs/internal/engineering-loop-maintainer.md` | **Stub** → labs |
| `docs/internal/capability-router-v2-full.md` | **Stub** → labs |
| `docs/internal/test-coverage.md` | Maintainer tables |
| `docs/internal/downshift-labs-readme.template.md` | Private repo scaffold |
| `docs/internal/backlog.md`, `docs/internal/next-steps.md` | **Stubs** → real content in `downshift-labs` |
| `docs/brand/rename.md`, `docs/brand/launch-discussion.md` | Rebrand ops (low secret, still maintainer) |
| `docs/internal/launch-waves.md` metrics / open-core rows | Strategy hints |
| **Not in repo** | Prod-derived labels, exports with prompts, pricing SKUs, agent squad contracts |

`docs/internal/` is **public git** — conventionally unlisted, not a vault.

### D — Secret (never commit)

| Path | Notes |
|------|--------|
| `~/.downshift/` (user machine) | `events.jsonl`, overrides, weights experiments |
| Env: API keys for `models pull` | User shell only |
| `.env`, credentials, customer data | Blocked by review + `.gitignore` patterns |
| CI secrets | GitHub Encrypted Secrets only |

---

## Mapping to publishing tiers

| A/B/C/D | [PUBLISHING-BOUNDARIES](publishing-boundaries.md) |
|---------|-----------------------------------------------------|
| **A** | Public product (green) |
| **B** | Public product or contrib (green) |
| **C** | Yellow (`docs/internal/`) or red (private repo) |
| **D** | Red — never in Downshift git |

---

## Checklist for new content

1. **Adopter must run or trust it?** → **A** → `docs/` index + link from README if top-level.
2. **Contributor needs it to patch safely?** → **B** → `docs/contrib/`, `benchmark/`, or code comments.
3. **Ops, strategy, or prod-derived eval?** → **C** → `downshift-labs`; at most stub + pointer in `docs/internal/`.
4. **Credentials, PII, raw session prompts?** → **D** → do not add; use local state dir only.

After moving **C** out of the repo, assume **git history** may still contain old
versions; rotation is not automatic.

---

## Completion status (classification program)

| Step | Status |
|------|--------|
| A/B/C/D definitions (this doc) | Done |
| Path inventory (summary table) | Done — refresh when layout changes |
| `PUBLISHING-BOUNDARIES` alignment | Done |
| Sensitive maintainer docs stubbed | Done (`BACKLOG`, `NEXT-STEPS`) |
| **Documentation audit (use vs inside vs MOAT)** | Done — [internal/doc-audit.md](internal/doc-audit.md) |
| Private `downshift-labs` repo (OC-1) | **Done** — private GitHub repo + `ci-eval` |
| Labs split (benchmarks + training ops) | **Done** — merged [#57](https://github.com/tiagovilasboas/downshift/pull/57) |
| Per-file audit of all 600+ paths | **Not required** — use audit + checklist |
| README moat trim (see audit) | Done |
| History rewrite for removed **C** | **Not done** — optional, high cost |

---

harness-downshift by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/downshift
