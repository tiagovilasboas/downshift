# Configuration and local state

Downshift stores local data under a single **state directory**. No cloud sync.

## State directory resolution

| Priority | Location |
|----------|----------|
| 1 | `DOWNSHIFT_STATE_DIR` (absolute path) |
| 2 | `~/.downshift/` when it exists |
| 3 | `~/.harness-downshift/` when it exists and `~/.downshift` does not |
| 4 | `~/.downshift/` (default for new installs) |

**Migration.** `downshift setup` (never a hook) moves the files of `~/.harness-downshift/`
into `~/.downshift/` when the latter is absent or empty: rename where possible, copy and
remove across devices. It leaves a one-line `MOVED.txt` in the legacy directory and prints
what moved; `--dry-run` only lists it. When `~/.downshift/` already holds files, nothing is
moved and setup names both directories so you can merge by hand. With
`DOWNSHIFT_STATE_DIR` set, setup does not migrate.

Check the effective path:

```bash
downshift doctor
```

## Effective settings: `downshift config`

`downshift config [--json]` prints every effective setting with its value and where
it comes from. It is read-only: it reads what the hooks read and writes nothing.

| Source | Meaning |
|--------|---------|
| `env` | An environment variable set for the process (named in `env` in the JSON) |
| `file` | A file under the state dir, or one an environment variable points at |
| `default` | Nothing configured: the built-in behaviour |
| `legacy` | `state_dir` only: the pre-rename `~/.harness-downshift` |

Settings shown: `state_dir`; `session_models` per harness (native export, discovery
cache, `session-models.json`, inferred from the current model, or none); `quota_mode`;
`quiet`;
`catalog` (embedded or user override); `semantic_boost` and `semantic_embedder`;
`infer_session`. The session lists reflect the files on this machine; a list in a
hook payload overrides them at spawn time.

## Files under the state dir

| File | Purpose |
|------|---------|
| `events.jsonl` | Routing telemetry (no prompt text) |
| `agents.jsonl` | Optional spawn log (dsmon-hook) |
| `catalog.json` | User catalog override (`downshift models pull`) |
| `openrouter.json` | Optional OpenRouter shadow list: recorded as `gateway_would`, never a routing target (below) |
| `session-models.json` | Per-harness session allowlists |
| `cursor-composer-model.json` | Cursor only: last seen composer slug per `session_id` (written by `cursor-touch` and `end cursor`; read for Task baseline when `preToolUse` omits `model`) |
| `exec-events.jsonl`, `exec-skips.jsonl` | `downshift compact` metrics and skip reasons. Past 10 MiB a file moves to `<name>.1` (one generation); readers sum `.1` and the current file |
| `compressor-learning.json` | Compressor learning store; written under `compressor-learning.json.lock` (a busy lock skips the observation after 200 ms) |
| `loop-events.jsonl` | Routing v2 training events |
| `weights.json` | Classifier weight override |

Override paths:

- `DOWNSHIFT_EVENT_LOG` — telemetry JSONL (advanced)
- `DOWNSHIFT_SESSION_MODELS` — session allowlist file

Every variable the routing core reads, with its default, is listed in one place: the doc comment of `internal/core/env.go`. The ones not covered above:

- `DOWNSHIFT_QUOTA_MODE=required` — the quota gate needs fresh evidence and keeps abandoned snapshots holding ([quota.md](quota.md))
- `DOWNSHIFT_DISCOVERY=off`, `DOWNSHIFT_DISCOVERED=<path>`, `DOWNSHIFT_DISCOVERY_TTL=<duration>` — the discovered-models layer, its cache and its freshness (default 24h) ([session-discovery.md](../reference/session-discovery.md))
- `DOWNSHIFT_NB_DOWNSHIFT` was removed on 2026-11-01 with the naive-Bayes opinion code. No hook was ever registered in production, so it never changed a spawn; it is now ignored
- `DOWNSHIFT_EXPLICIT_UPSHIFT` and `DOWNSHIFT_NB_UPSHIFT` were removed on 2026-10-31 with the legacy upshift path (Downshift never raises a tier on its own). They no longer have any effect
- `DOWNSHIFT_INFER_SESSION` was removed on 2026-10-26 with the inferred session layer (discovery replaced it). It no longer has any effect
- `DOWNSHIFT_NO_ROUTE` and `DOWNSHIFT_CONTROL_SHARE` were removed on 2026-11-02 (owner decision: every spawn is routed). They no longer have any effect. Only the per-session pause (`downshift session --pause`) leaves spawns unrouted, and only when you ask for it
- `DOWNSHIFT_CURSOR_NATIVE_FILE=<path>` — Cursor's native session export (each harness that has one declares it in `HarnessCapabilities.NativeSessionEnv`)

Outside the core: `DOWNSHIFT_EXEC=0` (kill switch for the `downshift compact` shell wrapper, on by default: the hook leaves commands unchanged and `downshift compact` stops compacting; the file switch is `"exec": false` in the context config, with no CLI off switch), `DOWNSHIFT_QUIET`, `DOWNSHIFT_SEMANTIC_BOOST` (hash semantic boost; `0` opts out, and the legacy spelling `DOWNSHIFT_MINILM=0` still works), `DOWNSHIFT_LOG_MAX_MB` / `DOWNSHIFT_LOG_MAX_BACKUPS`. `downshift config` prints the effective value and source of most of them. Graphify, `DOWNSHIFT_MINILM_EMBED`, and `DOWNSHIFT_E5_*` were removed on 2026-10-17 and are ignored; `DOWNSHIFT_EXPLICIT_UPSHIFT` and `DOWNSHIFT_NB_UPSHIFT` were removed on 2026-10-31 and are ignored. `DOWNSHIFT_DISCOVERY`, `DOWNSHIFT_DISCOVERY_TTL` are not reported by `config` yet.

CCR (compress-cache-retrieve) is on by default and keeps the raw output of every compacted command under `~/.downshift/ccr/`. `DOWNSHIFT_CCR=0` turns it off completely; `"ccr": false` in `context-optimization.json` (state dir, same file as `"exec"`) keeps only the output of failed runs, which `"ccr_failures": false` or `DOWNSHIFT_CCR_FAILURES=0` also turns off; `DOWNSHIFT_CCR=1` forces every run on. The environment wins over the file. Balanced compaction (on by default) cuts an output over 16 KB once CCR stored its raw output; `"balanced": false` or `DOWNSHIFT_BALANCED=0` keeps outputs lossless. `balanced_threshold_kb`, `balanced_log_head`, `balanced_log_tail`, `balanced_hunk_lines`, `balanced_diff_lines`, `balanced_matches_per_file` and `balanced_match_lines` in the same file change the threshold and the cut (absent or zero keeps each default); `downshift doctor` prints the values in effect. See [ccr.md](ccr.md).

Classifier evaluation switches, for contributors comparing routing precision: `DOWNSHIFT_DOMAIN_SIGNALS=0`, `DOWNSHIFT_DOMAIN_INTERACTIONS=0` and `DOWNSHIFT_DOMAIN_DEDUP=0` each turn off one layer of domain enrichment ([contrib/classifier.md](../contrib/classifier.md)). All three are on by default and `downshift config` does not report them.

## OpenRouter allowlist (optional)

OpenRouter models are gateway rows (`provider: "openrouter"`, full `vendor/model` id).
They are never chosen automatically. Listing a row in `openrouter.json` under the state
dir makes it a shadow row: Downshift records which listed row it would have used, and
never routes a spawn to it. Nothing is listed by default, so the file is off until you
create it.

```json
{
  "version": 1,
  "harness": "opencode",
  "models": [
    {"id": "deepseek/deepseek-v4.1-flash", "tier": "mid", "max_effort": "medium", "zdr": true},
    {"id": "x-ai/grok-x", "tier": "frontier", "max_effort": "high", "zdr": false}
  ]
}
```

- `id`: the full `vendor/model` slug. An id without a slash is rejected.
- `tier`: `small`, `mid` or `frontier`.
- `max_effort`: the highest effort the model may get: `low`, `medium` or `high`.
  Lower levels are allowed too.
- `zdr`: recorded on the row only. It does not block any request yet.
- No price is stored: the row has no cost, so savings are not computed for it.

The file is all or nothing. An invalid JSON, an unknown tier or effort, an id without a
slash, a duplicate id, a file over 256 KiB, or an id that clashes with another row makes
Downshift ignore the whole file and print one warning to stderr. The embedded catalog
stays in force, and a bad file never blocks a spawn.

### Shadow behaviour

A listed row is shadow only. The live route, the rewrite and the model written to the
spawn are the same with and without `openrouter.json`. A spawn that already runs a listed
model keeps it, as for any explicit-only model.

For every decision Downshift records `gateway_would` (the slug) and `gateway_would_tier`
(the decision tier) in the telemetry event, but only when a listed row of that tier exists.
The row of that tier is chosen by the slug in alphabetical order (the product uses no price).
The event carries no prompt, task text or path, and no token or dollar figure is computed for
the row.

`downshift stats --detail` prints one line, and only when at least one decision would have
been served by a listed row:

```text
shadow: would route 12 of 80 spawns to x-ai/grok-x
```

If several slugs appear, the most frequent one is shown with the count of distinct slugs.
The same choice is made for every harness, because the list is a user fact. Remove the file
to stop recording. Use this shadow evidence before any row is made a live target.

## Routing by task class (on by default)

When no pin, profile, rule or tier hint decided a spawn, Downshift reads the task
class. A **decisive** `TRIVIAL` or `SIMPLE` task with no risk category moves down to
the `small` tier; the routing note says `verdict: "class"`. Decisive means the top
class clearly won, a content signal fired and no risk rule lifted it. `MEDIUM` and
`COMPLEX` never move, and the class never raises a tier. A tier floor still applies
after it. On the labelled eval sets no task the class sends to `small` is labelled
above small (`TestClassRoutingNeverRoutesHarderWorkToSmall`). To turn it off:

```json
{ "version": 1, "routing": { "by_class": false } }
```

The project file wins over the user file (`~/.downshift/policy.json`).

Two rules keep it from making things worse:

- **No second downshift after a failure.** When a spawn Downshift moved down in a
  session ended in error or was interrupted in the last 10 minutes, the class does
  not move the next spawn of that session down (`reason: "held_after_failure"`).
  A declared rule, profile or tier hint still decides as usual.
- **Agent default tier.** A squad agent spawned with no model of its own runs on the tier its
  `agents/<name>.md` declares (`default_tier`), only when that is below the inherited model.
  Set `"agent_default_tier": false` in `routing` to turn it off. A named model, profile, rule or
  floor still decides first.
- **Advice instead of an automatic raise.** When the orchestrator asks for the small
  tier on a decisive `COMPLEX` task and nothing else decided, the spawn runs as asked
  and the orchestrator gets a note (Claude Code `additionalContext`) suggesting the mid
  tier if the result falls short. To move such a spawn up one tier, to mid and never
  past it, set `"raise_complex": true` in `routing`. Claude Code writes the raise only
  when the session's model list holds the mid model; otherwise the note is sent.

## Project tier floor (optional)

A project can require a minimum tier with `.downshift/policy.json` in the project
directory or any parent (nearest file wins, at most 8 levels up). A floor only
**raises** a tier; it never lowers one. Applied once, in the shared core, so every adapter inherits it; each adapter only reports where its payload keeps the working directory.

```json
{
  "version": 1,
  "min_tier": "small",
  "min_tier_by_class": { "COMPLEX": "frontier" },
  "min_tier_by_risk": { "payment": "frontier" }
}
```

- Tiers: `small`, `mid`, `frontier`. Classes: `TRIVIAL`, `SIMPLE`, `MEDIUM`, `COMPLEX`.
- Risk categories are the names in `internal/core/risk.go` (for example `secret`, `payment`, `destructive`).
- The highest applicable rule wins.
- Fail-open: a missing, oversized, symlinked or invalid file (unknown field, tier, class or category) is ignored and the spawn routes as if there were no file. The floor still goes through the session allowlist and quota gates, so it cannot select a model the session does not offer.
- The hook runner takes the project directory from the payload, and falls back to the hook process's working directory when the payload has none. Offline commands such as `benchmark` have no project directory and never read a policy file. Where each harness keeps it (evidence level, 2026-10-11):

  | Harness | Payload field | Evidence |
  |---|---|---|
  | Claude Code | `cwd` | Documented by the vendor |
  | Cursor | `cwd`, else first `workspace_roots` | Third-party and forum reports only; `cwd` is reported empty in places |
  | Codex | `cwd` | Third-party guides only |
  | Antigravity | `cwd`, else first `workspacePaths` | IDE docs for `workspacePaths`; CLI `cwd` from a third-party guide |
  | KiroCrew | `cwd` | Search summary of the vendor page; the Crew hook payload has no captured sample |

  Only Claude Code is confirmed. For the others, capture a real payload under `docs/evidence/` before relying on the floor; if the field is absent the floor is looked up from the process directory, so run the hook from the project directory.
- A floor changes nothing when the current model already sits at that tier. Where a harness does not honor model rewrites (see the harness matrix), the floor stops a downshift from being attempted but cannot force the model.

## Squad fan-out cap (optional)

How many independent parts the squad runs in one parallel wave. Default 6.

```json
{ "version": 1, "orchestration": { "max_parallel": 8 } }
```

- Positive integer, 1 to 64, in the project or the user `policy.json` (project wins). An invalid value is ignored, the next layer applies, and the reason is listed under `notes` in the MCP `playbook` answer.
- Precedence: `DOWNSHIFT_MAX_PARALLEL` (user), a cap a harness declares (`HarnessCapabilities.MaxParallel`, none today), the policy file, the default.
- The MCP `playbook` (no name) returns the effective `max_parallel` for every harness: `max`, `when_strained` (halved, minimum 1, for rate limiting or low headroom in `status`) and `source`.

## Workers (MCP `delegate`)

```json
{ "version": 1, "orchestration": { "workers": "read", "max_workers_per_session": 32 } }
```

- `orchestration.workers`: `off`, `read` or `write`. Precedence: env `DOWNSHIFT_WORKERS` (`off`, `read`, `bypass` = write), the user file, the project file, the default. The project file may only lower the mode; a project `write` is ignored with a note. A policy file that exists but cannot be used (a symlink, over the size limit, invalid JSON or an invalid value) sets workers to `off`, and `downshift doctor` shows why.
- Default: `read`, only for worker CLIs with an observed refused write in read-only mode (Claude today). An explicit `read` or `write` from env or the user file enables both claude and codex. On Windows the default is `off`; explicit settings still work.
- `downshift setup --workers off|read|write` writes the key to the user file; `--yes` alone writes nothing about workers. `downshift doctor` shows a `workers` row (mode, agents, source).
- `orchestration.max_workers_per_session`: at most this many jobs per session. Default 32, `0` = no cap (user file only). A project file can only lower it, to a value above 0. `delegate` refuses with `cap` beyond it.
- `orchestration.quota_stop_pct`: the share of a provider's quota window, 1 to 100, at which `delegate` refuses to start workers (code `quota`, naming the provider, used percent and the line). Default 90. Precedence: env `DOWNSHIFT_QUOTA_STOP_PCT`, the project file, the user file, the default. A provider that is rate limited always stops it; an unreadable quota never does. Read on every call.
- `orchestration.budget_tokens`: the most tokens the workers of one MCP session may spend (a whole number above 0; unset = no cap). Once the tokens that finished workers reported reach it, `delegate` refuses to start another wave (code `budget`, naming spent and cap). Precedence: env `DOWNSHIFT_BUDGET_TOKENS`, the project file, the user file, none. A worker whose CLI wrote no usage adds nothing, so an unknown count never blocks. A single wave can carry its own cap with `budget_tokens` on the `delegate` call: once its workers (follow-up turns included) reported that many tokens, `send` and `steer` refuse for them, and `collect` lists the wave with `over_budget`. Read on every call.

## Gateway profile for Claude Code (alias workaround)

Some owners run Claude Code through an OpenRouter-compatible endpoint with a launcher
such as `~/.local/bin/claude-gateway`. This section documents that pattern and its
limits. It is a workaround, not a supported Downshift feature.

The launcher does four things:

1. Exports `ANTHROPIC_BASE_URL` to the gateway endpoint.
2. Reads the gateway key from the OS keychain at launch (on macOS, for example,
   `security find-generic-password -s <service> -w`) and passes it to the process
   environment for that run only. The key is never written to a file, a shell profile
   or the state dir.
3. Maps the aliases to gateway ids with `ANTHROPIC_DEFAULT_HAIKU_MODEL` and
   `ANTHROPIC_DEFAULT_SONNET_MODEL`.
4. Starts `claude` with the launcher's arguments.

Why it is an alias workaround: Claude Code accepts only `sonnet`, `opus`, `haiku` or
`fable` in the subagent `model` field. A gateway id such as `vendor/model` cannot go
there. Downshift therefore records the alias the router chose. The model that actually
ran comes from the `resolved` and usage records, which show the real id reported for
each run. Do not read the alias in the subagent frontmatter as the model that ran.

Warning about the context window: when Claude Code does not recognize a gateway id it
prints `unrecognized_model` at startup and assumes a 200K window. The official page
([model configuration](https://code.claude.com/docs/en/model-config), section "Correct
the window for a gateway or custom model ID") does not name a variable whose only job is
to silence that warning. It says the warning stops once Claude Code recognizes the id or
uses the window you give it. The variable it names for the window is
`CLAUDE_CODE_MAX_CONTEXT_TOKENS` (for example `CLAUDE_CODE_MAX_CONTEXT_TOKENS=400000`).
Setting it sets the window; it is not documented as a silencer. Other remedies the page
lists: `claude update`, a `modelOverrides` entry, or a `[1m]` suffix for a 1M-window model.

Example shape, without secrets (`~/.local/bin/claude-gateway`):

```bash
#!/usr/bin/env bash
# Key is read from the keychain at launch and kept only in this process.
export ANTHROPIC_BASE_URL="https://<gateway-host>/api/v1"   # endpoint from the gateway docs
export ANTHROPIC_AUTH_TOKEN="$(security find-generic-password -s claude-gateway -w)"
export ANTHROPIC_DEFAULT_HAIKU_MODEL="<vendor/small-model-id>"
export ANTHROPIC_DEFAULT_SONNET_MODEL="<vendor/mid-model-id>"
exec claude "$@"
```

Check the variable name your gateway expects for the key; the example uses
`ANTHROPIC_AUTH_TOKEN`. The model ids are the gateway's own; the example leaves them as
placeholders. A `claude-or -p` run that prints no `unrecognized_model` line needs the
window set with `CLAUDE_CODE_MAX_CONTEXT_TOKENS`, or a recognized id. This was not run against a live gateway in this change.

## Migrating from `~/.harness-downshift`

```bash
mkdir -p ~/.downshift
cp -a ~/.harness-downshift/. ~/.downshift/
downshift doctor   # should show state_dir under ~/.downshift
```

Or set `export DOWNSHIFT_STATE_DIR=$HOME/.harness-downshift` until you copy data.
