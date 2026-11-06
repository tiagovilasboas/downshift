// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package headroom_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/contextopt/providers/headroom"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

func fake(installed bool, version string) *headroom.Provider {
	return &headroom.Provider{
		LookPath: func(file string) (string, error) {
			if installed && file == headroom.Binary {
				return "/usr/local/bin/headroom", nil
			}
			return "", errors.New("not found")
		},
		CommandRunner: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(version), nil
		},
	}
}

func TestHeadroom_IsCompanionAndNeverProvider(t *testing.T) {
	p := fake(true, "headroom 1.4.2\n")
	if !contextopt.IsCompanion(p) {
		t.Fatal("headroom must be a companion")
	}
	if p.HasCapability(contextopt.CapCommandOutputOptimization) {
		t.Error("headroom does not take over the in-process command-output path")
	}
	if !p.HasCapability(contextopt.CapToolResponseOptimization) {
		t.Error("expected tool response optimization capability")
	}
}

func TestHeadroom_DetectParsesVersion(t *testing.T) {
	ctx := context.Background()
	installed, ver, path, err := fake(true, "headroom, version 1.4.2\n").Detect(ctx)
	if err != nil || !installed || ver != "1.4.2" || path != "/usr/local/bin/headroom" {
		t.Fatalf("detect = %v %q %q %v", installed, ver, path, err)
	}
	installed, ver, path, err = fake(false, "").Detect(ctx)
	if err != nil || installed || ver != "" || path != "" {
		t.Fatalf("absent detect = %v %q %q %v", installed, ver, path, err)
	}
}

func TestHeadroom_ValidateNeedsBinary(t *testing.T) {
	err := fake(false, "").Validate(context.Background())
	if err == nil || !strings.Contains(err.Error(), "headroom-ai") {
		t.Fatalf("expected install hint, got %v", err)
	}
	if err := fake(true, "1.0.0").Validate(context.Background()); err != nil {
		t.Fatalf("installed must validate: %v", err)
	}
}

// Configure records the companion only. Provider stays native and nothing
// else in the config moves; Disable clears only the companion.
func TestHeadroom_ConfigureIsBookkeepingOnly(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	ctx := context.Background()
	p := fake(true, "1.0.0")

	if err := p.Configure(ctx, t.TempDir(), "claude-code"); err != nil {
		t.Fatal(err)
	}
	cfg, err := contextopt.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Companion != "headroom" || cfg.Provider != "native" || !cfg.Enabled {
		t.Fatalf("after enable: %+v", cfg)
	}

	diag, _ := p.GetDiagnostics(ctx, "")
	if !diag.Installed || len(diag.Issues) != 0 {
		t.Fatalf("diag = %+v", diag)
	}
	if !strings.Contains(strings.Join(diag.Recommendations, "\n"), "HEADROOM_BEACON=off") {
		t.Error("privacy note must be in the recommendations")
	}

	if err := p.Disable(ctx, t.TempDir(), "claude-code"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = contextopt.LoadConfig()
	if cfg.Companion != "" || cfg.Provider != "native" || !cfg.Enabled {
		t.Fatalf("after disable: %+v", cfg)
	}
}

func TestHeadroom_DiagnosticsFlagEnabledButMissing(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	ctx := context.Background()
	if err := fake(true, "1.0.0").Configure(ctx, "", ""); err != nil {
		t.Fatal(err)
	}
	diag, _ := fake(false, "").GetDiagnostics(ctx, "")
	if diag.Installed || len(diag.Issues) == 0 || !strings.Contains(diag.Issues[0], "not on PATH") {
		t.Fatalf("diag = %+v", diag)
	}
	m, _ := fake(false, "").GetMetrics(ctx, "")
	if m.TokenState != "unavailable" || m.SavedTokens != 0 {
		t.Fatalf("metrics must stay unavailable, got %+v", m)
	}
}

func TestHeadroom_SupportTableCoversEveryHookHarness(t *testing.T) {
	want := map[string]bool{"claude-code": true, "cursor": true, "codex": true, "antigravity": true, "kirocrew": true}
	for _, h := range fake(true, "").SupportedHarnesses() {
		delete(want, h.HarnessID)
	}
	if len(want) != 0 {
		t.Fatalf("missing harness rows: %v", want)
	}
}
