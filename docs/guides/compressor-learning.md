# Compressor Learning

> **Status: in development.** It works, but its interface may change between releases.

The compressor learning system observes unrecognised command outputs and
identifies candidates for new compressors. It is a **guia layer** (Fowler):
it surfaces patterns before the developer writes the compressor.

## How it works

1. When `downshift compact` runs a command whose output is not compacted
   (format unknown or not matching the expected format), the learning hook
   runs after the output is written and the exit code is known, under a
   short deadline (500 ms), so the record is on disk when the command returns.

2. The hook extracts a **signature** from the command: the base name of
   `argv[0]` (no directory, lowercase, `.exe` stripped) and, for a command
   that takes subcommands (`git`, `gh`, `go`, `npm`, `cargo`, `kubectl`,
   `docker` and the like), at most one subcommand word (`argv[1]`). For any
   other command (`grep`, `rg`, `ls`, `find`) the first argument is a pattern
   or a path, so it is never stored. A subcommand is stored only if it is
   made of lowercase letters, digits, `-` and `_` as written, does not start
   with `-`, and is at most 24 bytes. Any other word is dropped, never stored: paths, URLs,
   `KEY=value` pairs, tokens, branch names, flags. A command whose name fails
   that shape is recorded as `other` with no subcommand.

3. The signature, count, and the **shape** of one output (counts and
   buckets, never text) are stored in `~/.downshift/compressor-learning.json`.

4. When a signature reaches 5+ occurrences, it becomes a **suggestion**
   for a new compressor.

## Performance

- **No impact on the output or the exit code**: the hook runs after the
  output is already written. Its write is bounded by a 500 ms deadline; a
  hook that overruns is abandoned and the command returns as usual.
- **Fail-open**: errors and panics in the hook are silently ignored.
- **Kill switch**: Set `DOWNSHIFT_LEARNING=0` to disable entirely.

## Commands

```bash
# Show suggestions (highest impact first)
downshift learning suggestions

# Show statistics
downshift learning stats

# Export the store as JSON (shapes and counts, no output text)
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

- Command signature (command base name plus at most one safe subcommand
  word, for example `git push` or `go test`)
- Count of executions
- First and last seen timestamps
- Total bytes and average bytes
- The size of one output (`sample_bytes`) and its **shape** (`shape`):
  - line count and per-line class counts: `blank`, `short` (≤ 20 bytes),
    `long` (> 120 bytes), `indented`, `number_heavy` (≥ 30% digits),
    `path_like`, `url_like`, `status_like` (ok / PASS / FAIL / error ...);
    classes are independent, so one line may count in several
  - `line_lengths`: four buckets (0–20, 21–80, 81–160, over 160 bytes)
  - `bytes`: a histogram by class (letters, digits, space, punctuation, other)
  - `first_line_shape`: a truncated hash of the first line's character-class
    mask (letters, digits and spaces collapsed), so outputs with the same
    header layout share it
  - `scanned_bytes`: how much of the output the counts cover (at most 64 KiB)
- execwrap family, when known

## What it never records

- Full command line
- File paths, directories, URLs, credentials, `KEY=value` pairs, branch
  names or any argument beyond the one safe subcommand word
- A store written by an earlier version may hold such words (the first 3
  argv words). On first load it is cleaned: entries whose command word fails
  the shape are dropped, extra words are removed, entries that become
  identical are merged, and the file is rewritten atomically (temp file and
  rename, owner-only 0600).
- **Any output text.** Schema 1 (before this rule) kept up to 4 KiB of raw
  output per signature, which could hold branch names, URLs and file paths.
  A schema 1 store is migrated on first use: the sample is dropped and the
  file rewritten at schema 2. `downshift learning clear` purges it now.
- User data or environment variables

## For compressor authors

Use `downshift learning export` to see the shapes. The counts tell you whether
an output is table-like (many number-heavy lines), log-like (many long or
path-like lines) or a status summary, and `first_line_shape` tells you whether
the header is stable. Reproduce the command locally to see the actual text
when writing the detection regex: the store never holds it.
