# Documentation index

Docs are grouped by audience: **guides/** (how-tos), **reference/** (specs and
contracts), **design/** (architecture and design notes), **evidence/** (dated
runtime proof) and **internal/** (maintainer planning). Markdown file names
under `docs/` are lowercase kebab-case; only names a tool requires (`README.md`,
`SKILL.md`, `AGENTS.md`) keep their exact casing.

## Start here

| Doc | For |
|-----|-----|
| **[guides/install.md](guides/install.md)** | Install binary, hooks per harness, troubleshooting (humans + agents) |
| [design/architecture.md](design/architecture.md) | What Downshift is and how hooks flow |
| [guides/when-to-use.md](guides/when-to-use.md) | Downshift vs gateways and direct SDK |
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
| [../skills/downshift-audit/SKILL.md](../skills/downshift-audit/SKILL.md) | Read-only agent skill: audit hook wiring, stats and fixed context size |

## Reference

| Doc | Topic |
|-----|--------|
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
| [design/architecture.md](design/architecture.md) | Architecture overview |
| [design/engineering-loop.md](design/engineering-loop.md) | Feedback loop |
| [design/minilm-semantic.md](design/minilm-semantic.md) | Semantic boost |
| [design/classifier-shadow.md](design/classifier-shadow.md) | Shadow classifier |
| [design/context-optimization.md](design/context-optimization.md) | Native Context Compressor & Context Sensor |
| [design/router-generalization.md](design/router-generalization.md) | Eval methodology and holdout discipline |
| [design/capability-router-v2.md](design/capability-router-v2.md) | Router v2 (experimental; short) |
| [design/decision-intelligence.md](design/decision-intelligence.md) | Advisory layer |
| [design/graphify-integration.md](design/graphify-integration.md) | Graphify as a library, not hook escalation |
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
