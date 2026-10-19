# downshift-dock

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

A Claude Code mod (2.1.287+) that draws one line above the prompt with buttons that bring
back panels that went away:

- **Files** runs `/filetree`, which opens the filetree mod's pane.
- **Agents** runs `/agents-info open`, which opens the savvy-progress agents panel.

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
- Button colours follow the terminal theme (`variant="primary"`); the label and the "downshift"
  text use the brand green `#22C55E` and muted `#9CA3AF`. No background is set.

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
