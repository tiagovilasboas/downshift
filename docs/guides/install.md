# Installation and harness setup

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

Canonical guide for humans and coding agents installing Downshift on a user machine.

**Related:** [config.md](config.md) · [session-models.md](../reference/session-models.md) · [harness-matrix.md](../reference/harness-matrix.md) · [examples/README.md](../../examples/README.md) · [Portuguese guides](../pt/README.md)

**Harness status:** Claude Code is Completed; Codex and Cursor are Functional; Antigravity and KiroCrew are Experimental (see [Experimental harnesses](#experimental-harnesses)); OpenCode is Beta (adapter and route wire in this repository, plugin wiring in development). The sections below follow that order. Definitions and evidence: [harness-matrix.md](../reference/harness-matrix.md#status-per-harness).

## Health check

After install:

```bash
downshift doctor          # version, state dir (~/.downshift), hooks per harness
downshift models available --harness claude-code   # the models the orchestrator can choose from
```

Optional adapter smoke (from a clone of this repo):

```bash
./examples/run-all.sh
```

Availability discovery and usage quota are configured separately. See
[session-discovery.md](../reference/session-discovery.md) for model listings and
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

The installer downloads the release archive and `checksums.txt` from the same
GitHub release and verifies the sha256 before anything is installed; a mismatch
or a missing checksum file stops it. `DOWNSHIFT_SKIP_CHECKSUM=1` installs
unverified and says so. Releases are also on the
[Releases page](https://github.com/tiagovilasboas/downshift/releases) with the
same `checksums.txt`.

**Build provenance (optional).** Releases built by the `release` job carry a GitHub
build-provenance attestation. After the sha256 check, if the `gh` CLI is installed,
`install.sh` runs `gh attestation verify <archive> --repo tiagovilasboas/downshift`
and prints `Build provenance verified`. It never blocks on this by default: with no
`gh`, no `gh auth login`, or a release older than the attestations, it prints a
warning and continues on the sha256 alone. `DOWNSHIFT_REQUIRE_ATTESTATION=1` makes
any outcome other than "verified" an error that installs nothing;
`DOWNSHIFT_SKIP_ATTESTATION=1` skips the check. The installer ends by saying what was proven: "sha256 and GitHub build provenance" or "sha256 only" (an intact download, not proof of who published it). It accepts only a strict `vMAJOR.MINOR.PATCH[-prerelease]` version, downloads with `curl --proto '=https' --tlsv1.2`, and installs through a temporary name plus a rename. By hand:
`gh attestation verify downshift_<ver>_<os>_<arch>.tar.gz --repo tiagovilasboas/downshift`.

`install.sh` treats a non-flag argument as the release version and refuses an unknown flag. (The `--explicit-upshift` flag and the `DOWNSHIFT_EXPLICIT_UPSHIFT` runtime switch were removed 2026-10-31: Downshift never raises a tier on its own, so there was nothing to opt into. The installer now rejects the removed flag. See [session-models.md](../reference/session-models.md#explicit_only-models).)

**Step 2 — Preview, then wire the hooks:**
```bash
downshift setup --dry-run   # prints the diff for each harness found; writes nothing
downshift setup             # launches the interactive First-Run Wizard (TTY) with smoke checks
downshift setup --yes       # automated zero-prompt setup for scripts and CI
downshift setup --no-tui    # standard command-line mode even inside an interactive terminal
```

Running `downshift setup` in an interactive terminal opens the **Setup Wizard**: it discovers harnesses in community ranking order (Claude Code, Cursor, OpenCode, Codex, Antigravity, KiroCrew (Experimental)), guides you through policy presets (Balanced, Aggressive, Safe), detects companion gateways (OmniRoute and 9router), previews the backup plan, and executes an in-process live smoke handshake (<50ms) across all registered adapters before completing.

For Claude Code, `setup` wires `PreToolUse` (matcher `Agent|Task`), `PostToolUse`, `PostToolUseFailure` and `SubagentStop`, sets the statusline when none exists, and sets `"model": "opusplan"` (Opus in plan mode, Sonnet for execution), `"permissions": {"defaultMode": "plan"}` (sessions start in plan mode) for keys you have not set; existing values are kept, and `--no-session-policy` skips both. Claude Code's built-in You should know mod (`"enabledPlugins": {"cc-plugin-you-should-know@builtin": true}`) is opt-in because each check is a billed background request to the session's model: `setup` prints a suggestion, and `downshift setup --you-should-know` writes it (`/plugin disable cc-plugin-you-should-know@builtin` turns it off) ([main session policy](../reference/session-models.md#main-session-policy-plan-mode-and-opusplan-claude-code)). It also finds Cursor, OpenCode, Codex, Antigravity and KiroCrew (Experimental), keeps unrelated keys and existing hooks, and is idempotent. `downshift setup --undo` restores the latest backup. KiroCrew (Experimental) gets a generated wrapper and a `preToolUse` entry in its existing `config.json`; see [KiroCrew](#kirocrew-policy-mode-through-the-native-kiro-cli-hook). The manual JSON is in [Manual setup](#manual-setup-claude-code) below.

**Step 3 — Check the models** your harness offers:
```bash
downshift doctor                                          # checklist: binary, hooks per harness, state
downshift models available --harness claude-code          # the models, their tiers and where each came from
```

**Step 4 — Confirm it's working.** Open a new Claude Code session and ask it to spawn a subagent for a trivial task. Then run `downshift session --why`: it lists each subagent with the model it asked for, the model it got and the reason. When the orchestrator starts the spawn's prompt with `[tier=small]`, the subagent runs on the small-tier model.

A reason in that list means the hook fired and decided before the subagent started.
Whether the harness applied it is a separate fact; `downshift stats` separates
`rewrite_emitted` from honored, and the matrix says what is verified per plan.

**Verify the rewrite was honored** (important for beta exit validation):
- Ask Claude Code: *"spawn a subagent to rename the userId variable in auth.ts"*
- Open Claude Code's internal logs or monitor (check the subagent's actual model in the UI)
- Confirm it started on haiku, not the session's default frontier model
- This proves the hook's rewrite was not just emitted—it was **actually applied** by the harness

**Step 5 — See what happened.** `downshift stats --days=1` shows decisions and tokens saved; `downshift session --tree` shows this session's subagents as a tree. In Claude Code, the plugin exposes `/downshift:status`; the optional manual alias `/downshift` is in [The /downshift command](#the-downshift-command-claude-code).

**Step 6 — (Optional) Watch it live.** `downshift watch` redraws the current session's subagent tree (model, end status, tokens, why) every second, for every harness; `downshift session --tree` prints it once, and `/downshift:status` or `/downshift` shows it inside Claude Code. The older floating widget is still available: [dsmon — live monitor widget](#dsmon--live-monitor-widget).

### Manual setup (Claude Code)

Use this instead of `setup` if you prefer to edit `~/.claude/settings.json` yourself. The PreToolUse matcher is `Agent|Task`. Claude Code's `Task` schema accepts only family names (`haiku`, `sonnet`, `opus`, `fable`) in `model`. Downshift writes the catalog's family name for you and leaves the spawn alone when a model has none.

```json
{
  "hooks": {
    "PreToolUse": [
      { "matcher": "Agent|Task", "hooks": [{ "type": "command", "command": "downshift claude-code" }] }
    ]
  }
}
```

For child status and real usage, also add `PostToolUse` and `PostToolUseFailure` (`downshift claude-code-post-tool-use`) and `SubagentStop` (`downshift claude-code-subagent-stop`), as `setup` does.


## Testing locally before connecting the hook

Before adding the hook to Claude Code, test the hook logic locally:

**Test 1 — Models and tiers (terminal):**
```bash
downshift models available --harness claude-code
```

**Test 2 — Full hook (JSON):**
```bash
downshift claude-code < examples/claude-code-pretooluse-session.json
```

This mimics exactly what Claude Code will send. The output is the JSON the harness will apply, plus stderr feedback with the decision and a feedback ID for tracking.

See [examples/](../../examples/README.md) for sample payloads. `claude-code-pretooluse-session.json` carries a `session_models` list, so it shows a real rewrite.

> **⚠️ Claude Code Pro/Max/Teams/API only.** Free plan has no real subagents and blocks network installs. See [Plan compatibility](#plan-compatibility--read-before-installing) before proceeding.

---
## Install (Claude Code)

**Status: Completed.** Rewrite observed honored, usage and child end recorded, plugin and session policy ([evidence](../evidence/claude-code-rewrite-honored-2026-10-06.md), [billing audit](../evidence/billing-gap-audit.md)).

Build or install the single binary (nothing else to install at runtime):

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

**Fastest path:** `downshift setup` finds Claude Code, Cursor, Codex, Antigravity and KiroCrew (Experimental) on this machine and merges every hook (plus the Claude Code statusline when none is set) into their hook files, using this binary's absolute path. It keeps unrelated keys and existing hooks, is idempotent, prints a diff, and saves a timestamped backup. `--dry-run` writes nothing, `--yes` skips the question, `--undo` restores the latest backup. For KiroCrew it also writes a wrapper script (see [KiroCrew](#kirocrew-policy-mode-through-the-native-kiro-cli-hook)). The manual JSON follows.

Add the hook to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Agent|Task",
        "hooks": [
          { "type": "command", "command": "downshift claude-code" }
        ]
      }
    ]
  }
}
```

That is the wiring. From the next session on, every subagent spawn goes through
the hook; whether the model is rewritten depends on the task (confident, no
risk) and on the plan ([harness matrix](../reference/harness-matrix.md)). On
natural work most spawns are held, and that is by design.

### Plugin install (Claude Code)

The same four hooks and the `/downshift:status` command ship as a Claude Code plugin
whose root is this repository. It needs the `downshift` binary on PATH (step 1
above) and replaces step 2 for Claude Code only. `downshift setup` does this install for you when it can: it runs `claude plugin marketplace add` and `claude plugin install downshift@downshift`, then removes the Downshift hooks it wrote earlier into `settings.json` (the plugin carries them; keeping both would route every spawn twice). It installs the plugin only when `claude` is on PATH and a login shell resolves `downshift` to the same binary that runs `setup`, because the plugin's hooks call `downshift` by name; otherwise it keeps writing the settings hooks and says why. `--no-plugin` always keeps them in `settings.json`. Setup also refuses to point hooks at a binary in the temporary directory, a Go build directory or a world-writable directory (install it somewhere stable first; `--allow-unsafe-binary` overrides). The other harnesses still use
`downshift setup`.

```text
/plugin install downshift --marketplace tiagovilasboas/downshift      # Claude Code 2.1.275+
```

or in two steps, `/plugin marketplace add tiagovilasboas/downshift` then
`/plugin install downshift@downshift`. The plugin wires `PreToolUse`,
`PostToolUse`, `PostToolUseFailure`, `SubagentStop`, and `UserPromptSubmit` (for Downshift Code Intelligence) from
[`hooks/hooks.json`](../../hooks/hooks.json) (each entry only calls the binary),
adds the `downshift-setup`, `downshift-audit` and `downshift-tune` skills, and the
command below as `/downshift:status`.
It does not set `opusplan`, plan mode, the statusline or
the You should know mod: those are settings, not hooks, and `downshift setup` writes them (You should know only with `--you-should-know`). Set
`"model": "opusplan"` and `"permissions": {"defaultMode": "plan"}` in `~/.claude/settings.json` yourself
(or `/model opusplan` in a session), and `/plugin enable cc-plugin-you-should-know@builtin` for You should know
(a billed request per check), if you want them without `setup`; see
[main session policy](../reference/session-models.md#main-session-policy-plan-mode-and-opusplan-claude-code).

Use one channel. A plugin hook and a settings hook with the same command both
run, so a session wired through both routes every spawn twice. `downshift setup`
skips the hooks when it sees the plugin enabled in `~/.claude/settings.json`
(it still applies the session policy and statusline), and `downshift doctor`
warns when it finds both. To switch from `setup` to the plugin, run `downshift
setup --undo` first. Detection reads the user settings file only; a plugin
enabled from a project's `.claude/settings.json` is not seen.

### The /downshift command (Claude Code)

Optional when installed by hand. A slash command that shows what Downshift did
in the current session (a tree of subagents, why the last one ran where it did,
pause and resume):

```bash
mkdir -p ~/.claude/commands
cp commands/status.md ~/.claude/commands/downshift.md   # from this repository
```

With the plugin it is already there as `/downshift:status`. Then in Claude Code:
`/downshift:status`, `/downshift:status --why`, `/downshift:status --pause`, `/downshift:status --resume`.
The optional manual copy above retains `/downshift` as a local alias.
The command only runs `downshift session` through the Bash tool, which its
`allowed-tools` line limits to that command. The same `downshift session` works
in a terminal for any harness. See [session-view.md](session-view.md).

### Downshift Code Intelligence (DCI)

Downshift includes an optional code intelligence engine that discovers repository code, extracts Go AST and lexical symbols, scores relevance via BM25, and injects compact structural context (<=3,500 bytes UTF-8) via `UserPromptSubmit` in Claude Code.

By default, DCI is `off`. To enable it in a repository:

```bash
downshift dci enable active   # Inject context into Claude Code prompts
downshift dci enable observe  # Observe and record telemetry without injecting
downshift dci status          # Show active mode and cache statistics
downshift dci doctor          # Check permissions, discovery, and latencies
downshift dci benchmark       # Run local performance and relevance benchmarks
```

To view DCI telemetry and metrics:
```bash
downshift stats --dci         # Display DCI-specific prompts, injections, and latencies
```
See [code-intelligence.md](../design/code-intelligence.md) for full architectural details.

## Install (Cursor)

**Status: Functional.** The rewrite is emitted on a real spawn; the harness applying it is unconfirmed, and it is discarded on Free and legacy Pro plans ([evidence](../evidence/cursor-rewrite-honored-2026-10-10.md), [gap diagnostic](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/cursor/findings/F1-cursor-gap-diagnostic.md)). On Cursor 3.27.12, Task `preToolUse` often omits `model` and `model_id`; Downshift remembers the parent composer slug from earlier hooks and from `subagentStop` ([internal spike](../internal/tasks/2026-10-24-cursor-live/spike.md)).

**Fastest path:** `downshift setup cursor --dry-run`, then `downshift setup cursor --yes`. That merges the hooks below (and your existing entries) and points them at the binary that ran setup.

Cursor's hook protocol mirrors Claude Code's for routing — `preToolUse` with
`updated_input.model` on matcher `Task`. Build the binary, then add hooks to
`.cursor/hooks.json` (project) or `~/.cursor/hooks.json` (global):

```json
{
  "version": 1,
  "hooks": {
    "sessionStart": [
      { "command": "downshift cursor-touch" }
    ],
    "beforeSubmitPrompt": [
      { "command": "downshift cursor-touch", "matcher": "UserPromptSubmit" }
    ],
    "preToolUse": [
      { "command": "downshift cursor", "matcher": "Task" },
      { "command": "downshift cursor", "matcher": "Shell" }
    ],
    "beforeShellExecution": [
      { "command": "downshift cursor", "matcher": "*" }
    ],
    "subagentStop": [
      { "command": "downshift end cursor" }
    ]
  }
}
```

`cursor-touch` records the session composer model into
`~/.downshift/cursor-composer-model.json` (see [config.md](config.md)) so Task
routing has a baseline when the spawn payload is empty. It always exits 0 with
`{}` on stdout. `subagentStop` runs `end cursor`, which links child-end telemetry
(`subagent_id` on 3.27.12+) and refreshes the cache from the parent slug in the
stop payload.

The `matcher: "Task"` entry handles subagent spawns. The `matcher: "Shell"` entry
lets `downshift compact` shorten the output of allowlisted test and build commands:
the hook answers `{"updated_input": {...}}` with `command` replaced and every other
field kept. This is unverified live: Cursor ignored `updated_input` for Task on some
plans, so Shell needs its own proof (see the
[shell rewrite table](../reference/harness-matrix.md)). To check, run `go test ./...`
in the agent and look for `downshift compact` in the command Cursor shows. Cursor
watches the config and reloads it on save.

The `beforeShellExecution` entry is the fallback for the same compaction. Cursor
sends every shell command (matcher `*`) with no tool envelope; when the command
belongs to an allowlisted test or build family, the hook answers
`{"permission":"allow","agent_message":"downshift: this command prints a long log. Run it again exactly as: …downshift compact -- …"}`
so the agent reruns it compacted, and otherwise answers a bare `{"permission":"allow"}`.
It never denies. If the `Shell` rewrite above is honored, the command Cursor then reports
is already wrapped and this entry stays silent. Cursor's docs say `agent_message` is sent to
the agent for `beforeShellExecution`; that is unverified on a live build.

## Install (Codex)

**Status: Functional.** The rewrite is emitted on a real spawn; the harness applying it is inferred, not observed ([evidence](../evidence/codex-rewrite-honored-2026-10-02.md)).

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

### Shell command compaction (Unverified live)

`downshift setup` also registers a second `PreToolUse` entry with matcher
`^Bash$` running the same `downshift codex`. It rewrites an allowlisted test or
build command (for example `go test ./...`) into `downshift compact -- ...`
through `updatedInput.command`, so only the PASS/FAIL summary reaches the model.
Codex applies `updatedInput` only with `permissionDecision: "allow"`, which the
hook sets on a rewrite and nowhere else. Turn it off with `DOWNSHIFT_EXEC=0` or
`"exec": false`. Trust the new entry in `/hooks` like the first one. Not yet
observed in a live Codex session ([harness matrix](../reference/harness-matrix.md)).

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

Downshift applies the selected reasoning effort, except when a guardrail holds the decision (the spawn keeps its requested effort) or when the decision is unconfident and does not rewrite the model. A code review is a
frontier task, so a `gpt-5.6-terra` session routes its subagent to
`gpt-5.6-sol` at high effort. `gpt-6-astra` is marked `routing: "explicit_only"`
in the catalog. Routing never selects an `explicit_only` model in any
direction: it is the owner's deliberate choice. If the current subagent is
already running on Astra, the model is preserved unchanged. Cursor
`claude-fable-5-1-thinking-high` is the same kind of `explicit_only` model.
Claude Code `claude-fable-5-1` is not `explicit_only`; append that id to the
session list so an operator floor lift can reach it. (The `explicit_upshift`
opt-in that once let an upshift select an `explicit_only` model was removed
2026-10-31.) To mark any other model as explicit-only, add
`"routing": "explicit_only"` to its entry in `~/.downshift/catalog.json` (the
state dir; `~/.harness-downshift/` is read only when it is the legacy dir in use).

A purely mechanical prompt such as `Use a subagent to only list the .go files`
is routed to `gpt-6-luna` at low effort when that exact id is in the session
allowlist.

> Codex's `multi_agent_v2` spawn schema is still evolving. downshift preserves
> the reserved fields and fails open, but pin the exact Codex build you deploy
> and keep an acceptance test on a real spawn.

## Session allowlist

Downshift writes a model id only when that id is in the current session. The catalog supplies tier, cost, family, and effort for ids that are also in the session. It never adds an id the session does not have. If the session list cannot be determined, the hook leaves the current model unchanged.

Cursor, Claude Code, and Codex hooks send the active model, not the full picker list. Codex also sends a session ID; Downshift validates and hashes that ID before recording it, so local routing decisions can be grouped without persisting the raw session identifier. Record selectable model ids in `~/.downshift/session-models.json` (legacy: `~/.harness-downshift/`). A top-level harness list is a fallback; an exact `sessions.<harness>.<session_id>` list takes precedence. Lists are operator-curated: Downshift does not query the Codex model picker or infer account entitlement. The one exception is Claude Code: with no list from any source it infers the session from the current model (the catalog tier ladder below it, ending at that model), so it can downshift but never upshift past what the session already runs; `DOWNSHIFT_INFER_SESSION=off` disables this. If a hook payload includes `session_models` or `available_models`, that list is used and the file is skipped. Details and evidence: [session-models.md](../reference/session-models.md). [session-models.example.json](../examples/session-models.example.json) is one Cursor session from 2026-09-27. It is an example, not the default for every user.

For Cursor quota, the hook can read a complete native export named by
`DOWNSHIFT_CURSOR_NATIVE_FILE` offline before the canonical quota cache. A
malformed or stale configured export holds routing. The external experimental
extension was tested on Cursor 3.24.9 and could not access the built-in-only
`cursor` API, even with an explicit proposed-API flag; no export was produced.
Installing that extension does not currently enable collection on that build.
See [the runtime diagnostic](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/cursor/findings/F1-cursor-gap-diagnostic.md). Provider
membership remains authoritative; do not replace it with a fixed model list.

## Compact steering (harnesses without a shell hook)

Claude Code rewrites an allowlisted Bash command in place, so it needs no steering. The other harnesses cannot, so `downshift setup` also installs the compact steering: one short text that tells the agent to run an allowlisted test, build or status command as `downshift compactor -- <command>`, and to run anything else without the compactor. The family list in it is the one the refusal message names, generated from the same policy (`downshift setup --dry-run` shows the exact text). Setup is idempotent, `--dry-run` writes nothing and `--undo` restores the file it changed.

- **Codex**: `~/.codex/AGENTS.md`, the global instructions file. Setup adds a marked block (`<!-- downshift:compact:begin -->` to `:end -->`) and keeps everything else in the file. Codex reads `AGENTS.override.md` first when it exists, and then ignores `AGENTS.md`, so the block is not read in that case; the compactor still works when invoked by hand.
- **Antigravity**: `~/.gemini/AGENTS.md`, one of the global rules files Antigravity reads (documented alongside `~/.gemini/GEMINI.md`). Same marked block; existing text is kept.
- **OpenCode**: `~/.config/opencode/AGENTS.md`, the documented global rules file. Same marked block. OpenCode is wired by its own plugin, so the steering is the only thing setup writes for it.
- **KiroCrew**: `~/.kiro/crew/workspace/.kiro/steering/downshift-compact.md`, a dedicated steering file with `inclusion: always` frontmatter. Setup owns the whole file.
- **Cursor**: not installed by setup. Cursor reads rules only from the project (`.cursor/rules/*.mdc`, with `alwaysApply: true` in the frontmatter), and its User Rules live in Settings, not in a file under `~/.cursor`. To add it to a project, copy the steering text from `cmd/downshift/assets/compact-steering.md` into `.cursor/rules/downshift-compact.mdc` with that frontmatter.

If a harness you use reads a different global file, the same block can be pasted there by hand between the two markers.

## MCP server (`setup`, on by default; `--no-mcp` skips it)

`downshift setup` also registers `downshift mcp serve` as the MCP server `downshift` in Claude Code and Codex. The dry run shows it like any other change; `--no-mcp` leaves both configurations alone. **Claude Code**: setup runs `claude mcp add -s user downshift -e DOWNSHIFT_MANAGED=setup -- <this binary> mcp serve` and reads `~/.claude.json` first to see whether the entry exists (it never starts a server). The `DOWNSHIFT_MANAGED=setup` env marks the entry as setup's: "up to date" and `--undo` require the marker, the command and the arguments, and an entry named `downshift` without them is reported as a warning and left as it is; setup still succeeds. **Codex**: a block between `# downshift:mcp:begin` and `# downshift:mcp:end` in `~/.codex/config.toml`, edited as text so your comments and other servers stay; any other definition of `downshift` outside the markers (table, dotted key or inline table) is a warning, the file is left as it is, and setup still succeeds. The printed diff shows only the lines inside the markers. The binary must pass the same check as the hooks (`--allow-unsafe-binary` overrides). `--dry-run` prints the command and the diff and writes nothing; `--undo` removes only the downshift entry and block.

**Cursor, Antigravity and KiroCrew** get the same entry through their `mcpServers` JSON map (`~/.cursor/mcp.json`, `~/.gemini/config/mcp_config.json`, `~/.kiro/settings/mcp.json`): only the `downshift` member is written, every other server and key keeps its text and order, and `--undo` removes just that member. **Claude Desktop Chat** is registered the same declarative way, in `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS; the `<UserConfigDir>/Claude` path): Claude Desktop is an MCP host with no hooks, skills or steering, so setup only adds the server there and leaves unrelated keys such as `coworkUserFilesPath` byte-for-byte. Setup never creates the file when the Claude directory is absent. Restart Claude Desktop to pick the server up; from Chat only the read and apply tools work (providers, status, playbook, apply), since Chat has no subagent spawns to route.

## Experimental harnesses

> **Experimental (proof of concept).** KiroCrew runs in policy mode only (block and retry, no in-place rewrite) and Antigravity's rewrite rides an `overwrite` envelope that is not a documented vendor field; each has one observed child on one build and neither records a child end. Every feature is still computed for them (layering rule, `cmd/downshift/conformance_test.go`), and the adapters, tests and hooks stay in the repository, but treat them as proofs of concept, not as supported installs. Install them only if you accept that.

### Antigravity (Google DeepMind)

**Status: Experimental.** One observed child on desktop 2.21.1 ([evidence](../evidence/antigravity-executor-ack.md)); no child end recorded.

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

When Antigravity attempts to spawn a subagent (e.g. inheriting the parent model or requesting `pro`), `downshift antigravity` intercepts the tool call, evaluates the **last** entry in `Subagents` that has a non-empty `Prompt` (not necessarily `Subagents[0]`; trailing rows without `Prompt` are skipped), and rewrites **that** subagent's `Model` (`flash_lite` for mechanical tasks, `flash` for the mid tier) through an `overwrite` envelope that is not a documented vendor field, logging the routing event to `~/.downshift/events.jsonl`.

### KiroCrew (policy mode through the native Kiro CLI hook)

**Status: Experimental.** Block and retry observed once on Kiro CLI 2.29.0; no in-place rewrite ([evidence](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/kirocrew/findings/F5-kirocrew-native-hook.md)).

For Kiro CLI 2.29, register Downshift through KiroCrew's persistent
`agent.kiro_hooks` configuration. KiroCrew generates the agent spec consumed
by Kiro CLI, whose native `preToolUse` hook runs before the tool: `exit 0`
allows it; `exit 2` blocks it and returns stderr to the agent. There is no
`updated_input` channel, so the adapter uses **policy mode**. It evaluates a
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
model from the parent's model or a UI title. The decider is deterministic;
the agent must retry a blocked spawn with the recommended model.

#### Setup

**Automatic.** Run `downshift setup kirocrew --dry-run`, read the diff, then `downshift setup kirocrew`. Setup:

* writes a wrapper to `~/.downshift/bin/kirocrew-pre-tool-use.sh` (`.cmd` on Windows) that runs the binary doing the setup, passes the payload on stdin and the exit code through, and exits 0 with a line on stderr when the binary is gone, so a removed install never breaks KiroCrew;
* adds that wrapper to `agent.kiro_hooks.preToolUse` in `~/.kiro/crew/config.json`, next to the hooks already there. A `kirocrew-pre-tool-use` wrapper you registered by hand, from any directory, is pointed at the generated one instead of being added twice;
* installs the compact steering;
* removes a Downshift row from `~/.kiro/crew/hooks.json` when that file exists. That list does not enforce policy. The row duplicates `agent.kiro_hooks` and goes stale when it still points at a checkout. Other rows in the file stay. Quit KiroCrew before setup and start it again afterwards: a running Crew keeps that file in memory and writes the old copy back on the next hook run.

It edits `config.json` only if the file exists (KiroCrew creates it; a missing file means KiroCrew is not set up, and setup says so). It writes the wrapper before the config and skips the config if the wrapper fails. `downshift setup --undo` restores both. Then restart KiroCrew so it regenerates its agent spec, and run `downshift doctor`.

Run it from your own terminal. If KiroCrew's policy blocks an agent session from writing its own `config.json`, setup exits 1 with `write failed … operation not permitted`, leaves the wrapper in `~/.downshift/bin/` and the config untouched, and keeps the backup it took first (`config.json.downshift-bak-…`, which you can delete); run it again yourself.

**Manual.** The same result by hand, from a checkout of this repository:

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
             "command": "/absolute/path/to/downshift/scripts/kirocrew-pre-tool-use.sh"
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
   selectable in your installed Kiro CLI. Follow [session-models.md](../reference/session-models.md)
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
`downshift setup kirocrew` removes a Downshift row from that file. Do not
turn the row back on: enabling it does not make the exit code enforce policy.

#### Acceptance evidence and coverage

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
[the build-specific evidence](https://github.com/tiagovilasboas/downshift-labs/blob/main/reverse/kirocrew/findings/F5-kirocrew-native-hook.md).

KiroCrew's Agent Dashboard app shows the run history and health of your agents.
It reads the agent run records of the ecosystem it is pointed at (`runs.db`),
not Downshift's state directory, so it does not show routing decisions or
savings. Use `downshift stats` for those.

## dsmon — live monitor widget

> **Prefer `downshift watch`.** The live session tree is built into the `downshift`
> binary (no separate build, every harness): see [session-view.md](session-view.md#live-downshift-watch).
> `dsmon` stays buildable and is documented below.

`cmd/dsmon` is a separate command inside this repo — a floating terminal widget
that shows model switches, harness, tier distribution and estimated tokens saved
in real time. It reads `~/.downshift/events.jsonl` and refreshes every
800 ms. Standard library only.

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
│  est. saved  ~7K tokens ¹                            │
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
| Est. saved (tokens) | Tokens that ran on a cheaper model than requested, the shared `telemetry.TokenSavings` view | Observed where the harness reports tokens; not a bill |
| Tokens | Observed child tokens, when the harness reports them | Not from a provider invoice |

**Token consumption note.** The `PreToolUse` routing hook does not receive
token usage. On Claude Code, a separate `SubagentStop` hook
(`downshift claude-code-subagent-stop`) reads the completed child's transcript
and records its reported token counts and model; see
[session-models.md](../reference/session-models.md#claude-code-real-usage).
Current Downshift views attribute and compare those tokens, with no dollar
amounts. Tokens moved to a cheaper model do not prove that fewer tokens were
consumed or that the bill was lower. Check the provider's billing dashboard
separately for charges.

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
Choose each role's tier and reasoning effort in config once.

> If a future Grok build adds `updatedInput` to `PreToolUse` (or a multi-model
> catalog), a `downshift grok` hook adapter becomes a drop-in — the decider
> and policy are already harness-agnostic. Until then, config is the right and
> documented lever.
## Plan compatibility — read before installing

**Not every plan supports subagent model routing.** This is a hard constraint
at the harness level, not a bug in Downshift. Evidence matrix: [harness-matrix.md](../reference/harness-matrix.md).

### Claude Code

| Plan | Subagents exist? | Model routing works? |
|---|---|---|
| Free | ❌ No real subagents | — |
| Pro / Max / Teams | ✅ Yes | ✅ Observed 2026-10-06 (API-reported child model) |
| API (direct) | ✅ Yes | Expected (same hook path); the 2026-10-06 evidence did not record the plan |

Claude Code free runs in a sandboxed container without network egress — `go install` will fail. The subagent feature itself only exists on paid plans.

### Cursor

| Plan / pricing | Subagents exist? | Model routing works? |
|---|---|---|
| Free | ✅ Limited | ⚠️ `updated_input.model` silently ignored (known issue) |
| Pro (request-based / legacy) | ✅ Yes | ⚠️ `model` field in Task only accepts `"fast"` — override silently dropped |
| Pro / Ultra (usage-based) | ✅ Yes | ⚠️ Rewrite emitted; child model unconfirmed ([P1.6](../reference/harness-matrix.md)) |

Cursor's `preToolUse` hook fires and Downshift runs, but the `model` rewrite is silently discarded on legacy and free plans. [Tracked on Cursor forum](https://forum.cursor.com/t/pretooluse-hook-updated-input-is-silently-ignored-for-the-task-tool/151985). On usage-based Pro/Ultra plans where subagent model selection is expanded, Downshift emits the rewrite; whether the harness runs the child on that model is unconfirmed (no child-model proof yet — [P1.6](../reference/harness-matrix.md), matrix row *Unconfirmed*).

### Codex

| Plan | Subagents exist? | Model routing works? |
|---|---|---|
| Any (with `multi_agent_v2` enabled) | ✅ Yes | ✅ Inferred 2026-10-02 (next spawn asked for the written model), not observed |

Requires `[features] multi_agent_v2` and `hooks = true` in `config.toml`.

### Summary

Downshift serves every harness above from the same core, and **Claude Code is the primary harness**: the most complete (inferred session model, usage and child-end hooks, `/downshift:status`, statusline) and the most verified. Pick it when you can choose.

Downshift is most effective on:
- **Claude Code Pro / Max / Teams / API** — full model routing via hook
- **Codex** with multi_agent_v2 enabled — model + reasoning_effort emitted; honored is inferred, not observed
- **Cursor Pro/Ultra** on usage-based plans — rewrite emitted; child model unconfirmed ([P1.6](../reference/harness-matrix.md))

On free plans or legacy Cursor pricing, the hook runs but model rewrites may be silently ignored by the harness. The tool fails open — the subagent still spawns, just without the model change.

## Troubleshooting

### 404 when downloading (no archive for this release)

`install.sh` prints `error: release <tag> has no <os>/<arch> archive yet`.
The tag exists, but its release has no archive for your platform (for example a
release created before its build finished). Nothing is wrong on your machine,
and the installer never falls back to another version on its own. Options:

1. Pick another tag from [Releases](https://github.com/tiagovilasboas/downshift/releases), then
   `curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh -s <tag>`.
2. With Go installed: `go install github.com/tiagovilasboas/downshift/cmd/downshift@<tag>`
   (the binary lands in `~/go/bin`).
3. Report it at https://github.com/tiagovilasboas/downshift/issues.

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

### Task spawn held with R9 / `requested_model: unknown` (Cursor)

On recent Cursor builds, Task `preToolUse` may carry `session_id` but leave
`model` and `model_id` empty. Without a warm composer cache, Downshift keeps the
spawn unchanged (`R9_UNCONFIDENT_UNKNOWN_MID`). Fix:

1. Run `downshift setup cursor --yes` so `sessionStart` and `beforeSubmitPrompt`
   call `downshift cursor-touch` (or add those hooks manually).
2. Start a **new** Cursor session (hooks load at session start).
3. Send at least one user message (warms cache via `beforeSubmitPrompt`) before
   spawning a Task subagent.
4. Confirm `~/.downshift/cursor-composer-model.json` exists and
   `downshift doctor --json` shows cursor hooks wired.

Honor of the rewrite is still [unconfirmed](../reference/harness-matrix.md) even
when routing emits a rewrite.

### No output on stderr — hook is not firing

Check the matcher. Claude Code's subagent tool is `Agent` (older builds: `Task`),
so the matcher must be `"Agent|Task"`, as `setup` and the plugin write it.
Confirm the binary itself with:

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

