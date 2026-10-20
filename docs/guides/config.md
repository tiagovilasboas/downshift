# Configuration and local state

Downshift stores local data under a single **state directory**. No cloud sync.

## State directory resolution

| Priority | Location |
|----------|----------|
| 1 | `DOWNSHIFT_STATE_DIR` (absolute path) |
| 2 | `~/.downshift/` when it exists |
| 3 | `~/.harness-downshift/` when it exists and `~/.downshift` does not |
| 4 | `~/.downshift/` (default for new installs) |

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
`scores` on/off and `score_routing` (shadow or live); `control_share`; `quiet`;
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
| `loop-events.jsonl` | Routing v2 training events |
| `weights.json` | Classifier weight override |
| `scores.json` | Public capability score cache (sources, selected source, rows); see [scores.md](../reference/scores.md) |

Override paths:

- `DOWNSHIFT_EVENT_LOG` — telemetry JSONL (advanced)
- `DOWNSHIFT_SESSION_MODELS` — session allowlist file
- `DOWNSHIFT_INFER_SESSION=off` — disable the inferred session (Claude Code); an unknown session then never rewrites
- `DOWNSHIFT_SCORES=off` — no network for public capability scores: no hook, install or doctor refresh, and the router ignores `scores.json`. Details: [scores.md](../reference/scores.md)
- `DOWNSHIFT_SCORE_ROUTING=1` — let the public-score pick change the routed model (default: shadow only, recorded in telemetry as `score_would`). Details: [scores.md](../reference/scores.md#capability-selection)

Every variable the routing core reads, with its default, is listed in one place: the doc comment of `internal/core/env.go`. The ones not covered above:

- `DOWNSHIFT_QUOTA_MODE=required` — the quota gate needs fresh evidence and keeps abandoned snapshots holding ([quota.md](quota.md))
- `DOWNSHIFT_EXPLICIT_UPSHIFT=1` — an upshift may pick `explicit_only` ids ([session-models.md](../reference/session-models.md#explicit_upshift))
- `DOWNSHIFT_DISCOVERY=off`, `DOWNSHIFT_DISCOVERED=<path>`, `DOWNSHIFT_DISCOVERY_TTL=<duration>` — the discovered-models layer, its cache and its freshness (default 24h) ([session-discovery.md](../reference/session-discovery.md))
- `DOWNSHIFT_NB_UPSHIFT=1`, `DOWNSHIFT_NB_DOWNSHIFT=1` — apply the naive-Bayes second opinion instead of recording it (default: shadow; R1 still holds an unconfident downshift)
- `DOWNSHIFT_CURSOR_NATIVE_FILE=<path>` — Cursor's native session export (each harness that has one declares it in `HarnessCapabilities.NativeSessionEnv`)

Outside the core: `DOWNSHIFT_EXEC=0` (kill switch for the `downshift compact` shell wrapper, on by default: the hook leaves commands unchanged and `downshift compact` stops compacting; the file switch is `"exec": false` in the context config, with no CLI off switch), `DOWNSHIFT_NO_ROUTE=1` (control baseline), `DOWNSHIFT_CONTROL_SHARE` (built-in control group), `DOWNSHIFT_QUIET`, `DOWNSHIFT_MINILM` (hash semantic boost; `0` opts out), `DOWNSHIFT_LOG_MAX_MB` / `DOWNSHIFT_LOG_BACKUPS`. `downshift config` prints the effective value and source of most of them. Graphify, `DOWNSHIFT_MINILM_EMBED`, and `DOWNSHIFT_E5_*` were removed on 2026-10-17 and are ignored. `DOWNSHIFT_EXPLICIT_UPSHIFT`, `DOWNSHIFT_DISCOVERY`, `DOWNSHIFT_DISCOVERY_TTL` and the two `NB_*` switches are not reported by `config` yet.

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
The row is the smallest listed row of the tier: the lowest known list price, and listed rows
have no price, so ties break on the slug in alphabetical order. The event carries no prompt,
task text or path, and no token or dollar figure is computed for the row.

`downshift stats --detail` prints one line, and only when at least one decision would have
been served by a listed row:

```text
shadow: would route 12 of 80 spawns to x-ai/grok-x
```

If several slugs appear, the most frequent one is shown with the count of distinct slugs.
The same choice is made for every harness, because the list is a user fact. Remove the file
to stop recording. Use this shadow evidence before any row is made a live target.

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
- The hook runner takes the project directory from the payload, and falls back to the hook process's working directory when the payload has none. Offline commands such as `benchmark` and `try` have no project directory and never read a policy file. Where each harness keeps it (evidence level, 2026-10-11):

  | Harness | Payload field | Evidence |
  |---|---|---|
  | Claude Code | `cwd` | Documented by the vendor |
  | Cursor | `cwd`, else first `workspace_roots` | Third-party and forum reports only; `cwd` is reported empty in places |
  | Codex | `cwd` | Third-party guides only |
  | Antigravity | `cwd`, else first `workspacePaths` | IDE docs for `workspacePaths`; CLI `cwd` from a third-party guide |
  | KiroCrew | `cwd` | Search summary of the vendor page; the Crew hook payload has no captured sample |

  Only Claude Code is confirmed. For the others, capture a real payload under `docs/evidence/` before relying on the floor; if the field is absent the floor is looked up from the process directory, so run the hook from the project directory.
- A floor changes nothing when the current model already sits at that tier. Where a harness does not honor model rewrites (see the harness matrix), the floor stops a downshift from being attempted but cannot force the model.

## Migrating from `~/.harness-downshift`

```bash
mkdir -p ~/.downshift
cp -a ~/.harness-downshift/. ~/.downshift/
downshift doctor   # should show state_dir under ~/.downshift
```

Or set `export DOWNSHIFT_STATE_DIR=$HOME/.harness-downshift` until you copy data.
