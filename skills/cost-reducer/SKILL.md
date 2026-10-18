---
name: cost-reducer
description: Reduce what a session spends with Downshift installed, in a fixed order with evidence first. Use only when the user names Downshift, or asks to cut subagent cost or tokens in a project where `downshift doctor` runs. Not for general billing questions. Diagnoses routing, fixed context, tool output and project policy; writes nothing without a yes.
---

# Cost reducer

Estimated, not billed. Downshift prices decisions at list price; on a subscription
plan there is no per-token invoice, only tokens and quota headroom. Never promise a
percentage. Never read or print task prompts. Every change to a file outside the
repository is shown first and waits for the user's yes.

## 1. Diagnose, in this order

Run all four before proposing anything. Record what you observed, not what you assume.

| Step | Command | What it tells you |
|---|---|---|
| Is routing on? | `downshift doctor --json` | failed checks, each with a `fix`; `companion` row says whether Headroom is recorded |
| What did routing do? | `downshift stats --days=7` | decisions, held guardrails, estimated savings; `frontier rewrites on doubt` above 0 is an alarm |
| What loads every session? | `wc -c` on the always-loaded files (section 3 of the audit skill) plus the skill descriptions and MCP tool schemas | fixed context per session, in bytes (tokens ≈ bytes / 4, say it is an estimate) |
| Where does tool output go? | `downshift context status`; `downshift context metrics` | native observe counts; whether a companion compressor is recorded |

If Downshift is not routing at all, stop here and hand over to the `downshift-setup` skill.

## 2. Pick the biggest lever

Order by expected effect, then apply one at a time and re-measure:

1. **Fixed context** when the always-loaded set is large or has content for one domain
   only. [references/context-diet.md](references/context-diet.md)
2. **Tool output** when sessions re-read long test, build or log output.
   [references/compress-output.md](references/compress-output.md)
3. **Project policy** when a low-risk repository keeps spawning at the frontier tier, or a
   high-risk one keeps rework from downshifts. [references/project-policy.md](references/project-policy.md)
4. **Misroutes** when a child failed because its model was too small, or a trivial task kept
   the frontier model. [references/misroute-triage.md](references/misroute-triage.md)

Routing itself needs no tuning here: it is deterministic and already applied. Do not teach
the user to word prompts so the classifier picks a smaller tier; that moves risk, not cost.

## 3. Report

Table: lever | evidence (command and the line you read) | change proposed | expected effect
(direction only) | confidence (observed, inferred, unverified). One change per row. After the
user applies one, re-run step 1 next week; `downshift report` compares routed and control from
their own usage.

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift
