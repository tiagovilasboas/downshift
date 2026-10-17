# Documentation index

Docs are grouped by audience: **guides/** (how-tos), **reference/** (specs and
contracts), **design/** (architecture and design notes), **evidence/** (dated
runtime proof) and **internal/** (maintainer planning). Markdown file names
under `docs/` are lowercase kebab-case; only names a tool requires (`README.md`,
`SKILL.md`, `AGENTS.md`) keep their exact casing.

## The suite in one table

Downshift is a suite for optimizing token use in coding agents; subagent routing is the main lever, not the whole
product. Status words below are the [capability matrix](reference/capability-matrix.md)'s; harness words stay
Completed (Claude Code), Functional (Codex, Cursor), Experimental (KiroCrew, Antigravity).

| Piece | One sentence | Status | Doc |
|---|---|---|---|
| Deterministic subagent routing | A `PreToolUse` hook rewrites one subagent's model and effort from the session's list. | Implemented; honored on Claude Code | [design/architecture.md](design/architecture.md) |
| Guardrails (doubt never spends money) | R1 to R9 hold unconfident moves; risk categories keep a floor. | Implemented | [reference/complexity-weights.md](reference/complexity-weights.md) |
| Project tier floor | `.downshift/policy.json` raises a minimum tier, never lowers one. | Implemented, every harness | [guides/config.md](guides/config.md#project-tier-floor-optional) |
| Quota and credit gate | Hook marks or native snapshots hold rewrites to models without credit. | Implemented, not validated everywhere | [guides/quota.md](guides/quota.md) |
| Session view and statusline | `session --tree`, `/downshift`, `watch`, stderr line, statusline (Claude Code). | Implemented | [guides/session-view.md](guides/session-view.md) |
| Telemetry and cost reports with a control group | Prompt-free `events.jsonl`, about 1 in 10 spawns unrouted, `stats` and `report`. | Implemented; estimates, not an invoice | [guides/report.md](guides/report.md), [reference/stats-export.md](reference/stats-export.md) |
| Context policy | `setup` sets `opusplan`, plan mode and You should know for unset keys; compressor observes bytes. | Implemented, limited (Claude Code) | [guides/claude-code.md](guides/claude-code.md), [design/context-optimization.md](design/context-optimization.md) |
| Models it knows | Embedded catalog, families per harness, `explicit_only`, `provider/` prefix strip, session inference. | Implemented | [reference/session-models.md](reference/session-models.md), [../README.md#the-llms-it-knows](../README.md#the-llms-it-knows) |
| Languages | EN and PT-BR signals with the word-hash boost; MiniLM and E5 fuse opt-in; E5 shadow with `shadow-report`. | Implemented (EN, PT-BR); E5 gain unproven | [contrib/classifier.md](contrib/classifier.md), [evidence/e5-int8-evaluation-2026-10-17.md](evidence/e5-int8-evaluation-2026-10-17.md) |
| Mods | cache-tax in the plugin; savvy-progress and filetree as sibling plugins; downshift-router opt-in prototype. | Shipped; router Experimental | [../mods/README.md](../mods/README.md), [design/mod-router.md](design/mod-router.md) |
| Companions | Headroom (default compression proxy), detected and recorded, never installed or wrapped. | Documented | [guides/companions.md](guides/companions.md) |
| Evidence ladder | Rungs A to E; rung D reached once (2026-10-15), rung E not measured. | Honest by construction | [guides/evidence.md](guides/evidence.md) |

## Start here

| Doc | For |
|-----|-----|
| **[guides/install.md](guides/install.md)** | Install binary, hooks per harness, troubleshooting (humans + agents) |
| [reference/capability-matrix.md](reference/capability-matrix.md) | Every capability with status (implemented, not validated everywhere, experimental, planned), evidence and limits |
| [design/architecture.md](design/architecture.md) | End-to-end execution flow, who owns each step (Downshift, harness, external), what it is not |
| [guides/claude-code.md](guides/claude-code.md) | Primary harness: opusplan, baseline inference, plan mode, emitted vs honored vs used |
| [guides/evidence.md](guides/evidence.md) | Evidence ladder: list price, telemetry estimate, controlled benchmark, real tasks, cost per completed task |
| [guides/benchmark-protocol.md](guides/benchmark-protocol.md) | Reproducible comparison: fixed Opus vs native opusplan vs opusplan + Downshift |
| [guides/when-to-use.md](guides/when-to-use.md) | Downshift vs gateways and direct SDK; Claude Code is the primary harness |
| [guides/config.md](guides/config.md) | State dir, env vars, project tier floor, migration |
| [reference/beta-exit.md](reference/beta-exit.md) | Beta graduation criteria |
| [../ROADMAP.md](../ROADMAP.md) | Public roadmap |
| [../LICENSE](../LICENSE) | Apache 2.0 |
| [../MONETIZATION.md](../MONETIZATION.md) | Optional hosted/support |
| [../GOVERNANCE.md](../GOVERNANCE.md) | Releases and decisions |

## Guides

| Doc | Topic |
|-----|--------|
| [guides/install.md](guides/install.md) | Installation and harness setup |
| [guides/config.md](guides/config.md) | Configuration and local state |
| [guides/when-to-use.md](guides/when-to-use.md) | When to use Downshift |
| [guides/companions.md](guides/companions.md) | Companions: Headroom (default, detected and recorded), recommended Claude Code mods (You should know, Token Weather, Replay Theater, reflect-mod), patterns Downshift follows but does not bundle |
| [guides/claude-code.md](guides/claude-code.md) | Claude Code guide: opusplan and Downshift, how the baseline model is found, what is inferred, plugin vs setup |
| [guides/evidence.md](guides/evidence.md) | What is proven at which level, and the metrics table (available, inferred, estimated, not yet) |
| [guides/benchmark-protocol.md](guides/benchmark-protocol.md) | Protocol for the main comparison and what each run must record |
| [../skills/README.md](../skills/README.md) | The three skills: jobs, measured context cost, delegation overhead |
| [guides/daily-truth.md](guides/daily-truth.md) | What Downshift does day to day (routes a subagent, not a session) |
| [guides/quota.md](guides/quota.md) | Observed usage quota |
| [guides/session-view.md](guides/session-view.md) | `downshift session`: subagent tree, why, pause |
| [guides/report.md](guides/report.md) | `downshift report`: routed vs control group, savings with CI |
| [guides/pilot.md](guides/pilot.md) | Two-week team pilot: routed versus control |
| [guides/probe.md](guides/probe.md) | `downshift probe`: what a hook payload really carries |
| [guides/kirocrew-routing-guide.md](guides/kirocrew-routing-guide.md) | KiroCrew subagent routing |
| [guides/billing-comparison-template.md](guides/billing-comparison-template.md) | Compare provider billing with `downshift stats --export` |
| [pt/README.md](pt/README.md) | Documentação em português |
| [../examples/README.md](../examples/README.md) | Sample hook JSON payloads |
| [guides/subagents.md](guides/subagents.md) | When a subagent is routable: omit `model`; the example agent in `agents/` |
| [../skills/downshift-setup/SKILL.md](../skills/downshift-setup/SKILL.md) | Agent skill: install or repair hooks on any harness, dry-run first |
| [../skills/downshift-audit/SKILL.md](../skills/downshift-audit/SKILL.md) | Read-only agent skill: audit hook wiring, stats and fixed context size |
| [../skills/cost-reducer/SKILL.md](../skills/cost-reducer/SKILL.md) | Agent skill: cut session tokens in a fixed order (routing, fixed context, tool output, policy, misroutes) |
| [../.claude-plugin/plugin.json](../.claude-plugin/plugin.json) | Claude Code plugin: `/plugin install downshift --marketplace tiagovilasboas/downshift` |
| [../mods/README.md](../mods/README.md) | Mods: cache-tax inside the plugin, savvy-progress and filetree as sibling plugins (one install each), re-vendor procedure and tests |
| [../mods/downshift-router/README.md](../mods/downshift-router/README.md) | Experimental mod (off by default): routing from `agent.spawn` through the same binary |

## Reference

| Doc | Topic |
|-----|--------|
| [reference/capability-matrix.md](reference/capability-matrix.md) | Capability matrix with status, evidence, limits (audit date 2026-10-17) |
| [reference/session-models.md](reference/session-models.md) | Session allowlists and Claude usage hooks |
| [reference/session-discovery.md](reference/session-discovery.md) | `models discover`: which models each harness offers, and how they are ranked |
| [reference/complexity-weights.md](reference/complexity-weights.md) | Signal weights, tier map, and per-harness model choice |
| [reference/harness-matrix.md](reference/harness-matrix.md) | Plan compatibility (harness x plan x rewrite honored) |
| [reference/scores.md](reference/scores.md) | Public capability scores: sources, licences, cache, refresh, privacy, shadow selection |
| [reference/stats-export.md](reference/stats-export.md) | Sharing stats safely |
| [reference/beta-exit.md](reference/beta-exit.md) | Beta exit task map |
| [reference/publishing-boundaries.md](reference/publishing-boundaries.md) | Public vs internal vs private docs |
| [reference/content-classification.md](reference/content-classification.md) | A/B/C/D content map (adoption → secret) |
| [reference/relicense.md](reference/relicense.md) | Relicense to Apache 2.0 (2026-10-06) |

## Design

| Doc | Topic |
|-----|--------|
| [design/architecture.md](design/architecture.md) | Architecture overview, BDI agentic diagram, packaging layers (memory, skills, hooks, agents, plugin) |
| [img/bdi-agentic-light.svg](img/bdi-agentic-light.svg) | BDI loop (beliefs / desires / intentions) — also in [README](../README.md#bdi-agentic-view-beliefs--intentions) |
| [design/engineering-loop.md](design/engineering-loop.md) | Feedback loop |
| [design/minilm-semantic.md](design/minilm-semantic.md) | Semantic boost (in-process word-hash embedder) |
| [design/mod-router.md](design/mod-router.md) | Can Downshift become a Claude Code mod? Design and the opt-in prototype's live runs |
| [internal/tasks/2026-10-16-multilingual-semantic-intelligence/README.md](internal/tasks/2026-10-16-multilingual-semantic-intelligence/README.md) | Archived multilingual E5 initiative (removed from product) |
| [design/classifier-shadow.md](design/classifier-shadow.md) | Shadow classifier |
| [design/context-optimization.md](design/context-optimization.md) | Native Context Compressor & Context Sensor |
| [design/router-generalization.md](design/router-generalization.md) | Eval methodology and holdout discipline |
| [design/capability-router-v2.md](design/capability-router-v2.md) | Router v2 (experimental; short) |
| [design/decision-intelligence.md](design/decision-intelligence.md) | Advisory layer |
| [design/langgraph-orchestration.md](design/langgraph-orchestration.md) | Optional planner |

## Contributors

| Doc | Topic |
|-----|--------|
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | Misroutes, layout, tests |
| [contrib/classifier.md](contrib/classifier.md) | v1 classifier tuning (signals, margin, shadow) |
| [contrib/layering.md](contrib/layering.md) | Layering rule: every feature serves every harness (checklists, worked examples) |
| [contrib/agent-reference.md](contrib/agent-reference.md) | On-demand detail for agents: compressor, telemetry, catalog, honor/usage seam |
| [contrib/prompt-corpus.md](contrib/prompt-corpus.md) | Prompt corpus |
| [../benchmark/README.md](../benchmark/README.md) | Benchmark splits |
| [evidence/](evidence/) | Dated runtime evidence (rewrite honored, audits) |
| [evidence/e5-int8-evaluation-2026-10-17.md](evidence/e5-int8-evaluation-2026-10-17.md) | Multilingual E5 as a routing candidate: baseline measured, E5 gain unproven, fuse stays off |
| [evidence/multilang-codex-2026-10-16.md](evidence/multilang-codex-2026-10-16.md) | Multilingual checks through the Codex adapter (wire response and telemetry only) |
| [evidence/baseline-freeze-2026-10-17.md](evidence/baseline-freeze-2026-10-17.md) | Baseline freeze manifest: frozen eval sets, sha256, reference numbers the next run must reproduce, how to verify |
| [evidence/plugin-mods-smoke-2026-10-17.md](evidence/plugin-mods-smoke-2026-10-17.md) | Plugin and mods under headless Claude Code 2.1.302: load, routing decisions, double routing with the router mod (n=1 per round) |

## Brand

| Doc | Topic |
|-----|--------|
| [brand/rename.md](brand/rename.md) | Harness → Downshift rename (done) |
| [brand/readme-hero.md](brand/readme-hero.md) | README hero drafts |

## Internal (maintainers)

Not required for install. See [reference/publishing-boundaries.md](reference/publishing-boundaries.md).

| Doc | Topic |
|-----|--------|
| [internal/README.md](internal/README.md) | Index |
| [internal/launch-waves.md](internal/launch-waves.md) | Launch execution checklist |
| [internal/backlog.md](internal/backlog.md) | Maintainer backlog (stub) |
| [internal/next-steps.md](internal/next-steps.md) | Maintainer checklist (stub) |
| [internal/gap-tasks.md](internal/gap-tasks.md) | Runtime and evidence task queue |
| [internal/tasks/](internal/tasks/) | Dated task waves |
| [internal/engineering-loop-maintainer.md](internal/engineering-loop-maintainer.md) | Promotion / dsmon appendix |
