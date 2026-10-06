# Publishing boundaries — public repo vs maintainer vs private

Downshift is an **open-source** repository. Not every markdown file is meant for
adopters, contributors, or investors reading the default doc path. This page is
the contract for what stays public, what lives under `docs/internal/`, and what
belongs in a **private** repo later ([OC-1 in LAUNCH-WAVES](internal/launch-waves.md)).

**Classification labels A / B / C / D** (adoption, community, competitive,
secret): [content-classification.md](content-classification.md).

---

## Three tiers

| Tier | Audience | Location | Goal |
|------|----------|----------|------|
| **Public (product)** | Installers, contributors, press | `README`, `docs/*` (indexed in [docs/README.md](README.md)) | Trust, install, misroutes, honest limits |
| **Public (maintainer, unlisted)** | You + co-maintainers; curious diggers | `docs/internal/` | Launch checklist, v2 archive, coverage tables (stubs for private ops) |
| **Private** | Solo / company strategy not ready to share | e.g. `downshift-labs` (planned) | Open-core SKU detail, agent handoff at scale, unreleased research |

**Important:** `docs/internal/` is still in the public git clone. It is “unlisted”
by convention (not in the main doc index hero), not DRM. True secrets do not
belong in git; use a private repository.

---

## Public — keep here (adoption & contribution)

| Area | Examples | Why public |
|------|----------|------------|
| Install & config | [config.md](config.md), [session-models.md](session-models.md), `install.sh` | Required to run |
| Architecture | [architecture.md](architecture.md), [when-to-use.md](when-to-use.md) | Positioning without over-selling |
| Harness honesty | [harness-matrix.md](harness-matrix.md), [beta-exit.md](beta-exit.md) | Beta trust |
| Contribute | [CONTRIBUTING.md](../CONTRIBUTING.md), [contrib/classifier.md](contrib/classifier.md) | Community quality |
| Published metrics | [benchmark/REPORT.md](../benchmark/REPORT.md) | Final numbers only (raw eval in `downshift-labs`) |
| Licence & trust | [LICENSE](../LICENSE), [relicense.md](relicense.md), [COMMERCIAL.md](../COMMERCIAL.md), [MONETIZATION.md](../MONETIZATION.md) | Apache 2.0 transparency (not a sales deck) |
| Experimental (short) | [capability-router-v2.md](capability-router-v2.md), [classifier-shadow.md](classifier-shadow.md) | Set expectations: v2 ≠ production hook |
| Direction | [ROADMAP.md](../ROADMAP.md), [GOVERNANCE.md](../GOVERNANCE.md) | OSS expectations |

**Moat (public framing):** harness integrations + eval discipline + release
velocity — not “secret regex.” Implementation detail that helps contributors
(e.g. [design/router-generalization.md](design/router-generalization.md)) stays
public; **launch playbooks and autonomous agent contracts** do not need to be
in the default path.

---

## `docs/internal/` — maintainer-oriented (still in public repo)

| File | Content |
|------|---------|
| [backlog.md](internal/backlog.md) | **Stub** — full backlog in private `downshift-labs` |
| [next-steps.md](internal/next-steps.md) | **Stub** — maintainer checklist off-repo |
| [launch-waves.md](internal/launch-waves.md) | Rebrand/OSS wave execution, metrics, open-core backlog |
| [capability-router-v2-full.md](internal/capability-router-v2-full.md) | Long v2 design archive |
| [test-coverage.md](internal/test-coverage.md) | Coverage tables for maintainers |
| [engineering-loop-maintainer.md](internal/engineering-loop-maintainer.md) | Promotion loop, dsmon, historical eval notes |
| [downshift-labs-readme.template.md](internal/downshift-labs-readme.template.md) | Scaffold for private repo layout |

Stubs at old paths (e.g. `docs/brand/launch-waves.md`) redirect here.

---

## Competitive sensitivity (green / yellow / red)

Use this when writing or reviewing docs. **Red** never ships in the public repo
(current content). **Yellow** may ship in shortened form or under `docs/internal/`
knowing git history and clones are public.

| Class | Examples | Tier |
|-------|----------|------|
| **Green** | Install, CONFIG, adapters, fail-open, `benchmark/REPORT.md`, contrib classifier, Apache licence | Public `docs/` |
| **Green** | “We may offer hosted/support” without SKU detail | [MONETIZATION.md](../MONETIZATION.md) |
| **Yellow** | Launch wave checklists, coverage tables, long v2 archive | `docs/internal/` |
| **Yellow** | Promotion thresholds, shadow/train workflow (no live production numbers) | `docs/internal/` or shortened [engineering-loop.md](engineering-loop.md) |
| **Red** | Autonomous agent squad contracts, DS/HC/KB handoff playbooks, personal repo paths | Private `downshift-labs` |
| **Red** | Production-derived labels, unredacted stats exports, enterprise pipeline | Private only |
| **Red** | Employer or personal KB dumps, embargoed partnerships | Never in Downshift git |

**Fork reality (Apache 2.0):** anyone can fork the **router code**. The moat is
not hiding the classifier; it is **harness coverage**, **eval discipline**,
**release quality**, and **ops you choose not to publish**. Do not treat
`docs/internal/` as a vault.

---

## Private (future) — do not put in this repo

| Content | Why private |
|---------|-------------|
| Unreleased cloud/SKU pricing, enterprise pipeline | Commercial strategy |
| Full agent squad handoffs at corporate scale | Operational noise for adopters |
| Copies of personal `rag-kb` or employer KB rules | Wrong repository — keep personal/corporate KB outside Downshift |
| Pre-announce embargoes, partnership drafts | Time-bound |

Planned home: **`downshift-labs`** (private) per launch waves OC-1.

---

## What we do *not* hide in OSS

- Deterministic classifier approach (signals, tests, benchmarks)
- Adapter hook shapes and fail-open behaviour
- Shadow / train CLI for **offline** eval
- That hosted/support may exist later ([MONETIZATION.md](../MONETIZATION.md))

---

## Checklist before adding a new doc

1. **Adopter** needs it to install or trust? → `docs/` (public index).
2. **Contributor** needs it to tune or review? → `docs/contrib/` or `benchmark/`.
3. **Maintainer** execution / waves / agent contract? → `docs/internal/`.
4. **Business** not ready for the world? → private repo, link nothing from README.
