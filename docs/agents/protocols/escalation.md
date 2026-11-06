# Escalation Protocol

When and how to escalate tasks to leadership agents.

## Escalation Triggers

### Automatic Escalation

These patterns in `config.toml` trigger automatic routing to leadership:

| Pattern | Target Agent | Reason |
|---------|--------------|--------|
| auth, authentication | security | Identity is security-critical |
| authorization, access control | security | Permissions are security-critical |
| encryption, secrets, credentials | security | Crypto requires expertise |
| payment, billing | architect | Money requires design review |
| database schema, migration | architect | Data changes are hard to reverse |
| api contract, breaking change | architect | Affects downstream consumers |

### Manual Escalation

Escalate manually when you encounter:

1. **Ambiguity** — task can be interpreted multiple ways
2. **Conflict** — requirements contradict each other
3. **Risk** — change could break production
4. **Scope creep** — task is bigger than estimated
5. **Expertise gap** — you don't know the right approach

## Escalation Format

```yaml
escalation:
  from: <your-agent>
  to: <leadership-agent>
  trigger: automatic | manual
  
  # The original task
  task: "Add password reset flow"
  
  # Why you're escalating
  reason: |
    Task involves auth tokens and email verification.
    Automatic escalation: matches "auth" pattern.
  
  # What you've learned (if any)
  analysis:
    - "Current auth uses JWT with 1h expiry"
    - "No existing password reset endpoint"
    - "Email service is configured but unused"
  
  # Questions for leadership
  questions:
    - "Reset token expiry: 15min or 1h?"
    - "Rate limit reset attempts?"
    - "Notify user of reset attempt via email?"
  
  # Your recommendation (optional)
  recommendation: |
    Suggest 15min token expiry with 3 attempts/hour rate limit.
    Email notification on every attempt (security best practice).
```

## Escalation Targets

| Situation | Escalate To |
|-----------|-------------|
| Security concern | security |
| Architecture decision | architect |
| Infrastructure/deploy | platform |
| AI/ML design | ai |
| Cross-domain conflict | architect (default) |

## De-escalation

Leadership agents de-escalate by:

1. **Making the decision** — answering the questions
2. **Decomposing** — splitting into smaller, non-escalated tasks
3. **Delegating** — assigning to specific agents with clear scope

```yaml
de_escalation:
  from: architect
  decision: |
    Password reset: 15min token, 3 attempts/hour, email on every attempt.
    
  tasks:
    - agent: coder
      task: "Implement password reset endpoint"
      scope: ["src/api/auth/reset.go"]
      
    - agent: coder  
      task: "Add email notification for reset attempts"
      scope: ["src/services/email/reset.go"]
      
    - agent: security
      task: "Review implementation before merge"
      scope: ["PR review"]
```

## Rules

1. **Escalate early** — uncertainty is cheaper to resolve before coding
2. **Provide context** — don't make leadership re-discover what you know
3. **Suggest, don't decide** — recommendations help, but leadership owns the call
4. **Accept the decision** — don't re-escalate the same issue

## Anti-patterns

❌ **Over-escalation**: "I don't know how to write a for loop"  
✅ **Correct**: Only escalate design/security/risk decisions

❌ **Under-escalation**: "I'll just add this auth endpoint myself"  
✅ **Correct**: Auth always escalates to security

❌ **Escalation ping-pong**: architect → security → architect → ...  
✅ **Correct**: One escalation, one decision, move forward

---

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift
