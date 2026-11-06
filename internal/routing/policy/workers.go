// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// Worker modes: off launches nothing, read launches read-only workers, write
// also lets a worker change files.
const (
	WorkersOff   = "off"
	WorkersRead  = "read"
	WorkersWrite = "write"
)

// Where a worker setting came from.
const (
	WorkersFromEnv     = "env"
	WorkersFromUser    = "user"
	WorkersFromProject = "project"
	WorkersFromDefault = "default"
	WorkersFromMarker  = "marker"
)

const (
	// WorkersEnv is the user's override: off, read or bypass (write).
	WorkersEnv = "DOWNSHIFT_WORKERS"
	// WorkerMarkerEnv is set inside a worker so it cannot start workers of its own.
	WorkerMarkerEnv = "DOWNSHIFT_WORKER"
)

// DefaultWorkersMode is the mode when nothing else sets one. It applies only to
// verified worker agents and never on Windows; with none verified it is off.
const DefaultWorkersMode = WorkersRead

// Workers is the resolved worker setting.
type Workers struct {
	Mode   string   // off, read or write
	Agents []string // worker agents that may run; empty when Mode is off
	Source string   // env, user, project, default or marker
	Notes  []string // ignored values and why
}

// ResolveWorkers resolves the worker mode. Precedence: the marker (forces off),
// DOWNSHIFT_WORKERS, the user file's orchestration.workers, the project file
// (which may only lower the default), then the default. known lists every
// worker agent and verified the ones proven safe for the default; both come
// from the harness capabilities. An explicit env or user read/write enables
// every known agent. On Windows, or with no verified agent, the default is off.
func ResolveWorkers(env func(string) string, loc Locations, known, verified []string, goos string) Workers {
	if strings.TrimSpace(env(WorkerMarkerEnv)) == "1" {
		return Workers{Mode: WorkersOff, Source: WorkersFromMarker}
	}
	var notes []string
	def := DefaultWorkersMode
	if goos == "windows" || len(verified) == 0 {
		def = WorkersOff
	}
	if raw := strings.TrimSpace(env(WorkersEnv)); raw != "" {
		if m, ok := envWorkersMode(raw); ok {
			return workersResult(m, WorkersFromEnv, known, notes)
		}
		notes = append(notes, fmt.Sprintf("%s=%q ignored: want off, read or bypass", WorkersEnv, raw))
	}
	um, note := fileWorkers(loc.User)
	notes = appendNote(notes, note)
	if um != "" {
		return workersResult(um, WorkersFromUser, known, notes)
	}
	pm, note := fileWorkers(loc.Project)
	notes = appendNote(notes, note)
	if pm != "" {
		if workersRank(pm) <= workersRank(def) {
			return workersResult(pm, WorkersFromProject, verified, notes)
		}
		notes = append(notes, fmt.Sprintf("%s: orchestration.workers %q ignored: a project file can only lower the setting", loc.Project, pm))
	}
	return workersResult(def, WorkersFromDefault, verified, notes)
}

func workersResult(mode, source string, agents, notes []string) Workers {
	w := Workers{Mode: mode, Source: source, Notes: notes}
	if mode != WorkersOff {
		w.Agents = append([]string(nil), agents...)
	}
	return w
}

func appendNote(notes []string, note string) []string {
	if note == "" {
		return notes
	}
	return append(notes, note)
}

func workersRank(mode string) int {
	switch mode {
	case WorkersRead:
		return 1
	case WorkersWrite:
		return 2
	}
	return 0
}

func envWorkersMode(raw string) (string, bool) {
	switch strings.ToLower(raw) {
	case "off":
		return WorkersOff, true
	case "read":
		return WorkersRead, true
	case "bypass":
		return WorkersWrite, true
	}
	return "", false
}

// fileWorkers reads orchestration.workers from one policy file. An absent
// file, or a file without the key, holds no value. A file that exists but
// cannot be used (a symlink or other non-regular file, too large, unreadable,
// invalid JSON, or an invalid value) fails closed: it resolves to off with a
// note doctor shows, so a broken file never turns a user's "off" into the
// default read. Symlinks are refused the same way readLayer refuses them.
func fileWorkers(path string) (string, string) {
	if path == "" {
		return "", ""
	}
	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", ""
	}
	if err != nil {
		return workersUnusable(path, err.Error())
	}
	if !fi.Mode().IsRegular() {
		return workersUnusable(path, "not a regular file")
	}
	if fi.Size() > maxBytes {
		return workersUnusable(path, fmt.Sprintf("larger than %d bytes", maxBytes))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return workersUnusable(path, err.Error())
	}
	var doc struct {
		Orchestration json.RawMessage `json:"orchestration"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return workersUnusable(path, "invalid JSON")
	}
	if doc.Orchestration == nil {
		return "", ""
	}
	var o struct {
		Workers json.RawMessage `json:"workers"`
	}
	if json.Unmarshal(doc.Orchestration, &o) != nil {
		return workersUnusable(path, "orchestration must be an object")
	}
	if o.Workers == nil {
		return "", ""
	}
	var v string
	if json.Unmarshal(o.Workers, &v) == nil {
		switch v {
		case WorkersOff, WorkersRead, WorkersWrite:
			return v, ""
		}
	}
	return workersUnusable(path, fmt.Sprintf("orchestration.workers %s is not off, read or write", strings.TrimSpace(string(o.Workers))))
}

// workersUnusable is the fail-closed answer for a policy file that exists but
// cannot be read as a worker setting: off, with the reason.
func workersUnusable(path, why string) (string, string) {
	return WorkersOff, fmt.Sprintf("%s: %s; workers off until it is fixed", path, why)
}
