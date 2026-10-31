# Telemetry & Observability

Downshift records every routing decision locally so you can measure savings,
debug issues, and validate changes — all without sending data anywhere.

## Privacy First

All telemetry is:

- **Local only** — stored in `~/.downshift/telemetry/`
- **No network** — nothing sent to external services
- **No prompts** — task content is never logged
- **Opt-out** — `DOWNSHIFT_TELEMETRY=0` disables entirely

## What's Recorded

Each routing event captures:

```json
{
  "timestamp": "2026-10-21T05:00:00Z",
  "harness": "claude-code",
  "complexity": "SIMPLE",
  "tier": "SMALL",
  "from_model": "claude-sonnet-4",
  "to_model": "claude-haiku-4",
  "outcome": "rewrite_emitted",
  "input_tokens": 1200,
  "output_tokens": 450
}
```

### Event Types

| Outcome | Meaning |
|---------|---------|
| `rewrite_emitted` | Model was changed (savings occurred) |
| `allow` | No change needed (already optimal) |
| `baseline` | Control group (left unchanged for measurement) |
| `blocked` | Routing disabled or policy prevented |

## Viewing Telemetry

### Session Summary

```bash
downshift session
```

Output:
```
Session: abc123
Started: 2026-10-21 04:30:00
Duration: 35 minutes

Spawns: 12
  Routed: 8 (67%)
  Control: 2 (17%)
  Unchanged: 2 (17%)

Tokens:
  Without routing: 145,000
  With routing: 89,000
  Saved: 56,000 (39%)
```

### Historical Stats

```bash
downshift stats --days=7
```

### Export for Analysis

```bash
downshift report --period=30d --format=json > monthly.json
```

## Control Group

Downshift maintains a control group for accurate measurement:

```
┌─────────────────────────────────────────┐
│           All Spawns (100%)             │
├─────────────────────┬───────────────────┤
│   Routed (90%)      │  Control (10%)    │
│   Model changed     │  No change        │
│   Savings applied   │  Baseline tokens  │
└─────────────────────┴───────────────────┘
```

The control group runs unchanged so you can compare:
- Tokens used with routing
- Tokens the same work would have used without routing

### Configure Control Share

```bash
export DOWNSHIFT_CONTROL_SHARE=0.05  # 5% control group
```

Default is 10%. Set to 0 for maximum savings (no measurement baseline).

## Telemetry Schema

Events are stored as newline-delimited JSON (JSONL):

```
~/.downshift/telemetry/
├── events-2026-10-21.jsonl
├── events-2026-10-20.jsonl
└── events-2026-10-19.jsonl
```

### Schema Version

The `schema_version` field tracks format changes:

```json
{
  "schema_version": "2",
  ...
}
```

Version 2 added:
- `actual_cost_usd` / `baseline_cost_usd` (legacy: older builds wrote them; the reader ignores them)
- `decision_intelligence` for DCI metadata
- `quota_status` for budget integration

## Rotation Policy

Old telemetry is automatically rotated:

- **Daily files** — one file per day
- **Retention** — 90 days by default
- **Compression** — files older than 7 days are gzipped

Configure retention:

```bash
downshift config set telemetry.retention_days 30
```

## Aggregation

### By Harness

```bash
downshift stats --by=harness
```

```
Harness        Spawns  Routed  Saved
claude-code    1,234   89%     42%
cursor         567     85%     38%
kirocrew       234     91%     45%
```

### By Complexity

```bash
downshift stats --by=complexity
```

```
Complexity  Spawns  Avg Tokens  Savings
TRIVIAL     456     800         65%
SIMPLE      389     1,500       55%
MEDIUM      234     4,200       25%
COMPLEX     155     12,000      0%
```

### By Model

```bash
downshift stats --by=model
```

## Live Dashboard

```bash
downshift watch
```

Real-time terminal view of routing events:

```
05:01:23  claude-code  SIMPLE→SMALL  haiku    ✓ saved 1.2k tokens
05:01:45  cursor       MEDIUM→MID    sonnet   ✓ saved 800 tokens  
05:02:01  kirocrew     COMPLEX       opus     ○ no change
05:02:15  claude-code  TRIVIAL→SMALL haiku    ● control group
```

Press `q` to quit.

## Debugging

### Why Was This Routed?

```bash
downshift session --why --session=<id>
```

Shows classification signals, tier selection, and model mapping.

## Integration

### Prometheus/Grafana

Export metrics in Prometheus format:

```bash
downshift stats --format=prometheus
```

```
downshift_spawns_total{harness="claude-code"} 1234
downshift_tokens_saved_total{harness="claude-code"} 56000
downshift_savings_ratio{harness="claude-code"} 0.42
```

### JSON Export

```bash
downshift stats --format=json
```

For custom dashboards and analysis.
