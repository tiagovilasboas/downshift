# Configuration Options

Complete reference for Downshift configuration.

## Configuration File

Location: `~/.downshift/config.json`

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DOWNSHIFT_EXEC` | `1` | Enable/disable compaction (`0` to disable) |
| `DOWNSHIFT_NO_ROUTE` | `0` | Disable routing (`1` to disable) |
| `DOWNSHIFT_LEARNING` | `1` | Enable/disable compressor learning |
| `DOWNSHIFT_CONTROL_SHARE` | `0.10` | Control group fraction (0 to disable) |
| `DOWNSHIFT_STATE_DIR` | `~/.downshift` | State directory |
| `DOWNSHIFT_QUIET` | `0` | Suppress non-error output |
| `DOWNSHIFT_SEMANTIC_BOOST` | `1` | Enable semantic scoring |

## Viewing Configuration

```bash
# View all effective settings with their source
downshift config

# JSON output
downshift config --json
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
