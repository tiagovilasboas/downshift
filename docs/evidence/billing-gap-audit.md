# Billing linkage audit — 2026-10-09

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

## Scope and limits

Read-only local routing metadata and the current CLI export, hermetic tests,
and a bounded read of the 5 matching native Claude Code subagent transcripts.
No billable turns or authentication changes were needed. No prompt contents,
account identifiers, transcript paths or credentials are emitted or stored
in this evidence. This audit does **not** establish provider invoice savings
or completed beta graduation.

## Before the correction

The 7-day metadata audit found 15 usage records and 15 priced records, but only
5 usage references resolved to routing decisions. The 15 usage records had
15 distinct hashed agent identities; the 10 unmatched records were separately
priced usage records, not duplicate agents. Positive catalog savings appeared
on 2 linked records and no unmatched records. There were no recorded
verification results. The log included 2 binary versions.

`Aggregate.UsageLinked` nevertheless counted every `outcome: "usage"`, making
an unmatched usage record indistinguishable from a linked one for the P3.4
threshold. `buildUsageEvent` also substituted the routed model for an unknown
requested baseline: zero savings was conservative, but the baseline pointer
incorrectly represented a known comparison.

## After the correction

Command run from the working source:

```sh
go run ./cmd/downshift stats --days=7 --export
```

Observed at **2026-10-09T14:44:24.493062Z**:

| Export field | Observed value |
|---|---:|
| `total` | 186 |
| `usage_records` | 15 |
| `usage_linked` | 5 |
| `actual_cost_events` | 15 |
| `actual_cost_usd` | 15.432977 |
| `real_cost_events` | 15 |
| `real_saved_usd` | 0.424866 |
| `baseline_no_route` | 0 |

Historical values were preserved. The read-only export did not create events.
The new `usage_records` field retains the previous raw count. `usage_linked`
now requires a reference to a routing decision inside the selected window,
in the same harness and compatible session (missing legacy session is allowed;
conflicting non-empty sessions are rejected). Repeated agent hashes count
once; without an agent hash, event identity deduplicates records. Distinct
agents are not collapsed merely because they reference the same decision.

New usage with a known routed model and unknown requested baseline retains
`actual_cost_usd`, with `baseline_cost_usd` absent. `actual_cost_events` and
`actual_cost_usd` expose the observed token price independently of whether a
baseline comparison exists. The historical `real_cost_events` field retains
its requirement that both price pointers exist, and savings remain zero when
the requested baseline is unknown.

All dollar amounts above are token-derived **catalog list-price estimates**,
not provider invoice amounts. Cached tokens are priced at the full input rate
as a conservative estimate. Baseline and routed estimates reuse the same
observed token counts; the baseline is a counterfactual, not an independently
measured execution. The metadata-only phase could not certify native provenance. The bounded
follow-up below establishes native model/token correspondence for the linked
records; it does not establish invoice charges or provider discounts.

## Bounded native provenance follow-up

At **2026-10-09T14:47:36Z**, filename-only discovery under the native Claude Code
projects directory hashed subagent filename identifiers using the existing
`HashSessionID` prefix. Only filenames matching the 5 linked usage records
were opened. Transcript reads were bounded to the same 32 MiB limit as the
production reader. Assistant usage was deduplicated by message ID, keeping the
last cumulative record; the last reported assistant model was compared with
the usage event. Only aggregate results were emitted:

| Native check | Result |
|---|---:|
| Linked records with agent hashes | 5 |
| Matching native transcript filenames | 5 |
| Native input/output/cache totals match the recorded usage | 5 / 5 |
| Native model exactly matches the recorded final model | 5 / 5 |
| Linked records with positive catalog savings | 2 |
| Positive-savings records with native tokens and model matched | 2 / 2 |
| Unreadable or oversized matching transcripts | 0 |
| Filename discovery limit (2,000) reached | No |

This establishes native provenance of the observed tokens and model for all
5 linked records, including both records behind the nonzero savings estimate.
The requested baseline still comes from the linked routing decision and uses
the same observed tokens as a catalog-priced counterfactual. Historical price
fields were not recalculated or changed. No transcript body, native identifier
or path was copied into the repository.

## Verification and remaining gates

`go test ./internal/telemetry ./internal/adapters/claudecode` and
`go test ./...` passed. Scoped `git diff --check` passed. Tests cover
unmatched usage, orphan references, non-decision references, harness/session
mismatch, references outside the export window, duplicate agent/event
identities, distinct agents linked to one decision, preserved historical cost
aggregates, and actual spend with an unknown or uncatalogued requested model.
The existing linked end-to-end test still verifies routed and requested prices
using the same observed token counts.

- **P3.4 remains open:** 5 validated linked records, below the required 50.
  Do not manufacture paid executions to satisfy the count.
- **P3.5 has technical evidence:** the CLI shows nonzero token-derived catalog
  savings, and both positive-savings records match native tokens/model. This
  does not close P3.4, P3.7, a billing period, or maintainer sign-off. A dashboard
  is not required to demonstrate this token-derived cost gate.
- **P3.7 remains open:** no control-group billing period or completed provider
  comparison is established here.
- Native per-child usage for Codex and Cursor remains an upstream integration
  gap; no shared account balance is presented as child billing evidence.

## Guia and sensor

| Concern | Guia | Sensor | Eixo |
|---|---|---|---|
| Linked evidence cannot be replaced by a raw count | `docs/stats-export.md`, inferencial | Deterministic reference join and deduplication in `Aggregate`, plus regression tests, computacional | behaviour |
| Unknown baseline must stay unknown while observed spend remains visible | `docs/stats-export.md`, inferencial | Adapter end-to-end tests for unknown and uncatalogued requested models; exported actual-cost fields, computacional | behaviour |
| Count contract remains understandable for consumers | Documented `usage_records` migration and retained field names, inferencial | Export and window tests, computacional | maintainability |

These tests run through the repository's existing `go test ./...` CI command
(`.github/workflows/ci.yml`); the current local full suite passed. The CLI export
is the continuous local observability sensor. A remote CI result or a pre-commit
hook execution is not claimed by this audit.
