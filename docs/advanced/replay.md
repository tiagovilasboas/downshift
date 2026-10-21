# Replay System

Replay re-runs past routing decisions through a new policy without affecting
the original outcomes. Use it to validate policy changes against historical data.

## How It Works

```
Historical events → New policy → Compare outcomes
        ↓                ↓              ↓
   What happened    What would    Difference
                    have happened
```

## Commands

### Basic Replay

```bash
downshift replay --period=7d
```

Re-runs last 7 days of decisions through the current policy.

### Compare Policies

```bash
downshift replay --period=30d --policy=new-policy.json
```

### Output

```
Replay Analysis (last 7 days)
=============================

Events replayed: 1,234
Policy version: v2.1 → v2.2

Changes:
  Same decision:     1,089 (88.3%)
  Different tier:      145 (11.7%)

Tier changes:
  SMALL→MID:          23 (more conservative)
  MID→SMALL:          89 (more aggressive)
  MID→FRONTIER:       12 (more conservative)
  FRONTIER→MID:       21 (more aggressive)

Estimated impact:
  Additional savings: +$45.20/week
  Risk increase:      +2.1% (MID→SMALL on medium tasks)
```

## Use Cases

### Validate Before Deploy

Before rolling out a policy change:

```bash
# Test new policy against 30 days of history
downshift replay --period=30d --policy=proposed.json

# If happy, deploy
cp proposed.json ~/.downshift/policy.json
```

### Measure Classifier Changes

After updating the classifier:

```bash
downshift replay --period=7d --verbose
```

See exactly which tasks would route differently.

### Incident Analysis

After a task failed due to wrong tier:

```bash
downshift replay --event=<event-id> --verbose
```

Understand why that decision was made.

## Sources

### Telemetry Events

Default source — replays from `~/.downshift/telemetry/`:

```bash
downshift replay --period=7d
```

### Claude Code Transcripts

Replay from Claude Code conversation logs:

```bash
downshift replay --source=claude-code --period=7d
```

### Custom JSONL

Replay from any JSONL file with the right schema:

```bash
downshift replay --source=./my-events.jsonl
```

## Flags

| Flag | Description |
|------|-------------|
| `--period=DURATION` | Time window (e.g., 7d, 30d, 1h) |
| `--policy=FILE` | Policy file to test |
| `--source=SOURCE` | Event source (telemetry, claude-code, file) |
| `--verbose` | Show individual decision changes |
| `--format=json` | JSON output |
| `--event=ID` | Replay single event |

## Verbose Output

```bash
downshift replay --period=1d --verbose
```

```
Event: abc123 (2026-10-20 14:30:00)
  Prompt: "implement OAuth2 with PKCE flow"
  Original: MEDIUM → MID (claude-sonnet-4)
  Replay:   COMPLEX → FRONTIER (claude-opus-4)
  Reason: New risk floor keyword "OAuth2"

Event: def456 (2026-10-20 15:45:00)
  Prompt: "rename userId to userID"
  Original: TRIVIAL → SMALL (claude-haiku-4)
  Replay:   TRIVIAL → SMALL (claude-haiku-4)
  Reason: No change
```

## Privacy

Replay requires the original prompt to re-classify. Options:

### Hash-Only Mode

If prompts weren't stored:

```bash
downshift replay --hash-only --period=7d
```

Shows tier distribution changes without per-event detail.

### Prompt Logging

Enable prompt logging for full replay:

```bash
downshift config set telemetry.log_prompts true
```

⚠️ This stores task text locally. Disable in sensitive environments.
