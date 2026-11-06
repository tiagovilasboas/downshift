# Agent Squad System

Downshift includes a config-driven agent orchestration system that works in any
harness (Claude Code, Cursor, Codex, Kiro Crew) without runtime dependencies or
native memory.

## Quick start — context triggers

The easiest way to use the squad: describe what you need, then activate.

### Full-power activation (up to 5 agents)

The orchestrator sizes the squad to the task (never more than 5 at once; a small task gets one agent) and reads the `providers` strain block first, shrinking the squad or asking you when headroom is low.

```
User: I need to add rate limiting to the /api/users endpoint with full test coverage

User: downshift full-power
→ full-power activated
  task: "add rate limiting to /api/users with full test coverage"
  routing: coder (mid) → tester (small) → documenter (small)
  escalation: none
  complexity: 3 components
```

Portuguese alias: `potencia-total` or `potência-total`

### Squad activation

```
User: the auth module has a vulnerability

User: downshift shift security
→ security (frontier) — context: "auth module vulnerability"
```

Available squads:
- `downshift shift security` — security squad
- `downshift shift quality` — tester, debugger, reviewer
- `downshift shift core` — coder, refactor, researcher
- `downshift shift leadership` — architect, platform, ai
- `downshift shift coder` — single agent

### List available agents (the grid)

```bash
downshift agents list
```

## How context triggers work

1. **You describe** what you need in natural language
2. **You say** `downshift full-power` or `downshift shift <squad>`
3. **The orchestrator reads** your recent chat context
4. **Extracts the task** from what you said
5. **Routes automatically** using `agents/config.toml`
6. **Shows the plan** before executing

No quotes, no arguments — just the trigger command.

## Explicit syntax (optional)

When you want to pass the task directly:

```bash
downshift full-power "add rate limiting with tests and docs"
downshift shift coder "fix the nil pointer in pool.go"
```

## Implicit routing (no command)

Even without any command, the orchestrator routes automatically:

```
User: fix the nil pointer in pool.go
→ coder (mid) — keyword: "fix"

User: why is the build slow?
→ researcher (small) — no keyword match, understand first

User: add OAuth authentication
⚠️ Escalation: "authentication" detected → security (frontier) reviews first
→ Then: coder (mid) implements
```

## Legacy syntax (backward compatible)

These older syntaxes still work:

```
squad coder "Fix the nil pointer in pool.go"
squad quality "Full review of PR #758"
squad downshift "Add rate limiting"   # same as downshift full-power "..."
```

Portuguese variants: `convoca`, `chama`, `time`, `potencia-total`

## How it works

1. **You say** what you need (no "squad" prefix required)
2. **The orchestrator reads** `agents/config.toml` for routing rules
3. **Escalation patterns check** first for sensitive tasks
4. **Task keywords match** against the `[routing]` table (first match wins)
5. **Complexity detection** routes multi-component tasks to architect
6. **Routing shown inline** — you see which agent was selected
7. **Agent spawns** with the right tier (small/mid/frontier)

The routing is deterministic and file-based — no LLM call, no network, no memory
required. Any harness that can read files can use the squad.

## Targets

| Target | Agents | Best for |
|--------|--------|----------|
| `downshift` | all 12 | Full project work |
| `core` | coder, refactor, researcher | Implementation |
| `quality` | tester, debugger, reviewer | Testing, review |
| `security` | security | Security audit |
| `performance` | performance | Optimization |
| `docs` | documenter | Documentation |
| `leadership` | architect, platform, ai | Design decisions |
| `<agent>` | one specific | Focused task |

## Agent catalog

| Agent | Domain | Tier | Specialty |
|-------|--------|------|-----------|
| coder | core | mid | Implementation, TDD, bug fixes |
| refactor | core | mid | Code restructuring, cleanup |
| researcher | core | small | Code analysis, tracing |
| tester | quality | small | Tests, coverage |
| debugger | quality | mid | Failures, flaky tests |
| reviewer | quality | mid | Code review |
| security | security | frontier | Security audit |
| performance | performance | mid | Profiling |
| documenter | support | small | Documentation |
| architect | leadership | frontier | System design |
| platform | leadership | frontier | Infra, DevOps |
| ai | leadership | frontier | AI/LLM, FinOps |

## Routing keywords

Task keywords route to agents automatically. See `agents/config.toml` for the
full table. Examples:

| Keyword | Agent |
|---------|-------|
| `implement`, `add feature`, `bug fix` | coder |
| `refactor`, `cleanup` | refactor |
| `analyze`, `investigate`, `trace` | researcher |
| `test`, `coverage` | tester |
| `review`, `code review` | reviewer |
| `security`, `audit`, `authentication`, `authorization` | security |
| `architecture`, `design` | architect |

When no keyword matches: `researcher` first (understand), then `architect` for
complex multi-file tasks.

## Escalation protocol

Some tasks **always** go through leadership first. The `[escalation]` section of
`agents/config.toml` defines these patterns:

- `authentication`, `authorization`, `oauth`
- `payment`, `billing`
- `database schema`, `migration`
- `breaking change`, `api contract`
- `security`, `encryption`, `secrets`, `credentials`

Escalation targets:
- Security patterns → `security` agent reviews first
- Data patterns → `architect` reviews first
- Everything else → `architect` by default

Example:
```
squad downshift "Add OAuth login"
```
→ Matches `auth` (escalation pattern)
→ `security` reviews the design first
→ Then `coder` implements with `security` verifying

## Tier selection

Tiers recommend model size per agent:

| Tier | Agents | Use |
|------|--------|-----|
| small | researcher, tester, documenter | Mechanical work |
| mid | coder, refactor, reviewer, debugger, performance | Most work |
| frontier | architect, security, platform, ai | Complex decisions |

Override with `[tier=small]` prefix in the task prompt.

## Files

| File | Purpose |
|------|---------|
| `agents/config.toml` | Routing, escalation, tiers, domains |
| `docs/agents/routing.md` | Human-readable routing matrix |
| `docs/agents/protocols/handoff.md` | Inter-agent transfer protocol |
| `docs/agents/protocols/escalation.md` | Escalation triggers and flow |
| `docs/agents/protocols/parallel-slices.md` | Parallel build in disjoint files: slice plan, waves, assembly |
| `agents/*.md` | Individual agent personas |
| `skills/downshift-squad/SKILL.md` | Skill for agent invocation |
| `skills/downshift-orchestrator/SKILL.md` | Orchestrator with routing |

## Cross-harness compatibility

The squad system works anywhere:

| Harness | How to invoke |
|---------|---------------|
| Claude Code | Natural language: "squad coder fix the bug" |
| Cursor | Same as Claude Code |
| Codex | Same as Claude Code |
| Kiro Crew | Same, or load skill `downshift-squad` |

No MCP tools, no memory system, no special integration required — just the
prompt convention and file access.

## Examples

### Single agent task
```
squad coder "Fix the nil pointer in pool.go line 42"
```
→ One coder agent, mid tier, direct implementation

### Quality review
```
squad quality "Full review of PR #758"
```
→ tester + debugger + reviewer in parallel
→ Coordinator integrates findings

### Full squad delivery
```
squad downshift "Add rate limiting to /api/users"
```
→ architect designs approach
→ coder implements
→ tester/debugger verify
→ reviewer checks quality
→ security audits
→ documenter updates docs

### Escalation in action
```
squad coder "Add a migration to rename users table"
```
→ Matches `migration` (escalation pattern)
→ architect reviews schema impact first
→ Then coder applies migration

---

See also:
- [docs/agents/README.md](../agents/README.md) — Agent catalog
- [agents/config.toml](../../agents/config.toml) — Routing configuration
- [skills/downshift-squad/SKILL.md](../../skills/downshift-squad/SKILL.md) — Squad skill
