# Session discovery

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

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

1. Hook payload (`session_models` / `available_models`). A present `included_models` or `unavailable_models` array is this call's credit report. The operator file is not a credit source.
2. Discovered cache (`discovered.json` in the state dir), the list recovered from that harness. It beats a handwritten allowlist.
3. Operator `session-models.json` only when nothing was recovered. `quota` in that file is not read.
4. Otherwise the session is unknown and the hook does not rewrite.

Environment: `DOWNSHIFT_DISCOVERY=off` disables the discovered cache (layer 2); `DOWNSHIFT_DISCOVERY_TTL`
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
| cursor | `cursor-agent models`, or verified `agent models` | Uses the CLI's existing login. Parses its account model listing; no login is started and no credentials are read by Downshift. |
| antigravity | none | No supported model-listing CLI/cache contract verified in the installed Google Antigravity 2.21.1. Explicit hook lists or the operator file remain necessary. |

A source that fails (not installed, offline, no existing login/key), returns no
models, times out, or emits malformed/incomplete output is reported and leaves
the harness's previous cache entry untouched. JSON output keeps diagnostics on
stderr. This preserves the entry, not its freshness: the normal cache TTL still
applies.

### Cursor evidence and limits

Cursor documents `agent models` as an account model listing in its [CLI command
reference](https://cursor.com/docs/cli/reference/parameters). `--output-format`
applies to agent print mode, so discovery does not assume a JSON model endpoint.
The installed `cursor-agent` 2026.09.10-fd3934a formatter emits `Available models`,
rows `id - Display (current, default)`, and a `Tip: use ...` footer. The parser
requires a complete listing and rejects unexpected rows instead of caching a
partial set. `cursor-agent` is preferred; the generic `agent` executable is used
only when its `models --help` identifies the Cursor account-listing command.
This matters when another product also installs a command called `agent`.

Live validation on 2026-10-09 confirmed the command exists but returned
`Authentication required`. The failure path and preserved cache were verified;
an authenticated listing was not measured on this machine. Fixtures test the
formatter contract, not a successful account request.

A listed model is available for selection, not proven to have included credit.
Cursor's CLI text does not expose the Cursor Models / Other Models balance or
model-to-pool membership. Do not infer credit from names, provider, catalog
prices, or discovery. Antigravity's launcher has no verified model-list command;
its internal authenticated language-server transport is not a supported source.

### Guia and sensor

| Concern | Guia | Sensor | Eixo |
|---|---|---|---|
| Preserve harness model availability | Documented source contract (inferencial); complete-list parser (computacional) | Parser/CLI/cache tests in `go test ./...` (computacional); source failure diagnostics at each discovery (computacional) | Behaviour / architecture fitness |
| Keep availability separate from credit | Source limitations and precedence above (inferencial) | Discovery cache has availability only; quota requires a separate live input, with source failures reported (computacional) | Behaviour |

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
- An authenticated Cursor listing validation on this machine, and a supported Antigravity source.
- Assigning a tier to "no catalog tier" models from the discovery report.
