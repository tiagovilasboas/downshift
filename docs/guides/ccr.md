# CCR: compress, cache, retrieve

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

CCR lets `downshift compact` cut more output without losing it: the raw output is
kept on disk and one command brings back any part of it. Full CCR is **on by default**
(since the change recorded in the changelog); turn it off with `"ccr": false` or
`DOWNSHIFT_CCR=0`. With full CCR off, the raw output of a **failed** compacted command is
still kept.

## What is kept, and when

| Run | Full CCR on (default) | Full CCR off |
|-----|-----------------------|--------------|
| Compacted, exit 0 | Kept | Not kept |
| Compacted, exit non-zero | Kept | Kept (failure retention) |
| Not compacted (output went through raw) | Nothing to retrieve | Nothing to retrieve |

The marker names the id, so the agent can run `downshift retrieve <id>` (or the MCP
`retrieve` tool). Same limits, redaction and secret-path refusals as below; a file read
of a secret path is never held, so never stored.

## Turn it off

| Setting | Effect |
|---------|--------|
| `"ccr": false` in `context-optimization.json` (state dir) | Full CCR off; failures still kept |
| `"ccr": false` and `"ccr_failures": false` | No raw output is kept |
| `DOWNSHIFT_CCR=0` | Off, including failure retention, even if the file says `true` |
| `DOWNSHIFT_CCR=1` | On, even if the file says `false` |
| `DOWNSHIFT_CCR_FAILURES=0` | Failure retention off; it changes nothing while full CCR is on |
| `"exec": false` / `DOWNSHIFT_EXEC=0` | Compaction is off, so nothing is stored |

The environment wins over the file. A config file that cannot be read turns full CCR off
(its own opt-out cannot be seen) and leaves failure retention on. See [config.md](config.md).

## What happens

With CCR on (the default), when `downshift compact -- <cmd>` compacts output:

1. The raw output is stored at `~/.downshift/ccr/<id>.txt`.
2. The marker line names the id:
   `[downshift compact: compacted N to M bytes; full output: downshift retrieve <id>]`
3. Runs of 8 or more same-shape lines (digits ignored) fold to the first 3 and last 2
   lines plus `[... N similar lines folded]`. A clean run the compressor chain would
   have left raw gets this fold too. The fold happens only when the raw output was
   stored, so every cut line stays retrievable.

## Retrieve

```bash
downshift retrieve <id> [--grep RE] [--head N] [--tail N] [--lines A:B]
downshift retrieve 3f9a1c2b7d40 --grep 'FAIL|panic' --tail 40
downshift retrieve 3f9a1c2b7d40 --lines 120:180
```

| Exit | Meaning |
|------|---------|
| 0 | Success |
| 1 | Id not found or expired |
| 2 | Bad arguments or a bad id |

The MCP tool `retrieve` takes `{id, grep, head, tail, lines}` and returns the same text.

## Storage limits

- Files are mode 0600 in a 0700 directory.
- Entries are kept for 24 hours.
- One entry is at most 8 MiB; the total is pruned to 64 MiB.
- Lines that look like secrets are replaced with
  `[downshift ccr: line withheld, it looked like a secret]`.

## Privacy

CCR writes command output to disk, which telemetry never does, and it is on by default,
so read this section before relying on it. The output stays local, is redacted line by line, and expires. Prompts and
file paths are still never stored in telemetry, but a stored output can contain
paths or text the command echoed.

- Redaction withholds whole lines that look like credentials (tokens, auth
  headers, cookies, URL credentials, JWTs, known key prefixes) and every line of
  a PEM block. It is a heuristic: turn CCR off (`DOWNSHIFT_CCR=0`) for commands that print
  secrets in other shapes.
- The store refuses a `ccr` directory that is a symlink or owned by another user,
  and never follows a link planted in place of an entry.
- On Windows the 0700/0600 modes do not apply; files inherit the profile's ACL.
- `downshift uninstall --purge` removes the store with the rest of `~/.downshift`.

## Measured effect

`make bench` ([bench.md](../reference/bench.md)): total saving is 48.9% with CCR off and
76.1% with CCR on (now the default); the generic family goes from 18.4% to 93.1%.

The exec log records raw and emitted bytes after the fold, so savings include CCR.
Known gap: bytes later fetched with `retrieve` are not subtracted yet.
