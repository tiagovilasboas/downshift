<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Public score index and capability-based model choice

Goal: Downshift learns, from PUBLIC sources only, how capable each model is, for
every harness's enabled session models (not only Claude), and uses it to choose
the model that fits the task. Refresh at setup and at most once a day.

Hard rules (AGENTS.md): hooks stay offline and zero-LLM; behaviour in shared
layers; harness facts are capability data; test at `core.Route` /
`runHookAdapter`; fail open; no prompts leave the machine; the refresh request
carries no user data; `DOWNSHIFT_SCORES=off` disables all network.

| ID | Task | Area |
|----|------|------|
| S1 | `internal/scoreindex`: public sources, id map, cache, `downshift models scores` | new package + cmd |
| S2 | Setup + daily trigger: detached refresh from the shared hook runner, install.sh, doctor | cmd/downshift, install.sh |
| S3 | Capability selection in `core` (shadow first), `ScoreHook`, telemetry | core + telemetry |
| S4 | Docs | docs, README, CHANGELOG |

Data contract shared by S1 and S3. File `scores.json` in the state dir:
`{"version":1,"updated":"RFC3339","rows":[{"id":"claude-haiku-5-5","harness":"claude-code","score":43.0,"source":"epoch-ai","fetched":"RFC3339","license":"CC-BY-4.0","url":"..."}]}`.
`score` is one comparable 0-100 capability value per model (same source for the
whole comparison). `core.ScoreHook func(harness, id string) (score float64, ok bool)`
is registered by `scoreindex` in `init`, like `TierFloorHook`.
