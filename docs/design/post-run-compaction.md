<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Post-run compaction (spike result, tasks M1, M2, M5)

Downshift by Tiago Vilas Boas\
https://github.com/tiagovilasboas/downshift-ai

Impact roadmap, phase M: replace a tool's output after the run where the
harness allows it. Written 2026-11-10.

## Question

Today the Compactor reaches a shell command only when the pre-run hook can
wrap it in `downshift compact`. Commands with a pipe, a redirect, a quote or
over 512 bytes are never wrapped (7,769 skips in five days in the value
roadmap's log), stderr turns compaction off, and the agent's own search tools
are never reached. Claude Code's `PostToolUse` hook may answer
`hookSpecificOutput.updatedToolOutput`, which replaces a built-in tool's result
before the model reads it, provided the value keeps the tool's output shape
(hooks reference, "PostToolUse decision control", read 2026-11-10). Does a
post-run answer reach what the wrapper misses, at a cost the hook can pay?

## Design

| Layer | What it holds |
|---|---|
| `core.HarnessCapabilities.ReplacesToolOutput` | The tool categories whose result a hook can replace. Claude Code: shell, search, read. Every other harness: none |
| `hookport.ToolOutput`, `Port.ParseToolOutput`, `Port.ReplaceToolOutput` | The harness-free view of a finished call (stdout, stderr, matches, file list, exit code, the presence of each field) and the adapter's two functions: read the payload, write the answer |
| `internal/postrun` | The policy: what to compact, how, and what the exec row says. No harness name |
| `execwrap.CompactCaptured` | The wrapper's own compaction (`compactHeld`, now shared by `gate.finish`) applied to output that already exists |
| `cmd/downshift` `postRunAnswer` | Called by the honor runner. Reads the switches, the CCR port and the balanced switch, logs the exec row (entry `post_tool_use`), fails open |
| `internal/adapters/claudecode/tooloutput.go` | Reads `tool_response` and answers `updatedToolOutput` with the tool's own object, only the text fields it carried replaced |

Rules:

- **No double work.** A command already in the wrapper's form, one the pre-run
  rewrite would wrap (`shellwrap.WouldRewrite`), and an output that ends with
  the wrapper's marker are left alone and not logged. Claude Code may report
  the command as written or as rewritten; both are covered.
- **Same compaction as the wrapper.** One plain allowlisted command gets its
  family's compressor. Anything else (a pipe, a redirect, a chain, an unknown
  program) gets only the balanced log cut, which needs the raw output stored by
  CCR and a gain of at least a quarter. stdout is compacted; stderr is cut only
  when it alone is over 16 KB.
- **Search (M2).** A Grep or Glob result over 16 KB is cut once CCR stored it:
  matches keep the first 20 of each file and a count per file of the rest
  (`compressor.BalanceMatches`), held to 12 KB; a file list keeps its head
  within 12 KB. The last line, or the last list entry, names the retrieve id.
- **Shape.** Only `stdout`, `stderr`, `content` and `filenames` are written, and
  only when the result already carried them. Every other field, counts
  included, is sent back as received. An interrupted run, an image, a
  `filenames` that is not a list of strings, or a `tool_response` that is not
  an object is never replaced.
- **Read is never touched** (M3 is a later task: an edit needs the exact text).
- **Switches.** On by default where the capability is declared.
  `DOWNSHIFT_POST_COMPACT=0` turns it off; `DOWNSHIFT_EXEC=0` and
  `"exec": false` turn it off with the rest of the Compactor. Without CCR
  nothing is cut with loss.
- **Fail open.** A payload over the post-run cap (16 MiB; every other hook
  keeps 1 MiB), a parse error, a store
  error or a panic leaves the hook's usual answer (`{}`).

## Measurements

Built binary, isolated `HOME` and state directory, 2026-11-10. "Wrapper" is
`downshift compact -- <argv>`; its bytes come from the exec row and the printed
output. "Post-run" pipes a real-shaped `PostToolUse` payload, built from what
the command printed, into `downshift claude-code-post-tool-use`; bytes are the
text fields of `tool_response` before and of `updatedToolOutput` after, and the
whole answer JSON. Commands ran on this repository.

| Command | Path | Before | After | Answer JSON |
|---|---|---:|---:|---:|
| `go test -count=1 -v ./internal/compressor/` | wrapper | 592,032 | 10,555 | n/a |
| same, `2>&1 \| cat` | post-run | 591,993 | 12,419 | 12,837 |
| same, `> f 2>&1; cat f` (redirect) | post-run | 591,993 | 12,419 | 12,837 |
| `git log -p -n 6` | wrapper | 496,903 | 28,282 | n/a |
| same, `\| cat` | post-run | 496,903 | 28,283 | 29,948 |
| same, `\| grep -v '^$'` | post-run | 496,886 | 28,318 | 29,986 |
| `git grep -n err -- internal \| grep -v _test` | post-run | 390,478 | 40,899 | 42,478 |
| `find . -name '*.go' \| sort` | post-run | 66,113 | 8,241 | 8,601 |
| `git grep -n err \| head -n 5000` | post-run | 570,143 | unchanged | `{}` |
| `git log -p -n 40` | wrapper | 4,168,017 | unchanged | n/a |
| same, `\| cat` | post-run | 4,168,017 | unchanged | `{}` |

The `| head -n 5000` pipe is a shape the pre-run rewrite already wraps, so the
post-run path leaves it to the wrapper and logs nothing. The 4 MB `git log` is
over the wrapper's 2 MB hold limit; the post-run hook now reads it (cap 16 MiB,
owner decision 2026-11-10; it was 1 MiB when these figures were taken).

Synthetic payloads in the shapes the hooks reference documents (Bash) or that
the result carries (Grep, Glob):

| Payload | Before | After | Answer JSON |
|---|---:|---:|---:|
| Bash, piped `go test -v` of 800 tests | 43,013 | 3,816 | 4,120 |
| Bash, same output, plain `go test -v ./...` (the wrapper's) | 43,013 | unchanged | `{}` |
| Bash, already in the wrapper's form | 43,013 | unchanged | `{}` |
| Bash, 80 KB stdout log and 36 KB stderr, piped | 116,302 | 24,732 | 25,474 |
| Grep content mode, 2,000 matches in 100 files | 157,499 | 12,403 | 12,864 |
| Grep files mode, 1,500 paths | 89,999 | 12,378 | 12,924 |
| Glob, 1,500 paths | 89,999 | 12,378 | 12,930 |

Every replaced answer kept every sibling field of `tool_response` unchanged.
Each considered call wrote one `exec-events.jsonl` row with entry
`post_tool_use`, the hashed session, the family (`other` for an unknown
command, `search-tool` for Grep and Glob), raw and emitted bytes and the CCR id,
so `downshift report` counts it.

Hook time on one machine (10 calls each): a Bash call left alone 33 ms, the
43 KB piped log 62 ms, the 157 KB Grep 106 ms (store write and redaction
included). The off switch brings the piped call back to 33 ms.

## Result

On Claude Code the post-run path reaches what the wrapper cannot: the piped and
redirected forms of the same `go test` run reach the model at 12 KB instead of
592 KB, within 2 KB of the wrapper's own figure, and a 2,000-match Grep at under
13 KB. It does **not** replace the wrapper on Claude Code yet:

- the wrapper sees output before the harness caps it, and its family
  compressors are lossless without CCR; the post-run path cuts with loss and
  only with CCR on;
- the post-run hook reads up to 16 MiB, while the wrapper holds up to 2 MB and
  spools the rest;
- whether Claude Code puts the whole output in `tool_response` for a result it
  would itself persist to a file is not verified here.

So both run: the wrapper first, the post-run path for everything the wrapper
did not take. Revisit after a week of `report` rows with entry
`post_tool_use` (roadmap I1), and after a live capture confirms the Grep and
Glob shapes (they are not documented; the adapter only rewrites the fields the
result carries).

## Honor check

The post-run hook writes its exec row before it can know whether the harness
used the replaced output, so the figures on those rows are what was **offered**
to the model, not what was saved. `downshift doctor` closes the gap offline
where the harness keeps a transcript of what the model received.

- **Shared computation.** `postrun.ClassifyHonor` takes the latest compacted
  `post_tool_use` rows (kept CCR id, emitted bytes, time) and a
  `hookport.ReadToolResultsFunc`, and returns counts only: *honored* (a tool
  result in the transcript names the id in its retrieve hint and its text has
  the emitted length, or one byte less, because a transcript drops a trailing
  newline), *not found* (the read covered the row and no result names the id),
  *mismatch* (results name the id, none at that length) and *unverified*.
  The id alone proves nothing: the pre-run wrapper prints the same hint, so the
  length has to agree too.
- **Unverified, never failed.** A row is unverified when it has no kept id (no
  join key) or no emitted bytes, when its harness has no transcript reader
  (layering rule 5; see the harness matrix), when no transcript could be read,
  when it was logged before the read's coverage, and, when a cap or the
  deadline cut the read short, when its id is simply not found.
- **One result, one row.** Each transcript result honors at most one row, so two
  rows that kept the same output need two results. Results are told apart by
  their `tool_use_id`, so a resumed session that copies another transcript's
  history counts once. Rows are taken in log order; a row takes a result one
  byte shorter than it emitted first (the usual shape), then an exact one, so
  when lengths collide the earlier row wins and a later one reads as mismatch.
- **Port.** The reader is the optional `Port.ReadToolResults`. It returns
  samples (id, text length, how many results), whether it could read, where its
  coverage starts and whether it was cut short; never text. Claude Code's
  reads `~/.claude/projects/**/*.jsonl` with one pass per line: it finds the
  literal `downshift retrieve [--session=<ref> ]<id>` hint and tests the 12-hex
  id against the wanted set, so the cost does not grow with the number of ids
  and a result that merely contains the id elsewhere does not count. Bounds:
  the 60 newest files, the last 8 MiB of each, 64 MiB in all, and 3 s (checked
  between files). Coverage starts at the later of the oldest line read and the
  newest moment anything unread may hold (a skipped file's mtime, the first
  line read of a tail-cut file). It runs in memory, keeps no text, id or path,
  prints none, and any error means unverified.
- **Surface.** One `post-run replacement` row per harness in `doctor`, over the
  latest 200 rows, never pooled across harnesses. It warns only when its
  harness has rows, none honored, and some not found or mismatched; unverified
  rows alone are ok. `report` says its post-run rows are offered output.
- **Limits.** An output the harness spills to a file, an image block, or text
  that is not valid UTF-8 can read as a mismatch or not found though the model
  got it. This errs toward a warning only when no row of the harness is
  honored. On one machine a first check matched 9 of 11 rows.

## Not verified

- A live Claude Code session honouring `updatedToolOutput` from this hook,
  beyond the transcript join above. The shape follows the hooks reference; the
  end-to-end check here is the built binary's answer.
- The Grep and Glob output shapes beyond `content`, `filenames` and the counts:
  undocumented, read from the result as it arrives.
- Whether `tool_input.command` in `PostToolUse` is the command as written or as
  rewritten by the pre-run hook. Both are handled.
