<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# T4: respect CLAUDE_CODE_SUBAGENT_MODEL as the inherited baseline

**Problem.** When the Agent call has no `model`, the adapter assumes the child
inherits the parent's transcript model (`claudecode/sessionmodel.go`). Per the
Claude Code docs the order is: per-call `model`, agent frontmatter, then
`CLAUDE_CODE_SUBAGENT_MODEL`, then the main conversation model. If the env var
is set, the real baseline is that value, so the recorded verdict and savings are
computed against the wrong model.

**Do.** In `ev.currentModel`, when the call has no model and the agent type
inherits, prefer `CLAUDE_CODE_SUBAGENT_MODEL` (when set and not `inherit`) over
the transcript model. Per `HarnessCapabilities.ModelOverrideEnv` the env name is
already declared there: read it through the capability, not a literal in the
adapter. Document the order in `docs/session-models.md`.

**Done when.** Tests for env set, env `inherit`, env unset, explicit model wins,
non-inheriting agent types untouched; `go test ./internal/adapters/claudecode/`
green.
