<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- The vendored `savvy-progress` mod draws its bar in the Downshift brand green (`#22C55E`) with brand ink on the percentage pill, instead of upstream's violet. It is the only local patch to the vendored file and is documented in `mods/savvy-progress/README.md`; `downshift setup` installs the mod, so the colours arrive with it.
- `downshift setup` on Claude Code installs the Downshift plugin through `claude plugin marketplace add` and `claude plugin install`, so the hooks and the bundled cache-tax mod arrive with one command. It removes the Downshift hooks it had written into `settings.json` (the plugin carries them; keeping both would route every spawn twice) and falls back to the settings hooks, naming the reason, when `claude` is not on PATH, when a login shell does not resolve `downshift` to the binary running `setup`, or when the install fails. `--no-plugin` keeps the settings hooks.
- `downshift setup` on Claude Code also installs the sibling mods `savvy-progress@downshift` (progress bar above the prompt, `/agents-info` panel of subagents) and `filetree@downshift` with `claude plugin install`, skipping any already enabled. They carry no routing, so they install even when routing falls back to the settings hooks, and a failed install is only a note. `--no-plugin` skips them; `downshift-router` stays opt-in (it would route twice).

### Changed

- `downshift context status` and `doctor` now say that on Claude Code the native context provider only measures bytes: a Claude Code hook cannot rewrite tool output, so real compression there needs the Headroom proxy, which Downshift does not start.
- Claude Code plugin session command renamed to `/downshift:status` (`commands/status.md`). Update the plugin and start a new session to load the new name. The optional manual `/downshift` alias remains available by copying the same command file.

### Removed

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
  in downshift-labs. [classifier-shadow.md](docs/design/classifier-shadow.md) shortened.
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
- **Docs (wave 2):** Public [capability-router-v2.md](docs/design/capability-router-v2.md) condensed;
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
