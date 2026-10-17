<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Public capability scores

Downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

Downshift keeps a local cache of public capability scores for the session
models each harness offers, and can use them to choose a subagent model by the
capability a task needs. The cache is refreshed from two public sources. The
routing decision itself never makes a network call: it reads the local cache.

Status: shadow by default. The score pick is computed and recorded in
telemetry, and it changes the routed model only with `DOWNSHIFT_SCORE_ROUTING=1`.

## Sources and licences

| Source (`source`) | What Downshift reads | Terms as recorded in the code | Endpoint |
|---|---|---|---|
| `epoch-ai` | The Epoch AI Capabilities Index (ECI) CSV: the `Model` (or `Display name`) and `eci` columns | CC BY 4.0 | `https://epoch.ai/data/eci_scores.csv` |
| `openrouter-aa` | The `intelligence_index` of `benchmarks.artificial_analysis` in the public models list. Model slugs come from OpenRouter; the index is Artificial Analysis data | "OpenRouter public API; Artificial Analysis data, attribution required" | `https://openrouter.ai/api/v1/models` |

Attribution, as printed by `downshift models scores --status`:

- Epoch AI, 'Capabilities & benchmarking', https://epoch.ai/benchmarks (CC BY 4.0)
- Model metadata: OpenRouter (https://openrouter.ai); intelligence index: Artificial Analysis (https://artificialanalysis.ai/). Local cache only.

Rules that apply to both:

- Local cache only. The numbers live in `scores.json` in the state dir. They are
  never copied into `catalog.json`, and never committed to the repository. The
  refresh code writes only `scores.json`.
- Credit both sources wherever the scores are shown or reused.
- Not verified here: the current terms of Artificial Analysis and OpenRouter for
  redistribution. This document grants no redistribution right, and the license
  string in the cache is the one the code records.

### Why one source per comparison

The two sources are on different scales. The comment in
`internal/scoreindex/sources.go` gives about 118-167 for Epoch ECI and about
4-58 for the Artificial Analysis index. Those ranges come from the code comment
and were not re-measured for this document.

Mixing them would make one number meaningless next to the other, so the router
uses one source for a whole comparison. The cache keeps every source's rows side
by side, and `selected_source` names the one whose rows the router may read.

`selected_source` is chosen at refresh time by coverage (`SelectSource`):

- Count the distinct enabled session models, across all harnesses, that each
  source has a row for.
- The source with the highest count wins. A tie goes to the higher-priority
  source, which is Epoch AI (it is listed first).
- A source with no rows is skipped. With no session lists at all, every coverage
  is 0 and the first source that has rows wins.

If the current session lists would select another source, `--status` prints a
`note: ... the next refresh applies it`. Nothing changes until the next refresh.

## The cache: `scores.json`

Location: the state dir (`~/.downshift/` by default, see [config.md](../guides/config.md)).
Written atomically through a temp file in the same directory, mode `0600`.

```json
{
 "version": 1,
 "updated": "2026-10-13T06:00:00Z",
 "selected_source": "epoch-ai",
 "sources": [
  {"name": "epoch-ai", "url": "https://epoch.ai/data/eci_scores.csv", "license": "CC-BY-4.0",
   "attribution": "...", "fetched": "2026-10-13T06:00:00Z", "etag": "\"...\"", "rows": 12, "status": "ok"}
 ],
 "rows": [
  {"id": "claude-haiku-5-5", "harness": "claude-code", "score": 43.0, "source": "epoch-ai",
   "fetched": "2026-10-13T06:00:00Z", "license": "CC-BY-4.0", "url": "https://epoch.ai/data/eci_scores.csv"}
 ]
}
```

The values above are an illustration of the shape, not real data.

- `rows` hold every source. Only rows whose `source` equals `selected_source`
  are served to the router.
- `status` per source is `ok`, `not-modified`, or `error: <reason>`.
- A source that fails, or returns an empty or unusable answer, keeps its previous
  rows, and its status shows the error. An empty answer never replaces a cache.
- If no source succeeds, the file is not touched and the refresh exits with an
  error.
- A file with a `version` other than `1` is ignored.
- Lookups do not check age. A stale cache keeps serving scores until a refresh
  succeeds.

Epoch revalidates with `If-None-Match` using the stored ETag. A `304` keeps the
cached Epoch rows and sets status `not-modified`. OpenRouter is always fetched in
full.

## The id map

`internal/scoreindex/idmap.json` is embedded in the binary. It maps public model
names to the session ids that run them, per harness:

```json
{"name": "claude-haiku-5-5", "epoch": ["Claude Haiku 5.5"],
 "openrouter": ["anthropic/claude-haiku-5.5"],
 "ids": [{"harness": "claude-code", "id": "claude-haiku-5-5"}]}
```

- Names match exactly, case-insensitively. There is no prefix or family guessing.
- Epoch names drop an effort suffix before the lookup (`_low`, `_medium`,
  `_high`, `_xhigh`, `_max`). OpenRouter slugs are matched as they are, so a
  variant slug such as `...:batch` is a different name and is not mapped.
- An enabled session id with no entry is reported as `uncovered` in `--status`.
  It never gets a guessed score.
- The capability path needs a score for every candidate. One uncovered candidate
  makes the path abstain for that decision (see below).

Coverage per harness is listed in [harness-matrix.md](harness-matrix.md#public-score-coverage-capability-path).

### Adding a mapping, with evidence

Only add an id when the public row is clearly the same model:

1. Find the public row: the Epoch `Model` value in the CSV, or the OpenRouter
   `id` in the models list. Record the name, the source, and the date you read
   it.
2. Confirm the session id is the one the harness lists for that model. Do not
   map a different tier, effort or variant under the same name.
3. Add the id to an existing entry, or add an entry with `name`, the source
   names and the `ids`. Names must not collide with another entry. The embedded
   map is validated when it is loaded, and `DefaultIDMap` panics on an invalid
   file, which a unit test is meant to rule out.
4. Run `go test ./internal/scoreindex/`, then `downshift models scores --refresh`
   and `downshift models scores --status`. Check the row appears under the right
   harness.
5. Put the evidence (names, source, date) in the commit or PR description.

A wrong mapping scores a model with another model's benchmark, so treat it as a
defect, not as a gap.

## Commands

`downshift models scores` with these flags:

| Flag | Effect |
|---|---|
| (none) or `--status` | Print the cache path, age, selected source, each source's coverage, each harness's enabled models with score or `uncovered`, and the attributions. |
| `--refresh` | Fetch both sources now and rewrite `scores.json`. Prints one line per source. |
| `--quiet` | No output on success. Used by the background refresh and by `install.sh`. |
| `--json` | Print the status as JSON. With `--refresh`, the refresh prints nothing and then the JSON status. |

Exit codes: a failed refresh exits `1`. A refresh that is disabled (`DOWNSHIFT_SCORES=off`)
or blocked by a running refresh exits `0` with a message (silent with `--quiet`).

Each source request has a 10 s timeout; a whole `--refresh` has 40 s.

## Setup and the daily refresh

Three things start a refresh. Each is best effort and never fails the caller:

- **Install.** `install.sh` runs `models scores --refresh --quiet` after the
  binary is installed, unless `DOWNSHIFT_SCORES=off`. If it fails, the installer
  prints a note only.
- **Doctor.** `downshift doctor` prints a `scores:` line. When the cache is
  missing or stale, it starts a background refresh.
- **Hooks (daily).** After the response is written, every hook run through the
  shared runner (`runHookAdapter`, rewrite and policy mode alike) calls
  `maybeRefreshScores`. It stats `scores.json`. When the file is missing,
  or its modification time is 24 h or older, it creates the lock file
  `scores-refresh.lock` in the state dir and starts a detached
  `downshift models scores --refresh --quiet`. The child gets its own session,
  its stdio goes to the null device, and the hook does not wait for it.

The hook path never waits, never prints and never fails because of the refresh.
Its cost is one `stat`, plus one lock file when a refresh is due.

Lock behaviour, from the code:

- `scores-refresh.lock` is created by the hook and is not removed when the
  refresh finishes. It counts as abandoned after 10 minutes and is then replaced.
  So while the cache stays stale, a hook starts at most one refresh per 10 minutes.
- `scores-fetch.lock` is held by the refresh itself and removed when it ends. It
  stops two refreshes from running together, for example an install and a hook.
- A successful refresh rewrites `scores.json` and restarts the 24 h clock. A
  failing refresh does not write the file, so the hook keeps retrying at the
  lock rate until a refresh succeeds.

## Turning it off

`DOWNSHIFT_SCORES=off` (case-insensitive):

- no hook refresh, no install refresh, no doctor refresh;
- `--refresh` does nothing and exits `0`;
- `scores.json` is not read, so no score is served;
- the capability path abstains, and routing is the catalog-tier path (the same
  as before this feature). Shadow fields are not recorded either.

The switch does not delete `scores.json`.

## Privacy

What leaves the machine: one anonymous `GET` per source, when a refresh runs.

- The Epoch request carries `If-None-Match` with the stored ETag. The OpenRouter
  request has no validator.
- Headers: `User-Agent: downshift/<version> (+https://github.com/tiagovilasboas/downshift)`
  and `Accept`. No API key, no cookie, no authorization header, no request body.
- Not sent: prompts, task text, session ids, enabled-model lists, file paths,
  telemetry or any hashed identifier. The session lists are read locally to pick
  the source and to print the status.
- Redirects are followed only to the same host and scheme, at most three.
- Response size is capped at 4 MiB (Epoch) and 8 MiB (OpenRouter).
- Not avoidable by this code: the two servers see the client IP address and the
  `User-Agent` string, as with any HTTP request. What the providers log is not
  verified here.
- The first outbound call can be started by a hook, when `scores.json` does not
  exist yet and the installer has not run. This is by design, and the only way to
  prevent it is `DOWNSHIFT_SCORES=off`.

## Capability selection

The capability path lives in `internal/core/score_select.go` and is shared by
every harness, through the same `PlanSession` call that the catalog path uses. It
does not parse model names.

For each decision it picks the cheapest candidate that reaches the level the
task tier needs, measured against the session's own range:

```
need = lo + (hi - lo) * f(tier)
f(small) = ScoreFracSmall    = 0.00
f(mid)   = ScoreFracMid      = 0.60
f(frontier) = ScoreFracFrontier = 0.95
```

`lo` and `hi` are the lowest and highest scores among the candidates. The
candidates are the session ids that can be written now: not `inherit`, not
exhausted, not `explicit_only` (the capability path never picks one, even with
explicit upshift on), and inside any closed credit set. Quota is checked at pick
time.

A candidate passes when its score is at least `need` (within `1e-9`) and its quota
allows it. From the passing candidates:

- if every passing candidate has a catalog price, pick the cheapest (input plus
  output price per 1M tokens);
- otherwise pick the first in session order.

The three constants are the only tuning knobs. Their values are hand-set. This
document does not claim that they were derived from a measured evaluation.

### When it abstains

The capability path returns nothing, and the catalog-tier path decides, when:

- no score hook is linked (the score index is not linked into the binary);
- the classification is `COMPLEX`;
- the decision has any risk signal;
- a deterministic lift applies: risk floor, graph escalation, NB upshift, or an
  operator tier floor. These choose a tier on purpose, and a score must not undercut
  them;
- the session list is unknown, or there are fewer than two candidates;
- any candidate has no score;
- no candidate passes;
- the decision is unconfident and the model already running has a score lower than
  the pick. An unconfident decision never moves up from the running model.

Guardrail holds still stop everything, as they do for the catalog path.

### Shadow and live

- **Shadow (default).** For every decision, `RecordScoreWould` stores the pick in
  the decision. The routed model is unchanged. Telemetry records the difference
  (below).
- **Live (`DOWNSHIFT_SCORE_ROUTING=1`).** The same pick replaces the catalog-path
  target, after the normal guardrails. It is applied only when the plan rewrites
  the model and the rewritten id equals the pick. The reason code `SCORE_ROUTED`
  is then added.

The figures in telemetry mean the same in both modes, because the comparison is
always against the catalog-tier path.

### Telemetry

These fields are written to `events.jsonl` on routing events. They carry model
ids and numbers only. No prompt, and no score value.

| Field | Meaning |
|---|---|
| `score_would` | Session model id the capability path would pick. Absent when it abstained or no index is linked. |
| `score_differs` | `true` when that id is not the model the catalog-tier path leaves the child on. |
| `score_would_cost_ratio` | List price of the pick divided by the list price of the catalog-path model, rounded to 4 decimals. Absent when either price is unknown. |

Reason code `SCORE_ROUTED` appears only when the live pick was applied. A shadow
pick is never a reason code.

`downshift stats` prints a `Score pick differs` line when any decision has a pick:
the count of decisions where the pick differs, and the average cost ratio. It
says `(shadow unless DOWNSHIFT_SCORE_ROUTING=1)`.

Not verified: that a live pick improves cost or quality. There is no measurement of
that in this document. Read the shadow figures first.

## Limits

- A score is a benchmark estimate for a model, not a measure of one task.
- Epoch ECI and the Artificial Analysis index are different measures. One
  comparison uses one of them.
- Scores are only as correct as the id map.
- Coverage at runtime depends on whether the public source listed the model at the
  last successful refresh. This document does not check live coverage. Run
  `downshift models scores --status` for the current state.
- Requires network for refresh. Offline, the last cache is used as it is.

## Related

- [session-models.md](session-models.md): the session list and the capability path.
- [config.md](../guides/config.md): the state dir and the environment variables.
- [harness-matrix.md](harness-matrix.md): which session ids are mapped per harness.
