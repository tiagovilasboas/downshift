# Kiro IDE: how Downshift works here

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

Kiro IDE is an **observation-only** harness: the adapter records routing decisions
and provides advisory tools via MCP, but cannot rewrite the model of a subagent in
place. This page explains the architecture, setup, and limitations.

## 1. Kiro IDE cannot rewrite models

**Evidence**: REA 2026-11-10 ([kiro-kirocrew-rea-binary-2026-11-10.md](../evidence/kiro-kirocrew-rea-binary-2026-11-10.md))

| Capability | Kiro IDE | Claude Code |
|------------|----------|-------------|
| `CanRewriteModel` | `false` | `true` |
| `updatedInput.model` | Not supported | Supported |
| Hook protocol | exit 0/2 only | exit 0 + JSON payload |

Kiro's hook contract is:
- **exit 0**: allow the tool call
- **exit 2**: block + stderr message to the LLM

There is no `updatedInput` rewrite path. The `invoke_sub_agent` payload carries no
`model` field (the model is decided internally by the harness). Custom agent
`model:` fields in `.kiro/agents/*.md` are ignored (GitHub issue #6637).

## 2. MCP mode: advisory routing

Since Downshift cannot intercept and rewrite spawns, it provides **advisory tools**
the orchestrator can query before making a decision:

| Tool | Purpose |
|------|---------|
| `providers` | List available models with tiers |
| `apply` | Dry-run: what would be applied for a choice |
| `playbook` | Working rules for workers |
| `status` | Quota advice |

The orchestrator (Kiro's main session) calls these tools, reads the advice, and
chooses how to spawn subagents using steering instructions.

## 3. Setup

### 3.1. Install Downshift

```bash
brew install tiagovilasboas/tap/downshift
# or: go install github.com/tiagovilasboas/downshift-ai/cmd/downshift@latest
```

### 3.2. Configure MCP server

Create or edit `.kiro/settings/mcp.json` in your workspace (or `~/.kiro/settings/mcp.json` for global):

```json
{
  "mcpServers": {
    "downshift": {
      "command": "downshift",
      "args": ["mcp", "serve"],
      "disabled": false
    }
  }
}
```

If `downshift` is not in PATH, use the full path:

```json
{
  "mcpServers": {
    "downshift": {
      "command": "/opt/homebrew/bin/downshift",
      "args": ["mcp", "serve"],
      "disabled": false
    }
  }
}
```

### 3.3. Configure model list

Add the models your Kiro account can use to `~/.downshift/session-models.json`:

```json
{
  "kiro": [
    "claude-haiku-4.5",
    "claude-sonnet-4.5",
    "claude-sonnet-4.6",
    "claude-opus-4.5"
  ]
}
```

Verify with:

```bash
downshift models available --harness kiro
```

### 3.4. Create steering file

Create `.kiro/steering/downshift-routing.md` to instruct Kiro to use the MCP tools:

```markdown
---
inclusion: auto
name: downshift-routing
description: Advisory model routing for subagent spawns using Downshift MCP
---

# Downshift Model Routing

Before spawning a subagent with `invoke_sub_agent`, call `downshift.providers`
with `{"harness": "kiro"}` to see available models and their tiers.

## Tier guidelines

| Tier | Use for |
|------|---------|
| small | Mechanical work: grep, read, rename, single command |
| mid | Implementation, tests, refactoring |
| frontier | Architecture, complex debugging, difficult decisions |

## Workflow

1. Before spawn: call `providers(harness: "kiro")` to see models
2. Choose tier based on the task complexity
3. For mechanical work, add `[tier=small]` to the subagent prompt
4. Call `apply(harness: "kiro", tier: "small")` to verify the choice

## Important

Kiro IDE cannot enforce model routing. The `model:` field in custom agents is
ignored. This steering provides **advisory guidance** only. The orchestrator
chooses how to delegate based on the advice.
```

## 4. Workflow example

When delegating to `context-gatherer`:

1. **Before spawn**: The orchestrator calls `downshift.providers(harness: "kiro")`
2. **Response**: Returns `{models: [{id: "claude-haiku-4.5", tier: "small"}, ...]}`
3. **Decision**: For a codebase exploration task, choose `mid` tier
4. **Spawn**: Invoke `context-gatherer` (Kiro picks the model internally)

The steering instructs the orchestrator to consider task complexity, but the
final model selection is made by Kiro's internal routing.

## 5. Limitations

| What | Status |
|------|--------|
| Model rewrite | ❌ Not possible |
| Effort level | ❌ Not supported |
| Shell command wrap | ❌ Not supported |
| Child usage tracking | ❌ Not available |
| Quota enforcement | Advisory only |

## 6. Comparison with KiroCrew

KiroCrew has the same limitation (`CanRewriteModel: false`), but adds:
- `CanAdviseShellWrap: true` — can suggest wrapped commands via exit 2
- `SessionEnv: "KIRO_SESSION_ID"` — session tracking

Both harnesses use advisory MCP mode. The difference is that KiroCrew's hook
can block a spawn and suggest a different tier via stderr, while Kiro IDE
can only allow or block.

## 7. References

- [harness-matrix.md](../reference/harness-matrix.md) — capability comparison
- [kirocrew-routing-guide.md](kirocrew-routing-guide.md) — KiroCrew specifics
- [REA evidence](../evidence/kiro-kirocrew-rea-binary-2026-11-10.md) — binary analysis
