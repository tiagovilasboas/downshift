# Model prices

Downshift prices its savings estimates with list prices (USD per 1M tokens)
stored in the catalog, `internal/catalog/catalog.json`. This page is the policy
for keeping them honest.

## Policy

1. **Embedded snapshot, offline fallback.** The prices are compiled into the
   binary. The hook never fetches a price: it must work offline and add no
   latency. The catalog carries a top-level `prices_as_of` date (`YYYY-MM-DD`)
   saying when the numbers were last verified.
2. **Weekly sync PR.** `.github/workflows/sync-prices.yml` runs every Monday
   (and on demand: Actions, `sync-prices`, Run workflow). It runs
   `scripts/sync_catalog_prices.py` against the OpenRouter public model list
   (no key, no secrets), runs `go test ./internal/catalog/...`, and, when a
   price or `prices_as_of` changed, pushes `chore/sync-prices` and opens a PR.
   A date-only diff means the prices were re-checked and are unchanged. The PR
   is reviewed and merged by a person; nothing reaches `main` unattended.
3. **Coherence tests guard the data.** `go test ./internal/catalog` fails when a
   routable entry has no input or output price, when an alias carries a
   different price than the model it names, when an Anthropic cache price
   breaks the list multipliers (write 1.25x input; read 0.1x, except 0.05x on
   Opus 5.5 and Sonnet 5.5 and 0.025x on Fable 5.1), or when `prices_as_of` is
   missing or not a date.
4. **Staleness is visible.** `downshift doctor` warns when `prices_as_of` is more
   than 30 days old. Prices only age between releases, so the fix is to upgrade
   to the latest release.
5. **Estimates are labelled.** A saving computed from these prices is an
   estimate at list prices, not an invoice. `downshift try` appends
   `(est., list prices as of <date>)` to the percentage it shows.
6. **You can override.** A user file at `~/.harness-downshift/catalog.json`
   is merged over the embedded catalog by harness and model id, so a negotiated
   or newer price can be set there. `downshift models pull` adds models that a
   provider lists and the catalog lacks, with `tier: unknown` and no price: set
   the tier and prices in that file before relying on them. The override does
   not change `prices_as_of`, which describes the embedded snapshot.

## What the sync touches

Only entries of the `claude-code`, `codex` and `kirocrew` harnesses whose id
equals a source model id exactly (provider prefix dropped, dots read as dashes).
Input, output, cache read and cache write prices are updated; a cache price the
source does not list is kept. Nothing is inferred for an id with no exact match.
If the source cannot be fetched or parsed the script exits non-zero and leaves
the catalog alone, so the workflow run fails visibly.

Run it locally, from a saved copy of the source or live:

```
python3 scripts/sync_catalog_prices.py --dry-run
python3 scripts/sync_catalog_prices.py --snapshot models.json --today 2026-10-19
```

## Repository setting the workflow needs

A PR opened by the `GITHUB_TOKEN` needs **Settings, Actions, General, "Allow
GitHub Actions to create and approve pull requests"**. Without it the workflow
still pushes `chore/sync-prices`, writes the compare link in the job summary,
and fails so the run is noticed.

A PR opened with `GITHUB_TOKEN` does not start `pull_request` workflows, so the
required check `test` would never report. The workflow therefore dispatches
`ci.yml` on `chore/sync-prices` (`workflow_dispatch` runs are exempt from that
rule); its checks attach to the branch head commit, which is the PR head.

## Sources

| Vendor | Verified against |
|---|---|
| Anthropic | https://platform.claude.com/docs/en/about-claude/pricing and the models overview, 2026-10-19 |
| OpenAI (codex) | https://developers.openai.com/api/docs/pricing, 2026-10-19 (OpenAI lists no cache-write price; a codex cache write figure comes from OpenRouter) |
| Others (xAI, Google, Cursor, GLM) | Entered by hand; not verified by the sync unless the id matches the source |
