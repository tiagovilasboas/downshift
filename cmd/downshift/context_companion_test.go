// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/contextopt/providers/headroom"
	"github.com/tiagovilasboas/downshift/internal/contextopt/providers/native"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

func fakeHeadroom(installed bool) *headroom.Provider {
	return &headroom.Provider{
		LookPath: func(string) (string, error) {
			if installed {
				return "/opt/bin/headroom", nil
			}
			return "", errors.New("not found")
		},
		CommandRunner: func(context.Context, string, ...string) ([]byte, error) { return []byte("headroom 2.0.0"), nil },
	}
}

func useRegistry(t *testing.T, installed bool) {
	t.Helper()
	t.Setenv(paths.EnvStateDir, t.TempDir())
	prev := contextRegistry
	contextRegistry = func() *contextopt.Registry {
		reg := contextopt.NewRegistry()
		reg.Register(native.NewProvider(compressor.ModeObserve))
		reg.Register(fakeHeadroom(installed))
		return reg
	}
	t.Cleanup(func() { contextRegistry = prev })
}

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	rc := runContext(args, &out, &errOut)
	return rc, out.String(), errOut.String()
}

// `enable headroom` without the binary refuses with the install hint and
// records nothing.
func TestContext_EnableCompanionNeedsBinary(t *testing.T) {
	useRegistry(t, false)
	rc, _, errOut := run(t, "enable", "headroom")
	if rc != 1 || !strings.Contains(errOut, "headroom-ai") {
		t.Fatalf("rc=%d stderr=%q", rc, errOut)
	}
	cfg, _ := contextopt.LoadConfig()
	if cfg.Companion != "" {
		t.Fatalf("nothing must be recorded, got %+v", cfg)
	}
}

// A detected companion is recorded by `enable` (default) and by
// `enable headroom`; the native provider stays the provider and the hook
// flag stays on. `disable headroom` clears only the companion.
func TestContext_CompanionRecordedByDefaultWhenDetected(t *testing.T) {
	useRegistry(t, true)
	rc, out, _ := run(t, "enable")
	if rc != 0 || !strings.Contains(out, "Companion headroom v2.0.0 detected and recorded") || !strings.Contains(out, "headroom wrap claude") {
		t.Fatalf("rc=%d out=%s", rc, out)
	}
	cfg, _ := contextopt.LoadConfig()
	if cfg.Provider != "native" || cfg.Companion != "headroom" || !cfg.Enabled {
		t.Fatalf("cfg = %+v", cfg)
	}

	_, out, _ = run(t, "status")
	if !strings.Contains(out, "Active Provider: native") || !strings.Contains(out, "Companion: headroom (v2.0.0, recorded as active") {
		t.Fatalf("status = %s", out)
	}

	rc, out, _ = run(t, "disable", "headroom")
	if rc != 0 || !strings.Contains(out, "Companion headroom cleared") {
		t.Fatalf("rc=%d out=%s", rc, out)
	}
	cfg, _ = contextopt.LoadConfig()
	if cfg.Provider != "native" || cfg.Companion != "" || !cfg.Enabled {
		t.Fatalf("after disable headroom: %+v", cfg)
	}

	_, out, _ = run(t, "status")
	if !strings.Contains(out, "Companion available: headroom v2.0.0 detected, not recorded") {
		t.Fatalf("status must hint at the detected companion:\n%s", out)
	}

	rc, _, _ = run(t, "enable", "headroom")
	if rc != 0 {
		t.Fatal("enable headroom")
	}
	rc, _, _ = run(t, "disable")
	if rc != 0 {
		t.Fatal("disable")
	}
	cfg, _ = contextopt.LoadConfig()
	if cfg.Enabled || cfg.Companion != "" {
		t.Fatalf("plain disable clears everything, got %+v", cfg)
	}
}

func TestContext_DisableUnknownCompanion(t *testing.T) {
	useRegistry(t, true)
	if rc, _, _ := run(t, "disable", "native"); rc != 2 {
		t.Fatalf("native is not a companion, rc=%d", rc)
	}
}

func TestContext_ProvidersListsCompanion(t *testing.T) {
	useRegistry(t, false)
	_, out, _ := run(t, "providers")
	if !strings.Contains(out, "id: headroom") || !strings.Contains(out, "planned/unverified via config") {
		t.Fatalf("providers = %s", out)
	}
}
