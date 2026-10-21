# Compressor Learning

Downshift learns which commands produce uncompacted output and suggests candidates
for new compressors. This is a **guia layer** (Fowler): it surfaces patterns before
you write the compressor.

## How It Works

When a command passes through `downshift compactor` but its output is not compacted
(unknown format), the learning system:

1. **Extracts a signature** — first 3 words, normalized (lowercase, versions stripped)
2. **Records metadata** — count, timestamps, total bytes, truncated sample
3. **Stores locally** — `~/.downshift/compressor-learning.json`
4. **Suggests when ready** — patterns with 5+ occurrences become suggestions

## Zero Performance Impact

The learning hook runs **asynchronously** in a separate goroutine:

```
Command runs → Output written → Exit code returned → [async] Learning hook
                                      ↑
                              User sees this immediately
```

Your command completes at full speed. The learning hook runs fire-and-forget
after the output is already written.

## Commands

### View Suggestions

```bash
$ downshift learning suggestions

Compressor suggestions (highest impact first):

→ 1. npm audit
      Count: 47 runs · Avg: 12.3 KB · Total: 578.1 KB
      Family: npm-install

  2. pip show
      Count: 23 runs · Avg: 1.2 KB · Total: 27.6 KB

→ = recommended for new compressor (high count + high bytes)
```

### View Statistics

```bash
$ downshift learning stats

Compressor Learning Stats
─────────────────────────
Unique signatures:  12
Total commands:     156
Total bytes:        2.3 MB
Ready to suggest:   3
```

### Export Raw Data

```bash
$ downshift learning export
{
  "version": 1,
  "entries": {
    "abc123": {
      "signature": {"words": ["npm", "audit"], "hash": "abc123"},
      "count": 47,
      "first_seen": "2026-10-01T10:00:00Z",
      "last_seen": "2026-10-21T12:00:00Z",
      "total_bytes": 591052,
      "sample_bytes": 4096,
      "sample": "...",
      "family": "npm-install"
    }
  }
}
```

### Clear Store

```bash
$ downshift learning clear
Cleared learning store
```

## Kill Switch

Disable learning entirely:

```bash
export DOWNSHIFT_LEARNING=0
```

Or in config:

```json
{
  "learning": false
}
```

## What Gets Recorded

| Field | Description |
|-------|-------------|
| Signature | First 3 words, normalized |
| Count | Number of occurrences |
| Timestamps | First and last seen |
| Total bytes | Cumulative output size |
| Sample | Truncated output (4KB max) |
| Family | execwrap family, when known |

## What Never Gets Recorded

- Full command line
- File paths or arguments
- User data or environment variables
- Output beyond the truncated sample

## Privacy

The learning system records only:
- Normalized command names (no paths, no arguments)
- Byte counts and timestamps
- One truncated sample per signature

It never records:
- Full command lines
- File contents
- Environment variables
- User data

## For Compressor Authors

Use `downshift learning export` to inspect samples. The `sample` field shows
the format structure, helping you write detection regexes and compression logic.

Example workflow:

1. Run `downshift learning suggestions` to find high-impact candidates
2. Export and inspect the sample for the top candidate
3. Write a compressor in `internal/compressor/`
4. Test with the corpus: `go test ./internal/compressor/...`
5. Submit a PR

See [Adding Compressors](../contributing/compressors.md) for the full guide.
