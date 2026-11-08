# Handoff Protocol

How agents transfer work to each other in Downshift.

## When to Handoff

1. **Task complete** — your part is done, another agent continues
2. **Out of scope** — task needs expertise you don't have
3. **Blocked** — you need a decision or input from another agent

## Handoff Format

When handing off to another agent, provide:

```yaml
handoff:
  from: <your-agent>
  to: <target-agent>
  reason: complete | out_of_scope | blocked
  
  # What you did
  completed:
    - "Implemented rate limiting in middleware"
    - "Added tests for happy path"
  
  # What's left
  remaining:
    - "Security review of token bucket implementation"
    - "Load testing under high concurrency"
  
  # Artifacts created
  artifacts:
    - path: "src/middleware/ratelimit.go"
      status: ready_for_review
    - path: "src/middleware/ratelimit_test.go"
      status: complete
  
  # Context the next agent needs
  context: |
    Used token bucket algorithm with Redis backend.
    Configuration in config/ratelimit.yaml.
    Decision: 100 req/min per user, 1000 req/min per IP.
  
  # Blockers (if reason is blocked)
  blockers:
    - "Need architect decision on burst allowance"
```

## Handoff Chains

Common handoff sequences:

```
researcher → architect → coder → tester → reviewer
     │            │
     └── "needs design" ──┘

security → coder → security (review)
    │         │
    └── "implement fix" ──┘

debugger → coder → tester (verify)
    │         │
    └── "found root cause" ──┘
```

## Rules

1. **Never drop context** — include everything the next agent needs
2. **List artifacts explicitly** — paths, not descriptions
3. **State blockers clearly** — what decision is needed, from whom
4. **Keep handoffs minimal** — prefer finishing your part completely

## Example: Coder → Reviewer

```yaml
handoff:
  from: coder
  to: reviewer
  reason: complete
  
  completed:
    - "Implemented user preferences API"
    - "Added CRUD endpoints"
    - "Wrote unit tests (92% coverage)"
  
  artifacts:
    - path: "src/api/preferences.go"
      status: ready_for_review
    - path: "src/api/preferences_test.go"
      status: complete
    - path: "docs/api/preferences.md"
      status: complete
  
  context: |
    PR #760 ready for review.
    Uses existing auth middleware.
    Follows REST conventions from user.go.
```

## Example: Security → Architect (Blocked)

```yaml
handoff:
  from: security
  to: architect
  reason: blocked
  
  completed:
    - "Audited auth flow"
    - "Found IDOR vulnerability in /api/users/{id}"
  
  blockers:
    - "Need decision: add ownership check in controller or middleware?"
    - "Trade-off: controller is simpler, middleware is consistent"
  
  context: |
    Current code trusts user-provided ID without checking ownership.
    Attack: User A can read User B's data by guessing ID.
    Severity: HIGH
```

---

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai
