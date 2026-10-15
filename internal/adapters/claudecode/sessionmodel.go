// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"

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

// sessionModelFromTranscript returns the model of the last assistant message in
// the parent session transcript. Claude Code's PreToolUse payload carries the
// transcript path but not the session model. Only the model id is kept; the
// message content is never stored.
func sessionModelFromTranscript(path string) string {
	return hookutil.LastInTranscriptTail(path, assistantModel)
}

// assistantModel reads message.model from an assistant line. Claude Code marks
// locally generated messages with "<synthetic>"; those are skipped.
func assistantModel(line []byte) string {
	if !bytes.Contains(line, []byte(`"assistant"`)) {
		return ""
	}
	var row struct {
		Type    string `json:"type"`
		Message struct {
			Model string `json:"model"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &row) != nil || row.Type != "assistant" || strings.HasPrefix(row.Message.Model, "<") {
		return ""
	}
	return row.Message.Model
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
