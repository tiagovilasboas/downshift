// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contextopt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/contextopt/providers/native"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

func TestRegistry_RegistrationAndRetrieval(t *testing.T) {
	reg := contextopt.NewRegistry()
	p := native.NewProvider(compressor.ModeObserve)
	reg.Register(p)

	got, ok := reg.Get("native")
	if !ok {
		t.Fatalf("expected provider native to be found")
	}
	if got.Name() != p.Name() {
		t.Errorf("expected name %s, got %s", p.Name(), got.Name())
	}
	if len(reg.List()) != 1 {
		t.Errorf("expected 1 provider in list, got %d", len(reg.List()))
	}
}

func TestConfig_LoadSave(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(paths.EnvStateDir, tmp)

	cfg, err := contextopt.LoadConfig()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Enabled {
		t.Errorf("expected default enabled=false")
	}

	cfg.Enabled = true
	cfg.Provider = "native"
	cfg.ActiveHarness = []string{"codex", "claude-code"}
	if err := contextopt.SaveConfig(cfg); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := contextopt.LoadConfig()
	if err != nil {
		t.Fatalf("re-load failed: %v", err)
	}
	if !loaded.Enabled {
		t.Errorf("expected loaded enabled=true")
	}
	if loaded.Provider != "native" {
		t.Errorf("expected provider native, got %s", loaded.Provider)
	}
	if len(loaded.ActiveHarness) != 2 {
		t.Errorf("expected 2 active harnesses, got %d", len(loaded.ActiveHarness))
	}
}

func TestConfig_GracefulDegradationOnMissingDir(t *testing.T) {
	t.Setenv(paths.EnvStateDir, filepath.Join(os.TempDir(), "non-existent-dir-123456"))
	cfg, err := contextopt.LoadConfig()
	if err != nil {
		t.Fatalf("load should not fail on missing dir: %v", err)
	}
	if cfg.Enabled {
		t.Errorf("expected default disabled")
	}
}
