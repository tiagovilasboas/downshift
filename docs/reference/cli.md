# CLI Reference

Complete reference for all Downshift commands.

## Installation & Setup

### `downshift setup`

Configure Downshift hooks for your harnesses.

```bash
downshift setup [harness...] [flags]
```

**Flags:**
- `--dry-run` — Preview changes without applying
- `--undo` — Restore from backup
- `--yes`, `-y` — Skip confirmation prompts
- `--no-plugin` — Skip plugin installation (Claude Code)
- `--no-session-policy` — Skip session policy configuration

**Examples:**
```bash
downshift setup              # Configure all detected harnesses
downshift setup claude-code  # Configure only Claude Code
downshift setup --dry-run    # Preview changes
downshift setup --undo       # Restore previous configuration
```

### `downshift doctor`

Diagnostic health check.

```bash
downshift doctor [flags]
```

**Flags:**
- `--verbose` — Show detailed output
- `--json` — Output as JSON

**Checks:**
- Binary location and version
- Hook configuration
- Permissions
- State directory
- Catalog validity

---

## Daily Use

### `downshift session`

Show savings for the current session.

```bash
downshift session [flags]
```

**Flags:**
- `--tree` — Interactive tree view of subagents
- `--json` — Output as JSON

### `downshift stats`

Show global statistics.

```bash
downshift stats [flags]
```

**Flags:**
- `--days=N` — Limit to last N days (default: all)
- `--dci` — Include DCI telemetry
- `--routing` — Show routing breakdown
- `--json` — Output as JSON

### `downshift report`

Generate a savings report.

```bash
downshift report [flags]
```

**Flags:**
- `--period=DURATION` — Time period (e.g., `7d`, `30d`, `1w`)
- `--format=FORMAT` — Output format (`text`, `json`, `csv`)

**Examples:**
```bash
downshift report --period=7d
downshift report --period=30d --format=json
```

### `downshift watch`

Live terminal dashboard of routing events.

```bash
downshift watch
```

Press `q` to quit.

---

## Output Compaction

### `downshift compactor`

Run a command with output compaction.

```bash
downshift compactor -- <command> [args...]
```

**Aliases:** `compact`, `exec`

**Examples:**
```bash
downshift compactor -- go test ./...
downshift compactor -- cargo build --release
downshift compactor -- git status
```

**Exit codes:**
- Returns the wrapped command's exit code
- Compaction only happens on exit 0 with empty stderr

---

## Compressor Learning

### `downshift learning suggestions`

Show compressor candidates sorted by impact.

```bash
downshift learning suggestions [flags]
```

**Flags:**
- `--limit=N` — Maximum suggestions to show (default: 10)
- `--json` — Output as JSON

### `downshift learning stats`

Show learning store statistics.

```bash
downshift learning stats
```

### `downshift learning export`

Export raw learning data as JSON.

```bash
downshift learning export
```

### `downshift learning clear`

Reset the learning store.

```bash
downshift learning clear
```

---

## Model Routing

### `downshift try`

Preview routing decision for a task.

```bash
downshift try "<task>" [flags]
```

**Flags:**
- `--verbose` — Show detailed classification
- `--check` — Verify guardrails
- `--json` — Output as JSON

**Examples:**
```bash
downshift try "rename the userId variable"
downshift try --verbose "implement OAuth2 authentication"
```

### `downshift models`

Manage model catalog.

```bash
downshift models <subcommand>
```

**Subcommands:**
- `discover` — Discover models from current session
- `check` — Verify catalog pricing
- `pull` — Refresh remote scores

---

## Code Intelligence (DCI)

### `downshift dci enable`

Enable code intelligence.

```bash
downshift dci enable
```

### `downshift dci disable`

Disable code intelligence.

```bash
downshift dci disable
```

### `downshift dci impact`

Query blast radius for a symbol.

```bash
downshift dci impact <target> [flags]
```

**Flags:**
- `--json` — Output as JSON

### `downshift dci inspect`

Dry-run context retrieval.

```bash
downshift dci inspect "<prompt>" [flags]
```

**Flags:**
- `--json` — Output as JSON

### `downshift dci benchmark`

Measure indexing performance.

```bash
downshift dci benchmark
```

---

## Advanced

### `downshift context compress`

Low-level stdin compaction.

```bash
downshift context compress [flags]
```

**Flags:**
- `--exit=N` — Pretend exit code is N (default: use real)

### `downshift replay`

Replay events from telemetry.

```bash
downshift replay [flags]
```

### `downshift benchmark`

Run performance benchmarks.

```bash
downshift benchmark
```

### `downshift version`

Print version information.

```bash
downshift version
```

---

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DOWNSHIFT_EXEC` | Enable/disable compaction | `1` |
| `DOWNSHIFT_ROUTE` | Enable/disable routing | `1` |
| `DOWNSHIFT_LEARNING` | Enable/disable learning | `1` |
| `DOWNSHIFT_CONTROL_SHARE` | Control group share | `0.10` |
| `DOWNSHIFT_STATE_DIR` | State directory | `~/.downshift` |
| `DOWNSHIFT_LOG_LEVEL` | Log verbosity | `info` |

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Usage/argument error |

For `downshift compactor`, the wrapped command's exit code is returned.
