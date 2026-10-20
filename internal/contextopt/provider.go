// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package contextopt provides an opt-in, extensible framework for
// context optimization providers (e.g. native, external companions).
// Downshift remains strictly responsible for model routing and agent orchestration.
// Optimization providers handle tool-output or context compression.
//
// Two kinds of provider exist. The native provider is in-process and is the
// built-in default. A companion is an external tool the user installs and
// runs (for example a compression proxy); Downshift detects it, records that
// it is in use and points to its own commands, but never installs, launches
// or configures it on the user's behalf.
package contextopt

import (
	"context"
	"time"
)

// Capability defines a specific capability supported by a context optimization provider.
type Capability string

const (
	CapCommandOutputOptimization Capability = "command_output_optimization"
	CapToolResponseOptimization  Capability = "tool_response_optimization"
	CapContextRetrieval          Capability = "context_retrieval"
	CapSessionIntegration        Capability = "session_integration"
	CapHookSupport               Capability = "hook_support"
	CapAgentInstructionSupport   Capability = "agent_instruction_support"
	CapMetricsSupport            Capability = "metrics_support"
	CapOriginalOutputRecovery    Capability = "original_output_recovery"
)

// IntegrationMethod explains how a provider integrates with a specific harness.
type IntegrationMethod string

const (
	MethodHook             IntegrationMethod = "hook"
	MethodAgentInstruction IntegrationMethod = "agent_instruction"
	MethodEnvWrapper       IntegrationMethod = "env_wrapper"
	MethodConfig           IntegrationMethod = "config"
)

// HarnessSupport details how a harness is supported by a provider.
type HarnessSupport struct {
	HarnessID string            `json:"harness_id"`
	Supported bool              `json:"supported"`
	Method    IntegrationMethod `json:"method"`
	Notes     string            `json:"notes,omitempty"`
}

// Diagnostics contains operational diagnostics about a provider installation.
type Diagnostics struct {
	ProviderID         string           `json:"provider_id"`
	Installed          bool             `json:"installed"`
	BinaryPath         string           `json:"binary_path,omitempty"`
	Version            string           `json:"version,omitempty"`
	SupportedHarnesses []HarnessSupport `json:"supported_harnesses"`
	ActiveHarnesses    []string         `json:"active_harnesses,omitempty"`
	Issues             []string         `json:"issues,omitempty"`
	Recommendations    []string         `json:"recommendations,omitempty"`
}

// Metrics represents measured and estimated token reduction metrics.
type Metrics struct {
	ProviderID    string    `json:"provider_id"`
	CapturedAt    time.Time `json:"captured_at"`
	TotalCommands int64     `json:"total_commands"`
	InputTokens   int64     `json:"input_tokens"`
	OutputTokens  int64     `json:"output_tokens"`
	SavedTokens   int64     `json:"saved_tokens"`
	SavingsPct    float64   `json:"savings_pct"`
	IsEstimated   bool      `json:"is_estimated"`
	// TokenState is "unavailable" until a harness reports provider usage.
	// Zero token fields are not a measurement.
	TokenState string `json:"token_state,omitempty"`
}

// Config represents the persisted context optimization settings in Downshift.
type Config struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"` // e.g. "native"
	// Companion is the id of an external companion the user enabled next to
	// the provider (for example "headroom"). It is bookkeeping: the hooks do
	// not change behaviour, and the native observe record stays on. Empty
	// means none.
	Companion     string   `json:"companion,omitempty"`
	ActiveHarness []string `json:"active_harnesses,omitempty"`
	Scope         struct {
		MainAgent     bool `json:"main_agent"`
		SpawnedAgents bool `json:"spawned_agents"`
	} `json:"scope"`
	Safety struct {
		PreserveOriginalOutput   bool `json:"preserve_original_output"`
		AllowFullContextRecovery bool `json:"allow_full_context_recovery"`
	} `json:"safety"`
	Observability struct {
		Enabled bool `json:"enabled"`
	} `json:"observability"`
	// ExitCodeContract is the persisted measurement rule. The native hook
	// records CompressExit in observe mode and preserves a non-zero exit.
	// It cannot replace the tool result the model sees, and it does not
	// compress prompts.
	ExitCodeContract string `json:"exit_code_contract,omitempty"`
	// Exec is the one file switch for the compact wrapper. Absent or true means
	// on; an explicit false turns it off. There is no CLI command for it, and
	// DOWNSHIFT_EXEC=0 still overrides both.
	Exec *bool `json:"exec,omitempty"`
}

// ExecOn reports whether the compact wrapper is on: absent or true.
func (c Config) ExecOn() bool { return c.Exec == nil || *c.Exec }

// ExitCodeContractObserve is the rule Configure persists for the native provider.
const ExitCodeContractObserve = "observe_only;nonzero_exit_preserved;hook_cannot_replace_tool_output"

// Companion marks a provider that runs outside the Downshift process and is
// enabled next to the in-process provider rather than instead of it.
// Configure for a companion persists Config.Companion only.
type Companion interface {
	Provider
	IsCompanion() bool
}

// HasCapability reports whether p lists cap. Providers delegate their method
// to it so the lookup lives once.
func HasCapability(p Provider, cap Capability) bool {
	for _, c := range p.Capabilities() {
		if c == cap {
			return true
		}
	}
	return false
}

// IsCompanion reports whether p is an external companion.
func IsCompanion(p Provider) bool {
	c, ok := p.(Companion)
	return ok && c.IsCompanion()
}

// Provider defines the contract that any context optimization companion must implement.
type Provider interface {
	ID() string
	Name() string
	Capabilities() []Capability
	HasCapability(cap Capability) bool
	SupportedHarnesses() []HarnessSupport
	Detect(ctx context.Context) (installed bool, version string, path string, err error)
	Validate(ctx context.Context) error
	Configure(ctx context.Context, targetDir string, harnessID string) error
	Disable(ctx context.Context, targetDir string, harnessID string) error
	HealthCheck(ctx context.Context) error
	GetDiagnostics(ctx context.Context, targetDir string) (*Diagnostics, error)
	GetMetrics(ctx context.Context, projectDir string) (*Metrics, error)
}
