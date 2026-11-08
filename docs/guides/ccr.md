# CCR: compress, cache, retrieve

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift-ai

CCR lets `downshift compact` cut more output without losing it: the raw output is
kept on disk and one command brings back any part of it. It is **on by default**
(owner decision, 2026-11-06): every compacted run is kept. Before that date full CCR
was opt-in and only the output of failed runs was kept; both settings below still exist.

## What is kept, and how to change it

| Setting | Effect |
|---------|--------|
| nothing set | Every compacted run is kept, failures included |
| `"ccr": false` in `context-optimization.json` (state dir) | Only the output of a **failed** compacted command is kept |
| `"ccr": false` and `"ccr_failures": false` | Nothing is kept |
| `DOWNSHIFT_CCR=0` | Nothing is kept, failures included, even if the file says otherwise |
| `DOWNSHIFT_CCR=1` | Every compacted run is kept, even if the file says `"ccr": false` |
| `DOWNSHIFT_CCR_FAILURES=0` / `1` | Failure-only keeping off / on, even if the file says otherwise |
| `"exec": false` / `DOWNSHIFT_EXEC=0` | Compaction is off, so nothing is stored |

The environment wins over the file. A `context-optimization.json` that cannot be read
never turns on keeping every run: it falls back to failures only. See [config.md](config.md).

Same limits, redaction and secret-path refusals for both modes (below); a file read of
a secret path is never held, so never stored.

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
  `[downshift ccr: line withheld, it looked like a secret]`, and structured credentials
  (AWS STS session credentials, SSM parameter values, and AWS environment variables)
  are redacted in place to `[REDACTED]`.

## Privacy

CCR writes command output to disk, which telemetry never does; that is why it was
opt-in until 2026-11-06 and why it has an off switch. The output stays local, is
redacted line by line, and expires after 24 hours. Prompts and
file paths are still never stored in telemetry, but a stored output can contain
paths or text the command echoed.

- Redaction withholds whole lines that look like credentials (tokens, auth
  headers, cookies, URL credentials, JWTs, known key prefixes) and every line of
  a PEM block. AWS STS credentials (`AccessKeyId`, `SecretAccessKey`, `SessionToken`,
  `ASIA...` keys across JSON, CLI table, text, and environment variables) and AWS SSM
  Parameter Store outputs (`Value` inside `Parameter`/`Parameters` JSON blocks, CLI text,
  and table parameter rows) are automatically redacted to `[REDACTED]` without breaking
  normal non-secret JSON fields or metadata. It is a heuristic: do not turn CCR on for
  commands that print secrets in other unsupported shapes. If you work with such output,
  set `DOWNSHIFT_CCR=0`, and run `rm -r ~/.downshift/ccr` to drop what is already kept.
- The store refuses a `ccr` directory that is a symlink or owned by another user,
  and never follows a link planted in place of an entry.
- On Windows the 0700/0600 modes do not apply; files inherit the profile's ACL.
- `downshift uninstall --purge` removes the store with the rest of `~/.downshift`.

## Measured effect

`make bench` ([bench.md](../reference/bench.md)): total saving is 48.9% with CCR off and
76.1% with CCR on, which is now the default; the generic family goes from 18.4% to 93.1%.

The exec log records raw and emitted bytes after the fold, so savings include CCR.

### Bytes read back are subtracted

A saving that the agent takes back is not a saving. Each compacted run that kept its
raw output records the CCR id (a hash of the stored text) in `exec-events.jsonl`. Each
`downshift retrieve` (the command and the MCP tool, both) appends the id and the
number of bytes it returned to `exec-retrievals.jsonl`, and nothing else: no query,
no output, no path. When the logs are read, the bytes returned for an id are matched
to the runs that kept it and subtracted from their saving, never below zero.

- `report`, `session`, the panel, the context summary and `stats` show the net saving.
  The compactor line adds `Read back with retrieve: N bytes` when there is any.
- A session counts only the retrievals of its own runs.
- Two runs with the same output share one id, so one read is shared out between
  them in log order, each up to what it saved.
- Logs written before this change have no ids and no retrievals, and their totals
  do not move.
- A small run can end at zero: the marker that names the id costs bytes, and a
  `retrieve` of the whole output returns more than the run saved.
