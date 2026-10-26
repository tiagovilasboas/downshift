// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// sessionLogName turns a parent_session reference into its log file name.
// KiroCrew stores "dashboard:chat-7-1790861092" as
// "dashboard_chat-7-1790861092.jsonl": only the first ":" separates the
// namespace from the id, and it becomes "_". A "/" or ".." never leaves the
// sessions directory: the name is used as a single path element.
func sessionLogName(parent string) string {
	name := strings.Replace(parent, ":", "_", 1)
	return name + ".jsonl"
}

// modelFromSessionLog reads the parent chat log and returns the model on the
// last assistant line (meta.turn_stats.model). The log mixes models over its
// lifetime; the last assistant turn is the current one. The log is streamed
// one line at a time, so memory is bounded by the longest line, never by the
// file. Lines are not capped: real KiroCrew lines reach ~660 KB, and a cap
// that stops the scan would return a stale model. The path is confined to the
// sessions directory.
func modelFromSessionLog(crew, parent string) string {
	name := sessionLogName(parent)
	if name != filepath.Base(name) || strings.Contains(name, "..") {
		return "" // reject a parent ref that is not a plain file name
	}
	path := filepath.Join(crew, "sessions", name)
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	r := bufio.NewReader(f)
	last := ""
	for {
		line, err := r.ReadBytes('\n')
		if m := assistantModel(line); m != "" {
			last = m
		}
		if err != nil {
			// io.EOF ends the log normally; any other read error keeps the
			// last model seen so far.
			return last
		}
	}
}

// assistantModel returns the cleaned meta.turn_stats.model of an assistant
// record, or "" for any other line, blank lines and malformed JSON.
func assistantModel(line []byte) string {
	var rec struct {
		Role string `json:"role"`
		Meta struct {
			TurnStats struct {
				Model string `json:"model"`
			} `json:"turn_stats"`
		} `json:"meta"`
	}
	if json.Unmarshal(line, &rec) != nil || rec.Role != "assistant" {
		return ""
	}
	return cleanModel(rec.Meta.TurnStats.Model)
}
