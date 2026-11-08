---
name: downshift-squad
description: Invoke Downshift agent squads for coordinated multi-agent work. Use when the user says "squad", "time", "convoca", or wants to delegate to specialized agents. For "downshift full power", "força total" or "downshift shift <squad>", use the downshift-orchestrator skill.
---

# Squad — Downshift Agent Orchestration

Invoke specialized agent squads for coordinated work. The squad system routes tasks
to the right agents based on domain expertise.

## Syntax

```
squad <target> "<task>"
```

Or in natural language:
- "squad core fix the race condition"
- "chama o time de qualidade pra revisar"
- "convoca o coder pra implementar"

## Targets

| Target | Agents | Best for |
|--------|--------|----------|
| `downshift` | all 14 agents | Full project: feature + tests + docs + review |
| `core` | coder, coder-back, coder-front, refactor, researcher | Implementation, refactoring, analysis |
| `quality` | tester, debugger, reviewer | Testing, debugging, review |
| `security` | security | Security audit, threat modeling |
| `performance` | performance | Profiling, optimization |
| `docs` | documenter | README, API docs, guides |
| `leadership` | architect, platform, ai | Design, infra, AI decisions |
| `<agent>` | one specific | Focused single-agent task |

## Agent Catalog (14 agents)

| Agent | Domain | Mode | Tier | Specialty |
|-------|--------|------|------|-----------|
| **coder** | core | write | mid | Full-Stack Integrator, vertical slices, E2E |
| **coder-back** | core | write | mid | Backend specialist: Go/Node/PHP/APIs, DB, concurrency |
| **coder-front** | core | write | mid | Frontend & visual swarm: React 19, Next, Tailwind 4, Canvas 2D |
| **refactor** | core | write | mid | Code restructuring, cleanup |
| **researcher** | core | read | small | Code analysis, dependency tracing |
| **tester** | quality | read | small | Run tests, coverage analysis |
| **debugger** | quality | read | mid | Test failures, flaky tests |
| **reviewer** | quality | read | mid | Code review, quality gates |
| **security** | security | read | frontier | Security audit, threat modeling |
| **performance** | performance | read | mid | Profiling, bottlenecks |
| **documenter** | support | write | small | Documentation |
| **architect** | leadership | read | frontier | System design, cross-layer |
| **platform** | leadership | read | frontier | Infra, DevOps, CI/CD |
| **ai** | leadership | read | frontier | AI/LLM, model routing, FinOps |

## How it works

Routing is driven by `agents/config.toml` — the single source of truth for which
agent handles each task. The orchestrator reads this file so harnesses without
native memory (Claude Code, Cursor, Codex) route consistently.

1. **Load config**: Read `agents/config.toml` for `[routing]`, `[domains]`,
   `[escalation]`, and `[tiers]`
2. **Check escalation first**: If the task matches any `escalation.always_escalate`
   pattern, route through the escalation target before implementation (see below)
3. **Parse target**: Match task keywords against `[routing]` (first match wins);
   if none match, fall back to `[defaults]` (`unknown_task`, `complex_task`)
4. **Resolve domain**: For `squad <domain>` commands, expand the domain to its
   agents via `[domains]`
5. **Select topology**:
   - Single agent → direct delegation
   - Single domain → star (coordinator + workers)
   - Multi-domain → mesh (parallel with shared state)
6. **Pick tier**: Use `[tiers]` to assign small/mid/frontier per agent
7. **Spawn workers**: Use `downshift delegate` with agent prompts
8. **Coordinate**: Poll results, integrate outputs

## Examples

### Single agent
```
squad coder "Fix the nil pointer in pool.go line 42"
```
→ Spawns one coder agent with the task

### Domain squad
```
squad quality "Full review of PR #758"
```
→ Spawns tester + debugger + reviewer in star topology
→ Each runs their specialty, coordinator integrates

### Leadership decision
```
squad architect "Design the new payment module"
```
→ Spawns architect with frontier tier for design work

### Full squad
```
squad downshift "Add rate limiting to /api/users endpoint"
```
→ architect designs the approach
→ coder implements
→ tester/debugger verify
→ reviewer checks quality
→ security audits
→ documenter updates docs

## Routing rules (from config.toml)

Keyword → agent mapping lives in the `[routing]` table of `agents/config.toml`.
The FIRST matching keyword wins, so more specific patterns are listed first.
Below is a summary; `agents/config.toml` is authoritative.

| Task keyword (examples) | Routed to |
|--------------|-----------|
| `backend`, `api`, `endpoint`, `database`, `sqlite`, `mysql`, `concurrency`, `idempotency` | coder-back |
| `frontend`, `ui`, `canvas`, `visual swarm`, `swarm visual`, `css`, `tailwind`, `pwa`, `web worker` | coder-front |
| `vertical slice`, `end to end`, `full stack`, `integration`, `implement`, `add feature`, `bug fix` | coder |
| `refactor`, `cleanup`, `restructure` | refactor |
| `analyze`, `investigate`, `find where`, `trace` | researcher |
| `test`, `coverage`, `run tests` | tester |
| `debug`, `failing test`, `flaky` | debugger |
| `review`, `code review`, `PR review` | reviewer |
| `document`, `readme`, `api docs` | documenter |
| `security`, `audit`, `vulnerability`, `authentication`, `authorization`, `access control` | security |
| `performance`, `slow`, `optimize`, `profile`, `profiling` | performance |
| `architecture`, `design`, `system design`, `cross-module` | architect |
| `deploy`, `ci/cd`, `infrastructure`, `devops` | platform |
| `llm`, `model routing`, `embedding`, `machine learning`, `ai agent`, `ai model` | ai |

When no keyword matches, `[defaults]` applies: `unknown_task = "researcher"`
(understand first), and multi-file work routes to `complex_task = "architect"`
for decomposition.

### Domains (from config.toml)

`squad <domain>` expands via the `[domains]` table. Keep the skill and config in
sync — these are the groups defined in `agents/config.toml`:

```
squad core "..."        → coder, coder-back, coder-front, refactor, researcher
squad quality "..."     → tester, debugger, reviewer
squad security "..."    → security
squad performance "..." → performance
squad support "..."     → documenter
squad leadership "..."  → architect, platform, ai
```

## Escalation protocol (sensitive tasks)

Some tasks ALWAYS go through leadership before any implementation. The
`[escalation]` section of `agents/config.toml` lists the `always_escalate`
patterns and the routing targets.

When a task matches an `always_escalate` pattern, the orchestrator routes it to
the escalation target FIRST, then lets the implementer proceed only after the
review clears.

`always_escalate` patterns include:
`authentication`, `authorization`, `oauth`, `payment`, `billing`,
`database schema`, `migration`, `breaking change`, `api contract`,
`security`, `encryption`, `secrets`, `credentials`.

Escalation targets (`[escalation.targets]`):

| Pattern type | Target | Covers |
|--------------|--------|--------|
| `security_patterns` | security | auth, encryption, secrets |
| `data_patterns` | architect | database, migration, api contract |
| `default` | architect | everything else sensitive |

### Escalation examples

```
squad downshift "Add OAuth login to the API"
```
→ matches `auth` (always_escalate) → security reviews the design first
→ only then coder implements, with security verifying the result

```
squad coder "Add a migration to rename the users table"
```
→ matches `migration` (always_escalate) → architect reviews schema impact first
→ then coder applies the migration

## Tier selection (from config.toml)

The `[tiers]` table recommends a model tier per agent:

- `small` → researcher, tester, documenter (mechanical work)
- `mid` → coder, coder-back, coder-front, refactor, reviewer, debugger, performance (most work)
- `frontier` → architect, security, platform, ai (complex decisions)

An explicit `[tier=...]` in the task overrides the config default.
Call `downshift providers` before spawning to see available models.

---

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai
