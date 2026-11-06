# Downshift Agents

A Claude Code mod that shows, beside the chat, what your session's subagents
are doing: task progress, a live subagent list, Downshift Compactor figures and
Downshift routing decisions. It only observes. It never calls a model and never
changes a model, prompt, tool result or routing decision.

> Downshift by Tiago Vilas Boas
> https://github.com/tiagovilasboas/downshift

## What you get

- **A band above the prompt**: one line with the task progress, active and
  total agents, blocked tasks, and the Compactor token estimate. Segments drop
  as the terminal narrows. The band yields to a survey and keeps any other
  mod's band beneath its own row.
- **A dashboard pane** that opens by itself whenever a subagent is spawned.
  It takes no focus, and in a terminal under 144 columns it waits until the
  terminal is wider or you run `/ds-agents`. It has these sections:
  - **Tasks**: the progress bar, the current task and blocked tasks.
  - **Subagents**: a tree with status, the model and its evidence, and the
    elapsed time. Expanded, it adds type, requested and decided model, tools
    used and measured tokens.
  - **Compactor**: runs, raw bytes, bytes cut, and the estimated tokens avoided.
  - **Routing**: Downshift's decisions for this session.
  - **Efficiency**: per harness, never summed: the premium capacity preserved
    (`75% (3 of 4 completed tasks ran below frontier)`, or `unobserved`, never
    0%), spawns by tier, completed of ended, and the rework rate. Counts and
    shares only: no tokens, no money.
  - **Buttons**: active/all filter, expand, refresh, hide the band.
- **`/ds-agents summary`**: the same figures as plain text, for surfaces that
  draw no pane.
- **`/ds-agents report`**: the full `downshift report`, with the Compactor
  broken down per harness and family, printed in the session.

| Command | Does |
| --- | --- |
| `/ds-agents` or `/ds-agents open` | open the dashboard pane |
| `/ds-agents close` | close it |
| `/ds-agents summary` | print tasks, agents, Compactor and routing as text |
| `/ds-agents report` | print `downshift report`: the Compactor per harness and family, and routing |
| `/ds-agents help` | list the options |

## Install

Requires Claude Code 2.1.309 or later (the mod API this was built against).
The `downshift` binary on `PATH` is optional: without it, the Compactor and
routing sections say "unavailable" and everything else still works.

`downshift setup` installs it: it writes this folder to
`~/.claude/skills/downshift-agents/`, which Claude Code loads as
`downshift-agents@skills-dir` in every new session. The files go through the
setup diff, get a backup, and `downshift setup --undo` removes them;
`--no-mods` skips the mod. Setup leaves the folder alone when the marketplace
copy below is already enabled, and it removes this folder from a
`CLAUDE_CODE_PLUGIN_DIRS` export in your shell profile (shown in the diff), so
the mod never loads twice.

For this session only, from a clone of this repository:

```bash
claude --plugin-dir ./mods/downshift-agents
```

From the repository's marketplace instead of setup:

```bash
claude plugin marketplace add tiagovilasboas/downshift
```

```bash
claude plugin install downshift-agents@downshift
```

## Where each number comes from

| Figure | Source | Kind |
| --- | --- | --- |
| Tasks done / total | `TaskCreate` / `TaskUpdate` results, else `TodoWrite` | measured |
| Agent status | `agent.spawn` and the agent's `turn.complete` reason | measured |
| Effective model | `turn.complete` usage, or Downshift's observed model | measured |
| Requested / decided model | the Agent call's `model`; Downshift's decision | stated, labelled as such |
| Agent tokens | `turn.complete` usage (input + output, cache read apart) | measured |
| Compactor raw and cut bytes | `downshift session --panel`, `compactor` block | measured by Downshift |
| Est. tokens avoided `*` | Downshift: cut bytes / 4 | **estimate**, always marked `*` |
| Routing counts, lower-tier workload | `downshift session --panel` for this session id | measured by Downshift |
| Efficiency per harness | `downshift session --panel`, `efficiency` block (the `downshift report --json` block, last 30 days) | measured by Downshift |

Rules the mod keeps:

- A model is called effective only with evidence. Otherwise the pane says
  `(requested)`, `(decided)` or `(inherited)`.
- An agent is completed only when its own turn ends with an answer. An aborted
  turn shows as cancelled, an error or refusal as failed.
- Tokens below the session model are workload, not savings. The pane says so
  ("Same work on a lighter model: it spares the top model's quota."), and its only
  "tokens saved" figure is the Compactor's, an estimate (bytes cut ÷ 4).
- A child nests under a parent only when the spawn named that parent, and the
  parent is listed.
- With no tasks reported, the pane shows no total and no percentage.

## Stale agents and tasks

An agent is closed only by its turn ending, and a task only by a successful
`TaskUpdate`. When that event never comes (an interrupted session, a spawn the
engine never named), the count would stay on "1 running" for good. So a
30-second tick marks a running agent, or an in-progress task, `stale` once it
has been silent for 120 s. The pane and `/ds-agents summary` show it apart
("0 active · 1 stale · 1 total"), and the active filter keeps stale rows
visible. Nothing is deleted or completed by the sweep: a later turn closes a
stale agent, a tool call from it makes it run again, and a task a tool states
as in progress runs again. A turn under an id no spawn named closes the single
open provisional agent when exactly one exists; with several it is left to the
sweep. The tick stops itself once nothing is running. A refresh dropped by the
3 s gap is repeated once after it, so the last update is not lost.

The statusline "N subagents" comes from the Go side and is a cumulative spawn
count with no running state. It is unchanged here; a Go-side liveness package
was deliberately deferred.

## Data handling

- State lives in the session's `$.state`: agents (at most 100), tasks (at most
  200) and the last Downshift snapshot. Prompts, tool output and file paths
  are never stored. Labels are the Agent tool's short description, cut to 80
  characters.
- The only process the mod runs is
  `downshift session --panel --session=<this session id>`. It runs at most
  once every 3 s and always with a 4 s timeout. The mod makes no network
  request.

## Limitations

- The Compactor block covers Downshift's report window (the last 30 days)
  across all sessions. Downshift's exec log has no session id, so the
  per-session cut is not available.
- There is no "last compaction" event and no emitted-bytes field. Raw minus
  cut bytes is only an approximation, so the mod does not show it.
- The task tools have no "failed" state. A task fails only if the tool says
  so. A stopped agent does not complete or fail any task.
- No reliable per-agent "waiting for permission" state is exposed, so none is
  shown.
- Elapsed time moves when an event arrives. The only timer is the 30 s stale sweep, which runs while something is running.
- Workflow agents are left out. Their ids match no agent list.
- `claude -p` draws no pane or band. `/ds-agents summary` still works there.

## Develop

```bash
claude plugin validate mods/downshift-agents
```

```bash
claude plugin test mods/downshift-agents
```

Layout, by what each folder does: `hooks/features/agents` (roster),
`tasks` (board), `downshift` (panel reader), `metrics` (labels and evidence),
`band`, `dashboard` and `command`. `hooks/shared` holds the state accessors
and the formatting.

Licence: Apache-2.0, see the repository's [LICENSE](../../LICENSE).
