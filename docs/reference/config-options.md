# Configuration Options

Complete reference for Downshift configuration.

## Configuration File

Location: `~/.downshift/config.json`

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DOWNSHIFT_EXEC` | `1` | Enable/disable compaction |
| `DOWNSHIFT_ROUTE` | `1` | Enable/disable routing |
| `DOWNSHIFT_LEARNING` | `1` | Enable/disable learning |
| `DOWNSHIFT_CONTROL_SHARE` | `0.10` | Control group fraction |
| `DOWNSHIFT_STATE_DIR` | `~/.downshift` | State directory |
| `DOWNSHIFT_LOG_LEVEL` | `info` | Log verbosity |
| `DOWNSHIFT_TELEMETRY` | `1` | Enable/disable telemetry |

## Config Commands

```bash
# View all config
downshift config list

# Get a value
downshift config get <key>

# Set a value
downshift config set <key> <value>
```

## Common Settings

### Telemetry

```bash
downshift config set telemetry.retention_days 30
downshift config set telemetry.log_prompts false
```

### Routing

```bash
downshift config set routing.confidence_threshold 0.6
downshift config set routing.risk_floor true
```

### Learning

```bash
downshift config set learning.min_observations 3
downshift config set learning.decay_days 30
```

## Project-Level Config

Create `.downshift/policy.json` in your project:

```json
{
  "floor": "MID"
}
```

## See Also

- [Configuration Guide](../guides/config.md)
- [CLI Reference](cli.md)
