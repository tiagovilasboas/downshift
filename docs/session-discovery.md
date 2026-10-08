# Session discovery

Downshift asks each harness which models the current account can use, instead of
relying only on a compiled-in catalog or a hand-written list. The embedded catalog
stays as metadata (tier, price, family) that ranks what was found.

```bash
downshift models discover              # all harnesses with a source; writes the cache
downshift models discover --dry-run    # report only
downshift models discover --harness=codex,kirocrew --json
```

Hooks never call the network or run a CLI. They read the cache this command writes,
so the routing hot path stays offline and deterministic.

## Order of precedence

1. Hook payload (`session_models` / `available_models`).
2. Operator `session-models.json` (explicit intent, per-session lists, quota marks).
3. Discovered cache (`discovered.json` in the state dir).
4. Otherwise the session is unknown and the hook does not rewrite.

Environment: `DOWNSHIFT_DISCOVERY=off` disables layer 3; `DOWNSHIFT_DISCOVERY_TTL`
(Go duration, default `24h`, `0` = never expires) bounds how old an entry may be;
`DOWNSHIFT_DISCOVERED` overrides the path. An expired entry is an unknown session,
so run `discover` on a schedule (cron, launchd, or a session-start hook).

## Where each harness is discovered from

| Harness | Source | Notes |
|---|---|---|
| codex | `~/.codex/models_cache.json` | Only entries Codex lists in its picker (`visibility: list`). |
| kirocrew | `kiro-cli chat --list-models --format json` | `auto` is Kiro's router, skipped. |
| grok | `grok models` | Works without login. Grok has no catalog tiers, so nothing is usable for routing yet. |
| claude-code | Anthropic Models API | Needs `ANTHROPIC_API_KEY`. Subscription-only sessions have no listing endpoint and are skipped. |
| cursor | none | `cursor-agent --list-models` needs a login and its output format is unverified. Keep using `session-models.json`. |
| antigravity | none | No listing mechanism found. Keep using `session-models.json`. |

A source that fails (not installed, offline, no key) is reported and leaves the
harness's previous cache entry untouched.

## How discovered models are ranked

Capability order comes from the catalog, not from model names: tier first, then
output price. Within one family only the newest model is kept: the provider's
`created_at` when it reports one (Anthropic does), otherwise the numbers inside the
ids, used only to break a tie inside a family. The report lists three groups:

- **tier inherited from its family**: the id is not in the catalog, but its family is.
  It is usable and ranked like its family.
- **superseded**: a newer model of the same family exists.
- **no catalog tier**: reported by the harness, unknown to the catalog (for example a
  new model line, or a version like `gpt-6.1-sol` that the family prefix of
  `gpt-6-sol` does not cover). Never a routing target until the catalog gives it a tier.

## Not done yet

- Catalog as a session fallback. Today an unknown session never rewrites. Writing a
  catalog-only id can block a spawn on a harness that does not offer it (Kiro does not
  offer Haiku 5.5, for example), so any fallback has to be opt-in per harness.
- Cursor and Antigravity sources.
- Assigning a tier to "no catalog tier" models from the discovery report.
