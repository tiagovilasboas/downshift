# Content classification (A / B / C / D)

Canonical map for **what belongs in the public Downshift repo**, what is
**maintainer-only but still cloned**, and what must live **outside git**.
Operational rules for adding docs: [PUBLISHING-BOUNDARIES.md](PUBLISHING-BOUNDARIES.md).

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
- Public benchmark splits and outcome fixtures (`benchmark/`)
- Adapter hook shapes (`internal/adapters/`)

**C applies to documentation and data you choose not to publish**, not to
pretending the OSS binary is opaque. Competitive moat = harness coverage, eval
discipline, velocity, and **ops you keep private** — see
[PUBLISHING-BOUNDARIES.md](PUBLISHING-BOUNDARIES.md).

| Material | Class | Where |
|----------|-------|--------|
| Heuristics / thresholds in Go | **B** (OSS source) | This repo |
| Playbooks, squad handoffs, live prod labels | **C** | `downshift-labs` |
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
| `docs/ARCHITECTURE.md`, `docs/WHEN-TO-USE.md`, `docs/CONFIG.md` | Core user docs |
| `docs/session-models.md`, `docs/HARNESS-MATRIX.md` | Integration |
| `docs/BETA-EXIT.md`, `docs/stats-export.md`, `docs/billing-comparison-template.md` | Trust / beta |
| `docs/MINILM-SEMANTIC.md`, `docs/pt/` (user track) | Guides |
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
| `docs/design/router-generalization.md` | Eval methodology |
| `docs/CLASSIFIER-SHADOW.md`, `docs/ENGINEERING-LOOP.md` | Offline improvement |
| `docs/CAPABILITY-ROUTER-V2.md` | Short experimental status |
| `docs/DECISION-INTELLIGENCE.md`, `docs/LANGGRAPH-ORCHESTRATION.md` | Extension points |
| `docs/DS-04-CODEX-OBSERVER-CONTRACT.md` | Privacy contract |
| `benchmark/*.json`, `benchmark/README.md`, `benchmark/outcomes/` | Reproducible eval |
| `internal/**` (implementation) | Forkable OSS |
| `.github/ISSUE_TEMPLATE/` | Misroute, stats export, harness |
| `orchestration/` | Optional planner (LangGraph) |
| `tests/ds04_observer_contract/` | Privacy tests |

### C — Competitive / internal (minimize in public clone)

| Path | Notes |
|------|--------|
| `docs/internal/LAUNCH-WAVES.md` | Launch execution |
| `docs/internal/ENGINEERING-LOOP-MAINTAINER.md` | Promotion thresholds |
| `docs/internal/CAPABILITY-ROUTER-V2-FULL.md` | Long v2 archive |
| `docs/internal/TEST-COVERAGE.md` | Maintainer tables |
| `docs/internal/downshift-labs-README.template.md` | Private repo scaffold |
| `docs/internal/BACKLOG.md`, `docs/internal/NEXT-STEPS.md` | **Stubs** → real content in `downshift-labs` |
| `docs/brand/RENAME.md`, `docs/brand/LAUNCH-DISCUSSION.md` | Rebrand ops (low secret, still maintainer) |
| `docs/internal/LAUNCH-WAVES.md` metrics / open-core rows | Strategy hints |
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

| A/B/C/D | [PUBLISHING-BOUNDARIES](PUBLISHING-BOUNDARIES.md) |
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
| **Documentation audit (use vs inside vs MOAT)** | Done — [internal/DOC-AUDIT.md](internal/DOC-AUDIT.md) |
| Private `downshift-labs` repo (OC-1) | **Pending** — only true vault |
| Per-file audit of all 600+ paths | **Not required** — use audit + checklist |
| README moat trim (see audit) | **Pending** |
| History rewrite for removed **C** | **Not done** — optional, high cost |

---

harness-downshift by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/downshift
