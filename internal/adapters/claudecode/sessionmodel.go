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
// sets no model. Other agent types may pin their own model in their
// definition, so the parent model would be a guess for them.
var inheritingAgentTypes = map[string]bool{"": true, "general-purpose": true, "claude": true}

// inheritsSessionModel reports whether this Task call runs on the session model.
func inheritsSessionModel(ti map[string]any) bool {
	return hookutil.StringField(ti, "model") == "" && inheritingAgentTypes[hookutil.StringField(ti, "subagent_type")]
}

// subagentModelFromEnv returns the model the harness override variable pins for
// subagents (name declared in core.ClaudeCodeCaps.ModelOverrideEnv). Claude
// Code resolves a subagent model as: per-call model, agent frontmatter, this
// variable, then the main conversation model. Empty when unset, blank or
// "inherit", which means the parent model applies.
func subagentModelFromEnv() string {
	name := core.ClaudeCodeCaps.ModelOverrideEnv
	if name == "" {
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
