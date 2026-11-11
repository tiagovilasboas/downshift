// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kirocrew

import (
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookutil"
)

// observedSpawnTools are spawn tools whose call names no model, so there is
// nothing to block on and no model a respawn could name. kiro-cli 3.x spawns
// through "subagent" with subAgentName, prompt and explanation; the child model
// is set by the named agent (docs/evidence/kirocrew-hook-payloads-2026-11-11.md).
var observedSpawnTools = map[string]bool{
	"subagent": true,
}

// observeSpawn records the routing decision of a spawn KiroCrew cannot act on.
// It never blocks and returns no note, so the runner records an allow.
func observeSpawn(ev Event, ti map[string]any, res core.Resolver) (Output, string, core.Decision) {
	task := hookutil.TaskText(ti, "prompt", "task", "description", "explanation")
	if task == "" {
		return allow(), "", core.Decision{}
	}
	session := core.ResolveSession(harnessID, ev.SessionModels, ev.AvailableModels)
	if ev.ModelMetadata != nil {
		session.Metadata = ev.ModelMetadata
	}
	model := hookutil.StringField(ti, "model")
	decision := core.ObserveSpawn(core.Request{
		Prompt:         task,
		Harness:        harnessID,
		AgentType:      hookutil.StringField(ti, "subAgentName"),
		ModelInherited: model == "",
		CurrentModelID: model,
	}, core.KiroCrewCaps, session, res)
	return allow(), "", decision
}
