// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package rtk

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/contextopt"
)

func TestRTKProvider_Capabilities(t *testing.T) {
	p := NewProvider()
	if p.ID() != "rtk" {
		t.Fatalf("expected id rtk, got %s", p.ID())
	}
	if !p.HasCapability(contextopt.CapCommandOutputOptimization) {
		t.Errorf("expected CapCommandOutputOptimization")
	}
	if !p.HasCapability(contextopt.CapMetricsSupport) {
		t.Errorf("expected CapMetricsSupport")
	}
	if p.HasCapability(contextopt.CapToolResponseOptimization) {
		t.Errorf("did not expect CapToolResponseOptimization")
	}
}

func TestRTKProvider_DetectMock(t *testing.T) {
	p := NewProvider()
	p.LookPath = func(file string) (string, error) {
		if file == "rtk" {
			return "/fake/bin/rtk", nil
		}
		return "", errors.New("not found")
	}
	p.CommandRunner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "/fake/bin/rtk" && len(args) == 1 && args[0] == "--version" {
			return []byte("rtk 0.44.2\n"), nil
		}
		return nil, errors.New("unexpected command")
	}

	installed, ver, path, err := p.Detect(context.Background())
	if err != nil {
		t.Fatalf("detect failed: %v", err)
	}
	if !installed {
		t.Fatalf("expected installed=true")
	}
	if ver != "0.44.2" {
		t.Errorf("expected ver 0.44.2, got %s", ver)
	}
	if path != "/fake/bin/rtk" {
		t.Errorf("expected path /fake/bin/rtk, got %s", path)
	}
}

func TestRTKProvider_ValidateMock(t *testing.T) {
	p := NewProvider()
	p.LookPath = func(file string) (string, error) {
		return "/fake/bin/rtk", nil
	}
	p.CommandRunner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if len(args) == 1 && args[0] == "--version" {
			return []byte("rtk 0.44.2\n"), nil
		}
		if len(args) == 2 && args[0] == "rewrite" && args[1] == "git status --short" {
			return []byte("rtk git status --short\n"), nil
		}
		return nil, errors.New("fail")
	}

	if err := p.Validate(context.Background()); err != nil {
		t.Fatalf("expected validation success, got: %v", err)
	}
}

func TestRTKProvider_GetDiagnostics(t *testing.T) {
	tmp := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmp, "RTK.md"), []byte("# RTK"), 0644)

	p := NewProvider()
	p.LookPath = func(file string) (string, error) {
		return "/fake/bin/rtk", nil
	}
	p.CommandRunner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("rtk 0.44.2\n"), nil
	}

	diag, err := p.GetDiagnostics(context.Background(), tmp)
	if err != nil {
		t.Fatalf("diagnostics failed: %v", err)
	}
	if !diag.Installed {
		t.Errorf("expected installed=true")
	}
	if len(diag.ActiveHarnesses) == 0 {
		t.Errorf("expected active harness detection for project-local RTK.md")
	}
}
