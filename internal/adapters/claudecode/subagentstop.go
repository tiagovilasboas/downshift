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
	"time"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
	"github.com/tiagovilasboas/harness-downshift/internal/telemetry"
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
	f, err := os.Open(path)
	if err != nil {
		return subagentUsage{}, false
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() || fi.Size() > maxTranscriptBytes {
		return subagentUsage{}, false
	}

	type usage struct {
		in, out, cached int64
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
				byID[row.Message.ID] = usage{u.InputTokens, u.OutputTokens, u.CachedTokens}
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
	}
	if total == (telemetry.TokenUsage{}) {
		return subagentUsage{}, false
	}
	return subagentUsage{Model: model, Usage: total}, true
}

// HandleSubagentStop prices a finished subagent from its own transcript.
//
// The PostToolUse payload of an async launch carries no tokens, so the real
// spend is read here, once the subagent stops. The agent id (hashed) joins this
// record to the "resolved" observation made at launch, and through it to the
// routing decision. A spawn the hook never rewrote has no such observation: its
// usage is still recorded, unlinked, priced as unrouted.
//
// It is fail-open and idempotent: any problem returns "" without writing, and
// an agent already priced is skipped (SubagentStop can fire more than once).
// The returned note is for stderr.
func HandleSubagentStop(raw []byte, binaryVersion string, r core.Resolver) string {
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return ""
	}
	agentHash := telemetry.HashSessionID(stringFieldAny(payload, "agent_id", "agentId"))
	path := stringFieldAny(payload, "agent_transcript_path")
	if agentHash == "" || path == "" || !strings.HasSuffix(path, ".jsonl") {
		return ""
	}
	sessionHash := telemetry.HashSessionID(stringFieldAny(payload, "session_id", "sessionId"))

	prior, _ := telemetry.ReadEvents()
	var resolved *telemetry.Event
	for i := range prior {
		ev := &prior[i]
		if ev.AgentHash != agentHash {
			continue
		}
		if ev.Outcome == telemetry.OutcomeUsage {
			return "" // this agent is already priced
		}
		if ev.Outcome == telemetry.OutcomeResolved {
			resolved = ev
		}
	}

	got, ok := readSubagentUsage(path)
	if !ok {
		return ""
	}

	var target *telemetry.Event
	if resolved != nil {
		for i := range prior {
			if !telemetry.IsCostOnlyOutcome(prior[i].Outcome) && decisionKey(prior[i]) == resolved.LinkedDecision {
				target = &prior[i]
			}
		}
	}
	ev := buildUsageEvent(got.Usage, got.Model, sessionHash, binaryVersion, target, r, time.Now().UTC())
	ev.AgentHash = agentHash
	telemetry.Record(ev)
	if target != nil {
		return "usage linked to " + target.Verdict + " decision"
	}
	return "usage recorded without matching decision"
}
