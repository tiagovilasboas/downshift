// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	ConfigFileName = "dci.json"
)

// ConfigPath returns the path to the DCI configuration file in repoRoot.
func ConfigPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".downshift", ConfigFileName)
}

// LoadConfig loads the DCI configuration from repoRoot/.downshift/dci.json.
// If the configuration file does not exist, DefaultConfig() is returned without error.
func LoadConfig(repoRoot string) (Config, error) {
	path := ConfigPath(repoRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), fmt.Errorf("reading dci config: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("parsing dci config: %w", err)
	}

	// Validate mode
	switch cfg.Mode {
	case ModeOff, ModeObserve, ModeActive:
	default:
		cfg.Mode = ModeOff
	}

	return cfg, nil
}

// SaveConfig atomically writes the DCI configuration to repoRoot/.downshift/dci.json
// with secure permissions (0600 file, 0700 dir).
func SaveConfig(repoRoot string, cfg Config) error {
	path := ConfigPath(repoRoot)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling dci config: %w", err)
	}

	return WriteSecureFile(path, append(data, '\n'))
}

// SetMode updates the operational mode in repoRoot/.downshift/dci.json.
func SetMode(repoRoot string, mode Mode) error {
	cfg, err := LoadConfig(repoRoot)
	if err != nil {
		cfg = DefaultConfig()
	}
	cfg.Mode = mode
	return SaveConfig(repoRoot, cfg)
}
