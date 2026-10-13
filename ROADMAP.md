# Roadmap — Downshift

Public direction for the open-source project. Timelines are indicative.

**Status:** beta — exit criteria in [docs/beta-exit.md](docs/beta-exit.md).  
**Product roadmap (owner, milestones and tasks):** [docs/product/roadmap.md](docs/product/roadmap.md)  
**Maintainer execution:** [docs/internal/launch-waves.md](docs/internal/launch-waves.md) (see [publishing boundaries](docs/publishing-boundaries.md)).

Downshift by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/downshift

---

## Product vision

**Downshift** is an open-source **model router**: classify workload complexity,
apply catalog policy, and select model tier **before execution**, without an LLM
in the routing loop.

Today the primary integration is **coding harness hooks** (Claude Code, Cursor,
Codex, and others). The core is intentionally **gateway-shaped** for a future
HTTP proxy, but that is not the current shipping surface.

---

## Now (beta)

| Theme | Deliverable |
|-------|-------------|
| Harness routing | Stable PreToolUse rewrites + adapter coverage |
| Honest telemetry | `rewrite_emitted` vs harness-reported honor (`resolved`) |
| Project controls | Optional per-project tier floor (`.downshift/policy.json`), counted in `stats`; how a child ended (`child_ended`) |
| Session discovery | `downshift models discover` per harness ([session-discovery.md](docs/session-discovery.md)); Cursor and Antigravity sources, catalog as an opt-in fallback |
| Safe by default | Doubt never spends money (R7, R8, semantic boost capped); inferred session on Claude Code; `SEMANTIC_BOOST` provenance and a frontier-on-doubt alarm in `stats` |
| Real cost | SubagentStop usage linkage (Claude Code); expand as harnesses expose usage |
| Evaluation | Public [benchmark/REPORT.md](benchmark/REPORT.md); full regression in maintainer downshift-labs; shadow observation |
| Docs | User docs (install, session models), [classifier contrib](docs/contrib/classifier.md), misroute workflow |
| Eval rigor | [router-generalization](docs/design/router-generalization.md) summary; detailed study in labs |

**Beta exit blockers (community / dogfood):** see [beta-exit.md](docs/beta-exit.md)
pillars P2 (multi-user exports) and P3 (billing evidence).

---

## Next (0–3 months)

| Priority | Item |
|----------|------|
| P0 | ~~Rename to **Downshift**~~ — done ([rename.md](docs/brand/rename.md)) |
| P0 | ~~State directory `~/.downshift`~~ — done ([config.md](docs/config.md)) |
| P1 | ~~Condensed docs~~ — [WHEN-TO-USE](docs/when-to-use.md), trimmed [CAPABILITY-ROUTER-V2](docs/capability-router-v2.md) |
| P1 | More harness matrix rows (Cursor paid plans, etc.) |
| P1 | Optional: container image on GHCR |
| P1 | Independent outcome suite and cost per completed task before any tier-memory promotion ([gap-tasks](docs/gap-tasks.md): GAP-SUITE, GAP-MEMORY-COUNTS) |
| P1 | Real hook payloads for Cursor, Codex, Kiro, Antigravity, and the Claude Code failure event (GAP-PAYLOADS, GAP-CHILD-FAILURE) |
| P1 | Learned complexity scorer for EN and PT-BR (hashed word and char n-grams plus structural features, embedded JSON under 1 MB, shadow first, abstain to MEDIUM). Needs about 2,400 labelled prompts, at least 40% PT-BR |
| P1 | Confident upshift targets the smallest model that meets the tier, not the last id in the list (decision pending; `explicit_upshift` keeps the strongest) |
| P1 | Session-start discovery hooks for harnesses that expose one (Codex, Cursor, Kiro); needs evidence per harness before `InferSession` is enabled there |
| P2 | Promote routing v2 only with reviewed labels + shadow evidence |

---

## Later (3–12 months)

| Theme | Notes |
|-------|--------|
| HTTP gateway | OpenAI-compatible proxy sharing the same `core` policy (design TBD) |
| SDKs | TypeScript/Python clients scoped (`@downshift/*`, not npm `downshift`) |
| Cloud (optional paid) | Hosted routing, analytics, governance — see [MONETIZATION.md](MONETIZATION.md) |
| Learned routing | Upshift-only second opinions; see [router-generalization](docs/design/router-generalization.md) |

---

## Explicit non-goals (for now)

- Competing with OpenRouter on catalog breadth or hosted billing
- LLM-in-the-loop routing on the production hot path
- Storing prompts in telemetry or training on raw task text

---

## How to influence the roadmap

1. **Misrouted prompts** — [CONTRIBUTING.md](CONTRIBUTING.md) (`misroute:` issues)
2. **New harness adapters** — [.github/ISSUE_TEMPLATE/new-harness.md](.github/ISSUE_TEMPLATE/new-harness.md)
3. **Discussions** — use GitHub Discussions when enabled (feature requests)

Maintainer-only execution detail lives under [docs/internal/](docs/internal/).
