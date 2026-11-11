# Downshift compactor: wrap allowlisted commands

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift-ai

This harness has no shell-rewrite hook, so compaction is not automatic. When
you run an allowlisted command yourself, invoke it through the compactor:

```
downshift compactor -- <command> [args...]
```

`downshift` resolves from PATH. The compactor cuts clean, verbose output to its
PASS/FAIL summary and keeps the real exit code, so a red run stays red.

## Large outputs

An output larger than 16 KB is cut to what you act on first: the head, the
tail and the failure lines of a log; every file and hunk header of a diff;
the first matches of each file of a search. The full output is stored, and the
last line names it:

```
[downshift compact: 591441 to 33342 bytes; downshift retrieve --session=4f2a9c01b3d7 85298d5a7eb4]
```

Run the command the line shows, as printed (`--session=...` included when it
is there), when you need a line that was cut. The id is its last word. A line
ending `original not kept` means no copy exists: the cut is final, rerun the
command to see more. Set
`DOWNSHIFT_BALANCED=0` to keep large outputs whole.

## Allowlisted families

Allowlisted families: go test, go build, git status, git diff, npm test, jest, vitest, pytest, phpunit, cargo test, make, tsc, golangci-lint run, playwright test, gh pr, gh run, ls, find, grep -rn x .

Any other command is refused (exit 126). Run it without `downshift compactor`.

## Rules

- Wrap only a single command. Do not put pipes, `&&`, `;`, redirections or
  subshells inside the wrapped command; run those plainly.
- Do not use the compactor to hide a failure. Read the summary and the exit code.
