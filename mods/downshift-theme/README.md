# downshift-theme

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Two Claude Code color themes that follow the Downshift site: black first, white
and grey text, green only as a small accent (success, `!` shell border, added
diff words). `Downshift` (dark, base `dark`) and `Downshift Light` (base `light`,
accent `#15803D` for contrast). In the dark theme the assistant label (`claude`) is white and the permission prompt (`permission`) light grey `#D6DAE2`, so they differ. Needs Claude Code 2.1.118 or later.

This mod is **optional**. `downshift setup` does **not** install it. It has no
hooks, no commands and no routing logic.

## Install

```text
/plugin install downshift-theme@downshift
/theme
```

Pick `Downshift` or `Downshift Light` in the list.

## Switch back

Run `/theme` again and choose any built-in preset (or auto), or uninstall with
`/plugin uninstall downshift-theme@downshift`. Plugin themes are read-only; copy
one into `~/.claude/themes/` to edit it.

## Limits

- The tokens only recolor Claude Code's own UI. They cannot change fonts, and
  the terminal's own background and palette stay yours.
- Mod panels (savvy-progress, downshift-dock) follow the active
  theme; they carry no colors of their own here.
- Diff tokens are backgrounds: added and removed lines use dark green and red
  tints so the grey text stays readable (checked at 4.5:1 or better).
- Not verified visually: nobody has seen it render until a person runs `/theme`.

## Test

```bash
python3 mods/downshift-theme/tests/check-themes.py
claude plugin validate mods/downshift-theme
```
