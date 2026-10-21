# Subagent Model Routing

Downshift routes subagent tasks to right-sized models without calling an LLM to
make the decision. Mechanical work runs on cheaper models; complex work keeps
the expensive ones.

## How It Works

When your harness spawns a subagent, Downshift's PreToolUse hook:

1. **Extracts the task text** from the spawn payload
2. **Classifies complexity** using strict heuristics + FNV word-hash scoring
3. **Applies risk floors** — auth, payment, secrets always keep high-tier models
4. **Returns the routing decision** — or leaves unchanged if unsure

```
$ downshift try "rename the userId variable"
→ small: opus → haiku
why: trivial task, no risk, confident
```

## The Golden Rule

**Doubt never spends money.** When the classifier is not confident:

- The spawn keeps the model it requested
- No downshift happens
- You pay what you would have paid anyway

This means Downshift can only save money, never cost more.

## Classification Tiers

| Tier | Examples | Typical Model |
|------|----------|---------------|
| **Trivial** | Renames, formatting, simple lookups | Haiku / Flash |
| **Small** | Test runs, file listings, git status | Haiku / Flash |
| **Medium** | Bug fixes, simple features | Sonnet |
| **Large** | Architecture, multi-file refactors | Opus |
| **Complex** | Security, auth, payment logic | Opus (floor) |

## Risk Floors

Certain keywords trigger a risk floor that prevents downshifting:

- **Security**: `auth`, `credential`, `secret`, `token`, `password`, `encrypt`
- **Financial**: `payment`, `billing`, `invoice`, `refund`, `transaction`
- **Data**: `delete`, `drop`, `truncate`, `migration`
- **Infrastructure**: `deploy`, `production`, `database`

Risk floors are configurable in `~/.downshift/policy.toml`.

## Guardrails (R1-R9)

Nine invariant rules that the router never violates:

| Rule | Description |
|------|-------------|
| R1 | Never route to a model not in the catalog |
| R2 | Never downshift when confidence is low |
| R3 | Never violate a risk floor |
| R4 | Never change reasoning effort without model change |
| R5 | Never route explicit-only models automatically |
| R6 | Preserve sibling fields in updatedInput |
| R7 | Hold unconfident frontier picks with no current model |
| R8 | Hold unconfident upshifts |
| R9 | Fail open on any error |

## Testing Routing

Preview what Downshift would do:

```bash
# Single task
downshift try "rename the userId variable"

# With verbose output
downshift try --verbose "implement OAuth2 authentication"

# Check guardrails
downshift try --check "delete all user data"
```

## Harness Support

| Harness | Hook | Rewrite Honored |
|---------|------|-----------------|
| Claude Code | PreToolUse → updatedInput.model | ✅ Verified |
| Cursor | preToolUse → updated_input.model | ✅ Verified |
| Codex | PreToolUse → model + reasoning_effort | ✅ Verified |
| KiroCrew | preToolUse (allow/block only) | ⚠️ No rewrite |

## Configuration

### Disable Routing

```bash
# Environment variable
export DOWNSHIFT_ROUTE=0

# Or in config
{
  "route": false
}
```

### Custom Policy

Create `~/.downshift/policy.toml`:

```toml
[floors]
# Add custom risk keywords
keywords = ["hipaa", "pci", "gdpr"]

[tiers]
# Adjust classification weights
trivial_max_words = 10
small_max_words = 50
```

## Metrics

```bash
# See routing decisions
downshift stats

# Detailed breakdown
downshift stats --routing
```
