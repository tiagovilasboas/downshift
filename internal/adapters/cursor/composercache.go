// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// Cursor Task preToolUse often omits model and model_id (live 3.27.12) while the
// common hook schema still carries them on sessionStart and beforeSubmitPrompt.
// Parent JSONL transcripts use role/message and carry no model id. This cache
// remembers the last composer model per session_id/conversation_id.

type composerEntry struct {
	ModelID   string `json:"model_id,omitempty"`
	Model     string `json:"model,omitempty"`
	UpdatedAt string `json:"updated_at"`
}

var composerMu sync.Mutex

func sessionKey(ev Event) string {
	if ev.SessionID != "" {
		return ev.SessionID
	}
	return ev.ConversationID
}

func touchComposer(ev Event) {
	key := sessionKey(ev)
	if key == "" {
		return
	}
	rememberComposer(key, ev.ModelID, ev.Model)
}

// RememberFromHook stores composer model fields from any Cursor hook that
// carries the common schema (sessionStart, beforeSubmitPrompt, preToolUse).
func RememberFromHook(ev Event) { touchComposer(ev) }

func rememberComposer(key, modelID, model string) {
	if key == "" || (modelID == "" && model == "") {
		return
	}
	composerMu.Lock()
	defer composerMu.Unlock()
	m, _ := loadComposerFile()
	m[key] = composerEntry{
		ModelID:   modelID,
		Model:     model,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	_ = saveComposerFile(m)
}

func recalledComposer(key string) string {
	if key == "" {
		return ""
	}
	composerMu.Lock()
	defer composerMu.Unlock()
	m, _ := loadComposerFile()
	e, ok := m[key]
	if !ok {
		return ""
	}
	if e.ModelID != "" {
		return e.ModelID
	}
	return e.Model
}

func loadComposerFile() (map[string]composerEntry, error) {
	dir, err := paths.StateDir()
	if err != nil {
		return map[string]composerEntry{}, err
	}
	path := filepath.Join(dir, "cursor-composer-model.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]composerEntry{}, err
	}
	var m map[string]composerEntry
	if json.Unmarshal(data, &m) != nil {
		return map[string]composerEntry{}, nil
	}
	return m, nil
}

func saveComposerFile(m map[string]composerEntry) error {
	dir, err := paths.StateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "cursor-composer-model.json"), data, 0o600)
}

// TouchFromStop records the parent composer model Cursor sends on subagentStop.
func TouchFromStop(sessionID, modelID, model string) {
	rememberComposer(sessionID, modelID, model)
}
