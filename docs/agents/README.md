# Downshift Agents

Agent definitions for the Downshift orchestration framework.

## Architecture

Agent behavior is split into one machine-readable config and two kinds of
human-readable references:

| File | Role | Read by |
|------|------|---------|
| [config.toml](../../agents/config.toml) | Machine-readable routing, escalation, domains, and tier tables | Orchestrator skill |
| [routing.md](routing.md) | Human-readable routing matrix and examples for `config.toml` | Humans |
| [protocols/](protocols/) | Inter-agent coordination contracts | Agents + humans |

### config.toml

Single source of truth for orchestration. The orchestrator reads it to decide
which agent handles each task. It defines:

- `[routing]` — task keyword → agent (first match wins, specific patterns first)
- `[defaults]` — fallback for unknown tasks, complex-task decomposition, default tier
- `[domains]` — agents grouped for `squad <domain>` commands
- `[escalation]` — patterns that always go through leadership first, and their targets
- `[tiers]` — model-tier recommendation per agent
- `[memory]` / `[hooks]` — optional pattern storage and future task hooks

### routing.md

The human-readable companion to `config.toml`. Explains the routing rules,
keyword matching, escalation, complexity handling, and tier selection with
worked examples. When you add a route in `config.toml`, document it here.

### protocols/

Contracts for how agents coordinate:

- [protocols/handoff.md](protocols/handoff.md) — transferring work between agents (format, chains, rules)
- [protocols/escalation.md](protocols/escalation.md) — when and how to escalate to leadership agents
- [protocols/parallel-slices.md](protocols/parallel-slices.md) — several agents building one feature in disjoint files (plan, waves, assembly)

## Agent Catalog (14 agents)

| Agent | Domain | Mode | Tier | Primary Use |
|-------|--------|------|------|-------------|
| [coder](../../agents/coder.md) | core | write | mid | Full-Stack Integrator, vertical slices, E2E |
| [coder-back](../../agents/coder-back.md) | core | write | mid | Backend specialist: Go/Node/PHP/APIs, databases, concurrency |
| [coder-front](../../agents/coder-front.md) | core | write | mid | Frontend & visual swarm: React 19, Next, Tailwind 4, Canvas 2D, PWA |
| [refactor](../../agents/refactor.md) | core | write | mid | Code restructuring, cleanup |
| [researcher](../../agents/researcher.md) | core | read | small | Code analysis, dependency tracing |
| [tester](../../agents/tester.md) | quality | read | small | Run tests, coverage analysis |
| [debugger](../../agents/debugger.md) | quality | read | mid | Test failures, flaky tests |
| [performance](../../agents/performance.md) | performance | read | mid | Profiling, bottleneck analysis |
| [architect](../../agents/architect.md) | leadership | read | frontier | System design, cross-layer architecture |
| [reviewer](../../agents/reviewer.md) | quality | read | mid | Code review, test strategy, quality gates |
| [security](../../agents/security.md) | security | read | frontier | Security audit, threat modeling |
| [documenter](../../agents/documenter.md) | support | write | small | READMEs, API docs, guides |
| [platform](../../agents/platform.md) | leadership | read | frontier | Infra, DevOps, CI/CD, databases |
| [ai](../../agents/ai.md) | leadership | read | frontier | AI/LLM architecture, model routing, FinOps |

## Domains

Agents are organized into domains for routing:

- **core**: coder, coder-back, coder-front, refactor, researcher — building and understanding code
- **quality**: tester, debugger, reviewer — ensuring code works correctly
- **security**: security — protecting against vulnerabilities
- **performance**: performance — making code fast
- **support**: documenter — explaining code to humans
- **leadership**: architect, platform, ai — high-level decisions and architecture

## Mode

- **write**: Agent can modify files (uses worktree)
- **read**: Agent only reads and reports (uses `--read-only`)

## Tier Selection

Agents suggest tiers, but the dispatcher decides:

```
[tier=small]    → Mechanical work: researcher, tester, documenter
[tier=mid]      → Most work: coder, coder-back, coder-front, refactor, reviewer, debugger, performance
[tier=frontier] → Complex decisions: architect, security, platform, ai
```

## Usage

### Context triggers (recommended)

Just say what you need, then activate — it reads your context automatically:

```
User: I need to add rate limiting to the /api/users endpoint with tests

User: downshift full-power
→ full-power activated
  task: "add rate limiting to /api/users with tests"
  routing: coder (mid) → tester (small)
  escalation: none
```

Portuguese alias: `potencia-total` or `potência-total`

### Squad activation

```
User: the auth module needs a security review

User: downshift shift security
→ security (frontier) — context: "auth module security review"
```

Available squads:
```bash
downshift shift security    # security squad
downshift shift quality     # tester, debugger, reviewer
downshift shift core        # coder, refactor, researcher
downshift shift leadership  # architect, platform, ai
downshift shift coder       # single agent
```

### List available agents (the grid)

```bash
downshift agents list       # show all agents with domains and tiers
```

### Explicit task (optional)

When you want to pass the task directly instead of reading context:

```bash
downshift full-power "add rate limiting with full test coverage"
downshift shift coder "fix the nil pointer in pool.go"
```

### Implicit routing

If you just describe what you need without any command, the orchestrator routes automatically:

```
User: fix the nil pointer in pool.go
→ coder (mid) — keyword: "fix"

User: why is the build slow?
→ researcher (small) — no keyword match, understand first

User: add OAuth authentication
⚠️ Escalation: "authentication" detected → security (frontier) reviews first
→ Then: coder (mid) implements

User: add user preferences with API, database, and UI
📐 Complex task (3+ components) → architect (frontier) decomposes first
```

### Legacy syntax (backward compatible)

```bash
squad coder "Fix the nil pointer"      # still works
squad quality "Full review of PR #758" # still works
squad downshift "Add rate limiting"    # same as downshift full-power "..."
```

Portuguese variants also work: `convoca`, `chama`, `time`.

## Format

Each agent is a Markdown file with YAML frontmatter:

```yaml
---
name: agent-name
description: "One-line description"
domain: core|quality|security|performance|support|leadership
mode: read|write
default_tier: small|mid|frontier
capabilities:
  - capability-1
  - capability-2
tools: Read, Grep, Glob, Edit, Bash
---

# Instructions for the agent
...
```

---

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai
