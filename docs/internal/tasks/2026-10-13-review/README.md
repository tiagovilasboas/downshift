<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Review 2026-10-13: staff-level code review tasks

Source: staff review of the money path (classify, policy, guardrails, session
selection, Claude Code adapter, runner, telemetry). Layering rule applies
(AGENTS.md): behaviour in shared layers, tests at `core.Route`/`runHookAdapter`.

| ID | Severity | Task | Area |
|----|----------|------|------|
| T1 | High | Single `deterministicLift` helper; operator tier floor must not be held by R7/R8; remove dead `ShouldRewriteModel` | core guardrails |
| T2 | High | Pick the target by catalog tier, not list position | core session_select |
| T3 | High | `stats` must be able to show a loss (negative real savings, upshift cost) | telemetry |
| T4 | Medium | Claude Code: respect `CLAUDE_CODE_SUBAGENT_MODEL` as the inherited-model baseline | claudecode adapter |
| T5 | Low | Bound classification input (first 32 KB) | core classifier |
| T6 | Medium | Gate the outcome-eval pin in CI (needs an owner decision) | process |

Rules for every task: no `Co-Authored-By`, no push, header on new Go files,
tests first for behaviour changes, `go test -p 2 ./...` before done.
