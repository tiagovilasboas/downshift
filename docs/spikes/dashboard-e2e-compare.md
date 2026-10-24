# Spike: dashboard e2e compares the page to the API

**Status:** in progress
**Created:** 2026-10-24
**Author:** Tiago Vilas Boas

## What is already true

The dashboard is the embedded SPA behind `downshift dashboard`. Playwright
(`dashboard/e2e/dashboard.spec.ts`) loads `?e2e=1` and checks that the shell
renders, tabs change the URL, and a 7s refresh logs no console error. The Go
sensor is `internal/api/e2e_stub_test.go`. Neither one compares a number on
the page to the JSON that produced it.

`?e2e=1` returns two sessions (claude-code 48_000, codex 12_000) and a panel
whose compactor estimate is 90_000 (`internal/api/e2e_stub.go`).

## Gap

A render can stay green while the footer, the tokens tab, or the compactor
tab show a different total than `/api/sessions` and `/api/session/current`.

## Task

| ID | Points | Done when |
|----|--------|-----------|
| E1 | 2 | `dashboard/e2e/compare.spec.ts` reads both APIs, reads the three test ids, asserts equality, writes `test-results/evidence.json` |
| E2 | 2 | `pnpm e2e` green on this machine (Chromium installed, `DOWNSHIFT_BIN` built) |
| E3 | 3 | Same comparison against a temp event log, not only the stub, so a live accounting change cannot drift from the page |

E1 and E3 are `compare.spec.ts` (`?e2e=1`) and `compare-live.spec.ts` (temp
`events.jsonl` plus `exec-events.jsonl`, no stub flag). E2 is `pnpm e2e` with
Chromium and `DOWNSHIFT_BIN`.

## Out of scope

Harness segmentation of `downshift report` is a separate change. Dollar
figures stay out of the dashboard.
