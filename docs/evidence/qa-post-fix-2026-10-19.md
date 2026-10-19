# QA post-fix re-verification — 2026-10-19

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

Independent re-verification of `origin/main` at `e427099f` after the PRs
#189-#208 that followed the earlier QA review of `ac98599f` (verdict BLOCK).
Tests and evidence only. No production code or test was changed. Prompts,
account identifiers and tokens are not recorded.

**Verdict: CONDITIONAL.** Every code-level finding that could be re-tested is
closed. Three repository or CI conditions remain open (P1 below); none of them
is a defect in the shipped binary.

## Scope and honesty notes

- The text of the earlier BLOCK list was not available to this pass. "Earlier
  finding" below means the theme of each merged fix PR (#189-#208). A finding
  that was in the original list but is not named by any of those PRs would not
  be covered here.
- Tested commit: `e427099f` (worktree at that commit). `origin/main` has moved
  one docs-only commit since (`5b81844a`, #209); not re-tested.
- Measured vs not measured is stated per section. Nothing here measures token
  or dollar savings, routing accuracy or a live harness.

## Environment

- Apple M3 Pro, 11 logical CPUs, macOS 26.6.2 (Darwin 25.6.0), Go 1.27.1,
  arm64, Node 20.19.3, Python 3.
- Not a quiet machine: macOS background services (`softwareupdated`, `mmdc`)
  and other agent sessions were running. Load average was 4.5-9.2 during the
  test and benchmark runs, 4.5-6.2 during the latency runs. Single timings vary.
- Every run used an empty temporary `HOME` created under `/tmp/qa-evid`. The
  real `~/.downshift/context-optimization.json` has mtime `1792405447` and size
  `470` before the first command and after the last one: unchanged.
- After the full `go test ./...` with the isolated `HOME`, that `HOME` held zero
  files outside the Go toolchain's own cache and telemetry counters (checked
  with `find`). The suite does not leave files in `HOME`.
- Binaries built with `go build -o <tmp>/downshift ./cmd/downshift` (also
  `dsmon-hook`, `dsmon-server`).

## 1. Build, vet, unit tests, coverage

| Check | Command | Result |
|---|---|---|
| Build + vet | `go build ./... && go vet ./...` | PASS, no output |
| Tests | `HOME=<tmp> go test ./... -count=1 -cover` | PASS: 54 packages `ok`, 0 failures, 3 packages without tests |
| gofmt | CI step, `gofmt -l .` | not re-run locally (CI green on main) |

Packages with no tests (0.0%): `cmd/dsmon-server` (a 20-line `main`),
`internal/session`, `tools/multilang`.

Coverage versus the earlier baseline (statement coverage):

| Package | Earlier | Now |
|---|---|---|
| `cmd/dsmon` | 8.9% | 23.7% |
| `internal/classify` | 74.1% | 75.4% |
| `internal/nbtier` | 67.6% | 67.6% |
| `internal/core` | 67-100% | 88.8% |
| `cmd/downshift` | n/a | 77.5% |
| `cmd/dsmon-hook` | n/a | 82.3% |
| adapters | n/a | claudecode 90.1, antigravity 90.4, codex 87.6, cursor 84.3, kirocrew 74.3, opencode 94.8 |

Still low: `internal/paths` 31.7%, `internal/adapt` 38.7%, `cmd/dsmon` 23.7%,
`internal/server` 67.5%.

Flaky-test characterisation. `TestRunCommand_BackgroundChildDoesNotHang`
(`internal/hookctx`): `-count=5` on the loaded machine, 5/5 PASS (0.11-0.27 s);
`-count=10` with 12 busy-loop processes added, 10/10 PASS (0.12-0.21 s). No
flake reproduced (n = 15). Not proven stable on a slow CI runner.

## 2. Race detector

`HOME=<tmp> go test -race -count=1` on `internal/core`, `internal/classify`,
`internal/adapters/...`, `internal/telemetry`, `internal/server`,
`internal/hookctx`, `internal/sensor`, `internal/childend`, `cmd/downshift`,
`cmd/dsmon-hook`: all `ok`, no data race reported. Wall time 3 m 42 s
(`internal/core` alone 215 s).

Also `GOMAXPROCS=1 go test ./internal/classify ./internal/core`: both PASS
(51 s and 22 s), so the concurrent risk scan (#197) still passes its
equivalence tests on one CPU.

## 3. Fail-open black box, built binary, all six adapters

Harness: a throwaway Python `subprocess` script (not committed) against the built binary,
one fresh isolated `HOME`, state dir, event log and `DOWNSHIFT_CONTROL_SHARE=0`
per case, cwd outside the repo. Payloads: invalid JSON `{not json`, empty
stdin, `[]`, wrong types
(`{"tool_name":5,"tool_input":"x","toolCall":7,"v":"a","model":[1]}`), a 1 MiB +
10 byte prompt, a real rewrite payload (from `examples/` shapes plus
`session_models`), and a stdin that stays open and never delivers EOF.
Command shape: `<tmp>/downshift <harness> < payload`.

All 42 runs exited 0 except one, which is the designed behaviour (kirocrew is an
exit-code contract: a real downshift is a block with exit 2 and a message on
stderr). No run timed out; the never-closing stdin ended at 2.02-2.06 s for
every adapter (cap 2 s).

| Adapter | invalid / empty / `[]` / wrong types / 1 MiB+ | never-closing stdin | real rewrite |
|---|---|---|---|
| claude-code | exit 0, stdout `{}` (no `permissionDecision`) | exit 0, `{}`, 2.06 s | exit 0, `permissionDecision` allow + `updatedInput` (model `haiku`) |
| codex | exit 0, `{}` | exit 0, `{}`, 2.04 s | exit 0, allow + `updatedInput` |
| cursor | exit 0, `{"permission":"allow"}` | exit 0, same, 2.04 s | exit 0, `updated_input` + `agent_message` |
| antigravity | exit 0, `{"decision":"allow"}` | exit 0, same, 2.04 s | exit 0, `overwrite.Subagents[].Model` = `flash_lite` |
| kirocrew | exit 0, empty stdout | exit 0, empty, 2.05 s | exit 2 (block, policy contract) |
| opencode | exit 0, `{"v":1,"action":"allow","reasonCode":"fail_open"}` | exit 0, same, 2.03 s | exit 0, `action` route, model `deepseek-v4.1-flash-free` |

PR #192 contract (neutral `{}` for an unchanged spawn): PASS for claude-code
and codex, including a valid Task payload whose model is already the small tier
and a non-spawn `Bash` tool call (both `{}`).

Observation (not a failure): with a session allowlist that lists
`claude-haiku-5-5` instead of `claude-haiku-4-5`, the claude-code hook answers
`{}` (no rewrite) and `downshift try` prints "not rewritten: the session's model
list does not allow it" while still quoting "~98% cheaper". The cause was not
traced (the rewrite target is the alias `haiku`). See P2-4.

## 4. Security regression spot checks

| Fix | Test | Result |
|---|---|---|
| #191 XSS in dashboard | `node web/app.test.cjs` | PASS, 4/4 (includes "hostile event and agent fields render inert and chips use no inline handlers") |
| #191 bounded `agents.jsonl` input | `dsmon-hook` with `model` = `<img src=x onerror=alert(1)>` and `<script>alert(1)</script>` + 5000 `A`, `harness` = `<svg onload=1>` | PASS: exit 0; both records stored with `"model":"unknown"`, no harness; file mode `0600` |
| #191 CSP, no inline script | `web/index.html` | PASS: meta CSP `script-src 'self'`; only `boot.js` and `app.js` are loaded; no `onclick`/inline `<script>` |
| #191 loopback-only server | `internal/server` tests (host/origin check `localOnly`) | PASS in suite; not exercised live (port 7474 is held by the owner's own server, which was not touched) |
| #192 symlinked DCI log | repo `.downshift/dci-events.jsonl` symlink to a 644 file, `downshift dci hook claude-code`; and `DOWNSHIFT_DCI_EVENT_LOG` pointing at a symlink | PASS: victim file content, size (7) and mode (644) unchanged in both; hook exit 0 |
| #191 catalog override cap | `~/.downshift/catalog.json` = 5.2 MB valid-shaped JSON, also a symlink to a 5 MB file | PASS: hook exit 0, stderr "larger than 4194304 bytes", embedded catalog used, rewrite still produced |
| malformed override | truncated JSON, `[]`, 4 KB random bytes, directory, FIFO | PASS: each exit 0 in 0.01-0.02 s, no panic or goroutine trace, warning on stderr, rewrite still produced |
| #201 catalog load under recover | same cases as above | PASS (black box only; the panic-injection unit test is in the suite) |

## 5. Performance (macOS M3 Pro, loaded machine, isolated `HOME`)

Cold hook latency: Python `perf_counter` around a spawn of the built binary, one
Claude Code `PreToolUse` Task payload that produces a real rewrite on stdin,
10 warm-up runs, then n = 200 alternating with `downshift version` (no stdin).

| Case | n | p50 | p95 | p99 | max |
|---|---|---|---|---|---|
| `downshift claude-code` Task rewrite | 200 | 9.1 ms | 10.8 ms | 12.5 ms | 15.4 ms |
| `downshift version` (floor of this loop) | 200 | 7.4 ms | 8.8 ms | 11.4 ms | 16.0 ms |

Earlier: p50 14.7 ms before the fsync removal (#198), 9.9 ms after; the perf
note of the same day records 8.5 ms. Now 9.1 ms: the removal holds. A no-op Go
binary floor was not re-measured here.

Benchmarks, `go test -run '^$' -bench . -benchmem`:

| Benchmark | Earlier | Now |
|---|---|---|
| `RiskScanNoMatch1M` (11 CPUs) | 753 ms before, ~238 ms after #197 | 278-288 ms (`-benchtime=5x -count=3`); 354 ms in the first, noisier pass |
| `RiskScanNoMatch1M` (`GOMAXPROCS=1`) | not stated | 758-858 ms (no speed-up, no regression versus 753 ms) |
| `RiskScanShort` | 43 µs | 45 µs |
| `RiskScanBenign1M` / `Worst1M` / `Adversarial1M` | 39 / 34 / 15 ms | 46 / 37 / 18 ms |
| `core.ClassifyTypical` | 313 µs | 316 µs |
| `codeintel` Unindexed / Indexed / Packager | 502 / 107 µs / 1.1 µs | 564 / 155 µs / 1.3 µs |
| `telemetry.AppendTo` | 64 µs after fsync removal | 60 µs |

`NoMatch1M` is about 17-20% slower than the 238 ms quoted. I attribute it to
machine load (the benchmark is CPU-parallel), but did not re-run on a quiet
machine, so that is unproven. The one-CPU figure confirms the #197 gain only
exists with more than one CPU.

`downshift dci benchmark` (cwd inside the repo): cold index 540 ms (earlier
582 ms), warm p95 5.35 ms (earlier 5.2 ms), all five query checks PASS. The
"Warm Sync Latency 167 ns" line looks like a no-op or cache hit; do not quote it.
`downshift context benchmark`: bytes only (`go test all-ok` 75 -> 30), tokens
and cost "not measured" as designed.

## 6. CI

| Item | Finding |
|---|---|
| `ci.yml` on main | last 13 push runs: 13 success. Required check `test` ran on `e427099f`: success, 1 m 36 s |
| Branch protection (`main`) | required check `test`, strict (up to date), conversation resolution, enforce admins: on. **Force pushes: allowed.** Deletions: not allowed. Signed commits: not required |
| `-race` | not in `ci.yml` (`go test ./...`). Runs only in `ci-full.yml` (cron weekdays 09:00 UTC + manual); not a required check |
| `ci-full.yml` history | 8 scheduled runs since 2026-10-07: 3 success, 5 failure. Failures seen: `internal/core` hit the 10 min `go test` timeout under `-race` (2026-10-16); `TestClassifyLatencyLargePrompts` / `...AdversarialInput` failed under `-race` (2026-10-12); other failures not classified (logs show only stderr noise). One manual run on 2026-10-19 (`b262c04a`) passed in 6 min. No scheduled run since 2026-10-16, including none for the 2026-10-19 09:00 UTC slot at the time of checking. n = 1 post-fix pass is not evidence of stability |
| ci-classify allowlist (#196) | wired: the `changes` job runs `bash scripts/test-ci-classify-changes.sh` before classifying. Run locally: "all cases passed". Allowlist is images under `docs/` that still exist; every other path runs Go |
| Actions pinned by tag, not SHA | `actions/setup-node@v7` (`ci.yml`), `actions/setup-python@v7` and `astral-sh/setup-uv@v7` (`ci.yml`, `ci-full.yml`), `actions/upload-artifact@v7` (`ci-full.yml`). Checkout, setup-go, goreleaser, attest-build-provenance are SHA-pinned; goreleaser is pinned to `v2.18.3` |
| `permissions` | top-level `contents: read` in all three workflows; write scopes only on the `release` job (`contents`, `id-token`, `attestations`) and the `sync-prices` job (`contents`, `pull-requests`, `actions`). No workflow lacks a block |
| Release provenance (#204) | SHA pins and attestation step present in `ci.yml`; **not exercised**: the last tag (`v0.1.0-beta.14`) predates it and the `release` job was `skipped` on main |
| `sync-prices.yml` (#190) | has never run (merged today; first schedule is the next Monday 06:17 UTC). Not exercised end to end |
| Open PRs | #207 (docs) only |

## 7. Status of each earlier finding

| Finding (PR) | Status | Evidence |
|---|---|---|
| One savings computation for stats, dashboard, dsmon (#189) | PASS | `stats`, `dsmon` and the server call `telemetry.Summarize`; parity tests (`summary_parity_test.go`, `summary_test.go`, `compute_test.go`) pass |
| Stored XSS, unbounded hook and catalog input (#191) | PASS | section 4 |
| Hook overrides ask rules; telemetry symlink (#192) | PASS for claude-code and codex; see P1-2 for cursor and antigravity | sections 3 and 4 |
| Older Claude prices, weekly sync (#190) | PARTIAL | catalog tests pass; the sync workflow has never run; script gaps in P2-1. Prices were not compared with vendor pages (no network fetch done here) |
| Go tests skipped for non-docs paths; test isolation (#196, #205) | PASS | classifier test passes; suite leaves nothing in `HOME` and the real state file is untouched |
| Per-spawn fsync (#198) | PASS | hook p50 9.1 ms, p95 10.8 ms, n = 200; `AppendTo` 60 µs |
| Large-prompt risk scan (#197) | PASS with a caveat | 278-288 ms on 11 CPUs under load; 758-858 ms on one CPU; equivalence tests pass with `GOMAXPROCS=1` and `-race` |
| Catalog load can deny a spawn (#201) | PASS | section 4, all hostile overrides fail open |
| "You should know" default (#199) | PASS | `setup --dry-run` default leaves `enabledPlugins` empty and prints "You should know is off"; `--you-should-know` adds `cc-plugin-you-should-know@builtin` |
| Risk vocabulary (#208) | PARTIAL | `chmod 777`, `cors`, cookie, path traversal and ssh/api token prompts are flagged "touches a risk area"; the raw header `Access-Control-Allow-Origin: *` is not (P2-3) |
| Dead code (#202) | PASS | no remaining reference to `antigravitybridge`; build and vet clean |
| Release hardening (#204) | NOT VERIFIED at runtime | static only |
| Dock mod (#195) | NOT VERIFIED | its `register.test.ts` needs `claude plugin test`; not run here and not in CI (P2-2) |

## 8. Open gaps, ranked

### P0
None found.

### P1

1. **Force pushes are allowed on `main`.** `allow_force_pushes` is true while
   the repository relies on the PR plus `test` gate. Owner action (repository
   setting); not changed by this pass.
2. **Explicit allow remains on unchanged spawns for cursor
   (`{"permission":"allow"}`) and antigravity (`{"decision":"allow"}`).** This
   is the class of problem #192 closed for claude-code and codex. Whether those
   two harnesses treat it as overriding a user prompt rule was not verified
   here, so this is a hypothesis to check against their hook documentation, not
   a confirmed defect. Also, a real claude-code/codex rewrite still returns
   `permissionDecision: allow` with `updatedInput`; that is needed by the wire
   today, but it means a rewritten spawn skips the user's ask rule.
3. **No reliable race gate.** `-race` is not in the required `test` check;
   `ci-full` failed 5 of 8 scheduled runs, has not run on schedule since
   2026-10-16, and `internal/core` takes 215 s under `-race` locally against a
   10 min timeout that already failed once. Concurrency changes (#197) are
   covered only by the manual run.

### P2

1. `scripts/sync_catalog_prices.py` has no test of its own. Verified on a
   copy with `--dry-run`/`--snapshot`: empty, malformed, missing and unpriced
   snapshots exit 1 and leave the catalog alone (good); a non-numeric price
   (`"abc"`) and `NaN` end in an unhandled traceback (exit 1, catalog
   untouched); a negative price (`-0.000004`) and a 1000x price are accepted
   and written. In both bad cases the follow-up `go test ./internal/catalog`
   failed, but only because a cache-write consistency check happened to
   disagree, not because the script validates ranges. The PR still needs a
   human review of the numbers.
2. `mods/downshift-dock` (and the other mods) are tested with
   `claude plugin test`, which `ci.yml` does not run; the dock's tests assert
   its own contract only (per its own header).
3. Raw header `Access-Control-Allow-Origin: *` without the word "cors" is not
   a risk term; the classifier then reports "no risk" and may downshift.
4. `try` quotes "~98% cheaper" for `claude-haiku-5-5` ($0.10 input per 1M)
   while the hook cannot rewrite to it under an allowlist that only lists that
   id. Catalog price and reachability for that id were not reconciled.
5. Four actions are pinned by mutable tag (list in section 6). They run with
   `contents: read` only, and Dependabot is on a weekly schedule; the release
   job is SHA-pinned.
6. Dashboard CSP is a `<meta>` tag only; `frame-ancestors` is ignored in that
   form, so there is no framing protection from the page (the server's
   loopback host/origin check remains).
7. Coverage: `cmd/dsmon` 23.7%, `internal/paths` 31.7%, `internal/adapt`
   38.7%, three packages with no tests.
8. Four recent commits carry
   `Co-authored-by: Tiago Vilas Boas <...@MacBook-Pro-de-Tiago.local>`, a
   machine-local address in public history. Not an AI attribution; a hygiene
   item for the commit tooling.

## Not measured

Token or dollar savings, routing accuracy, live harness behaviour (no Cursor,
Codex, Claude Code or Antigravity session was driven), Linux or Windows,
`claude plugin test`, the dashboard in a real browser, the `sync-prices` and
release workflows end to end, vendor price accuracy, and a quiet-machine rerun
of the benchmarks.
