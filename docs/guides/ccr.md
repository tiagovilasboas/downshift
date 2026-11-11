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

## Balanced: large outputs are cut, never lost

On by default (owner decision, 2026-11-10). When CCR stored the raw output of a run
and what the agent would read is still larger than 16 KB, the Compactor cuts it to
what is acted on first, and the marker names the id to read the rest back:

| Output | What the agent reads |
|--------|----------------------|
| test runs, builds, pushes, any log | the first 60 and last 80 lines, and every failure line in between (up to 300) with the 3 lines after it |
| `git diff`, `git show` | every file and hunk header, the first 30 lines of each hunk, at most 250 in all |
| `grep`, `rg`, `git grep` | the first 20 matches of each file, at most 250, then a count per file of the rest |
| a command led by `grep`, `rg` or `git grep` that no family covers (`grep -rn x . \| head`), at any size | rows grouped by file, the first 25 of each file and 200 in all, each kept row cut at 80 bytes once a row is dropped, a count per file of the rest |

A line longer than 300 bytes is cut; a line the log cut keeps for a failure word keeps
its first 300 and its last 200 bytes, since an assertion often ends the line. File
reads and agent-desktop reads are never cut this way. The cut is made only when it saves at least a quarter of the text, and
never without stored raw output: with CCR off, this log, diff and search cut does not run.
Family summaries (`git log`, `docker ps`, lint and build reports, a long file read) still cut with CCR
off; their marker then ends `original not kept`.
`DOWNSHIFT_BALANCED=0` or `"balanced": false` in `context-optimization.json` turns it
off; `downshift doctor` shows the mode, the threshold and the line caps in effect as
`compactor mode`.

The threshold and the line caps can be set in `context-optimization.json` (state dir);
an absent or zero key keeps its default:

| Key | Default | What it sets |
|-----|---------|--------------|
| `balanced_threshold_kb` | 16 | size above which an output is cut (at most 2048) |
| `balanced_log_head` / `balanced_log_tail` | 60 / 80 | first and last lines of a log |
| `balanced_hunk_lines` / `balanced_diff_lines` | 30 / 250 | diff lines per hunk / in all |
| `balanced_matches_per_file` / `balanced_match_lines` | 20 / 250 | search matches per file / in all |

### Output over the hold limit

The Compactor holds at most 2 MB of a run's output in memory. Before 2026-11-10 a
larger output was released and printed whole (a 13 MB `git grep` printed 13 MB). Now,
when CCR keeps every run and balanced is on, stdout past 2 MB is written to a private
temporary file instead, and memory keeps only its first 24 KB and its last 8 KB. At
exit the file is read back once and stored, and the run prints the head, a line
`… N bytes cut (downshift retrieve <id>)`, the tail and the marker. Measured on this
repository: `git grep -n "[est] "` 13,647,888 bytes printed 32,876 bytes, and
`downshift retrieve` returned every line (171 secret-looking lines withheld by the
redaction below). Stderr is printed as it comes while stdout is spooled. With CCR or
balanced off, with an output over 32 MiB, or when the store refuses it, the run prints
everything raw as before and `report` lists it as `too_large`.

### Stderr is held, not a reason to stream

A run that wrote on stderr used to stream its whole output raw, unless its family
reads stderr with its report. Now both streams are held in write order: stdout is
compacted (or cut, as above) and the stderr lines follow it whole, on stderr. Output
that stays raw is replayed in the order it was written. When a terminal is on stdin,
the first stderr byte still releases the output, so a prompt is never hidden
(`report`: `stderr_output`).

Same limits, redaction and secret-path refusals for both modes (below); a file read of
a secret path is never held, so never stored.

## After the run: tool results the hook replaces

On a harness whose hook can replace a tool's result after it ran (Claude Code,
see the [harness matrix](../reference/harness-matrix.md#replace-tool-output-after-the-run-2026-11-10)),
the same store backs a second path. A shell command the pre-run rewrite did not
wrap (a pipe, a redirect, a long command) and a large Grep or Glob result are
cut after the run, and only when CCR stored the raw text first: a log to its
head, tail and failure lines, a search to the first matches of each file and a
count of the rest, a file list to its head. The last line names the
`downshift retrieve` id, as for a wrapped run. Read results are never changed.
With CCR off nothing is cut this way; `DOWNSHIFT_POST_COMPACT=0` turns the path
off on its own. Design and measurements:
[post-run compaction](../design/post-run-compaction.md).

## What happens

With CCR on, when `downshift compact -- <cmd>` compacts output:

1. The raw output is stored at `~/.downshift/ccr/<id>.txt`.
2. The marker line names the id:
   `[downshift compact: N to M bytes; downshift retrieve <id>]`
   (with a session: `... downshift retrieve --session=<ref> <id>]`, see
   [Bytes read back are subtracted](#bytes-read-back-are-subtracted))
3. Runs of 8 or more same-shape lines (digits ignored) fold to the first 3 and last 2
   lines plus `[... N similar lines folded]`. A clean run the compressor chain would
   have left raw gets this fold too. The fold happens only when the raw output was
   stored, so every cut line stays retrievable.

## Retrieve

```bash
downshift retrieve <id> [--grep RE] [--head N] [--tail N] [--lines A:B] [--session=REF]
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
- One entry is at most 32 MiB (8 MiB before 2026-11-10, raised so a spooled output
  fits); the total is pruned to 128 MiB.
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

`make bench` ([bench.md](../reference/bench.md)): total saving is 52.6% with CCR off and
69.4% with CCR on, which is now the default; the generic family goes from 18.4% to 93.1%.

The exec log records raw and emitted bytes after the fold, so savings include CCR.

### Bytes read back are subtracted

A saving that the agent takes back is not a saving. Each compacted run that kept its
raw output records the CCR id (a hash of the stored text) in `exec-events.jsonl`. Each
`downshift retrieve` (the command and the MCP tool, both) appends the id and the
number of bytes it returned to `exec-retrievals.jsonl`, and nothing else: no query,
no output, no path. When the logs are read, the bytes returned for an id are matched
to the runs that kept it and subtracted from their saving, never below zero.

Two sessions that keep the same output share one id, so the id alone cannot say who
read it. When a run has a session, its marker carries a short reference to it:
`downshift retrieve --session=<ref> <id>`, where `<ref>` is the first 12
hex characters of the hashed session (the same hash the exec log carries). The
`retrieve` command takes `--session=<ref>` (12 to 64 lowercase hex characters, else it
is ignored) and records the 12-character reference in the row as `session`. A row with
a session is charged only to runs of that session that kept the id. Without a session
in the marker (a harness or a call that has none) the hint is just
`downshift retrieve <id>`.

When no copy of the raw output exists (CCR off, `"ccr": false` on a clean run, a family that
never stores its output such as `curl` or `wget`, or a store that failed), a cut output ends
`[downshift compact: N to M bytes; original not kept]`: the cut is final, so run the
command again if you need what was cut. The older spelling
`compacted N to M bytes; full output: downshift retrieve <id>` is still read, never printed.

What is session-scoped and what is per id:

| Read | Charged to |
|------|------------|
| `downshift retrieve --session=<ref> <id>` (copied from a marker) | the runs of that session that kept `<id>` |
| `downshift retrieve <id>` without the flag | every run that kept `<id>`, in log order |
| MCP tool `retrieve` (it records no session) | every run that kept `<id>`, in log order |
| Rows written before the `session` field | every run that kept `<id>`, in log order |

The post-run hook that cuts a large tool result carries the session in its hint the
same way, when the harness gave it a valid one. The MCP `retrieve` tool takes the id
as the last token of the hint (a pasted `--session=<ref> <id>` works) and records no
session, so a read through it is per id. A marker that would end up as long as the
raw output is not written: the run keeps the raw text.

- `report`, `session`, the panel, the context summary and `stats` show the net saving.
  The compactor line adds `Read back with retrieve: N bytes` when there is any.
- A session counts only the retrievals of its own runs (a read without a session
  is matched by id, see above).
- Two runs with the same output share one id, so one read is shared out between
  them in log order, each up to what it saved.
- Logs written before this change have no ids and no retrievals, and their totals
  do not move.
- A small run can end at zero: the marker that names the id costs bytes, and a
  `retrieve` of the whole output returns more than the run saved.
