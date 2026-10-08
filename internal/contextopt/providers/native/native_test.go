// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package native_test

import (
	"context"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/contextopt/providers/native"
)

func TestNativeProvider_Basics(t *testing.T) {
	p := native.NewProvider(compressor.ModeSafe)

	if p.ID() != "native" {
		t.Fatalf("expected ID native, got %s", p.ID())
	}
	if p.Name() != "Downshift Native Context Compressor" {
		t.Fatalf("expected Name Downshift Native Context Compressor, got %s", p.Name())
	}

	if !p.HasCapability(contextopt.CapCommandOutputOptimization) {
		t.Errorf("expected CapCommandOutputOptimization")
	}
	if p.HasCapability(contextopt.CapHookSupport) {
		t.Errorf("expected CapHookSupport to be false for pure native filter")
	}

	ctx := context.Background()
	installed, ver, path, err := p.Detect(ctx)
	if !installed || err != nil || ver != "built-in" || path != "in-process" {
		t.Errorf("unexpected detect output: installed=%v, ver=%s, path=%s, err=%v", installed, ver, path, err)
	}

	if err := p.Validate(ctx); err != nil {
		t.Errorf("expected Validate to succeed: %v", err)
	}

	if err := p.HealthCheck(ctx); err != nil {
		t.Errorf("expected HealthCheck to succeed: %v", err)
	}

	if err := p.Configure(ctx, t.TempDir(), "claude-code"); err != nil {
		t.Errorf("expected Configure to succeed: %v", err)
	}

	if err := p.Disable(ctx, t.TempDir(), "claude-code"); err != nil {
		t.Errorf("expected Disable to succeed: %v", err)
	}

	diag, err := p.GetDiagnostics(ctx, t.TempDir())
	if err != nil || diag == nil || diag.ProviderID != "native" || !diag.Installed {
		t.Errorf("unexpected diagnostics: diag=%+v, err=%v", diag, err)
	}

	metrics, err := p.GetMetrics(ctx, t.TempDir())
	if err != nil || metrics == nil || metrics.ProviderID != "native" {
		t.Errorf("unexpected metrics: metrics=%+v, err=%v", metrics, err)
	}
}
