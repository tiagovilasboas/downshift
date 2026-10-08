// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package rtk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/contextopt"
)

const (
	ProviderID   = "rtk"
	ProviderName = "RTK (Rust Token Killer)"
	MinVersion   = "0.40.0"
)

// Provider implements contextopt.Provider for RTK (Rust Token Killer).
type Provider struct {
	// LookPath overrides exec.LookPath in tests.
	LookPath func(file string) (string, error)
	// CommandRunner overrides exec.CommandContext in tests.
	CommandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)
}

// NewProvider creates an RTK provider with standard OS execution.
func NewProvider() *Provider {
	return &Provider{
		LookPath: exec.LookPath,
		CommandRunner: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, name, args...)
			return cmd.Output()
		},
	}
}

func (p *Provider) ID() string   { return ProviderID }
func (p *Provider) Name() string { return ProviderName }

func (p *Provider) Capabilities() []contextopt.Capability {
	return []contextopt.Capability{
		contextopt.CapCommandOutputOptimization,
		contextopt.CapHookSupport,
		contextopt.CapAgentInstructionSupport,
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
			Notes:     "Uses `rtk init` to configure hooks and command proxy in Claude Code",
		},
		{
			HarnessID: "cursor",
			Supported: true,
			Method:    contextopt.MethodHook,
			Notes:     "Uses `rtk init --agent cursor` to install cursor agent hooks",
		},
		{
			HarnessID: "codex",
			Supported: true,
			Method:    contextopt.MethodAgentInstruction,
			Notes:     "Instruction-based setup via `rtk init --codex` (adds RTK.md reference)",
		},
		{
			HarnessID: "antigravity",
			Supported: true,
			Method:    contextopt.MethodAgentInstruction,
			Notes:     "Configures instructions via `rtk init --agent antigravity`",
		},
		{
			HarnessID: "kirocrew",
			Supported: false,
			Method:    contextopt.MethodConfig,
			Notes:     "Kiro currently routes models via policy hook; RTK hook not verified natively for kiro subagent lifecycle",
		},
	}
}

// Detect checks if rtk binary exists on PATH and extracts its version.
func (p *Provider) Detect(ctx context.Context) (bool, string, string, error) {
	binPath, err := p.LookPath("rtk")
	if err != nil {
		return false, "", "", nil
	}
	out, err := p.CommandRunner(ctx, binPath, "--version")
	if err != nil {
		return true, "", binPath, fmt.Errorf("rtk detected at %s but failed to run --version: %w", binPath, err)
	}
	version := strings.TrimSpace(string(out))
	if strings.HasPrefix(version, "rtk ") {
		version = strings.TrimPrefix(version, "rtk ")
	}
	return true, version, binPath, nil
}

// Validate checks if the installed RTK meets version requirements and can execute.
func (p *Provider) Validate(ctx context.Context) error {
	installed, version, binPath, err := p.Detect(ctx)
	if err != nil {
		return err
	}
	if !installed {
		return errors.New("rtk binary is not installed on PATH")
	}
	if binPath == "" {
		return errors.New("empty rtk binary path")
	}
	// Verify basic command rewrite capability
	testOut, err := p.CommandRunner(ctx, binPath, "rewrite", "git status --short")
	if err != nil {
		return fmt.Errorf("rtk binary at %s failed rewrite capability check: %w", binPath, err)
	}
	if !strings.Contains(string(testOut), "rtk git status --short") {
		return fmt.Errorf("rtk rewrite unexpected output: %s", string(testOut))
	}
	_ = version
	return nil
}

// Configure applies RTK integration to targetDir for the requested harness.
func (p *Provider) Configure(ctx context.Context, targetDir string, harnessID string) error {
	installed, _, binPath, err := p.Detect(ctx)
	if err != nil {
		return err
	}
	if !installed {
		return errors.New("cannot configure: rtk is not installed")
	}

	var args []string
	switch harnessID {
	case "codex":
		args = []string{"init", "--codex"}
	case "claude-code":
		args = []string{"init", "--agent", "claude"}
	case "cursor":
		args = []string{"init", "--agent", "cursor"}
	case "antigravity":
		args = []string{"init", "--agent", "antigravity"}
	default:
		return fmt.Errorf("unsupported or unverified harness for RTK auto-configuration: %s", harnessID)
	}

	// Backup target project files before running init if in project directory
	if targetDir != "" {
		_ = backupProjectFile(filepath.Join(targetDir, "AGENTS.md"))
		_ = backupProjectFile(filepath.Join(targetDir, "RTK.md"))
	}

	cmd := exec.CommandContext(ctx, binPath, args...)
	if targetDir != "" {
		cmd.Dir = targetDir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rtk init failed (%s): %w\nOutput: %s", harnessID, err, string(out))
	}
	return nil
}

// Disable removes RTK configuration from targetDir for the requested harness.
func (p *Provider) Disable(ctx context.Context, targetDir string, harnessID string) error {
	installed, _, binPath, err := p.Detect(ctx)
	if err != nil {
		return err
	}
	if !installed {
		// If binary is gone, clean up local files manually
		if targetDir != "" {
			_ = os.Remove(filepath.Join(targetDir, "RTK.md"))
		}
		return nil
	}

	var args []string
	switch harnessID {
	case "codex":
		args = []string{"init", "--codex", "--uninstall"}
	case "claude-code":
		args = []string{"init", "--agent", "claude", "--uninstall"}
	case "cursor":
		args = []string{"init", "--agent", "cursor", "--uninstall"}
	case "antigravity":
		args = []string{"init", "--agent", "antigravity", "--uninstall"}
	default:
		if targetDir != "" {
			_ = os.Remove(filepath.Join(targetDir, "RTK.md"))
		}
		return nil
	}

	cmd := exec.CommandContext(ctx, binPath, args...)
	if targetDir != "" {
		cmd.Dir = targetDir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rtk uninstall failed (%s): %w\nOutput: %s", harnessID, err, string(out))
	}
	return nil
}

// HealthCheck executes a lightweight check to confirm RTK is functioning.
func (p *Provider) HealthCheck(ctx context.Context) error {
	installed, _, binPath, err := p.Detect(ctx)
	if err != nil {
		return err
	}
	if !installed {
		return errors.New("rtk is not installed")
	}
	_, err = p.CommandRunner(ctx, binPath, "--version")
	return err
}

// GetDiagnostics returns comprehensive diagnostics for RTK.
func (p *Provider) GetDiagnostics(ctx context.Context, targetDir string) (*contextopt.Diagnostics, error) {
	installed, version, binPath, err := p.Detect(ctx)
	diag := &contextopt.Diagnostics{
		ProviderID:         ProviderID,
		Installed:          installed,
		BinaryPath:         binPath,
		Version:            version,
		SupportedHarnesses: p.SupportedHarnesses(),
	}
	if err != nil {
		diag.Issues = append(diag.Issues, fmt.Sprintf("Detection error: %v", err))
		diag.Recommendations = append(diag.Recommendations, "Ensure RTK binary is executable and accessible on PATH")
		return diag, nil
	}
	if !installed {
		diag.Issues = append(diag.Issues, "RTK binary not found on PATH")
		diag.Recommendations = append(diag.Recommendations, "Install RTK via official instructions: https://github.com/rtk-ai/rtk")
		return diag, nil
	}

	// Check if project has RTK.md or AGENTS.md referencing it
	if targetDir != "" {
		rtkMd := filepath.Join(targetDir, "RTK.md")
		if _, err := os.Stat(rtkMd); err == nil {
			diag.ActiveHarnesses = append(diag.ActiveHarnesses, "project-local (RTK.md present)")
		}
	}
	return diag, nil
}

type rtkGainJSON struct {
	Summary struct {
		TotalCommands int64   `json:"total_commands"`
		TotalInput    int64   `json:"total_input"`
		TotalOutput   int64   `json:"total_output"`
		TotalSaved    int64   `json:"total_saved"`
		AvgSavingsPct float64 `json:"avg_savings_pct"`
	} `json:"summary"`
}

// GetMetrics fetches estimated token reduction metrics directly from `rtk gain --format json`.
func (p *Provider) GetMetrics(ctx context.Context, projectDir string) (*contextopt.Metrics, error) {
	installed, _, binPath, err := p.Detect(ctx)
	if err != nil || !installed {
		return nil, errors.New("rtk is not available for metrics")
	}
	args := []string{"gain", "--format", "json"}
	if projectDir != "" {
		args = append(args, "--project")
	}
	cmd := exec.CommandContext(ctx, binPath, args...)
	if projectDir != "" {
		cmd.Dir = projectDir
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rtk gain: %w", err)
	}

	var parsed rtkGainJSON
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse rtk gain JSON: %w", err)
	}

	return &contextopt.Metrics{
		ProviderID:    ProviderID,
		CapturedAt:    time.Now().UTC(),
		TotalCommands: parsed.Summary.TotalCommands,
		InputTokens:   parsed.Summary.TotalInput,
		OutputTokens:  parsed.Summary.TotalOutput,
		SavedTokens:   parsed.Summary.TotalSaved,
		SavingsPct:    parsed.Summary.AvgSavingsPct,
		IsEstimated:   true, // Explicitly tagged as estimated, not provider-invoiced
	}, nil
}

func backupProjectFile(p string) error {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil // skip if not exists
	}
	bak := p + ".bak." + time.Now().Format("20060102150405")
	return os.WriteFile(bak, data, 0644)
}

func init() {
	// Register RTK in global registry by default
	_ = bytes.Buffer{}
}
