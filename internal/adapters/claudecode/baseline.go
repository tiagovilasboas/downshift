// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"os"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookutil"
)

// currentModel is the model the subagent would run on without a rewrite: the
// Task "model", the payload "model", then, for agent types that inherit (see
// inheritingAgentTypes), the harness override env
// (core.ClaudeCodeCaps.ModelOverrideEnv; Explore and Plan only when forced) and
// finally the parent session model from the transcript. It never reads the
// settings fallback, which needs a catalog (see resolveModel).
func (ev Event) currentModel(ti map[string]any) string {
	m, _ := ev.resolveModel(ti, nil)
	return m
}

// resolveModel is resolveBaseline without the pin flag.
func (ev Event) resolveModel(ti map[string]any, res core.Resolver) (string, bool) {
	b := ev.resolveBaseline(ti, res)
	return b.Model, b.FromSettings
}

// baseline is the model a spawn would run on without a rewrite, and how it was
// learned.
type baseline struct {
	Model string
	// FromSettings: inferred from the configured model, not seen in the
	// transcript (see configuredModel); the decision says its baseline is a guess.
	FromSettings bool
	// Pinned: the agent definition names the model. It is the author's explicit
	// choice, so the baseline is a fact and routing must leave the spawn alone.
	Pinned bool
	// Named: the model is a choice the call or its environment made (Task
	// model, payload model, definition pin, subagent env), not an inherited one.
	Named bool
}

// resolveBaseline is currentModel plus two things that need more than the
// payload. An agent type that is not a built-in inheriting one is looked up in
// its definition (lookupAgentDefinition): no `model:` or `inherit` inherits like
// general-purpose, a named model is pinned, and a type with no definition found
// keeps an unknown baseline. For an inheriting spawn whose transcript holds no
// model, the last resort is the model the session is configured to use
// (configuredModel), which needs res to turn an alias into a catalog id. Order
// for an inheriting spawn with no per-call model: env override, ParentModel (a
// fact from the mod), transcript, then settings. ParentModel is never inferred.
func (ev Event) resolveBaseline(ti map[string]any, res core.Resolver) baseline {
	if model := hookutil.StringField(ti, "model"); model != "" {
		return baseline{Model: model, Named: true}
	}
	if ev.Model != "" {
		return baseline{Model: ev.Model, Named: true}
	}
	typ := hookutil.StringField(ti, "subagent_type")
	if !inheritingAgentTypes[typ] {
		def := lookupAgentDefinition(ev.Cwd, typ)
		switch {
		case def.Pins():
			return ev.pinnedBaseline(typ, def.Model, res)
		case !def.Inherits():
			return baseline{}
		}
	}
	if m := subagentModelFromEnv(typ); m != "" {
		return baseline{Model: m, Named: true}
	}
	if ev.ParentModel != "" {
		if exploreMayLeaveTheMainModel(typ, ev.ParentModel) {
			return baseline{}
		}
		return baseline{Model: ev.ParentModel}
	}
	// A nested spawn inherits its spawner's model, which the session transcript
	// does not hold (and neither does the configured model): unknown, so held.
	if !core.ClaudeCodeCaps.TranscriptNamesSpawner(ev.SpawnedFromSubagent()) {
		return baseline{}
	}
	parent := parentModelAtSpawn(ev.TranscriptPath, ev.ToolUseID)
	inferred := false
	if parent == "" && res != nil {
		parent = configuredModel(ev.Cwd, ev.PermissionMode, res)
		inferred = parent != ""
	}
	if exploreMayLeaveTheMainModel(typ, parent) {
		return baseline{}
	}
	return baseline{Model: parent, FromSettings: inferred}
}

// pinnedBaseline is the baseline of a definition that names its model. Claude
// Code's force flag (CLAUDE_CODE_SUBAGENT_MODEL_FORCE) lets the override
// variable win over a definition; otherwise the definition wins. Without a
// catalog the raw value stands; with one an alias becomes a catalog id, and a
// model the catalog does not know leaves the baseline empty but still pinned,
// so it is never rewritten.
func (ev Event) pinnedBaseline(typ, raw string, res core.Resolver) baseline {
	if envFlagOn(os.Getenv(subagentModelForceEnv)) {
		if m := subagentModelFromEnv(typ); m != "" {
			return baseline{Model: m, Named: true}
		}
	}
	if res == nil {
		return baseline{Model: raw, Pinned: true, Named: true}
	}
	return baseline{Model: resolveConfiguredModel(raw, ev.PermissionMode, res), Pinned: true, Named: true}
}
