<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Reconciling Anthropic references, benchmark claims, and Downshift empirical evidence (2026-11-07)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

What this document establishes: a source-to-claim audit comparing official Anthropic documentation and published routing
research against Downshift's empirical runtime measurements, resolving Issue #27 (parent spike #21, P1).

It clarifies the boundary between vendor-published benchmark results (such as large defect search and research multi-agent
evaluations) and Downshift's own empirical observations across 46 runs on Claude Code, KiroCrew real-use trials, and the
CCR compactor corpus. It addresses cache pricing nuances, parent token overhead, subscription realities, subagent
completion versus functional task acceptance, and links existing repository gaps ([GAP-SUITE](evaluation-suite-inventory.md),
[GAP-CHILD-FAILURE](billing-gap-audit.md), [GAP-WORKFLOW-AGENTS](workflow-agents-bypass-pretooluse-2026-10-11.md)).

Downshift is an independent open-source tool. Anthropic has not validated, tested, or endorsed Downshift.

---

## 1. Executive context and core principles

Vendor documentation and academic research frequently present multi-agent orchestration and dynamic model routing as
universal levers for cost reduction. However, published vendor gains often arise from specialized workloads (e.g.,
broad, read-only search across massive repositories) that differ fundamentally from everyday software engineering.

Under owner decisions O1 (no dollar accounts in-product) and O8 (orchestrator chooses; Downshift applies within declared
policy deterministically), Downshift evaluates claims under four strict principles:

1. **Vendor claims remain scoped to vendor workloads.** A finding on research tasks or defect search does not establish
   savings on coding tasks with shared files, build gates, or AST modifications.
2. **Harness honesty.** Downshift distinguishes what the hook emits (`rewrite_emitted`), what the harness executor
   actually honors (`resolved` / `rewrite_honored`), and what the child subagent executes.
3. **No tokens without acceptance.** Subagents reporting `completed` provide no economic value if their code fails
   functional verification. A cheaper model that breaks a test forces expensive lead-session rework that eliminates
   all financial gains.
4. **Separation of levers.** Context compaction (cutting tool stdout/stderr) and model delegation (routing subagents)
   act on different token streams; their savings compose multiplicatively, but their evidence must never be conflated.

---

## 2. Complete source-to-claim matrix

The matrix below reconciles external vendor claims and research literature against Downshift's empirical measurements.

### 2.1 External vendor guidance and published router research

| Claim / Source | Exact workload | Comparator / Baseline | Sample size ($n$) | Date & version | Metric measured | Source URL | Adoption status in Downshift |
|---|---|---|--:|---|---|---|---|
| **Anthropic Orchestrator Strategy: Delegate Bulk Work** | Multi-agent defect search across large codebases | Single Opus agent vs Opus lead + Sonnet/Haiku workers | Undisclosed vendor test set | 2025–2026 | ~33% cost reduction on large defect search | [Optimizing for cost and intelligence](https://platform.claude.com/docs/en/about-claude/models/optimizing-for-cost-and-intelligence#orchestrator-strategy-delegate-bulk-work) | **Scoped / Not generalizable.** Defect search is read-only exploration across thousands of files. Everyday coding involves tight package dependencies and edit conflicts where coordination overhead wipes out savings. |
| **Anthropic Multi-Agent Research System** | Open-ended research queries and synthesis | Single Opus 4 agent vs Opus 4 lead + Sonnet 4 subagents | Internal research benchmark | 2025-06-13 | +90.2% performance improvement; ~15× higher token consumption | [Building effective agents](https://www.anthropic.com/engineering/multi-agent-research-system) | **Adopted as architectural principle P4 & P6.** Confirms strong lead + cheap worker topology, but warns that multi-agent systems inflate token volume by 15×. Not applicable to standard code edits. |
| **Anthropic Emerging Multiagent Systems** | Security vulnerability discovery on 15 open-source repositories | Independent parallel agents vs coordinating swarm | 45 agents; 15 projects; 6.5M vs 27M tokens | 2026-08-13 (Claude Mythos Preview) | 21 vs 266 findings; comparable tokens per finding inside assigned directories | [Multiagent systems](https://www.anthropic.com/research/multiagent-systems) | **Adopted as principle P2 & P3.** Swarms expand search scope but do not make individual tokens more productive; shared dependency collaboration degraded sharply. |
| **Anthropic Riemann Zeta Study** | High-level mathematical discovery | Single-agent vs 60 subagents | 1 case study (60 subagents) | 2026-08-10 | Agent specialization: 2 core, 13 ideas, 30 failed, 13 validators, 2 writers | [Riemann zeta](https://www.anthropic.com/research/riemann-zeta) | **Adopted as principle P3 & P5.** 50% of subagents added no value; verification agents are necessary to prevent invalid drift. |
| **Anthropic Alignment Science** | Multi-agent organizational safety and alignment | Single agent vs multi-agent squads | 12 tasks (10 consultancy, 2 software team) | 2026-04 (arXiv 2604.10290) | Business effectiveness vs ethics / alignment degradation | [AI organizations](https://alignment.anthropic.com/2026/ai-organizations/) | **Adopted as principle P8.** Multi-agent groups can amplify misaligned actions; flow-level guardrails, worktree isolation, and human review before merge are required. |
| **Anthropic Claude Code Model Config** | Subagent execution under native Claude Code | Monolithic Opus session vs native `opusplan` | Default CLI behavior | Claude Code v2.1.292+ | Opus plans in plan mode, Sonnet executes subagents | [Claude Code model config](https://code.claude.com/docs/en/model-config#opusplan-model-setting) | **Integrated as baseline comparator.** Downshift O8 sits on top of `opusplan`, routing subagents further down to Haiku or maintaining Sonnet based on policy. |
| **Anthropic Managed Agents Platform** | Server-side cloud orchestration | Hosted session threads vs subagent sandboxes | Cloud platform API | 2026-10-01 (header `managed-agents-2026-04-01`) | Subagent threads, dynamic workflows, advisor turns | [Managed agents](https://platform.claude.com/docs/en/managed-agents/multiagent-orchestration) | **Not applicable to local hooks.** Managed Agents runs entirely server-side; local PreToolUse hooks never execute on platform threads. |
| **RouteLLM (LM-SYS)** | Preference-trained strong/weak model request routing | Fixed GPT-4 vs router (GPT-4 + Mixtral/Claude) | MT-Bench, Chatbot Arena | 2024–2026 | >85% cost reduction at 95% GPT-4 quality on MT-Bench | [RouteLLM](https://github.com/lm-sys/RouteLLM) | **Reference only.** Request-level chatbot routing does not capture agentic coding context, tool interactions, or multi-turn context accumulation. |
| **RouterBench / LLMRouterBench** | Router evaluation frameworks | Precomputed model responses | 21 datasets, 33 models | 2024–2026 | Router accuracy vs simple/random baselines | [LLMRouterBench](https://aclanthology.org/2026.findings-acl.1881/) | **Reference only.** Confirms that wielu complex routers fail to beat simple deterministic baselines. Downshift uses deterministic policy, not ML inference. |
| **SWE-Router** | Coding agent trajectory escalation | Small model start vs escalation to frontier | SWE-bench lite | 2026-07 (arXiv 2607.00053) | Dynamic escalation after initial agent trajectory | [SWE-Router](https://arxiv.org/abs/2607.00053) | **Research comparison.** SWE-Router acts mid-trajectory; Downshift O8 acts deterministically at subagent spawn boundaries (`PreToolUse`). |

---

### 2.2 Downshift empirical benchmark measurements

| Measurement / Workload | Exact workload | Comparator / Baseline | Sample size ($n$) | Date & version | Metric measured | Empirical result | Primary evidence artifact |
|---|---|---|--:|---|---|---|---|
| **Four-Arm Mechanical Battery** | 4 mechanical refactoring tasks (rename, doc comments, constants, test mirror; 5 pieces each) | Arm A (Opus fixed), Arm B (native `opusplan`), Arm C (`opusplan` + Downshift routing), Arm D (C + compactor) | 46 live runs (stopped at 46 of 48) | 2026-10-21 (main `13f1c91d`) | Total tokens (input, output, cache read/write) per accepted task; 95% bootstrap CI | **24.1% fewer tokens** per accepted run (B vs C, CI +2.1% to +46.7%); 16.5% per run (CI +0.6% to +29.7%); 12/12 acceptance. Cache accounted for >98% of tokens. | [four-arm-mechanical-2026-10-21.md](four-arm-mechanical-2026-10-21.md) |
| **KiroCrew Real-Use Trial** | Full coding session combining subagent routing and shell advice compaction | Native KiroCrew execution without Downshift | 6 control tasks, one multi-hour session | 2026-10-21 | Combined total token consumption per task | **Up to ~53% fewer tokens** per task (directional, combined routing + compactor). | [roadmap.md](../../docs/roadmap.md#now-beta) (line 68), [kirocrew-routing-guide.md](../guides/kirocrew-routing-guide.md) |
| **CCR Compactor Corpus Benchmark** | Synthetic corpus across 53 CLI command families (test runners, git, build tools, linters) | Raw tool command output vs compacted output | 107 fixtures | Current (`make bench`) | Byte reduction and approximate token reduction (bytes/4) | **51.1% raw byte reduction** without CCR; **72.0% byte reduction with CCR** (folding runs of 8+ repetitive lines); 65.2% on lossless families with zero diagnostic loss. | [bench.md](../reference/bench.md), [compaction-corpus-2026-10-21.md](compaction-corpus-2026-10-21.md) |
| **Batch b5 Mechanical A/B** | 4 mechanical tasks, 1 subagent per piece (at least 3 spawns/run) | Unrouted control arm (`DOWNSHIFT_NO_ROUTE=1`) vs routed arm | 24 runs (12 routed, 12 control) | 2026-10-15 | Claude Code `total_cost_usd` list price; acceptance check | **12% cost reduction** ($0.59 vs $0.67); **33% reduction on pure renames** ($0.49 vs $0.74); 19 of 56 spawns downshifted; 12/12 accepted. | [subagent-routing-ab-2026-10-15.md](subagent-routing-ab-2026-10-15.md) |
| **Development Outcome Suite** | 40 Go coding tasks across 4 tiers (10 TRIVIAL, 10 SIMPLE, 10 MEDIUM, 10 COMPLEX) | Small (`qwen3-coder-30b`), Mid (`deepseek-v4-flash`), Frontier (`claude-opus-5.5`) | 40 tasks × 3 models | 2026-10-03 | Functional pass rate via hidden `check_test.go` | **Small: 27/40 (67.5%)**, **Mid: 31/40 (77.5%)**, **Frontier: 40/40 (100.0%)**. On TRIVIAL + SIMPLE, small fails 4 of 20 (**−20.0pp gap**). | [evaluation-suite-inventory.md](evaluation-suite-inventory.md), [baseline-freeze-2026-10-17.md](baseline-freeze-2026-10-17.md) |
| **Held-out Suite (GAP-SUITE Smoke)** | 6 independent Go coding tasks outside development suite | Small, Mid, Frontier models | 6 tasks × 3 models | 2026-10-11 | Functional correctness check | **Small: 2/6 (33.3%)**, **Mid: 4/6 (66.7%)**, **Frontier: 6/6 (100.0%)**. Confirms substantial capability gap on unseen code. | [evaluation-suite-inventory.md](evaluation-suite-inventory.md), [gap-tasks.md](../internal/gap-tasks.md) |
| **Real Usage 30-Day Telemetry** | Natural everyday coding, copy editing, mod styling, and audits | Same operator, mixed harnesses | 1,175 subagent decisions | 2026-10-19 | Rewrite verdicts, guardrail holds, normalized cost delta | **16.4% downshifted, 4.7% upshifted, 19.7% held by guardrail**. Normalized units: **−1.9%** (inside noise). Everyday savings **not established**. | [real-usage-2026-10-19.md](real-usage-2026-10-19.md) |

---

## 3. Resolving stale, contradictory, and conflated claims

### 3.1 Harness and compaction reality versus canonical matrix

Historical documentation and external commentary have occasionally stated that *"only Claude Code rewrites commands
automatically"*. This statement is stale and contradicted by canonical implementation in
[`docs/reference/harness-matrix.md`](../reference/harness-matrix.md) and [`internal/core/capabilities.go`](../../internal/core/capabilities.go):

1. **Universal Direct Compaction:** Running `downshift compact -- <cmd>` works universally in **every terminal and harness**
   (including unsupported or experimental environments) because it executes as an ordinary CLI wrapper.
2. **Automatic In-Place Command Rewriting (`CanRewriteShellCommand: true`):**
   - **Claude Code:** Supported out of the box via `PreToolUse` on `Bash`.
   - **OpenCode:** Supported automatically via the bridge plugin (`~/.config/opencode/plugins/downshift/`).
   - **Cursor, Codex, Antigravity:** Supported via their respective hook adapters for POSIX shells, constrained by
     read-only safety rules (`UnknownUserRules: true` or `RewriteImpliesApproval: true`).
3. **Command Advice (`CanAdviseShellWrap: true`):**
   - **KiroCrew:** Does not support in-place string rewriting on shell calls; instead, its native hook exits with code 2
     and provides the compacted command suggestion via stderr for agent retry.

### 3.2 "Routed only" reporting versus end-to-end task completion

A critical distinction exists between telemetry reporting on routed subagents and measuring end-to-end task savings:
- **`outcome: "rewrite_emitted"`:** Confirms that the hook emitted a model modification string. It does not prove the harness executed it.
- **`rewrite_honored: true`:** Confirms the harness executor launched the child under the requested model tier (verified on Claude Code via API-reported model in child transcripts; inferred on Codex; unconfirmed on Cursor).
- **`child_ended: "completed"`:** Confirms that the subagent process terminated cleanly (exit 0). It does **not** evaluate whether the generated code compiles, passes tests, or satisfies developer intent.
- **`downshift report` scope:** Measures routed tokens per completed task and rework rate (attempts redone within 10 minutes) for routed spawns. It intentionally emits no dollar amount, does not judge code quality, and has no unrouted control arm since 2026-11-02.

### 3.3 Decomposing routing and compaction

The figure of *"up to 53% token savings"* observed on KiroCrew combined two distinct mechanisms operating on different surfaces:
- **Routing:** Reduced subagent model execution cost when delegating mechanical tasks.
- **Compaction:** Reduced verbose test runner stdout from entering the session context.

Because compaction reduces parent context while routing changes worker model rates, their savings compose multiplicatively:

$$\text{Spend}_{\text{combined}} = \text{Spend}_{\text{baseline}} \times (1 - r) \times (1 - c)$$

Where $r$ is the effective routing savings ratio and $c$ is the context compaction reduction ratio. They must never be
reported as a single routing achievement. Isolated routing on mechanical tasks demonstrated **24.1% token savings**
([four-arm battery](four-arm-mechanical-2026-10-21.md)), while CCR compaction demonstrated **72.0% byte reduction**
([bench.md](../reference/bench.md)).

---

## 4. Prompt caching, parent tokens, and subscription accounting

### 4.1 The cache read dominance and pricing paradox

In modern coding agent sessions, prompt cache reads represent **over 98% of total token volume** (observed across all
arms in [`four-arm-mechanical-2026-10-21.md`](four-arm-mechanical-2026-10-21.md)).

When evaluating model downshifts using official Anthropic API list prices (`catalog.json` v2):

| Model | Input (uncached) | Output | Cache read | Cache write |
|---|--:|--:|--:|--:|
| `claude-haiku-4-5` (small) | $1.00 / M | $5.00 / M | **$0.10 / M** | $1.25 / M |
| `claude-sonnet-5-5` (mid) | $2.00 / M | $10.00 / M | **$0.10 / M** | $2.50 / M |
| `claude-opus-5-5` (frontier) | $4.00 / M | $20.00 / M | **$0.20 / M** | $5.00 / M |

**The Haiku vs Sonnet cache paradox:**
- On uncached input and output tokens, Haiku is 50% cheaper than Sonnet.
- On cache reads, **Haiku and Sonnet have the exact same price ($0.10 per 1M tokens)**.
- Because subagent spawns initialize with massive cached system prompts and tool schemas (50k–100k tokens) and generate
  only 200–500 output tokens, downshifting a child from Sonnet to Haiku produces **virtually zero financial savings on
  cache reads**.
- Significant financial savings on cache reads occur **only when downshifting from Opus ($0.20/M) to Sonnet or Haiku ($0.10/M)**,
  cutting cache read spend by 50%.

### 4.2 Estimated API prices versus subscription plans

Downshift's telemetry records tokens, not currency. Converting tokens to dollars via list prices creates substantial
discrepancies with user reality:
1. **Flat-Rate Subscriptions (Pro, Team, Max, Cursor Pro/Ultra):**
   - Users pay a fixed monthly subscription fee. They receive no per-token bill or invoice discount.
   - Throttling occurs against rolling 5-hour and weekly usage limits.
   - What Downshift preserves on subscription plans is **quota runway and session headroom**, delaying rate-limit lockouts.
2. **API Pay-As-You-Go Billing:**
   - Invoices include minimum billing blocks, cache write premiums, and parent session tokens.
   - Downshift hooks see only subagent spawns and executed commands; they cannot observe the parent session's internal
     reasoning turns or conversation history accumulation.

### 4.3 Subagent completion versus task acceptance

In automated benchmarks, subagent "completion" must never be conflated with user "acceptance":
- When an orchestrator delegates to a small worker, that worker frequently exits with status `completed` while introducing
  subtle defects, incomplete logic, or invalid assumptions.
- Per [`evaluation-suite-inventory.md`](evaluation-suite-inventory.md), the small tier fails **32.5% of development coding
  tasks** (and 20.0% of TRIVIAL + SIMPLE tasks).
- If the child fails, the parent agent must spend expensive frontier tokens diagnosing the failure and repairing the code.
  A single repair turn in the lead session erases the marginal savings of 10–20 cheap subagent spawns.

---

## 5. Repository gaps and structural boundaries

Downshift explicitly tracks open architectural boundaries and technical gaps:

1. **[GAP-SUITE](evaluation-suite-inventory.md) (Evaluation Independence):**
   The 40 development outcome tasks are frozen, but held-out validation currently relies on a 6-task smoke subset.
   Expanding the held-out suite to 20+ tasks is required before claiming broad tier accuracy on unseen coding domains.
2. **[GAP-CHILD-FAILURE](billing-gap-audit.md) (Rework Telemetry Undercounting):**
   Harness hook payloads on Claude Code and other platforms do not deliver the `PostToolUseFailure` error payload to
   external hooks. Downshift detects child failure through `SubagentStop` exit status, but cannot inspect failure stack
   traces directly.
3. **[GAP-WORKFLOW-AGENTS](workflow-agents-bypass-pretooluse-2026-10-11.md) (Local Workflow Bypass):**
   Spawns created by local Claude Code `Workflow` scripts bypass `PreToolUse` hooks entirely, triggering only
   `SubagentStart` without prompt or model rewrite channels. Users must set `agent({model: ...})` directly inside
   workflow scripts.
4. **[GAP-COORDINATION](delegation-boundaries-and-overhead-2026-11-07.md) (Parent Turn Overhead):**
   Unmeasured token consumption during orchestrator briefing and synthesis phases. Spawning subagents for trivial or
   serial tasks increases total tokens compared to direct inline execution by the lead agent.

---

## 6. What remains unknown

To preserve scientific rigor, Downshift explicitly documents what has not been measured:
- **Everyday coding savings:** Downshift has not proven token savings in natural, unconstrained everyday developer workflows.
  The 30-day telemetry log showed a net normalized saving of −1.9% (within noise).
- **Subscription limit translation:** It is unknown how many tokens saved in subagents translate into additional hours of
  work under Anthropic's proprietary subscription quota algorithms.
- **Cross-provider live coding:** Downshift catalog supports cross-provider IDs, but live automated routing across
  different provider APIs during an active coding session has not been benchmarked for latency and failure rates.

---

## Related documentation

- [True north: foundational principles and source checks](../true-north.md)
- [Evidence ladder: what each saving figure means](../guides/evidence.md)
- [Compaction benchmark report](../reference/bench.md)
- [Canonical harness and capabilities matrix](../reference/harness-matrix.md)
- [Evaluation suite inventory (Issue #23)](evaluation-suite-inventory.md)
- [Delegation boundaries and coordination overhead (Issue #24)](delegation-boundaries-and-overhead-2026-11-07.md)
- [Claude Code execution and workflow reconciliation (Issue #26)](claude-execution-workflow-reconciliation-2026-11-07.md)
- [Four-arm mechanical battery (2026-10-21)](four-arm-mechanical-2026-10-21.md)
- [Post-O8 benchmark smoke protocol (Issue #22)](post-o8-benchmark-protocol-2026-11-07.md)
