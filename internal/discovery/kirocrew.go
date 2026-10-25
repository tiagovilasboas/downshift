// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// KiroCrew does not expose the running model as an environment variable, unlike
// every other harness Downshift reads. The model is inferred from the gateway's
// own session files on disk:
//
//  1. KIRO_SESSION_ID identifies the current (sub)agent session.
//  2. ~/.kiro/crew/subagents/<id>/state.json whose "session_id" equals it holds
//     "resolved_model"/"requested_model" and a "parent_session" reference.
//  3. When the state file has no usable model, the parent chat session's log,
//     ~/.kiro/crew/sessions/<parent>.jsonl, records the live model on every
//     assistant line as meta.turn_stats.model; the last such line wins.
//
// This is a best-effort, read-only filesystem inference. It needs read access
// to the KiroCrew data directory, and the JSONL layout is KiroCrew's, not a
// stable public contract: either may change. On any failure it returns
// "unknown" with an error, never a guess. Callers treat "unknown" as
// "model not determined", not as a model id.

// KiroModelUnknown is returned when no model can be resolved for a session.
const KiroModelUnknown = "unknown"

// EnvKiroCrewDir overrides the KiroCrew data directory (~/.kiro/crew). Tests
// and operators set it; the hook never does.
const EnvKiroCrewDir = "DOWNSHIFT_KIROCREW_DIR"

// maxJSONLBytes bounds how much of one session log is scanned. Session logs
// grow without limit; the model only needs the last assistant turn, so the log
// is read line by line with a bounded scan buffer rather than loaded whole.
const maxJSONLBytes = 64 << 10

var errKiroModelUnresolved = errors.New("kirocrew: model not resolved for session")

// kiroModelCache memoizes resolution per session id. The routing hot path can
// ask for the same session many times; the JSONL is read at most once per id
// per process. A resolved "" (unknown) is cached too, so a missing session is
// not re-walked on every call.
var kiroModelCache = struct {
	sync.Mutex
	m map[string]string
}{m: map[string]string{}}

// InferKiroCrewModel resolves the model id for a KiroCrew session id, reading
// the gateway's session files under ~/.kiro/crew (or $DOWNSHIFT_KIROCREW_DIR).
// It returns the model id, or ("unknown", error) when the session cannot be
// mapped to a model. Results are cached per session id.
func InferKiroCrewModel(sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return KiroModelUnknown, errKiroModelUnresolved
	}

	kiroModelCache.Lock()
	if cached, ok := kiroModelCache.m[sessionID]; ok {
		kiroModelCache.Unlock()
		if cached == "" {
			return KiroModelUnknown, errKiroModelUnresolved
		}
		return cached, nil
	}
	kiroModelCache.Unlock()

	model := resolveKiroModel(crewDir(), sessionID)

	kiroModelCache.Lock()
	kiroModelCache.m[sessionID] = model
	kiroModelCache.Unlock()

	if model == "" {
		return KiroModelUnknown, errKiroModelUnresolved
	}
	return model, nil
}

// ResetKiroModelCache clears the per-session memo. Tests use it; production
// never needs it because a session id maps to one model for its lifetime.
func ResetKiroModelCache() {
	kiroModelCache.Lock()
	kiroModelCache.m = map[string]string{}
	kiroModelCache.Unlock()
}

// crewDir is the KiroCrew data directory: $DOWNSHIFT_KIROCREW_DIR, else
// ~/.kiro/crew. An empty string (no home) makes every lookup miss, which
// resolves to "unknown" rather than an error exit.
func crewDir() string {
	if d := os.Getenv(EnvKiroCrewDir); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kiro", "crew")
}

// resolveKiroModel returns a model id or "" (unknown). It never errors: the
// caller maps "" to the sentinel error so one failed walk cannot crash a hook.
func resolveKiroModel(crew, sessionID string) string {
	if crew == "" {
		return ""
	}
	state, ok := findKiroState(crew, sessionID)
	if !ok {
		return ""
	}
	// A concrete model recorded on the state file is the most direct signal.
	if m := cleanModel(state.ResolvedModel); m != "" {
		return m
	}
	if m := cleanModel(state.RequestedModel); m != "" {
		return m
	}
	// Otherwise follow the parent chat session's log to the live model.
	if state.ParentSession == "" {
		return ""
	}
	return modelFromSessionLog(crew, state.ParentSession)
}

// kiroState is the subset of state.json this inference reads.
type kiroState struct {
	SessionID      string `json:"session_id"`
	ParentSession  string `json:"parent_session"`
	ResolvedModel  string `json:"resolved_model"`
	RequestedModel string `json:"requested_model"`
}

// findKiroState scans subagents/*/state.json for the one whose session_id
// matches. The directory is small (one entry per live/recent subagent).
func findKiroState(crew, sessionID string) (kiroState, bool) {
	entries, err := os.ReadDir(filepath.Join(crew, "subagents"))
	if err != nil {
		return kiroState{}, false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(crew, "subagents", e.Name(), "state.json")
		data, err := readBounded(path)
		if err != nil {
			continue
		}
		var st kiroState
		if json.Unmarshal(data, &st) != nil {
			continue
		}
		if st.SessionID == sessionID {
			return st, true
		}
	}
	return kiroState{}, false
}

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
// lifetime; the last assistant turn is the current one. Reads are bounded and
// confined to the sessions directory.
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

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), maxJSONLBytes)
	last := ""
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec struct {
			Role string `json:"role"`
			Meta struct {
				TurnStats struct {
					Model string `json:"model"`
				} `json:"turn_stats"`
			} `json:"meta"`
		}
		if json.Unmarshal(line, &rec) != nil {
			continue
		}
		if rec.Role != "assistant" {
			continue
		}
		if m := cleanModel(rec.Meta.TurnStats.Model); m != "" {
			last = m
		}
	}
	return last
}

// cleanModel normalizes a model field and rejects values that are not a model
// id: empty, KiroCrew's "auto" router, and the "opusplan" planner alias, which
// are routing modes rather than a concrete model. The returned id must pass the
// shared discovery id shape so it can never carry control characters.
func cleanModel(s string) string {
	s = strings.TrimSpace(s)
	switch s {
	case "", "auto", "opusplan":
		return ""
	}
	if !modelIDPattern.MatchString(s) {
		return ""
	}
	return s
}
