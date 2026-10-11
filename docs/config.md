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

Override paths:

- `DOWNSHIFT_EVENT_LOG` — telemetry JSONL (advanced)
- `DOWNSHIFT_SESSION_MODELS` — session allowlist file

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
- The hook runner uses the payload's `cwd` field when the harness sends one, and the hook process's working directory otherwise. Offline commands such as `benchmark` and `try` have no project directory and never read a policy file. Which harnesses send `cwd` has not been verified here; if yours does not, run the hook from the project directory.
- A floor changes nothing when the current model already sits at that tier. Where a harness does not honor model rewrites (see the harness matrix), the floor stops a downshift from being attempted but cannot force the model.

## Migrating from `~/.harness-downshift`

```bash
mkdir -p ~/.downshift
cp -a ~/.harness-downshift/. ~/.downshift/
downshift doctor   # should show state_dir under ~/.downshift
```

Or set `export DOWNSHIFT_STATE_DIR=$HOME/.harness-downshift` until you copy data.
