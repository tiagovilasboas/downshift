<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Universal Orchestrator, MCP and Harness Engineering Approach

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Status: Owner direction, 2026-10-29. This document defines the engineering and testing
strategy for multi-agent delegation, MCP tool conformance, and harness interoperability.
Its purpose is to accelerate delivery with industry reference standards while eliminating
the need for bespoke, fragile test batteries.

---

## 1. Principles: No Bespoke Test Batteries

1. **The Lead LLM is the Orchestrator:** Do not build heavy workflow engines, state machines,
   or multi-turn conversation simulators in Go. The orchestrator LLM plans and decomposes;
   Downshift provides deterministic execution, policy bounding, and context compaction.
2. **Standard Conformance over Custom Mocks:** Use official protocol tooling (e.g., Anthropic's
   `@modelcontextprotocol/inspector`) to test MCP compliance rather than writing custom
   mock JSON-RPC clients.
3. **Fowler Architectural Fitness Functions:** Enforce layering, SRP, and zero-leakage
   invariants with sub-second deterministic Go tests (`internal/archtest`), avoiding flaky
   end-to-end multi-agent simulations.
4. **Golden File Contract Testing:** Test command compaction via table-driven fixtures
   (`*.raw` to `*.golden`). Expanding compactor coverage requires only capturing a real CLI
   trace, not building synthetic runner mocks.
5. **External Verification Gate (SWE-bench style):** Worker success is determined by
   computational outcomes (`exit code == 0` on the target verification command and `git diff`
   confined to declared bounds), not by evaluating conversational text.

---

## 2. Model Tier Division: Lead vs Haiku 5.5

Following the True North findings ([docs/true-north.md](../../true-north.md)) and owner decision O8:

| Role | Target Tier / Model | Responsibilities | Constraints |
|---|---|---|---|
| **Orchestrator (Lead)** | Frontier / Mid (Opus 5.5 / Sonnet 5.5) | Problem decomposition, dependency mapping, file boundaries, verification gate definition, final integration. | Never performs raw repetitive mechanical edits directly when tasks can be delegated. |
| **Worker (Subagent)** | Small (`claude-haiku-5-5` / `[tier=small]`) | Mechanical implementation, single-file edits, test fixture generation, lint/format fixes, repetitive adapter boilerplate. | Runs in an isolated Git worktree; strictly bounded to assigned files; cannot alter architecture or re-delegate. |

---

## 3. Delegation Contract: The Task Envelope

Every subagent delegation must provide a structured envelope:

```json
{
  "task_id": "DS-W01",
  "goal": "Brief, unambiguous objective in 1-2 sentences",
  "tier_hint": "small",
  "allowed_files": [
    "path/to/target_file.go",
    "path/to/target_file_test.go"
  ],
  "verification_gate": "downshift compactor -- go test ./internal/target/...",
  "instructions": "Specific, mechanical steps to perform without architectural changes."
}
```

If the worker's changes touch files outside `allowed_files` or if the `verification_gate`
fails, the worktree is discarded (`git worktree remove --force`) and the task is reworked
or escalated.

---

## 4. Testing & Conformance Stack

### A. MCP Server Conformance
- Test the stdio transport and schema validation with the official MCP Inspector:
  ```bash
  npx @modelcontextprotocol/inspector downshift mcp serve
  ```
- Test internal Go tool handlers (`internal/mcp/tools`) as pure unit functions with
  table-driven inputs and expected envelopes.

### B. Compactor Coverage
- Store real CLI output samples in `internal/execwrap/testdata/` or `internal/compressor/testdata/`.
- Run `go test ./internal/execwrap/...` to assert byte savings and preserved status/exit codes.

### C. Architectural Fitness
- Run `go test ./internal/archtest/...` before every PR.
- Ensures zero imports of adapters from core, zero prompts in telemetry, and strict package isolation.

---

## 5. Reusable Execution Prompts

### A. Prompt for the Lead Orchestrator (Decomposition & Delegation)

```markdown
You are the Lead Architect and Orchestrator.
Your goal is to break down the task: "{{TASK_OBJECTIVE}}" into independent, mechanical work units.

Rules:
1. Decompose only truly independent subtasks. Shared dependencies must be serialized.
2. For each mechanical workstream, delegate to a small-tier worker using the hint `[tier=small]`.
3. Provide each worker with an explicit Task Envelope:
   - Specific Goal (1 sentence)
   - Bounded file paths (only the files the worker is allowed to read and edit)
   - Verification command (e.g., `downshift compactor -- go test <package>`)
4. Never delegate architectural decisions or cross-cutting refactoring to the small tier.
5. Review the resulting git diff and verification evidence before accepting the work.
```

### B. Prompt for the Haiku 5.5 Mechanical Worker

```markdown
You are a Mechanical Software Implementer running on the small tier (`claude-haiku-5-5`).
You execute bounded, deterministic tasks with precision and minimal diffs.

Your Assigned Task:
- Goal: {{GOAL}}
- Bounded Scope: {{ALLOWED_FILES}}
- Verification Gate: {{VERIFICATION_COMMAND}}

Rules:
1. Touch ONLY the files listed in Bounded Scope. Any edit outside this list will be rejected.
2. Follow existing code patterns, clean code, and SRP. Do not add premature abstractions or comments.
3. Wrap all allowlisted terminal commands with `downshift compactor -- <command>`.
4. Run the Verification Gate before finishing:
   `{{VERIFICATION_COMMAND}}`
5. Report completion with the exact verification output and exit code. Do not provide conversational filler.
```

---

## 6. Native Markdown Skills & Engineering Playbooks Deployment

Unlike heavy orchestrators that enforce state machines and rules inside closed binaries or desktop apps, Downshift adopts an open, transparent **Harness-Native Markdown Architecture**.

### A. Harness Target Directories (Ordered by Community Adoption)
During `downshift setup`, Downshift inspects detected harnesses in order of community adoption and creates/links native `.md` skill files:

| Rank | Harness | Target Directory | Mechanism |
|:---:|---|---|---|
| 1 | **Claude Code** | `~/.claude/skills/` | Native Claude Code skills |
| 2 | **Cursor** | `~/.cursor/rules/` | Cursor rule files (`.mdc` / `.md`) |
| 3 | **OpenCode** | `~/.config/opencode/skills/` | Open-source agent skills & `AGENTS.md` |
| 4 | **OpenAI Codex** | `~/.codex/skills/` or `.codex/rules/` | Native instructions & skills |
| 5 | **Google Antigravity** | `~/.gemini/antigravity/skills/` | Antigravity skill bundles |
| 6 | **KiroCrew** | `~/.kiro/steering/` | Kiro steering files |

### B. Core Shipped Skills (`.md`)
1. **`downshift-orchestrator.md`**: Guides the session's Lead LLM on how to decompose complex tasks, check quota headroom before delegating, and communicate via Downshift's MCP tools (`downshift mcp serve`).
2. **`downshift-delegate.md`**: Provides the standard Task Envelope template for subagents, injecting explicit file boundaries, small-tier model hints, and verification commands.
3. **`downshift-rules.md`**: Inviolable operational rules for workers (no unverified code changes, strict minimal diffs, mandatory compactor command wrapping).

### C. Standard Engineering Playbooks (`.md`)
1. **`clean-code.md`**: Single Responsibility Principle (SRP), KISS, YAGNI, file length limits (< 300 lines), descriptive naming conventions.
2. **`tdd-verification.md`**: Test-driven development discipline, unit test coverage expectations, regression avoidance.
3. **`security-boundaries.md`**: OWASP ASVS rules, credential hygiene (zero secrets in code, args or logs), sandbox compliance.

By placing these capabilities into human-readable Markdown files controlled by the user's own harness, the orchestration remains fully auditable, easily customizable, and completely free of proprietary runtime lock-in.

