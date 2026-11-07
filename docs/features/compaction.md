# Output Compaction

> **Status: stable.**

Downshift's compactor reduces verbose command output to essential information,
preserving exit codes and failure details while dramatically cutting token usage.

## How It Works

When you wrap a command with `downshift compactor`:

```bash
downshift compactor -- go test ./...
```

Downshift:

1. **Runs the command** with no shell, capturing stdout/stderr
2. **Checks the exit code and stderr** — compacts a clean run (exit 0) and a failed `go test`, `vitest`, `playwright test`, `jest`, `pytest` or `cargo test` run, which keep every failure line (see [Failing runs](#failing-runs)). Any stderr byte keeps the run raw, except for the families whose report or notes are on stderr (Jest, Vitest, go test, git commit, git push): their stderr is read with the report, and a line the compressor does not know still keeps the run raw. A failed run that wrote to stderr is raw, except a failed Vitest run, whose failure report is on stderr
3. **Identifies the format** — matches output against known compressor patterns
4. **Compacts if recognized** — reduces to PASS/FAIL summary
5. **Preserves if not** — unknown formats pass through unchanged

## Compression Ratios

| Command | Typical Output | Compacted | Ratio |
|---------|---------------|-----------|-------|
| `go test` (50 packages, all pass) | ~2,000 tokens | ~50 tokens | 97% |
| `git status` (clean) | ~200 tokens | ~20 tokens | 90% |
| `cargo build` (no warnings) | ~1,500 tokens | ~100 tokens | 93% |
| `npm test` (all pass) | ~1,000 tokens | ~80 tokens | 92% |
| `pytest` (100 tests pass) | ~800 tokens | ~60 tokens | 92% |

## What Gets Preserved

Compaction **never hides problems**:

- ❌ Failures → full output preserved
- ❌ Warnings → full output preserved  
- ❌ Stack traces → full output preserved
- ❌ Non-zero exit codes → output preserved, except a failed `go test`, `vitest`, `playwright test`, `jest`, `pytest` or `cargo test` run, which loses only the lines of tests that passed
- ✅ Clean success → compacted summary

## Failing runs

A run that exits non-zero is compacted for six tools. In each, every failure
line is kept byte for byte and in order: the FAIL blocks, assertions, expected
and received values, diffs, code frames, stack frames and the summary. Only
the lines of tests that passed are cut, and one line at the top says how many:

| Tool | What is cut | Summary line |
|------|-------------|--------------|
| `go test` | `ok` package lines, `--- PASS` lines and the `=== RUN` headers of passing tests | `Go test: ... (failures kept verbatim)` |
| `vitest` 3, 4 and 5 | files marked `✓`, and passed tests and suites in the test tree of a file that failed | `Vitest: N passing files, M passing test lines cut (failures kept verbatim)` |
| `playwright test` (list reporter) | `✓` lines of tests that passed at their first attempt | `Playwright: N passing tests cut (failures kept verbatim)` |
| `jest` (also direct `jest`, whose report is on stderr) | passing files and passing test lines | `Jest: N passing files, M passing test lines cut (failures kept verbatim)` |
| `pytest` | lines of tests that passed | one line at the top with the count cut |
| `cargo test` | the whole block of a crate that passed, and the passing `test ... ok` lines of a crate that failed | one line at the top with the count cut |

For Vitest everything from the first `Failed Tests`, `Failed Suites` or
`Unhandled Errors` heading on is kept untouched. Before it, failed, skipped
and todo tests keep their line, and so do the `stdout` and `stderr` blocks a
test printed and the Node notices. For Playwright everything from the first
numbered failure block on is kept untouched. Before it, failed tests, skipped
tests and tests that passed only on a retry (flaky) keep their line.

The run stays raw, with the program's own exit code, when:

- a line before the failure section is not one the tool's reporter prints. For Playwright this includes what a test printed with `console.log`, which appears between the test lines;
- the failure section is not whole: no failure heading or block, no summary, a Vitest section without its last `[N/N]` footer, Playwright blocks out of order, or a Playwright summary whose failed and flaky counts differ from the number of blocks;
- the transcript holds more than one run header;
- the output has a NUL byte, invalid UTF-8, CRLF, no final newline, or an escape sequence other than colour (colour is removed before parsing, so the compacted text is plain);
- the output is another reporter's (`--reporter=verbose`, `dot`, `json`) or exceeds 2 MB;
- there is nothing to cut, or the result would not be shorter.

A failed Playwright run that wrote to stderr is raw, since any stderr byte
releases the held output. The compacted text goes to stdout and ends with the
`[downshift compact: ...]` marker, as for a clean run.

The Jest, pytest and cargo grammars follow the same contract: a line the grammar does not know keeps the run raw. `tsc` is not compacted when it fails, since a failing `tsc` prints only errors.

The saving depends on how many tests passed. On the real transcripts used as
test fixtures (vitest 3.2.7, 4.1.12 and 5.0.5, `@playwright/test` 1.64.1; 10 to
15 tests, 4 failing) the output is 3% to 6% smaller, because almost all the
bytes are the failures themselves. The Vitest transcripts of a file that cannot
be imported cut two lines, about 0.1%.

## Generic compaction

Output of no known shape from a test runner, build tool, linter, package script
or `make` (not `git` or `gh`) goes through a generic chain when the run exits 0.
The tool families are listed in `genericFamilies`. The steps run in this order:

1. Base64 runs of 1 KB or more become `[base64 image/png, 50 KB elided]`. Hex-only runs are untouched.
2. Danger gate: any danger word, stack frame or failure marker anywhere keeps the run raw, byte for byte. The one exception is the frames of a foldable stack run (step 2b).
2b. Stack fold: a run of more than 12 contiguous frames of one language (JS/Node `at fn (file:L:C)`, Java `at pkg.Class.m(File.java:N)`, Python `File "x.py", line N` plus its code line, Go `pkg.fn(...)` plus `\tfile.go:N +0x..`) keeps its first 5 and last 5 frames and becomes `[N frames elided]` in the middle. Every line outside the run, including the message above it, still goes through the gate.
3. Three or more identical adjacent non-blank lines become `line [xN]`.
4. Output longer than 252 lines keeps the first 50 lines, `[N lines elided]` and the last 200.

Limits of the stack fold: a trace whose message says `Error: boom`, `Exception`, `panic:`,
`Traceback` or `stack` stays raw, so the fold only helps frames-only noise (a logger printing its
caller chain, a script echoing frames after a neutral message). It is rare by design. A short run
(12 frames or fewer), a run mixing languages and an unrecognised `at x:N` line are not folded.
JS/Node frames end in `file:line:col`, which the format detector reads as search results first, so
that shape reaches the generic chain only when the output is not otherwise detected as search.

The run stays raw when the exit code is not 0, when the danger gate fires, or
when no step made the output shorter. The saving is counted in the bytes
account, like the rest of the compactor.

## Lockfile diffs

`git diff` of lockfiles only (`package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`,
`Cargo.lock`) on a run that exits 0 becomes one block per file: counts of
packages added, removed and changed, and up to 100 package lines
(`+ name version`, `- name version`, `~ name old -> new`). The hunks, hashes and
resolved URLs are cut; run `git diff -- <file>` for them.

The diff goes through the ordinary git diff compressor instead (every change
line kept) when a file is not a lockfile, a line is not diff or hunk syntax, a
line outside the package lines carries a danger word, a `deprecat*` notice or a
secret (a registry URL with a token), or no package version moved.

## Git log

A default-format `git log` that exits 0 becomes one line per commit: the 7-character
hash, the decorations if any (`(HEAD -> main, tag: v1)`) and the subject verbatim.
Author, date, merge parents and message bodies are cut (lossy, by owner decision:
the body is dropped even when it mentions `fail` or `panic`; the subject is always
kept). The log is recognised before the test-runner detectors, so a body that
quotes test output does not keep it raw.

Any line outside the default shape (a `--stat` block, a patch, `gpg:` signature
lines, a missing `Author:`) returns the whole input raw with reason
`git_log_unrecognized_structure_preserved`. `--oneline` output is not a default-format
log and is not touched. A non-zero exit stays raw.

## Git diff and git show

A unified diff drops the `index` and redundant header lines and trims unchanged
context to two lines around each change; every `+`/`-` line, hunk header and file
boundary is kept, and the compressor verifies it (`changeLinesPreserved`).

The danger gate looks at every line except unchanged context lines. A danger word
(`error`, `fail`, `panic`, `warning`, ...) on a `+`/`-` line, a file or hunk
header, a commit message or a stray non-diff line (for example `warning:`) keeps
the whole input raw. A danger word on a context line is source code that did not
change, so it no longer blocks compaction. A context line is a line with a single
leading space inside a hunk, counted from the hunk header's line counts. A secret
anywhere (`token`, `password`, a private key, ...) still keeps the input raw.

## Progress frames

Output with carriage-return progress frames (`frame\rframe\rframe\n`) was always
kept raw. It now collapses each line to its last frame, but only when a terminal
would show exactly that text. The run stays raw, byte for byte, unless all hold:

- only printable ASCII, `\n` and `\r` (SGR colour is removed first; any other escape, a tab or a non-ASCII character keeps it raw);
- no CRLF line ending;
- every frame is at least as long as the one before it (a shorter frame leaves the tail of the previous one on screen, so the final frame alone would lose text);
- no danger word, stack frame or failure marker in any frame, including the ones cut;
- exit code 0 or unknown, and the result is shorter.

Families: the generic-chain families and `npm install`. Flutter and Dart keep their own exact validation of the `\r` status line.

## Bazel

`bazel build` and `bazel test` write their report on stderr. A run that exits 0
compacts to `[x] bazel: build completed successfully, N total actions in Ts`,
the `Target ... up-to-date:` lines with their output paths, and the
`Executed N out of M tests` line. Cut: the server start line, the analysis,
`Found` and process counters, the elapsed-time line and the PASSED rows.
The run stays raw, byte for byte, on a `WARNING`, `ERROR` or `DEBUG` line, a
FAILED or flaky row, a progress line, a danger word or any line not named here.
Shape taken from the Bazel user manual; not checked against a live Bazel.

## Turborepo

A `turbo run` that exits 0 compacts to `[x] turbo: N successful, N total (C cached) in T`
when every line is a header bullet (`• ...`), a cache notice (`pkg:task: cache miss, executing <hash>`,
`cache hit, replaying logs <hash>`), an empty prefixed line, a script echo (`pkg:task: > ...`) or the
Tasks/Cached/Time summary, and every task succeeded. Any other prefixed line is the task's own output,
so the run stays raw, as does a warning, an error, a stack frame, a `Failed:` line or a danger word.
Shape taken from the Turborepo docs; not checked against a live turbo.

## Nx

An `nx` run that exits 0 compacts to `[x] nx: ran target T for project P in Ns` (or `for N projects`)
when every line is blank, a rule line, a command echo (`> ...`), an `NX` banner or the cache notice.
Any other line is the task's own output, so the run stays raw, as does a warning, an error, a stack
frame or a danger word. Shape taken from the Nx docs; not checked against a live nx.

## Deno and Zig

`deno test`, `deno check`, `deno lint`, `zig build` and `zig test` are allowlisted families that go
through the generic chain only (repeated lines folded, a long output capped, base64 elided). Any
danger word keeps the run raw, and so does a non-zero exit. That includes "0 failed" in the deno test
summary, so a short green `deno test` stays raw; only a long log (over about 250 lines) or repeated
lines shrink. These families run the project's own test code, the same trust as `go test` and
`npm test`.

Refused (never run through `compact`): `deno run|install|task|compile|eval|upgrade`, `deno ... --watch`,
`deno lint --fix`, any deno argument naming a remote module (`://`), `zig run`, a `zig build` step whose
name starts with `run`, `--watch` and `--test-cmd*`.

The fixtures are hand-written: deno and zig were not installed where this was built, so the output
shapes are not checked against a live tool. Only the generic rules apply, so no tool-specific shape
is assumed. jupyter is a deliberate non-goal: no allowlisted command prints notebook JSON, and
stripping an `.ipynb` is a file transform, not command output.

## Infrastructure and cloud (not compacted)

No compressor exists for pulumi, cdk, ansible, helm, aws, gcloud or az. pulumi, cdk and ansible
commands are not allowlisted. helm `list`/`status`, `aws` describe/list/get, `gcloud` read verbs and
`az` list/show are allowlisted but pass through: no fixture or doc here confirms the helm table, and
the cloud CLIs print JSON, table or text chosen by `--output`, with no single shape to compact. A
shape is added only after a captured fixture proves it.

## Observability and debugging (not compacted)

No compressor exists for `sentry-cli`, `datadog-ci`, `aws logs` or stack traces (#815, Phase 6).
The corpus fixtures for `sentry-cli` and `datadog-ci` are one-line placeholders, and no doc in this
repository confirms the `sentry-cli issues|projects|releases list` table. `datadog-ci` is allowlisted
for `version` only; every other subcommand uploads or tags. `aws logs` (`tail`, `filter-log-events`,
`get-log-events`) prints JSON, table or text by `--output`, and folding log messages would drop the
data being asked for. Stack traces have only the narrow fold described under
[Generic compaction](#generic-compaction). A shape is added only after a captured fixture proves it.

## npm audit

`npm audit` exits non-zero when it finds a vulnerability at or above
`--audit-level`, and a non-zero run stays raw. A run that exits 0 with findings
below that level (for example `--audit-level=critical`) lists the low and
moderate packages by name and count and cuts their advisories. Every block of a
high or critical package, and the summary and `npm audit fix` lines, stay
verbatim. A line the report grammar does not know (an `npm warn`, an error,
a field npm added) keeps the whole report raw.

## Package installs

`npm`, `pnpm`, `yarn` and `bun` install, add and ci runs that exit 0 become one
summary line (`[x] pnpm: +40 packages in 3.2s`). Each line must be one the
compressor names: the summary lines, funding notice, progress, the `+ name
version` list. Audit findings the install printed (`3 vulnerabilities (...)`,
`To address ...`) are kept verbatim after the summary. Any other line keeps the
run raw, byte for byte: every `npm warn`, `npm error`, `WARN`, `warning`,
`error`, `info` incompatibility notice, deprecation, stack frame, and the
`changed`/`removed` forms of the npm summary.

## Listings and search

`ls`, `find`, `grep`, `rg` and `git grep` are compacted only in the plain forms listed in
[harness-matrix.md](../reference/harness-matrix.md). A form with any other flag is
refused, and a non-zero exit stays raw. `find` and `grep` are lossless: every
path and every match survives. `ls` is lossy and runs the danger gate.

`rg -n PATTERN PATH...`, `git grep -n PATTERN` and `rg --files` use the grep and
find compressors above (match rows grouped by file, and a path tree). `rg` needs
`-n` and a path, since without a path it reads standard input. `-r`, `--pre`, `-z`,
context, counts, file lists, `-o`, `-v`, `--json` and other flags are refused. The
exact forms are in [harness-matrix.md](../reference/harness-matrix.md).

### ls: names only

`ls -la internal` becomes one name per line, a directory ending in `/`. Dropped:
the `total` line, the permission, owner, size and date columns, and the `.` and
`..` rows. `ls -p` already marks directories, so its slash is kept as printed.
Because a dropped column may hide a warning, any danger word in the listing
(for example `errors.go`) keeps the output raw. So do several operands (a name
header per directory), a row the long-form grammar does not read, a blank line,
and an empty directory.

### find: path tree

Paths become an indented tree: one component per line, one space per level. A
directory that `find` did not print ends in `/`, so the names without it are
exactly the input paths. Nothing is dropped and no danger gate runs. The output
stays raw for a duplicate path, a double slash, a blank line, a component that
starts with a space, and when nothing is gained.

The acting primaries (`-exec`, `-execdir`, `-ok`, `-okdir`, `-delete`,
`-fprint`, `-fprint0`, `-fprintf`, `-fls`) are refused and never run through
`compact`. So are `-ls`, `-printf`, `-print0` and `-X`, which change the output
from a list of paths.

### grep: matches by file

Matches are grouped under their file, in grep's order. Each match keeps its line
number and its text; only its leading whitespace is dropped. Nothing is dropped
and no danger gate runs. A form needs `-n`, and a file name on every row (`-r`,
`-H`, or two paths). Context (`-A`, `-B`, `-C`), counts (`-c`), file lists
(`-l`, `-L`), `-o`, `-q`, `-v`, `-h`, byte offsets, NUL output and long options
are refused. A context row, a separator or a binary-file notice keeps the output
raw.

Sizes at HEAD (bytes, marker included): `ls -la internal` 2836 to 429,
`find internal -name '*.go'` 31819 to 17440, `grep -rn func internal/compressor`
55145 to 39973. The lossy `ls` is the smallest; `find` and `grep` keep every fact.

## Markdown bodies

`gh issue view` and `gh release view` (exit 0) print a header of `key:\tvalue`
lines, a `--` line and a markdown body. The header stays untouched. In the body
only what is provably redundant goes: runs of blank lines (not next to indented
code), whole-line `<!-- -->` comments, lines made only of shields.io, badgen,
badge.fury or codecov badges, and a heading repeated right under itself. Code
fences, tables, indented code and every other link stay verbatim. Prose is
conservative: any danger word (`error`, `fail`, `warning`, ...), stack frame,
header line outside `key:\tvalue` or unclosed fence keeps the whole run raw.
`gh pr view` keeps its own summary.

Not built from the same roadmap phase: web-md (HTML to markdown) and toon (TOON
encoding of JSON). `curl` and `wget` are not allowlisted (guarded programs),
and `gh api` returns JSON that callers parse, so no allowlisted command emits
either input.

## Relative paths

On a compacted result, an absolute path under the working directory loses that
prefix (`/Users/x/repo/src/a.ts:3` becomes `src/a.ts:3`), in POSIX and Windows
forms. A raw answer keeps its bytes, and so do `git diff` and `git show`. Inside
a compacted result, a line with a danger word or a secret-looking word stays as
printed. Durations are not stripped.

## Stack families

Families added for CSS and HTML (`stylelint`, `htmlhint`, `html-validate`,
`tailwindcss`, `sass`, `postcss`), JavaScript and TypeScript (`next lint`, the
`lint`, `typecheck`, `type-check` and `check` package scripts, `knip`, `depcheck`,
`madge --circular`), Python (`flake8`, `pylint`, `python -m pytest|unittest|mypy|flake8|pylint`,
`tox`, `nox`, `coverage`), Django (`manage.py test|check|showmigrations` and
read-only forms), React Native (`detox`, `xcodebuild`, `pod`, `expo`, `adb`)
go through the generic chain. Forms that never exit or that act (watch, fix, init,
inspect, archive, deploy) are refused. Each accepted form is listed in
[harness-matrix.md](../reference/harness-matrix.md).

## CCR: reversible compaction (on by default)

Unless `"ccr": false` is set in the context config or `DOWNSHIFT_CCR=0` is in the
environment (which wins), `downshift compact` stores the raw output under `~/.downshift/ccr/<id>.txt`
and its marker names the id: `full output: downshift retrieve <id>`. Runs of 8 or
more same-shape lines then fold, because every cut line can be read back with
`downshift retrieve <id> [--grep RE] [--head N] [--tail N] [--lines A:B]` or the
MCP tool `retrieve`. On the bench corpus the total saving goes from 48.9% to 76.1%.
Bytes later retrieved are not yet subtracted from the savings figures. On by
default; limits, privacy and retention are in [ccr.md](../guides/ccr.md).

## Benchmark

`make bench` writes [bench.md](../reference/bench.md): raw against compacted bytes
per family on the test corpus, with tokens approximated as bytes/4.
`internal/compressor/testdata/bench/baseline.tsv` holds the committed per-family
baseline, and `TestBenchGuard` fails `go test ./internal/compressor` when a family's
saving drops past its tolerance. `make bench-baseline` rewrites the table; see
[bench-baseline.md](../contrib/bench-baseline.md) for when that is allowed.

## Supported Tools

See [Supported Tools](../reference/supported-tools.md) for the complete list of
70+ supported command families.

## Configuration

### Disable Compaction

```bash
# Environment variable
export DOWNSHIFT_EXEC=0

# Or in config (~/.downshift/config.json)
{
  "exec": false
}
```

### Per-Command Disable

The compactor refuses non-allowlisted commands, so you can always run directly:

```bash
# This runs through compactor
downshift compactor -- go test ./...

# This runs directly (no compaction)
go test ./...
```

## Claude Code Integration

On Claude Code, `downshift setup` installs a plugin that automatically rewrites
allowlisted commands to use the compactor. No manual wrapping needed.

The plugin only rewrites commands that:
- Match the allowlist
- Have no shell metacharacters (pipes, redirects, globs)
- Are not already denied by your permission rules

## Metrics

Track your compaction savings:

```bash
# Current session
downshift session

# Historical
downshift stats
downshift report --period=7d
```

## Technical Details

- **No shell execution** — commands run directly via `exec`, not through a shell
- **Exit code preservation** — the original exit code is always returned
- **Stderr passthrough** — raw output keeps stderr byte for byte on its own stream; only a compacted run of a family that reads stderr with its report (Jest, Vitest, go test, git commit, git push) prints its kept stderr lines inside the summary, and a failed Vitest run prints its failure report, read from stderr, on stdout
- **Format detection** — each compressor has a regex pattern for its format
- **Fail-open** — unrecognized formats pass through unchanged
