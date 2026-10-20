# downshift-dock

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

A Claude Code mod (2.1.287+) that draws one line above the prompt, right aligned. Two status chips
and two icon buttons:

- **DCI** chip: green when `<workspace>/.downshift/dci.json` has mode `active`, grey for `observe` or `off`.
- **Compact** chip: green when `~/.downshift/context-optimization.json` enables the native provider.
  In Claude Code the native compactor measures only (a hook cannot rewrite tool output).
- **Mode** button: shows `◇ performance` or `◆ economy` and switches between them. Economy lowers
  the tier of a confident, risk-free subagent task one step; risk and doubt still hold. The press
  writes `mode.json` in the state dir (`DOWNSHIFT_STATE_DIR`, else `~/.downshift`). Hooks read that
  file on every call, so the change applies to the next spawn, not to a spawn already running.
- Pressing a chip shows a toast with what it means and the command that changes it. The mod API has no
  tooltip, so the toast is the detail.
- **▤ Files** runs `/filetree`, which opens the filetree mod's pane.
- **◈ Agents** runs `/agents-info open`, which opens the savvy-progress agents panel.

The dock reads those two config files and writes only `mode.json`.

Original Downshift code (Apache-2.0), not vendored. No routing logic.

## How a press works

The mod API lets a mod run another plugin's slash command: `$.command.run({ command, args })`
queues the command as if the person typed it, through that plugin's own `command.run` hook. The
dock opens no pane itself (a pane id is scoped to the plugin that owns it), so it needs the
command to exist.

- A button shows only when `$.command.list()` reports its command as a plugin command, checked
  at session start, once more 2 s later (other mods register in their own start) and after each
  turn. Until a listing answers, both show. With neither installed the band is not drawn.
- The command is queued until the session is idle. A press during a streaming turn runs when the
  turn ends. If the run is rejected, a toast names the plugin to enable.
- Black-first look: icons and labels follow the terminal theme (`plain` buttons, dim at rest, inverted
  under the pointer); the separator is brand green `#22C55E` only while at least one of the two panels
  is open and grey `#5C6168` otherwise. No background is set.

## Install and test

```text
/plugin install downshift-dock@downshift
```

```bash
claude plugin validate mods/downshift-dock
claude plugin test mods/downshift-dock
```

The tests run against the mock engine: they prove which buttons are drawn and which command a
press runs. They do not prove that the real filetree or agents panel opens in a live session.
