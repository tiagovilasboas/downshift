// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

// StateFileName is the gateway switch file in Downshift's state directory.
const StateFileName = "gateway.json"

// SupportedName is the one gateway Downshift routes workers through.
const SupportedName = "9router"

// State is the user's gateway switch. Only delegate workers read it; native
// harness subagents cannot be routed through a gateway.
type State struct {
	Enabled  bool   `json:"enabled"`
	Name     string `json:"name,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	// Retired is set on load when the file names a gateway Downshift no longer
	// supports (for example omniroute); the switch then reads as off.
	Retired bool `json:"-"`
}

// StatePath resolves gateway.json in the state directory.
func StatePath() (string, error) {
	return paths.Join(StateFileName)
}

// LoadState reads the switch. A missing file is the zero State (off).
func LoadState() (State, error) {
	p, err := StatePath()
	if err != nil {
		return State{}, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return State{}, err
	}
	if s.Name != "" && s.Name != SupportedName {
		s.Enabled, s.Retired = false, true
	}
	return s, nil
}

// SaveState writes the switch, creating the state directory when needed.
func SaveState(s State) error {
	p, err := StatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0o600)
}

// WorkerEnv returns the environment entries that point a delegate worker of the
// given agent at the enabled gateway: ANTHROPIC_BASE_URL for claude (no /v1),
// OPENAI_BASE_URL for codex (with /v1). It is nil when the gateway is off, the
// endpoint is empty or the agent is unknown. It never carries an API key.
func WorkerEnv(s State, agent string) []string {
	base := strings.TrimRight(strings.TrimSpace(s.Endpoint), "/")
	base = strings.TrimSuffix(base, "/v1")
	if !s.Enabled || base == "" {
		return nil
	}
	switch agent {
	case "claude":
		return []string{"ANTHROPIC_BASE_URL=" + base}
	case "codex":
		return []string{"OPENAI_BASE_URL=" + base + "/v1"}
	}
	return nil
}
