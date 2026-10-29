# Migration notes (unreleased / W7.6)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Notes for the next release. Status: draft on `downshift/mvp-ponto-zero`. Everything here is measured in
tokens; no figure is converted to money.

## What changes for you

### 0. The dashboard commands are removed

`downshift dashboard` and `downshift serve` are gone. The web dashboard code stays in the repository until its
removal is scheduled (roadmap W6.6). For the same data in the terminal, use `downshift session --tree`,
`downshift watch` and `downshift report`. Savings are reported in tokens only.

If you installed the `dsmon-server` launch agent, it ran `downshift serve`. Stop it with:

```bash
launchctl bootout gui/$(id -u)/com.tiagovilasboas.dsmon-server
```


### 1. The orchestrator chooses the model, Downshift applies it (decision O8)

Downshift never picks a model on its own. In order, the first match wins: a pinned model is kept; a
declared profile; the first matching declared rule; the orchestrator's tier hint (it can only move
down); otherwise the requested model is kept. Doubt keeps the requested model.

If you relied on the old built-in table (`Explore` to small, and so on), it is gone. Without a
`policy.json` and without a hint, nothing changes. To get the same effect, declare profiles in
`.downshift/policy.json` (`downshift policy init` shows a starting file and writes it only after a
yes) or let the orchestrator put `[tier=small]` at the start of the spawn prompt. The
`downshift-orchestrator` skill, installed by `setup`, carries the guidance. Check a file with
`downshift policy check`.

### 2. The MCP server is registered by default

`downshift setup` registers `downshift mcp serve` in Claude Code (through `claude mcp add -s user`)
and in Codex (a marked block in `config.toml`). The dry run shows it first, `--no-mcp` skips it and
`setup --undo` removes it. Other servers in those files are left alone. A conflicting entry that is
not Downshift's is reported as a warning and left as it is.

The server has four information tools: `providers`, `playbook`, `status` and `apply`. It never picks
a model and takes no task text.

### 3. On Claude Code, small now means the newest Haiku

The catalog's small target for Claude Code is `claude-haiku-5-5`, written as the native `haiku` alias
(observed on 2026-10-28: the alias runs the newest Haiku). Before, the small target was
`claude-haiku-4-5`. Your policy and tier floors still bound it. The quality comparison of the newest
Haiku against the old one has not been run yet, so treat the change as unproven on quality.

### 4. Setup writes less

- It no longer writes `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1`, because that makes Claude Code override the
  model the orchestrator chose. A value you set yourself is left alone.
- It no longer writes `session-models.json`. Models are the ones the harness reports; the file is an
  optional manual override. On Claude Code, with no detected list, the catalog stand-in is used and
  `doctor` says so instead of warning.

## Removed

- The built-in agent-type table and the automatic upgrade after a recorded failure.
- The built-in risk veto in the guard. Protection is the orchestrator's judgment plus tier floors you
  declare.
- Dollar accounting. Reports show tokens per harness.
- `downshift try`. Inspect decisions with `downshift session --why` or `downshift stats`.
- `DOWNSHIFT_NB_*` variables are no longer read.

`downshift policy economy` still exists and now acts only on declared policy; its removal is pending.

## State directory

The default is `~/.downshift`. An existing `~/.harness-downshift` is used as a fallback if
`~/.downshift` does not exist; if both exist, `~/.downshift` is used and `doctor` reports the legacy
directory. No automatic data migration happens between the two.

## Rollback

```bash
downshift setup --undo        # restores the hook files and removes the MCP registration
git checkout main && make install
```

Or remove hooks and state entirely:

```bash
downshift uninstall --purge
```
