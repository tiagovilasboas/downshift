# filetree (vendored mod)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

This directory is a vendored copy of **filetree**, a Claude Code mod by Kurt
Buhler, MIT License ([LICENSE](LICENSE)). It ships in the Downshift
marketplace as its own plugin (`/plugin install filetree@downshift`): Claude
Code loads one hooks module per plugin and refuses a second `session.start`
hook in it, and the mod's state and pane are keyed by its plugin name, so it
cannot be folded into the `downshift` plugin unchanged.

| | |
|---|---|
| Upstream | https://github.com/data-goblin/claude-code-filetree |
| Version | 0.2.26 |
| Commit | `1b3e72ce1bd265b0a9aeedb993bdf848139e29cc` (2026-10-16) |
| Files | `.claude-plugin/plugin.json`, `hooks/` (`hooks.json`, `register.tsx`, `button.tsx`, `git.ts`, `icons.ts`, `open.ts`, `rows.tsx`, `tree.ts`), `tests/` and `types/index.d.ts` byte-identical to upstream except the Downshift local patch below; upstream's `scripts/check-open.ts` (a Node dev script), `media/`, `.githooks/` and CI are not vendored |
| Loaded by | its own `hooks/hooks.json`, `"modules": ["./register.tsx"]`; listed in [`../../.claude-plugin/marketplace.json`](../../.claude-plugin/marketplace.json) as `filetree`, source `./mods/filetree` |
| Needs | Claude Code 2.1.287 or later; in the terminal the fullscreen layout (`/tui fullscreen`) and at least 110 columns; the desktop app docks the pane itself |

## Downshift local patch

Owner decision 2026-10-19: colours follow the Downshift site (black first,
white and grey, green only as an accent). Colours only, no behaviour. Reapply
with `git apply mods/filetree/downshift-local.patch` after a re-vendor.

| File | Change |
|---|---|
| `hooks/tree.ts` | `DEFAULT_THEME` (dark) `accent` `#5b9bd5` to `#f5f7fa`, `muted` `#808a96` to `#868a90` |
| `hooks/git.ts` | `TONES` (dark shimmer) `orange` (writes) to white ramp, `purple` (reads) and `teal`/`blue`/`cyan` to grey ramps; `green` (commits) and `red` kept |
| `hooks/icons.ts` | git status colours match the `downshift-theme` mod: added and untracked `#22c55e`, modified `#d6dae2`, renamed and copied `#868a90` (light: `#15803d`, `#374151`, `#4b5563`) |
| `hooks/register.tsx` | `ADD_COLOR` `#98c379` to `#22c55e`, `DEL_COLOR` `#e06c75` to `#f87171` (light: `#15803d`, `#b91c1c`); the modified-count fallback `#e5c07b` to `#d6dae2` |
| `tests/surfaces.test.tsx` | expected colours follow the values above |

The patch was regenerated 2026-10-19 against the pinned upstream commit. The
light theme's tree colours and the diff colours otherwise are untouched; the
light add colour `#15803d` is 5.0:1 on white (AA), not the AAA the upstream
light palette targeted.

## What it does

An IDE-style file tree of the session cwd in the right sidebar (`/filetree`,
`/filetree <path>` pins another folder): git status and `+N -N` per file,
`?:N M:N D:N` per folder, branch and ahead/behind in the header, search,
sizes or dates, and a shimmer on what Claude reads (purple), writes (orange)
and commits or pushes (green), with the git or `gh` action shown at the
bottom. Double-click or Enter opens a file in the default app (or the editor
or an in-pane preview, by option); the selected file is passed to Claude as
one line of prompt context, and `@path` mentions reveal the file.

**It spends nothing.** No model request and no network. It runs local
commands: `git` (status, diff, ls-files, rev-parse, diff-tree, scoped to the
cwd), `find` and `du` (or a listing walk on Windows), `uname`, a `sh` probe
for a Nerd Font (`fc-list`, `ps`), `touch`/`rm` of a marker file in `$TMPDIR`
on macOS, and only on your double-click or key press the opener (`xdg-open`
or `gio`, `open`, PowerShell), `tmux` with your editor, `qlmanage`, or the
command you set in `Open with`.

## What it reads, runs, sends and persists

From `claude plugin validate mods/filetree` (2.1.302):

- hooks: `session.start`, `config.set{theme}`, `command.run{filetree}`,
  `tool.call` (Edit, Write, NotebookEdit, Bash, PowerShell), `tool.call{Read}`,
  `tool.call{Grep|Glob}`, `ui.message`, `classic.SessionStart`,
  `classic.CwdChanged`, `classic.FileChanged`, `ui.focus`, `ui.scroll`,
  `prompt.submit`, `prompt.attachment`, `ui.render{Pane filetree}`. The three
  `classic.*` hooks and `prompt.attachment` do not run in a plugin a person
  installs where an organization manages Claude Code.
- calls: `$.clock.*`, `$.command.register`, `$.config.list`, `$.env.get`,
  `$.fs.list`, `$.fs.read`, `$.fs.stat`, `$.process.run`, `$.session.cwd`,
  `$.session.id`, `$.session.surfaces`, `$.state.get`, `$.state.set`,
  `$.ui.close`, `$.ui.copy`, `$.ui.focus`, `$.ui.invalidate`, `$.ui.open`,
  `$.ui.resolve`, `$.ui.toast`. No `$.http`, `$.model` or `$.store`.
- env reads: `EDITOR`, `VISUAL`, `HOME`, `USERPROFILE`, `OS`, `TMUX`,
  `TMPDIR`, `SSH_CONNECTION`, `SSH_TTY`, `HERDR_ENV`, `HERDR_BIN_PATH`; env
  writes: none. No secret is read.
- Files read: directory listings and stats under the tree root, `.git` and the
  `gitdir:` file, `~/.local/state/omarchy/current/theme/colors.toml` when
  present, and the file you preview (up to 4 MB, 90,000 characters shown).
- Prompt: `prompt.submit` adds one context line naming the selected file; it
  reads `@path` mentions and `<task-notification>` blocks and never rewrites
  the text. Tool results of Bash are read for paths (`rg`, `grep`, `find`,
  `cat`, ... output) to shimmer the files.
- Persists: nothing across sessions. Session state (`$.state`, keys
  `filetree.tree/theme/activity`) holds the tree, git summary, theme colours
  and the last six git activities. Writes to disk: only the macOS marker file
  it removes afterwards.

## Options

`activity`, `glyphs`, `follow`, `start`, `column`, `hints`, `open`, `edit`:
in `/config` under filetree; meanings in the upstream README.

## Both forms installed

If `filetree@claude-code-filetree` (the upstream marketplace install, or the
pre-rename `filetree@filetree`) is also enabled, two panes open and both
register `/filetree`. Uninstall one (`/plugin uninstall filetree@claude-code-filetree`).

## Update procedure

Re-vendor, never edit in place:

```bash
git clone --depth 1 https://github.com/data-goblin/claude-code-filetree.git /tmp/claude-code-filetree
rm -rf mods/filetree && mkdir -p mods/filetree
cp -r /tmp/claude-code-filetree/.claude-plugin /tmp/claude-code-filetree/hooks \
      /tmp/claude-code-filetree/tests /tmp/claude-code-filetree/types mods/filetree/
rm mods/filetree/.claude-plugin/marketplace.json
cp /tmp/claude-code-filetree/LICENSE mods/filetree/LICENSE
claude plugin validate mods/filetree && claude plugin test mods/filetree
```

Then restore this README, update the version and commit here and in
[NOTICE](../../NOTICE), and read the upstream diff for new calls and new
`$.process.run` commands (`claude plugin validate` lists the calls) before
committing.

## Verify

```bash
claude plugin validate mods/filetree   # hooks: ... calls: ... $.process.run ... env reads: EDITOR, ... VISUAL
claude plugin test mods/filetree       # 68 tests across 2 files
```
