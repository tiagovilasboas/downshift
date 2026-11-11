# CLI Reference

Complete reference for all Downshift commands.

## Installation & Setup

### `downshift setup`

Configure Downshift hooks for your harnesses. In a terminal, `downshift setup` opens the setup
wizard in four steps: pick the harnesses (detected ones start checked), pick the components (space
toggles, `a` all, `n` none), scroll the diff file by file, then confirm. Removing other `downshift`
binaries is off unless you check it, and the confirm defaults to No when it is checked. `esc` goes
back a step, `ctrl+c` cancels; nothing is written before the confirm. After applying, the wizard
prints the equivalent `downshift setup ...` command. Naming harnesses narrows the first step, and
flags set the starting state of the second. `--yes`, `--dry-run` and `--undo` skip the wizard, for
scripts, CI and agents; without a terminal, setup prints the diff and asks `[y/N]`.

```bash
downshift setup [harness...] [flags]
```

**Flags:**
- `--dry-run` — Preview changes without applying
- `--undo` — Restore from backup
- `--yes`, `-y` — Skip the wizard and the confirmation prompt
- `--tui` — Open the wizard even when stdin is not a terminal
- `--no-tui` — Plain prompts even in a terminal
- `--no-plugin` — Skip plugin installation (Claude Code)
- `--no-mods` — Skip the downshift-agents mod (Claude Code)
- `--no-session-policy` — Skip session policy configuration
- `--no-skills` — Skip the Downshift skills
- `--no-steering` — Skip the harness instructions file (steering and agent triggers). With `--undo`, `--no-skills` and `--no-steering` also skip restoring those files
- `--no-policy` — Do not offer `.downshift/policy.json` in the current directory. Setup never offers it from the directory that holds the state directory (usually HOME): there that path is the user policy, so setup prints `policy: skipped:` and plans nothing
- `--keep-binaries` — Keep other `downshift` binaries on PATH
- `--you-should-know` — Turn on the "You should know" mod (each check is a billed background request)
- `--no-mcp` — Do not register the MCP server (`downshift mcp serve`) in Claude Code (through `claude mcp add -s user`) and Codex (a marked block in `~/.codex/config.toml`). Registration is on by default and shown in the dry run first; `--undo` removes only what setup added. `--mcp` is deprecated (registration is the default) and still accepted

**Examples:**
```bash
downshift setup              # Setup wizard for all detected harnesses (in a terminal)
downshift setup claude-code  # Setup wizard for Claude Code only
downshift setup --dry-run    # Preview changes
downshift setup --undo       # Restore previous configuration
```

### `downshift update`

Install the latest release over this binary, then refresh every harness.

```bash
downshift update [--check] [--yes] [--version vX.Y.Z]
```

**Flags:**
- `--check` — Say whether a newer release exists; change nothing
- `--yes`, `-y` — Pass `--yes` to setup: write the harness files without asking
- `--version vX.Y.Z` — Install this release instead of the latest

Steps: resolve the tag from the GitHub releases page (no API call), download the archive for this platform, check its sha256 against `checksums.txt` and, with the `gh` CLI, its build provenance, replace the binary in place (on Windows the running file is moved to `downshift.exe.old`, removed on the next update), then run the new binary's `setup` and `doctor`. A current binary skips the download; setup still runs. Every download is https only.

### `downshift uninstall`

Restore the harness hook configurations setup changed and remove the wrappers it generated.

```bash
downshift uninstall [--dry-run] [--yes] [--purge]
```

**Flags:**
- `--dry-run` — Show what would be restored, write nothing
- `--yes`, `-y` — Do not prompt before restoring
- `--purge` — Also remove the `~/.downshift` state directory and every backup file

It runs the same restore as `downshift setup --undo`, then removes the generated wrappers.

### `downshift doctor`

Diagnostic health check.

```bash
downshift doctor [flags]
```

**Flags:**
- `--verbose` — Show detailed output
- `--json` — Output as JSON
- `--quiet` — Print only the failed and warning checks
- `--harness NAME` — Check the selected harness alongside global checks. File-based MCP registration confirms configuration only, not a live connection.
- `--self-test` — With `--harness`, check the compactor, a local MCP tool call, and a synthetic hook probe. These do not prove the host application loaded or invoked its hook.
- `--check-latest` — Also ask GitHub for the latest release (one request, 2 s timeout, cached 24 h); offline is "unknown", never a failure

**Checks:**
- Binary location and version
- Hook configuration
- Permissions
- State directory
- Catalog validity

---

## Daily Use

### `downshift` (no arguments): the console

In a terminal, a bare `downshift` on a configured machine opens a full-screen
console: this session at a glance, the health checks that need a fix, and a
command list (`:` or `Ctrl-K`) that runs every command on this page, including
`report`. Commands that write ask first (default no); commands that prompt or
keep running get the terminal and return to the console when they exit.

Keys: `:` commands, `r` refresh, `q` quit; in the list, type to filter, words
after a name are passed as flags (`report --days=3`), `enter` runs, `esc` closes.

In a pipe, a script, or with `DOWNSHIFT_PLAIN=1`, it prints the plain text
summary instead. Colour follows the terminal theme and turns off with `NO_COLOR`.

### Read-only commands open in the console

When a person runs a read-only command in a terminal (`downshift session`,
`downshift doctor`, `downshift stats`, `downshift report`, ...), it opens the
console on that command's output. `r` reruns it; `q` or `esc` quits, and the
output is printed to the terminal with the command's own exit code.

Everything else keeps today's plain text, byte for byte (pinned by golden
tests). One shared decision (`internal/console/surface`) picks plain when any
of these holds, in this order:

1. `DOWNSHIFT_PLAIN` is set (the console's own child commands always set it);
2. a machine flag is present: `--json`, `--brief`, `--plain`, `--panel`,
   `--export`, `--quiet`, `-o`;
3. the command writes, takes over the terminal (`setup`, `watch`, ...), is a
   hook entry, or is not in the console's command list;
4. stdin or stdout is not a terminal (pipes, redirects, an agent's Bash tool);
5. `CI` is set, or `TERM=dumb`;
6. a harness's agent shell variable is set (see
   [harness-matrix.md](harness-matrix.md#console-or-plain-output-per-harness)).

`--plain` is the escape hatch for a person: `downshift session --plain` prints
the plain text in a terminal.

### `downshift session`

Show savings for the current session.

```bash
downshift session [flags]
```

**Flags:**
- `--tree` — The subagents as a tree (the default)
- `--why` — Why the latest decision was made, in plain words
- `--pause`, `--resume` — Classify and log only (up to 12 hours), and undo that
- `--session=ID` — Session id (default: the harness's own, else the most recent)
- `--json` — Output as JSON
- `--panel` — Per-spawn state and session savings as JSON, for a panel mod
- `--no-report` — With `--panel`: skip the 30-day report
- `--brief` — With `--panel`: only the fields the agents band draws

### `downshift stats`

Show global statistics.

```bash
downshift stats [flags]
```

**Flags:**
- `--days=N` — Look back N days (default: 30; `--days=0` is all time)
- `--by-harness` — Markdown table of spawns, tier and effort per harness
- `--detail` — Full report: units, honor, quota, usage and per-session tokens
- `--export` — Print a pasteable JSON summary without prompts

### `downshift accept`

Record whether the developer kept the code that subagents delivered.

```bash
downshift accept [--session=ID] [--used | --discarded]
```

**Flags:**
- `--session=ID` — Session hash to record (default: the most recent session)
- `--used` — Outputs were accepted and kept (the default)
- `--discarded` — Outputs were reverted or rewritten

The result feeds the acceptance figures of `downshift loop report` and `downshift stats`.

### `downshift loop`

Run an engineering loop: do a task, run a sensor command, and retry on failure, moving up a tier
(cascade) when a cheaper one keeps failing.

```bash
downshift loop [--task=DESC] [--sensor=CMD] [--iterations=N] [--timeout=DUR] [--session=ID] [--model=ID] [--json] [-- <sensor-cmd>]
downshift loop --batch=tasks.json [--continue-on-error] [--json]
downshift loop report [--session=ID] [--json]
```

**Flags:**
- `--task` — Task description or goal
- `--sensor` — Command that verifies the result (or write it after `--`)
- `--iterations` — Maximum iterations before the tripwire stops the loop (default: 3)
- `--timeout` — Maximum duration of the loop (default: 5m)
- `--session` — Session id for telemetry and cascade tracking
- `--model` — The model running the task, when known (used to pick the cascade step)
- `--batch` — Run the tasks of a JSON array `[{"task","sensor","model"}]` in order; stops at the first failure
- `--continue-on-error` — With `--batch`, keep going after a failed task
- `--json` — Print the result as JSON

`downshift loop report` prints the used and discarded summary of loop and `accept` runs. See the
[engineering loop guide](../guides/engineering-loop.md).

### `downshift config`

Print every effective setting with its value and where it came from (default, user file, project file or environment).

```bash
downshift config [--json]
```

**Flags:**
- `--json` — Machine-readable output

### `downshift report`

Generate a savings report. In a terminal it opens a report screen: a summary first (tokens saved,
share that ran below the session model and the part of it a Downshift rewrite caused, premium preserved and rework, one block per harness, never summed),
then the Savings, Routing, Efficiency and Quota tabs (`←`/`→` or `1`-`5`, `↑`/`↓` to scroll, `q`
quits). A harness with no routing evidence takes one line. `--text`, `--json`, `--markdown`,
`--plain`, `DOWNSHIFT_PLAIN`, an agent shell or a non-terminal print the plain report instead
(markdown by default).

```bash
downshift report [--days=N] [--json|--text|--markdown]
downshift report --daily [--days=N] [--json]
downshift report --history [--by=day|week] [--json]
downshift report clear [--dry-run] [--yes]
```

**Flags:**
- `--days=N` — Window in days (default: 30, the same window as the Agents panel)
- `--daily` — One row per calendar day of the window (local time zone, today included), with per harness the Compactor runs (`runs`, `compacted`), the tokens it kept out of the context (`saved`, net of what `retrieve` read back) and the billable tokens that ran below the session model (`below`). Tokens only, never summed across columns or harnesses
- `--history` — The whole retained history instead of a window, by week (Monday start, the default) or by day with `--by=day`. Cannot be combined with `--days` or `--daily`
- `--json` — Output JSON instead of text
- `--text` — Plain text columns
- `--markdown` — Markdown (the default when output is not a terminal)

Any other flag exits 2 with the usage line.


**Examples:**
```bash
downshift report --days=7
downshift report --days=30 --json
```

Under the routed table the report breaks **rework by routing verdict** down for the routed
spawns. Rework is a failed spawn (error or interrupt) that a later
spawn redid in the same session within 10 minutes, the same count as the `rework` column above. Each verdict
row shows its spawns, ended spawns, completed tasks (usage observed, as in the table) and rework rate. Each
row names the policy key that turns that verdict off (`class`: `routing.by_class: false`; `class_up`:
`routing.raise_complex: false`; `agent_default`: `routing.agent_default_tier: false`; `rule`: `routing.rules[].enabled: false`; `profile`: `routing.profiles`; `guard_floor`:
`min_tier`, `min_tier_by_class` or `min_tier_by_risk`). A verdict with no such key (for example `hint_down`
or `held_after_failure`) says so. The report flags no verdict and never changes the policy.
Decisions with no routing note are one `not_recorded` row, and a spawn the class held after a recent failure is
the `held_after_failure` row. `--json` carries the rows as `rework_by_verdict`, each with
`reworked_correlation_ids`: the `correlation_id` of every decision of that verdict that was followed by
rework, so the misrouted spawns can be found in the event log.

The report opens with **COMPACTION SAVINGS PER HARNESS**, per harness and never summed, before the
routed table. Each harness carries what the agents pane shows for a session, here for the whole window:
tokens saved, the share of raw output cut (bytes), runs and compacted, hook runs against direct calls,
bytes read back with `retrieve` (already subtracted), the spawns whose model or effort changed or that
were held, and the share of subagent billable tokens that ran below the session model. `--json` carries
these rows as `summary_by_harness`. Then it ends with more sections. They carry no money and no `$`. There is no cross-harness total line.

- **Why spawns changed**, per harness: the spawns in the window counted by routing verdict
  (`kept`, `profile`, `rule`, `hint_down`, `class`, `class_up`, `default`, `agent_default`, `guard_floor`, `held_unknown`, `refused_config`)
  and the top reasons (`rule:<id>`, `profile:<name>`, `<verdict>:<reason>`). It is built from the
  routing note each decision event holds (enums, tiers and declared ids only). A decision with no note
  is counted as "without a routing note", never guessed. Harnesses with no events are listed as `no events`.
- **Quota used per harness**: one line per harness, `quota used (5h · week)` from the native
  snapshot (`unknown` when none exists, and marked when the snapshot has expired). The routing tokens
  are the `Routing: tokens on a lighter model, per harness` section of the markdown report (the text report keeps the `Routing tokens, by harness` block) (`ran below the session model` first, then `of which by a Downshift rewrite`,
  `unobserved` for a harness with no rankable spawn, never 0), and the Compactor tokens are the leading
  section; neither is repeated here.

The **Efficiency per harness** section follows the Compactor section: premium capacity preserved (completed tasks that ran below the frontier tier), spawns by the tier they ran on, and the routed table's completed and rework figures (`efficiency` in `--json`; see [report.md](../guides/report.md#efficiency)).

`--json` carries the same data as `why_spawns_changed`, `tokens_by_harness` and
`compaction_by_harness` (the leading section's per-harness rows). An empty event log prints a plain
sentence for each section and exits 0.

`downshift session --panel` adds `routing_why`: one entry per spawn of the session with the requested
model, the final model, the verdict and the reason, plus `cell`, the same in one short line
(`requested → final · rule:fast-lane`, or `note not recorded`).

#### `downshift report clear`

```bash
downshift report clear [--dry-run] [--yes]
```

Starts the measurement from zero. It moves every measurement log (routing and child-end
events, exec and compaction records, context sensor, loop, DCI and probe logs, compressor learning) to
`backups/report-clear-<time>` in the state directory; nothing is deleted, so moving the files back undoes it.
Config, policy, session models, model discovery and quota are kept. It asks first (default No) and refuses
without a terminal unless `--yes` is passed; `--dry-run` lists the files without moving them.

**Flags:**
- `--dry-run` — List the files without moving them
- `--yes`, `-y` — Do not ask (required without a terminal)

### `downshift export`

Print the one artefact an outside operator shares after a measurement week (measurement-week issue).

```bash
downshift export [--days=N]
```

**Flags:**
- `--days=N` — Window in days (default: 30; `--days=0` is all time)

Any other flag exits 2 with the usage line.

It reads the same windowed events as `downshift report` and prints JSON with counts and token
totals only: per-group `spawns`, `ended`, `completed` and `reworked`, the token totals
(`routed_billable`, `control_billable`, `moved_to_smaller_model`, `compactor_avoided`), a per-harness
list and a fixed note. It never prints prompts, task text, file paths, session ids, model names,
hostnames or usernames; the only free string is a harness name. Before printing, the command checks
its own output against a strict allowlist of field names and values and refuses to emit anything that
does not pass (the check is tested in `internal/operatorexport`). See the
[operator guide](../guides/operator.md) for the full week.

This differs from `downshift learning export`, which dumps the compressor-learning store; `downshift
export` is the privacy-safe counts-and-tokens artefact.

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

`--harness=<name>` before the `--` names the harness the run belongs to in the report, for a harness whose
shell exports no session variable. The hook rewrite and the steering `downshift setup` writes add it; an
unknown name is ignored and the harness is read from the environment.

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

### `downshift retrieve`

Print the full output the compactor kept for an id.

```bash
downshift retrieve <id> [--grep RE] [--head N] [--tail N] [--lines A:B]
```

**Flags:**
- `--grep` — Keep only the lines matching this RE2 pattern
- `--head` — Keep the first N lines
- `--tail` — Keep the last N lines
- `--lines` — Keep lines A to B (1-based, inclusive), each prefixed with its number
- `--session` — Session reference copied from the marker (`retrieve --session=<ref> <id>`); charges the read to that session. Ignored when malformed

The id is the 12-hex marker the compactor prints while CCR is on (the default; `DOWNSHIFT_CCR=0` turns it off). Kept output lives under
the state directory for 24 hours (see [ccr.md](../guides/ccr.md)). Exit 0 on success, 1 when the output is gone
or expired, 2 on bad arguments.

### `downshift discover`

Find shell commands the compactor would have changed but that ran bare.

```bash
downshift discover [--since 7d] [--harness claude-code|codex|all] [--top N] [--json]
```

**Flags:**
- `--since` — Window to scan: `7d`, `36h`, `90m` (default `7d`)
- `--harness` — `claude-code`, `codex` or `all` (default `all`)
- `--top` — How many missed command groups to list (default 10)
- `--json` — Print the report as JSON

It streams the Claude Code transcripts (`~/.claude/projects`) and Codex sessions (`~/.codex/sessions`) in memory,
asks the same policy `compact` uses about each shell command, and reports commands seen, already compacted,
compactible but bare (with the size of their results), skipped by reason, and the top missed groups by family and
program name. The metachar skips are split by kind (`metachar_by_kind` in `--json`): `heredoc`, `subst`, `redirect`,
`or_list`, `background`, `pipe_grep`, `pipe_other`, `quote`, `glob`, `other`, chosen in that order of precedence
(a trailing `2>&1` is not a redirect). The skip log (`exec-skips.jsonl`) carries the same code in an optional `kind`
field. Nothing is written. The report holds families, program names, counts and sizes only, never
arguments or paths. A Codex `exec` call that runs several commands splits its output size between them; a command
whose text is built from a variable cannot be read and is not counted.

### `downshift enable`

Print a POSIX sh script that defines shell functions for common tools (go,
cargo, npm, git, docker, kubectl, pip, gh, ...). Add it to your shell rc:

```bash
eval "$(downshift enable)"
```

**Alias:** `shell-init` (legacy)

Each function asks the binary whether the command is allowlisted
(`downshift compactor --allowlisted`, one extra fork, no run, no log row). An allowlisted
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
downshift learning suggestions
```

It takes no flags; it lists the ten commands with the most to gain.

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

### `downshift quota`

Budget evidence for a harness: show it, import it, or collect it from a local transcript.

```bash
downshift quota status --harness=ID [--model=M]
downshift quota import --harness=ID [--source=S] [--ttl=D]    # JSON on stdin, at most 1 MiB
downshift quota collect --harness=codex --transcript=PATH
downshift quota claude-statusline                             # statusLine bridge, reads stdin
```

**Flags:**
- `--harness` — Harness whose quota is observed (required except for `claude-statusline`)
- `--source` — `codex-usage`, `claude-statusline`, `cursor-usage` or `normalized`
- `--ttl` — Observation lifetime, at most 15m (default 5m)
- `--transcript` — Explicit local Codex transcript (`collect` only)
- `--model` — Exact model to inspect (`status` only)

See the [quota guide](../guides/quota.md).

### `downshift policy`

Validate and manage the routing policy file (`.downshift/policy.json`).

```bash
downshift policy check
downshift policy init [--dry-run] [--yes] [--bare]
downshift policy apply --harness claude-code [--dry-run] [--yes]
```

**Flags:**
- `--dry-run` — Show the change, write nothing (`init`, `apply`)
- `--yes`, `-y` — Do not ask before writing (`init`, `apply`)
- `--bare` — Write only the version, no `explore` profile (`init`)
- `--harness` — Harness to compile profiles for (`apply`; only `claude-code`)

`check` reads the nearest project policy and the user policy and exits 0 when both are usable, or 1 with
the first refusal. `init` creates an empty project policy after a yes. `apply` writes the declared
`routing.profiles` into `<project>/.claude/agents/<name>.md`, changing only the model line of an existing
file. See the [policy guide](../guides/policy.md).

### `downshift playbook`

The working rules a delegated worker receives: named markdown sets, `General` always applying.

```bash
downshift playbook list | show <name> | edit <name> | add <name> | remove <name> | restore <name> | check
```

`edit` and `add` open the file in `$VISUAL` or `$EDITOR`. Files live in `<state dir>/playbooks/<id>.md`, with `# Name`
on the first line; names match regardless of case and accents. `General` cannot be removed; a shipped playbook stays
removed until `restore`. A playbook file that is a symbolic link is refused. `check` warns above 8000 characters.

### `downshift plan`

Reads a delegation plan (JSON on stdin) written by the lead LLM, checks ids,
agents, points (1, 2, 3, 5), dependencies and cycles, fills tier, playbook, wave
and route, and prints a markdown table (`--json` for JSON). See [plan](../guides/plan.md).

### `downshift gateway`

Inspect local the 9router companion gateway and switch delegate workers onto it.

```bash
downshift gateway [status] [--json]
downshift gateway on [name] [--endpoint URL]   # name defaults to 9router
downshift gateway off
```

**Flags:**
- `--json` — Machine-readable status (`status`)
- `--endpoint` — Gateway URL (`on`)

`on` runs the gateway health check and prints the result, but enables the switch
even when the check fails (it warns). The state lives in `gateway.json` in the
state directory. While on, workers started by `downshift mcp serve` `delegate`
get `ANTHROPIC_BASE_URL` (claude) or `OPENAI_BASE_URL` (codex, with `/v1`) in
their environment. Only delegate workers route through the gateway; native
harness subagents cannot. No API key is ever written or passed by Downshift:
configure provider keys in the gateway itself.

### `downshift mcp serve`

An MCP server on standard input and output, started by the harness that registered it (registered by `downshift setup`; the `--mcp` flag is deprecated, registration is on by default and `--no-mcp` skips it).

```bash
downshift mcp serve
```

It answers both protocol eras (the `initialize` handshake of 2025-11-25 and earlier, and the per-request metadata with
`server/discover` of 2026-07-28). Tools, named by their verb under the server name `downshift`: `providers {harness}`
(the models this harness offers now), `playbook {name?}`, `status {}` (quota advice per harness, plus the workers this
session delegated: id, state, agent and harness, read or write, elapsed, tokens once known, and a short summary; the
active ones and the most recent finished ones, with a count of any left out) and
`apply {harness, model?, effort?, tier?, agent_type?, dir?}` (what would be applied, as `downshift route` computes it),
`plan` (checks a delegation plan, as `downshift plan` does) and `retrieve` (reads back output the compactor kept in CCR).
Every successful result is a JSON object with `data` and a `strain` block. The server never picks a model and accepts no
task text; `dir` must be inside the directory the server was started in; string arguments are capped at 256 bytes (`dir`
at 4096) and a result at 64 KiB.

The server registers 13 MCP tools: the six above and seven worker tools. The worker tools (`delegate`, `collect`, `cancel`, `diff`, `send`, `steer`, `release`) follow the worker mode: `off`, `read` or `write`, from env `DOWNSHIFT_WORKERS` (`off`, `read`, `bypass`), then `orchestration.workers` in the user and project `policy.json` (see [config](../guides/config.md)), then the default (`read` for Claude, `off` on Windows). `status` reports `worker_mode` (mode, agents, source, notes). `delegate` refuses with code `workers_off`, `write_off`, `dir`, `cap`, `quota` or `budget`, with a fix line. `budget` means the workers already spent `orchestration.budget_tokens` (or `DOWNSHIFT_BUDGET_TOKENS`); a `delegate` call may also pass `budget_tokens` for its own wave, after which `send` and `steer` refuse for that wave's workers and `collect` lists it under `waves`. `quota` means a provider is rate limited or at or past the stop line (default 90% of a window used; set `orchestration.quota_stop_pct` in `policy.json` or `DOWNSHIFT_QUOTA_STOP_PCT`, 1 to 100); an unreadable quota is never a refusal. A `send` or `steer` turn waits for a free slot like a new worker, is refused once the CLI's token budget is spent, and a session runs at most 4 follow-up turns per allowed worker (128 by default). Tools declare MCP annotations: `delegate`, `send` and `steer` set `destructiveHint` and `openWorldHint` true (a worker calls a model API and may change files); `release` and `cancel` set `destructiveHint` true (`release` also takes `mark`, `used` or `discarded`, your verdict on the results it lets go; `status` lists a report per ended worker, released ones included: wave, state, result bytes, tokens, `transcript_on_disk` and the mark, never task text or a path); the other tools set both false. A worker runs with `DOWNSHIFT_WORKER=1`, and a server started with it has workers off. A line that is not JSON, or a message above 1 MiB, ends the session.

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

#### `downshift models list`

Show the effective catalog: the embedded one, or the user override in the state directory.

#### `downshift models check`

Query the provider APIs and report new or untiered models. It makes network calls and writes nothing.

#### `downshift models pull`

Write the user `catalog.json` under the state directory, adding new provider models with tier unknown.
The embedded catalog is not changed.

#### `downshift models discover`

Ask each harness which models the account can use and cache the answer.

```bash
downshift models discover [--harness=IDS] [--dry-run] [--json]
```

**Flags:**
- `--harness` — Comma-separated harness ids (default: every harness with a source)
- `--dry-run` — Report without writing the cache
- `--json` — Print the result as JSON

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


## Advanced

### `downshift context`

Context optimisation status, providers and compression (experimental). Subcommands: `status`, `providers`,
`doctor`, `enable`, `disable`, `metrics`, `compress` and `benchmark`.

### `downshift context enable`

Enable the native compactor, or record a companion provider.

```bash
downshift context enable [provider] [--exec | --no-exec]
```

**Flags:**
- `--exec` — Add the shell wrapper (native provider only)
- `--no-exec` — Leave the shell wrapper off (native provider only)

`provider` defaults to `native`; `headroom` records the companion.

### `downshift context compress`

Low-level stdin compaction.

```bash
downshift context compress [observe | safe | off] [--exit N]
```

**Flags:**
- `--observe`, `--safe`, `--off` — The mode, also accepted as a positional word (default: `safe`)
- `--exit` — Pretend the command's exit status is N, from 0 to 255 (default: unknown)

### `downshift eval-outcome`

Maintainer tool: outcome eval with executable checks, comparing the pass rate of small and frontier tiers.

```bash
downshift eval-outcome --verify
downshift eval-outcome --record --tier=small|mid|frontier --model=ID --solver='<cmd>'
downshift eval-outcome --report [--write=FILE | --check=FILE]
```

**Flags:**
- `--verify` — Offline: every stub fails, every reference passes, recorded results reproduce
- `--record` — Solve every task with `--solver` (makes model calls; refused in CI)
- `--tier`, `--model`, `--solver` — Label, model id and shell command (prompt on stdin, answer on stdout) for `--record`
- `--report` — Print the generated README block
- `--write` — With `--report`: rewrite the marked block in FILE
- `--check` — With `--report`: exit 1 if the marked block in FILE is stale
- `--tasks`, `--runs` — Task and recorded-runs directories

`--verify` runs the task and solution Go code with your privileges and no sandbox.

### `downshift dci`

Removed: code intelligence is gone. The command stays only so a harness configured by an older setup keeps
working; it prints a one-line notice (silent for `dci hook <harness>`) and exits 0.

### `downshift help`

Show the common commands, or every command with `--all` (`-a`). The default list holds the daily commands
only. Commands marked experimental in `internal/clispec` (`context`, `learning`, `loop`) and the maintainer
tools (`eval-outcome`, `probe`) appear only under `--all`; they stay callable and `docs-manifest` flags them
`"experimental": true`.

```bash
downshift help [--all]
```

### `downshift version`

Print version information.

```bash
downshift version
```

---

## Hook and Observation Commands

A harness calls these with one JSON event on standard input; `downshift setup` writes the calls, and you do
not run them by hand. They fail open: on any error the subagent runs unchanged. `downshift doctor` checks that
they are wired.

### `downshift claude-code`, `downshift cursor`, `downshift codex`, `downshift kiro`, `downshift antigravity`, `downshift opencode`

The route hook of each harness: it rewrites a subagent's `model` (and `reasoning_effort`) as the routing policy
allows. See the [harness matrix](harness-matrix.md).

### `downshift kirocrew`

The KiroCrew route hook, in policy mode: exit 0 allows the spawn and exit 2 blocks it.

### `downshift honor`, `downshift usage`, `downshift end`

Post-spawn observations, called as `downshift <name> <harness>`: `honor` records whether the routing was
honored, `usage` records tokens, and `end` marks a subagent as finished.

### `downshift probe`

Maintainer tool: `downshift probe <event>` records a hook payload's field names (never prompts or paths) in
`probe.jsonl` in the state directory. See the [probe guide](../guides/probe.md).

---

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DOWNSHIFT_EXEC` | Enable/disable compaction | `1` |
| `DOWNSHIFT_BALANCED` | `0` keeps every compacted output lossless. On (the default), an output over 16 KB whose raw output CCR stored is cut to its head, tail and failure lines (a diff to its headers and first hunk lines, a search to its first matches per file); the marker names the `downshift retrieve` id. Output over the 2 MB hold limit is spooled to a temporary file and printed as its head, tail and retrieve id. Overrides `"balanced"` in the config file; the threshold and line caps are the config keys `balanced_threshold_kb`, `balanced_log_head`, `balanced_log_tail`, `balanced_hunk_lines`, `balanced_diff_lines`, `balanced_matches_per_file`, `balanced_match_lines` ([ccr.md](../guides/ccr.md)) | `1` |
| `DOWNSHIFT_POST_COMPACT` | `0` turns off post-run compaction: on a harness whose hook can replace a tool's result after it ran (Claude Code), a shell output the pre-run rewrite did not wrap (a pipe, a redirect, a long command) and a large Grep or Glob result are compacted before the model reads them, with the raw text kept by CCR. Read results are never changed. Off whenever `DOWNSHIFT_EXEC=0` or `"exec": false` | `1` |
| `DOWNSHIFT_DESKTOP_DELTA` | `0` sends every agent-desktop read as the whole compact view, never a delta from the previous read ([agent-desktop](../guides/agent-desktop.md)) | `1` |
| `DOWNSHIFT_LEARNING` | Enable/disable learning | `1` |
| `DOWNSHIFT_STATE_DIR` | State directory | `~/.downshift` |
| `DOWNSHIFT_QUOTA_FILE` | Path of the quota evidence file | `<state dir>/quota.json` |
| `DOWNSHIFT_KIROCREW_DIR` | KiroCrew data directory, read to resolve a session's model | `~/.kiro/crew` |
| `DOWNSHIFT_INSTALL_DIR` | Directory `install.sh` installs the binary into | `~/.local/bin` |
| `DOWNSHIFT_BIN` | Read by the OpenCode bridge: the `downshift` binary it runs for each call (argv, no shell); must be an absolute path to an existing file, else the bridge does nothing | `downshift` on PATH |
| `DOWNSHIFT_WORKER_ENV` | Extra environment names a worker may inherit, comma- or space-separated (for example a Codex provider's `env_key`). Without it a worker gets only what its CLI needs: path, home, locale, proxy, `ANTHROPIC_*`, `CLAUDE_*`, `OPENAI_*`, `CODEX_*`, `DOWNSHIFT_*`, and `AWS_*` or Google variables only when Claude is set to Bedrock or Vertex | unset |
| `DOWNSHIFT_BUDGET_TOKENS` | Tokens all workers of one MCP session may spend together; once reached, `delegate` refuses a new wave (code `budget`). Overrides `orchestration.budget_tokens`. Unset means no cap | unset |
| `DOWNSHIFT_WORKER_TOKEN_BUDGET` | Tokens each worker CLI may spend in one MCP session; once spent, `delegate`, `send` and `steer` refuse for that CLI. `0` or unset means no budget | unset |
| `DOWNSHIFT_COMPACTOR_ACTIVE` | Set by the shell helper while it runs `downshift compactor`, so a nested command is not wrapped twice. Not for you to set | unset |

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Usage/argument error |

For `downshift compactor`, the wrapped command's exit code is returned.
