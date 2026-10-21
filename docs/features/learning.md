# Compressor Learning

The compressor learning system observes unrecognised command outputs and
identifies candidates for new compressors. It is a **guia layer** (Fowler):
it surfaces patterns before the developer writes the compressor.

## How it works

1. When `downshift compact` runs a command whose output is not compacted
   (format unknown or not matching the expected format), the learning hook
   is called **asynchronously** in a separate goroutine.

2. The hook extracts a **signature** from the command: the first 3 words,
   normalised (lowercase, version suffixes stripped). This never includes
   file paths, arguments, or other sensitive data.

3. The signature, count, and a truncated sample are stored in
   `~/.downshift/compressor-learning.json`.

4. When a signature reaches 5+ occurrences, it becomes a **suggestion**
   for a new compressor.

## Performance

- **Zero impact on command execution**: The hook runs in a goroutine after
  the command output is already written. The command exit code returns
  immediately.
- **Fire-and-forget**: Errors in the hook are silently ignored.
- **Kill switch**: Set `DOWNSHIFT_LEARNING=0` to disable entirely.

## Commands

```bash
# Show suggestions (highest impact first)
downshift learning suggestions

# Show statistics
downshift learning stats

# Export raw store as JSON
downshift learning export

# Clear the store
downshift learning clear
```

## Example output

```
$ downshift learning suggestions

Compressor suggestions (highest impact first):

→ 1. npm audit
      Count: 47 runs · Avg: 12.3 KB · Total: 578.1 KB
      Family: npm-install

  2. pip show
      Count: 23 runs · Avg: 1.2 KB · Total: 27.6 KB

→ = recommended for new compressor (high count + high bytes)
```

## What it records

- Command signature (first 3 words, normalised)
- Count of executions
- First and last seen timestamps
- Total bytes and average bytes
- One truncated sample (4 KB max) for inspection
- execwrap family, when known

## What it never records

- Full command line
- File paths or arguments
- Output content beyond the truncated sample
- User data or environment variables

## For compressor authors

Use `downshift learning export` to see the raw samples. The `sample` field
contains a truncated output that shows the format structure. This helps you
write detection regexes and compression logic for the new format.
