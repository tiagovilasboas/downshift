# Safety & Guardrails

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Downshift saves tokens, and it must not cost task success. Since decision O8
(2026-10-28) the orchestrator LLM chooses the model for every delegation, and Downshift
applies that choice, enforces what you declared, and measures the result. Downshift never
picks a model on its own. This page says what still protects you and what no longer does.

"Zero LLM" means Downshift's own code never calls an LLM and adds no request or token cost to
the hot path. It does not mean "no LLM decides": the orchestrator's choice is applied
deterministically.

## What changed on 2026-10-28

Removed from the product:

- The built-in risk floor and risk veto. Before O8 the guard refused to move a task that
  touched a risk category (secret, authz, crypto, payment, vuln, pii, destructive,
  concurrency, prompt-injection) to a cheaper tier. That rule is gone.
- The built-in default agent-type table (Explore and similar agents on the small tier). Its
  content is now guidance in the `downshift-orchestrator` skill that `downshift setup`
  installs.
- The automatic upgrade after a recorded failure. The orchestrator decides the next model.
- `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1` is no longer written by setup, because it overrides the
  model the orchestrator chose per call.
- The old claims that "complex work never downgrades" and "risk keywords force COMPLEX" as
  Downshift guarantees. The classifier still computes a class and risk categories, but only as
  telemetry and as the key of floors you declare. It does not decide routing.

## The residual risk, stated plainly

A small model can now receive a secret, authentication, payment, cryptography or destructive
task if the orchestrator sends it there, or if a rule or profile you declared does. Downshift
will not stop that unless you declared a floor. The effect is on the quality of the work, not
on permissions: the harness grants the same tool permissions at every tier.

Who owns the risk now: first the orchestrator, guided by the playbook skill (guidance, never
enforced); second you, through floors in `policy.json`. Downshift owns the refusals below and
the measurement.

## What still protects you

| Control | What it does |
|---|---|
| Declared floors | `min_tier`, `min_tier_by_class` and `min_tier_by_risk` in `policy.json` raise a decision to at least that tier. They only raise. |
| Unknown model refused | A model id the harness catalog does not know is never written. |
| `explicit_only` refused | A model marked `explicit_only` is never chosen automatically. |
| Doubt keeps | If Downshift cannot read the request, the requested model is kept. |
| Fail-open | 1 MiB input cap, 2 s read, 1.5 s deadline; a panic or an invalid policy leaves the spawn unchanged. |
| Prompt-free telemetry | Telemetry stores enums, tiers, models, tokens and times. Task text stays in memory and never reaches disk. |
| Tokens only | Savings are reported in tokens, never in dollars. |

## Declare a floor for sensitive work

Put this in `.downshift/policy.json` (project) or your user policy file:

```json
{
  "version": 1,
  "min_tier": "small",
  "min_tier_by_class": { "COMPLEX": "frontier" },
  "min_tier_by_risk": {
    "secret": "frontier",
    "authz": "frontier",
    "payment": "frontier",
    "crypto": "frontier"
  }
}
```

Unknown tiers, classes and risk categories are rejected when the file is read, so a typo cannot
silently disable a floor. Know the limits:

- `min_tier_by_risk` is keyed on a text scan of the prompt. A prompt that avoids the keywords
  is not floored. Only `min_tier` applies to every spawn.
- The floor needs a model the catalog knows. A spawn that names an unknown model may escape it.
- An unreadable or invalid file means no floor (fail-open), not a block.
- A harness that cannot rewrite the model gets the decision computed and recorded, not applied.
  See [harness-matrix](../reference/harness-matrix.md).
- A `policy.json` inside a cloned repository is read like yours. Read a new project's
  `.downshift/policy.json` before trusting it; rules in it can lower a tier.

## Playbooks and prompt injection

Playbook text (`downshift playbook`) is prefixed to a worker's first message, so a shared or
edited playbook file is an instruction channel. Keep playbooks to your own text, and review a
set you copy from someone else before using it. The files live under the state directory
(`playbooks/`, folder mode `0700`, files `0600`). The repository's own `CLAUDE.md` or
`AGENTS.md` wins over a playbook on conflict. Playbooks are never enforced and cannot grant
permissions.

## Status of the MCP delegation server

`downshift mcp serve` is specified, not built. When it exists it is meant to run over stdio
only, with no listener, with the prompt held in memory, and with no permission-bypass flag.
Do not rely on any of that yet. The review is in
[appsec-review-o8](../internal/appsec-review-o8.md).

## Monitoring

```bash
downshift stats --under-tier
```

An under-tier event means a task ran on a lower tier and showed retries, a user correction or
a poor outcome signal. With O8 this is how a bad choice by the orchestrator becomes visible. It
is a measurement, not a block.

## Disable routing

```bash
DOWNSHIFT_ROUTE=0 downshift claude-code
```

## Recovery

If a task fails because the model was too small, choose a larger model in the next delegation
(Downshift does not upgrade for you). To undo a policy change, edit or remove the
`policy.json` entry; routing reads it on every call.

## Audit trail

A routing note records the verdict, the reason, the tier from and to, and the rule or profile
that applied. It never records task text. The `replay` command was removed (2026-10-26). The
routing direction is in [routing v2](../design/routing-v2.md); decision O8 supersedes the parts
of it that describe a built-in risk veto or default table.
