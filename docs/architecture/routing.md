> **Historical — 2026-10-22.** This document describes an earlier design.
> The live reference is [`docs/design/architecture.md`](../design/architecture.md).

# Routing Engine

The routing engine classifies task complexity and selects the optimal model
tier without any LLM calls.

## Pipeline

```
Task Prompt → Classify → Apply Guardrails → Select Model → Emit Decision
                 ↓              ↓                ↓
           Complexity      Risk Floor      Tier Mapping
           (NB + regex)    (keywords)      (catalog)
```

## Classification

### Naive Bayes + Patterns

The classifier combines:

1. **Word-level Naive Bayes** — trained on labeled tasks
2. **Regex patterns** — ~100 weighted signals
3. **Structural rules** — word count, code references

### Complexity Levels

| Level | Examples |
|-------|----------|
| TRIVIAL | rename, format, move |
| SIMPLE | add field, fix typo |
| MEDIUM | implement feature |
| COMPLEX | architecture, security |

## Guardrails

### Risk Floor

Keywords that force COMPLEX:
- Security: auth, credential, vulnerability
- Data: migration, schema, database
- Money: payment, billing, refund

### Confidence Threshold

Low confidence → route conservatively:

```go
if confidence < 0.6 {
    return higherTier(classified)
}
```

## Model Selection

### Tier Mapping

```
TRIVIAL, SIMPLE → SMALL tier  (Haiku, GPT-4o-mini)
MEDIUM          → MID tier    (Sonnet, GPT-4o)
COMPLEX         → FRONTIER    (Opus, o1)
```

### Catalog

Model metadata in `internal/catalog/`:
- Tier assignment
- Pricing
- Capabilities

## Adaptive Memory

Outcomes feed back into routing:

```
Task fails on SMALL → Memory: key needs MID
Similar task arrives → Route to MID
```

## See Also

- [Classification](../features/classification.md)
- [Safety & Guardrails](../features/safety.md)
