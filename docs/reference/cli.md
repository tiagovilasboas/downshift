# CLI Reference

Complete reference for all Downshift commands.

## Installation & Setup

### `downshift setup`

Configure Downshift hooks for your harnesses.

```bash
downshift setup [harness...] [flags]
```

**Flags:**
- `--dry-run` — Preview changes without applying
- `--undo` — Restore from backup
- `--yes`, `-y` — Skip confirmation prompts
- `--no-plugin` — Skip plugin installation (Claude Code)
- `--no-session-policy` — Skip session policy configuration
- `--no-mcp` — Do not register the MCP server (`downshift mcp serve`) in Claude Code (through `claude mcp add -s user`) and Codex (a marked block in `~/.codex/config.toml`). Registration is on by default and shown in the dry run first; `--undo` removes only what setup added. `--mcp` is still accepted

**Examples:**
```bash
downshift setup              # Configure all detected harnesses
downshift setup claude-code  # Configure only Claude Code
downshift setup --dry-run    # Preview changes
downshift setup --undo       # Restore previous configuration
```

### `downshift doctor`

Diagnostic health check.

```bash
downshift doctor [flags]
```

**Flags:**
- `--verbose` — Show detailed output
- `--json` — Output as JSON

**Checks:**
- Binary location and version
- Hook configuration
- Permissions
- State directory
- Catalog validity

---

## Daily Use

### `downshift session`

Show savings for the current session.

```bash
downshift session [flags]
```

**Flags:**
- `--tree` — Interactive tree view of subagents
- `--json` — Output as JSON

### `downshift stats`

Show global statistics.

```bash
downshift stats [flags]
```

**Flags:**
- `--days=N` — Limit to last N days (default: all)
- `--dci` — Include DCI telemetry
- `--routing` — Show routing breakdown
- `--json` — Output as JSON

### `downshift report`

Generate a savings report.

```bash
downshift report [--days=N] [--json]
downshift report clear [--dry-run] [--yes]
```

**Flags:**
- `--days=N` — Window in days (default: 30, the same window as the Agents panel)
- `--json` — Output JSON instead of text

Any other flag exits 2 with the usage line.

`downshift report clear` starts the measurement from zero. It moves every measurement log (routing and child-end
events, exec and compaction records, context sensor, loop, DCI and probe logs, compressor learning) to
`backups/report-clear-<time>` in the state directory; nothing is deleted, so moving the files back undoes it.
Config, policy, session models, model discovery and quota are kept. It asks first (default No) and refuses
without a terminal unless `--yes` is passed; `--dry-run` lists the files without moving them.

**Examples:**
```bash
downshift report --days=7
downshift report --days=30 --json
```

The report ends with two sections before its final figure. They carry no money and no `$`.

- **Why spawns changed**, per harness: the spawns in the window counted by routing verdict
  (`kept`, `profile`, `rule`, `hint_down`, `default`, `guard_floor`, `held_unknown`, `refused_config`)
  and the top reasons (`rule:<id>`, `profile:<name>`, `<verdict>:<reason>`). It is built from the
  routing note each decision event holds (enums, tiers and declared ids only). A decision with no note
  is counted as "without a routing note", never guessed. Harnesses with no events are listed as `no events`.
- **Tokens per harness**: one line per harness, `tokens avoided (Compactor)` and `tokens moved to a
  smaller model` in two columns that are never added, then `quota used (5h · week)` from the native
  snapshot (`unknown` when none exists, and marked when the snapshot has expired). A harness with no
  usage bound to a decision shows `unobserved`, not 0.

`--json` carries the same data as `why_spawns_changed` and `tokens_by_harness`. An empty event log
prints a plain sentence for each section and exits 0.

`downshift session --panel` adds `routing_why`: one entry per spawn of the session with the requested
model, the final model, the verdict and the reason, plus `cell`, the same in one short line
(`requested → final · rule:fast-lane`, or `note not recorded`).

### `downshift watch`

Live terminal dashboard of routing events.

```bash
downshift watch
```

Press `q` to quit.

---

## Output Compaction

### `downshift compactor`

Run a command with output compaction.

```bash
downshift compactor -- <command> [args...]
```

**Aliases:** `compact`, `exec`

**Examples:**
```bash
downshift compactor -- go test ./...
downshift compactor -- cargo build --release
downshift compactor -- git status
```

**Exit codes:**
- Returns the wrapped command's exit code
- Compaction only happens on exit 0 with empty stderr
- 126 when the command is outside the allowlist (nothing runs)

**Allowlist query** (`--allowlisted`): answers whether argv is allowlisted
without running it. Exit 0 when it is, 1 when it is not, 2 without the `--`.
Prints nothing and writes no exec log row. The shell helper from
`downshift enable` calls it before wrapping.

```bash
downshift compactor --allowlisted -- go test ./...   # exit 0
downshift compactor --allowlisted -- docker run x    # exit 1
```

### `downshift enable`

Print a POSIX sh script that defines shell functions for common tools (go,
cargo, npm, git, docker, kubectl, pip, gh, ...). Add it to your shell rc:

```bash
eval "$(downshift enable)"
```

**Alias:** `shell-init` (legacy)

Each function asks the binary whether the command is allowlisted
(`compactor --allowlisted`, one extra fork, no run, no log row). An allowlisted
command runs as `downshift compactor -- <command>`; any other command runs as
typed with `command`, so `git -C <dir> status`, `npm --version` or
`docker run` behave exactly as without the helper. The functions only act
when stdout is not a terminal (agent context, pipes, `$(...)`, CI); an
interactive terminal always sees the raw output.

**Environment variables:**
- `DOWNSHIFT_SHELL_ALIASES=0` — the script defines nothing
- `DOWNSHIFT_FORCE_COMPACT=1` — compact in an interactive terminal too

After upgrading Downshift, re-run `eval "$(downshift enable)"` (or open a new
shell) so the functions pick up the current script.

---

## Compressor Learning

### `downshift learning suggestions`

Show compressor candidates sorted by impact.

```bash
downshift learning suggestions [flags]
```

**Flags:**
- `--limit=N` — Maximum suggestions to show (default: 10)
- `--json` — Output as JSON

### `downshift learning stats`

Show learning store statistics.

```bash
downshift learning stats
```

### `downshift learning export`

Export the learning store as JSON: signatures, counts and output shapes, never output text.

```bash
downshift learning export
```

### `downshift learning clear`

Reset the learning store.

```bash
downshift learning clear
```

---

## Model Routing

### `downshift route`

Show what Downshift would apply for one spawn, given the orchestrator's choice. It never picks a model on its own and
accepts no prompt text.

```bash
downshift route --harness=ID [--model=ID] [--effort=E] [--tier=T] [--agent-type=T] [--dir=DIR] [--json]
```

**Flags:**
- `--harness` — the harness id (required); an unknown id exits 2 and lists the valid ones
- `--model` — the model the orchestrator chose (the request)
- `--effort`, `--tier` — the orchestrator's effort and tier hint (a hint only moves a request down)
- `--agent-type` — the subagent type, when it matters to a declared profile
- `--dir` — the project directory whose `.downshift/policy.json` applies
- `--json` — a stable object: `schema_version` 1, `harness`, `requested_model`, `selected_model`, `tier`, `effort`,
  `verdict`, `reason`, `applied` and `note` (enums, tiers and numbers only)

A model the harness catalog does not know, or an invalid policy file, keeps the requested model and says so in
`reason`. The hook, this command and the MCP tool `apply` use the same code.

**Examples:**
```bash
downshift route --harness=claude-code --model=<id> --tier=small
downshift route --harness=codex --json
```

### `downshift playbook`

The working rules a delegated worker receives: named markdown sets, `General` always applying.

```bash
downshift playbook list | show <name> | edit <name> | add <name> | remove <name> | restore <name> | check
```

`edit` and `add` open the file in `$VISUAL` or `$EDITOR`. Files live in `<state dir>/playbooks/<id>.md`, with `# Name`
on the first line; names match regardless of case and accents. `General` cannot be removed; a shipped playbook stays
removed until `restore`. A playbook file that is a symbolic link is refused. `check` warns above 8000 characters.

### `downshift mcp serve`

An MCP server on standard input and output, started by the harness that registered it (see `downshift setup --mcp`).

```bash
downshift mcp serve
```

It answers both protocol eras (the `initialize` handshake of 2025-11-25 and earlier, and the per-request metadata with
`server/discover` of 2026-07-28). Tools, named by their verb under the server name `downshift`: `providers {harness}`
(the models this harness offers now), `playbook {name?}`, `status {}` (quota advice per harness, plus the workers this
session delegated: id, state, agent and harness, read or write, elapsed, tokens once known, and a short summary; the
active ones and the most recent finished ones, with a count of any left out) and
`apply {harness, model?, effort?, tier?, agent_type?, dir?}` (what would be applied, as `downshift route` computes it).
Every successful result is a JSON object with `data` and a `strain` block. The server never picks a model and accepts no
task text; `dir` must be inside the directory the server was started in; string arguments are capped at 256 bytes (`dir`
at 4096) and a result at 64 KiB. A line that is not JSON, or a message above 1 MiB, ends the session.

### `downshift models`

Manage model catalog.

```bash
downshift models <subcommand>
```

**Subcommands:**
- `discover` — Discover models from current session
- `check` — Verify catalog pricing
- `pull` — Write new provider models to the user catalog, with tier unknown
- `available` — Show the models a spawn can use now on one harness, and where each came from

#### `downshift models available`

```bash
downshift models available --harness <id> [--json]
```

Lists the detected models for one harness: id, tier (`small`, `mid`, `frontier` or `unknown`), efforts when known and
source (`payload`, `discovery`, `file`, `catalog-fallback`). A model with no tier evidence is marked untiered: it is
never the target of a hint or a rule, though the orchestrator may name it. The text output starts with when the
discovery cache was written. With no list it prints `no detected list for <id>; run downshift models discover
--harness <id>` and exits 0. `--json` prints `harness`, `models[]`, `fallback` (true when only the catalog stand-in
exists) and `cache_written_at` when known. An unknown harness exits 2 and names the valid ids. It reads local files
only. `downshift doctor` reports the same list as the `available models` check.

---

## Code Intelligence (DCI)

### `downshift dci enable`

Enable code intelligence for the current repository (`.downshift/dci.json`).
DCI only acts where its prompt hook runs; `downshift setup` installs it for
Claude Code (`UserPromptSubmit`), and the Claude Code plugin ships the same hook.

```bash
downshift dci enable [active|observe] [--budget=N] [--dry-run]
```

`--dry-run` prints the config it would write and the prompt hooks setup
installs, and writes nothing.

### `downshift dci disable`

Disable code intelligence.

```bash
downshift dci disable
```

### `downshift dci impact`

Query blast radius for a symbol.

```bash
downshift dci impact <target> [flags]
```

**Flags:**
- `--json` — Output as JSON

### `downshift dci inspect`

Dry-run context retrieval.

```bash
downshift dci inspect "<prompt>" [flags]
```

**Flags:**
- `--json` — Output as JSON

### `downshift dci benchmark`

Measure indexing performance.

```bash
downshift dci benchmark
```

---

## Advanced

### `downshift context compress`

Low-level stdin compaction.

```bash
downshift context compress [flags]
```

**Flags:**
- `--exit=N` — Pretend exit code is N (default: use real)

### `downshift version`

Print version information.

```bash
downshift version
```

---

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DOWNSHIFT_EXEC` | Enable/disable compaction | `1` |
| `DOWNSHIFT_ROUTE` | Enable/disable routing | `1` |
| `DOWNSHIFT_LEARNING` | Enable/disable learning | `1` |
| `DOWNSHIFT_CONTROL_SHARE` | Control group share | `0.10` |
| `DOWNSHIFT_STATE_DIR` | State directory | `~/.downshift` |
| `DOWNSHIFT_LOG_LEVEL` | Log verbosity | `info` |

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Usage/argument error |

For `downshift compactor`, the wrapped command's exit code is returned.
