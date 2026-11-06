<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Curation before the next battery (2026-10-21, evening)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Scope: the accounting (telemetry, `report`, `session --panel`, savvy-progress tiles), the
public claims (site, README, llms.txt), AGENTS.md against the repository, and the code merged
on 2026-10-21. Read-only audit of `origin/main` 21530d69 and the owner's own state directory
(aggregates only). Nothing was changed. Each item is one PR unless noted; points are story points
(1, 2, 3, 5); a block over 5 is split. Full audit tables (file:line evidence) are in the three
scratchpad files named at the end.

## 0. The five things that decide what we can say

| # | Finding | Why it matters |
|---|---|---|
| 1 | **The "KiroCrew real-use evidence" page is Claude Code data.** Today's log has 2 KiroCrew rows (both policy blocks, no usage) and 42 usage rows, all `claude-code`. The 42 decisions, 6 downshifts, 2.7M vs 5.7M, 53%, 114M tokens, the Compactor 347/37 totals (all-harness exec log, two days) and the DCI 6/6 (dated 2026-10-19) were read from the shared machine log and attributed to KiroCrew. | The page is live at `/docs/kirocrew-2026-10-21/` (hand deploy of an unmerged branch; not in sitemap; its inline style hash is not in the CSP). Site PR #40 would put 53% in the hero. Site #38 (merged) says KiroCrew is Functional; the matrix, the site's own install page and the page itself say Experimental. |
| 2 | **"Tokens" and "Tokens saved" count cache reads at face value.** Each turn re-reads the subagent's whole context as `cache_read`; the same agent shows ~160k new tokens in Claude Code and 5.6M in Downshift (30×). Today's `tokens_saved_estimate` = 15.9M, of which 15.89M is cache read on sonnet instead of opus, 6.5k is work, 5.4k is Compactor. | The headline metric promises "saved" tokens that still ran, and makes the Downshift panel disagree with the native Agents panel by an order of magnitude. |
| 3 | **The site's Compactor before/after table is overstated.** "After" is the summary size from the marker, not the emitted bytes (which include the 48–50-byte marker line). Real cuts: 46.7 / 51.5 / 53.8 / 52.9 / 31.7%, not 47.8 / 57.8 / 61.5 / 63.1 / 48.9%. Package labels cannot be reproduced (exec events carry no command). | Published on the home (EN and PT) by site #39, from numbers handed over without recomputation. |
| 4 | **`stats` miscounts shifts and rework is never measured.** "Upshifted 6" are effort-only changes (haiku→haiku); "Downshifted 7" includes 2 KiroCrew blocks that never respawned; every `child_ended` is `launched`, so "rework 0.0%" is structural. | Numbers that read as outcomes are artefacts. |
| 5 | **AGENTS.md diverges from the code it governs.** 228 lines against its own 200-line rule; "no shell metacharacters" while #410/#424 wrap `cd x && X`, `| tail/head`, `;` and `&&` chains; architecture block misses 11 packages; cache-tax "bundled" in ~12 docs and in setup's own output strings while `hooks.json` has `modules: []`. | Agents reading AGENTS.md make decisions on stale rules. |

## 1. Public claims: what to retract, what to keep

Order: retract first (no decision needed), then re-publish from one source.

| Step | Action | Where | Pts | Needs |
|---|---|---|---|---|
| 1.1 | Close site PR #40 unmerged (53% hero). | downshift-site | 0 | owner |
| 1.2 | Remove `/docs/kirocrew-2026-10-21/` from production: redeploy `main` (the page exists only in the hand deploy). Delete branch `docs/metrics-update-2026-10-21` or rewrite it as "Claude Code real use, 2026-10-21" with the harness named correctly. | downshift-site | 1 | owner |
| 1.3 | **Owner decision 2026-10-21: KiroCrew is Functional.** Make the repo match the site: extend the matrix definition of Functional to cover policy mode (block and retry observed on a real spawn), then update harness-matrix.md, README, docs/README, PT, llms.txt, install.md, value-per-harness and the site's install and capability pages in one PR. Replace "223 calls" (matches nothing) with the real figure or nothing. | repo + site | 2 | — |
| 1.4 | Fix the Compactor before/after table with `emitted_bytes` (marker included) and "single runs, bytes"; drop package labels. Or drop the table. | site index.html, pt-br | 1 | — |
| 1.5 | Merge repo PR #427 (four-arm evidence page) so the site's link stops 404-ing; add the 24% line to README and llms.txt (they still say "up to about 12%"). | downshift | 1 | PR is BEHIND: update branch, wait CI, merge |
| 1.6 | Regenerate the site's `llms.txt` and the real-usage page from the repo (site says 1,217/243 with percentages computed for 1,175; llms.txt lists three status words and omits Beta). | site | 2 | — |
| 1.7 | Decide where "Tokens saved (estimate)" may appear in public: nowhere until item 2.1 lands. | — | 0 | **decision** |

## 2. Accounting: one honest definition

Superseded by the owner's brief of 2026-10-21 (logic first, same numbers on every surface):
the spike and the parts are in [`../2026-10-21-accounting-roadmap/README.md`](../2026-10-21-accounting-roadmap/README.md).
The rows below stay as the audit trail; each maps to a test in part P2 there.

| Step | Change | Files | Pts | Needs |
|---|---|---|---|---|
| 2.1 | **One ruler, the harness's.** Headline tokens = input + output + cache write (what Claude Code's Agents panel shows). Cache reads shown apart, named. `tokens_saved_estimate` weights cache reads at the price ratio (0.1) or excludes them; label becomes "tokens moved to a cheaper model (estimate)". Today's 15.9M becomes about 1.6M. Shared definition in `telemetry` (A1), consumed by report, panel, mod tile. | telemetry/token_saving.go, report.go, sessionpanel.go, mods/savvy-progress (1.2.13 via patch) | 5 | **decision**: weight 0.1 or exclude |
| 2.2 | `verdictOK` must use spawn state: `StateObserved && Verdict=="OK"`, so held (guessed baseline, credit gate) never ranks as saved. Tests for GuessHeld and CreditHeld. (#416 follow-up, A2) | telemetry/token_direction.go | 2 | — |
| 2.3 | `stats`: count model changes only in Downshifted/Upshifted; report blocked (KiroCrew) separately; models mix from the model that ran, not the decision target. (A4, A11) | telemetry/aggregate.go | 2 | — |
| 2.4 | Rework: print "not measurable" until a failure end status is ever seen; a usage record must not outrank `stopped`; rework attaches only within the same group and agent type. (A6, A7) | telemetry/tasks.go, respawn.go | 2 | — |
| 2.5 | One population: headline, saved %, report and panel count the same spawns (control shown as a share, blocked excluded everywhere). (A9) | stats_headline.go, token_saving.go, report.go | 2 | — |
| 2.6 | Compactor counts: exclude `refused` and `not_found` from Commands; add `entry: hook|direct` to exec events (no harness can be attributed today, and the #410 env fallback stamps direct calls too); add `compactor_scope` to report JSON and label the tile "(all sessions)" or scope per session. (A12, A20, A22) | execwrap/metrics.go, exec.go, report.go, mod | 3 | — |
| 2.7 | Small: `tokens.records` always 0 in JSON; bootstrap interval computed but never printed; `doctor` "rewrite honored" uses inferred honor; panel reads last 2 MiB while report reads all; effort-only label names the classifier tier. (A16, A17, A29, A28, A5) | various | 3 | — |
| 2.8 | Delete `internal/accounting` or make the panel use it (it claims to be the only session function, has no callers, and scopes the Compactor the opposite way). (A13) | internal/accounting | 2 | — |
| 2.9 | Docs: `--period` → `--days` (README:181); `stats-export.md` cheaper/unchanged table; `agent-reference.md` `serve`/web; `daily-truth.md` #416 rule; README says `stats` shows Compactor numbers (it does not). (A23–A27) | docs | 2 | — |

## 3. Harness truth and KiroCrew

| Step | Change | Pts | Needs |
|---|---|---|---|
| 3.1 | KiroCrew is Functional (owner decision, see 1.3). What stays unmeasured there until a real `postToolUse` payload with child usage is captured: tokens per child, routed-vs-control, and any saving figure (matrix row 203 "not confirmed"). Say so wherever KiroCrew numbers appear. | 1 | — |
| 3.2 | Any per-harness claim needs a harness field: decisions have it, usage rows have it (all claude-code today), exec events do not (2.6). `report` prints the harness next to routed/control. | 1 | — |
| 3.3 | Remove the kirocrew adapter's tier-based context-trim hint (policy text in an adapter, layering rule 2). | 1 | — |

## 4. AGENTS.md and docs

| Step | Change | Pts |
|---|---|---|
| 4.1 | AGENTS.md ≤ 200 lines: move the architecture block and invariant detail to docs/design/architecture.md; add a docscheck line-count test. | 3 |
| 4.2 | Compact wording: replace "no shell metacharacters" (AGENTS, compaction.md, companions.md, compactor-default-on-adr.md, context-optimization.md PT) with the shapes of policy_compound.go / policy_chain.go; delete "including `npm test`" (it is wrapped); allowlist = "the families in harness-matrix.md (~180)". | 2 |
| 4.3 | cache-tax is not bundled: fix ~12 docs, the 3 setup output strings, and the site pages. | 3 |
| 4.4 | Architecture block: add accounting, benchmark, cursorbridge, decisionintelligence, docscheck, gateway, lifecycleobserver, outcome, replay, testenv, testrace, compressor/learning blank import, cmd/dsmon*. | 2 |
| 4.5 | archtest: allow-lists (not deny-lists) for core/session/fuse/codeintel; rule that adapters never import classify/fuse/semantic/nbtier; test that no telemetry row struct gains prompt/task/path; install_test asserts the seeded list lacks claude-fable-5-1. | 3 |
| 4.6 | Stale: skills/harness-engineer "bytes stay separate"; mods/README savvy-progress version; install.md hook counts and skills list (4 skills, 8 hook entries); cli.md `enable`; agent-reference `serve` and the dsmon launchd script that runs it; "opt-in" in five code comments; site pages still installing downshift-dock. | 3 |

## 5. Code merged today: bugs to fix before the next battery

| Step | Finding | Files | Pts | Needs |
|---|---|---|---|---|
| 5.1 | **Critical.** `eval "$(downshift enable)"` (#392/#408) wraps whole tool families through `compactor --`, and `Run` refuses anything outside the allowlist: `go build`, `go version`, `git -C x status`, `npm install`, `docker run` exit 126 and never run, in any non-TTY shell. Anyone who enabled the shell helper is affected. | cmd/downshift/shell_init.sh, execwrap/run.go | 3 | ship first; warn users |
| 5.2 | **Critical (privacy).** The compressor learning store keeps up to 4 KiB of raw command output per family (branch names, PR URLs, paths) in `~/.downshift/compressor-learning.json`; the async write is cut at process exit (0-byte `.tmp`). Violates "telemetry never stores prompts, tasks or file paths". | compressor/learning/learning.go, execwrap/run.go | 3 | ship first; purge the file on upgrade |
| 5.3 | `cd\nrm && go test` is rewritten to `cd rm && …` (cdPrefix splits on any whitespace); `cd -`, `cd -P x` and `export GOFLAGS=-toolexec=…; go test` are wrapped through the chain path and their tests were deleted (#410/#424). | execwrap/policy_compound.go, policy_chain.go | 2 | — |
| 5.4 | #424 behaviour change undocumented: `git status && rm -rf x` and `go test ; git push --force` are rewritten (only the allowlisted segment is wrapped). CHANGELOG + security note. | CHANGELOG, harness-matrix | 1 | owner awareness |
| 5.5 | **Root cause of the 54 passing `go test` not compacted:** not the stderr gate. The compressor refuses `-v` transcripts (several packages, PAUSE/CONT, skip reasons, second PASS line) and the "no tests to run" warning; 33 of 44 were ≤141 B with no gain after the marker. | compressor/go_verbose.go, compressor.go | 5 | — |
| 5.6 | Tests leak into the real state dir: six packages isolate with `HOME` only while `DOWNSHIFT_STATE_DIR` takes priority (core, catalog, models, dsmon-hook, training, telemetry). Writes catalog.json, openrouter.json, agents.jsonl, loop-events.jsonl, context-*.json into the owner's state. | the test helpers listed in audit-bugs.md B2 | 2 | — |
| 5.7 | Attribution: `report` pools every harness with no label; direct `compactor` calls take `CLAUDE_CODE_SESSION_ID`, so a KiroCrew run from a Claude Code terminal is credited to Claude Code (#410). | telemetry/report.go, exec.go, session.go | 3 | — |
| 5.8 | Small: `UpshiftNotApplied` also counts observed rows (#422); Windows never rewrites (QuoteExecutable needs a leading `/`, undocumented); e2e battery depends on `~/.claude`; no test for the `shell-init` alias; hookctx flake under load. | various | 3 | — |
| 5.9 | The battery runner (downshift-labs): state per run and judge HOME isolated; part 2 needs the #424 binary **after** 5.3. | — | 0 | — |

No defects found in #414, #421, #423, #418, #411/#412, #419/#426.

## 6. Order before any new battery

1. Stop the bleeding: 5.1 (shell helper blocks commands) and 5.2 (raw samples on disk), one PR each, then a release so installers get them.
2. Retract: 1.1, 1.2, 1.3, 1.4 (site), 1.5 (merge #427). One evening.
3. Accounting definition: owner decision on 2.1, then 2.1 + 2.2 + 2.3 in three PRs.
4. Rewrite safety and test isolation: 5.3, 5.6; Compactor parser 5.5.
5. Only then part 2 of the battery (natural tasks), capped in **total** tokens, with the exec-event `entry` field so the Compactor arm is attributable.

## Owner decisions

- D1. Cache reads in the headline: weight 0.1 or exclude (2.1).
- D2. Where the single "tokens moved" number may appear in public, if at all, before part 2.
- D3. Decided: KiroCrew is Functional (owner, 2026-10-21); the matrix definition is extended to policy mode.
- D4. Branch `docs/metrics-update-2026-10-21`: delete or rewrite as Claude Code real use.

## Sources

- `scratchpad/audit-accounting.md` (30 findings, A1–A30)
- `scratchpad/audit-divergence.md` (58 findings, A–F, 55.5 pts)
- `scratchpad/audit-bugs.md` (pending)
- Log aggregates: today 175 claude-code rows (42 usage, 6 control, 11 rewrites), 12 codex, 2 kirocrew; exec events 354 (328 direct, 26 hook).
