// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package paths resolves Downshift's local state directory with legacy fallback.
package paths

import (
	"os"
	"path/filepath"
)

const (
	// EnvStateDir overrides the state directory (absolute path).
	EnvStateDir = "DOWNSHIFT_STATE_DIR"
	DirName     = ".downshift"
	LegacyName  = ".harness-downshift"
)

// Source describes how StateDir was chosen.
type Source string

const (
	SourceEnv     Source = "env"
	SourceDefault Source = "default"
	SourceLegacy  Source = "legacy"
)

// Resolve returns the effective state directory and how it was chosen.
func Resolve() (dir string, source Source, err error) {
	if v := os.Getenv(EnvStateDir); v != "" {
		return v, SourceEnv, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	newDir := filepath.Join(home, DirName)
	legacyDir := filepath.Join(home, LegacyName)
	if isDir(newDir) {
		return newDir, SourceDefault, nil
	}
	if isDir(legacyDir) {
		return legacyDir, SourceLegacy, nil
	}
	return newDir, SourceDefault, nil
}

// StateDir is the effective local state directory.
func StateDir() (string, error) {
	dir, _, err := Resolve()
	return dir, err
}

// Join builds a path under the effective state directory.
func Join(elem ...string) (string, error) {
	root, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, filepath.Join(elem...)), nil
}

// EventsPath is events.jsonl for routing telemetry.
func EventsPath() (string, error) { return Join("events.jsonl") }

// AgentsPath is agents.jsonl for spawn-side logging.
func AgentsPath() (string, error) { return Join("agents.jsonl") }

// CatalogPath is the user catalog override file.
func CatalogPath() (string, error) { return Join("catalog.json") }

// SessionModelsPath is session-models.json unless DOWNSHIFT_SESSION_MODELS is set.
func SessionModelsPath() (string, error) {
	if p := os.Getenv("DOWNSHIFT_SESSION_MODELS"); p != "" {
		return p, nil
	}
	return Join("session-models.json")
}

// LoopEventsPath is loop-events.jsonl for routing v2 training events.
func LoopEventsPath() (string, error) { return Join("loop-events.jsonl") }

// WeightsPath is weights.json for classifier overrides.
func WeightsPath() (string, error) { return Join("weights.json") }

// AdaptMemoryPath is adapt-memory.json for local tier memory from feedback.
func AdaptMemoryPath() (string, error) { return Join("adapt-memory.json") }

// DisplaySessionModels returns the path shown in hook warnings.
func DisplaySessionModels() string {
	if p := os.Getenv("DOWNSHIFT_SESSION_MODELS"); p != "" {
		return p
	}
	return DisplayStateDir() + "/session-models.json"
}

// DisplayStateDir is a short human label for docs and stderr (tilde when under $HOME).
func DisplayStateDir() string {
	dir, err := StateDir()
	if err != nil {
		return "~/" + DirName
	}
	home, err := os.UserHomeDir()
	if err == nil {
		if rel, err := filepath.Rel(home, dir); err == nil && rel != ".." && !filepath.IsAbs(rel) {
			return "~/" + rel
		}
	}
	return dir
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
