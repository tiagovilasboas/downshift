# Antigravity applied rewrite — native child evidence

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

Verified on 2026-10-09 with the installed Antigravity **2.21.1** desktop app.
This closes the real-spawn proof for P5.5 for this build and observed child,
with a [PASS in the final local QA gate](final-gap-qa.md).
It does not establish quota pool membership.

## Observed chain

The controlled parent request asked for `invoke_subagent` with `Model: pro`
and a trivial rename prompt. The hook emitted `flash_lite`. The desktop app's
local conversation database independently records:

| Observation | Native record | Value |
| --- | --- | --- |
| Original invocation | Parent `steps[8].step_payload`, protobuf path `5.4.3`, JSON `Subagents[0].Model` | `pro` |
| Invocation after hook | Same row, protobuf path `5.29.3`, JSON `Subagents[0].Model` | `flash_lite` |
| Child identity | Same row, path `140.2.6.2.143.10.1` | Matches the child's `trajectory_meta.cascade_id` |
| Child executor model | Child `executor_metadata[0].data`, path `10.1.28` | `gemini-3.5-flash-lite` |
| Child generation model | Child `gen_metadata.data`, path `1.19` | Ten records report `gemini-3.5-flash-lite` |

The rewritten arguments and the generation metadata belong to the same child
through the native child reference, rather than through timing alone. The model
identifier above comes from native metadata, not from the child's answer or
self-description. Repeated generation rows include streaming observations;
they are **not** ten independent spawns or billable usage events.

The parent executor metadata reports `gemini-3.8-flash-high`. A separate control
invocation retained `pro` and its child executor configuration recorded
`gemini-3.1-pro-low`. The control supplies context only; the applied argument
and generation metadata chain above is the proof of the downshift.

The existing hook event at `2026-10-09T14:30:07.354104Z` records `TRIVIAL`,
`DOWNSHIFT`, final model `flash_lite`, and `outcome: rewrite_emitted`. Its
`requested_model` is `unknown`; the original model is therefore established by
the native invocation record, not that event. No event was rewritten or
retrospectively promoted to `executor_acknowledged`.

## Reproduction and privacy

The [sanitized observation](antigravity-executor-ack.json) includes row digests,
hashed parent/child identifiers, model identifiers, and protobuf field paths.
It excludes raw prompts, conversation IDs, local paths, credentials, and full
database blobs. The original native databases remain local.

The [manual sensor](../../scripts/collect-antigravity-evidence.py) opens two
explicit SQLite paths with `mode=ro`. For this observation, the parent step
index was 8. Resolve the parent and child files from the local conversation
being tested, then run:

```sh
python3 scripts/collect-antigravity-evidence.py \
  --parent-db "$PARENT_DB" --child-db "$CHILD_DB" --step-index 8
```

It requires one requested child, the exact native child reference, a changed
model argument, executor model metadata, and agreeing generation model metadata.
Missing fields, inconsistent models, malformed protobuf or an unmatched child
produce a nonzero exit without a report. It emits only a manual observation;
it does not modify hooks or append automatic honor/usage telemetry.

These protobuf paths are a **private, build-specific storage format**, not an
official supported API. Revalidate after Antigravity updates. The script must
stay outside routing: hook execution still reads bounded offline caches and
must never query the private database or start a provider CLI.

## Remaining gaps

- The alias-to-model resolution observed here is not a permanent mapping for
  every account or build, and does not establish quota pool membership.
- Discovery of parent picker models must not silently populate subagent aliases.
- No actual billing, comparative savings, or automatic completion collector is
  established by this observation.
- Installed Downshift reported `dev`; this evidence describes that installed
  runtime, not verification of a separately released binary. Its executable
  SHA-256 was `b2845410bfd5d3c53593d9cabe6ec28957e520dc23f5749099b036aeb72770fc`.

## Guia and sensor

| Concern | Guia | Sensor | Eixo |
| --- | --- | --- | --- |
| Executor honor | This protocol distinguishes emission, applied arguments, and native generation metadata (inferencial) | Read-only collector requires a linked child and agreeing model fields (computacional) | behaviour |
| Private storage boundary | Build restriction and explicit manual invocation (inferencial) | Malformed/changed schema fails with exit 1; collector stays outside hooks (computacional) | architecture fitness |
| Evidence privacy | Sanitized output contract (inferencial) | Explicit output allowlist of hashes, model IDs and field paths (computacional) | maintainability |
