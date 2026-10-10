# Daily truth

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

Downshift routes a subagent. It does not orchestrate a session. The daily
product is three claims. Each claim is absolute only for the spawn whose own
harness hook observed it. Absence is not inference. The hook stdin is the
source of truth for that spawn.

This page sequences the surface. It does not close or replace
[gap-tasks.md](gap-tasks.md), and it does not declare a beta exit.

## Already true

- `internal/hookport` runs optional honor and usage. A nil func returns
  unobserved. The `honor` and `usage` commands print that line and do not
  invent a model or a token count.
- Claude Code registers both funcs. Cursor, Codex, Antigravity, KiroCrew, and
  Grok get a port id and nil funcs.
- Escalation stays in `internal/core`. Adapters map wire formats only.
- Quota evidence has one order for every harness: hook payload, then native
  export, then discovery, then the operator allowlist. The operator file
  `quota` member is ignored. `unknown`, `stale`, and `exhausted` are not
  `available`.
- The report and the dashboard already exclude a held rewrite from applied
  savings. A catalog counterfactual is not a child's token count. Context
  compression records observe-mode bytes and does not replace tool output the
  model already saw.
- The report prints honor, quota, and usage as observed, unobserved,
  available, held, or unknown. A later same-session spawn is not honor.
  `CountInferredHonored` remains for callers that still want the inference;
  the report does not print it as honor.

## Surface

One result per decision, on the report and on hook stderr, for every harness
tab:

| Claim | Values | Observed only when |
|---|---|---|
| Honor | `observed` \| `unobserved` | That harness's Honor func recorded the child model for this decision |
| Quota | `available` \| `held` \| `unknown` | Credit for this spawn was `available` |
| Usage | `observed` \| `unobserved` | That harness's Usage func recorded this child's tokens |

`held` means the rewrite was not applied because credit was exhausted, stale,
or required and missing. `unknown` is not a credited route and does not add
savings. A nil Honor or Usage func is `unobserved`, including Cursor, Codex,
Antigravity, KiroCrew, and Grok. Claude is `observed` only when its existing
port recorded honor or child usage.

## Slices

### 1. Truthful spawn surface — this slice

Show the three values above on hook stderr and on the report, including every
harness tab. Reuse `hookport`, telemetry, and the report. Do not add a
collector.

What changes: the report stops presenting later-spawn inference as honor.
`unknown` and `held` stop adding credited routes and savings. Spawn stderr
prints `honor`, `quota`, and `usage` for that decision. At spawn time honor
and usage stay `unobserved` until that harness's own port records them.

### 2. Honor and usage adapters — blocked

Add a map entry only when that hook documents the field. Do not infer a pool
from a model name. Do not copy Claude fields (`resolvedModel`,
`agent_transcript_path`, `rate_limits`) onto another harness.

- **claude-code:** Honor and Usage already registered (`resolvedModel` on
  PostToolUse; SubagentStop child token usage).
- **codex:** `transcript_path` feeds `quota.CollectCodex` (subscription
  windows), not child token usage; no hook field for the executed child model
  or that child's tokens.
- **cursor:** no hook field documented for the executed child model or that
  child's tokens.
- **antigravity:** no hook field documented for the executed child model or
  that child's tokens.
- **kirocrew:** no hook field documented for the executed child model or that
  child's tokens.
- **grok:** no hook field documented for the executed child model or that
  child's tokens.

### 3. Claude statusline — consumer tested; live observation blocked

The consumer is tested. `SessionList.WithUsageQuota` loads a stored
`claude-statusline` snapshot when the hook payload has no usage quota. A high
`used_percentage` holds the Claude rewrite (quota held, not available). A low
`used_percentage` can leave it available. A `context_window`-only observation
is not subscription quota and does not authorize a rewrite.

Live observation stays blocked until the operator already has a statusline
path. This slice does not install a statusline into `~/.claude/settings.json`
and does not record a live `rate_limits` observation.

### 4. Codex child usage — blocked

`transcript_path` feeds `quota.CollectCodex` (subscription windows). It is
not child token usage. There is no documented hook field for the executed
child model or that child's tokens. Absence stays unobserved
(`ObservedChildTokens` records == 0), not a measured zero (records == 1,
sum == 0).

`TestCodexTranscriptPathIsNotObservedChildUsage` locks that a Codex payload
containing `transcript_path` does not count as observed child usage. This is
not a live child-usage observation.

### 5. End-of-day number — done (this definition)

The number is the sum of observed child tokens (`ObservedChildTokens` /
`observed_child_tokens`). An unobserved harness contributes nothing and is not
shown as a measured zero (`observed_child_usage == 0` means unobserved). Catalog
USD savings stay a list-price counterfactual, not this token sum. This is not an
invoice or provider billing claim.
