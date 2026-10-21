# Quickstart

Get Downshift running in 5 minutes.

## Prerequisites

- An AI coding agent (Claude Code, Codex, Cursor, or KiroCrew)
- macOS, Linux, or Windows (WSL)

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh
```

If it reports that a release has no archive for your platform, choose another tag from [Releases](https://github.com/tiagovilasboas/downshift/releases) (`sh -s <tag>`); see [install troubleshooting](install.md#404-when-downloading-no-archive-for-this-release).

## Configure

```bash
downshift setup
```

This auto-detects your installed harnesses and configures hooks.

## Verify

```bash
downshift doctor
```

Check that everything is working.

## Try It

Run a command through the compactor:

```bash
downshift compactor -- go test ./...
```

Or test classification:

```bash
downshift try "rename userId to userID"
```

## Next Steps

- [Configuration Guide](config.md)
- [Feature Documentation](../features/)
- [CLI Reference](../reference/cli.md)
