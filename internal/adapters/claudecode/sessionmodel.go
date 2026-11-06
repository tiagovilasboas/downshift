// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"os"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookutil"
)

// inheritingAgentTypes run on the parent session's model when the Task call
// sets no model: the general-purpose and catch-all types, and the built-in
// Explore and Plan (sub-agents docs, "Built-in subagents"). Any other type is
// defined by a file, or is a built-in that runs on its own model
// (builtinPinnedAgentTypes), so the parent model would be a guess for it: its
// definition decides (agentdef.go), and a type whose definition is not found
// keeps an unknown baseline.
var inheritingAgentTypes = map[string]bool{"": true, "general-purpose": true, "claude": true, "Explore": true, "Plan": true}

// envDefaultOnlyAgentTypes use CLAUDE_CODE_SUBAGENT_MODEL only when it is forced
// onto every subagent (CLAUDE_CODE_SUBAGENT_MODEL_FORCE); as a plain default it
// reaches general-purpose and the catch-all but not these built-ins.
var envDefaultOnlyAgentTypes = map[string]bool{"Explore": true, "Plan": true}

// subagentModelForceEnv is the variable that makes subagentModelFromEnv win over
// every definition and per-call choice (Claude Code v2.1.257 and later).
var subagentModelForceEnv = core.ClaudeCodeCaps.ModelForceEnv

// subagentModelFromEnv returns the model the harness override variable pins for
// subagents (name declared in core.ClaudeCodeCaps.ModelOverrideEnv). Claude
// Code resolves a subagent model as: per-call model, agent frontmatter, this
// variable, then the main conversation model. Empty when unset, blank or
// "inherit", which means the parent model applies.
//
// subagentType matters for Explore and Plan: they take the variable only when
// the force flag is on.
func subagentModelFromEnv(subagentType string) string {
	name := core.ClaudeCodeCaps.ModelOverrideEnv
	if name == "" {
		return ""
	}
	if envDefaultOnlyAgentTypes[subagentType] && !envFlagOn(os.Getenv(subagentModelForceEnv)) {
		return ""
	}
	v := strings.TrimSpace(os.Getenv(name))
	if strings.EqualFold(v, "inherit") {
		return ""
	}
	return v
}

// emptyWait and callWait keep the shared bounded transcript wait
// (hookutil.TranscriptWait) from being paid twice by one hook call.
var emptyWait, callWait hookutil.TranscriptWait

// sessionModelFromTranscript returns the model of the last assistant message in
// the parent session transcript (message.model of the last "assistant" line;
// locally generated messages carry "<synthetic>" and are skipped by the shared
// pick). Claude Code's PreToolUse payload carries the transcript path but not
// the session model. Only the model id is kept; the message content is never
// stored.
//
// When the file exists but holds no assistant model yet, it is read again for a short, bounded time. A missing
// file is not waited for: a session run without persistence never writes one, and waiting would only delay
// every spawn.
func sessionModelFromTranscript(path string) string {
	if m := assistantModel(path); m != "" {
		return m
	}
	return emptyWait.Poll(path, path, func() string { return assistantModel(path) })
}

// parentModelAtSpawn is the model of the parent at the moment it spawned: the
// model of the assistant message that issued this very tool call. The last
// assistant model in the transcript (sessionModelFromTranscript) is the model of
// the previous turn when the session switched models in between (/model,
// opusplan entering plan mode), and a baseline from it priced a spawn on the
// wrong model: one spawn recorded a $2.38 overspend that was never the router's.
// The line for the call may not be on disk yet (the transcript is written
// asynchronously), so it is waited for, bounded as the first-spawn wait is, and
// then the last assistant model stands in. Without a call id, only that stand-in.
func parentModelAtSpawn(path, toolUseID string) string {
	if toolUseID == "" || path == "" {
		return sessionModelFromTranscript(path)
	}
	if m := hookutil.TranscriptModelOfCall(path, "assistant", "message", toolUseID); m != "" {
		return m
	}
	if _, err := os.Stat(path); err != nil {
		return sessionModelFromTranscript(path)
	}
	if m := callWait.Poll(path, path+"\x00"+toolUseID, func() string {
		return hookutil.TranscriptModelOfCall(path, "assistant", "message", toolUseID)
	}); m != "" {
		return m
	}
	return assistantModel(path) // the wait above already covered a transcript with no model yet
}

// assistantModel is one read of the transcript tail: message.model of the last
// assistant line.
func assistantModel(path string) string {
	return hookutil.TranscriptModel(path, "assistant", "message")
}

// envFlagOn reads a boolean environment flag: unset, empty, "0" and "false" are off.
func envFlagOn(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v != "" && v != "0" && v != "false"
}

// exploreMayLeaveTheMainModel reports a main model under which Claude Code can
// run the built-in Explore on a different one: on a subscription, Console
// account or gateway it uses the Opus alias when the main model is Fable, while
// on Bedrock and similar providers it stays on the main model. Which applies is
// not visible to a hook, so the baseline is left unknown.
func exploreMayLeaveTheMainModel(subagentType, mainModel string) bool {
	return subagentType == "Explore" && strings.Contains(strings.ToLower(mainModel), "fable")
}
