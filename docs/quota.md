# Observed usage quota

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

Discovery answers which models a harness offers. Quota answers whether a fresh
usage observation permits an available candidate. `session-models.json` remains
an availability configuration; its `quota` member is ignored.

## Gate and freshness

A hook-supplied `usage_quota` snapshot takes precedence over a configured native
export and `quota.json` in the [state directory](config.md). The native export
takes precedence over the canonical cache for that harness. Once either source
is present, model rewrites require `available` evidence for the candidate's exact harness and scope.
`exhausted`, `unknown`, and `stale` hold the model rewrite. A malformed cache or a
cache with no entry for this harness also holds it.

Without a usage source, routing retains its existing behaviour and records quota
as `unknown`. Set `DOWNSHIFT_QUOTA_MODE=required` to hold rewrites in that case too.
This setting never invents credit. Hook `included_models` and
`unavailable_models` marks are intersected with quota evidence and the session
availability list. `explicit_upshift` stays off by default.

Snapshots have an observation time and expiry, with a maximum lifetime of 15
minutes. The CLI defaults to 5 minutes. Future observations, elapsed reset times,
missing percentages and missing required windows cannot authorize a rewrite.
The writer serializes updates, rejects older observations for a harness, and
atomically writes a private file. An abandoned `.lock` directory produces an
explicit writer error; the hook reader does not acquire the writer lock.

Quota is filtered after establishing the task's capability floor. Exhausting a
frontier candidate cannot promote a remaining cheap candidate into a frontier
model. An available percentage is an observation, not a capacity reservation or
a guarantee that the next call incurs no charge.

## Codex native collection

```bash
downshift quota collect --harness codex --transcript /path/to/session.jsonl
downshift quota status --harness codex --model exact-model-id
```

The collector reads a bounded tail of a regular transcript file, extracts native
`event_msg` / `token_count` subscription windows, and preserves the original event
timestamp. Re-reading an old transcript does not refresh its quota. It persists
no prompts, responses, credentials or purchased-credit balance.

When Codex supplies `transcript_path` in the hook event, the adapter collects this
evidence locally for that call. A supplied path that cannot produce valid usage
holds the rewrite. Confirm that the actual consumer supplies this field before
claiming automatic integration; the standalone collector and a simulated hook
are separate evidence.

Fresh output from the Codex usage-limits tool can also be imported:

```bash
downshift quota import --harness codex --source codex-usage < fresh-usage.json
```

This source accepts `rateLimitsByLimitId.codex.primary/secondary.usedPercent`
and `resetsAt`. Shared subscription windows apply to this harness. A separate
bucket with `normalModelSlug` applies only to that exact model; only its reported
windows are used. `credits.hasCredits=false` is not subscription exhaustion.

## Claude Code statusline bridge

For an empty statusline configuration, this optional setting connects native
usage updates to the local cache and displays a compact percentage line:

```json
{
  "statusLine": {
    "type": "command",
    "command": "downshift quota claude-statusline"
  }
}
```

Preserve an existing statusline. Its wrapper can pass the same input to
`downshift quota import --harness claude-code --source claude-statusline` before
rendering its existing output. The bridge does not edit settings automatically.

The [native statusline contract](https://code.claude.com/docs/en/statusline)
publishes `rate_limits.five_hour/seven_day.used_percentage` and `resets_at`.
`context_window.used_percentage` is context occupancy and is never quota. Native
rate limits may be absent before the first response. Missing fresh input does
not renew a previous observation; it expires at its original deadline.

## Cursor pools

```bash
downshift quota import --harness cursor --source cursor-usage < fresh-usage.json
```

The native parser accepts the current-period response fields `plan_usage` with
`auto_percent_used` / `api_percent_used`, `billing_cycle_end`, and
`auto_bucket_models` (or their camelCase forms). Model membership comes from the
reported `auto_bucket_models`, never names or a catalog family. Cursor Models
quota applies only to those exact IDs. Other Models can be computed as the
complement only when a runtime collector supplies `available_models` and
`available_models_complete: true`; these are collector metadata, not fields
published by the current-period endpoint. Otherwise that pool's membership stays
unknown. A percentage without membership cannot authorize any model.

An importer is not a continuous collector. Do not replay a saved native response
as a new observation: native imports assume freshly captured input. Cursor CLI
model listing, dashboard usage, and an authenticated collector each require their
own runtime evidence. See [session-discovery.md](session-discovery.md).

### Hook-side native export

The hook can read a fresh raw Cursor export without running a CLI or touching
credentials. Set `DOWNSHIFT_CURSOR_NATIVE_FILE` in the hook's environment to the
file written by the optional bridge. The hook parses that bounded regular file
on each invocation before the canonical cache. Its
`observed_at` is preserved, so an old export expires instead of being refreshed.
If the configured file is malformed, unreadable or too large, the quota gate
holds the rewrite. The provider still controls the export cadence; Downshift
does not poll Cursor or claim a balance that the payload does not associate with
exact model IDs.

## Operator observations and observability

`quota import --harness H --source normalized` accepts a version-1 snapshot with
`harness`, `observed_at`, `expires_at`, and `windows`. Each window has `id`,
`scope` (`harness` or `models`), optional exact `model_ids`, `used_percent`, and
`resets_at`. This path always records source `operator-normalized`; it does not
authenticate the provider or impersonate a native source.

Decision telemetry records `quota_status` and `quota_source`, without raw source
payloads. `quota status` prints the local snapshot and evaluated status. A model
rewrite emitted by the hook still requires independent executor acknowledgement
before it can be called honored.

Guia inferencial: scope, source and freshness contracts in this document.
Sensors computacionais: validation, the model-write gate, regression tests in CI,
and prompt-free quota telemetry. Eixos: behaviour and architecture fitness.
