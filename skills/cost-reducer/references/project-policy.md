# Project tier floor

`.downshift/policy.json` in the project (or a parent, nearest wins) sets a minimum tier.
It only raises; it never lowers. It is applied once in the shared core, so every harness
inherits it.

```json
{
  "version": 1,
  "min_tier": "small",
  "min_tier_by_class": { "COMPLEX": "frontier" },
  "min_tier_by_risk": { "payment": "frontier", "secret": "frontier" }
}
```

- Tiers `small`, `mid`, `frontier`. Classes `TRIVIAL`, `SIMPLE`, `MEDIUM`, `COMPLEX`.
  Risk names are those in `internal/core/risk.go` (`secret`, `payment`, `destructive`, ...).
- Invalid or oversized files are ignored (fail open), so validate by eye and with
  `downshift policy check` run from the project directory.
- Use it to stop rework, not to save money: a floor costs tokens. Propose one when
  `downshift stats` shows failed or respawned children on a risky repository, or when a
  low-risk repository needs no floor at all and the user can remove one.
- The floor passes through the session allowlist; it cannot pick a model the session does
  not offer, and it cannot force a model on a harness that does not honor rewrites
  (`docs/reference/harness-matrix.md`).

Show the file before writing it. `docs/guides/config.md` has the full contract.
