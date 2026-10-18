---
name: downshift-setup
description: Install or repair Downshift hooks on the user's machine for any harness (Claude Code, Cursor, Codex, Antigravity, KiroCrew). Use when asked to install, set up, wire, fix or undo Downshift, or when subagents never change model. Dry-run first, write only after a yes.
---

# Downshift setup

Assisted, never silent. Every write outside the repository is shown as a diff and
waits for the user's yes. Never read or print task prompts. This skill is for the
first day: once `downshift doctor` is clean, ignore it (a hand-copied install can
delete it; the plugin copy stays and costs one description line).

## 1. Binary

1. `downshift version`. If missing: `curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh`
   (macOS / Linux) or the Releases page or `go install`; see `docs/guides/install.md`.
2. Claude Code only: the plugin (`/plugin marketplace add tiagovilasboas/downshift`, then
   `/plugin install downshift@downshift`) wires the same hooks and the `/downshift:status`
   slash command without touching `~/.claude/settings.json`. Use one
   path, plugin or `setup`, not both: `setup` skips the hooks when it sees the plugin enabled,
   and `downshift doctor` warns when both are wired.

## 2. Hooks

1. `downshift setup --dry-run`: one diff per harness found. Show it. Explain the Claude Code
   extras it adds only for keys the user has not set: `"model": "opusplan"` (Opus plans,
   Sonnet executes), sessions starting in plan mode, Claude Code's built-in You should know mod
   (each check is a billed background request: say so), a statusline. `--no-session-policy`
   skips the first three.
2. Only after a clear yes: `downshift setup --yes`. Backups are kept; `downshift setup --undo`
   restores the latest.
3. KiroCrew needs its wrapper script and is only reported by `setup`; follow the KiroCrew
   section of `docs/guides/install.md`. Grok is config, not a hook.

## 3. Check

1. `downshift doctor --json`: handle every `warn` or `fail` with its `fix`, asking before any
   config write. A `companion` row about Headroom is optional; see `docs/guides/companions.md`.
2. Session models: Claude Code infers them from the current model. Other harnesses need
   `downshift models discover` or `~/.downshift/session-models.json`
   (`docs/reference/session-models.md`).
3. Plan limits decide whether a rewrite is honored, not Downshift
   (`docs/reference/harness-matrix.md`). Say so when the plan is Free or legacy Cursor Pro.
4. Ask the user to open a **new** harness session, spawn a trivial subagent, and check
   `downshift session`, or in Claude Code `/downshift:status` (plugin) or `/downshift` (manual alias).
   The stderr line `downshift: TRIVIAL task → downshift to …`
   proves the hook fired; the matrix says whether it was applied.

Do not invent hook paths, matchers or catalog entries; everything above is in
`docs/guides/install.md` and `examples/`.

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift
