# Task Classification

Downshift classifies every agent task into a complexity level in microseconds, with zero API calls.

> **Status under O8 (2026-10-28).** The classification **does not choose the model any more**: the orchestrator
> does, and Downshift applies that choice within your `policy.json` (see [routing](routing.md)). The
> classifier (complexity class, risk categories) remains as **telemetry**, as the **key of the floors you
> declare**, and in DCI. The built-in risk floor that forced COMPLEX and kept risky work off small models was
> removed on 2026-10-28; sections below that describe a tier mapping or a risk floor as a routing decision
> describe the pre-O8 behaviour.

## The Problem

AI coding agents default to their most capable (and expensive) model for every
task. But most tasks don't need frontier-tier reasoning:

- "rename userId to userID" → trivial
- "add a nullable email field" → simple
- "implement OAuth2 flow" → complex

Using the same model for all three wastes money on the easy ones.

## How Classification Works

### Complexity Levels

Downshift maps tasks to four complexity levels:

| Level | Description | Examples |
|-------|-------------|----------|
| **TRIVIAL** | Mechanical work | rename, format, move files, git ops |
| **SIMPLE** | One isolated change | add a field, fix a typo, small bug |
| **MEDIUM** | Feature across files | implement endpoint, refactor module |
| **COMPLEX** | Architecture/debugging | security review, cross-system debug |

### Tier Mapping

Pre-O8, complexity mapped to model tiers (now only a label; the orchestrator picks the tier):

```
TRIVIAL  →  SMALL tier    (e.g., Haiku, GPT-4o-mini)
SIMPLE   →  SMALL tier
MEDIUM   →  MID tier      (e.g., Sonnet, GPT-4o)
COMPLEX  →  FRONTIER tier (e.g., Opus, o1)
```

This is the "gearbox rule": match capability to need.

## The Classifier

### Deterministic, Not LLM-Based

Classification uses pattern matching, not another LLM call:

1. **Regex patterns** — ~100 weighted patterns for task signals
2. **Structural rules** — word count, code references, file mentions
3. **Risk categories** — security/data/money keywords are recorded (they no longer force COMPLEX)
4. **Domain signals** — architecture, migration, debugging

The classifier processes 32KB of prompt in <1ms.

### Pattern Examples

```
"rename"           → TRIVIAL signal
"add field"        → SIMPLE signal
"implement"        → MEDIUM signal  
"security review"  → COMPLEX signal (risk floor)
"migrate database" → COMPLEX signal
```

Signals are weighted and combined. The highest-scoring complexity wins,
with ties breaking toward higher complexity (safe side).

### Risk categories (the built-in floor was removed)

Pre-O8, certain keywords automatically elevated to COMPLEX regardless of other signals:

- **Security**: auth, permission, credential, secret, vulnerability
- **Data**: migration, schema change, database alter
- **Money**: payment, billing, subscription, refund
- **Architecture**: system design, cross-service, distributed

These categories are still detected and recorded as telemetry. They no longer force a tier. Declare a floor in
`policy.json` if a project needs one.

## Confidence and Safety

### Low Confidence Handling

When the classifier isn't confident:

```
High confidence → route to classified tier
Low confidence  → route conservatively (higher tier)
```

An uncertain classification never saves money. It either routes correctly
or errs on the side of capability.

### Cost Safety Invariants

Two invariants protect against bad routing:

1. **Complex work never downgrades** — A COMPLEX task stays on FRONTIER
2. **Uncertain work never costs** — Low confidence = no downshift

These are tested in `cost_safety_invariants_test.go`.

## Commands

### Preview Classification

```bash
downshift session --why   # the classification recorded for the latest decision
```

(The `downshift try` command was deleted; the output below is historical.)

Output:
```
Task: rename userId to userID across the codebase
Complexity: TRIVIAL
Tier: SMALL
Signals:
  - "rename" (weight: 10, class: TRIVIAL)
  - "across the codebase" (weight: 5, class: SIMPLE)
Final: TRIVIAL (score: 15 vs SIMPLE: 5)
```

### Verbose Classification

Historical: `downshift try --verbose` showed all matched patterns and their weights.

## Customization

### Project Policy Floor

Create `.downshift/policy.json` in your project:

```json
{
  "floor": "MID"
}
```

This sets a minimum tier for the project. A TRIVIAL task in a critical
codebase still gets MID-tier models.

### Environment Override

```bash
DOWNSHIFT_NO_ROUTE=1  # Disable routing entirely
```

## Accuracy

The classifier is tested against a hand-labeled corpus of real tasks.
Current metrics:

- **Tier accuracy**: 94% (task gets correct-or-higher tier)
- **Exact match**: 78% (task gets exact complexity)
- **Under-tier rate**: 0.3% (task gets lower tier than needed)

The under-tier rate is the critical metric. A task that gets a higher
tier than needed wastes tokens; a task that gets a lower tier than needed
might fail.

## How It Compares

| Approach | Latency | Cost | Accuracy |
|----------|---------|------|----------|
| Always frontier | 0ms | High | 100% (overkill) |
| LLM-based routing | 500ms+ | Medium | ~90% |
| **Downshift patterns** | **<1ms** | **Zero** | **94%** |

Pattern-based classification is fast enough to run on every spawn with
no perceptible delay.
