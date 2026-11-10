// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package missedsavings

import (
	"encoding/json"
	"time"
)

// event is what one transcript line says about shell commands: calls made
// (command text, in memory only) and results returned (size only).
type event struct {
	at      time.Time
	calls   []call
	results []result
}

type call struct {
	id      string
	command string
}

type result struct {
	id    string
	bytes int64
}

// parser turns one transcript line into an event; ok is false for any line
// that is malformed or irrelevant.
type parser func(line []byte) (event, bool)

// parseTime reads an RFC 3339 stamp; the zero time means unknown.
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// textBytes sizes a tool result: a JSON string, or an array of blocks with a
// "text" field. Anything else counts as zero.
func textBytes(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	switch raw[0] {
	case '"':
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return int64(len(s))
		}
	case '[':
		var parts []struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(raw, &parts) == nil {
			var n int64
			for _, p := range parts {
				n += int64(len(p.Text))
			}
			return n
		}
	}
	return 0
}
