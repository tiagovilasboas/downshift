# Routing Matrix

This document describes how tasks are routed to agents. The orchestrator skill
reads `config.toml` for the machine-readable rules; this file is the human-readable
reference.

## Quick Reference

| Task Type | Primary Agent | Fallback | Tier |
|-----------|---------------|----------|------|
| Backend, API, database, endpoint | coder-back | coder | mid |
| Frontend, UI, canvas, visual swarm, PWA | coder-front | coder | mid |
| Full-stack integration, vertical slices | coder | — | mid |
| Implement, add feature | coder | — | mid |
| Refactor, cleanup | refactor | coder | mid |
| Analyze, investigate | researcher | — | small |
| Test, coverage | tester | debugger | small |
| Debug, failing test | debugger | coder | mid |
| Review, PR review | reviewer | — | mid |
| Document, README | documenter | — | small |
| Security, audit, auth | security | architect | frontier |
| Performance, optimize, profile | performance | — | mid |
| Architecture, design | architect | — | frontier |
| Deploy, CI/CD, infra | platform | — | frontier |
| AI, LLM, model routing | ai | architect | frontier |

## Domain Squads

Use these for broad tasks that need multiple agents:

```bash
squad core "Build the new user module"        # coder, coder-back, coder-front, refactor, researcher
squad quality "Full review of PR #759"        # tester, debugger, reviewer
squad security "Audit the auth flow"          # security
squad leadership "Design payment architecture" # architect, platform, ai
```

## Routing Rules

### 1. Keyword Matching

The orchestrator matches task keywords against `config.toml` routing table.
First match wins, so more specific patterns are listed first.

```
"fix authentication bug" → matches "auth" → security (escalation)
"fix login button"       → matches "fix bug" → coder
```

### 2. Escalation

Tasks matching escalation patterns **always** go through leadership first:

- **Security patterns** → `security` agent
  - authentication, authorization, oauth
  - encryption, secrets, credentials
  - security, vulnerability

- **Data patterns** → `architect` agent  
  - database schema, migration
  - api contract, breaking change

- **Default** → `architect` agent
  - payment, billing
  - anything else in escalation list

### 3. Complexity

- **Simple task** (1-2 files): route directly to matching agent
- **Complex task** (3+ files): route to `architect` for decomposition first
- **Unknown task** (no keyword match): route to `researcher` to analyze

### 4. Tier Selection

Each agent has a default tier, but the orchestrator can override:

```
[tier=small]    # Mechanical work: search, read, rename, single edit
[tier=mid]      # Most work: implement, review, debug
[tier=frontier] # Complex decisions: design, security, architecture
```

## Examples

### Simple bug fix
```
User: "Fix the nil pointer in pool.go"
Route: coder (keyword "fix", single file)
Tier: mid
```

### Security-sensitive change
```
User: "Add rate limiting to the auth endpoint"
Route: security → architect (escalation: "auth")
Tier: frontier
```

### Multi-file feature
```
User: "Add user preferences with API and UI"
Route: architect (complex: 3+ files expected)
Then: architect decomposes into coder tasks
```

### Unknown domain
```
User: "Why is the build slow?"
Route: researcher (no keyword match)
Then: researcher analyzes, recommends performance or platform
```

## Adding New Routes

Edit `config.toml`:

```toml
[routing]
"my keyword" = "agent-name"
```

Then update this document with the human-readable explanation.

---

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai
