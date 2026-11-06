// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"bufio"
	"bytes"
	"encoding/json"
)

// Tokens is what one worker spent, as its CLI reported it. Counts only: no
// prices and no dollars (owner decision). The classes do not overlap: Input is
// fresh (uncached) input, and Total, which the per-CLI budget sums, is the sum
// of all four. Each parser maps its CLI onto these classes.
type Tokens struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
	Total      int64 `json:"total"`
}

// claudeTokens reads the usage block of the single JSON object that
// `claude -p --output-format json` writes. Claude already reports cache reads
// and cache writes apart from input_tokens. It returns nil when the output
// carries no usage.
func claudeTokens(stdout []byte) *Tokens {
	var r struct {
		Usage *struct {
			Input      int64 `json:"input_tokens"`
			Output     int64 `json:"output_tokens"`
			CacheRead  int64 `json:"cache_read_input_tokens"`
			CacheWrite int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(bytes.TrimSpace(stdout), &r) != nil || r.Usage == nil {
		return nil
	}
	u := r.Usage
	return total(Tokens{Input: u.Input, Output: u.Output, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite})
}

// codexTokens sums the usage of every turn.completed event that
// `codex exec --json` writes as JSON lines. Codex counts cached input inside
// input_tokens (the OpenAI convention), so the cached parts are taken out of
// Input. Lines that are not JSON or carry another event are skipped. It
// returns nil when no turn reported usage.
func codexTokens(stdout []byte) *Tokens {
	var t Tokens
	found := false
	sc := bufio.NewScanner(bytes.NewReader(stdout))
	sc.Buffer(make([]byte, 0, 64*1024), maxOutput*4)
	for sc.Scan() {
		var ev struct {
			Type  string `json:"type"`
			Usage *struct {
				Input      int64 `json:"input_tokens"`
				Cached     int64 `json:"cached_input_tokens"`
				CacheWrite int64 `json:"cache_write_input_tokens"`
				Output     int64 `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(sc.Bytes(), &ev) != nil || ev.Type != "turn.completed" || ev.Usage == nil {
			continue
		}
		found = true
		t.Input += ev.Usage.Input
		t.Output += ev.Usage.Output
		t.CacheRead += ev.Usage.Cached
		t.CacheWrite += ev.Usage.CacheWrite
	}
	if !found {
		return nil
	}
	t.Input = max(t.Input-t.CacheRead-t.CacheWrite, 0)
	return total(t)
}

// total fills Total with the sum of the four classes.
func total(t Tokens) *Tokens {
	t.Total = t.Input + t.Output + t.CacheRead + t.CacheWrite
	return &t
}
