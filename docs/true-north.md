<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# True north

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Status: owner direction, 2026-10-29. This page says what Downshift is for, which published studies the
direction rests on, what each study does and does not show, and what our own data proves so far. Where a
study could not be read at its primary source, the page says so in the source list (section 8).

## 1. The north in one paragraph

**The orchestrator LLM decides; Downshift applies the decision inside the user's policy, keeps the context
small, and measures the result as cost per accepted task.** More intelligence goes where planning and
judgment are, cheap models go where the work is bounded and independent, and a claim of savings is made only
against a baseline that compares accepted work, not raw tokens.

Downshift's own code never calls a model (owner decision O8, 2026-10-28). What stays deterministic is the
part that must be: applying `policy.json`, refusing unknown or `explicit_only` model ids, keeping the
requested model in doubt, compacting command output and counting tokens.

## 2. Principles

| # | Principle | Where it lives |
|---|---|---|
| P1 | The LLM chooses the agent, model and effort. Downshift applies the choice within the declared policy; a built-in table never overrides it. | Decider, `providers` and `apply` MCP tools, orchestrator playbook |
| P2 | Delegate only work that is independent. Parallel agents on shared dependencies lose. | `delegate` description, playbook |
| P3 | Few workers first. A small spawn budget, raised only when an evaluation justifies it. | Worker queue (limit 4 running, 8 tasks per call), plan-runner budget admission (P6 card) |
| P4 | A strong lead for planning, decisions and synthesis; cheap workers for bounded tasks. | `[tier=small]` hint, small-tier target in the catalog |
| P5 | Nothing counts as done until the acceptance checks pass. | Outcome suite, `report` rework column, worker acceptance (plan runner) |
| P6 | Cost is measured per accepted task, never as tokens saved alone. | `downshift report`, the V2 baseline |
| P7 | Work that is deterministic runs outside the model's context, and context is compacted. | Compactor (`downshift compact`), DCI |
| P8 | Guardrails on one agent do not cover a group of agents. The flow needs its own gates. | Policy floors, worktrees for write workers, review before merge |

## 3. What the studies say

Every row says what was measured, in the authors' words where they could be read, and what it does not
show. Verification status is in section 8.

### 3.1 A lead that is stronger than its workers pays off on research, at a high token price

Anthropic, *How we built our multi-agent research system* (2025-06-13): a lead Claude Opus 4 with Claude
Sonnet 4 subagents "outperformed single-agent Claude Opus 4 by 90.2%" on Anthropic's internal research
eval, and multi-agent systems "use about 15× more tokens than chats".

- Supports: P4 (strong lead, cheaper workers) and P6 (the token price is part of the result).
- Limits: an internal eval on research tasks. The same post says coding is a weaker fit because "most coding
  tasks involve fewer truly parallelizable tasks than research", and that multi-agent work only pays when the
  task is valuable enough to cover the extra tokens. Do not carry the 90.2% over to programming.

### 3.2 The lead's intelligence matters more than the worker's

Claude Opus 4.5 system card, orchestration experiment on an internal search benchmark (as transcribed from
a third-party summary; see section 8 for the verification gap):

| Workers \ Lead | Sonnet 4.5 lead | Opus 4.5 lead |
|---|--:|--:|
| Single agent | 42.0% | 74.8% |
| Haiku 4.5 subagents | 66.2% | 87.0% |
| Sonnet 4.5 subagents | 66.5% | 85.4% |
| Opus 4.5 subagents | 81.6% | 92.3% |

- Reading, if the figures hold: with Sonnet workers, changing the lead from Sonnet to Opus moves the result
  from 66.5% to 85.4% (+18.9 points); Opus leading Haiku workers (87.0%) beat Opus alone (74.8%); the best
  result used Opus workers (92.3%), at a higher cost.
- Supports: P4 and the idea that routing should look at the planning step, not only at the implementation.
- Limits: one internal benchmark, search not programming, models that are now older. Haiku (87.0%) ahead of
  Sonnet (85.4%) shows noise, not a ranking of worker models. The table says nothing about cost.

### 3.3 More agents do not mean more efficiency

Anthropic, *Patterns and problems in emerging multiagent systems* (2026-08-13): 45 agents on 15 open-source
projects. With Claude Mythos Preview, independent parallel agents found 21 vulnerabilities in a 6.5 million
token run; a coordinating swarm found 266 in a 27 million token run. The authors point out that roughly half
of the swarm's findings were outside the directories the independent agents were assigned, and that inside
those directories the two methods "seem comparable in terms of tokens per vulnerability found". Only 12
vulnerabilities were found by both.

- Supports: P2 and P3. Coordination widens what is explored; it did not make each token more productive.
- Limits: security search, one model family, search scopes that differed between the arms. The post also
  reports difficulty collaborating on software projects with shared dependencies.

### 3.4 Many subagents, few contributors

Anthropic, *Riemann zeta* mathematics research (2026-08-10, updated 2026-08-13): of 60 subagents, 2
developed the key ideas, 13 contributed ideas to them, 30 tried and failed to develop new ones, 13 validated
the arguments and 2 helped write the first paper.

- Supports: P3 and P5. Specialisation and verification helped; half the agents added no result.
- Limits: a single case study in mathematics.

### 3.5 Teams of agents can be more effective and less aligned

Anthropic Alignment Science, *AI Organizations Can Be More Effective but Less Aligned* (arXiv 2604.10290;
the page shows no date, so April 2026 is an inference from the identifier): on 12 tasks (10 consultancy
scenarios built from real enforcement actions and 2 software-team tasks), groups of agents scored higher on
the business goal and lower on ethics than single agents. The gap depended strongly on the model; the model
trained and tested for agentic safety showed much smaller gaps on the consultancy tasks.

- Supports: P8. Checks on one agent do not certify a group; the gates sit around the flow.
- Limits: simulated scenarios, a small task count, results that vary by model and configuration.

### 3.6 Choose the model and the effort by task, and delegate independent work

OpenAI, *A model guide for the GPT-6 family* (2026-10-02, per secondary coverage): model choice and
reasoning effort are a trade between intelligence and price; low effort fits routine work such as small
edits; the API can change effort mid-conversation without rewriting the prompt prefix, which keeps the cache;
independent parts of a request can proceed while a tool runs. The API's native multi-agent mode uses the
same model for the subagents, so a strong lead with cheap workers needs per-agent configuration in the
Agents SDK or a layer of the caller's own.

- Supports: P1 and P4. The same idea applies outside Claude: let the caller pick model and effort per step.
- Limits: guidance, not an experiment. We could not read the page itself (section 8).

### 3.7 Compaction and determinism outside the model

OpenAI, *The builder's guide to GPT-5.6*: keep reasoning and compact context, parallelise only when needed,
and run deterministic work outside the model's context. On ARC-AGI-3, enabling retained reasoning and
compaction moved one model's score from 13.3% to 38.3% with roughly six times fewer output tokens.

- Supports: P7 as a direction. Context kept small is a real lever.
- Limits: the gain is a model-plus-harness result on one benchmark, and OpenAI reports it; it is not a result
  of multi-agent orchestration. It is also a different mechanism from ours: Downshift compacts the output of
  shell commands, not the model's reasoning. It does not validate our compaction numbers.

### 3.8 Patterns already in the design

The routing design was built on Anthropic's published orchestration patterns: orchestrator-workers, async
multi-agent orchestration, *Plan big, execute small*, dynamic workflows and the coordination-patterns article.
They are listed with links and with what each one changed in [routing v2](design/routing-v2.md), section 11.

## 4. What the studies add up to

- A stronger lead is worth its cost when the work splits into independent parts (3.1, 3.2). It is not a
  general result for coding (3.1).
- Adding agents has diminishing and sometimes negative returns (3.3, 3.4). The first workers matter most.
- Verification and acceptance are where agent teams earn their keep (3.4) and where they fail silently
  (3.5).
- Smaller context and work done outside the model are separate levers from orchestration (3.7).
- No study measures Downshift. They justify the questions below; the answers come from our own runs.

## 5. What our own data shows today

| Claim | State | Record |
|---|---|---|
| The hook applies the orchestrator's model and tier hint on real Claude Code spawns | Observed. Rewrite honoured 10 of 10; the `[tier=small]` hint moved two real spawns (2026-10-28) | [rewrite honoured](evidence/claude-code-rewrite-honored-2026-10-06.md), [unprompted runs](evidence/mcp-unprompted-2026-10-29.md) |
| Routing to a smaller tier lowers tokens on mechanical delegated work | Measured once, before the O8 change (classifier-driven routing): 24.1% fewer tokens per accepted run against `opusplan`, interval +2.1% to +46.7%, acceptance 12 of 12. Forced delegation, one operator, Claude Code only, token counts | [four-arm battery](evidence/four-arm-mechanical-2026-10-21.md) |
| Routing saves tokens in everyday use | **Not established.** `downshift report` flips sign between a 30-day and a 7-day window | [MVP closing state](internal/tasks/2026-10-27-mvp/README.md#closing-state-2026-10-29) |
| The Compactor cuts command output | Measured in bytes on a synthetic corpus (82.3% on files it compacted, 23.7% overall) and on real use (186 of 2,491 commands, about 12k tokens in 30 days, bytes / 4). Not LLM tokens measured on a controlled suite | [corpus record](evidence/compaction-corpus-2026-10-21.md) |
| The orchestrator calls the MCP tools unprompted | **Not observed** in two runs; it did add the hint by itself | [unprompted runs](evidence/mcp-unprompted-2026-10-29.md) |
| Quality holds when work moves to a smaller tier | Checks passed in the four-arm battery; the 40-task outcome suite has recorded runs only for non-Claude models | `downshift-labs` outcome suite |

The honest summary: the mechanism works and is measured; the claim that it saves tokens on real work is
not proven.

## 6. The experiment that decides it

Proposal, not a result. It is the V2 baseline of the [MVP plan](internal/plans/mvp-plan.md), widened to
cover orchestration.

| Arm | Question |
|---|---|
| Cheap model alone | Cost baseline |
| Strong model alone | Quality baseline |
| Strong lead + cheap workers | Does delegation save money without losing quality? |
| Strong lead + strong workers | What is the ceiling, and what does it cost? |

- Tasks: 40 real programming tasks with checks that fail before and pass after (the outcome suite shape).
- Primary metric: **cost per accepted task** = total cost of all runs ÷ tasks completed correctly. Because
  Downshift keeps no dollar account (owner decision O1), the cost here is tokens per accepted task, with
  the model tier named for each arm.
- Also recorded: first-pass acceptance, latency, tokens by kind, retries, human rework and agents that added
  nothing.
- A result is reported with its interval. A regression on accepted tasks blocks the claim, whatever the
  average says.
- Cost: model tokens. The owner approves the budget before any run.

## 7. Hypotheses this opens for the roadmap

All four are hypotheses. None is built or proven for Downshift.

1. **Intelligence-aware routing.** The playbook asks how much intelligence the plan needs, not only how
   hard the implementation is. A cheap model may do the work; the plan and the synthesis may need a strong one.
2. **Spawn budget.** Start with two or three workers when tasks are truly independent. The queue allows four
   running today; whether fewer is better is a question for the experiment above.
3. **Tier separation.** Strong models for planning, architecture and merging; cheap models for bounded tasks
   with a clear brief.
4. **Quality gates.** Tests, security checks and acceptance criteria run before work counts as done, and a
   group of agents is gated as a group (3.5).

If the experiment supports them, they become cards. If it does not, they are dropped.

## 8. Sources and what was verified

"Read" means the primary page was fetched and the figures in this page were checked against it on
2026-10-29. "Secondary" means the primary page could not be read here and the figures were confirmed only
through search summaries or third-party write-ups. "Not checked" means we have not confirmed it.

| Source | Date | Status |
|---|---|---|
| [How we built our multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system), Anthropic | 2025-06-13 | Read. The 90.2%, 15× and coding caveat match |
| [Claude Opus 4.5 system card](https://assets.anthropic.com/m/64823ba7485345a7/Claude-Opus-4-5-System-Card.pdf), section 2.7 | 2025 | **Not checked.** The PDF is 11 MB and could not be extracted here. The table in 3.2 comes from a third-party summary. Check it against the PDF before quoting it outside this repository |
| [Patterns and problems in emerging multiagent systems](https://www.anthropic.com/research/multiagent-systems), Anthropic | 2026-08-13 | Read. The 45 agents, 15 projects, 21 vs 266 findings, 6.5M vs 27M tokens and the caveats match |
| [Riemann zeta research with 60 subagents](https://www.anthropic.com/research/riemann-zeta), Anthropic | 2026-08-10 | Read. The 2, 13, 30, 13 and 2 split matches |
| [AI Organizations Can Be More Effective but Less Aligned](https://alignment.anthropic.com/2026/ai-organizations/), Anthropic Alignment Science | undated (arXiv 2604.10290) | Read. The 12 tasks and the model-dependent gap match |
| [A model guide for the GPT-6 family](https://openai.com/index/practical-guide-building-gpt-6/), OpenAI | 2026-10-02 (secondary) | Secondary. The page returned HTTP 403; the model and effort trade and async independent work appear in search excerpts |
| [The builder's guide to GPT-5.6](https://openai.com/index/builders-guide-to-gpt-5-6/), OpenAI | 2026-08-13 (secondary) | Secondary. The page returned HTTP 403; the 13.3% to 38.3% and the six-fold output reduction appear in search excerpts and third-party write-ups |
| [Orchestrator-workers](https://github.com/anthropics/claude-cookbooks/blob/main/patterns/agents/orchestrator_workers.ipynb), [async multi-agent](https://github.com/anthropics/claude-cookbooks/blob/main/patterns/agents/async_multi_agent_orchestration.ipynb), [Plan big, execute small](https://github.com/anthropics/claude-cookbooks/blob/main/managed_agents/CMA_plan_big_execute_small.ipynb), [Dynamic workflows](https://github.com/anthropics/claude-cookbooks/blob/main/claude_agent_sdk/08_Dynamic_workflows.ipynb), Anthropic cookbooks | 2026 | Used in the routing design; summarised in [routing v2](design/routing-v2.md), section 11 |
| [Multi-agent coordination patterns](https://claude.com/blog/multi-agent-coordination-patterns), Anthropic | 2026 | Used in the routing design; not re-read for this page |

Not cited on purpose: a vendor claim that a newer small model costs about 75% less per task than the
previous one. It is a claim about the vendor's own usage profile, we have not confirmed it, and Downshift
does not report money.

## 9. What we will not claim

- That a stronger lead, or more agents, always helps. Section 3 shows when it did not.
- That the research results carry over to programming. Most were measured on search, research or
  mathematics.
- Savings in money. Downshift reports tokens per harness.
- Savings from routing in everyday use, until the experiment in section 6 has a baseline.
- That compacting model reasoning and compacting command output are the same thing.
