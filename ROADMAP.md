# Roadmap — Downshift

Public direction for the open-source project. Timelines are indicative.

**Status:** beta — exit criteria in [docs/BETA-EXIT.md](docs/BETA-EXIT.md).  
**Execution waves:** [docs/brand/LAUNCH-WAVES.md](docs/brand/LAUNCH-WAVES.md).

Downshift by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/harness-downshift

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
| Real cost | SubagentStop usage linkage (Claude Code); expand as harnesses expose usage |
| Evaluation | Seed/holdout regression gates; outcome tasks; shadow classifier observation |
| Docs | User docs (install, session models), contributor misroute workflow |

**Beta exit blockers (community / dogfood):** see [BETA-EXIT.md](docs/BETA-EXIT.md)
pillars P2 (multi-user exports) and P3 (billing evidence).

---

## Next (0–3 months)

| Priority | Item |
|----------|------|
| P0 | Public positioning and rename to **Downshift** ([RENAME.md](docs/brand/RENAME.md)) |
| P0 | State directory `~/.downshift` with legacy fallback |
| P1 | Condensed architecture and “when to use” vs hosted gateways |
| P1 | More harness matrix rows (Cursor paid plans, etc.) |
| P1 | Optional: container image on GHCR |
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
