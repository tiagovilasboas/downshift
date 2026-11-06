// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// AdviceComplexOnSmall is Decision.Advice for a decisive COMPLEX task the
// orchestrator asked to run on the small tier, when nothing raised it. The
// adapter turns it into a note the orchestrator reads; the spawn runs as asked.
const AdviceComplexOnSmall = "complex_on_small"

// AdviceAboveAgentDefault is Decision.Advice for a spawn that pinned a model
// above its agent's declared default tier. It only informs: the spawn runs as
// pinned.
const AdviceAboveAgentDefault = "above_agent_default"

// AgentPin is the agent and the tiers behind AdviceAboveAgentDefault.
type AgentPin struct {
	Agent   string // agent name without the harness prefix
	Default Tier   // tier its definition declares
	Pinned  Tier   // tier of the model the call named
}

// AgentPinNote is the sentence the orchestrator reads for an AgentPin. It names
// no task text.
func AgentPinNote(p AgentPin) string {
	return "downshift: " + p.Agent + " defaults to " + p.Default.String() + "; you pinned " + p.Pinned.String() + ". It runs as pinned."
}
