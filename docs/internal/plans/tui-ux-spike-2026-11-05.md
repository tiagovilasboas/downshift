<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# TUI UX spike (2026-11-05)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

The owner asked for UX polish across the terminal UI. The setup wizard is "next, next" with no real choices, and `downshift report` is hard to read. Four read-only agents mapped the code: the wizard, the report, every console surface, and a shared design. Findings come from source reading. The report findings also come from a PTY capture at 80x24. Nothing was built.

## Setup wizard

- **Library.** `setupTUI` (`cmd/downshift/setup_tui.go`) is not a TUI. It reads lines with `bufio` and draws static lipgloss cards with hard-coded colours (86, 63, 212, 241), apart from the console theme. Bubble Tea v2 and lipgloss v2 are in go.mod, but only `internal/console` uses Bubble Tea. Neither `bubbles` nor `huh` is a dependency.
- **Flow.** A harness card is followed by two prompts: "recommended defaults? [Y/n]", then the diff, then "Apply? [y/N]". There is no choice to make.
- **Hidden choices.** No setup flag is offered in the wizard: `--no-mcp`, `--no-mods`, `--no-plugin`, `--no-session-policy`, `--you-should-know`, `--no-policy`, `--keep-binaries`. Harnesses are every detected one. Skills, steering and agent triggers are always planned.
- **Problems:**
  - The diff prints twice, once as the preview and again on apply.
  - One "y" also removes other `downshift` binaries (`setup_prune.go`) and registers MCP. Neither can be declined on its own.
  - The harness list in the wizard omits claude-desktop.
  - Prompts are in English, but the accepted answers include Portuguese ones (`s`, `sim`).
  - `In == nil` applies silently.

## Report

- **Paths.** Markdown is the default (`cmd/downshift/report.go`, `internal/telemetry/report_markdown.go`). `--text` and `--json` exist, and `--markdown` is accepted and ignored. The report has no colour anywhere.
- **In the console.** The console opens only for `report` and `report --days=7`, and never in an agent shell. It runs the child plain, so the markdown source shows raw: `#`, `|---|`, `**`.
- **Problems:**
  - **No summary first.** The first screen is a table of tokens avoided per harness. The routing and efficiency headline comes later.
  - **Too long.** Markdown is 217 lines, text 165. Footnotes repeat. 19 text lines are wider than 80 columns, the widest 127.
  - **Numbers lack context.** Figures such as `82k` have no share. "unobserved" and `0` look alike. Jargon is printed as is: `hint_down`, `guarded_segment`, `child_done`, `agent_default`.
  - **Figures disagree (bug).** Markdown shows 126 routed spawns over all harnesses, and text shows 125 for the current one. The markdown routed row sums harnesses, which breaks the "never summed across harnesses" rule.
  - **Duplicates.** Commands and compacted counts appear twice. For claude-code, "Top reasons" repeats "by verdict". Markdown omits permission modes and the "Excluded" note.

First screen, proposed:

```
Downshift · last 30 days · 1652 commands · 176 compacted
Tokens saved (estimate)  ████████░░  82k   claude-code
Routed to cheaper tier   ███████░░░  69%   of 9.1M billable
Premium preserved        █████████░  94%   101 of 107 tasks
Rework                   0%          0 of 107 ended
[Savings] Routing  Efficiency  Quota     ←→ tab · enter expand · --days=N
```

## Console surfaces (`internal/console`)

There is one theme (`shared/theme`, NO_COLOR handled well) and one frame. The ten polish issues with the most user impact:

1. Palette rows run to about 105 columns on an 80-column terminal (`palette.go:116`).
2. Output lines are not clipped to the width (`output.go:91`).
3. The last output line cannot be reached: `maxTop` assumes one row more than the body has (`output.go:56`).
4. The spinner never animates: there is no tick (`home.go:72`, `output.go:86`).
5. There is no cancel while a command runs.
6. A command error shows as "No subagent spawns recorded" (`home.go:84`).
7. `oneLine` slices bytes and can split UTF-8 (`health.go:84`).
8. The header tab always shows Home (`app.go:148`).
9. The hint line overflows on a narrow terminal (`frame.go:46`).
10. There is no scroll position in the output pane.

No test renders `View()` at 80x24 or under NO_COLOR.

## Design

- **Placement.** Extend `internal/console/shared/*` and add the features as sibling packages. Add no new dependency (YAGNI). Bubble Tea v2 has no widgets, so the few components needed are built in the shared layer.
- **Theme and frame.**
  - `shared/theme` gets an `Info` role and width breakpoints: narrow under 60 columns, normal under 100, wide above.
  - `shared/frame` gets `Header(title, step)` and `Footer([]Key)` on one key-help line, truncated when narrow.
- **New shared components.**
  - `shared/pick` offers `List` and `MultiSelect`. Space toggles, `a`/`n` select all or none, enter confirms, and an item can be disabled with a reason.
  - `shared/confirm` is the confirm dialog.
  - `shared/diffview` is a scrollable diff, one file at a time.
- **Features.**
  - `console/setupwizard` takes `Options{Harnesses, Components, Preview}` and returns a `Selection`. `cmd` maps it to `setupOptions` and calls the same `setupRun`. The wizard ends by printing the equivalent `downshift setup ...` command.
  - `console/reportview` renders a `reportview.Data` DTO. Each feature defines its own ports and imports neither `cmd` nor setup logic.
- **Plain output.** It stays byte for byte. `wantsSetupWizard` and `surface.Decide` are unchanged. A new pure `surface.Styled(Facts)` decides when the report is drawn styled; `--text`, `--json` and `--markdown` keep their paths.
- **Archtest.** Allow the `internal/console/` prefix once, in the first slice, and keep the stdlib-only exceptions for `shared/command` and `surface`.
- **Tests.** Plain `Update` calls, as in `app_test.go`. Golden `View()` files at widths 80 and 60 with no colour, plus one colour golden per feature. Key-sequence tests for `pick`. A table test maps a Selection to setup flags.
- **Must stay unchanged:**
  - `cmd/downshift/testdata/plain/*`: setup dry-run, report, report --days=7, uninstall dry-run.
  - the `report_why`, `report_main_work` and `report_below_session` fixtures;
  - the machine-flag cases in `setup_tui_test.go`.

  Add goldens for `report --text` and `--json` before touching shared report code: today only substring tests cover them.
- **Risks:**
  - **Windows.** Legacy conhost has no VT support; `--no-tui` and the plain flow stay the fallback, and `test-os` must run the new packages.
  - **Machine flags.** `--yes`, `--dry-run`, `--undo` and `--no-tui` never open the wizard; a test pins this.
  - **File size.** Split by step and by section to stay under 300 lines a file.

## Delivery plan

Story points, at most 5 a slice. At most 3 PRs open at once.

| # | Slice | SP | Files | Depends on | Wave |
|---|---|---|---|---|---|
| S0 | Report figures: per harness in markdown, never summed; drop the duplicates | 2 | `internal/telemetry/report_markdown.go`, `cmd/downshift/report.go` | none | W1 |
| S1 | Archtest prefix rule, theme roles and widths, frame header and footer | 3 | `internal/archtest/`, `internal/console/shared/{theme,frame}/` | none | W1 |
| S2 | Goldens for `report --text` and `--json`; report data DTO | 3 | `cmd/downshift/report_data.go`, new golden tests | none | W1 |
| S3 | Console bugs 1-10 with 80x24 and NO_COLOR view tests | 3 | `internal/console/{home,palette,output}/`, `app.go` | S1 | W2 |
| S4 | `pick` (List and MultiSelect) | 3 | `internal/console/shared/pick/` | S1 | W2 |
| S5 | `confirm` and `diffview` | 3 | `internal/console/shared/{confirm,diffview}/` | S1 | W2 |
| S6 | `setupwizard` model, pure, with Preview injected | 5 | `internal/console/setupwizard/` | S4, S5 | W3 |
| S7 | `reportview` renderer and goldens | 3 | `internal/console/reportview/` | S1, S2 | W3 |
| S8 | Wire the wizard into setup; map Selection to flags; per-component planner filter | 3 | `cmd/downshift/setup_tui*.go` | S6 | W4 |
| S9 | `surface.Styled` and the styled report | 3 | `internal/console/surface/styled.go`, `cmd/downshift/report_styled.go` | S7 | W4 |
| S10 | Docs (`cli.md`, install guide) and changelog | 1 | `docs/`, `CHANGELOG.md` | S8, S9 | W5 |

Total: 32 SP. W1 has three parallel slices with no shared files. In each later wave, parallel slices touch disjoint packages.

## Not covered

- No wizard run and no terminal capture of the console surfaces; their findings come from source reading.
- The `huh` and `bubbles` APIs were not checked. The plan does not use them.
- The report's headline figure (tokens saved, routing share) needs the owner to confirm what comes first.
