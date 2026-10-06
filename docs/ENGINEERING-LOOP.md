# Engineering loop readiness

`harness-downshift` by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/harness-downshift

## Current state

Downshift can make a deterministic routing decision at the harness hook boundary
for Claude Code, Cursor, and Codex. It records local routing metadata without
storing task prompts. The adapters share the same routing policy, so safety
changes in `internal/core` apply consistently across all three harnesses.

The runtime now supports a **manual, harness-agnostic feedback loop**. Each
routed task gets an opaque feedback ID. An engineer can mark the result as
success, retry, or failed with `downshift feedback`; only explicit minimum-tier
labels enter training. This records reviewed outcomes without storing prompts.
The loop does not receive completion signals from harnesses automatically.

## Safety gates already in the runtime

- Hook payload reads are limited to 1 MiB. Invalid or oversized input follows
  each harness's fail-open response.
- Local routing event files are written with owner-only permissions.
- A low-confidence decision cannot automatically downgrade the harness's
  selected model. Upshifts and explicit-only model preservation keep their
  existing behavior.
- Training datasets reject unknown labels and empty prompts instead of
  silently treating unknown labels as `MID`.
- Feedback events are append-only in `~/.harness-downshift/loop-events.jsonl`;
  they are separate from the cost telemetry log.
- Candidate weights can be evaluated with `benchmark --compare
  --candidate-weights=...`; evaluation never activates them.
- `DOWNSHIFT_SHADOW_WEIGHTS` records candidate predictions alongside hook
  recommendations without changing them. `downshift shadow-report` compares
  these observations with explicit reviewed labels. See
  [the shadow guide](CLASSIFIER-SHADOW.md) for limitations and the semantic roadmap.
- Train/validation splitting is deterministic and stratified. Weights are
  validated for finite bounded coefficients and saved atomically with private
  file permissions.

## Runtime routing evidence

Every hook invocation receives an opaque `correlation_id`: an allowlisted
hexadecimal ID from the hook payload is preserved, otherwise Downshift creates
one. The routing event at `~/.harness-downshift/events.jsonl` records that ID,
an RFC3339 timestamp, hook source/agent, hashed session identifier, requested
and final model/reasoning effort, verdict/tier, policy and binary versions,
and an outcome/error code. It never records task text, prompts, or secrets.

`outcome: "rewrite_emitted"` means only that Downshift emitted a compatible
hook rewrite. It is **not** proof that a vendor harness accepted or used it.
That needs a harness-specific acknowledgement carrying the same correlation
ID; no such acknowledgement contract is assumed today. Input timeout and parse
failures are recorded as prompt-free error events and fail open, preserving the
subagent spawn.

### Observing the loop live — dsmon

`cmd/dsmon` is a floating terminal widget that tails `events.jsonl` in real
time and renders routing decisions as they arrive:

```bash
go build -o dsmon ./cmd/dsmon && ./dsmon
```

It shows the current harness, the last seven routing decisions (verdicts,
models, tiers, savings), and today's aggregate statistics (events, downshifts,
upshifts, estimated cost savings shown as `~$ est.` with a footnote).
Estimated savings are derived from the
`estimated_savings` fraction in each event and a rough average spawn cost;
they are directionally correct but not provider billing data. Actual token
counts are not available at the hook layer. Events may optionally carry
`input_tokens`/`output_tokens`/`cached_tokens` plus
`actual_cost_usd`/`baseline_cost_usd`; when present, dsmon renders an
additional `real saved` line and `/api/status` exposes
`real_saved_usd`/`real_cost_events`.

Fowler loop: this section is the **guia inferencial** for architecture fitness
and behaviour; the hook E2E JSONL correlation test and fail-open deadline test
are **sensores computacionais** in CI. Local JSONL is a continuous,
prompt-free operational signal, not proof of an effective provider-side model
change.

## Promotion loop

1. Collect routing metadata locally; never collect prompts by default.
2. After reviewing a task, record `success`, `retry`, or `failed` with its
   feedback ID. Add `--required-tier=...` only when an engineer has reviewed the
   minimum required tier. A successful run alone proves sufficiency, not a
   minimal tier.
3. Train a candidate with `downshift train --from-events --output=candidate.json`.
4. Compare candidate weights with the legacy policy on a labelled set that
   was not used to edit signals, using `downshift benchmark <file> --compare
   --candidate-weights=candidate.json`. `benchmark/holdout.json` does not
   qualify: it was tuned against. Track
   unsafe downgrades, missed upshifts, task success, rework, and cost separately.
5. Do not promote v2 onto the hook from this loop. A later promotion would
   require a fresh set that was not used to edit signals, no regression on
   the agreed quality and safety gates, and a retained rollback to
   `core.Route`. That switch is not implemented.

There are no online weight updates from a single run. A hook decision is not
evidence that the selected model succeeded, and an unreviewed label can teach
the router to repeat a bad choice. Outcome integration with harness completion
events remains future work; the current common denominator is explicit review
through the CLI.

## Current promotion decision

The hook path is `core.Route`: legacy regex scoring plus a monotonic semantic boost. `routingv2` is CLI-only (`downshift train`, `downshift benchmark --compare`). It is not promoted and it is not on the hook.

Historical comparison, not the current 500-task split (`benchmark/tasks.json` 200 plus `benchmark/holdout.json` 300): on a small seed, candidate router v2 scored 43.3% tier accuracy versus 70.0% for the legacy router, with unsafe downgrades at 71.4% versus 42.9%. Do not cite those percentages as the result of the 500-task split.

v2 stays off the hook until a fresh comparison on a set that was not used to edit signals. `benchmark/holdout.json` is burned. Reported tier accuracy of 100% there is the regression net after signal edits, not a promotion result.

## Remaining work for a closed-loop system

- Integrate harness completion/retry signals where each harness has a stable
  event contract, while preserving the shared outcome schema and opt-in review.
- Report per-harness outcome rates and sample counts so small feedback samples
  cannot imply more certainty than they support.
- Add per-harness contract checks for model preservation, fail-open behavior,
  and protocol output to CI.
- Establish a promotion threshold for unsafe downgrades and quality regression,
  then require explicit review of candidate weights before release.
