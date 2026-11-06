# Next batch (2026-10-21)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Owner decisions taken on 2026-10-21, then the tasks they unblock. Each task is
one PR, with an estimate (S up to 1 h, M 1 to 3 h, L 3 to 6 h of agent work,
review and CI included) and the check that closes it. Continues
[2026-10-21-surgical-roadmap](../2026-10-21-surgical-roadmap/README.md).

## Decisions

- `downshift-dock` leaves the marketplace. The code stays in `mods/downshift-dock/`; it is no longer offered.
- OpenCode: spike first, then fix the gap that keeps every OpenCode decision on "requested model unknown".
- OmniRoute: out of scope. The adapter from #360 stays as it is; no price or list work.
- C3: a requested `sonnet` that ran on `haiku` with verdict OK counts as saved.

## Tasks

| # | Task | Est. | Workstream | Closes when |
|---|------|------|------------|-------------|
| T1 | Remove `downshift-dock` from `.claude-plugin/marketplace.json` and from the docs that tell users to install it | S | docs/CI | the marketplace lists no dock; docscheck green |
| T2 | OpenCode spike: how the OpenCode plugin or payload carries the requested model; write the finding with sources | M | OpenCode | a spike note names the field (or proves there is none) |
| T3 | OpenCode fix: the hook receives the requested model, so a trivial task picks the live allowlisted row | L | OpenCode | `downshift try --harness opencode` on a trivial task picks the live small row; test at the shared layer |
| T4 | C3: a run on a cheaper tier than requested counts as saved even without a rewrite (verdict OK), kept apart in the JSON as `saved_without_rewrite` | M | telemetry | table test on the owner's shape (sonnet requested, haiku ran, OK) |
| T5 | C1: split `held` into `held by guardrail` (R1, R5, R8 doubt or risk) and `upshift not applied`, in the panel JSON and the session header | M | telemetry | QA recompute matches; header shows both |
| T6 | C2: `report` names the price source of each gateway run (`snapshot` or `unknown`), never a tier price for a gateway id | S | telemetry | a gateway run with no snapshot prints `unknown` |
| T7 | E1: Playwright free text after a `✓` line, and a tsc directory with a danger word, come back byte-identical | S | compressor | byte-identical regression tests for both inputs |
| T8 | Investigate the 5 clean `go test` runs over 10 KB that were not compacted on 2026-10-21; fix or document why | M | compressor | a test with the shape that was missed, or a written reason |
| T9 | D1 and D2: document the `claude-or` alias profile and set the context-window variable the Claude Code docs name for gateway ids | S | docs/CI | config.md section; a `claude-or -p` run prints no `unrecognized_model` line |
| T10 | E2: inventory of Compactor families in the docs, with a test that compares the doc table to `FamilyOf` | M | docs/CI | the test fails when a family is added without a doc row |
| T11 | Run `make e2e` in CI | S | docs/CI | a CI job runs the battery on pull requests that touch Go |

Total: 5 S, 5 M, 1 L, roughly 16 to 24 hours of agent work spread over four workstreams.

## Not in this batch

- Refused direct `downshift compact` calls (34 on 2026-10-20 and 21, exit 126): needs a decision between running unknown commands without compaction and listing only allowlisted ones in the steering.
- The Agents panel UI/UX proposal (contrast, 12 px minimum, row spacing, compact height): waiting for the owner's go-ahead.
- OmniRoute price and listing (out of scope), economy labels, DCI hook layering, `adapt_eval`, site domain (owner decisions).
- Issues #103 and #104 (control week, external exports): real usage, not code.

## Order

Wave 1, four agents in parallel, no shared files between workstreams:
compressor (T7, T8), telemetry (T4, T5, T6), docs/CI (T1, T9, T10, T11), OpenCode (T2, then T3).
Wave 2: QA runs `make e2e` and the mod suites on `main` after the merges.
