// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package hookutil

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// TranscriptTailBytes bounds how much of a session transcript is read.
// The latest session state sits at the end, and transcripts grow without bound.
const TranscriptTailBytes = 1 << 20

// LastInTranscriptTail scans the tail of a JSONL transcript and returns the
// last non-empty value pick extracts from a line. pick sees one line at a
// time and must return only an identifier (a model id), never content. The
// first line after a mid-file seek is partial; pick must tolerate bad JSON.
// Any problem returns "" (fail open).
func LastInTranscriptTail(path string, pick func(line []byte) string) string {
	if path == "" || filepath.Ext(path) != ".jsonl" {
		return ""
	}
	// Lstat before Open: opening a FIFO or device would block the hook until
	// the harness timeout. A symlink is not followed either.
	if fi, err := os.Lstat(path); err != nil || !fi.Mode().IsRegular() {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return ""
	}
	if off := fi.Size() - TranscriptTailBytes; off > 0 {
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return ""
		}
	}
	var last string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), TranscriptTailBytes)
	for sc.Scan() {
		if v := pick(sc.Bytes()); v != "" {
			last = v
		}
	}
	return last
}

// TranscriptModel returns the model id of the last line in the transcript tail
// whose "type" is lineType, read from <container>.model (Claude Code:
// assistant/message, Codex: turn_context/payload). An id in angle brackets is
// a placeholder, not a model (Claude Code writes "<synthetic>" on locally
// generated messages), and is skipped. Only the id is kept; the line's content
// is never returned. Any problem returns "" (fail open).
func TranscriptModel(path, lineType, container string) string {
	return LastInTranscriptTail(path, modelLinePick(lineType, container))
}

// modelLinePick builds the pick TranscriptModel scans with. A cheap substring
// check skips lines that cannot match before any JSON is decoded.
func modelLinePick(lineType, container string) func(line []byte) string {
	marker := []byte(`"` + lineType + `"`)
	return func(line []byte) string {
		if !bytes.Contains(line, marker) {
			return ""
		}
		var row map[string]json.RawMessage
		if json.Unmarshal(line, &row) != nil {
			return ""
		}
		var typ string
		if json.Unmarshal(row["type"], &typ) != nil || typ != lineType {
			return ""
		}
		var inner struct {
			Model string `json:"model"`
		}
		if json.Unmarshal(row[container], &inner) != nil || strings.HasPrefix(inner.Model, "<") {
			return ""
		}
		return inner.Model
	}
}
