---
name: downshift-orchestrator
description: Playbook for the orchestrator (the lead session) when it delegates work to subagents. Use when splitting a task into subagents or choosing the model and effort of a spawn, with Downshift installed. Also use when the user says "downshift full power" or "full-power", "potência total" or "potencia-total", "força total" or "forca-total", "downshift full force" or "full-force" (the squad the task needs, up to 5 at once), or "downshift shift <squad>" (security, quality, core, leadership).
triggers:
  - downshift full-power
  - downshift shift
  - potencia-total
  - downshift full-force
  - forca-total
---

# Orchestrator playbook

You choose the model and the effort of every subagent. Downshift applies your choice,
checks it against the project policy and measures it. It does not pick a model on its own.

## 0. Context triggers — activation commands

### `downshift full-power` — squad activation (cap 5)

When the user says `downshift full-power` (with no task argument), this is a **context trigger**:

1. **Read the recent chat context** — scan the last few messages for the task
2. **Extract the implicit task** — what does the user want done?
3. **Apply full routing** — escalation → keywords → defaults → complexity
4. **Activate the agents the task needs** — spawn the right agents from the grid, never more than 5 at once; a small or indivisible task gets one agent. Before spawning, call the MCP `providers` tool and read its `strain` block: if headroom is low or a provider is rate-limited, shrink the squad or ask the user

Dispatch in the same turn. Do not explore alone: delegate the exploration (a researcher or architect maps the code while the others start), and spawn the agents in one message so they run in parallel.

**Output format:**
```
→ full-power activated
  task: "add rate limiting to /api/users"
  routing: coder (mid) → tester (small) → documenter (small)
  escalation: none
  complexity: 3 components (API + tests + docs)
```

Portuguese alias: `potencia-total` or `potência-total`

Old names `downshift full-force` and `forca-total` still trigger it, so older habits keep working.

If the context is unclear, ask: "What task should full-power work on?"

### `downshift shift <squad>` — squad activation

Activate a specific squad to work on the current context:
```
downshift shift security    → security squad reads context, works
downshift shift quality     → quality squad (tester, debugger, reviewer)
downshift shift core        → core squad (coder, coder-back, coder-front, refactor, researcher)
downshift shift coder       → single agent
```

### `downshift agents list` — show available agents (the grid)

List all agents in the grid with their domains and tiers.

## 1. Implicit routing — detect the right agent automatically

**Every task gets routed.** Before you act on any request, match it against the routing
config. No special syntax needed — the user says what they want, you detect who handles it.

### Step 1: Check escalation first

Match the task against `[escalation].always_escalate` patterns in `agents/config.toml`:

```
authentication, authorization, oauth, payment, billing,
database schema, migration, breaking change, api contract,
security, encryption, secrets, credentials
```

If ANY pattern matches:
```
⚠️ Escalation: [pattern] detected → [security|architect] reviews first
```

Route to the escalation target (security for auth/secrets, architect for data/api)
BEFORE any implementation. See `docs/agents/protocols/escalation.md`.

### Step 2: Match routing keywords

Match task text against `[routing]` keywords (first match wins):

| Keywords | Agent | Tier |
|----------|-------|------|
| backend, api, endpoint, database, sqlite, mysql, concurrency, idempotency | coder-back | mid |
| frontend, ui, canvas, visual swarm, swarm visual, css, tailwind, pwa, web worker | coder-front | mid |
| vertical slice, end to end, full stack, integration, implement, add feature, bug fix | coder | mid |
| refactor, cleanup, restructure | refactor | mid |
| analyze, investigate, find where, trace | researcher | small |
| test, coverage, run tests | tester | small |
| debug, failing test, flaky | debugger | mid |
| review, code review, PR review | reviewer | mid |
| document, readme, api docs | documenter | small |
| security, audit, vulnerability, authentication, authorization | security | frontier |
| performance, slow, optimize, profile, profiling | performance | mid |
| architecture, design, system design, cross-module | architect | frontier |
| deploy, ci/cd, infrastructure, devops | platform | frontier |
| llm, model routing, embedding, machine learning, ai agent, ai model | ai | frontier |

### Step 3: Apply defaults

- **No keyword match** → `researcher` (understand first)
- **3+ files or components mentioned** → `architect` (decompose first)

### Step 4: Output the routing decision

Always show the routing as the first line of your response:

```
→ coder (mid) — keyword: "fix bug"
```

Or for escalation:
```
⚠️ Escalation: "authentication" detected → security (frontier) reviews first
→ Then: coder (mid) implements
```

Or for complex tasks:
```
📐 Complex task (API + database + UI) → architect (frontier) decomposes first
```

### Context triggers (primary)

These are the primary activation commands — they read the chat context automatically:

- `downshift full-power` — all agents activation (reads context, coordinates the grid)
- `downshift shift <squad>` — activate specific squad on current context
- `potencia-total` — Portuguese alias for full-power

### Explicit syntax (optional)

When you want to pass the task explicitly:

- `downshift full-power "task"` — all agents with explicit task
- `downshift shift <squad> "task"` — specific squad with explicit task

### Legacy syntax (backward compatible)

These older syntaxes still work:

- `squad <agent> "task"` — explicit agent selection
- `squad <domain> "task"` — domain expansion (core, quality, security, etc.)
- `squad downshift "task"` — same as `downshift full-power "task"`
- `convoca`, `chama`, `time` — Portuguese variants

When explicit syntax is used, skip context reading and route directly.

## 1. Split the work and decide whether to delegate

Spawning a subagent is not free: each worker pays for its own system prompt, project context, and return review.
Evaluate whether delegation saves tokens or adds overhead before splitting.

### Delegation criteria

- **When to delegate:**
  - Independent, parallelizable, large bounded slices with disjoint file boundaries (`files` do not overlap).
  - Mechanical multi-file operations (e.g. bulk renames, constant extraction, doc-comment sweeps, test mirroring) where small or mid tier workers can run concurrently.
  - Slices of 3–5 story points where execution work substantially exceeds the prompt brief and review tokens.
- **When NOT to delegate (do the work yourself):**
  - Tightly coupled serial edits where step N+1 depends on step N's uncommitted code or intermediate state.
  - Short, trivial, or 1–2 story point tasks: a single edit inline is faster and spends far fewer tokens than briefing and spawning a worker.
  - High coordination overhead: when composing the prompt brief, specifying interfaces, and reviewing the return consumes more tokens than doing the work in the lead session.
  - Overlapping file targets: never delegate concurrent edits touching the same files.
- **Retain user choice and the natural baseline:**
  - Single-session execution without delegation is the natural control baseline and the right choice for bounded work.
  - Honor user intent: if the user asks for direct execution or does not request a squad, work sequentially in the lead session. Never force delegation simply because subagents or worker tools exist.

### Step-by-step splitting

1. Restate the task in one sentence. If it reads two ways, ask first.
2. Estimate each part in story points (1, 2, 3, 5, 8, 13). Split any part of 8 or more.
3. Parts are independent when they share no file, generated artifact or pending decision. Fan out
   one wave of min(ready independent parts, cap). Dependent parts go in a later wave.
4. Write the parts as plan JSON (`goal`, `parts[]` with `id`, `title`, `agent`, `points`, `depends_on`, `files`) and call the MCP `plan` tool (fallback: pipe it to `downshift plan` in the shell). It returns every problem if the plan is invalid; otherwise it fills tier, wave and route. Follow its Next steps wave by wave.
5. Do 1–2 point parts yourself: a spawn pays for its own context.
6. Read the cap before fanning out: `playbook` (no name) returns `max_parallel[<your harness>]`
   (default 6). Call `status`: if a provider is `rate_limited` or its headroom is not `ok`, use
   that entry's `when_strained` (the cap halved, minimum 1). A cap you were told directly wins.

## 2. Choose the model per spawn

Before every spawn, not once per session, call the Downshift MCP tool `providers` for your harness and read the live list:
you may only pick a model it shows. Each entry gives the tier (small, mid, frontier), the source and the efforts.
A model marked untiered has no tier evidence: name it only on purpose, never through a tier hint.

- `providers`: the models on offer now. Call it first, before each spawn.
- `apply`: previews what Downshift would apply for the model and effort you chose, under the project policy.
- `playbook`: the working rules to hand a worker.
- `plan`: validates your plan JSON and fills tier, wave and route per part, with next steps.
- `status`: quota headroom and advice; read it before fanning out many workers.

Fallback when the MCP server is absent: run `downshift models available --harness <your harness>`. If that
is missing or prints no list, use the models the harness itself offers and say so.

| Kind of work | Tier | Effort |
|---|---|---|
| Read-only lookup: find, list, summarise existing files | small | low |
| Mechanical edit: rename, format, boilerplate, one-file change | small | low |
| Review, documentation audit, test writing, a bounded implementation | mid | medium |
| Design, debugging a hard failure, decisions that span modules | frontier | high |

- Built-in agent types have no default model here: `Explore`, `claude-code-guide` and
  `statusline-setup` follow the first row's tier unless you say otherwise; `general-purpose` and
  `Plan` follow the parent unless you say otherwise. Say it on the call.
- Pass the model on the call (`model`). If the harness does not accept a per-call model,
  send a tier hint in the prompt (`[tier=small]`, `[tier=mid]`, `[tier=frontier]`).
- Never send a secret, an auth, payment or data-protection change to a small model
  unless you have read the diff yourself. Downshift no longer blocks it for you.

## 3. Check the result

- `downshift session --why` (or the MCP tool `status`) shows which model each spawn ran on and why.
- `downshift doctor` shows whether the hooks fired and the last decision.
- If a spawn failed, decide the next model yourself. Nothing moves it up or down on its own.

## 4. Delegate with an envelope

A worker sees only the task text and its directory, so write the whole brief into each delegated task:

- **Goal:** one sentence, unambiguous.
- **Allowed files:** the exact paths the worker may edit.
- **Verification:** the exact command that must exit 0, run as `downshift compactor -- <command>`.
- **Tier hint:** `[tier=small]` at the start of the prompt for mechanical work: a search, a read, a rename,
  running one command and reporting it, or a small well-specified edit. The hint only moves down, never up,
  and a pinned model or a declared policy wins. Choose by tier, never by model name.

Accept a result only when the verification exits 0 and the diff stays inside the allowed files. You keep the
decomposition and the design. A worker does not split work or choose the architecture.

## 5. Work with a running worker

- `collect` reads results; pass the `ack` back. A worker that ran several turns delivers one result per turn.
- `diff` shows what a write worker changed in its worktree. Review that instead of asking it to paste code.
- `send` gives the same worker a related follow-up, reusing its context. Unrelated work goes to a new `delegate`.
- `steer` corrects a worker that is going the wrong way, without losing its context. It stops the live turn and
  runs your correction first; work the stopped turn had not saved is lost, files already in its worktree stay.
- `cancel` stops a worker you no longer need.
- `release` the workers you are done with: it drops them and deletes the session transcript they left on disk.
  Account for every worker you started: send it more work or release it.

## What Downshift still enforces

- The project `.downshift/policy.json`: rules, profiles and tier floors you declared.
- Unknown model ids and models marked `explicit_only` in the catalog are refused.
- A decision that cannot be made in time or cannot be read keeps the model you asked for.

## Reference

- `agents/config.toml` — routing, defaults, domains, escalation and tiers (source of truth).
- `docs/agents/routing.md` — human-readable routing matrix and examples.
- `docs/agents/protocols/handoff.md` — handoff envelope and chains.
- `docs/agents/protocols/escalation.md` — escalation triggers, envelope and de-escalation.
