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

## Files under the state dir

| File | Purpose |
|------|---------|
| `events.jsonl` | Routing telemetry (no prompt text) |
| `agents.jsonl` | Optional spawn log (dsmon-hook) |
| `catalog.json` | User catalog override (`downshift models pull`) |
| `session-models.json` | Per-harness session allowlists |
| `loop-events.jsonl` | Routing v2 training events |
| `weights.json` | Classifier weight override |
| `scores.json` | Public capability score cache (sources, selected source, rows); see [scores.md](scores.md) |

Override paths:

- `DOWNSHIFT_EVENT_LOG` — telemetry JSONL (advanced)
- `DOWNSHIFT_SESSION_MODELS` — session allowlist file
- `DOWNSHIFT_INFER_SESSION=off` — disable the inferred session (Claude Code); an unknown session then never rewrites
- `DOWNSHIFT_SCORES=off` — no network for public capability scores: no hook, install or doctor refresh, and the router ignores `scores.json`. Details: [scores.md](scores.md)
- `DOWNSHIFT_SCORE_ROUTING=1` — let the public-score pick change the routed model (default: shadow only, recorded in telemetry as `score_would`). Details: [scores.md](scores.md#capability-selection)

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
