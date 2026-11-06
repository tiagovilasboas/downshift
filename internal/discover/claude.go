// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"bytes"
	"encoding/json"
)

var (
	claudeToolUse    = []byte(`"tool_use"`)
	claudeToolResult = []byte(`"tool_result"`)
)

// claudeLine is the slice of a Claude Code transcript line this package reads.
type claudeLine struct {
	Timestamp string `json:"timestamp"`
	Message   struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type claudeBlock struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	ToolUseID string `json:"tool_use_id"`
	Input     struct {
		Command string `json:"command"`
	} `json:"input"`
	Content json.RawMessage `json:"content"`
}

// parseClaude reads Bash tool_use blocks (assistant lines) and tool_result
// blocks (user lines) from a Claude Code project transcript.
func parseClaude(line []byte) (event, bool) {
	if !bytes.Contains(line, claudeToolUse) && !bytes.Contains(line, claudeToolResult) {
		return event{}, false
	}
	var l claudeLine
	if json.Unmarshal(line, &l) != nil || len(l.Message.Content) == 0 || l.Message.Content[0] != '[' {
		return event{}, false
	}
	var blocks []claudeBlock
	if json.Unmarshal(l.Message.Content, &blocks) != nil {
		return event{}, false
	}
	ev := event{at: parseTime(l.Timestamp)}
	for _, b := range blocks {
		switch {
		case b.Type == "tool_use" && b.Name == "Bash" && b.Input.Command != "":
			ev.calls = append(ev.calls, call{id: b.ID, command: b.Input.Command})
		case b.Type == "tool_result" && b.ToolUseID != "":
			ev.results = append(ev.results, result{id: b.ToolUseID, bytes: textBytes(b.Content)})
		}
	}
	return ev, len(ev.calls)+len(ev.results) > 0
}
