<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **The orchestrator LLM chooses the model; Downshift applies and measures it (owner decision O8, 2026-10-28).**
  The decider keeps a pinned model, then applies a declared profile, the first matching declared rule and the
  orchestrator's tier hint (down only for now); everything else keeps the requested model. The guard still refuses
  foreign ids and `explicit_only` models and keeps on doubt. Downshift's own code still calls no LLM.
- `downshift setup` no longer writes `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1`: with it, Claude Code overrides the
  per-call model the orchestrator chose. A value you set yourself is left alone.

- The docs no longer say "zero dependencies": the binary links a few third-party libraries through its command layer. What holds, and is now checked by a test, is that every package under `internal/` uses the Go standard library only, except the MCP transport that wraps the official Go SDK.

### Added

- `downshift report` shows why spawns changed or stayed (verdict and reason counts per harness) and tokens per harness in two columns that are never summed; `downshift session --panel` shows the requested model, the final model, the verdict and the reason per spawn. Tokens only, no money. Events carry a prompt-free `routing_note`.
- The MCP `providers` tool accepts the model ids the orchestrator sees. They are stored in the discovery cache as a hint (`source: "mcp"`): a fresh detection always wins and the hint expires after one hour.

### Changed (models)

- `downshift setup` no longer writes `session-models.json`. Models are always the ones the harness reports; the file is only an optional manual override, and setup names the `downshift models discover` command when a harness has no detected list yet. `doctor` no longer warns forever for Claude Code, which uses the catalog stand-in.

### Removed (O8)

- The built-in default agent-type table (N11) and the economy-mode step that read it. `downshift policy economy`
  still exists and now acts only on declared policy; its removal is pending.
- The automatic upgrade after a recorded failure (`CASCADE_UPGRADE`).
- The built-in risk veto in the guard. Protection is now the orchestrator's judgment, guided by the playbook, and
  tier floors you declare in `policy.json`.

### Added

- `downshift models available --harness ID`: the models a spawn can use right now on a harness, from the payload, the discovery cache or your session-models file (the catalog only fills in tiers, and is the last-resort stand-in on Claude Code, flagged as such). `doctor` gets an `available models` check.
- `downshift route --harness ID`: show what would be applied for one spawn given the orchestrator's choice; the hook uses the same code.
- `downshift mcp serve`: an MCP server on stdio, built on the official Go SDK and serving both protocol eras (the `initialize` handshake and the per-request metadata of 2026-07-28). Four information tools for the orchestrator: `providers` (the models this harness offers now), `playbook`, `status` (quota advice) and `apply` (what would be applied for a choice); every result carries a `strain` block. It never picks a model and takes no task text.
- `downshift setup` registers the server in Claude Code (through `claude mcp add -s user`) and Codex (a marked block in `config.toml`) by default; `--no-mcp` skips it, the dry run shows it first, `setup --undo` removes it, and other servers and comments stay untouched.
- `downshift playbook` (`list`, `show`, `edit`, `add`, `remove`, `restore`, `check`): named working rules a delegated
  worker receives. General always applies; a delegation may name one more set. Shipped: General, Backend, Frontend.
- The `downshift-orchestrator` skill, installed by `downshift setup`: how to split work and choose model and effort
  per spawn (it carries the defaults the removed table used to hold).
- **Default profile table (W7.1):** (removed again by O8, see above) built-in agent type mapping for Claude Code (`Explore` -> small/Haiku, `claude-code-guide` -> small/Haiku, `statusline-setup` -> small/Haiku; `general-purpose` and `Plan` keep requested model). Table runs as default precedence (level 5), preserved behind explicit project policies and hints, with automatic elevation to mid (Sonnet) when risk signals are detected.
- **Policy init command (W7.2):** `downshift policy init` generates `.downshift/policy.json` with dry-run preview and interactive confirmation gate, exposing the full profile and rule configuration schema.
- **Economy mode (W7.9):** `"mode": "economy"` in `policy.json` and `downshift policy economy [on|off]` command to shift all agent profiles down one tier within safety boundaries (risk never drops to small, explicit_only preserved).
- **Autonomous engineering loop (F1.6):** `downshift loop` command running an autonomous BDI loop (`Plan -> Implement -> Sensor -> Cascade -> Accept`) with sensor verification via compactor and automatic failure registration.
- **Episodic failure cascade & inter-process dead-ends sensor (F1.1, F1.2):** cross-process directory lock (`dead_ends.lock`) and atomic JSON state tracking to automatically auto-upgrade subagent tiers on repeated task failures.
- **Zero-prompt CLI entry (F0.1):** running `downshift` without arguments now launches an intelligent status and telemetry summary view.
- **Interactive TUI setup (F0.3):** `downshift setup --tui` providing interactive terminal interface for harness configuration and dry-run preview.
- **Uninstall and purge (F0.2):** `downshift uninstall` with `--purge` flag completely removing state directory and restoring all harness backups.
- **Dynamic model tier discovery (F0.4):** auto-seeding relative tiers (`cheap`, `standard`, `frontier`) in `session-models.json` on `downshift setup`.
- **Subagent model force configuration:** setup configures `"CLAUDE_CODE_SUBAGENT_MODEL_FORCE": "1"` in Claude Code's `settings.json`, ensuring subagent rewrites by Downshift are honored across built-in agent types.

- **KiroCrew harness detection:** `SessionEnv: "KIRO_SESSION_ID"` in `KiroCrewCaps` enables
  `envHarness()` to attribute exec-events to `kirocrew` instead of `unknown`. The `Model` field
  in exec-events is populated via `InferKiroCrewModel()`, which reads the session model from
  `~/.kiro/crew/sessions/<id>/state.json`. PRs #559, #560, #561.
- **REA curation:** reverse engineering analysis content moved to
  [downshift-labs](https://github.com/tiagovilasboas/downshift-labs); integration evidence
  (rewrite-honored, executor-ack, benchmarks) stays in Downshift. Added `scripts/curate-rea.sh`
  for future curation. PR #562.

- **Cursor adapter:** composer baseline cache (`downshift cursor-touch` on
  `sessionStart` and `beforeSubmitPrompt`, `~/.downshift/cursor-composer-model.json`);
  `subagentStop` links by `subagent_id` on Cursor 3.27.12+. `downshift setup cursor`
  wires the full hook set. Rewrite honor on Cursor remains unconfirmed.
- **Compactor:** `go test` clean runs fold passed tests that logged output (#497); failed runs keep every failure line, cutting only passing noise (#491); `2>&1` and a single quoted command word are accepted (#499); `VAR=value` prefixes (#494); Jest reports on stderr compact (#505); pnpm/yarn exec, PHP and `docker compose exec` forms (#507, #506); refusal and steering lists name the allowed families for every harness (#498).
- **Shell rewrite:** Codex (`^Bash$` PreToolUse, #503) and Cursor (`Shell` preToolUse, #512); Cursor advise fallback on `beforeShellExecution` (#512).
- **Routing:** the subagent type is a doubt-filling signal for Claude Code (`Explore`, `Plan`, #496); `spawn_run` batches are routed per entry (#504); confident short read-only lookups classify as Simple (#492).
- **Accounting:** one `Compute` feeds `report`, `session --panel`, the dashboard API and the mods (#489, #500, #508, #520); the panel carries the report block (#500); `tokens_saved_short` and `scope_label` (#531); `summary`, `saved_total` and `--no-report` / `--brief` on `session --panel` (#530, #540).
- **Report:** a window with more than one harness prints the current harness first and groups the others by name. `--json` adds `current_harness` and `by_harness`.
- **Cursor:** a picker slug whose catalog id is already in the session (for example `grok-4.7` → `grok-4.7-xhigh`) is a session model, so a confident downshift rewrites. A slug that resolves outside the session stays unchanged.
- **downshift-progress** (in development, not in the marketplace yet): scaffold, flow tools, data client, band, Agents pane and roster (#528, #529, #532, #533, #535, #538, #539, #541, #542).
- **CI:** `make check` with Windows and Linux vet; `merge_group` trigger; lefthook pre-commit (gofmt, vet, staticcheck, tests of staged packages) and pre-push gates (#522, #523, #526, #534).

### Fixed

- **Claude Code session list:** PR #570 removed the session inferred from the current
  model, but Claude Code sends no session list in the hook payload, so every spawn had an
  unknown session and kept its model: no rewrite reached a subscriber. Tests and the smoke
  passed only because they sent `session_models`, which the real harness does not. The
  candidates are now catalog data (`session_candidates` in `catalog.json`) read when a
  harness declares `SessionFromCatalog` (on only for Claude Code): one automatic candidate
  per tier below the current model, ending at the current model, so an upshift never goes
  past it. explicit_only rows are never added; an unknown or empty current model still
  holds; a list from the payload or discovery wins. The smoke now covers a payload without
  `session_models`.

- `downshift session`: switches and effort changes are counted apart; held rows split into guardrail holds and upshifts not applied (#422, #400, #509).
- Compactor: Playwright and tsc output with danger words stay raw (#414); cover runs with no-test packages compact (#421).
- Routing: native haiku canonicalises to `claude-haiku-5-5` (#525 closed as obsolete, fixed on main); the timing budgets run only on CI (#524).
- Tests: the test env drops git hook variables so a test cannot rewrite the shared repo config (#536).

### Removed

- The vendored `filetree` mod (`mods/filetree/`, plugin `filetree@downshift`) left the repository and the marketplace (owner decision 2026-10-24); `downshift setup` no longer installs it and never uninstalls it from existing users.

## [0.1.3] - 2026-10-23

### Added

- **Compactor — linters now compress (C-2a):** `eslint`, `ruff`, `mypy` and `golangci-lint`
  outputs are now compacted. The compressors existed but were shadowed by generic detectors
  (ripgrep stole golangci-lint's `file:line:col:`, TSC stole ruff/mypy's `Found N errors`).
  Detectors moved before TSC/ripgrep, multiline-aware `(?m)` regexes, ANSI guard (fail-open).
  Measured: eslint 304→189, ruff 174→112, mypy 214→113, golangci 167→132 bytes.

- **Compactor — builds now compress (C-2b):** `go build`, `cargo build` and `dotnet build`
  compiler diagnostics are now compacted. `go build` had the same shadowing issue (ripgrep);
  `cargo build` and `dotnet build` were misidentified as `cargo test` via the error danger word.
  Reordering + per-build fingerprint guards (Compiling marker for cargo, MSBuild grammar for
  dotnet, go vet exclusion for go build). ANSI guard. Measured: go 128→113, cargo 164→82,
  dotnet 171→107 bytes.

- **Dashboard — real token savings per session (B-W3):**
  - `GET /api/sessions` now includes a `tokens` field per session: `routing: TokenSaving`
    (routing decisions) + optional `compactor` (exec events for that session). Fixes a bug
    where the endpoint passed 0 to `TokensSavedEstimate` for the compactor, unlike the
    individual session endpoint.
  - Sessions tab: expanded session shows a `TokensSummary` donut (% real saved from routing)
    and a compactor line when the session has exec events.

- **Dashboard v2:** sessions-first UI with expandable agents, harness filter chips, bidirectional
  URL sync for tab navigation, E2E harness with `?e2e=1` deterministic stub, `downshift dashboard`
  command.

- **KiroCrew — shell advise (A-P1):** the KiroCrew `preToolUse` hook now advises a compacted
  retry when a shell command is in an allowlisted family. The hook emits exit 2 with a stderr
  message showing the exact wrapped command to run.

- **KiroCrew setup (A-P8):** `downshift setup kirocrew` strips the informational
  "Downshift KiroCrew" row from `~/.kiro/crew/hooks.json` (it had no effect as the real hook
  lives in `agent.kiro_hooks`). `doctor` warns when the row persists; `--undo` restores the
  backup.

### Fixed

- **KiroCrew:** hook now recognises the live shell tool name (`shell` instead of the old name).
- **DCI:** code intelligence hook runs through the shared prompt-context runner, fixing a layering
  violation where it bypassed the runner's session/harness context.

### Changed

- **Shellwrap domain extracted** to `internal/shellwrap/` (was inline in the hook runner).
  Architecture docs updated to reflect the real package graph.
- Accounting v1
  - `internal/catalog/pricebook.go`: PriceBook wrapper with `PriceFor(model)` that
    returns `exact`, `upper_bound`, `inferred`, or `unpriced` quality. Slug
    inference maps gateway slugs (HAIKU, SONNET) to canonical model ids.
  - `internal/accounting/formula/`: Pure `Saved()` function implementing the
    spec formula: `Saved = C_baseline − C_actual` where actual cost is the sum
    of each bucket's tokens at its model's price, and baseline cost is all
    tokens at the requested model's price.
  - `internal/accounting/harness.go`: `AggregateByHarness()` groups exec events
    by harness and entry type, enabling per-harness cost reporting.
  - Golden test fixtures: 12 regression test cases in JSON format covering
    all quality levels and edge cases.
  - Spec with documented limitations: +15% positive bias on downshifted runs
    (more turns at cheaper model inflates the baseline), compaction priced
    conservatively at input cost once. See `docs/design/accounting.md`.

### Changed

- Exec events gain two attribution fields (`harness` and `entry`). Direct
  `downshift compactor -- …` calls no longer fall back to the harness session
  env var for their session: a direct call carries no session unless
  `--session=` is given, preventing Claude Code's `CLAUDE_CODE_SESSION_ID`
  from being credited to a KiroCrew or terminal run (audit A22). Old rows
  without the fields are read as `entry: unknown`. See
  `docs/reference/spawn-state.md` for the field definitions.

### Added

- `exec-events.jsonl` rows gain two new fields (additive, absent in old rows):
  `harness` (e.g. `"claude-code"`, `"kirocrew"`, or `"unknown"`) and `entry`
  (`"hook"` when the adapter set `--session=`; `"direct"` otherwise). Per-harness
  Compactor reporting now has the attribution data it needs.

### Fixed

- Privacy: the compressor learning store (`~/.downshift/compressor-learning.json`) no longer holds output text. Schema 1 kept up to 4 KiB of raw command output per signature, which could contain branch names, PR URLs and file paths. Schema 2 keeps the output's size and a shape (per-line class counts, line-length buckets, a byte-class histogram and a hash of the first line's character-class mask; documented in `docs/features/learning.md`). An existing store is migrated on first use: the samples are dropped and the file is rewritten at schema 2; `downshift learning clear` purges it now. `export` and `suggestions` carry the shape only. The runner also waits for the learning write under a 500 ms deadline instead of leaving it in a goroutine that process exit cut off (a 0-byte `compressor-learning.json.tmp` was left behind); saves go through a unique temp file and rename, and stale temp files are removed on load. The command's output and exit code are unchanged.
- `install.sh` probes the archive URL before downloading. A release with no archive for your platform now ends with an explicit message (pick another tag, `go install`, or report it) instead of a bare `curl: (22) 404`. CI gains a `verify-release` job that fails a tag whose release is missing `checksums.txt` or an archive, or whose `release` job did not succeed.

### Changed

- One final metric, `tokens_saved_estimate` (owner decision 2026-10-21): routing tokens saved plus the Compactor's saved bytes / 4, labelled an estimate. Defined once in `telemetry.TokensSavedEstimate`. `downshift report` ends with `Tokens saved (estimate): N` and drops the separate Compactor bytes and tokens lines (`--json` keeps the parts and adds the total); `session --panel` adds `tokens_saved_estimate` (this session's routing plus the Compactor all-time total); `savvy-progress` 1.2.12 replaces the `Downshifted` and `Compactor total` tiles with one `Tokens saved` tile. This reverses the earlier rule that the Compactor bytes are never added to the tokens.
- `downshift setup` refuses to write hooks that point at a binary under the temporary directory, a Go build directory (`go run`, `go test`) or a world-writable directory without the sticky bit: the hook would break when the file is cleaned, or run whatever another user puts there. `--dry-run` and `--undo` only warn; `--allow-unsafe-binary` overrides.
- The Compactor hook no longer requires a Bash allow rule on the original command (owner decision 2026-10-20). It rewrites an allowlisted command unless a deny or ask rule in the managed, user, project or local settings matches it (as written or normalised); unreadable or unparsable settings still mean no rewrite. The hook emits no `permissionDecision`, so approval prompts follow the user's permission mode. No allow rule is installed or required.
- Docs: the harness matrix, companion guide, README, AGENTS.md and the Portuguese README now describe the Compactor's native rewrite as the code does: no Bash allow rule is needed, and only a matching deny or ask rule stops it.

### Added

- `downshift setup` wires the KiroCrew `preToolUse` hook. It writes a wrapper to `~/.downshift/bin/kirocrew-pre-tool-use.sh` (`.cmd` on Windows) and adds it to `agent.kiro_hooks.preToolUse` in `~/.kiro/crew/config.json` beside the existing hooks, only when that file exists. A wrapper registered by hand is pointed at the generated one. `--undo` restores both. An install made with `install.sh` no longer needs a repository checkout for this. `doctor` now reads the KiroCrew wiring from the same table as the other harnesses and accepts the wrapper from any directory.
- `downshift session --panel` gains an optional `compactor` block (commands, compacted, raw and saved bytes, estimated tokens) for the session. The hook adds the hashed session to the wrapped command (`downshift compact --session=<hash> -- …`) and the runner logs it in `exec-events.jsonl`; older records belong to no session. Bytes are measured, tokens are bytes / 4 and are not added to the routing figures.

- Opt-in `downshift exec` shell wrapper (owner decision 2026-10-19; off by default at the time). Superseded before release: the wrapper is now `downshift compact` (`exec` stays an alias), on by default, and needs no Bash allow rule (see Changed). `downshift context enable native --exec` turns it on, `--no-exec` or `downshift context disable` turns it off, and `DOWNSHIFT_EXEC=0` overrides both. When on, the PreToolUse hook rewrites a shell command to `downshift exec -- <command>` only when it is `go test ...` or `git status ...`, every byte is in a small safe set (no pipe, list, redirection, `$`, backtick, parentheses, quotes, globs, braces, tilde, backslash, newline), it is not already wrapped, and the running binary resolves to an executable file (the command invokes it by absolute, symlink-resolved, single-quoted path, never a PATH lookup); anything else runs unchanged. `downshift exec` runs the program with no shell, inherits stdin, forwards SIGINT/SIGTERM/SIGHUP/SIGQUIT, and exits with the program's code (127 when not found, 126 when it cannot run, 128+N on signal N). Output keeps the order the program wrote it in: stdout is held for at most 2 seconds, and only while compaction is still possible; the first stderr byte, 2 MiB or the 2 second bound flushes it and the run stays raw. Stdout is compacted only when the exit code is 0, stderr is empty and the compressor recognises that command's own format, and a compacted result ends with a `[downshift exec: compacted N to M bytes]` marker line. Otherwise it is byte-identical. The child runs in its own process group and a forwarded stop signal reaches the whole group (a SIGKILL of the wrapper itself cannot be forwarded). `DOWNSHIFT_EXEC_DEPTH=1` in the child environment stops nested wrapping. The policy is harness-free (`internal/execwrap`); the hook runner reads `HarnessCapabilities.CanRewriteShellCommand` (true only for Claude Code). The Claude Code adapter only parses `tool_input.command` and emits `updatedInput` with every sibling field kept and no `permissionDecision`, so the harness permission flow still runs on the rewritten command. The hook rewrites only when the user's own Bash allow rule matches the original command and no deny or ask rule in the managed, user, project or local settings does (a settings read or parse error means no rewrite), and `downshift exec` itself refuses anything outside the allowlist with exit 126. No `downshift exec` allow rule is needed or advised. Claude Code needs a manual `Bash` PreToolUse entry (`enable --exec` prints it; `setup` does not write it). Not yet observed on a live session. Telemetry is prompt-free: `exec-events.jsonl` holds the family, raw and emitted bytes, exit class and a compacted flag, never command text, args, paths or output. `downshift context metrics` adds `exec_metrics` with measured bytes saved and `estimated_tokens_saved` (bytes / 4, labelled estimated).

- Token facts behind the savings (owner request 2026-10-19). Downshift does not reduce tokens: a routed child runs about the same tokens on a cheaper model, and the dollar saving is those tokens priced on the requested model. `downshift stats` now prints `Tokens moved to a cheaper model: ... · to a pricier model: ...` next to the dollar line (input / output / cache), and `--detail` adds a per-harness, per-model-pair table with the token mix by model. `stats --export` carries the same as `tokens_rerouted` (documented in `docs/reference/stats-export.md`). Harnesses with no token source (today Cursor, Codex, OpenCode) read `unobserved`, never zero. `downshift report` adds tokens per completed task for the routed and control groups, in text and `--json`, so a cheaper model that needed more tokens shows.
- Release job: goreleaser is pinned to `v2.18.3` (was `~> v2`) and each archive listed in `checksums.txt` gets a build-provenance attestation (`actions/attest-build-provenance`, pinned by SHA). Verify a download with `gh attestation verify <file> --repo tiagovilasboas/downshift`. `install.sh` does not verify the attestation yet; it still checks the sha256 in `checksums.txt` (follow-up). Not exercised until the next `v*` tag.
- The vendored `savvy-progress` mod draws its bar in the Downshift brand green (`#22C55E`) with brand ink on the percentage pill, instead of upstream's violet. It is the only local patch to the vendored file and is documented in `mods/savvy-progress/README.md`; `downshift setup` installs the mod, so the colours arrive with it.
- `downshift setup` on Claude Code installs the Downshift plugin through `claude plugin marketplace add` and `claude plugin install`, so the hooks and the bundled cache-tax mod arrive with one command. It removes the Downshift hooks it had written into `settings.json` (the plugin carries them; keeping both would route every spawn twice) and falls back to the settings hooks, naming the reason, when `claude` is not on PATH, when a login shell does not resolve `downshift` to the binary running `setup`, or when the install fails. `--no-plugin` keeps the settings hooks.
- `downshift setup` on Claude Code also installs the sibling mods `savvy-progress@downshift` (progress bar above the prompt, `/agents-info` panel of subagents) and `filetree@downshift` with `claude plugin install`, skipping any already enabled. They carry no routing, so they install even when routing falls back to the settings hooks, and a failed install is only a note. `--no-plugin` skips them; `downshift-router` stays opt-in (it would route twice).

- New Downshift-owned mod `downshift-dock` (plugin `downshift-dock@downshift`, `mods/downshift-dock/`): a one-line band above the prompt in the Downshift brand green with **Files** and **Agents** buttons. A press runs the other mod's own slash command through `$.command.run` (`/filetree`, `/agents-info open`), so a closed pane comes back with one click or keypress. A button shows only when the session lists that command as a plugin command, and both show until a listing answers. `downshift setup` installs it with the other mods. Not vendored: original code, no routing logic.

### Fixed

- The shell helper from `downshift enable` broke every wrapped tool outside the allowlist. Its functions for go, git, npm, docker, aws, gh, make, pip and the rest ran `downshift compactor -- <command>` whenever stdout was not a terminal, and the compactor refuses a command it does not allowlist with exit 126, so `git -C <dir> status`, `npm --version`, `docker run` and similar never ran inside `$(...)`, pipes, scripts, CI or an agent shell. The helper now asks the binary first (`downshift compactor --allowlisted -- <command>`: exit 0 or 1, no run, no output, no exec log row, one extra fork) and wraps only an allowlisted command; anything else runs as typed through `command`. The script also parses under dash (`sh` on Debian and Ubuntu): the hyphenated `golangci-lint` function, which dash rejected and which aborted the whole eval, is now defined only where the shell accepts it and aliased elsewhere. Affects everyone who ran `downshift enable`: re-run `eval "$(downshift enable)"` or open a new shell after upgrading.
- `savvy-progress` 1.2.8: the band's `×N` badge counts only the agents the panel lists (running ones, finished ones still inside the 10 s window, and planned tasks), not every agent of the session. The mod's tests cover both cases.
- `savvy-progress` 1.2.7: the `✕` on the band now works when the band follows the live subagents (it cleared a flow the band no longer used, so the band came straight back); the band returns with the next agent. The `✕` moves to the right edge of the row. The finished-agents clock read (#333) no longer throws when the host gives no clock, which turned two upstream tests red on `main`.

- The savvy-progress bar kept coming back purple: Claude Code caches a plugin by its version, and the cached 1.2.1 on machines that installed it before the colour patch still held upstream's `#8f8cf4`. The mod now ships as 1.2.2 (only `plugin.json` `version` differs from upstream, carried by `downshift-local.patch`), so a plugin update replaces the cache. To get it: `claude plugin marketplace update downshift`, then `claude plugin update savvy-progress@downshift`, then restart Claude Code.

- Context sensor recorded 0 bytes for every Bash result (598 commands measured, `total_output_bytes` 0 everywhere): Claude Code's Bash `tool_response` carries `{stdout, stderr}`, and the observer read only a `content` string. It now reads stdout and stderr, content blocks and a plain `output` string, and the plugin and `setup` register the PostToolUse hook for `Bash` too (observe only: byte counts and the compressor's potential, never the text). Existing installs need `downshift setup` or a plugin update to add the Bash matcher; the sensor then shows how much `downshift context compress` would cut on real commands.
- The compressor squelches `git status` only when the tree is clean and up to date (or has no upstream). Before, "Your branch is ahead of 'origin/main' by 3 commits" and similar lines were dropped from a clean-tree summary; now ahead, behind, diverged, detached or any other extra line keeps the original output.
- Compressor corpus defects fixed. Repeated-line log dedup now runs only after the go test, status and log detectors had their say, so a failing test run or a `git log` with repeated lines stays raw. Output with no final newline (possibly cut off) or with carriage returns is returned unchanged. The go test summary now keeps the pass count, every package name (shared module prefix stated once) and the coverage figure, and is returned raw unless it is strictly shorter than the input. The adversarial corpus tests run by default; `DOWNSHIFT_CORPUS_STRICT` is gone.

- Cursor catalog prices verified against https://cursor.com/docs/models (2026-10-19): every Cursor row gains its cache-read price (and cache-write on Anthropic and OpenAI rows), so Cursor dollar figures are no longer an input-rate upper bound; `composer-2.5` (0.5/2.5), `composer-2.5-fast` (3/15), `glm-5.2-high` (1.4/4.4) and `gemini-3.8-flash-high` (output 3.75) are corrected; the Fast Grok slug `grok-4.7-high-fast` (4/12) gets its own row instead of being priced as the standard Grok 4.7. Codex and xAI rows were re-checked and already matched their vendor pages: the Codex frontier (`gpt-6-sol`, 2/10) under mid (`gpt-5.6-terra`, 2/12) inversion is the vendor's list price.

- Savings accounting and baselines (low-savings review, 2026-10-19): a custom or plugin agent is no longer an unknown baseline just because its type is not a built-in. Its definition decides (`.claude/agents`, user `agents`, installed plugins): no `model:` or `inherit` inherits the parent model, so a downshift counts; a named model, and the built-ins `claude-code-guide` and `statusline-setup`, are a known baseline that routing leaves alone (reason `AGENT_PINS_MODEL`). The parent model is read from the assistant message that issued the Task call, not the last one written, so a `/model` switch between turns no longer leaves a stale baseline. A usage record is credited to the router only when its decision applied a rewrite: a spawn left alone that ran on another model no longer shows as a saving or an overspend (one day showed a false `$2.38 cost more`). A child stopped and resumed now records the growth of its transcript at each stop; before, only its first stop was priced. `stats` counts honor over applied rewrites only, instead of reporting every unchanged spawn as "unobserved".

### Changed

- `savvy-progress` 1.2.5: the first panel tile reads `Downshifted` and a `Compactor` tile shows, with a ↓, the bytes the Compactor cut in the session, from the new `compactor` block of `downshift session --panel`. Not summed with the downshifted tokens: different units and certainty. No `compactor` block, no tile.

- `savvy-progress` panel and band say less (owner request 2026-10-20; mod 1.2.4, patch regenerated against the pinned upstream commit): the band and the panel header show one short line (`Downshift · 2 switched · 1 held`) instead of the long tokens sentence, the per-type line and the work/cache percentages are not drawn, each row shows a one-word status and `↓` plus the tokens that agent moved to a smaller model, and the tile reads `On smaller model` in English. The CLI output, `tokens_line` and the JSON contract are unchanged.

- `downshift report` text output shows tokens only. The dollar column, the API lens block and the dollar footer are gone; the table shows tokens per completed task, and the plan lens is labelled tokens only. The JSON report and `downshift stats --export` are unchanged and keep their dollar fields.

- Tokens first, dollars in the report (owner decision 2026-10-19). Every live view (session tree, `session --panel` and the savvy-progress band and rows, statusline, `watch`, `dsmon`, the child-end line) and the `stats` headline show **tokens saved from the requested model**: the share of child tokens that ran on a cheaper model than requested, per agent, per agent type and per session, with work (input+output) and cache (read+write) counted apart, since cache reads carry the smallest price gap (Sonnet to Haiku saves nothing on them). Control, held, kept and refuted children count in the totals, never as saved; a pricier child is shown as such. No dollar in these views. `downshift report` prices the recorded tokens at the current catalog when it runs (`telemetry.RepriceUsage`, `prices_as_of` in the header and JSON), so a price correction reaches the whole history, and reads them through an API lens (dollars per completed task with its interval, no percentage) and a plan lens (tokens only, `tokens_saved`). The report's dollar percentage (`percent`, `percent_low`, `percent_high`) is removed. `stats --detail` drops its dollar blocks and points at the report; `stats --export` keeps its dollar fields. `session --tree --json` and `session --panel` replace their dollar fields with `tokens` and `tokens_label` / `tokens_line` / `by_agent_type`. The vendored savvy-progress mod draws the new fields through its local patch (36 mod tests pass).

- Classifier: a long brief (80+ words, not bounded) that states its task is scored by that task, with the rest of the brief weighed as context. The task is the section after `Task:` / `Tarefa:`, or else the sentences that open with a work verb. Outside it, Trivial and Simple words do not vote and each Complex row counts 1 (all of them together count 1 under an explicit `Task:`); review and retry wording keeps full weight. The focus never raises a class, never makes Complex confident, and an unconfident lower read only clears confidence; the risk floor still reads the whole brief. A negated scope word (`do not touch the architecture`, `without migrating`, `não mexa na arquitetura`) no longer votes Complex at any length; phenomena such as deadlock or race condition are never negated. New reason code `TASK_FOCUS`. On the maintainers' labelled sets (1074 items) no item changed; details in `docs/reference/complexity-weights.md`.
- Colour and vocabulary pass on the Claude Code surfaces (owner decision 2026-10-19, `docs/` curation): `savvy-progress` tiers use a grey ramp, the per-role pixel crab costumes give way to one grey crab with a green pixel when done, and the panel shows `small`, `medium` and `frontier`; `filetree` git colours match the `downshift-theme` mod; `downshift-dock` drops the "· panels" suffix and its dot is green only while a panel is open; `downshift-theme` dark `permission` is `#D6DAE2`. The `downshift-local.patch` files are regenerated against the pinned upstream commits.
- Black-first look for every integrated surface (owner decision 2026-10-19): `downshift-dock` band uses theme text, muted `#868a90` chrome and a small green dot; `savvy-progress` runs white, finishes green, pill ink `#0a0a0a` (constants `ACCENT`, `DONE`, `PILL_INK`, `CLAY`); `filetree` dark theme and shimmer tones go white/grey; `cache-tax` cold band and status icons go grey with green for warm. Vendored mods change only through `downshift-local.patch` files plus README and NOTICE entries; `AGENTS.md` and `mods/README.md` updated.

- `downshift setup` no longer turns on Claude Code's built-in You should know mod by default: each check is a billed background request. It prints a suggestion when the key is unset and writes it only with `downshift setup --you-should-know` (a value you set is still never replaced). Supersedes the 2026-10-17 default.
- `downshift context status` and `doctor` now say that on Claude Code the native context provider only measures bytes: a Claude Code hook cannot rewrite tool output, so real compression there needs the Headroom proxy, which Downshift does not start.
- Claude Code plugin session command renamed to `/downshift:status` (`commands/status.md`). Update the plugin and start a new session to load the new name. The optional manual `/downshift` alias remains available by copying the same command file.

### Removed

- Offline simulation gate (`internal/simbench`, `docs/guides/simulation.md`, `make sim`), added earlier in this cycle: removed with the dollar fields (#292).
- Graphify (`internal/graphify`, `DOWNSHIFT_GRAPHIFY_*`), multilingual E5 shadow/fuse (`DOWNSHIFT_E5_*`, `internal/semantic/e5shadow`), and external MiniLM embed commands (`DOWNSHIFT_MINILM_EMBED`, `data/minilm.json`). Semantic boost is the in-process FNV hash embedder only (`DOWNSHIFT_MINILM=0` opts out). `downshift shadow-report` summarises routing v2 candidate observations only.

### Added

- Claude Code mod path (phase 2a): the adapter reads optional `parent_model` as the baseline for inheriting agent types before the transcript; the shared runner records `entry` (`hook` or `mod`) on every decision; `HarnessCapabilities.HookEntries` lists both paths for Claude Code; `stats` counts by entry and `doctor` warns when one `tool_use_hash` carries two decisions. The opt-in `downshift-router` mod always sends `entry: "mod"`. Design: `docs/design/mod-router.md`.
- Claude Code mod path (phase 2b): `downshift-router` listens for `turn.complete` on subagent loops, accumulates API usage per `agentId`, and when the agent is terminal flushes totals through `downshift end claude-code` with inline `usage` (no child transcript). The Claude Code `ParseChildEnd` / SubagentStop parser prefers inline usage over a transcript path; recorder dedup still prices each agent once. On `turn.step`, the mod applies `effort` from the spawn decision: the Claude Code adapter includes `reasoning_effort` in `updatedInput` only when `entry` is `mod` (settings `CanApplyEffort` unchanged). Design: `docs/design/mod-router.md`.
- Harness status vocabulary: added **Beta** as a fourth word (owner decision 2026-10-18) and marked the OpenCode integration Beta (adapter and route wire v1 in #162; its plugin wiring is still in development).

### Experimental

- `mods/downshift-router/`: a standalone, opt-in Claude Code mod (2.1.287+) that routes a subagent's model from the in-process `agent.spawn` event by running `downshift claude-code` over the existing stdin contract and returning `next({ ...e, model })`; off unless `userConfig.enabled` or `DOWNSHIFT_MOD_ROUTER=1`, fails open, never denies, not loaded by the root plugin. Phase 2b also prices finished subagents from `turn.complete` usage via `downshift end claude-code`. Design, the quoted `agent.spawn` fields and two live runs showing that the settings `PreToolUse` hook fires first and that both paths route each spawn when both are installed: `docs/design/mod-router.md`.

- OpenCode project-scoped plugin (F2): `mods/downshift-opencode/` forwards `subagent_spawn_intent` to `downshift opencode`, applies route via `ctx.session.switchModel`, and sends observations (switch_attempt/model_selected/child_context/child_done) to `downshift honor opencode` + `downshift end opencode`. Marker-gated (`.dsspike-switch-enabled`), fail-open, v2-only, project-scoped. E2E: H1-H5 (intent→route→switch ok→context→flush) + negative (gated) + C1-C3 (bin_missing/checksum_mismatch/parse_error) verified; usage seam NOT PROVEN. Install: copy dir + `config.json` 0600 to `.opencode/plugins/downshift-opencode/`. [WIP branch feat/opencode-plugin]
- Catalog: added opencode models `beam-free` (small), `longcat-2.5-preview-free` (mid), `nemotron-3-ultra-free` (frontier) to the embedded catalog (internal/catalog/catalog.json).

### Security

- `install.sh` downloads `checksums.txt` from the same release and verifies the archive's sha256 before installing; a mismatch or a missing checksum file is a hard stop, `DOWNSHIFT_SKIP_CHECKSUM=1` opts out loudly. Before, the one-liner installed whatever the download returned.

### Fixed

- Classifier: Portuguese mirrors of existing English complexity signals and risk phrases (same weight and class), including `reescreva`, `revisão de código`, `distribuído`, `documento de design`, `refatorar`, `implementar`, `depurar`, `controle de acesso`, `cartão de crédito`, `assinatura digital` and `modelo mais barato`. `payment function` and `payment endpoint` take the same floor as `pagamento`. The six-word trivial nudge no longer turns a Complex tie into Trivial. An English "short summary" stays unscored, so a compare-and-summarise task is not a confident small write. Sensor: `TestQAEnPtParity`.
- `stats --export` omits dollar cost fields (`actual_cost_usd`, `real_saved_usd`, `max_session_cost_usd`, `max_subagent_cost_usd`) when nothing measured them, instead of writing literal `0`.
- E5 fuse (opt-in, default off): a lift by the multilingual fuse is logged as `E5_BOOST` and an abstention as `E5_ABSTAIN`, never as `SEMANTIC_BOOST`, and each decision carries a prompt-free `e5_fuse` object (`model_id`, `nearest_label`, `similarity`, `action`, `latency_ms`, `error_code`); the fuse-off hash boost is kept as a floor under the fused class, so enabling the fuse can no longer end a prompt one class below fuse-off; the embed cache folds command whitespace, so `DOWNSHIFT_E5_CMD` and `DOWNSHIFT_E5_SHADOW_CMD` that differ by spacing embed once. Flag off stays byte-identical.
- Self-check: a risk abstention no longer exempts a decision whose tier was lifted afterwards by the semantic boost, so an abstained MEDIUM lifted to frontier with no current model is held by R7 (R9 for mid) instead of written.
- KiroCrew: the policy-mode runner now sets the project directory, so `.downshift/policy.json` floors apply there too.
- `setup`: with the Claude Code plugin enabled, no hooks are written to the settings file (session policy and statusline still are); `doctor` and the install guide describe the one-channel rule. The transcript reader checks the file type before opening it, so a FIFO cannot block a hook.
- `downshift models list` sorts a copy of the catalog: it used to sort the caller's entries in place, so every later reader in the same process (the conformance suite is one) saw a permuted catalog. Sensor: `TestList_DoesNotReorderTheCallersCatalog`.

### Added

- OpenCode route adapter (phase 1): `downshift opencode` reads the wire v1 `subagent_spawn_intent` request with a strict decode and per-field caps (unknown fields dropped), routes through the shared core and answers one allow-listed JSON object (`allow` or `route` with `reasonCode`; a fail-open answer omits `ttlMs`). Invalid input answers `fail_open` and is recorded as `outcome=fail_open`; `DOWNSHIFT_NO_ROUTE`, a session pause and the control group answer `no_route` with the cacheable TTL. `downshift end opencode` parses the `child_done` observation envelope (status and inline usage counts), and the post-spawn honor reader reports a resolved model only from an arrival-ordered switch chain (`switch_attempt` ok, then `model_selected`, then the first `child_context`) compared against the decision target; a batch without the chain records nothing. The token usage seam is **not proven** and stays unobserved, so no cost is recorded for this harness yet, and the OpenCode plugin that forwards the events is phase 2. The catalog carries the zen model ids with no list price (cost fields zero, never a fake `$0`).
- `setup` turns on Claude Code's built-in mod You should know (`cc-plugin-you-should-know@builtin`) the way it sets `opusplan` and plan mode: only when `enabledPlugins` has no entry for it (owner decision 2026-10-17). A `false` you set is kept and reported as `set by you`; `--no-session-policy` skips it; `--undo` restores the backup; `doctor` reports `main session: you-should-know on (...)`. It spends usage: each check is a billed background request to the session's model. The default is data on the setup target (`Parent` + `Key`, here `enabledPlugins`), so no harness is named in the merge code. The `enabledPlugins` key is the one `/plugin enable` is expected to persist; Claude Code's docs name the command, not the settings key, so confirm it on a live session.
- The Claude Code plugin bundles the cache-tax mod (Karan Bansal, MIT, v2.2.1 vendored under `hooks/cache-tax/`, owner decision 2026-10-17): on Claude Code 2.1.287+ a cold send on a context of 50k tokens or more is dropped once with its estimated rewrite price, `/keepwarm` keeps the cache warm through a break with one small model request per 50 idle minutes (it spends usage; armed by you or for three hours after a detected cold write), `/cache-tax` prints the card. `setup` never writes it (a mod is plugin-only); `doctor` warns when `cache-tax@claude-code-mods` is also enabled, since two copies would guard twice. Upstream's 56 tests ship with it (`claude plugin test .`); NOTICE carries the MIT licence.
- Two more vendored Claude Code mods ship as sibling plugins of the Downshift marketplace under `mods/`, one install each (a plugin loads one hooks module and all three hook `session.start`): `savvy-progress@downshift` (johnnyvizz, MIT, v1.2.1: progress bar above the prompt and an `/agents-info` panel of subagents) and `filetree@downshift` (Kurt Buhler, MIT, v0.2.26: file tree pane showing what Claude reads, writes and commits). Upstream suites ship with them (`claude plugin test mods/<name>`); NOTICE carries the MIT licences. `setup` never writes them.
- Companions guide: four Claude Code mods recommended. Replay Theater (Anthropic sample: step through the last turn's edits, no model call) and reflect-mod (claude-reflect, MIT: corrections become `CLAUDE.md` rules after one small Haiku check per short prompt; the saved rules are always-loaded context, so prune them) join the two below. You should know (`cc-plugin-you-should-know@builtin`, built in, off by default) shows one thing you might have missed above the prompt; the section states its cost (a billed background request per check) and when it pays. Token Weather (sample mod, Claude Code 2.1.287+) draws the main session's context window above the prompt; the section says how it complements Downshift's statusline, the install commands (validated on 2.1.302) and why to read a hooks module before installing. `setup` turns You should know on unless you set it yourself (see Added); beyond that Downshift installs and detects none of the four.
- `downshift shadow-report` summarises the multilingual E5 observations from the telemetry log (`events.jsonl`, `--telemetry=` to point elsewhere): observed and scored counts, helper error codes, would-lift rate overall and by regex label, nearest-label distribution, abstain share, a false-downgrade proxy (risk decision, no lift, nearest label below the regex one; a proxy, not a reviewed error), latency p50/p95/max, and the same for the `e5_fuse` trace; `--text` prints the human form, JSON stays the default and is byte-identical when no E5 events exist. This is how E5 is validated in shadow mode on a real log before any enable decision.

### Changed

- Hook sample formerly at the repo root as `hook-input.json` lives under `examples/` as `claude-code-pretooluse-session.json` (class A fixture). Docs, install guide and `examples/run-all.sh` use that path; the public root no longer carries a hook payload.
- Docs repositioning: the README, the Portuguese README, `llms.txt`, the docs index and `when-to-use.md` present Downshift as a suite for optimizing token use in coding agents, not only a router. Each has a one-table view of the pieces with the capability-matrix status word for each (deterministic routing, guardrails, project tier floor, quota and credit gate, session view and statusline, telemetry and reports with a control group, context policy, mods, companions, evidence ladder), a summary of the catalog families per harness with the `explicit_only` rule, the `provider/` prefix strip and session inference on Claude Code, the three language layers (word-hash boost by default, MiniLM opt-in, E5 fuse opt-in with shadow mode and `shadow-report`) with what is and is not measured, and the mods (cache-tax in the plugin, savvy-progress and filetree as sibling plugins, downshift-router experimental) next to the recommended mods and companions. Harness status words are unchanged; the three-arm `opusplan` comparison is listed as not yet reported.
- `setup` no longer writes `autoCompactWindow` (owner decision 2026-10-17; the owner keeps it in their own harness settings). The value `400000` only moves auto-compact on 1M-context models and is a no-op on 200k ones; set it yourself in `~/.claude/settings.json` if you want it. `doctor` no longer reports it.
- Baseline freeze manifest (`docs/evidence/baseline-freeze-2026-10-17.md`, hashes in the labs `eval/FREEZE.json`): every evaluation set with sha256, item count, freeze commit and the reference numbers the next run must reproduce; a frozen set is never tuned against. The seed-accuracy discrepancy is resolved by rebuilding at each commit: 69.0% was the classifier at `be1054ac` (2026-10-06), 73.5% is the classifier from `236c057` (2026-10-12) onward; `benchmark/report.md` now carries 73.5%.
- `docs/guides/evidence.md` gains two sections: the three saving scenarios (same provider smaller model: measured narrowly on mechanical delegated work; same model lower effort: not measured, emission only; cross-provider: not measured, lookup pinned by tests) with the capability, metric and status for each, and where Downshift adds value beyond native `opusplan` and gateways, each claim tied to a test or evidence file or marked unproven.
- Harness status in public docs follows three words (owner decision 2026-10-17): Claude Code is **Completed** (rewrite observed honored, usage and child ends recorded, plugin and session policy), Codex and Cursor are **Functional** (rewrite emitted on real spawns and the conformance suite passes; honoring inferred on Codex, unconfirmed on Cursor), KiroCrew and Antigravity are **Experimental** (policy mode or one observed child; proof of concept) and move out of the first screen into an "Experimental harnesses" section of the README and the install guide. Adapters, tests and examples are unchanged: every feature is still computed for every harness.

### Evaluated

- Multilingual E5 (ONNX INT8) as an opt-in routing candidate: baseline of the current classifier measured and reproducible on M3 (170 items: 34.1% tier accuracy, 19.4% confident coverage, 0 unsafe confident, warm p95 3.2 ms) and the five legacy splits (match the previous labs baseline item by item); the E5 weights are unreachable from the evaluation environment, so accuracy and coverage gains stay unproven; an architecture-identical ONNX graph shows a warm INT8 session at 11 to 70 ms p95 but the fresh-process helper at about 1 s p50 cold, so the latency gate reads BLOCK as wired; three plumbing defects recorded (no `E5_BOOST` reason code, fuse-on can end below fuse-off, embed cache keyed on the raw command). Decision: `DOWNSHIFT_E5_FUSE` stays off, E5 stays a shadow candidate; owner runbook `downshift-labs/scripts/e5_eval.sh`. Evidence: `docs/evidence/e5-int8-evaluation-2026-10-17.md`.

### Changed

- Positioning: Downshift is presented as an optimizer for the cost and efficiency of agent execution; subagent model routing is the main lever, not the whole product. The README first screen now states the problem, the capabilities with a status word each (implemented, not validated everywhere, shadow), how the pieces connect, and what is proven versus not. New docs: `docs/reference/capability-matrix.md` (audit of every capability against code, tests and dated evidence), `docs/guides/claude-code.md` (opusplan and Downshift, baseline inference, emitted vs honored vs used), `docs/guides/evidence.md` (evidence ladder and metrics table; cost per completed task is the primary metric), `docs/guides/benchmark-protocol.md` (fixed Opus vs native opusplan vs opusplan + Downshift), `docs/design/architecture.md` rewritten around the end-to-end flow and ownership of each step, `skills/README.md` (jobs, measured context cost, delegation overhead). No capability was added by this change; several were described more precisely or more modestly.

### Added

- Attribution reads "Downshift by Tiago Vilas Boas" across the repository; the legacy state directory `~/.harness-downshift`, hash salts and schema ids are unchanged for compatibility.
- Claude Code plugin at the repository root: `.claude-plugin/plugin.json` and `marketplace.json`, `hooks/hooks.json` with the same four hooks `setup` writes (each only calls the binary), the `/downshift` command under `commands/`, the skills under `skills/` and one example agent. Install with `/plugin install downshift --marketplace tiagovilasboas/downshift`; the binary is still required. Claude Code only; `setup` stays the path for every other harness.
- `doctor` and `setup` know the plugin (`Plugin` on the setup target): hooks count as wired via the plugin, both channels at once warn that every spawn is routed twice, and `setup` prints the note before writing.
- Skills `downshift-setup` (install or repair hooks on any harness, dry-run first, remove after install) and `cost-reducer` (fixed diagnosis order, then one lever at a time from `references/`: context diet, compress output, project tier floor, misroute triage), next to `downshift-audit`.
- `agents/mechanical-worker.md`, an example subagent with no pinned model, and `docs/guides/subagents.md` on when a subagent is routable.
- `docs/design/architecture.md` gains the packaging layers table (memory, skills, hooks, agents, plugin) and which of them are harness-neutral.
- Companions: `internal/contextopt` distinguishes the in-process native provider from external companions (`contextopt.Companion`, `Config.Companion`). Headroom is the default companion: `downshift context enable` records it when the `headroom` binary is on PATH, `context enable headroom` / `context disable headroom` manage the record, `context status` shows it, and `downshift doctor` prints a `companion` row (warn with the exact fix when detected but not recorded, or recorded but missing). Downshift never installs, launches, wraps or configures the companion, and hooks are unchanged; its metrics stay `unavailable`. Guide: `docs/guides/companions.md`.
- Provider-prefixed ids: tests pin that an Anthropic-family id behind any `provider/` prefix (`cc/claude-opus-4-8`) resolves by the OpenRouter rule, and that ids outside the catalog (`glm/glm-5.2`, `claude/combo/...`) stay unknown so the spawn is left unchanged.
- Docs: Claude Code is named the primary harness (most complete, most verified) in the README, `when-to-use.md`, `install.md` and the Portuguese README; every harness with an adapter still gets every feature. `when-to-use.md` gains a Headroom row; `context-optimization.md` gains a providers-and-companions section.

### Fixed

- Multilingual eval (M3): **unsafe confident recommendations are 0** on the frozen 170-row set after mixed English-helper language guard, bounded delegation risk abstention, and narrow parallel high-stakes patterns; legacy benchmark `cls-after` splits unchanged locally.
- Mixed English trivial stubs with foreign risk prose (`rename this helper; …`) set `UnsupportedLanguage` instead of a confident small-tier route.
- Confident MEDIUM (or SIMPLE) prompts with risk on long agent delegation briefs or M3 parallel payment/permission wording clear confidence via `RiskAbstain` without changing complexity; guardrails treat that as a deterministic lift for R7/R9.
- Claude Code: the small tier is `claude-haiku-4-5`, the model the native `haiku` alias runs (Claude Code 2.1.289, observed 2026-10-15). It was `claude-haiku-5-5` ($0.10/$0.50), so a downshift was recorded at Haiku 5.5 prices, about 10x too low, and the decision note promised "~95% cheaper". The note now shows the list-price saving: about 50% from Sonnet, 75% from Opus. Haiku 5.5 keeps its own catalog entry on its exact id and is never the native small target. A session file that lists only `claude-haiku-5-5` no longer downshifts to Haiku, because Claude Code cannot address 5.5 by alias; add `claude-haiku-4-5` to the list.
- Telemetry: cache-creation tokens are priced at the cache-write price when the catalog has one (Haiku 4.5: $1.25 per 1M, not the $0.10 read price). Claude Code's own cost for one observed Haiku 4.5 run is reproduced exactly.
- Bounded delegation briefs no longer default to COMPLEX by length alone: a prompt of 80+ words with at least two of a scope limit, a test command and a reply budget drops the length point and counts toward MEDIUM; two different COMPLEX keywords still win, and the risk floor is unchanged. On 79 held-out agent-written briefs COMPLEX fell from 25 to 13 and confident MEDIUM rose from 0 to 26; the five benchmark splits are identical item by item.
- Language coverage checks now inspect the full task, including unsupported-language text after the 32 KB scoring cap, and record guard hits even when no scoring signal matches.
- Language-guard abstentions carry the prompt-free `UNSUPPORTED_LANGUAGE` reason code in routing events, `stats` and `why` for every harness.
- Claude Code: with an empty transcript, the bounded wait for the session model ran twice per hook call (about 850 ms); it now runs once.
- A downshift held because its baseline was guessed (Claude Code settings, Cursor parent `model_id`) now carries the reason code `HELD_GUESSED_BASELINE` instead of an unexplained `allow`.
- A prompt that mixes an English trivial verb with a risk written in another language (ES, FR, DE, JA) is no longer downshifted: `Classify` marks prompts with non-Latin script or Spanish/French/German markers unconfident, so the guardrails keep the requested model. Portuguese gains risk terms for access control, vulnerabilities, personal data and prompt injection.
- Cursor: `tool_input.model` of `inherit` now uses the parent `model_id` as the routing baseline, same as a missing model, so trivial spawns downshift instead of logging an unknown baseline.
- Hook failure telemetry (`INVALID_EVENT`, `READ_ERROR`, and other fail-open paths) now records the harness id on each error row so `events.jsonl` can be filtered by adapter.
- Codex: `gpt-6.1-sol` is in the embedded catalog for identity and frontier tier (from `models_cache.json`); list prices stay absent until a vendor snapshot matches exactly, so routing baselines are known while cost telemetry remains unknown for that id.
- Long delegation briefs that are not bounded (fewer than two scope, test, or reply limits) no longer treat negated or quoted TRIVIAL keywords (`no git commit`, `` `git log` ``, `git branch` labels, rename-in-path segments) as one-step jobs.
- `hookctx` timing test: a background helper no longer inherits the hook stdout pipe under `go test ./...`, so CI does not hit the 5 s command timeout on loaded runners.

### Changed

- One hook runner for every harness: `runHookAdapter` takes a wire strategy (rewrite: JSON response, exit 0; policy: deny message on stderr, exit 2). The hand-copied KiroCrew runner is gone, so KiroCrew gets the project floor, session pause (once its payload carries a session id) and the same telemetry path as the other harnesses.
- Shared-layer conformance suite (`cmd/downshift/conformance_test.go`): eight routing scenarios driven through the runner for every registered harness, with payload builders and response decoders as data. The per-adapter copies of those tests (556 lines) are removed; adapter packages keep only wire-specific tests.
- Adapters share one child-status mapper (`telemetry.ChildStatusFromText`, synonym table plus per-harness fallback) and one transcript model reader (`hookutil.TranscriptModel`); Cursor now also maps `succeeded`/`done` and `errored`/`failure`.
- CI: the cross-platform build matrix runs on every Go change, and plugin files trigger the Go job; a parity test keeps `hooks/hooks.json` equal to the setup target.
- Scores refresh backs off for six hours after a refresh that reached no source.
- Docs: BDI agentic router diagram (`docs/img/bdi-agentic-*.svg`) and README section map perception → beliefs → desires → deliberation → intentions → action for every harness adapter.
- Optional `DOWNSHIFT_E5_SHADOW=1` records multilingual E5 ONNX observations on `Route()` without changing tier (Phase D shadow); see `downshift doctor` and initiative doc §14.
- Optional `DOWNSHIFT_E5_FUSE=1` runs the same encoder inside `ClassifyWithSemantic` (Phase E plumbing, default off, fail-open). It does not reuse `DOWNSHIFT_E5_SHADOW_CMD`; reserved centroids `ABSTAIN`/`FOREIGN` may clear confidence only. Owner enable still waits on labs M3 + legacy gates (initiative §15).
- Confident upshifts pick the least session model at the needed tier (same target a downshift of that task would use); guessed baselines and `explicit_upshift` still keep the strongest tier.
- Session lists can carry per-id metadata (tier, supported effort, optional list prices) via `core.ResolveSessionModels`; native or operator order ranks capability but does not invent prices, and a missing rate stays unknown for cost and savings.
- The risk scan runs each category's regex only when the text holds a literal every match needs. Routing a 1 MiB prompt fell from about 2.9 s to 78 ms in the worst measured case; the categories found are identical (differential test against the previous scan).

## [0.1.0-beta.14] — 2026-10-15

### Fixed

- Risk floor reads the full prompt. A risk signal past the 32 KB classification cap no longer lets a trivial head write the small model.
- Risk signals: `drop the users table` (named table), PT-BR phrasings (charging the card, production data, access token, certificate check), and a new `concurrency` category (race, mutex, atomic, deadlock).
- Codex: a decision held by the guardrails (R1, R8) no longer raises `reasoning_effort`; a held change leaves the spawn as requested.
- Claude Code: on a baseline guessed from settings (`BASELINE_FROM_SETTINGS`), a downshift is written only to the small tier; a mid-tier target could be a hidden upshift.
- Telemetry: a rewrite refuted by a linked `resolved` record is not applied, saves nothing and is never inferred honored. Usage priced on an inferred baseline carries `baseline_source: inferred`, and `stats` says how many events rest on it.
- `benchmark` prints confident coverage, accuracy when confident and abstentions apart from errors. Accuracy and gates are unchanged.
- Upshift ranks the session by catalog tier and never writes a model weaker than the current one; an unordered session list once wrote haiku over sonnet for a risky COMPLEX task. Opt-in score routing never picks below the decision tier.
- Risk patterns narrowed (bare race/atomic, `cobra` CLI, course certificates, `base de produtos`) and widened (SQL truncate/`DELETE FROM`, PT table drops, key rotation, thread-safe); `sync.WaitGroup` now matches. The full-prompt risk scan runs once per prompt.
- Cursor: with the parent's `model_id` as baseline, a downshift is written only to the small tier.
- Codex: an unconfident decision changes effort only alongside a model rewrite. Antigravity logs a rewrite only for the child it rewrote and records the written model. KiroCrew's respawn message names the target's tier.
- Telemetry: a refuted rewrite is priced on the model that ran and adds no saving, model-mix entry or frontier-on-doubt count in stats, statusline or session tree. Export adds `real_cost_inferred_baseline`; the session tree and child-end line mark inferred baselines. A spawn with no observed and no requested model is unpriced in both report groups.
- Post hooks: the context-sensor lock gives up after 500 ms, and SubagentStop no longer blocks on a FIFO transcript path.

### Changed

- **Documentation brought up to date with the first measurements.** README gains "What the evidence shows", the quickstart says that `setup` also sets `opusplan` and plan mode (and `--no-session-policy`), and the status names the open beta-exit pillars. New [evidence page](docs/evidence/subagent-routing-ab-2026-10-15.md): routed versus control on real and mechanical tasks, what it supports and does not, the defects the runs exposed. `beta-exit.md` P2, P3 and P4 statuses and the pilot lessons are updated.

## [0.1.0-beta.13] — 2026-10-15

### Changed

- **Claude Code: parallel Agent calls in a session's first assistant message are routed again.** When the transcript has no model yet (4 of 5 spawns per run in 3 runs of 5 parallel Agent calls, all held as `HELD_UNCONFIDENT_UNKNOWN_MID`), an inheriting agent type now falls back to the configured model: `ANTHROPIC_MODEL`, then the `model` setting (`.claude/settings.local.json`, `.claude/settings.json`, user `settings.json`), with `opusplan` and the `sonnet`/`opus`/`haiku`/`fable` aliases resolved through the catalog. The decision carries the new reason code `BASELINE_FROM_SETTINGS`. The `--model` flag and `/model` are not visible to a hook, so the baseline is an inference; a model in the transcript always wins.
- Documented a limit of the Claude Code baseline: several Agent calls in the first assistant message of a session leave all but the last without a baseline (see `docs/harness-matrix.md`).

## [0.1.0-beta.12] — 2026-10-15

### Fixed

- **Claude Code: the first spawn of a session often had no baseline, so it was never routed.** Claude Code writes the transcript asynchronously, and the PreToolUse hook can run before the assistant message that carries the session model is on disk (measured: no assistant line at the hook in 2 of 3 fresh `claude -p` runs, and one 0.5 s later). The decision then read `unknown` and was held (`HELD_UNCONFIDENT_UNKNOWN_MID`). The hook now re-reads an existing transcript for up to 400 ms when it holds no model yet. A missing file is not waited for, so sessions run without persistence (`--no-session-persistence`) are not delayed; they still have no baseline and are not routed.

### Added

- **The subagent type on every Claude Code decision, and unknown baselines counted by type.** Each decision records `agent_type` as a label: a built-in name, `unspecified`, or `custom` (a custom agent's own name is never stored, because `stats --export` is pasted into public issues). `stats --detail` gains "Baseline by agent type" (decisions and unknown baselines per type, most unknown first) and the export gains `agent_types`. It turns "the unknown rate dropped" into a number that can be attributed, such as the `Explore` and `Plan` fix. Events written before this have no type and are not counted per type.

### Added

- **`downshift probe <event>` (maintainer tool).** Wired as a harness hook, it records the field names of a payload, the values of a short list of non-sensitive fields (model ids, tool name, agent type) and an 8-character hash of each id to `probe.jsonl`, and always answers allow. It never records prompts, task text, paths or e-mail addresses. Built to answer, from real payloads, whether Cursor exposes the session model to a hook, which decides the baseline for `inherit` spawns. See `docs/probe.md`.

### Fixed

- **Published adapt numbers refreshed to match the code.** `benchmark/REPORT.md` now reports the classifier and shared-memory floor as 37/40 (was 38/40), with the recomputed move counts, matching the values pinned in `internal/adapt/outcome_eval_test.go`.

## [0.1.0-beta.11] — 2026-10-15

### Fixed

- **Antigravity: a spawn of several subagents paired one child's task with another child's model.** The routing row's decision and requested model come from the last subagent that has a `Prompt`, but its task text (what the training and feedback record stores) came from the first subagent. A payload of a trivial child followed by a heavy one recorded the trivial task with the heavy child's tier. Task text now follows the same child. Single-subagent payloads are unchanged.
- **Claude Code `Explore` and `Plan` spawns were never routed.** The hook inferred the session model only for `general-purpose` and `claude`, so for the built-in `Explore` and `Plan` the baseline stayed `unknown` and the guardrail held every one (`HELD_UNCONFIDENT_UNKNOWN_MID`). They run on the session model (sub-agents docs), so they now inherit it like `general-purpose`. `CLAUDE_CODE_SUBAGENT_MODEL` reaches them only with `CLAUDE_CODE_SUBAGENT_MODEL_FORCE` on, and `Explore` under a Fable session stays `unknown` because Claude Code may run it on Opus there. Custom types and the built-ins with their own model (`statusline-setup`, `claude-code-guide`) are unchanged. Before and after, on a trivial `Explore`/`Plan` task under a Sonnet session: no rewrite, then `haiku`.

## [0.1.0-beta.10] — 2026-10-15

### Changed

- **`downshift help` shows eight commands** (`setup`, `doctor`, `try`, `stats`, `report`, `session`, `config`, `feedback`), one per step of install, check, use and fix. `watch`, `models`, `quota` and `stats --detail`/`--export` move to `downshift help --all`; nothing is removed or renamed. `help --all` now lists `stats --detail`.
- **`downshift help --all` is grouped** into Everyday, More about what happened, Models and quota, Plumbing, Maintainer and Experimental. `downshift setup`, missing from the flat list, is now in it. `context`, `serve` and `dsmon` are marked experimental: kept and unchanged, but not part of the routing pitch, and `downshift watch` is named as the supported live view. No command was removed or renamed.
- **`stats` says what its number is.** The headline ends with "Estimated at list price, not an invoice. Measured against a control group: downshift report".
- **`downshift setup` ends with "What changes for you":** who picks subagent models, the control-group share (and `DOWNSHIFT_CONTROL_SHARE=0` to turn it off), and, only when it was written, the Claude Code session policy with `--no-session-policy` to skip it.
- **`downshift setup` also starts Claude Code sessions in plan mode.** Next to `"model": "opusplan"`, it sets `"permissions": {"defaultMode": "plan"}` when no `defaultMode` is set, so every session plans before it edits and the opusplan split applies. This replaces Claude Code's built-in starting mode (auto mode on recent versions). Existing values are never replaced, and `downshift doctor` reports both. Opt out with `downshift setup --no-session-policy`, which wires the hooks only. See `docs/session-models.md`.

## [0.1.0-beta.9] — 2026-10-15

### Added

- **Main-session policy: opusplan on Claude Code.** `downshift setup` writes `"model": "opusplan"` (Opus in plan mode, Sonnet for execution) when `~/.claude/settings.json` has no model; a pinned model is kept and setup prints how to switch. `downshift doctor` reports it as `main session`. Downshift keeps routing the subagents underneath. See `docs/session-models.md`.

### Changed

- **New installs now get `"model": "opusplan"`.** Plan mode runs on Opus, which costs more than a Sonnet-only session; execution stays on Sonnet. The saving is not measured yet. To opt out, pin a model (`"model": "claude-sonnet-5-5"`, `/model` or `--model`) before running `downshift setup`.

### Fixed

- `downshift setup` no longer adds a second hook when an Antigravity hook already lives inside a named group of `~/.gemini/config/hooks.json`: it repoints the binary path there.

## [0.1.0-beta.8] — 2026-10-14

### Added

- **Control group and `downshift report`.** A hash-chosen share of spawns is left unrouted on purpose (`outcome: baseline`, reason code `CONTROL`) in every shared runner. `downshift report [--days=N] [--json]` compares routed and control spawns by cost per completed task and rework rate, with a 95% bootstrap interval, and prints `not enough data` below 20 completed, priced tasks per group. See `docs/report.md`.
- **Team pilot protocol.** `docs/pilot.md` (two weeks, control share 0.5, Claude Code) and the "Pilot weekly report" issue template.
- `downshift setup [--dry-run]` wires the hooks into every harness found (backup, diff, `--undo`); `downshift doctor` is now a checklist (`--verbose`, `--json`).
- `downshift watch`: the session tree, live. Children with no end after 2 h show `?`.
- `downshift config [--json]`: every effective setting with its value and source.

### Changed

- **The control group is on by default.** `DOWNSHIFT_CONTROL_SHARE` defaults to `0.10`, so about 1 in 10 spawns now runs on the model the harness asked for. Set it to `0` to route every spawn. `DOWNSHIFT_NO_ROUTE=1` and session pauses are unchanged and are not counted as control.
- A stale quota snapshot holds rewrites for at most 1 h past expiry, then counts as absent (required mode keeps holding); `doctor` and the statusline say so.

### Fixed

- `TestSession_FastOnALargeLog` skips its wall-clock bound under `-race`, which failed the scheduled full CI.
- **Silent overspend on upshift.** The "doubt must not spend money" gate in `ShouldRewriteModel` had no production caller, so an unconfident UPSHIFT on a known session list was written (a Haiku parent on `"help"` went to Opus). Guardrail R8 (`R8_UNCONFIDENT_UPSHIFT`) now holds it in `Check`; R7 (`R7_UNCONFIDENT_UNKNOWN_FRONTIER`) holds an unconfident frontier recommendation when no current model is known. Deterministic lifts (risk floor, graph escalation, opted-in NB upshift) stay writable. See `docs/session-models.md`.
- **Semantic boost over-classified to COMPLEX.** The hash-embedder boost lifted prompts with no regex signal to the frontier tier (the word "hook", web research, 8 of 40 trivial eval tasks). The boost is now ignored for no-signal prompts and can never produce COMPLEX without regex evidence. On the recorded outcome runs the pinned classifier floor moves from 38/40 to 37/40; the pin is explicit in `internal/adapt/outcome_eval_test.go`.
- Hard Portuguese work no longer sits on MEDIUM: PT-BR mirror of the COMPLEX vocabulary (arquitetura, migração, redesenho, auditoria de segurança, sem downtime) and of the TRIVIAL/SIMPLE vocabulary, plus PT-BR risk terms (pagamento, reembolso, estorno, segredo, criptografia).

### Added

- **One child-end layer for every harness (N5).** `hookport.ChildEnd` is the shared contract and `internal/childend` the single recorder for `resolved`, `child_ended` and `usage` records (catalog pricing, linking to the decision by tool-call id or session, dedup, the child-end line), so the report, cost per completed task, rework and the tree's status work beyond Claude Code. `downshift end <harness>` reads a harness's post-spawn or stop payload; adapters only parse it (`ParseChildEnd`). Claude Code is moved onto the contract with no behaviour change (`claude-code-post-tool-use` and `claude-code-subagent-stop` stay as aliases). Codex records the end and status from PostToolUse for `spawn_agent`, but no cost: its usage source stays `none` until a real payload proves `transcript_path` is the child's rollout (parent tokens would corrupt cost per completed task), and the recorder refuses usage for any harness whose source is `none`. Cursor `subagentStop` has a status-only parser. Both are unverified until a real payload is captured. New capability data `HarnessCapabilities.ChildEndEvent` and `UsageSource`; `setup` wires the end hook from it; `report` and `doctor` print whether completion is measurable per harness. See `docs/harness-matrix.md`.
- **Session view (U3).** `downshift session [--tree|--why|--pause|--resume] [--session=<id>] [--json]` shows the session's subagents as an aligned tree (model, status, cost, short reason; a loss reads `over $x`, a redo `↻`), explains the latest decision in plain words, and pauses routing for one session (control-group behaviour, reason `PAUSED`, 12 h expiry). Claude Code also gets the `/downshift` command (`.claude/commands/downshift.md`). New capability `HarnessCapabilities.SessionEnv`. See `docs/session-view.md`.
- **Public capability scores (shadow by default).** `downshift models scores` (`--refresh`, `--status`, `--quiet`, `--json`) keeps a local `scores.json` of public scores for the enabled session models of every harness: Epoch AI ECI (CC BY 4.0) and the Artificial Analysis index via OpenRouter (local cache only, never in `catalog.json`). Install, doctor and a hook trigger refresh it at most once a day, in a detached process that never blocks a hook. `DOWNSHIFT_SCORES=off` disables all network. The capability pick is recorded as `score_would`, `score_differs` and `score_would_cost_ratio`; it changes the routed model only with `DOWNSHIFT_SCORE_ROUTING=1` (reason code `SCORE_ROUTED`). See `docs/scores.md`.
- **Inferred session (Claude Code).** With no hook list, native export, discovery cache or operator file, the session list is inferred from the current model: the catalog tier ladder below it, ending at the model itself. Downshift can descend; upshift never passes the current model. New capability `HarnessCapabilities.InferSession` (on for Claude Code only); `DOWNSHIFT_INFER_SESSION=off` disables it. `RewritePlan.Session` carries the planned list to the write guard.
- **Semantic-boost provenance.** `Decision.SemanticLift` and the reason code `SEMANTIC_BOOST` mark a tier raised by the embedder, not a rule. `downshift stats` prints `Frontier decisions N (on doubt, no deterministic lift: M)`, the semantic-boost share, and a WARNING when M is above zero.
- `docs/harness-matrix.md`: what each harness exposes for session discovery and quota (hook payload, model listing, quota source, session-start hook).
- Tests: 178-prompt EN/PT-BR corpus, guardrail truth table, cost sweep, adversarial semantic cases, 63 PT-BR cases, provenance tests.

### Changed

- Guardrail R9 (`R9_UNCONFIDENT_UNKNOWN_MID`): with no known current model and an unconfident classification, a mid-tier recommendation is no longer written (it could be an unnecessary upshift). Confident results and deterministic lifts (risk floor, graph escalation, NB upshift, operator tier floor) still apply. Previously an unrecognised trivial prompt fell to MEDIUM and was written as Sonnet.

- `downshift help` now shows only the commands a user needs, grouped by task; `downshift help --all` lists every command, including maintainer tools. No command was renamed or removed.

### Added

- Decision trace on every routing event: `routing_policy` (`deterministic-v1`) and `reason_codes`, sorted stable codes for the signals and guardrails behind the decision (for example `RISK_FLOOR`, `HELD_UNCONFIDENT_UNKNOWN_MID`, `TIER_FLOOR`, `NO_SIGNAL`). Prompt-free, recorded by the shared runner for every harness, counted in the export. No probability is logged; older events parse unchanged. See `docs/stats-export.md`.
- `downshift stats --by-harness`: Markdown review table of spawns, tier, effort, held-by-guardrail and linked tokens per harness (prompt-free).
- Prompt corpus harness (`internal/core/prompt_corpus_test.go`) reading hand-labelled files from `internal/core/testdata/corpus/`.

- Test that `Route` is a fixed point on its own output (hook coexistence).
- `docs/evidence/vendor-hook-docs-2026-10-11.md`: vendor hook docs compared with the adapters.
- Classifier signals for mechanical edits and commands (EN + PT-BR), refined with a development/control split; see docs/contrib/prompt-corpus.md.
- `adapt.CountPredictor` (shrinkage adapted from BerriAI/litellm, MIT; see NOTICE) and evaluator policy `adapt_counts_loo`. Shadow-only.
- `telemetry.RespawnAfterFailure` and a "Respawned after a failure" line in `stats` (hint only, prompt-free, harness-agnostic). Anchoring on the newest event is inspired by the LiteLLM stall detector (MIT); the code is independent.
- First held-out outcome tasks and a smoke run recorded (see gap-tasks GAP-SUITE).
- Evidence note: Workflow-created agents bypass PreToolUse (GAP-WORKFLOW-AGENTS).
- Test that an empty hook stdin fails open in the shared runner.

### Changed
- **Architecture:** session and quota planning is one shared step (`Decision.PlanSession`); harness-name branches in `quota` and native sessions are capability fields; one harness registry in `cmd/downshift`. No behaviour change for existing harnesses.
- **CLI:** `downshift honor|usage <harness>` no longer appear in `help` (still work; documented in `docs/contrib/agent-reference.md`).
- **Catalog:** Claude Haiku 5.5 (`claude-haiku-5-5`, $0.10/$0.50 per 1M tokens for
  prompts up to 100k; $0.50/$2.50 above) is the Claude Code small tier (KiroCrew keeps `claude-haiku-4-5`: the Kiro account list offers no 5.x model).
  Older `claude-haiku-4-5` ids in `session-models.json` still match through the
  `claude-haiku` family (not an alias: KiroCrew owns that canonical id). The small-vs-frontier price gap is now ~97% (was ~75%) at list price for prompts up to 100k tokens; above that Haiku 5.5 costs $0.50/$2.50 and the gap is ~87%.

### Added
- **Project tier floor:** optional `.downshift/policy.json` raises the minimum tier per project, class or risk category. Raise-only and fail-open; applied once in the core, so every adapter inherits it. `stats` and the export count decisions it raised (`policy_floor`). See `docs/config.md`.
- **How a child ended:** `child_ended` events record a closed status (`completed`, `launched`, `interrupted`, `error`, `stopped`, `unknown`) for Claude Code Agent/Task children, shown in `stats` and the export (`child_status`). The error text is never stored; the event never counts as a decision. Needs `claude-code-post-tool-use` registered under `PostToolUseFailure`; field names are from the vendor docs, not yet captured from a real session.
- **Cost per session and subagent** in `stats` and the export (`cost_sessions`, `cost_subagents`, `max_session_cost_usd`, `max_subagent_cost_usd`); report only, no budget enforcement.
- **`baseline_source`** on cost events (`requested` today).
- **`doctor`** reports model-override variables that a harness declares (`HarnessCapabilities.ModelOverrideEnv`), without claiming the override is verified.
- **`eval-adapt`** adds leave-one-task-out, cost per completed task, `--price-*`, `--ref-in/--ref-out` and `--fail-penalty` (evaluation only; routing unchanged).
- **Skill** `skills/downshift-audit`: read-only audit of hook wiring, stats and fixed context size.
- **Docs:** layering rule (`docs/contrib/layering.md`), agent reference, Portuguese telemetry chapter updated.
- **Session discovery:** `downshift models discover` asks each harness which models the
  account can use (Codex `models_cache.json`, `kiro-cli chat --list-models`, `grok models`,
  the Anthropic Models API with a key) and writes a ranked cache that hooks read after the
  hook payload and `session-models.json`. Newest model per family, ranked by catalog tier
  and price, never by name. Hooks stay offline. See `docs/session-discovery.md`.
- **CLI:** `downshift verification-report [--events=<file>]` prints, per tier, how many
  usage events were verified (`passed`/`failed`/`none`), how many predate the field,
  and the priced cost. Read-only. Coverage counts a missing field as unrecorded, never none.
- **Telemetry:** `verification` (`passed`/`failed`/`none`) on Claude Code `usage`
  events. `SubagentStop` reads the subagent transcript for a recognised test or
  lint command and its `is_error` result; the command and output are never stored.
  It is a local signal, not a quality guarantee: a subagent that runs no check
  reads as `none`.

### Changed
- **Docs:** Slim [README.md](README.md) (hero + quickstart + index). Full install,
  per-harness hooks, dsmon, plan compatibility, and troubleshooting moved to
  [docs/install.md](docs/guides/install.md). [AGENTS.md](AGENTS.md) and [llms.txt](llms.txt)
  point agents at INSTALL. Added [examples/run-all.sh](examples/run-all.sh) adapter smoke.

## [0.1.0-beta.7] — 2026-10-06

### Changed
- **Benchmarks & training ops:** Public repo no longer ships benchmark JSON,
  outcome suite, or `tools/minilm` / `tools/baseline`. Curated metrics:
  [benchmark/REPORT.md](benchmark/report.md). Maintainer eval and regression gates
  run in private **downshift-labs** ([#57](https://github.com/tiagovilasboas/downshift/pull/57)).
- **README / docs:** Messaging aligned with labs split (REPORT, not in-repo outcome
  CI); [BETA-EXIT](docs/reference/beta-exit.md) P4.5 note; `Makefile report` no longer
  overwrites README ([#58](https://github.com/tiagovilasboas/downshift/pull/58) hygiene).
- **Docs:** Maintainer shadow/engineering/v2 long form stubbed in OSS; full copies
  in downshift-labs. classifier-shadow.md shortened (file since removed).
- **CI:** Public `ci` / `ci-full` run `go test` and adapter smoke only (sample
  fixture). Removed obsolete outcome-scope scripts.

### Added
- **Docs:** [content-classification.md](docs/reference/content-classification.md) (A/B/C/D
  inventory, code vs docs policy for competitive content).
- **Docs:** [internal/doc-audit.md](docs/internal/doc-audit.md) (use vs inside vs
  MOAT flags for README, docs, benchmarks, examples).
- **Docs:** [benchmark/EVAL-PRIVATE.md](benchmark/eval-private.md) policy for
  public vs maintainer eval.
- **Docs:** [docs/brand/social-preview.md](docs/brand/social-preview.md) (W1-5 upload steps).

## [0.1.0-beta.6] — 2026-10-06

### Changed
- **Docs:** Maintainer backlog/checklist stubs; competitive sensitivity matrix in
  [publishing-boundaries.md](docs/reference/publishing-boundaries.md); public
  [engineering-loop.md](docs/design/engineering-loop.md) trimmed (detail in
  `docs/internal/engineering-loop-maintainer.md`).
- **Docs:** [publishing-boundaries.md](docs/reference/publishing-boundaries.md) (public vs
  `docs/internal/` vs private). Launch waves and test coverage moved under
  `docs/internal/` with stubs at old paths.
- **Docs (wave 2):** Public capability-router-v2.md condensed (file since removed);
  full design moved to [docs/internal/capability-router-v2-full.md](docs/internal/capability-router-v2-full.md).
  Contributor classifier guide: [docs/contrib/classifier.md](docs/contrib/classifier.md).
  ROADMAP and CONTRIBUTING aligned with Downshift branding.
  `examples/` hook fixtures; GitHub issue template for beta stats exports (P2.3).
- **Rename:** GitHub repository and Go module are `tiagovilasboas/downshift`
  (`go install github.com/tiagovilasboas/downshift/cmd/downshift@latest`). State
  directory: `~/.downshift` with legacy fallback `~/.harness-downshift`; `downshift doctor`.
  See [docs/config.md](docs/guides/config.md).
- **Docs:** README repositioned as **Downshift** (deterministic model router hero),
  plus [docs/when-to-use.md](docs/guides/when-to-use.md) (vs LiteLLM, OpenRouter, direct SDK).
  `llms.txt` and `docs/pt/README.md` aligned.
- **Licence:** project relicensed from BUSL 1.1 to **Apache License 2.0**
  (2026-10-06). Archived BUSL text: `LICENSE-BSL-1.1-ARCHIVE.md`. Contributor
  terms: contributions are under Apache 2.0 (see CONTRIBUTING.md).

### Fixed
- Claude Code: a spawn that requested a model explicitly was never rewritten. The Task schema only accepts the native names (`haiku`/`sonnet`/`opus`), the hook did not recognise them as catalog models and held the spawn as foreign. `native_name` now resolves like an alias, so explicit requests are routed like the full id.
- Claude Code: the hook wrote the full catalog id (`claude-haiku-4-5`) into `updatedInput.model`, but the Task/Agent schema accepts only family names, so the harness rejected the rewrite and blocked the spawn. The adapter now writes the catalog's `native_name` and allows the spawn unchanged when an entry has none.

### Added
- `downshift claude-code-subagent-stop` (SubagentStop hook): prices a finished Claude Code subagent from its own transcript (tokens, API-reported model, real cost) and links it to the routing decision through a hashed agent id. Fail-open, idempotent.
- Claude Code PostToolUse records an `outcome: "resolved"` observation from `tool_response.resolvedModel` (the model the harness chose for the child), linked to the decision and compared through the catalog; `stats` counts a match as honored. The launch payload of an async subagent carries no token usage, so the `usage` path (P3.4) stays idle until a harness reports tokens.
- Catalog field `native_name` (family-level, inherited across versions and by `models pull` overrides) and `core.Model.Native`.
- `RewritePlan.WriteName` and `HarnessCapabilities.StrictModelName`: every adapter writes the plan's name (native name, else id) and a strict harness never writes a target without one. Claude Code is the only strict harness today.
- Opt-in classifier shadow: `DOWNSHIFT_SHADOW_WEIGHTS` records a candidate beside the production recommendation, and `downshift shadow-report` compares those observations with explicit reviewed labels. Guide: `docs/classifier-shadow.md`.

### Changed
- `downshift feedback <id> success` no longer writes an implicit minimum-tier label. Supervised training still requires `--required-tier`.

## [0.1.0-beta.5] — 2026-10-03

### Added
- Telemetry field `rewrite_honored: bool` to distinguish emission from honor.
- Smoke test for adapter golden payloads (E2E verification that hook fires).
- Evidence protocol for manual testing (Codex rewrite honored on real session).
- `downshift stats --days=N --export` for multi-user aggregation without prompts.
- Billing comparison template (`docs/billing-comparison-template.md`).
- Harness matrix (`docs/harness-matrix.md`) tracking adapter status per harness.

### Changed
- Improved guardrail R1 to preserve explicit-only models when already assigned.
- README reorganized for clarity on harness-level limitations vs router readiness.
- Classifier now includes MiniLM semantic scoring (enabled by default).

### Fixed
- Route caching respects per-session model allowlists.
- PostToolUse hook integration for usage event linkage (Claude Code path complete).

## [0.1.0-beta.4] — 2026-10-03

### Added
- `downshift benchmark --report` and `--gate` for offline evaluation.
- CapabilityRouter v2 pipeline: 13-signal extractor, deterministic safety floor, risk-weighted softmax.
- Graphify library for knowledge graph integration (command-fetcher mode).
- `DOWNSHIFT_MINILM=0` to opt out of semantic scoring.

### Changed
- Expanded curated benchmark dataset from 30 to 200 tasks.
- Holdout split (300 tasks) for regression gating, not generalization proof.
- Catalog v2 with version-agnostic family matching.

### Fixed
- OpenRouter model ID normalization (strips `provider/` prefix).

## [0.1.0-beta.3] — 2026-10-03

### Added
- Support for Antigravity (flash_lite, flash, pro).
- Deterministic classifier covering 40+ documented prompts.
- Event schema with cost normalization (CostUSD field).
- Multi-process JSONL lock and rotation policy.

### Changed
- Adapters now return `Task` struct with explicit fields for model/effort rewrite.
- Catalog entries support `routing:"explicit_only"` to prevent auto-downshift.

## [0.1.0-beta.2] — 2026-09-30

### Added
- Initial Claude Code, Cursor, and Codex adapters.
- Embedding in binary via `go:embed` for zero external model list dependencies.
- `downshift try` for local testing.
- `downshift models list|check|pull` for catalog inspection and override.

### Changed
- Classifier signals extracted and tunable in `internal/core/signals.go`.
- Hook output format: correlation ID + decision summary + stderr.

## [0.1.0-beta.1] — 2026-09-21

### Added
- Initial release: harness-downshift router core.
- PreToolUse hook integration for Claude Code.
- Deterministic complexity classifier.
- Tier assignment and model selection.
- Install script and basic CLI.

---

## Development

### Versioning

- **Beta** (v0.1.0-betaX): Router built, adapters tested, harnesses evolving. API may change.
- **Release Candidate** (v0.1.0-rcX): Beta exit criteria met, production-ready.
- **Stable** (v1.0.0+): API stable, harnesses broadly supported, real multi-user evidence.

### Before v1.0.0

Beta exit requires:
1. **Real-spawn verification** (P1): Hook rewrite honored on Claude Code (paid) and Cursor (Pro/Ultra).
2. **Multi-user evidence** (P2): ≥2 external contributors or non-maintainer sessions.
3. **Billing proof** (P3): ≥50 usage events with real provider costs.
4. **Benchmark generalization** (P4): New test set, blind, no tuning leakage.
5. **Harness coverage** (P5): Support matrix up-to-date with no stale TBDs.

See [docs/beta-exit.md](docs/reference/beta-exit.md) for detailed tracking.

---

**License:** Apache License 2.0 (from 2026-10-06). Prior: BUSL 1.1 — see LICENSE-BSL-1.1-ARCHIVE.md
