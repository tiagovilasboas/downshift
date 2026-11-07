// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// TranscriptNamesSpawner reports whether the session transcript a hook is given
// can name the model of the agent that issued the spawn. nested is true when the
// spawn was issued from inside a subagent, not by the main session.
//
// A spawn that names no model inherits its spawner's model. The main session's
// model is in the transcript; a subagent's is not when the harness hands every
// hook the session-wide transcript (SessionWideTranscript). Reading that
// transcript for a nested spawn would record the main model as the parent and as
// the model the spawn would have run on, and a downshift from it would be
// claimed against a model the spawn never inherited. Doubt keeps the requested
// model, so the baseline stays unknown and the spawn is held. A model the call,
// the agent definition or the environment names, and a parent model the caller
// reports as a fact (the mod's agent.spawn), do not depend on the transcript and
// are not affected. Adapters call this; they do not hold the rule themselves.
func (c HarnessCapabilities) TranscriptNamesSpawner(nested bool) bool {
	return !nested || !c.SessionWideTranscript
}
