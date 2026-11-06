# CCR: compress, cache, retrieve

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

CCR lets `downshift compact` cut more output without losing it: the raw output is
kept on disk and one command brings back any part of it. Full CCR is **opt-in and off
by default**; the raw output of a **failed** compacted command is kept by default.

## Failures are kept by default

When `downshift compact` compacts the output of a command that exited non-zero, the
raw output is stored and the marker names the id, so the agent can run
`downshift retrieve <id>` (or the MCP `retrieve` tool). Successful runs are not stored
unless full CCR is on. Same limits, redaction and secret-path refusals as below; a
file read of a secret path is never held, so never stored.

| Setting | Effect |
|---------|--------|
| `"ccr_failures": false` in `context-optimization.json` | Off |
| `DOWNSHIFT_CCR_FAILURES=0` | Off, even if the file says otherwise |
| `DOWNSHIFT_CCR_FAILURES=1` | On, even if the file says `false` |
| `DOWNSHIFT_CCR=0` | Off (this and full CCR) |
| `"exec": false` / `DOWNSHIFT_EXEC=0` | Compaction is off, so nothing is stored |

## Turn full CCR on

| Setting | Effect |
|---------|--------|
| `"ccr": true` in `context-optimization.json` (state dir) | On |
| `DOWNSHIFT_CCR=1` | On |
| `DOWNSHIFT_CCR=0` | Off, even if the file says `true` |

The environment wins over the file. See [config.md](config.md).

## What happens

With CCR on, when `downshift compact -- <cmd>` compacts output:

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

CCR writes command output to disk, which telemetry never does; that is why it is
opt-in. The output stays local, is redacted line by line, and expires. Prompts and
file paths are still never stored in telemetry, but a stored output can contain
paths or text the command echoed.

- Redaction withholds whole lines that look like credentials (tokens, auth
  headers, cookies, URL credentials, JWTs, known key prefixes) and every line of
  a PEM block. It is a heuristic: do not turn CCR on for commands that print
  secrets in other shapes.
- The store refuses a `ccr` directory that is a symlink or owned by another user,
  and never follows a link planted in place of an entry.
- On Windows the 0700/0600 modes do not apply; files inherit the profile's ACL.
- `downshift uninstall --purge` removes the store with the rest of `~/.downshift`.

## Measured effect

`make bench` ([bench.md](../reference/bench.md)): total saving is 48.9% by default and
76.1% with CCR on; the generic family goes from 18.4% to 93.1%.

The exec log records raw and emitted bytes after the fold, so savings include CCR.
Known gap: bytes later fetched with `retrieve` are not subtracted yet.
