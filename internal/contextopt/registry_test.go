// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contextopt_test

import (
	"os"
	"path/filepath"
	"runtime"
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
	if !cfg.Enabled {
		t.Errorf("expected default enabled=true")
	}
	if cfg.Provider != "native" {
		t.Errorf("expected default provider native, got %s", cfg.Provider)
	}

	cfg.Enabled = false
	cfg.Provider = ""
	if err := contextopt.SaveConfig(cfg); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := contextopt.LoadConfig()
	if err != nil {
		t.Fatalf("re-load failed: %v", err)
	}
	if loaded.Enabled {
		t.Errorf("expected loaded enabled=false after explicit save")
	}
}

func TestConfig_GracefulDegradationOnMissingDir(t *testing.T) {
	t.Setenv(paths.EnvStateDir, filepath.Join(os.TempDir(), "non-existent-dir-123456"))
	cfg, err := contextopt.LoadConfig()
	if err != nil {
		t.Fatalf("load should not fail on missing dir: %v", err)
	}
	if !cfg.Enabled {
		t.Errorf("expected default enabled=true on missing dir")
	}
}

// First write on a clean home: the state directory is private and so is the
// config (it used to be 0755 and 0644).
func TestSaveConfigCreatesAPrivateStateDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DOWNSHIFT_STATE_DIR", "")
	if err := contextopt.SaveConfig(contextopt.DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	p, _ := contextopt.ConfigPath()
	for path, want := range map[string]os.FileMode{filepath.Dir(p): 0o700, p: 0o600} {
		fi, err := os.Stat(path)
		if err != nil || fi.Mode().Perm() != want {
			t.Errorf("%s = %v %v, want %v", path, fi, err, want)
		}
	}
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 {
		t.Errorf("leftover temp files: %v", entries)
	}
}
