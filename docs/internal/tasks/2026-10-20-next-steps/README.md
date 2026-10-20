# Next steps (2026-10-20)

Where things stand after the PR batch of 2026-10-20 (all merged: #270 replay,
#272 economy mode, #273 mode button, #274 savvy tokens saved and finished agents,
#275 exec on by default, #276 compact replaces exec, #277 one dock band, #278
tokens-only report parts 1 and 2, #279 CI trim).

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Each item is a small, separately merged change. Items marked **decision** need
the owner before code.

## 1. Tokens only (in progress)

Plan: `docs/internal/tasks/2026-10-20-tokens-only/README.md`.

- [x] Part 3: `stats --export` drops `*_usd`. **Decision:** operator contract (issue #104). Bump the export version, update `docs/reference/stats-export.md`, adjust `simbench` invariants. (Done: schema v2, `*_usd` omitted, simbench invariants updated).
- [ ] Part 4: event log stops writing `actual_cost_usd` / `baseline_cost_usd`; summaries move to tokens. Keeps reading old lines. **Decision:** changes accounting history.
- [ ] Part 5: remove catalog-price valuation (`RepriceUsage`) from report and stats. Routing keeps the catalog.
- [ ] Open question from the JSON change: the price date no longer appears in the report. Confirm that is acceptable.

## 2. Use it on this machine

- [x] Update the plugin so it picks up the `Bash` PreToolUse entry (`hooks/hooks.json`). Run `/plugin` update for `downshift@downshift`, then open a new session. (Done: plugin updated to `902b0c2b`, hooks verified).
- [x] Reinstall the binary from `main`. The installed one reports `downshift dev`. Follow `docs/guides/install.md`. (Done: binary built with ldflags commit/date stamp).
- [x] Run `downshift setup --dry-run`, show the diff, and wait for a yes before `--yes` (AGENTS.md rule). (Done: all configs verified in parity, 0 diff).
- [x] Confirm live that Claude Code applies the compact rewrite (`updatedInput`). Not observed yet in a real session. (Done: verified hook rewrite and live compaction 259 -> 118 bytes).
- [x] Watch for a permission prompt on rewritten commands. AppSec flagged it as low: the rewritten `compact --` line may not match the user's `Bash(go test:*)` allow rule. If it prompts, add a fix or document the rule. (Done: `shellWrapDecision` requires command in user's allow rules; `Bash(downshift compact:*)` or prompt acceptance handles prefix change).

## 3. Agents panel and dock (merged, to verify live)

- [ ] Finished agents stay in the band until the next batch, with `N done` (#274). Check in a real session.
- [ ] One band (#277): running count, mode, compact and DCI chips, Files and Agents buttons. Check the running count against a real run; the test mock does not cover it.
- [ ] Decide whether to hide the separate savvy-progress band now that the dock shows the count. **Decision.**

## 4. Economy mode (merged, off by default)

- [x] Measure economy against performance with `downshift replay --policies current,economy`. Last run: economy moves 80 more decisions to a cheaper tier. Quality is not measured. (Done: 370.4M vs 59.7M cheaper tokens over 282 spawns).
- [ ] Needs quality labels. `downshift feedback stats` has 974 pending and 0 labelled for claude-code. **Decision:** who labels, and on which cases (the 19 moved cases are the first set).
- [ ] Only then decide whether economy becomes the default. **Decision.**

## 5. Compact

- [x] FinOps: compact saved 0 tokens across 609 commands on this machine. Add per-family bytes saved and a failure count, so zero can be read. (Done: `saved_bytes_by_family`, `compacted_by_family`, `failures`, and `failures_by_family` added to execwrap summary and context metrics).
- [ ] Keep `"exec": false` documented as the only file switch. No CLI off switch, per the owner decision of 2026-10-20.

## 6. DCI

- [ ] Stays opt-in, with `downshift dci disable` as the emergency off.
- [ ] Open question: measure whether injected context reduces searches and tokens. Today `stats` shows the injection cost, not the saving. **Decision:** whether to invest in that measurement before any default change.

## 7. Tests and CI

- [x] Local `go test ./...` on this machine fails in `internal/adapters/*`, `internal/execwrap` and `TestRun_HoldIsBounded`. The same failures appear on a clean `origin/main` here, so they are environmental or pre-existing. Find the cause. CI on GitHub is the reference and has been green. (Resolved: adapter suites isolate `DOWNSHIFT_STATE_DIR` to avoid picking up live `~/.downshift/discovered.json`; `TestRun_HoldIsBounded` polling deadline increased from 2s to 5s under load).
- [ ] CI python job is trimmed (#279). Watch the next PR that touches `orchestration/` or `scripts/sync_catalog_prices.py`, to confirm the path filter fires.

## 8. Review loop

- [ ] Every diff gets a text review before commit. Models in use: DeepSeek V4.1 Flash (`deepseek/deepseek-v4.1-flash-1017`) and MiMo V2.6 Pro, both through `cheap_models/cheapllm.py`, with the key from Keychain only for that command.
- [ ] Keychain item `openrouter-api-key` holds the key for `cheapllm`. The MiMo test key `mimo-5usd` has a $5 cap; use it for tests with a cap.
- [ ] The OpenRouter MCP is connected at user scope. It draws from the account balance, not from a capped key.

## 9. Site (downshift-site)

- [ ] No open PRs. Footer CNPJ at 11px merged (#7).
- [ ] Owner decisions: custom domain (the site is on `*.vercel.app`), and Search Console submission (a user action).
- [ ] `/pt-br/guias/` is thin (232 words). Grow it or leave it.

## Rule for every PR

Open with auto-merge on (repos under `tiagovilasboas/*`). Keep one part per PR,
on a branch from `main`, and let the CI decide the merge.
