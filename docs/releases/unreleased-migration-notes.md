# Migration notes (unreleased / W7.6)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Notes for the next release. Status: draft on `downshift/mvp-ponto-zero`.

## Highlights

### 1. Default subagent profile table (W7.1)
Without a project `policy.json`, built-in Claude Code subagent types map to
tiers automatically:
- `Explore`, `claude-code-guide`, `statusline-setup` ➔ `small` (Haiku)
- `general-purpose`, `Plan` ➔ keep requested model
- A risk signal in the prompt elevates the tier to `mid` (Sonnet).
- An explicit profile in `policy.json` wins over the table.

### 2. Economy mode (W7.9)
```bash
downshift policy economy on
```
Shifts every profile down one relative tier within safety boundaries (risk
signals never go to `small`).

### 3. Policy initialization (W7.2)
```bash
downshift policy init
```
Shows a dry run of the starting `.downshift/policy.json` and writes it only
after confirmation.

### 4. Claude Code subagent force
`downshift setup` adds `"CLAUDE_CODE_SUBAGENT_MODEL_FORCE": "1"` to
`~/.claude/settings.json`, so Claude Code honors subagent rewrites on built-in
types.

### 5. Loop command
`downshift loop --task="..." --sensor="..."` runs a sensor retry loop that feeds
the episodic failure tracker on failure and records acceptance on success.

---

## Breaking changes

1. **V1 routing and `try` removed:** Decider v2 is the only routing engine; the
   `downshift try` command was deleted. Inspect decisions with `downshift session`
   or `downshift stats`.
2. **Naive Bayes variables ignored:** `DOWNSHIFT_NB_*` environment variables are
   no longer read.
3. **State directory:** default is `~/.downshift`. An existing `~/.harness-downshift`
   is used as fallback if `~/.downshift` does not exist; if both exist,
   `~/.downshift` is used and `doctor` reports the legacy directory. No
   automatic data migration occurs between the two.

---

## Rollback

```bash
git checkout main
make install
```
Or remove hooks and state:
```bash
downshift uninstall --purge
```
