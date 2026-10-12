// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/hookutil"
)

// sessionTailBytes bounds how much of the parent transcript is read. The last
// assistant message sits at the end, and parent transcripts grow without bound.
const sessionTailBytes = 1 << 20

// inheritingAgentTypes run on the parent session's model when the Task call
// sets no model. Other agent types may pin their own model in their
// definition, so the parent model would be a guess for them.
var inheritingAgentTypes = map[string]bool{"": true, "general-purpose": true, "claude": true}

// inheritsSessionModel reports whether this Task call runs on the session model.
func inheritsSessionModel(ti map[string]any) bool {
	return hookutil.StringField(ti, "model") == "" && inheritingAgentTypes[hookutil.StringField(ti, "subagent_type")]
}

// sessionModelFromTranscript returns the model of the last assistant message in
// the parent session transcript. Claude Code's PreToolUse payload carries the
// transcript path but not the session model. Only the model id is kept; the
// message content is never stored.
func sessionModelFromTranscript(path string) string {
	if path == "" || filepath.Ext(path) != ".jsonl" {
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
	if off := fi.Size() - sessionTailBytes; off > 0 {
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return ""
		}
	}
	var model string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), sessionTailBytes)
	for sc.Scan() {
		line := sc.Bytes()
		if !bytes.Contains(line, []byte(`"assistant"`)) {
			continue
		}
		var row struct {
			Type    string `json:"type"`
			Message struct {
				Model string `json:"model"`
			} `json:"message"`
		}
		// The first line after a mid-file seek is partial and fails to parse.
		if json.Unmarshal(line, &row) != nil || row.Type != "assistant" {
			continue
		}
		// Claude Code marks locally generated messages with "<synthetic>".
		if m := row.Message.Model; m != "" && !strings.HasPrefix(m, "<") {
			model = m
		}
	}
	return model
}
