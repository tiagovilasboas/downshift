// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"testing"
)

func TestConfigLoadAndSave(t *testing.T) {
	tempDir := t.TempDir()

	// Initial load returns default with ModeOff
	cfg, err := LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Mode != ModeOff {
		t.Errorf("Expected initial mode Off, got %v", cfg.Mode)
	}

	// Change mode to active
	if err := SetMode(tempDir, ModeActive); err != nil {
		t.Fatalf("SetMode failed: %v", err)
	}

	loaded, err := LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig after set failed: %v", err)
	}
	if loaded.Mode != ModeActive {
		t.Errorf("Expected mode Active, got %v", loaded.Mode)
	}
	if loaded.MaxContextBytes != 3500 {
		t.Errorf("Expected default MaxContextBytes 3500, got %d", loaded.MaxContextBytes)
	}
}
