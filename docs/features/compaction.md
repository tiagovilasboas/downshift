# Output Compaction

Downshift's compactor reduces verbose command output to essential information,
preserving exit codes and failure details while dramatically cutting token usage.

## How It Works

When you wrap a command with `downshift compactor`:

```bash
downshift compactor -- go test ./...
```

Downshift:

1. **Runs the command** with no shell, capturing stdout/stderr
2. **Checks the exit code** — only compacts on success (exit 0) with empty stderr
3. **Identifies the format** — matches output against known compressor patterns
4. **Compacts if recognized** — reduces to PASS/FAIL summary
5. **Preserves if not** — unknown formats pass through unchanged

## Compression Ratios

| Command | Typical Output | Compacted | Ratio |
|---------|---------------|-----------|-------|
| `go test` (50 packages, all pass) | ~2,000 tokens | ~50 tokens | 97% |
| `git status` (clean) | ~200 tokens | ~20 tokens | 90% |
| `cargo build` (no warnings) | ~1,500 tokens | ~100 tokens | 93% |
| `npm test` (all pass) | ~1,000 tokens | ~80 tokens | 92% |
| `pytest` (100 tests pass) | ~800 tokens | ~60 tokens | 92% |

## What Gets Preserved

Compaction **never hides problems**:

- ❌ Failures → full output preserved
- ❌ Warnings → full output preserved  
- ❌ Stack traces → full output preserved
- ❌ Non-zero exit codes → full output preserved
- ✅ Clean success → compacted summary

## Supported Tools

See [Supported Tools](../reference/supported-tools.md) for the complete list of
70+ supported command families.

## Configuration

### Disable Compaction

```bash
# Environment variable
export DOWNSHIFT_EXEC=0

# Or in config (~/.downshift/config.json)
{
  "exec": false
}
```

### Per-Command Disable

The compactor refuses non-allowlisted commands, so you can always run directly:

```bash
# This runs through compactor
downshift compactor -- go test ./...

# This runs directly (no compaction)
go test ./...
```

## Claude Code Integration

On Claude Code, `downshift setup` installs a plugin that automatically rewrites
allowlisted commands to use the compactor. No manual wrapping needed.

The plugin only rewrites commands that:
- Match the allowlist
- Have no shell metacharacters (pipes, redirects, globs)
- Are not already denied by your permission rules

## Metrics

Track your compaction savings:

```bash
# Current session
downshift session

# Historical
downshift stats
downshift report --period=7d
```

## Technical Details

- **No shell execution** — commands run directly via `exec`, not through a shell
- **Exit code preservation** — the original exit code is always returned
- **Stderr passthrough** — stderr is never modified
- **Format detection** — each compressor has a regex pattern for its format
- **Fail-open** — unrecognized formats pass through unchanged
