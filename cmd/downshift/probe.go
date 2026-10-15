// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// `downshift probe <event>` is a maintainer tool. Wired as a harness hook, it
// records what a hook payload contains so a question like "does Cursor's
// sessionStart carry the session model?" is answered by a real payload and not
// by documentation. It writes one line per call to <state dir>/probe.jsonl:
// the field names, the values of a short list of fields that are not sensitive
// (model ids, the tool name, the agent type), and a short hash of each id so
// events can be joined. It never records prompts, task text, paths, e-mail
// addresses or an id in the clear, and it never blocks the harness: it always
// answers allow and exits 0.

const probeMaxBytes = 2 << 20 // stop recording at 2 MiB

var probeEventPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,39}$`)

// Fields whose value is a model id or a short label: kept as they are.
var probeValueFields = []string{"hook_event_name", "cursor_version", "model", "model_id", "subagent_model", "tool_name", "subagent_type", "composer_mode", "source"}

// Fields that hold an id: kept as an 8-character hash.
var probeIDFields = []string{"conversation_id", "generation_id", "parent_conversation_id", "session_id", "subagent_id", "tool_call_id", "tool_use_id", "agent_id"}

// Fields that hold a path: only whether one was sent.
var probePresenceFields = []string{"transcript_path", "agent_transcript_path"}

func runProbe(args []string, in io.Reader, out, errw io.Writer) int {
	event := ""
	if len(args) > 0 {
		event = args[0]
	}
	// Always answer the harness, whatever happens below.
	if event == "preToolUse" || event == "PreToolUse" {
		fmt.Fprintln(out, `{"permission":"allow"}`)
	} else {
		fmt.Fprintln(out, `{}`)
	}
	if !probeEventPattern.MatchString(event) {
		fmt.Fprintf(errw, "downshift probe: event name %q is not valid, nothing recorded\n", event)
		return 0
	}
	raw, _ := io.ReadAll(io.LimitReader(in, 1<<20))
	rec := probeRecord(event, raw)
	line, err := json.Marshal(rec)
	if err != nil {
		return 0
	}
	dir, err := paths.StateDir()
	if err != nil {
		return 0
	}
	file := filepath.Join(dir, "probe.jsonl")
	if info, err := os.Stat(file); err == nil && info.Size() >= probeMaxBytes {
		return 0
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return 0
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return 0
	}
	defer f.Close()
	f.Write(append(line, '\n'))
	return 0
}

func probeRecord(event string, raw []byte) map[string]any {
	rec := map[string]any{"at": time.Now().UTC().Format(time.RFC3339Nano), "event": event}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		rec["parse_error"] = true
		return rec
	}
	rec["top_keys"] = probeKeys(payload)
	fields := map[string]any{}
	probeFields(payload, "", fields)
	if ti, ok := payload["tool_input"].(map[string]any); ok {
		rec["tool_input_keys"] = probeKeys(ti)
		probeFields(ti, "tool_input.", fields)
	}
	if len(fields) > 0 {
		rec["fields"] = fields
	}
	return rec
}

func probeKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func probeFields(m map[string]any, prefix string, into map[string]any) {
	for _, k := range probeValueFields {
		if v, ok := m[k].(string); ok && len(v) <= 80 {
			into[prefix+k] = v
		}
	}
	for _, k := range probeIDFields {
		if v, ok := m[k].(string); ok {
			into[prefix+k] = probeIDHash(v)
		}
	}
	for _, k := range probePresenceFields {
		if v, ok := m[k].(string); ok && v != "" {
			into[prefix+k] = "present"
		}
	}
}

// probeIDHash is a stable 8-character digest of an id: enough to tell whether
// two events share one, not enough to recover it.
func probeIDHash(id string) string {
	if id == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("downshift-probe-v1:" + id))
	return hex.EncodeToString(sum[:])[:8]
}
