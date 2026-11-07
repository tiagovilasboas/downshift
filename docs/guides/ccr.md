# CCR: compress, cache, retrieve

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

CCR lets `downshift compact` cut more output without losing it: the raw output is
kept on disk and one command brings back any part of it. CCR is **native and always on**.
It is part of the compactor, not an option: there is no config key, environment variable
or flag that turns it off, and every output the compactor cuts is kept, a success as well as
a failure.

## Always on

- The only way to keep nothing is to turn compaction itself off (`"exec": false` in
  `context-optimization.json`, or `DOWNSHIFT_EXEC=0`): then nothing is cut, so nothing needs
  keeping. See [config.md](config.md).
- `"ccr"`, `"ccr_failures"`, `DOWNSHIFT_CCR` and `DOWNSHIFT_CCR_FAILURES` of earlier versions are
  ignored. A config file that still has the keys loads without error.
- Refusals stay: the output of `curl`, `wget`, `glab` and of a read of a secret path (`.env`, keys)
  is never stored, since it can carry cookies or tokens. Lines that look like secrets are
  withheld from every entry (see Privacy).
- If the store cannot be written (full disk, unwritable directory), the run still prints its
  compacted output, without an id: a storage error never changes a command's output or exit code.

## What happens

When `downshift compact -- <cmd>` compacts output:

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

CCR writes command output to disk, which telemetry never does, and since it is always
on, that is true for every cut output. The output stays local, is redacted line by line,
and expires after 24 hours. Prompts and
file paths are still never stored in telemetry, but a stored output can contain
paths or text the command echoed.

- Redaction withholds whole lines that look like credentials (tokens, auth
  headers, cookies, URL credentials, JWTs, known key prefixes) and every line of
  a PEM block. It is a heuristic, and CCR cannot be turned off: a command that prints
  secrets in other shapes will have them in `~/.downshift/ccr/` until the entry expires,
  so keep such commands out of the compactor's allowlist or run them unwrapped.
- The store refuses a `ccr` directory that is a symlink or owned by another user,
  and never follows a link planted in place of an entry.
- On Windows the 0700/0600 modes do not apply; files inherit the profile's ACL.
- `downshift uninstall --purge` removes the store with the rest of `~/.downshift`.

## Measured effect

`make bench` ([bench.md](../reference/bench.md)): total saving is 76.1% with CCR; the lossless
compressors alone give 48.9%, and the generic family goes from 18.4% to 93.1% with the fold.

The exec log records raw and emitted bytes after the fold, so savings include CCR.
Bytes later fetched with `retrieve` are taken off again; see the next section.

## Retrieved bytes are subtracted from the savings

A cut that the agent immediately undoes saved nothing. So every `retrieve` (the CLI and
the MCP tool go through the same function) that returns bytes writes one row to
`~/.downshift/exec-retrievals.jsonl`, next to the exec log:

```json
{"ts":"2026-11-06T18:02:11Z","id":"3f9a1c2b7d40","bytes":18432}
```

That is all it holds: the 12-hex id, the number of bytes returned (after `--grep`, `--head`,
`--tail`, `--lines`, and after the MCP tool's size cut) and the time. Never the command, the
output, the query, a path or a prompt. A `retrieve` that fails or returns nothing writes nothing.
The compacted run's own row in `exec-events.jsonl` gains the same `kept_id`, which is how the two
are matched.

`downshift report`, `downshift session` (the panel), `report --daily/--history`, the compaction
detail and the context summary all read the same figure:

- A compacted run's saved bytes are `raw - emitted - retrieved`, **floored at zero per run**. A
  run fetched back in full, or more than once, reports `0` saved bytes; it never goes negative
  and never reduces another run's saving.
- The bytes fetched back appear on their own line, next to the savings, so both numbers are
  visible: `Fetched back with retrieve: 20400 bytes (already taken off the saved bytes)` in the
  report (markdown, `--text`, the styled view), `retrieved_bytes` in `--json`, in the session panel
  and in the context summary (`exec_metrics.retrieved_bytes`). The line is absent when nothing was
  fetched back, and a log with no retrieval rows sums exactly as it did before.
- An id retrieved twice counts twice. A retrieval whose id matches no run in the log (the run
  rotated out, or the output was stored on another machine) changes nothing.
- Two runs can share an id when their redacted output is identical. The bytes fetched for that id
  are charged to the oldest run first, up to its saving, then to the next; the newest run takes
  any remainder, so `retrieved_bytes` always adds up to what was fetched.
- Rotation: the retrievals log rotates at 10 MiB like the exec log, and readers sum the `.1`
  generation with the current file. `downshift report clear` moves both away.
- Entries expire after 24 hours, so a retrieval always belongs to a recent run.

The token figures are still `saved_bytes / 4`, an estimate, now of the net bytes.
