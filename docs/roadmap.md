# Roadmap — Downshift

Public direction for the open-source project. Timelines are indicative.

**Status:** beta — exit criteria in [docs/reference/beta-exit.md](reference/beta-exit.md).  
**Product roadmap (owner, milestones and tasks):** [docs/product/roadmap.md](product/roadmap.md) · [Context & reasoning](product/roadmap-context-and-reasoning.md)  
**Value roadmap (compaction, proof, cheaper workers, 2026-11):** [docs/product/value-roadmap.md](product/value-roadmap.md)  
**Impact roadmap (blind spots, latency, proof in real use, 2026-11-10):** [docs/product/impact-roadmap.md](product/impact-roadmap.md)  
**Maintainer execution:** [docs/internal/launch-waves.md](internal/launch-waves.md) (see [publishing boundaries](reference/publishing-boundaries.md)).  
**Latest handoff (open PRs, findings, next steps):** 2026-11-02, routing, compactor and MCP ([milestone](https://github.com/tiagovilasboas/downshift-ai/milestone/1)).

Downshift by Tiago Vilas Boas\
https://github.com/tiagovilasboas/downshift-ai

---

## Product vision

**Downshift** is an open-source **model control layer**: the orchestrator chooses
the model of each subagent, and Downshift applies that choice within your declared
policy **before execution** and measures the result. It adds no LLM call of its own.

Today the primary integration is **coding harness hooks** (Claude Code, Cursor,
Codex, and others). The core is intentionally **gateway-shaped** for a future
HTTP proxy, but that is not the current shipping surface.

**Integration priorities (owner direction, 2026-10-18): Claude Code, Cursor and
OpenCode.** Claude Code has the strongest native evidence today. Cursor still
needs paid-plan rewrite proof. The OpenCode Go adapter and wire tests exist, and
`downshift setup opencode` installs a bridge plugin (Beta; the retry fallback is not verified yet). These are
product priorities, not a claim of equal release readiness across the three.

---

## Now (beta)

| Theme | Deliverable |
|-------|-------------|
| Harness routing | Stable PreToolUse rewrites + adapter coverage |
| Honest telemetry | `rewrite_emitted` vs harness-reported honor (`resolved`) |
| Project controls | Optional per-project tier floor (`.downshift/policy.json`), counted in `stats`; how a child ended (`child_ended`) |
| Session discovery | `downshift models discover` per harness ([session-discovery.md](reference/session-discovery.md)); Cursor and Antigravity sources, catalog as an opt-in fallback |
| Safe by default | Doubt never spends money (R7, R8, semantic boost capped); inferred session on Claude Code; `SEMANTIC_BOOST` provenance and a frontier-on-doubt alarm in `stats` |
| Real cost | SubagentStop usage linkage (Claude Code); expand as harnesses expose usage |
| Tokens first | Live views show tokens moved to cheaper models (work and cache apart), per agent, agent type and session; `downshift report` compares routed and control tokens per completed task, with no dollar output |
| Evaluation | Public [benchmark/report.md](benchmark/report.md); full regression in maintainer downshift-labs; shadow observation |
| Docs | User docs (install, session models), [classifier contrib](contrib/classifier.md), misroute workflow |
| Eval rigor | [router-generalization](design/router-generalization.md) summary; detailed study in labs |

**Beta exit blockers (community / dogfood):** see [beta-exit.md](reference/beta-exit.md)
pillars P2 (multi-user exports) and P3 (routed-versus-control token comparison).

---

## Next (0–3 months)

| Priority | Item |
|----------|------|
| P0 | ~~Rename to **Downshift**~~ — done ([rename.md](brand/rename.md)) |
| P0 | ~~State directory `~/.downshift`~~ — done ([config.md](guides/config.md)) |
| P1 | ~~Condensed docs~~ — [WHEN-TO-USE](guides/when-to-use.md), trimmed [ROUTING-V2](design/routing-v2.md) |
| ~~P0~~ | ~~Opt-in `downshift exec`~~ — superseded: the native compactor (`downshift compact`) is on by default with the plugin, `"exec": false` turns it off. Families: go test, git status, pytest, vitest, jest, tsc, cargo test, playwright, phpunit, next build, Gradle, Maven, Flutter/Dart. The conformance suite gates every family. |
| P1 | Child-token payloads for Cursor, Codex, OpenCode, KiroCrew and Antigravity so tokens-first views cover them (today `stats` shows all five as unobserved); Codex tier order decision |
| P1 | More harness matrix rows (Cursor paid plans, etc.) |
| ~~P1~~ | **OpenRouter parts 4–6 deferred (owner 2026-10-24):** shadow routing, OpenCode wiring (`openrouter/<slug>`), live rewrite behind a flag — after M1 proof, compactor backlog, and harness honor. Parts 1–3 merged or in review |
| ~~P1~~ | ~~DCI hook through the shared runner~~ — dropped: code intelligence (DCI) was removed on 2026-11-01 (#720) |
| P1 | Optional: container image on GHCR |
| P1 | Control group: 14 completed tasks against a minimum of 20 per group (`downshift report`), before any routing-cost claim |
| P1 | Publish the real-use measurement behind the "up to ~53% fewer tokens per task" figure as a file under `docs/evidence/` (1 pt): KiroCrew, 2026-10-21, routing and compactor combined, 6 control tasks, one session, plus the routing-only ~24% from the four-arm Claude Code test (46 runs). Third parties already quote the figure (the Montanha Tech site), so it needs a durable source with method and sample size, and it keeps the "directional, not definitive" qualifier |
| P1 | Independent outcome suite and cost per completed task before any tier-memory promotion ([gap-tasks](internal/gap-tasks.md): GAP-SUITE, GAP-MEMORY-COUNTS) |
| P1 | Real hook payloads for Cursor, Codex, Kiro, Antigravity, and the Claude Code failure event (GAP-PAYLOADS, GAP-CHILD-FAILURE) |
| P1 | Learned complexity scorer for EN and PT-BR (hashed word and char n-grams plus structural features, embedded JSON under 1 MB, shadow first, abstain to MEDIUM). Needs about 2,400 labelled prompts, at least 40% PT-BR |
| P1 | Confident upshift targets the smallest model that meets the tier, not the last id in the list (decision pending; `explicit_upshift` keeps the strongest) |
| P1 | Session-start discovery hooks for harnesses that expose one (Codex, Cursor, Kiro); needs evidence per harness before `SessionFromCatalog` is enabled there |
| ~~P1~~ | ~~Compactor: the notes `compact` appends after a compacted output (`2>/dev/null`, `\|\| true`) count in the "never longer than the raw output" check and in the emitted bytes~~ — done (2 pts, `internal/execwrap`) |
| P1 | Compactor: a `downshift retrieve` of an id two sessions kept is charged to every session that kept it, and `retrieve` records no session (proven by a failing test). Decide first: carry the hashed session in the marker's retrieve hint (longer marker), attribute by time (a heuristic), or document the per-id behaviour (3 pts, `internal/execwrap` and `internal/accounting`) |
| ~~P2~~ | ~~Compactor: the post-run rows say "offered", and an offline `report` or `doctor` check joins each compacted row that kept an id to the Claude Code transcript and counts honored, not found and mismatch~~ — done (3 pts): a `doctor` row per harness (`post-run replacement`: honored, not found, mismatch, unverified over the latest 200 rows, warning only when a harness has checked rows and none honored) plus the report wording that calls post-run figures "offered". Honored = the kept id's retrieve hint in a transcript result of the emitted length or one byte less, each result used once. Rows without a kept id, or outside the transcript coverage, stay unverified; see [post-run compaction](design/post-run-compaction.md#honor-check) |
| P2 | Compactor: with CCR off, a lossy cut says in its marker that no copy was kept, or the lossy families require CCR (2 pts; product decision first, see [ccr.md](guides/ccr.md)) |
| P2 | Promote routing v2 only with reviewed labels + shadow evidence |

---

## Later (3–12 months)

| Theme | Notes |
|-------|--------|
| HTTP gateway | OpenAI-compatible proxy sharing the same `core` policy (design TBD) |
| SDKs | TypeScript/Python clients scoped (`@downshift/*`, not npm `downshift`) |
| Cloud (optional paid) | Hosted routing, analytics, governance — see [commercial.md](reference/commercial.md) |
| Learned routing | Upshift-only second opinions; see [router-generalization](design/router-generalization.md) |
| OpenRouter integration | Parts 4–6 (shadow, OpenCode `openrouter/<slug>`, flagged live rewrite) — **last** among near-term work (owner 2026-10-24); parts 1–3 already landed or in review |

---

## Explicit non-goals (for now)

- Competing with OpenRouter on catalog breadth or hosted billing
- LLM-in-the-loop routing on the production hot path
- Storing prompts in telemetry or training on raw task text

---

## How to influence the roadmap

1. **Misrouted prompts** — [CONTRIBUTING.md](../CONTRIBUTING.md) (`misroute:` issues)
2. **New harness adapters** — [.github/ISSUE_TEMPLATE/new-harness.md](../.github/ISSUE_TEMPLATE/new-harness.md)
3. **Discussions** — use GitHub Discussions when enabled (feature requests)

Maintainer-only execution detail lives under [docs/internal/](internal/).
