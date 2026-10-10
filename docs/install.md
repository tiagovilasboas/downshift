# Installation and harness setup

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

Canonical guide for humans and coding agents installing Downshift on a user machine.

**Related:** [config.md](config.md) · [session-models.md](session-models.md) · [harness-matrix.md](harness-matrix.md) · [examples/README.md](../examples/README.md) · [Portuguese guides](pt/README.md)

## Health check

After install:

```bash
downshift doctor          # version, state dir (~/.downshift), catalog source
downshift try "rename userId in auth.ts" claude-code
```

Optional adapter smoke (from a clone of this repo):

```bash
./examples/run-all.sh
```

Availability discovery and usage quota are configured separately. See
[session-discovery.md](session-discovery.md) for model listings and
[quota.md](quota.md) for the native Claude Code statusline bridge, Codex transcript
collection, freshness, and required-quota mode. A successful discovery is not
proof of included credit, and installing a bridge is not proof that it has
received a live usage observation.

---

## Quickstart — zero to working hook in 2 minutes

**Step 1 — Install** (macOS / Linux):
```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh
```

`install.sh` still treats a non-flag argument as the release version. `--explicit-upshift` may come before or after that version. It writes `"explicit_upshift": true` into `session-models.json` without dropping harness lists that are already there. The same switch at runtime is `DOWNSHIFT_EXPLICIT_UPSHIFT=1`. Default is off. See [session-models.md](session-models.md#explicit_upshift).

**Step 2 — Verify the classifier** on your own prompts before wiring the hook:
```bash
downshift try "rename the userId variable" claude-code
# → TRIVIAL task → use claude-haiku-5-5 (small tier)

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
downshift: TRIVIAL task → downshift to claude-haiku-5-5 (~97% cheaper, prompts up to 100k tokens)
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

See [examples/](../examples/README.md) for sample payloads. `claude-code-pretooluse-session.json` carries a `session_models` list, so it shows a real rewrite.

> **⚠️ Claude Code Pro/Max/Teams/API only.** Free plan has no real subagents and blocks network installs. See [Plan compatibility](#plan-compatibility--read-before-installing) before proceeding.

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

Cursor, Claude Code, and Codex hooks send the active model, not the full picker list. Codex also sends a session ID; Downshift validates and hashes that ID before recording it, so local routing decisions can be grouped without persisting the raw session identifier. Record selectable model ids in `~/.downshift/session-models.json` (legacy: `~/.harness-downshift/`). A top-level harness list is a fallback; an exact `sessions.<harness>.<session_id>` list takes precedence. Lists are operator-curated: Downshift does not query the Codex model picker or infer account entitlement. If a hook payload includes `session_models` or `available_models`, that list is used and the file is skipped. Details and evidence: [session-models.md](session-models.md). [session-models.example.json](examples/session-models.example.json) is one Cursor session from 2026-09-27. It is an example, not the default for every user.

For Cursor quota, the hook can read a complete native export named by
`DOWNSHIFT_CURSOR_NATIVE_FILE` offline before the canonical quota cache. A
malformed or stale configured export holds routing. The external experimental
extension was tested on Cursor 3.24.9 and could not access the built-in-only
`cursor` API, even with an explicit proposed-API flag; no export was produced.
Installing that extension does not currently enable collection on that build.
See [the runtime diagnostic](evidence/cursor-gap-diagnostic.md). Provider
membership remains authoritative; do not replace it with a fixed model list.

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
in the catalog. With the default `explicit_upshift` off, downshift does not
select it. If the current subagent is already running on Astra, the model is
preserved unchanged. Set `"explicit_upshift": true` in `session-models.json`,
export `DOWNSHIFT_EXPLICIT_UPSHIFT=1`, or run `install.sh --explicit-upshift`
to let upshift select it when it is inside this harness's credit set (or when
this harness reported no credit set). Cursor `claude-fable-5-1-thinking-high`
is the same kind of opt-in. Claude Code `claude-fable-5-1` is not
`explicit_only`; append that id to the session list to opt in. To mark any
other model as explicit-only, add `"routing": "explicit_only"` to its entry
in `~/.harness-downshift/catalog.json`.

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

## KiroCrew (policy mode through the native Kiro CLI hook)

For Kiro CLI 2.29, register Downshift through KiroCrew's persistent
`agent.kiro_hooks` configuration. KiroCrew generates the agent spec consumed
by Kiro CLI, whose native `preToolUse` hook runs before the tool: `exit 0`
allows it; `exit 2` blocks it and returns stderr to the agent. There is no
`updated_input` channel, so the adapter uses **policy mode**. It classifies a
pending spawn with an explicit child model and:

- **right tier already** → `exit 0`, allow silently;
- **confident tier mismatch** → `exit 2`, block with a message naming the model
  to respawn with (and, for a small tier, a reminder to trim context so the
  child fits the smaller window);
- **uncertain downshift, unknown model, or non-subagent tool** → `exit 0`,
  fail-open. The router never blocks a spawn on its own doubt.

Use the `kirocrew-core` MCP tool `spawn_run` with explicit `task` and `model`
arguments. The native `use_subagent` / crew pipeline can omit the child model;
those calls are outside routing coverage. Downshift never infers the child
model from the parent's model or a UI title. The classifier is deterministic;
the agent must retry a blocked spawn with the recommended model.

### Setup

> **Important — two hook paths exist.** `~/.kiro/agents/kirocrew.json` is
> **generated** by KiroCrew and overwritten on every restart; edits there are
> lost. The durable registration path is `~/.kiro/crew/config.json` via
> `agent.kiro_hooks`. `~/.kiro/crew/hooks.json` is a separate, informational
> hook registry (growing `run_count` proves invocation; exit code does not
> block the tool path). Use `config.json` for policy enforcement.
> Confirmed on Kiro CLI 2.29.0, 2026-10-10.

1. Build the binary:
   ```bash
   go build -o downshift ./cmd/downshift
   ```
2. Make the checked-in wrapper executable:
   ```bash
   chmod +x scripts/kirocrew-pre-tool-use.sh
   ```
   The wrapper invokes the repository's `downshift` binary with the `kirocrew`
   subcommand. Keep it beside this build in the repository checkout.
3. Merge this entry into `~/.kiro/crew/config.json`, preserving existing
   configuration and hooks. Replace the example path with the absolute path
   to the wrapper in your checkout:
   ```json
   {
     "agent": {
       "kiro_hooks": {
         "preToolUse": [
           {
             "matcher": "*",
             "command": "/absolute/path/to/harness-downshift/scripts/kirocrew-pre-tool-use.sh"
           }
         ]
       }
     }
   }
   ```
   KiroCrew merges `kiro_hooks` with its bundled hooks when generating the
   agent spec. It accepts `command` as an existing absolute executable path,
   preserves `command` and `matcher`, and drops `timeout_ms`. Commands
   containing shell arguments are rejected. The wrapper carries the subcommand
   so it survives generation. **Do not edit `~/.kiro/agents/kirocrew.json`**:
   KiroCrew regenerates that file and your edits will be lost.
4. Populate the `kirocrew` session model list from the models actually
   selectable in your installed Kiro CLI. Follow [session-models.md](session-models.md)
   for ordering and discovery precedence. Without a known eligible target,
   policy mode allows the spawn unchanged.
5. Load [kirocrew-routing-guide.md](kirocrew-routing-guide.md) into the agent's
   instructions and restart KiroCrew so it regenerates its spec. Verify that
   the generated `preToolUse` includes the wrapper and existing bundled hooks.
6. Smoke-test the adapter with known available models (no spawn needed):
   ```bash
   printf '%s\n' '{"tool_name":"spawn_run","tool_input":{"task":"rename a variable","model":"YOUR_AVAILABLE_FRONTIER_MODEL"}}' \
     | ./scripts/kirocrew-pre-tool-use.sh
   # A confident mismatch with an eligible target returns exit 2 and model=...
   ```
   Replace the placeholder with a real current model. This checks adapter
   output only; it does not establish that the harness blocks execution.

`~/.kiro/crew/hooks.json` tool-call hooks are informational: they can execute
after the tool starts, and their exit code does not block that path. A growing
`run_count` proves invocation only. Do not use that registration as policy
enforcement or keep an informational duplicate of the native policy hook.

### Acceptance evidence and coverage

The linked instructions are the **guia inferencial** for the **behaviour**
eixo. The native pre-tool policy hook is the **sensor computacional** for that
same concern. Check a real `spawn_run` with a trivial task and an explicit
frontier model: prove it is blocked before the child starts, then retry once
with the recommended model. Verify the child's actual model in KiroCrew's
native subagent record. Missing-model calls must pass through unchanged.

Spec regeneration and a restart followed by these checks are the **sensor
computacional** for **architecture fitness**, paired with this setup guia.
Routing decisions in the state directory's `events.jsonl` provide continuous
local observations (default `~/.downshift`; legacy `~/.harness-downshift`).
A blocked decision proves enforcement only when paired with the native result;
it does not prove successful child execution, usage, or savings. A PostToolUse
compliance observer remains separate maintainer tooling (DS-04); this setup
does not install one. Native block, retry and served-model acknowledgement were
observed on 2026-10-10 with Kiro CLI 2.29.0; see
[the build-specific evidence](evidence/kirocrew-native-hook-2026-10-10.md).

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
│  04:03  opus   → haiku   trivial  -97%               │
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
dashboard. On Claude Code, real token counts come from a `SubagentStop` hook (`downshift claude-code-subagent-stop`), which prices the subagent from its own transcript once it finishes; see [session-models.md](session-models.md#claude-code-real-usage).

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
## Plan compatibility — read before installing

**Not every plan supports subagent model routing.** This is a hard constraint
at the harness level, not a bug in Downshift. Evidence matrix: [harness-matrix.md](harness-matrix.md).

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

Cursor's `preToolUse` hook fires and Downshift runs, but the `model` rewrite is silently discarded on legacy and free plans. [Tracked on Cursor forum](https://forum.cursor.com/t/pretooluse-hook-updated-input-is-silently-ignored-for-the-task-tool/151985). On usage-based Pro/Ultra plans where subagent model selection is expanded, routing works.

### Codex

| Plan | Subagents exist? | Model routing works? |
|---|---|---|
| Any (with `multi_agent_v2` enabled) | ✅ Yes | ✅ Yes — PreToolUse + updatedInput honored |

Requires `[features] multi_agent_v2` and `hooks = true` in `config.toml`.

### Summary

Downshift is most effective on:
- **Claude Code Pro / Max / Teams / API** — full model routing via hook
- **Codex** with multi_agent_v2 enabled — full model + reasoning_effort routing
- **Cursor Pro/Ultra** on usage-based plans with expanded subagent model selection

On free plans or legacy Cursor pricing, the hook runs but model rewrites may be silently ignored by the harness. The tool fails open — the subagent still spawns, just without the model change.

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
