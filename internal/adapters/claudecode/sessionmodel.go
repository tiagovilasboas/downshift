// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookutil"
)

// inheritingAgentTypes run on the parent session's model when the Task call
// sets no model: the general-purpose and catch-all types, and the built-in
// Explore and Plan (sub-agents docs, "Built-in subagents"). Other types may pin
// their own model in their definition (statusline-setup runs on Sonnet,
// claude-code-guide on Haiku, a custom type on whatever its frontmatter says),
// so the parent model would be a guess for them and the baseline stays unknown.
var inheritingAgentTypes = map[string]bool{"": true, "general-purpose": true, "claude": true, "Explore": true, "Plan": true}

// envDefaultOnlyAgentTypes use CLAUDE_CODE_SUBAGENT_MODEL only when it is forced
// onto every subagent (CLAUDE_CODE_SUBAGENT_MODEL_FORCE); as a plain default it
// reaches general-purpose and the catch-all but not these built-ins.
var envDefaultOnlyAgentTypes = map[string]bool{"Explore": true, "Plan": true}

// subagentModelForceEnv is the variable that makes subagentModelFromEnv win over
// every definition and per-call choice (Claude Code v2.1.257 and later).
const subagentModelForceEnv = "CLAUDE_CODE_SUBAGENT_MODEL_FORCE"

// inheritsSessionModel reports whether this Task call runs on the session model.
func inheritsSessionModel(ti map[string]any) bool {
	return hookutil.StringField(ti, "model") == "" && inheritingAgentTypes[hookutil.StringField(ti, "subagent_type")]
}

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

// transcriptWaitTotal and transcriptWaitStep bound how long the hook waits for the parent's assistant message
// to reach the transcript (variables so tests can shorten them).
//
// Claude Code writes the transcript asynchronously. On the first spawn of a session the PreToolUse hook can run
// before the assistant message that carries the session model is on disk: in 3 fresh `claude -p` runs the
// transcript had no assistant line at the hook in 2 of them, and one 0.5 s later. Reading it once then gave an
// unknown baseline, and an unknown baseline holds the rewrite, so those spawns were never routed. 400 ms was
// enough in every measurement and is small next to a hook's own timeout.
var (
	transcriptWaitTotal = 400 * time.Millisecond
	transcriptWaitStep  = 40 * time.Millisecond
)

// waitedEmpty remembers the last transcript whose bounded wait found no model.
// One hook call reads the transcript twice (RequestedModel for telemetry and
// Handle); the second read is still made, but the wait is not paid twice.
var waitedEmpty struct {
	sync.Mutex
	path string
}

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
	if path == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	waitedEmpty.Lock()
	again := waitedEmpty.path == path
	waitedEmpty.Unlock()
	if again {
		return ""
	}
	for waited := time.Duration(0); waited < transcriptWaitTotal; waited += transcriptWaitStep {
		time.Sleep(transcriptWaitStep)
		if m := assistantModel(path); m != "" {
			return m
		}
	}
	waitedEmpty.Lock()
	waitedEmpty.path = path
	waitedEmpty.Unlock()
	return ""
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
