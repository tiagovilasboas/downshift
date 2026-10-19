# Domain: Harness Adapters & Packaging

Harness-specific edge I/O adapters, fail-open execution runners, and multi-tier agent packaging.

Downshift is designed to serve every agent harness through thin, decoupled edge adapters. While the core deliberation and code intelligence subsystems remain strictly harness-neutral, edge adapters translate platform-specific hook wire payloads (Claude Code, Cursor, Codex, OpenCode, KiroCrew, and Antigravity) into canonical types, invoke core decisions, and serialize wire responses without embedding any business or routing policy.

---

## 1. Architectural Principles

1. **Zero Policy in Adapters**: Adapters only parse incoming wire payloads and emit standardized responses. Routing policy, tier mapping, and guardrails live exclusively in shared packages (`internal/core`, `internal/classify`).
2. **Layering Rule (Every Feature Serves Every Harness)**: Conformance tests (`conformance_test.go`) run the same decision scenarios across all registered harness adapters. No feature is built exclusively for a single harness in core logic.
3. **Fail-Open by Design**: The edge hook runner (`runHookAdapter`) wraps all adapter invocations with panic recovery, an explicit timeout deadline, and a 1 MiB bounded stdin read. If an adapter fails, crashes, or times out, it immediately returns the fail-open payload so agent execution proceeds uninterrupted.
4. **Thin Platform Packaging**: Platform integration surfaces (Claude Code plugins, Cursor hooks, Kiro scripts) are thin wrappers around the single `downshift` binary.
5. **No Native Intrusion**: Downshift does not modify harness binaries, patch closed-source runtimes, or wrap external network calls.

---

## 2. Multi-Harness Wire Architecture

```
[Agent Harness Tool Execution]
       │
       ▼ (stdin JSON payload, 1 MiB bounded read)
[runHookAdapter Runner]
       ├── Sets hook deadline (default 2500ms) & discovers ProjectDir
       ├── Recovers from panics ──► Fail-open allow
       │
       ▼
[Edge Harness Adapter (`internal/adapters/<harness>`)]
       ├── Parses wire payload (tool input, model, session identifiers)
       ├── Declares `HarnessCapabilities` (canRewrite, hasTokens, hasSubagents)
       ├── Calls `core.Route` / `core.PlanForSession` or `codeintel.Hook`
       └── Encodes wire response
              ├── Claude Code: updatedInput (model, reasoning_effort)
              ├── Cursor: updated_input (model)
              ├── Codex: model + reasoning_effort
              ├── OpenCode: subagent_spawn_intent mutation
              └── KiroCrew: exit 2 policy / retry block
```

---

## 3. The 5 Packaging Layers

Downshift organizes its distribution across five modular packaging layers, allowing any coding harness to load what it needs:

| Layer | Artifact Path | Clean Architecture Role | Harness Scope |
|---|---|---|---|
| **Layer 1: Memory** | `AGENTS.md` (and `CLAUDE.md`) | Always-loaded operational rules and instructions (under 200 lines) | Harness-neutral |
| **Layer 2: Skills** | `skills/downshift-setup`, `skills/downshift-audit`, `skills/cost-reducer` | Agent skills with structured procedures and judgment logic | Harness-neutral (SKILL.md standard) |
| **Layer 3: Hooks** | `downshift <harness>` subcommands, `PreToolUse`, `PostToolUse`, `SubagentStop` | Direct CLI entrypoints for hook interception and telemetry | Shared Go binary |
| **Layer 4: Agents** | `agents/mechanical-worker.md` | Model-omitted subagent definition example for autonomous routing | Claude Code / Harness-neutral |
| **Layer 5: Plugin & Mods** | `.claude-plugin/plugin.json`, `marketplace.json`, `mods/*` | Single-command installation bundle and modular extensions | Claude Code native plugin |

---

## 4. Claude Code Plugin & Mods

For Claude Code operators, Downshift packages hooks, skills, commands, and optional mods into a unified plugin:

- **Claude Code Plugin (`.claude-plugin/plugin.json`)**: Installs hooks (`PreToolUse`, `PostToolUse`, `SubagentStop`), custom commands (`/downshift`), and agent skills with a single command:
  ```bash
  /plugin install downshift --marketplace tiagovilasboas/downshift
  ```
- **Cache-Tax Mod (`mods/cache-tax`)**: Bundled inside the plugin. Monitors cold cache read fees on idle sessions and offers `/keepwarm` to maintain prompt cache warmth.
- **Downshift Router Mod (`mods/downshift-router`)**: An experimental in-process routing prototype that intercepts `agent.spawn` inside the node runtime, eliminating CLI process invocation overhead.
- **Sibling Mods**: Works seamlessly alongside recommended companion mods such as `savvy-progress` and `filetree`.

---

## 5. Harness Support Matrix

| Harness | Intercepted Event | Mutation Target | Status | Verification Reference |
|---|---|---|---|---|
| **Claude Code** | `PreToolUse` (`Agent`/`Task`) | `updatedInput.model` | Completed | [Observed honored write-up](../evidence/claude-code-rewrite-honored-2026-10-06.md) |
| **Codex** | `PreToolUse` (`spawn_agent`) | `model` + `reasoning_effort` | Functional | [Codex write-up](../evidence/codex-rewrite-honored-2026-10-02.md) |
| **Cursor** | `preToolUse` (`Task`) | `updated_input.model` | Functional | [Cursor write-up](../evidence/cursor-rewrite-honored-2026-10-10.md) |
| **OpenCode** | `subagent` spawn intent | Hook JSON response | Beta | [OpenCode benchmark](../evidence/benchmark-opencode-2026-10-18.md) |
| **KiroCrew** | Native CLI hook | Policy block / exit 2 | Experimental | [KiroCrew evidence](../evidence/kirocrew-native-hook-2026-10-10.md) |
| **Antigravity** | `PreToolUse` (`Task`) | Hook response | Experimental | [Antigravity evidence](../evidence/antigravity-executor-ack.md) |

---

## 6. CLI Edge & Diagnostics

- `downshift probe <event>`: Inspect incoming hook payloads from unknown or newly updated harnesses, logging field names to `probe.jsonl` without recording prompt text.
- `downshift setup <harness>`: Automate hook configuration and session policy installation with dry-run verification.
- `downshift doctor`: Comprehensive health check validating binary location, state directory permissions, hook wiring per harness, catalog status, and active overrides.

---

## 7. Deep References

- [Harness Compatibility Matrix](../reference/harness-matrix.md): Exhaustive breakdown of plans, hooks, and observed rewrite honor.
- [Layering Rule & Guidelines](../contrib/layering.md): Invariant rules ensuring adapters remain thin and core logic serves every harness.
- [Packaging Layers Architecture](../design/architecture.md#packaging-layers-what-an-agent-harness-loads): Design rationale for Memory, Skills, Hooks, Agents, and Plugin layers.
- [Claude Code Integration Guide](../guides/claude-code.md): Setup, opusplan configuration, and transcript usage monitoring.
- [Installation Guide](../guides/install.md): Step-by-step hook registration across all supported harnesses.
- [Claude Code Mods Overview](../../mods/README.md): Specifications and tests for bundled and sibling mods.
