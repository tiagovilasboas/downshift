// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// sessionLogChunk is the read unit when a chat log is scanned from its end. It
// bounds the memory of each read, not the length of a line: a line that
// straddles chunk boundaries is reassembled before it is parsed.
const sessionLogChunk = 64 << 10

// sessionLogName turns a parent_session reference into its log file name.
// KiroCrew stores "dashboard:chat-7-1790861092" as
// "dashboard_chat-7-1790861092.jsonl": only the first ":" separates the
// namespace from the id, and it becomes "_". A "/" or ".." never leaves the
// sessions directory: the name is used as a single path element.
func sessionLogName(parent string) string {
	name := strings.Replace(parent, ":", "_", 1)
	return name + ".jsonl"
}

// modelFromSessionLog returns the model on the last assistant line of the
// parent chat log (meta.turn_stats.model). The log mixes models over its
// lifetime; the last assistant turn is the current one, and it is at the end of
// the file, so the log is read backwards and the scan stops at the first valid
// model. Real logs are 7 to 18 MB, so reading forwards would cost the whole
// file on every call. Lines are not capped: real KiroCrew lines reach ~660 KB,
// and a cap that stops the scan would return a stale model. The path is
// confined to the sessions directory.
func modelFromSessionLog(crew, parent string) string {
	name := sessionLogName(parent)
	if name != filepath.Base(name) || strings.Contains(name, "..") {
		return "" // reject a parent ref that is not a plain file name
	}
	f, err := os.Open(filepath.Join(crew, "sessions", name))
	if err != nil {
		return ""
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return ""
	}
	return lastAssistantModel(f, info.Size())
}

// lastAssistantModel scans size bytes of r backwards in sessionLogChunk pieces
// and returns the model of the last assistant line that has a valid one. A
// short read fails closed with "". pending holds the tail of a line whose
// beginning lies in a chunk not yet read: it has no newline in it, so it is
// joined with the next (earlier) chunk before it is parsed.
func lastAssistantModel(r io.ReaderAt, size int64) string {
	var pending []byte
	for pos := size; pos > 0; {
		n := int64(sessionLogChunk)
		if pos < n {
			n = pos
		}
		pos -= n
		buf := make([]byte, n)
		if got, _ := r.ReadAt(buf, pos); int64(got) != n {
			return ""
		}
		data := append(buf, pending...)
		end := len(data)
		for {
			i := bytes.LastIndexByte(data[:end], '\n')
			if i < 0 {
				break
			}
			if m := assistantModel(data[i+1 : end]); m != "" {
				return m
			}
			end = i
		}
		pending = data[:end]
	}
	// pending is now the first line of the log, complete.
	return assistantModel(pending)
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
