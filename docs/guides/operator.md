<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Operator guide: one week, one export

You are asked for one thing: use Claude Code as you normally do for seven days with Downshift
installed, then run one command and share what it prints. That export is the evidence
[issue #104](https://github.com/tiagovilasboas/downshift/issues/104) needs from people who are not
the maintainer. It holds counts and token totals only. Nothing private leaves your machine.

**2026-11-02:** there is no control group any more, so the export holds the routed spawns only.

| | |
|---|---|
| Time | About 10 minutes on day 1, about 5 minutes on day 8. Nothing in between |
| Harness | Claude Code. It is the only harness where completion and tokens per task are measured today |
| What you share | One JSON file printed by `downshift export --days=7` |
| What changes for you | Nothing. Every subagent spawn is routed |

## Day 1: install and start

1. **Install a build that has `downshift export`.** The command ships from v0.5.1 (v0.5.0 was tagged but published no binaries). Check with
   `downshift export --help`: it prints the export's help. `unknown command "export"` means the
   build is too old; install from `main`:
   ```bash
   go install github.com/tiagovilasboas/downshift/cmd/downshift@main
   ```
   A later release from the [Releases](https://github.com/tiagovilasboas/downshift/releases) page
   works too. See [install.md](install.md) for the installer and PATH notes.
2. **Wire the hooks.** `downshift setup` finds Claude Code and merges its hooks, with a diff and a
   backup. Then `downshift doctor` must show the Claude Code hooks as wired.
3. **Start from zero.** `downshift report clear --yes` moves the old logs to a dated backup, so
   the week only counts this week. Write down the time you ran it.
4. **Write down four facts** for the day-8 post: the Claude Code plan you are on, the output of
   `downshift --version`, the start date, and the harness (Claude Code).

## Days 1 to 7: work normally

- Use subagents the way you usually do. Do not pick tasks for the test.
- Keep the same Downshift binary all week. A new binary mid-week starts a new week.
- Do not pause routing (`/downshift:status --pause`). Paused spawns are left out of the figures and
  shrink the sample.
- `downshift report` shows where the week stands at any time.

## Day 8: export and share

```bash
downshift export --days=7 > downshift-export.json
```

Open the file and read it before you share it. It has these fields and nothing else:

| Field | What it holds |
|---|---|
| `schema`, `window_days`, `generated_at` | Format version (`3`), the window, when it was made |
| `routed` | `spawns`, `ended`, `completed`, `reworked` (counts) |
| `tokens` | `routed_billable`, `compactor_avoided` (token totals) |
| `harnesses` | Per harness name: `session_billable`, `below_session_billable`, `fallback_billable`, `fallback_below_billable`, `unobserved_spawns`, `cache_read`, `compactor_avoided` (never summed across harnesses) |
| `note` | A fixed sentence saying what the export is |

It never contains prompts, task text, file paths, session ids, model names, hostnames or
usernames. The command checks its own output against a strict allowlist of field names and
values, and refuses to print anything that does not pass. That check is tested in
`internal/operatorexport`.

Post the file as a comment on [issue #104](https://github.com/tiagovilasboas/downshift/issues/104),
with the four facts from day 1 and the end date. A public handle is enough as your contributor
reference; say so if you prefer to stay anonymous in the summary.

Use `downshift export`, not `downshift stats --export`. The stats export is an older, larger
format meant for the maintainer's own logs.

## Afterwards: undo

- `downshift setup --undo` restores your hook files from the backup `setup` made.
- `downshift report clear --yes` again if you want the week's logs out of the way.

## What your export can and cannot show

One week from one operator is a direction, not a result. It shows how many routed spawns ran,
ended and completed on your work, their billable tokens, and how many were redone. It has no
unrouted side, so it cannot show that tokens were saved, and it cannot show a money saving: the
export has no prices. The maintainer combines two or more independent exports, keeps each
one's sample size and limits next to its number, and publishes the summary linked from
[beta-exit.md](../reference/beta-exit.md).

For a team running two weeks with three or more people, use the [pilot](pilot.md) instead.
