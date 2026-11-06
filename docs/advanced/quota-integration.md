# Quota & Budget Integration

Downshift reads provider budget state to make smarter routing decisions.
When quota is low, routing becomes more conservative.

## How It Works

Downshift checks quota status before routing:

```
Spawn arrives → Check quota → Adjust routing
                    ↓
          Available: normal routing
          Low: prefer cheaper tiers
          Exhausted: allow only (no rewrite)
```

## Quota Sources

### Harness Native

Some harnesses expose quota via their API:

```
Claude Code → reports remaining context window
Cursor      → reports monthly usage limits
```

Downshift reads these automatically when available.

### Provider Cache

For direct provider integration:

```bash
# Cache current quota
downshift quota refresh

# View cached state
downshift quota show
```

Output:
```
Provider: anthropic
Plan: team
Period: 2026-10 (resets in 12 days)

Usage:
  Input tokens:  4.2M / 10M (42%)
  Output tokens: 1.8M / 5M  (36%)

Status: AVAILABLE
```

### Manual Override

Set quota state directly:

```bash
downshift quota set --status=low
downshift quota set --status=exhausted
```

## Routing Behavior

### Available (default)

Normal routing based on classification:

```
TRIVIAL → SMALL
SIMPLE  → SMALL
MEDIUM  → MID
COMPLEX → FRONTIER
```

### Low

More conservative — prefer current model:

```
TRIVIAL → SMALL (unchanged)
SIMPLE  → Current model (hold)
MEDIUM  → Current model (hold)
COMPLEX → FRONTIER (unchanged)
```

Reasoning: when quota is low, avoid unnecessary model changes
that might consume more budget.

### Exhausted

No routing — allow all spawns unchanged:

```
All tasks → Current model (no rewrite)
```

When budget is exhausted, Downshift doesn't spend tokens
deciding about tokens.

## Configuration

### Quota File

`~/.downshift/quota-cache.json`:

```json
{
  "anthropic": {
    "status": "available",
    "usage_percent": 42,
    "resets_at": "2026-11-01T00:00:00Z",
    "updated_at": "2026-10-21T05:00:00Z"
  }
}
```

### Thresholds

```bash
# Consider "low" when >80% used
downshift config set quota.low_threshold 80

# Consider "exhausted" when >95% used
downshift config set quota.exhausted_threshold 95
```

### Refresh Interval

```bash
# Auto-refresh every 5 minutes
downshift config set quota.refresh_interval 5m
```

## Commands

### View Quota

```bash
downshift quota show
```

### Refresh Cache

```bash
downshift quota refresh
```

Fetches latest state from provider APIs (requires credentials).

### Manual Set

```bash
downshift quota set --provider=anthropic --status=low
```

For testing or when API refresh isn't available.

## Telemetry

Quota state is recorded with each decision:

```json
{
  "quota_status": "available",
  "quota_percent": 42
}
```

### Stats by Quota State

```bash
downshift stats --by=quota
```

```
Quota Status  Spawns  Routed  Held
available     1,100   89%     11%
low           120     45%     55%
exhausted     14      0%      100%
```

## Provider Integration

### Anthropic

```bash
export ANTHROPIC_API_KEY=sk-ant-...
downshift quota refresh --provider=anthropic
```

### OpenAI

```bash
export OPENAI_API_KEY=sk-...
downshift quota refresh --provider=openai
```

### Custom Provider

Implement a quota hook in `~/.downshift/hooks/quota.sh`:

```bash
#!/bin/bash
# Output JSON: {"status": "available|low|exhausted", "usage_percent": N}
curl -s https://your-api/quota | jq '{status: .tier, usage_percent: .used}'
```

## Best Practices

1. **Enable auto-refresh** — stale quota data leads to bad decisions
2. **Set conservative thresholds** — better to hold at 75% than exhaust at 100%
3. **Monitor quota-related holds** — high hold rate on "low" may need threshold tuning
4. **Use exhausted sparingly** — it disables all routing optimization
