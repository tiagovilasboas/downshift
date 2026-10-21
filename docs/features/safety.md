# Safety & Guardrails

Downshift exists to save money, but never at the cost of task success. Multiple
safety layers ensure routing decisions are conservative when uncertain.

## Core Invariants

### 1. Complex Work Never Downgrades

A task classified as COMPLEX always runs on the FRONTIER tier:

```
COMPLEX task → FRONTIER tier (always)
                 ↓
          Never routed to MID or SMALL
```

Complex work includes security reviews, architecture decisions, cross-system
debugging, and anything involving money or data migrations.

### 2. Uncertain Classification Never Costs

When the classifier isn't confident, it routes conservatively:

```
Low confidence → Higher tier (safe)
                    ↓
          Never saves money on uncertain work
```

An uncertain classification might waste tokens (higher tier than needed),
but it never risks task failure (lower tier than needed).

### 3. Risk Keywords Force COMPLEX

Certain domains automatically elevate to COMPLEX regardless of task wording:

| Domain | Keywords |
|--------|----------|
| Security | auth, permission, credential, secret, CVE, vulnerability |
| Data | migration, schema, database alter, DROP, TRUNCATE |
| Money | payment, billing, subscription, refund, charge |
| Critical | production, deploy, rollback, incident |

These "risk floor" keywords bypass normal classification.

## Guardrail Implementation

### Risk Floor

```go
// internal/core/selfcheck.go
func riskFloor(prompt string) bool {
    // Security keywords
    if matches(prompt, securityPatterns) {
        return true  // Force COMPLEX
    }
    // Data keywords
    if matches(prompt, dataPatterns) {
        return true
    }
    // ... etc
}
```

### Confidence Threshold

```go
// internal/routingv2/policy/policy.go
const LowConfidenceThreshold = 0.6

func Decide(probs TierProbabilities, safety Constraint) Tier {
    if probs.Confidence() < LowConfidenceThreshold {
        return conservativeTier(probs)  // Route up, not down
    }
    return probs.Best()
}
```

### Safe Verdict

Every routing decision carries a `SafeVerdict` that adapters must respect:

```go
type SafeVerdict struct {
    AllowDownshift bool  // Can we route to a cheaper tier?
    AllowUpshift   bool  // Can we route to a more capable tier?
    Reason         string
}
```

An adapter that ignores `SafeVerdict` is a bug.

## What Gets Blocked

### Never Downshift

- COMPLEX tasks (always FRONTIER)
- Tasks with risk floor keywords
- Low-confidence classifications
- Tasks in projects with a floor policy

### Never Route

- Foreign models (not in the catalog)
- Models with no tier metadata
- Spawns with explicit model override

## Testing Safety

### Cost Safety Invariants Test

```bash
go test ./internal/core -run TestCostSafetyInvariants
```

This test ensures:
1. COMPLEX → FRONTIER (always)
2. Low confidence → no downshift
3. Risk keywords → COMPLEX
4. Unknown baseline → no routing

### Adversarial Tests

```bash
go test ./internal/core -run TestAdversarial
```

Tests prompt injection and edge cases that might trick the classifier.

## Monitoring Safety

### Under-Tier Events

The telemetry tracks when a task might have been under-powered:

```bash
downshift stats --under-tier
```

An under-tier event means a task was routed to a lower tier and:
- Took multiple retries
- Resulted in user correction
- Had poor outcome signal

### Verification Report

```bash
downshift verification-report --period=7d
```

Shows tasks where the model might have been insufficient based on
post-hoc signals.

## Configuration

### Project Floor

Set a minimum tier for a project in `.downshift/policy.json`:

```json
{
  "floor": "MID"
}
```

Every task in this project gets at least MID tier.

### Disable Routing

For critical work, disable routing entirely:

```bash
DOWNSHIFT_ROUTE=0 downshift claude-code
```

Or in your shell profile for specific projects.

## Recovery

### What If Routing Was Wrong?

If a task fails due to insufficient model:

1. **Retry** — The harness will retry, and the next spawn may route higher
2. **Feedback** — `downshift feedback --task=<id> --outcome=failed`
3. **Adapt** — The adaptive memory learns from failures

### Rollback

If a policy change causes problems:

```bash
downshift config rollback  # Restore previous config
```

## Audit Trail

Every routing decision is logged with:

- Classification signals
- Confidence score
- Risk floor evaluation
- Final tier and model
- SafeVerdict

For compliance or debugging:

```bash
downshift replay --verbose --period=24h
```
