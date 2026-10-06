# Downshift

[![Build](https://github.com/tiagovilasboas/downshift/actions/workflows/ci.yml/badge.svg)](https://github.com/tiagovilasboas/downshift/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go 1.27](https://img.shields.io/badge/go-1.27-00ADD8.svg)](https://go.dev)
[![Status: beta](https://img.shields.io/badge/status-beta%20%E2%80%94%20practical%20testing-yellow)](https://github.com/tiagovilasboas/downshift/releases)

**Deterministic model routing for AI workloads.**

Downshift classifies each workload, applies catalog policy, and selects model tier and reasoning effort **before execution**—without an LLM in the routing loop. One Go binary: use it as a **coding harness hook** today; the same core is gateway-shaped for tomorrow.

```
Subagent / request
       ↓
  classify (rules + optional semantic boost)
       ↓
  policy + catalog → tier & model
       ↓
  execute on the chosen model
```

**Works today with Claude Code, Cursor, Codex, Antigravity, and KiroCrew.** Single binary, no runtime dependencies, no network calls for routing, no API keys to pick a tier.

![Downshift routing](docs/brand/downshift-routing-runtime-light.svg)

> **Beta — practical testing phase.** Hooks and adapters are production-minded; harness plans and builds vary in how faithfully they honor `updatedInput.model`. See [Plan compatibility](#plan-compatibility--read-before-installing) before installing.

### Why Downshift?

| | |
|---|---|
| **Right-sized models** | Trivial tasks downshift to small tiers; complex work stays on frontier. |
| **Deterministic** | Scored signals, not an LLM classifier, on the hot path. |
| **Testable** | `downshift try`, outcome benchmarks, optional shadow classifier (`DOWNSHIFT_SHADOW_WEIGHTS`). |
| **Observable** | Local telemetry (`events.jsonl`), feedback with explicit `--required-tier` for training labels. |
| **Harness-agnostic core** | Adapters only encode I/O; routing lives in `internal/core`. |

**Project docs:** [Architecture](docs/ARCHITECTURE.md) · [Roadmap](ROADMAP.md) · [Governance](GOVERNANCE.md) · [When to use](docs/WHEN-TO-USE.md) · [Full doc index](docs/README.md)

**License:** [Apache 2.0](LICENSE) — commercial use allowed. Optional hosted/support: [MONETIZATION.md](MONETIZATION.md).

### When to use Downshift

Use it for **subagent spawn routing** in coding harnesses when you want offline, deterministic tier selection. It is **not** an HTTP gateway like LiteLLM or a hosted marketplace like OpenRouter—those solve provider transport; Downshift solves **which model this spawn should run**. Full comparison: [docs/WHEN-TO-USE.md](docs/WHEN-TO-USE.md).

## Optional local LangGraph planner

The Go binary remains a deterministic, zero-runtime model router. For an explicitly opted-in local delegation-planning graph, see [Local LangGraph orchestration](docs/LANGGRAPH-ORCHESTRATION.md). It never chooses a tier/model or contacts an LLM; those decisions remain in Go.

**Keywords:** model routing · LLM cost optimization · agent hooks · Claude Code ·
Cursor · Codex · deterministic router · Downshift

---

## Quickstart — zero to working hook in 2 minutes

**Step 1 — Install** (macOS / Linux):
```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh
```

**Step 2 — Verify the classifier** on your own prompts before wiring the hook:
```bash
downshift try "rename the userId variable" claude-code
# → TRIVIAL task → use claude-haiku-4-5 (small tier)

downshift try "rearchitect the auth module to support multi-tenant" claude-code
# → COMPLEX task → use claude-opus-5-5 (frontier tier)
```

**Step 3 — Add the hook** to `~/.claude/settings.json`:
```json
{
  "hooks": {
    "PreToolUse": [
      { "matcher": "Task", "hooks": [{ "type": "command", "command": "downshift claude-code" }] }
    ]
  }
}
```

**Step 4 — Confirm it's working.** Open Claude Code, ask it to spawn a subagent for a trivial task. You'll see this in the terminal:
```
downshift: TRIVIAL task → downshift to claude-haiku-4-5 (~75% cheaper)
```

That line in stderr means the hook fired and rewrote the model before the subagent started.

**Verify the rewrite was honored** (important for beta exit validation):
- Ask Claude Code: *"spawn a subagent to rename the userId variable in auth.ts"*
- Open Claude Code's internal logs or monitor (check the subagent's actual model in the UI)
- Confirm it started on haiku, not the session's default frontier model
- This proves the hook's rewrite was not just emitted—it was **actually applied** by the harness

**Step 5 — (Optional) Open the live monitor.** `dsmon` is a floating terminal widget built into the same repo that tails `~/.harness-downshift/events.jsonl` and shows model switches, tier distribution and estimated savings in real time:

```bash
# Build the monitor binary (separate from the hook binary)
go build -o dsmon ./cmd/dsmon

# Open directly in your current terminal
./dsmon

# Or open a new floating window (auto-detects iTerm2 / kitty / Ghostty / Terminal.app)
./cmd/dsmon/launch.sh
```

See [dsmon — live monitor widget](#dsmon--live-monitor-widget) for the full widget reference.

## Testing locally before connecting the hook

Before adding the hook to Claude Code, test the classifier and hook logic locally:

**Test 1 — Classifier only (terminal):**
```bash
downshift try "rename the userId variable" claude-code
downshift try "rearchitect the payment flow" claude-code
downshift try "implement the CSV export" claude-code
```

**Test 2 — Full hook (JSON):**
```bash
downshift claude-code < examples/claude-code-pretooluse-session.json
```

This mimics exactly what Claude Code will send. The output is the JSON the harness will apply, plus stderr feedback with the decision and a feedback ID for tracking.

See [examples/](examples/README.md) for sample payloads. `claude-code-pretooluse-session.json` carries a `session_models` list, so it shows a real rewrite.

> **⚠️ Claude Code Pro/Max/Teams/API only.** Free plan has no real subagents and blocks network installs. See [Plan compatibility](#plan-compatibility--read-before-installing) before proceeding.

---

> *"Agent = Model + Harness."* — [Martin Fowler](https://martinfowler.com/articles/harness-engineering.html)

The **harness** is everything around the model that turns it into a working
agent: the loop, the tools, context assembly, and — the part this project cares
about — **how it delegates work to subagents**. Recent research names the
harness as *"the decisive lever against token maxing"*
([arXiv:2607.06906](https://arxiv.org/abs/2607.06906)).

`harness-downshift` is a focused piece of harness engineering. It doesn't touch
the model's reasoning; it engineers the **delegation layer** — the exact point
where a subagent's model is chosen — to optimize three things at once:

- **Token efficiency** — cheap models for cheap work; frontier tokens spent only where they earn it.
- **Cost reduction** — trivial subagents drop from frontier to small tier (~75% cheaper per call, based on published list prices: claude-haiku-4-5 $1/$5 vs claude-opus-5-5 $4/$20 per 1M input/output tokens). Session-level savings depend on your task mix — see [Cost evidence](#cost-evidence).
- **Control & predictability** — deterministic, rule-based routing you can read, test, and audit. No "Auto" black box, no LLM guessing in the loop.

Agent = Model + Harness. You can't cheaply swap the model. You *can* engineer
the harness. That's the whole game here.

---

## The cost problem nobody talks about

Agentic sessions are billed by the spawn, not by the session. Every time your
main agent hands off work to a subagent, that spawn is a separate billable event
at the session's model price.

The pattern plays out hundreds of times per day:

| What the subagent actually does | What it's billed as |
|---|---|
| List files in a directory | Frontier model spawn |
| Fix a typo in a comment | Frontier model spawn |
| Run a `git status` | Frontier model spawn |
| Generate a daily standup | Frontier model spawn |
| Rearchitect the auth module | Frontier model spawn ← this one earns it |

Every spawn in that list is identical on the invoice. `harness-downshift` makes
them different. It reads the task, classifies the work, and routes the spawn to
the minimum capable model before it starts — automatically, without you
touching a thing.

## What the router actually saves (real session data)

The `downshift serve` dashboard and `dsmon` monitor log every routing decision.
Here's what a typical engineering session looks like after one day:

```
39 routing events
25 downshifts  →  trivial/simple tasks routed away from frontier
 2 upshifts    →  tasks that needed more than the default model
 9 right-tier  →  no change needed

estimated savings: ~$0.20 (based on published list prices)
```

> **Note on the estimates:** savings are calculated from published token prices
> (haiku vs opus list price delta). They do **not** represent actual billing —
> your provider may have negotiated rates, volume discounts, or usage caps that
> change the real number. The router also only counts subagent spawns intercepted
> by the hook; direct model usage in the main session is not tracked.
> Treat these as directional, not as your invoice.

The meaningful number is not the dollar figure — it's the **25 times the router
prevented a frontier spawn for work that didn't need it**. That's 25 times you
didn't pay Opus prices to grep a folder.

---

## The tier model

The right model for the task. Not the most expensive one by default.

| Task complexity | Examples | Tier | Model |
|---|---|---|---|
| Trivial | rename, format, git commit, fix typo | small | **haiku-class** |
| Simple | add a field, fix a bug, one function | mid | **sonnet-class** |
| Medium | refactor a module, feature across files | mid | **sonnet-class** |
| Complex | rearchitect, migrate, race condition | frontier | **opus-class** |

`downshift` reads the task and picks the tier. Mechanical work goes to the cheap
model. Hard reasoning gets the frontier model. The "Auto" your harness ships
with does neither — it leaves every subagent on the most expensive model the
whole session.

---

## What it actually does

It runs as a **hook**. When your harness is about to spawn a subagent, it pipes
the spawn details to `downshift`, which:

1. **Classifies** the subagent's task: `TRIVIAL / SIMPLE / MEDIUM / COMPLEX`
   — deterministic scoring, no network, no extra tokens.
2. **Maps** the complexity to the minimum capable tier: `small / mid / frontier`.
3. **Rewrites** the subagent's model to the right one for that tier, before the
   subagent process starts.

The main session keeps the model you chose. Only the subagents get right-sized.

```
$ downshift try "rename the userId variable across auth.ts" claude-code claude-opus-5-5
Task:       rename the userId variable across auth.ts
Complexity: TRIVIAL
Intent:     trivial
Needs tier: small
Recommend:  claude-haiku-4-5
Current:    claude-opus-5-5
Verdict:    DOWNSHIFT
→ TRIVIAL task → downshift to claude-haiku-4-5 (~75% cheaper)
```

---

## Install (Claude Code)

Build or install the single binary (no runtime, no dependencies):

```bash
# Recommended — one-liner installer (macOS / Linux, detects arch automatically)
# Installs to /usr/local/bin/downshift — no PATH changes needed
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh

# Alternative — Go toolchain (any platform)
go install github.com/tiagovilasboas/downshift/cmd/downshift@latest
```

> **Go toolchain note:** `go install` places the binary in `~/go/bin`.
> If `downshift: command not found`, add Go's bin to your PATH:
> ```bash
> export PATH="$HOME/go/bin:$PATH"   # current session
> echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc  # permanent
> ```
> The `curl` installer above avoids this by installing directly to `/usr/local/bin`.

Pre-built binaries for macOS (arm64/amd64), Linux (arm64/amd64), and Windows (amd64)
are available on the [Releases](https://github.com/tiagovilasboas/downshift/releases) page.

Add the hook to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Task",
        "hooks": [
          { "type": "command", "command": "downshift claude-code" }
        ]
      }
    ]
  }
}
```

That's it. Every subagent your session spawns now runs on the right-sized model.

## Install (Cursor)

Cursor's hook protocol mirrors Claude Code's — same `preToolUse` interception,
`updated_input` to rewrite the subagent's model. Build the binary, then add the
hook to `.cursor/hooks.json` (project level) or `~/.cursor/hooks.json` (global):

```json
{
  "version": 1,
  "hooks": {
    "preToolUse": [
      { "command": "downshift cursor", "matcher": "Task" }
    ]
  }
}
```

The `matcher: "Task"` scopes the hook to subagent spawns only. Cursor watches
the config and reloads it on save.

## Session allowlist

Downshift writes a model id only when that id is in the current session. The catalog supplies tier, cost, family, and effort for ids that are also in the session. It never adds an id the session does not have. If the session list cannot be determined, the hook leaves the current model unchanged.

Cursor, Claude Code, and Codex hooks send the active model, not the full picker list. Codex also sends a session ID; Downshift validates and hashes that ID before recording it, so local routing decisions can be grouped without persisting the raw session identifier. Record selectable model ids in `~/.harness-downshift/session-models.json`. A top-level harness list is a fallback; an exact `sessions.<harness>.<session_id>` list takes precedence. Lists are operator-curated: Downshift does not query the Codex model picker or infer account entitlement. If a hook payload includes `session_models` or `available_models`, that list is used and the file is skipped. Details and evidence: [docs/session-models.md](docs/session-models.md). [docs/examples/session-models.example.json](docs/examples/session-models.example.json) is one Cursor session from 2026-09-27. It is an example, not the default for every user.

## Install (Codex)

Codex spawns subagents through a reserved `spawn_agent` tool under
`multi_agent_v2`. You can't put a model on the provider-visible call, but a
`PreToolUse` hook can inject the model **and** `reasoning_effort` into the tool
input before the child starts — which is exactly where `downshift` runs.

Enable the feature flags in `config.toml`:

```toml
[features]
hooks = true

[features.multi_agent_v2]
enabled = true
```

Register the hook in `.codex/hooks.json` (project) or `~/.codex/hooks.json`
(global). The matcher covers both the `Agent` and namespaced `spawn_agent`
tool names:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "(^Agent$|.*spawn_agent$)",
        "hooks": [
          { "type": "command", "command": "downshift codex" }
        ]
      }
    ]
  }
}
```

### Trust the hook before testing

Codex does not run newly configured local hooks until you review and trust
their exact command. This is a Codex safety control, not a downshift error.
After saving the configuration, start Codex and run:

```text
/hooks
```

Review and trust the `downshift codex` hook, then start a **new session**
before asking Codex to spawn a subagent. Until the hook is trusted, Codex skips
it and the subagent keeps the session model unchanged.

On Codex, downshift routes **two axes at once**: the model tier and the
reasoning effort (`low` for trivial work, up to `high` for frontier work).
For a session that offers GPT-6 Sol and Luna, a trivial subagent can route
from `gpt-6-sol` to `gpt-6-luna` at low effort — subject to the session
allowlist and Codex accepting the rewritten input.

Downshift always applies the selected reasoning effort. A code review is a
frontier task, so a `gpt-5.6-terra` session routes its subagent to
`gpt-5.6-sol` at high effort. `gpt-6-astra` is marked `routing: "explicit_only"`
in the catalog: downshift **never selects it automatically**, but if the current
subagent is already running on Astra the model is preserved unchanged — it was
the user's deliberate choice. To mark any other model as explicit-only, add
`"routing": "explicit_only"` to its entry in `~/.harness-downshift/catalog.json`.

A purely mechanical prompt such as `Use a subagent to only list the .go files`
is routed to `gpt-6-luna` at low effort when that exact id is in the session
allowlist.

> Codex's `multi_agent_v2` spawn schema is still evolving. downshift preserves
> the reserved fields and fails open, but pin the exact Codex build you deploy
> and keep an acceptance test on a real spawn.

## Antigravity (Google DeepMind)

Antigravity executes subagents via `invoke_subagent` and supports arguments modification in its `PreToolUse` hook protocol via `overwrite`.

Register the hook in `~/.gemini/config/hooks.json`:

```json
{
  "PreToolUse": [
    {
      "matcher": "invoke_subagent",
      "command": "downshift antigravity"
    }
  ]
}
```

When Antigravity attempts to spawn a subagent (e.g. inheriting the parent model or requesting `pro`), `downshift antigravity` intercepts the tool call, inspects the prompt inside `Subagents[0].Prompt`, and dynamically rewrites `Subagents[0].Model` to `flash` or `flash_lite` for mechanical tasks, logging the routing event to `~/.harness-downshift/events.jsonl`.

## KiroCrew (policy mode: block-and-instruct, not rewrite)

KiroCrew has subagents (`spawn_run`, `spawn_sub_agents`) and each spawn accepts
a per-child `model` — so the material the router needs is there. What it does
*not* have is a rewrite channel: its `preToolUse` hook contract is binary,
`exit 0` allows the tool and `exit 2` blocks it and relays stderr to the agent.
There is no `updated_input`, so the hook cannot swap the child's model in place
the way it does on Claude Code, Cursor, or Codex.

So the KiroCrew adapter runs in **policy mode** — the same lever Grok exposes,
used deliberately. It classifies the pending spawn and:

- **right tier already** → `exit 0`, allow silently;
- **confident tier mismatch** → `exit 2`, block with a message naming the model
  to respawn with (and, for a small tier, a reminder to trim context so the
  child fits the smaller window);
- **uncertain downshift, unknown model, or non-subagent tool** → `exit 0`,
  fail-open. The router never blocks a spawn on its own doubt.

The classifier is the same deterministic, prompt-free core — no LLM in the
loop. The difference from rewrite mode is that the agent respawns at the right
tier instead of the hook doing it silently; the discipline is forced, not
automatic.

### Setup

1. Build the binary:
   ```bash
   go build -o downshift ./cmd/downshift
   ```
2. Add a `preToolUse` hook scoped to the subagent tool. KiroCrew reads hook
   files from `~/.kiro/hooks/*.json`:
   ```json
   {
     "name": "downshift-subagent-router",
     "version": "1",
     "enabled": true,
     "hooks": {
       "preToolUse": [
         {
           "matcher": "subagent",
           "command": "/absolute/path/to/downshift kirocrew",
           "timeout_ms": 5000
         }
       ]
     }
   }
   ```
3. Test it from the terminal (no spawn needed):
   ```bash
   echo '{"tool_name":"spawn_run","tool_input":{"task":"rename a variable","model":"opus"}}' \
     | ./downshift kirocrew ; echo "exit=$?"
   # exit=2, stderr: "TRIVIAL task → downshift to claude-haiku-4-5 … Respawn with model=…"
   ```

When the hook blocks, KiroCrew relays the stderr to the agent, which respawns
the subagent at the recommended tier. Routing events are logged to
`~/.harness-downshift/events.jsonl` like every other adapter.

## dsmon — live monitor widget

`cmd/dsmon` is a separate command inside this repo — a floating terminal widget
that shows model switches, harness, tier distribution and estimated cost savings
in real time. It reads `~/.harness-downshift/events.jsonl` and refreshes every
800 ms. Zero external dependencies; pure stdlib Go.

```
╭──────────────────────────────────────────────────────╮
│  dsmon  downshift monitor                            │
│  harness  kirocrew        ◉ live                     │
│ switches today ────────────────────────────────────  │
│  04:03  opus   → haiku   trivial  -75%               │
│  04:02  haiku  → opus    complex  ↑                  │
│  04:01  opus   → sonnet  medium   -50%               │
│ stats ──────────────────────────────────────────────  │
│  35 events   22↓  2↑  8✓                             │
│  est. saved  $0.18  ~7K tokens ¹                     │
│  ¹ estimated · actual tokens not yet tracked         │
╰──────────────────────────────────────────────────────╯
  04:03:51 · ctrl+c to quit
```

### Build and run

```bash
# Build the monitor binary (separate from the hook binary `downshift`)
go build -o dsmon ./cmd/dsmon

# Run in the current terminal
./dsmon

# Open a floating terminal window (auto-detects iTerm2 / kitty / Ghostty / Terminal.app)
./cmd/dsmon/launch.sh
```

### What it shows

| Field | Source | Notes |
|---|---|---|
| Harness | `events.jsonl` → `harness` field | Last harness seen today |
| Switches | `events.jsonl` → `verdict` + model fields | Last 7 routing decisions |
| Events / ↓ ↑ ✓ | `events.jsonl` | Today's counts by verdict |
| Est. saved ($) | `estimated_savings` × $0.01 avg spawn cost | Rough estimate, not actual billing |
| Est. tokens | Derived from $ saved ÷ frontier output cost | Estimated, not from provider |

**Token consumption note.** The router has no access to provider-reported token
counts — it only sees what the harness passes to the preToolUse hook, which
does not include usage data. The estimates are directionally correct (more
downshifts = more savings) but not a substitute for your provider's billing
dashboard. On Claude Code, real token counts come from a `SubagentStop` hook (`downshift claude-code-subagent-stop`), which prices the subagent from its own transcript once it finishes; see [session-models.md](docs/session-models.md#claude-code-real-usage).

## Grok CLI (config, not hook)

Grok CLI is the honest exception, and it's worth being precise about why.

Grok has subagents (`spawn_subagent`, up to ~8 in parallel) and it reads
Claude Code / Cursor hook files. But its `PreToolUse` hook contract is
**allow-or-deny only** — `{ "decision": "deny", "reason": "…" }`. There is no
documented `updatedInput`, so a hook **cannot rewrite a subagent's model** the
way it can on Claude Code, Cursor, and Codex. A downshift hook on Grok could
only *block* a spawn, which isn't the job.

There's a second wrinkle: as of this writing Grok Build ships **one coding
model** (`grok-4.6`, with *configurable reasoning*), not a small/mid/frontier
model ladder. So on Grok the routing isn't "swap the model" — it's **dial the
reasoning effort**. Cheap work runs `grok-4.6` at low effort; the hard tasks
run it at high effort. Same principle, different knob.

Grok exposes both as **first-class config**, which is more robust than a runtime
rewrite. Set reasoning per subagent role in `~/.grok/config.toml`:

```toml
# Built-in read-only types → low reasoning (cheap, fast).
[subagents.personas.explore]
reasoning_effort = "low"

# Custom roles carry their own reasoning default.
[subagents.roles.reviewer]
description = "Reviews generated changes before commit"
default_capability_mode = "read-only"
reasoning_effort = "low"

[subagents.roles.architect]
description = "Cross-cutting design and migrations"
reasoning_effort = "high"    # torque for the hard curves

# If your catalog gains cheaper/stronger models, pin them per type too:
# [subagents.models]
# explore = "grok-4.6"
```

Precedence is explicit spawn override → role default → persona default → parent
session, so these pins hold unless the agent is told otherwise.
`downshift try "<prompt>" grok` tells you which tier a task wants; map that tier
to a role's reasoning effort in config once.

> If a future Grok build adds `updatedInput` to `PreToolUse` (or a multi-model
> catalog), a `downshift grok` hook adapter becomes a drop-in — the classifier
> and policy are already harness-agnostic. Until then, config is the right and
> documented lever.

---

## Capability Router v2 (experimental)

Production hooks use the **v1 deterministic classifier** (`core.Route`). **v2** is an offline / shadow pipeline (`train`, `benchmark --compare`, `DOWNSHIFT_SHADOW_WEIGHTS`) — it does **not** replace the hook unless you explicitly promote it after review.

Details: [docs/CAPABILITY-ROUTER-V2.md](docs/CAPABILITY-ROUTER-V2.md) · [docs/CLASSIFIER-SHADOW.md](docs/CLASSIFIER-SHADOW.md) · [docs/ENGINEERING-LOOP.md](docs/ENGINEERING-LOOP.md)

---

## Architecture

Package boundaries, hook flow, and experimental systems:
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

![Architecture](docs/img/architecture.svg?v=2)

### Updating or overriding the catalog

Default state dir: `~/.downshift` (legacy `~/.harness-downshift` supported).
Place `catalog.json` there to override the embedded default. Run
`downshift models list` or `downshift doctor` to see what is active.

```bash
# See the current effective catalog
downshift models list

# Discover new models from provider APIs (reads API keys from env)
ANTHROPIC_API_KEY=sk-... downshift models check

# Write new models to your override catalog (tier=unknown, assign manually)
ANTHROPIC_API_KEY=sk-... downshift models pull
```

The schema is in [`catalog.sample.json`](catalog.sample.json). Any field not
present in your override falls through to the embedded default.

---

## Why a hook, and why only subagents

The model of your **current turn** is loaded when the session starts — no
harness lets you swap it mid-turn from the outside (we tried; it doesn't work).
But a **subagent is a fresh process**. Its model is chosen at spawn time, in the
`Task` tool input, and a `PreToolUse` hook can rewrite that input before the
child starts.

That's the one place model selection is genuinely controllable from the outside
— and it happens to be where most of the cost hides. So that's where
`downshift` works.

---

## Harness support

| Harness | Subagents | Control mechanism | Status |
|---|---|---|---|
| **Claude Code** | ✅ Task tool (paid plans only) | `PreToolUse` hook → `updatedInput.model` | ✅ shipped |
| **Cursor** | ✅ Task tool | `preToolUse` hook → `updated_input.model` | ✅ shipped |
| **Codex** | ✅ `spawn_agent` (multi_agent_v2) | `PreToolUse` hook → `updatedInput.model` + `reasoning_effort` | ✅ shipped |
| **Antigravity** | ✅ `invoke_subagent` | `PreToolUse` hook → `overwrite.Subagents` | ✅ shipped |
| **Grok CLI** | ✅ `spawn_subagent` | **config**, not hook — `[subagents.roles/models]` in `config.toml` | ⚙️ config-based (see below) |
| **KiroCrew** | ✅ `spawn_run` / `spawn_sub_agents` | `preToolUse` hook → **policy mode** (exit 0 allow / exit 2 block + stderr; no rewrite path) | ✅ shipped |
| Kiro CLI (single-thread) | ❌ no subagents | — | not applicable |
| Claude.ai / ChatGPT web | ❌ closed | — | not possible |

**Policy mode vs rewrite mode.** The four rewrite adapters (Claude Code,
Cursor, Codex, Antigravity) swap the child's model *in place* via
`updated_input`. KiroCrew's `preToolUse` contract has no such channel — it is
binary: `exit 0` allows the spawn, `exit 2` blocks it and relays stderr to the
agent. So the KiroCrew adapter runs in **policy mode**: it classifies the
pending spawn and, on a *confident* tier mismatch, blocks with an actionable
message naming the model to respawn with. Downshifts require classifier
confidence (blocking on doubt would demote a task that needed the bigger
model); upshifts and non-subagent tools follow the shared fail-open rule. This
is why KiroCrew integration is *feature-complete, not automatic-rewrite*: the
gap was never the router, it was the harness's rewrite channel — KiroCrew
simply exposes allow/deny, so the router forces the discipline instead of
silently rewriting.

The classifier and policy are harness-agnostic — one brain. Each adapter
translates the decision into that harness's own mechanism.

---

## Plan compatibility — read before installing

**Not every plan supports subagent model routing.** This is a hard constraint
at the harness level, not a bug in harness-downshift.

### Claude Code

| Plan | Subagents exist? | Model routing works? |
|---|---|---|
| Free | ❌ No real subagents | — |
| Pro / Max / Teams | ✅ Yes | ✅ Yes — PreToolUse + updatedInput honored |
| API (direct) | ✅ Yes | ✅ Yes |

Claude Code free runs in a sandboxed container without network egress — `go install` will fail. The subagent feature itself only exists on paid plans.

### Cursor

| Plan / pricing | Subagents exist? | Model routing works? |
|---|---|---|
| Free | ✅ Limited | ⚠️ `updated_input.model` silently ignored (known issue) |
| Pro (request-based / legacy) | ✅ Yes | ⚠️ `model` field in Task only accepts `"fast"` — override silently dropped |
| Pro / Ultra (usage-based) | ✅ Yes | ✅ Works on plans with expanded subagent model selection |

Cursor's `preToolUse` hook fires and harness-downshift runs, but the `model` rewrite is silently discarded on legacy and free plans. [Tracked on Cursor forum](https://forum.cursor.com/t/pretooluse-hook-updated-input-is-silently-ignored-for-the-task-tool/151985). On usage-based Pro/Ultra plans where subagent model selection is expanded, routing works.

### Codex

| Plan | Subagents exist? | Model routing works? |
|---|---|---|
| Any (with `multi_agent_v2` enabled) | ✅ Yes | ✅ Yes — PreToolUse + updatedInput honored |

Requires `[features] multi_agent_v2` and `hooks = true` in `config.toml`.

### Summary

harness-downshift is most effective on:
- **Claude Code Pro / Max / Teams / API** — full model routing via hook
- **Codex** with multi_agent_v2 enabled — full model + reasoning_effort routing
- **Cursor Pro/Ultra** on usage-based plans with expanded subagent model selection

On free plans or legacy Cursor pricing, the hook runs but model rewrites may be silently ignored by the harness. The tool fails open — the subagent still spawns, just without the model change.

---

## How routing works (summary)

**No LLM on the hot path:** local scoring in Go, fail-open, same input → same tier
decision. Complexity (TRIVIAL → COMPLEX) maps to catalog **tiers** (small / mid /
frontier); adapters rewrite the harness payload or apply policy mode (KiroCrew).

Before you depend on it: run `downshift try "<your prompt>"` and read
[Plan compatibility](#plan-compatibility--read-before-installing).

| Need | Doc |
|------|-----|
| Hook flow, packages, state dir | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| Signals, margin, tuning, misroutes | [docs/contrib/classifier.md](docs/contrib/classifier.md) |

---

## Cost evidence

**Per-call cost differential (published list prices, October 2026):**

| Model | Input ($/1M tokens) | Output ($/1M tokens) |
|---|---|---|
| claude-haiku-4-5 (small tier) | $1 | $5 |
| claude-sonnet-5-5 (mid tier) | $2 | $10 |
| claude-opus-5-5 (frontier tier) | $4 | $20 |

Source: [Anthropic pricing](https://docs.anthropic.com/en/docs/about-claude/pricing), accessed 2026-10-06.

A single frontier subagent call grepping a directory is roughly 4× more
expensive than the same call on haiku. That's the raw per-call case.

**Session-level savings depend on your task mix.** After running downshift
for a few sessions, check your own numbers:

```
# Example output — your numbers will vary based on task mix and models used.
$ downshift stats

Last 30 days
─────────────────────────────────────
Subagent decisions    1,842

  Downshifted           972  52.8%
  Upshifted             611  33.2%
  Unchanged (OK)        259  14.0%

Normalised baseline   1,842 units
Normalised routed       891 units
Normalised savings      951 units  (51.6%)
─────────────────────────────────────
```

For dollar figures once you have real data: `downshift stats --cost-per-unit=<USD>`.

**Privacy note:** prompt contents are never stored. Each event records only
routing metadata — harness, complexity class, model IDs, verdict, and a
normalised savings fraction. Safe for corporate environments where task
prompts may contain sensitive information.

Each decision is recorded locally at `~/.harness-downshift/events.jsonl` —
no data leaves your machine. Use `downshift stats --days=7` for a weekly view.

> **Want to contribute real numbers?** Run downshift for a sprint, open an issue
> with your before/after cost, task volume, and false-downshift observations.
> First real dataset goes into this README with credit.

---

## Classifier benchmark

```bash
downshift benchmark path/to/tasks.json --report
```

Maintainer datasets are not in this clone. Published figures:
[benchmark/REPORT.md](benchmark/REPORT.md). Policy:
[benchmark/EVAL-PRIVATE.md](benchmark/EVAL-PRIVATE.md).

---

## Design principles

These are harness-engineering principles first, implementation choices second:

- **Deterministic over probabilistic.** Routing is scored rules you can read
  and test — not another model guessing. A harness you can't predict is a
  harness you can't trust with your budget.
- **Zero token overhead.** Classification is local pattern-matching. The router
  adds no LLM calls to the loop — it optimizes token spend without spending
  tokens to do it.
- **Fail-open.** A parse error, an unknown model, a bad event — any failure
  lets the subagent run unchanged. A cost optimizer must never become an
  availability risk.
- **Single binary.** Go, cross-compiled for macOS / Linux / Windows. No Python,
  no Node, no runtime. The harness layer should be boring and dependable.
- **Honest about limits.** It engineers the delegation layer — subagent models
  — not your main turn. It's a heuristic classifier, not an oracle. Good
  harness engineering states its blast radius.

---

## Where this fits

Teams running agents at scale are converging on the same shape: harnesses
(Cursor, Kiro, Claude Code, Codex) on the edge, and a **central corporate layer**
underneath them — governance, AI FinOps, observability, and a
**router / orchestrator** deciding which model handles what, across providers.

`harness-downshift` is a small, open, focused piece of that picture. It is the
**router at the delegation layer**: the deterministic rule that decides which
model a subagent gets, so cost per token is controlled by design rather than
reconstructed on the invoice. One brain, many harness adapters — the same
"all harnesses point at a central decision" architecture, in the open.

---

## Troubleshooting

### `downshift: command not found` after `go install`

`go install` places the binary in `~/go/bin`, which may not be in your PATH.

```bash
export PATH="$HOME/go/bin:$PATH"          # current session
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc  # permanent
```

The `curl` installer (`install.sh`) avoids this by writing directly to `/usr/local/bin`.

### The hook runs but the model is not being changed (Cursor)

On Cursor free plans and legacy request-based plans, `updated_input.model` is
silently discarded by the harness — this is a [known Cursor issue](https://forum.cursor.com/t/pretooluse-hook-updated-input-is-silently-ignored-for-the-task-tool/151985).
The hook fires and the binary runs (you'll see output on stderr), but the model
rewrite has no effect. See [Plan compatibility](#plan-compatibility--read-before-installing).

### No output on stderr — hook is not firing

Check the matcher. Claude Code uses `"Task"` (capital T); `"Agent"` does not
fire PreToolUse. Confirm with:

```bash
# Should print a JSON allow decision immediately
echo '{"hook_event_name":"PreToolUse","tool_name":"Task","model":"claude-opus-5-5","tool_input":{"prompt":"test"}}' | downshift claude-code
```

If that prints JSON, the binary works. If the hook still does not fire in a
live session, check that `settings.json` is valid JSON and that the path to
`downshift` is the absolute path (or is in PATH).

### `go install` fails on Claude Code free

Claude Code free runs in a sandboxed container with no network egress. Install
the binary on your local machine first, then use the hook — the binary runs on
your machine, not inside Claude Code's container.

### Verdict shows UNKNOWN instead of DOWNSHIFT

`UNKNOWN` means no current model was provided. Pass the current model as the
third argument to `try`:

```bash
downshift try "rename the variable" claude-code claude-opus-5-5
# → DOWNSHIFT
```

In the live hook this is handled automatically — the event carries the session
model and the adapter reads it.

---

## Contributing

Contributions are welcome — especially prompts the classifier gets wrong.

**The single most useful contribution** is a real subagent prompt that
`downshift` misroutes. Open an issue with:

- the prompt text (redact anything private),
- the complexity `downshift try "<prompt>"` returned,
- the complexity you expected, and why.

That feedback is what tunes the classifier against reality instead of against
our assumptions.

**Other ways to help:**

- **New harness adapter** — implement `internal/adapters/<harness>/` following
  the Claude Code adapter as a template. The `core` package is harness-agnostic;
  an adapter only translates a `core.Decision` into that harness's mechanism.
  See [CONTRIBUTING.md](CONTRIBUTING.md) for the step-by-step guide.
- **Model catalog updates** — prices and model IDs live in
  `internal/catalog/catalog.json`. Edit the JSON (with a source link in the
  PR description) — no Go changes needed. Or run `downshift models pull` to
  discover new models automatically.
- **Classifier signals** — new keyword/pattern signals for a complexity class,
  with a table-driven test case in `classifier_edge_test.go` that proves the
  improvement.

**Ground rules:**

- Keep `go test -race -cover ./...` green.
- Every classifier change ships with a table-driven test case.
- Small, focused PRs. Conventional commits in English.
- Be honest about limits in docs — no overselling what a heuristic can do.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full guide.

## Financial impact — what this means for a team

The numbers below use published list prices and conservative assumptions. Run `downshift stats` after a week to replace them with your own data.

### The math

A typical engineering session spawns ~50 subagents per day. Roughly half are mechanical (rename, list, grep, git, format). Without routing, all 50 run at the session's frontier price.

| Team size | Without downshift | With downshift | Monthly savings |
|---|---|---|---|
| 1 dev | ~$2.50/day | ~$1.56/day | **~$19/month** |
| 10 devs | ~$25/day | ~$15.60/day | **~$188/month** |
| 50 devs | ~$125/day | ~$78/day | **~$938/month** |
| 100 devs | ~$250/day | ~$156/day | **~$1,875/month** |

**Assumptions:** 50 spawns/dev/day at ~$0.05 each on the frontier model, 50% trivial (routed to small tier), 75% cost reduction on routed spawns (claude-opus-5-5 $4/$20 → claude-haiku-4-5 $1/$5), 20 working days/month. List prices October 2026. The table is derived from these assumptions: `with = without × (1 − 0.5 × 0.75)`.

### The multiplier effect

The per-spawn savings are small. The volume is not.

A team that runs 1,000 subagents per day and routes 500 of them to a model that's 4× cheaper runs those 500 at 25 cents on the dollar. At scale, the math compounds quickly — not because any single spawn is expensive, but because the pattern repeats thousands of times per week.

The efficiency gain compounds too. Frontier spawns for complex tasks run faster when they're not queued behind 200 trivial spawns hitting the same rate limits. Routing trivial work away from frontier also means the frontier tier is available when it matters.

### How to measure it for your team

After running `downshift` for one week:

```bash
downshift stats --days=7
```

Output:

```
Last 7 days
─────────────────────────────────────
Subagent decisions      847

  Downshifted           412  48.6%
  Upshifted             291  34.4%
  Unchanged (OK)        144  17.0%

Normalised baseline     847 units
Normalised routed       338 units
Normalised savings      509 units  (60.1%)
─────────────────────────────────────
```

For dollar estimates, pass your team's blended cost per frontier spawn:

```bash
downshift stats --days=7 --cost-per-unit=0.05
# → Estimated saved: $25.45 (509 routed spawns × $0.05)
```

**Important caveat.** These estimates are based on list prices and the number of routing decisions — not on actual provider billing. Token counts per spawn vary by task and model. Your actual savings may be higher (long frontier prompts avoided) or lower (very short spawns where the per-call overhead dominates). Treat these as directional. The most reliable data is your provider's billing dashboard before and after deploying downshift.

---

## What's missing for a complete project

Being honest: the router and adapters work today. These are the gaps between "works in practice" and "production-grade for a team":

| Gap | Why it matters | Status |
|---|---|---|
| **Real token counts** | Every savings figure today is estimated from routing decisions, not from actual provider usage data. The PostToolUse payload of an async subagent has no tokens, so Claude Code usage is read from the subagent transcript by a `SubagentStop` hook. | Claude Code: implemented, needs the SubagentStop hook in `settings.json`; no real-dollar billing period yet |
| **One week of real session data in the README** | The $0.20 in the current stats section is from a single day of testing. A week of real data from your own sessions would turn a directional estimate into a credible benchmark. | Needs real data |
| **End-to-end CI with a real spawn** | The test suite runs the classifier and the adapter logic. It does not spawn a real subagent and verify the model rewrite took effect. That integration test is the highest-confidence proof the whole chain works. | Not yet |
| **`downshift stats` fully functional** | The command exists in the README and in the binary. Verify it against a real `events.jsonl` with a week of data before promoting it as the primary measurement tool. | Verify |
| **Per-session before/after comparison** | "How much did this session cost without routing?" requires a baseline run. That needs a `--no-route` flag or a session where routing was disabled for comparison. | Planned |
| **Feedback loop closing** | The `downshift feedback` command collects outcomes but the training pipeline (`downshift train --from-events`) needs a curated dataset to improve the classifier. The first labelled dataset from real use is the highest-value contribution. | Waiting for data |

The infrastructure for all of these exists. What they need is time and real usage data — which is the honest state of every router project before it gets enough traffic to tune against.

---

This project started from a real pain I live every day as a developer running
multiple AI coding harnesses.

Every session I watch subagents inherit the main model — Opus doing a `git
status`, frontier models grepping directories, expensive tokens burning on
work that any cheap model handles identically. I wanted to fix that. The fix
was obvious in theory: route each subagent to the right model for its task.
The hard part was that harnesses don't expose a clean API for this, and
nobody had built a cross-harness solution that actually worked.

This is one of the most challenging projects I've built in my career —
not because the code is complex, but because it requires understanding
harness internals that most developers never touch. It sits below the
harness layer, at the exact point where a subagent's model gets decided,
and it works deterministically without adding tokens or latency to the loop.

Downshift your AI spend on routine work. Reserve frontier power for the tasks
that actually need it.

— [Tiago de Carvalho Vilas Boas](https://github.com/tiagovilasboas)

---

## References & inspiration

- **Martin Fowler — [Harness engineering for coding agent users](https://martinfowler.com/articles/harness-engineering.html).**
  The `Agent = Model + Harness` framing and the case for engineering the harness
  rather than chasing models. This article is the conceptual backbone of the
  project.
- **[The Harness Effect: How Orchestration Design Sets the Token Economics of
  Enterprise Agentic AI](https://arxiv.org/abs/2607.06906)** — names the harness
  as the decisive lever against token maxing.
- **[Triage: Routing Software Engineering Tasks to Cost-Effective LLM Tiers](https://arxiv.org/abs/2604.07494)**
  — proposes routing tasks to cheaper model tiers using code-health signals, with an evaluation protocol; it reports no measured routing results.
- **[claude-model-router-hook](https://github.com/tzachbon/claude-model-router-hook)**
  by tzachbon — a Claude-Code-only router that showed the `PreToolUse`
  `updatedInput` mechanism works. `harness-downshift` generalizes the idea
  across harnesses in a single Go binary.

---

## Status

**Beta — practical testing phase.**

The router is built and tested: adapters for Claude Code, Cursor, and Codex,
deterministic classifier covering 40+ documented prompts, catalog with
version-agnostic family matching, OpenRouter normalisation, and
`explicit_only` model preservation. The **CapabilityRouter v2** pipeline —
13-signal extractor, deterministic safety floor, risk-weighted softmax
classifier, offline training, and event collection — is complete and
available via `downshift train` and `downshift benchmark --compare`.

**The real gap is at the harness level, not in this tool.**
Model selection for subagents is an evolving feature in every harness:

- **Claude Code** — subagent model rewrite via PreToolUse + Task works, but
  only on paid plans (Pro/Max/Teams/API). Free plan has no real subagents.
- **Cursor** — the hook fires, but on free and legacy request-based plans
  `updated_input.model` is silently discarded. Works on Pro/Ultra with
  expanded model selection.
- **Codex** — works with `multi_agent_v2` enabled. The v2 spawn schema is
  still evolving upstream.
- **Grok** — hook is allow/deny only; routing is via `config.toml`.

Over the next few weeks, as harnesses broaden their own orchestration support,
the practical coverage of this tool will grow without any code changes on our
side. The architecture is ready — it's the harnesses catching up.

If the hook fires and the model rewrite takes effect on your setup, the tool
is fully operational. If it fires but the rewrite is silently ignored by the
harness, that is a harness limitation documented in
[Plan compatibility](#plan-compatibility--read-before-installing).

**Feedback most wanted:** prompts the classifier gets wrong. Open an issue
with the prompt, what `downshift try` returned, and what you expected.

## Classifier & outcome metrics

Curated public summary (maintainer-updated, not CI-generated):
[benchmark/REPORT.md](benchmark/REPORT.md).

## License

**Apache License 2.0** (`SPDX-License-Identifier: Apache-2.0`). See [`LICENSE`](LICENSE)
and [`NOTICE`](NOTICE). Use in personal, open-source, and **commercial**
products is allowed under the licence terms.

Versions before 2026-10-06 were under BUSL 1.1 ([archive](LICENSE-BSL-1.1-ARCHIVE.md)).
Relicense notes: [docs/RELICENSE.md](docs/RELICENSE.md).

How the maintainer plans to sustain the project (hosted, support): [MONETIZATION.md](MONETIZATION.md).

### AI agent notice

This repository includes [`llms.txt`](llms.txt) and [`AGENTS.md`](AGENTS.md).
AI coding agents should preserve Apache 2.0 headers and attribute the project
when generating docs or derivatives.

### Copyright

© 2026 [Tiago de Carvalho Vilas Boas](https://github.com/tiagovilasboas). See [NOTICE](NOTICE).
