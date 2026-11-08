<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Claude Code execution, model honor, and workflow reconciliation (2026-11-07)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

Issue #26 from parent spike #21. Reconciles official vendor documentation with observed runtime behavior,
clarifies observed vs unobserved capabilities, and maps boundaries between local Claude Code hooks, built-in
agent types, policy profiles, nested subagents, and Anthropic Managed Agents.

## Official Vendor References

1. [Claude Code Subagents](https://code.claude.com/docs/en/sub-agents)
2. [Claude Code Model Configuration](https://code.claude.com/docs/en/model-config)
3. [Anthropic Managed Agents: Multiagent Orchestration](https://platform.claude.com/docs/en/managed-agents/multiagent-orchestration)

---

## 1. Ordinary Delegated Subagents (`Agent|Task` Tool Calls)

### Observed Behavior
- **PreToolUse Hook Execution:** The `PreToolUse` hook fires reliably for ordinary delegated subagents when the hook
  matcher covers `Agent` or `Task` (observed live on 2026-10-06 and 2026-11-07 on Claude Code 2.1.319).
- **Model Rewrite Honor:** `updatedInput.model` is honored by the Claude Code harness executor when using family
  aliases: `"haiku"`, `"sonnet"`, `"opus"`, or `"fable"`. Verified live on 2026-10-06 in
  `docs/evidence/claude-code-rewrite-honored-2026-10-06.md`: a parent session running on Sonnet 5.5 had its subagent
  rewritten to `"haiku"`, and the child transcript (`tool_response.outputFile`) recorded API-reported model
  `claude-haiku-4-5-20251001` across all assistant turns.
- **Strict Family Validation:** Claude Code rejects full model IDs (such as `claude-haiku-4-5` or
  `claude-haiku-5-5`) in `updatedInput.model` with schema validation error:
  `model: expected one of "sonnet" | "opus" | "haiku" | "fable"`. A rejected rewrite fails and blocks the spawn
  rather than falling open. Downshift satisfies this via catalog `native_name` mapping (`StrictModelName: true`).
- **Sibling Fields:** Downshift preserves all sibling fields of the spawn input (`description`, `prompt`,
  `subagent_type`, `run_in_background`) in `updatedInput` (`RewritePreservesInput: true`).
- **Permission Flow:** Emitting a neutral response `{}` without `permissionDecision` defers approval to the
  user's normal permission rules while successfully applying `updatedInput.model`.

### Unobserved / Boundaries: Reasoning Effort
- **Vendor Documentation:** Vendor docs for Claude Code specify an `effort` parameter on `Agent` tool calls
  (`low`, `medium`, `high`, `xhigh`, `max`), supported per-invocation in Claude Code v2.1.292 or later.
- **Local Hook Limitation:** In standard settings-based `PreToolUse` hooks, rewriting `updatedInput.effort` or
  `updatedInput.reasoning_effort` in place without internal engine mods has not been observed honored on live builds.
  Downshift accordingly reports `CanApplyEffort: no` for Claude Code in its capability grid (`core.ClaudeCodeCaps.CanApplyEffort = false`).
- **Global Precedence:** The environment variable `CLAUDE_CODE_EFFORT_LEVEL` takes precedence over any subagent
  frontmatter or per-invocation parameter.

---

## 2. Built-in Agent Types and Policy Profiles

### Built-in Types Architecture
Claude Code defines several built-in subagent types with specialized behaviors:
- **`Explore`:** Fast, read-only agent for file discovery and codebase search. Write and Edit tools are denied.
  Starts without `CLAUDE.md` and without git status snapshots to preserve context.
  - *Model Resolution:* Inherits the main conversation model, except when the main model is Fable: on subscriptions,
    Console accounts, and LLM gateways (`ANTHROPIC_BASE_URL`), Explore runs on Opus; on Bedrock, Google Cloud, and
    Foundry, it stays on the main conversation model.
- **`Plan`:** Research agent used during plan mode. Write and Edit tools are denied. Starts without `CLAUDE.md` and
  git status snapshot. Inherits the main conversation model.
- **`general-purpose`:** Catch-all agent for complex multi-step tasks requiring exploration and modification. Inherits
  the main conversation model unless `CLAUDE_CODE_SUBAGENT_MODEL` is set.
- **`claude`:** General catch-all helper.
- **Helper Pinned Agents:** `statusline-setup` (pins Sonnet) and `claude-code-guide` (pins Haiku).

### Downshift Baseline Resolution
- `Explore`, `Plan`, `general-purpose`, and `claude` are classified as `inheritingAgentTypes`
  (`internal/adapters/claudecode/sessionmodel.go`).
- Downshift reads the parent session model from the tail of `transcript_path` (last assistant line) or falls back to
  settings (`BASELINE_FROM_SETTINGS`), holding guessed downshifts to the small tier.
- When the parent model is Fable, `Explore` baseline stays `unknown` (`exploreMayLeaveTheMainModel`) because a hook
  cannot distinguish gateway/subscription Opus routing from cloud provider main-model routing.
- Built-ins that pin their own models (`claude-code-guide`, `statusline-setup`) or custom agent definitions with
  `model:` pin their baseline (`AGENT_PINS_MODEL`) and are left untouched by normal routing.

### Subagent Type Hints vs Policy Profiles
1. **Type Hints (`HarnessCapabilities.AgentTypeHints`):**
   - In `internal/core/capabilities.go`, Downshift maps `"Explore": TierSmall` and `"Plan": TierMid`.
   - Used only to resolve tie-breaks when the task prompt classification is unsure and carries no complexity or
     risk signals.
2. **Policy Profiles (PR #18):**
   - `downshift policy init` proposes an `explore` profile:
     `{"name": "explore", "tier": "small", "harness": "claude-code"}`.
   - When configured in `.downshift/policy.json`, the resolver matches the subagent type case-insensitively
     (`Explore` matches profile `explore`).
   - The declared profile represents explicit user policy: it forces the `Explore` subagent to the small tier
     (e.g., Haiku), bounding orchestrator choice even when the task prompt text is classified as complex.

---

## 3. Nested Subagents: Vendor Claims vs Live Verification

### Vendor Claim
- The Claude Code subagents guide states: *"By default, a subagent can spawn subagents of its own, up to three
  layers below the main conversation."*

### Empirical Verification (Claude Code 2.1.319, 2026-11-07)
- Direct testing in `docs/evidence/claude-code-nested-spawn-2026-11-07.md` verified that subagents **cannot**
  spawn subagents in Claude Code 2.1.319.
- Live test 1: A `general-purpose` subagent instructed to spawn a subagent queried available tools
  (`ToolSearch select:Agent`), found no `Agent` tool, and reported that the tool was unavailable.
- Live test 2: A custom subagent definition with `tools: ["Agent", "Read"]` was explicitly loaded via `--agents`.
  When instructed to spawn a helper, it confirmed that the `Agent` tool is withheld by the harness within subagent sessions.
- In both sessions, the hook log captured exactly one `Agent` `PreToolUse` event: that of the main session. No nested
  spawn ever reached `PreToolUse` or executed.

### Subagent Tool Invocations and Hook Signal
- While subagents cannot invoke `Agent`, their other tool invocations (such as `Bash` or `ToolSearch`) **do** trigger
  `PreToolUse` and `PostToolUse` hooks.
- These payloads carry top-level fields `agent_id` and `agent_type`. However, their `session_id` and `transcript_path`
  match the parent session (`SessionWideTranscript: true`).
- Downshift reads `agent_id` via `isSubagentCall` to exclude subagent tool calls from main-session work reporting.
- If a nested spawn were ever delivered by a future release, Downshift's adapter detects
  `SpawnedFromSubagent() bool { return ev.AgentID != "" }`, recognizes that the transcript cannot identify the
  subagent spawner's model, and safely holds the decision as an unknown baseline (`HELD_UNCONFIDENT_UNKNOWN_MID`).

---

## 4. Workflows and Anthropic Managed Agents

There are two completely distinct workflow concepts in the Claude ecosystem, operating on different control planes:

### A. Local Claude Code `Workflow` Scripts (GAP-WORKFLOW-AGENTS)
- **Execution Surface:** Client-side within Claude Code CLI.
- **Observed Behavior (2026-10-11, `docs/evidence/workflow-agents-bypass-pretooluse-2026-10-11.md`):**
  - Spawns created by a Claude Code `Workflow` script bypass `PreToolUse` hooks completely.
  - Only `SubagentStart` fires. The payload carries `agent_id`, `agent_type`, `cwd`, and `session_id`, but contains
    **no task text** and **no model field**.
  - `SubagentStart` does not accept an `updatedInput` modification channel.
- **Routing Status:** Downshift cannot classify or rewrite these spawns. This is a harness event boundary, not an
  adapter defect.
- **Operational Mitigation:** Users must declare the model explicitly in the workflow script invocation
  (e.g., `agent({model: "haiku"})`). Downshift continues to route all standard interactive and delegated `Agent` calls.
- **Tool Filter:** Subagents running in Claude Code are denied the `Workflow` tool by default built-in filtering.

### B. Anthropic Managed Agents (Multiagent Orchestration Platform)
- **Execution Surface:** Hosted server-side API on Anthropic Platform (`https://api.anthropic.com/v1/agents`,
  `/v1/sessions`, `/v1/environments`, beta header `managed-agents-2026-04-01`).
- **Architecture:**
  - Configured via the `multiagent` block in agent definitions (`type: "multiagent_20261001"` or legacy `coordinator`).
  - Supports three distinct orchestration mechanisms:
    1. **Subagents (`subagents`):** One-level-deep delegation to persistent session threads (up to 25 child threads).
       Agents can be predefined (`predefined_agents`) or dynamically created inline (`inline_agents`).
    2. **Dynamic Workflows (`workflows`):** The primary agent writes a program executing in phases in background
       threads to process bulk work.
    3. **Advisor (`advisor`):** The primary thread consults an advisor model mid-turn via platform-spawned
       `anthropic.advisor` threads.
- **Hook Reachability:** Anthropic Managed Agents run entirely in hosted cloud sandboxes. Local Claude Code hooks
  (`~/.claude/settings.json`, `.claude-plugin/hooks`) **do not run** on Managed Agents platform threads. Local hook
  interception is impossible by architecture.
- **Model Enforcement on Managed Agents:**
  - Models for subagents are defined per-agent in `predefined_agents` or inherited from the primary agent.
  - Model override can only occur via the platform API at session creation (`agent` configuration overrides), not
    via local client hooks.

---

## Summary Matrix: Observed vs Unobserved Capabilities

| Surface / Invocation Type | Triggered Hook | Input Rewrite Channel | Model Honor Status | Notes / Citations |
|---|---|---|---|---|
| Claude Code ordinary delegated `Agent` | `PreToolUse` (`Agent`/`Task`) | `updatedInput.model` | **Observed Honored** | Family aliases only (`haiku`, `sonnet`, `opus`, `fable`). Full IDs rejected. Verified live 2026-10-06. |
| Claude Code `reasoning_effort` / `effort` | `PreToolUse` (`Agent`) | `updatedInput.effort` | **Unobserved** in settings hooks | Param documented in v2.1.292+; hook rewrite unobserved without mods; `CLAUDE_CODE_EFFORT_LEVEL` overrides. |
| Claude Code built-in `Explore` | `PreToolUse` (`Agent`) | `updatedInput.model` | **Observed Honored** | Inherits main model (Opus under Fable on sub/console). PR #18 `explore` profile routes to small tier. |
| Claude Code built-in `Plan` | `PreToolUse` (`Agent`) | `updatedInput.model` | **Observed Honored** | Inherits main model. Downshift hint: mid tier. |
| Claude Code nested subagent spawn | None | None | **Not Applicable** | Subagents lack `Agent` tool in 2.1.319 (verified live 2026-11-07). No nested spawn exists to route. |
| Claude Code subagent tool calls (Bash) | `PreToolUse` (`Bash`) | `updatedInput.command` | **Observed** (events fire) | Payloads carry `agent_id` and parent transcript. Compact wrapper applies. |
| Claude Code `Workflow` script spawns | `SubagentStart` only | None (`PreToolUse` bypassed) | **Not Routed** | GAP-WORKFLOW-AGENTS (2026-10-11). Set model on `agent()` call directly. |
| Anthropic Managed Agents (subagents) | Server-side threads | N/A (platform API) | **Not Applicable** | Hosted cloud control plane; local hooks do not run. Governed by `predefined_agents`. |
| Anthropic Managed Agents (dynamic workflows) | Server-side phases | N/A (platform API) | **Not Applicable** | Runs in cloud sandboxes. Governed by `workflows.predefined_agents`. |
| Anthropic Managed Agents (advisor) | Server-side thread (`anthropic.advisor`) | N/A (platform API) | **Not Applicable** | Platform consultation thread. Model configured via `advisor.model`. |
