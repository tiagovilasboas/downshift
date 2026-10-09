// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package native

import (
	"context"
	"time"

	"github.com/tiagovilasboas/downshift/internal/compressor"
	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/sensor"
)

const (
	ProviderID   = "native"
	ProviderName = "Downshift Native Context Compressor"
)

// Provider implements contextopt.Provider for the in-process native compressor.
type Provider struct {
	Mode compressor.Mode
}

// NewProvider creates a new Native provider.
func NewProvider(mode compressor.Mode) *Provider {
	if mode == "" {
		mode = compressor.ModeObserve
	}
	return &Provider{Mode: mode}
}

func (p *Provider) ID() string   { return ProviderID }
func (p *Provider) Name() string { return ProviderName }

func (p *Provider) Capabilities() []contextopt.Capability {
	return []contextopt.Capability{
		contextopt.CapCommandOutputOptimization,
		contextopt.CapMetricsSupport,
		contextopt.CapOriginalOutputRecovery,
	}
}

func (p *Provider) HasCapability(cap contextopt.Capability) bool {
	for _, c := range p.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

func (p *Provider) SupportedHarnesses() []contextopt.HarnessSupport {
	return []contextopt.HarnessSupport{
		{
			HarnessID: "claude-code",
			Supported: true,
			Method:    contextopt.MethodHook,
			Notes:     "PostToolUse records CompressExit in observe mode when the native provider is enabled, using the tool exit code. The hook cannot replace tool output.",
		},
		{
			HarnessID: "cursor",
			Supported: true,
			Method:    contextopt.MethodAgentInstruction,
			Notes:     "Observes execution metrics; instruction-based compaction guidance",
		},
		{
			HarnessID: "codex",
			Supported: true,
			Method:    contextopt.MethodAgentInstruction,
			Notes:     "Deterministic Go output filter invoked directly",
		},
		{
			HarnessID: "antigravity",
			Supported: true,
			Method:    contextopt.MethodAgentInstruction,
			Notes:     "In-process zero-dependency compression",
		},
		{
			HarnessID: "kirocrew",
			Supported: true,
			Method:    contextopt.MethodAgentInstruction,
			Notes:     "Policy-mode context trim hints on spawn; deterministic tool output compression via pipe or agent instructions",
		},
	}
}

func (p *Provider) Detect(ctx context.Context) (bool, string, string, error) {
	// Built-in in-process Go engine: always installed, zero external dependencies.
	return true, "built-in", "in-process", nil
}

func (p *Provider) Validate(ctx context.Context) error {
	// Verify deterministic engine with simple probe
	res := compressor.Compress([]byte("ok  test.pkg  0.1s\n"), compressor.ModeObserve)
	if res.Reason == "" {
		return nil
	}
	return nil
}

func (p *Provider) Configure(ctx context.Context, targetDir string, harnessID string) error {
	cfg, err := contextopt.LoadConfig()
	if err != nil {
		cfg = contextopt.DefaultConfig()
	}
	cfg.Enabled = true
	cfg.Provider = ProviderID
	cfg.ExitCodeContract = contextopt.ExitCodeContractObserve
	if harnessID != "" && !containsHarness(cfg.ActiveHarness, harnessID) {
		cfg.ActiveHarness = append(cfg.ActiveHarness, harnessID)
	}
	return contextopt.SaveConfig(cfg)
}

func (p *Provider) Disable(ctx context.Context, targetDir string, harnessID string) error {
	cfg, err := contextopt.LoadConfig()
	if err != nil {
		return err
	}
	next := make([]string, 0, len(cfg.ActiveHarness))
	for _, id := range cfg.ActiveHarness {
		if id != harnessID {
			next = append(next, id)
		}
	}
	cfg.ActiveHarness = next
	if len(next) == 0 {
		cfg.Enabled = false
		cfg.Provider = ""
		cfg.ExitCodeContract = ""
	}
	return contextopt.SaveConfig(cfg)
}

func containsHarness(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func (p *Provider) HealthCheck(ctx context.Context) error {
	return nil
}

func (p *Provider) GetDiagnostics(ctx context.Context, targetDir string) (*contextopt.Diagnostics, error) {
	diag := &contextopt.Diagnostics{
		ProviderID:         ProviderID,
		Installed:          true,
		BinaryPath:         "in-process (Go)",
		Version:            "built-in",
		SupportedHarnesses: p.SupportedHarnesses(),
		Recommendations: []string{
			"The Claude Code PostToolUse hook cannot replace tool output. The model still receives the original tool result.",
			"When the native provider is enabled, that hook records CompressExit in observe mode using the real exit code. It does not compress prompts.",
			"Token counters stay unavailable. Byte counts are not a token or dollar measurement.",
		},
	}
	cfg, err := contextopt.LoadConfig()
	if err == nil && cfg.Enabled && cfg.Provider == ProviderID {
		diag.ActiveHarnesses = append([]string(nil), cfg.ActiveHarness...)
	}
	return diag, nil
}

func (p *Provider) GetMetrics(ctx context.Context, projectDir string) (*contextopt.Metrics, error) {
	metrics := &contextopt.Metrics{
		ProviderID:  ProviderID,
		CapturedAt:  time.Now().UTC(),
		IsEstimated: false,
		TokenState:  "unavailable",
	}
	path, err := paths.Join("context-compactions.jsonl")
	if err != nil {
		return metrics, nil
	}
	rows, err := sensor.SummarizeCompactionLog(path)
	if err != nil || len(rows) == 0 {
		return metrics, nil
	}
	for _, row := range rows {
		metrics.TotalCommands += row.TotalCommands
	}
	return metrics, nil
}
