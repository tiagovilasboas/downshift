# Value roadmap: compaction where the tokens are, proof, and cheaper workers

Owner direction, 2026-11-06. Downshift by Tiago Vilas Boas\
https://github.com/tiagovilasboas/downshift

## Why

Downshift's unique part is **per-subagent model routing**: the orchestrator picks
the model of each spawn and Downshift applies it within the declared policy. No
comparable tool does this. Its compactor, though, saves little in real use. On the
maintainer's machine, from 2026-11-02 to 2026-11-06 (binary 0.7.0):

| Measure | Value |
|---|---|
| Commands that ran through the compactor | 2,584 |
| Of those, compacted | 272 (10.5%) |
| Bytes saved | 4.0% (12.15 MB to 11.66 MB) |
| Output in a shape no compressor recognised (`unrecognized_output`) | 1,120 runs, 7.3 MB, **60% of all bytes** |
| Biggest unrecognised: `go test` (9 runs) / `npm test` / `git push` | 2.7 MB / 1.9 MB / 1.3 MB |
| Failed runs passed through raw (`nonzero_exit`) | 307 runs, 2.3 MB (`make` alone 1.3 MB) |
| Commands the hook skipped | 7,769: metacharacter 5,191 (67%), too long 1,745, already wrapped 533, not allowlisted 267 |

Part of this is fixed on main and not in 0.7.0 (quoted words, `git commit` with
hooks). The rest is this roadmap. The 533 "already wrapped" came from a global
rule that told Claude Code to call the compactor directly; that rule is gone
(harness-hub #97), so those calls now go through the hook and are counted.

Two references set the bar:

- **RTK** (github.com/rtk-ai/rtk, Apache-2.0 CLI): a rewrite hook for 18 agents and
  100+ commands. It reads files with filter levels (`rtk read`, `rtk smart`),
  groups `grep`/`rg` by file, keeps the full output of a failure for `rtk recall`,
  and `rtk discover` scans past sessions for missed savings. It does not route
  models.
- **Alethe** (github.com/Kc1t/alethe-orchestrate-skill, MIT): no compaction. It
  saves by delegating to cheaper CLI workers (Codex, Kimi, any CLI) in waves, with
  a playbook catalogue, a quota stop line before each wave, per-wave budgets and a
  per-worker report where the lead reads only the result.

Neither tool touches the agent's own Read, Grep and Glob tools, where most file
content enters the context.

## Phases

Points are story points (1, 2, 3, 5, 8). Each task is one PR that passes `make
check` on its own. A task that touches the binary fills
[coverage.md](../contrib/coverage.md) first.

### A. Compaction where the bytes are

| ID | Task | Pts | Evidence |
|---|---|---|---|
| A1 | Recognise large `go test -v` and `go test -json` output; keep every FAIL, panic and the summary | 3 | 2.7 MB unrecognised in 9 runs |
| A2 | Recognise `npm test` / `pnpm test` / `yarn test` when they wrap jest or vitest (script banner, workspace prefixes) | 3 | 1.9 MB unrecognised |
| A3 | Failure summary for `make` and test runners: the failing target, the first error block and the exit line, nothing cut from a failure | 3 | 2.3 MB of failures passed raw |
| A4 | File-read family: `cat`, `head`, `tail`, `sed -n 'a,bp'`, `wc -l` on source files, with a mode for long files (outline: signatures and line numbers, full text kept for small files) | 5 | RTK `read`/`smart`; reading is the largest share of context |
| A5 | Search breadth: `rg` and `grep -r` without `-n`, `--files`, `-l`, `-c`; grouped by file with per-file and total caps | 3 | RTK grep grouping |
| A6 | Simple pipes and chains: `cmd \| tail -n N`, `cmd \| head`, `cmd \| grep x`, `cd dir && cmd`, `a && b` where every part is allowlisted | 5 | 5,191 metacharacter skips; needs a security review |
| A7 | Keep the full output of a compacted failure by default (CCR on failure only), with `downshift retrieve` | 3 | RTK tee/recall; CCR off by default today |

### B. Prove the value

| ID | Task | Pts |
|---|---|---|
| B1 | `downshift discover`: scan Claude Code transcripts (and Codex sessions) for Bash calls the compactor would have changed, with bytes and the top missed commands | 3 |
| B2 | `downshift report --daily` and `--history`, tokens only, per harness | 2 |
| B3 | After one week on the new binary, publish the real numbers in the README next to the bench | 1 |

### C. The agent's own tools

| ID | Task | Pts |
|---|---|---|
| C1 | Spike: can Read, Grep and Glob output be shortened per harness (Claude Code PostToolUse, an MCP read tool, or a steering rule toward `rg`/`cat` through the hook)? Write the evidence and a go/no-go | 3 |

### D. Cheaper workers (from Alethe)

| ID | Task | Pts |
|---|---|---|
| D1 | Playbook catalogue: scout-then-build, cascade, audit-at-scale, best-of-n; each with tier, effort and what it saves | 2 |
| D2 | Quota stop line before `delegate`: read headroom, refuse past the line and say why | 3 |
| D3 | Token budget per wave and per orchestration | 3 |
| D4 | Per-worker report: result kept, transcript left on disk, used or discarded marked | 3 |
| D5 | Codex worker adapter, then a generic CLI worker adapter | 5 |

### E. Setup fixes found while reinstalling

| ID | Task | Pts |
|---|---|---|
| E1 | Antigravity: when `~/.gemini/AGENTS.md` is a link, skip only that file; still register the MCP server and write the other files | 2 |
| E2 | Undo markers (`*.downshift-bak-*.new`) are written inside harness plugin folders (`~/.config/opencode/plugins/downshift/`); keep them in `~/.downshift/backups`. Also explain why the dry run said "would change" and the write said "new file" for the OpenCode plugin | 2 |
| E3 | Claude Code quota in the desktop app: the statusline never runs there, so quota routing reads a stale value; find another source or say so in `doctor` once | 3 |
| E4 | Claude Desktop rewrites its config while open: detect that it is running and hold the MCP entry until it quits instead of writing a value it will drop | 2 |

### F. Reach

| ID | Task | Pts |
|---|---|---|
| F1 | `json` (schema view), log dedup, `curl`/`wget` headers and body caps, `env` | 3 |
| F2 | `pnpm`/`bun install`, `rspec`, `rubocop`, `glab`, `prisma` | 3 |
| F3 | Automatic rewrite in more harnesses where the hook allows it (spike per harness) | 3 each |

## Order

1. **Wave A1**, in parallel, disjoint files: A1+A2+A3 (compressors), A4+A5 (read
   and search family), E1+E2 (setup).
2. **Wave A2**: A6 (pipes, with security review) and A7, then B1.
3. **Wave B**: D1–D4, then C1, E3, E4.
4. **Later**: D5, F1–F3, B2, B3.

At most three PRs open at once. A wave's PRs merge before the next wave opens new ones.

## Out of scope

- An API-traffic proxy (RTK's `rtk-proxy`): Downshift stays hook-shaped.
- A desktop workspace (Alethe's app): Downshift has the panel mod and the TUI.
- Any money figure: savings stay in tokens per harness.
