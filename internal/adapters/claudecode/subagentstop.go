// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/childend"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// maxTranscriptBytes bounds how much of a subagent transcript is read. The hook
// must stay cheap, and a transcript past this size is not worth pricing.
const maxTranscriptBytes = 32 << 20

// subagentUsage is what a finished subagent's transcript reports.
type subagentUsage struct {
	Model string // model of the last assistant message
	Usage telemetry.TokenUsage
}

// readSubagentUsage reads a Claude Code subagent transcript (JSONL). The API
// repeats one message id across several lines while streaming, with cumulative
// usage, so the last line per id is the one that counts. The model is what the
// API reported for the response, not what any hook asked for.
func readSubagentUsage(path string) (subagentUsage, bool) {
	if filepath.Ext(path) != ".jsonl" {
		return subagentUsage{}, false
	}
	// Lstat first: opening a FIFO or device blocks until a writer appears.
	if fi, err := os.Lstat(path); err != nil || !fi.Mode().IsRegular() {
		return subagentUsage{}, false
	}
	f, err := os.Open(path)
	if err != nil {
		return subagentUsage{}, false
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() || fi.Size() > maxTranscriptBytes {
		return subagentUsage{}, false
	}

	type usage struct {
		in, out, cached, cacheWrite int64
	}
	byID := map[string]usage{}
	var order []string
	var model string
	rd := bufio.NewReaderSize(io.LimitReader(f, maxTranscriptBytes), 1<<20)
	for {
		line, err := rd.ReadBytes('\n')
		if len(line) > 0 {
			var row struct {
				Type    string `json:"type"`
				Message struct {
					ID    string         `json:"id"`
					Model string         `json:"model"`
					Usage map[string]any `json:"usage"`
				} `json:"message"`
			}
			if json.Unmarshal(line, &row) == nil && row.Type == "assistant" && row.Message.ID != "" {
				u := telemetry.TokenUsageFromPayload(map[string]any{"usage": row.Message.Usage})
				if _, seen := byID[row.Message.ID]; !seen {
					order = append(order, row.Message.ID)
				}
				byID[row.Message.ID] = usage{u.InputTokens, u.OutputTokens, u.CachedTokens, u.CacheWriteTokens}
				if row.Message.Model != "" {
					model = row.Message.Model
				}
			}
		}
		if err != nil {
			break
		}
	}
	var total telemetry.TokenUsage
	for _, id := range order {
		u := byID[id]
		total.InputTokens += u.in
		total.OutputTokens += u.out
		total.CachedTokens += u.cached
		total.CacheWriteTokens += u.cacheWrite
	}
	if total == (telemetry.TokenUsage{}) {
		return subagentUsage{}, false
	}
	return subagentUsage{Model: model, Usage: total}, true
}

// parseSubagentStop turns a SubagentStop payload into a ChildEnd: the agent id
// (hashed by the recorder) joins the usage to the "resolved" observation made at
// launch, and through it to the routing decision. The usage and the model come
// from the subagent's own transcript, because the PostToolUse payload of an
// async launch carries no tokens. ok is false when the payload has no agent id
// or a readable .jsonl transcript with usage.
func parseSubagentStop(payload map[string]any) (hookport.ChildEnd, bool) {
	agent := stringFieldAny(payload, "agent_id", "agentId")
	path := stringFieldAny(payload, "agent_transcript_path")
	if agent == "" || path == "" || !strings.HasSuffix(path, ".jsonl") {
		return hookport.ChildEnd{}, false
	}
	got, ok := readSubagentUsage(path)
	if !ok {
		return hookport.ChildEnd{}, false
	}
	usage := got.Usage
	return hookport.ChildEnd{
		Harness:      harnessID,
		AgentKey:     agent,
		SessionID:    stringFieldAny(payload, "session_id", "sessionId"),
		Model:        got.Model,
		Usage:        &usage,
		Source:       hookport.SourceTranscript,
		IsChild:      true,
		Verification: readSubagentVerification(path),
	}, true
}

// HandleSubagentStop prices a finished subagent from its own transcript.
//
// The real spend is read here, once the subagent stops. A spawn the hook never
// rewrote has no launch observation: its usage is still recorded, unlinked,
// priced as unrouted. It is fail-open and idempotent (the shared recorder skips
// an agent already priced; SubagentStop can fire more than once). The returned
// note is for stderr.
func HandleSubagentStop(raw []byte, binaryVersion string, r core.Resolver) string {
	note, _ := handleSubagentStop(raw, binaryVersion, r)
	return note
}

// HandleSubagentStopOut is HandleSubagentStop that also returns the hook's
// stdout: a systemMessage with the child-end line when this call priced the
// subagent, the harness can show it and DOWNSHIFT_QUIET is not set. The line
// never changes what the subagent returned and never blocks.
func HandleSubagentStopOut(raw []byte, binaryVersion string, r core.Resolver) (any, string) {
	note, res := handleSubagentStop(raw, binaryVersion, r)
	if res.Usage == nil || res.Line == "" || !telemetry.ChildLineEnabled(core.ClaudeCodeCaps.CanShowChildLine) {
		return nil, note
	}
	return Output{SystemMessage: res.Line}, note
}

func handleSubagentStop(raw []byte, binaryVersion string, r core.Resolver) (string, childend.Result) {
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return "", childend.Result{}
	}
	ce, ok := parseSubagentStop(payload)
	if !ok {
		return "", childend.Result{}
	}
	res := childend.Record(ce, binaryVersion, r)
	return res.Note, res
}
