# Shadow Mode

Shadow mode lets you test routing policy changes without affecting live traffic.
Downshift records what *would* have happened alongside what *did* happen.

## How It Works

When shadow mode is enabled for a model or policy:

```
Spawn arrives → Normal routing (applied)
                     ↓
              Shadow routing (recorded only)
                     ↓
              Telemetry captures both
```

You can compare outcomes to validate changes before rollout.

## Use Cases

### Testing New Model Mappings

Before adding a new model to the catalog:

```bash
# Add model as shadow-only
downshift config set catalog.shadow.gpt-5-mini true

# Run your normal workflow
# Shadow decisions are recorded but not applied

# Analyze what would have happened
downshift shadow-report --model=gpt-5-mini --period=7d
```

### Validating Policy Changes

Before changing tier thresholds:

```bash
# Enable shadow policy
downshift config set policy.shadow.enabled true
downshift config set policy.shadow.small_ceiling 0.7  # New threshold

# Continue working
# Both policies run, only current applies

# Compare outcomes
downshift shadow-report --period=3d
```

### A/B Testing Routing Changes

Compare two routing strategies:

```bash
downshift shadow-report --period=7d --compare

Strategy    Spawns  Routed  SMALL  MID  FRONTIER  Est.Savings
current     1,234   89%     45%    35%  20%       38%
shadow      1,234   92%     52%    32%  16%       42%
```

## Shadow Report

```bash
downshift shadow-report [flags]
```

### Output

```
Shadow Analysis (last 7 days)
=============================

Total spawns: 1,234
Shadow divergence: 156 (12.6%)

Divergent decisions:
  Current→Shadow    Count   Impact
  MID→SMALL         89      +$12.40 saved
  FRONTIER→MID      45      +$34.20 saved
  SMALL→MID         22      -$8.80 cost (conservative)

Net estimated impact: +$37.80/week savings

Would-route breakdown:
  Model               Current  Shadow   Delta
  claude-haiku-4      456      545      +89
  claude-sonnet-4     389      367      -22
  claude-opus-4       234      178      -56
```

### Flags

- `--period=DURATION` — Analysis window (default: 7d)
- `--model=ID` — Filter to specific shadow model
- `--compare` — Side-by-side strategy comparison
- `--format=json` — JSON output for tooling

## Configuration

### Enable Shadow Mode

```bash
# Shadow a specific model
downshift config set catalog.shadow.<model-id> true

# Shadow a policy change
downshift config set policy.shadow.enabled true
```

### Shadow Catalog

Create `~/.downshift/shadow-catalog.json`:

```json
{
  "models": [
    {
      "id": "gpt-5-mini",
      "tier": "SMALL",
      "harness": "cursor",
      "shadow": true
    }
  ]
}
```

Models marked `shadow: true` are never routed to, only recorded.

### Shadow Policy

In `~/.downshift/config.json`:

```json
{
  "policy": {
    "shadow": {
      "enabled": true,
      "confidence_threshold": 0.65,
      "risk_floor_keywords": ["deploy", "migrate", "delete"]
    }
  }
}
```

## Gateway Shadow

For models behind a gateway (like Claude Code's gateway models):

```bash
downshift shadow-report --gateway
```

Shows what gateway pricing *would* have been for routed spawns.

## Replay with Shadow

Re-run past decisions through a shadow policy:

```bash
downshift replay --shadow --period=30d
```

Useful for validating policy changes against historical data.

## Telemetry Fields

Shadow decisions add fields to telemetry:

```json
{
  "outcome": "rewrite_emitted",
  "to_model": "claude-haiku-4",
  "shadow": {
    "would_model": "gpt-5-mini",
    "would_tier": "SMALL",
    "policy_version": "shadow-v2"
  }
}
```

## Best Practices

1. **Run shadow for at least a week** — get enough data for confidence
2. **Monitor divergence rate** — >20% divergence suggests significant change
3. **Check conservative moves** — shadow routing SMALL→MID is safe to accept
4. **Verify with replay** — `downshift replay --shadow` on historical data
5. **Graduate gradually** — move from shadow to 10% rollout to full

## Privacy

Shadow mode records:
- Model IDs (current and shadow)
- Tier decisions
- Timestamps

It does **not** record prompt content, even for shadow analysis.
