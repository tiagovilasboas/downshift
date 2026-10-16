// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codex

import (
	"bytes"
	"encoding/json"

	"github.com/tiagovilasboas/downshift/internal/hookutil"
)

// sessionModelFromTranscript returns the parent session's current model: the
// last turn_context payload.model in the rollout tail. A spawn_agent call
// without "model" runs on it (22 of 24 such spawns in the operator's rollouts,
// 2026-10-12; see docs/reference/harness-matrix.md). Only the model id is kept.
func sessionModelFromTranscript(path string) string {
	return hookutil.LastInTranscriptTail(path, turnContextModel)
}

func turnContextModel(line []byte) string {
	if !bytes.Contains(line, []byte(`"turn_context"`)) {
		return ""
	}
	var row struct {
		Type    string `json:"type"`
		Payload struct {
			Model string `json:"model"`
		} `json:"payload"`
	}
	if json.Unmarshal(line, &row) != nil || row.Type != "turn_context" {
		return ""
	}
	return row.Payload.Model
}
