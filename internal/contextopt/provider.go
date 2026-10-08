// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package contextopt provides an opt-in, extensible framework for external
// context optimization companions (e.g. RTK, Headroom, Tare).
// Downshift remains strictly responsible for model routing and agent orchestration.
// Optimization providers handle their own command-output or context compression.
package contextopt

import (
	"context"
	"time"
)

// Capability defines a specific capability supported by a context optimization provider.
type Capability string

const (
	CapCommandOutputOptimization Capability = "command_output_optimization"
	CapToolResponseOptimization   Capability = "tool_response_optimization"
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
	MethodHook            IntegrationMethod = "hook"
	MethodAgentInstruction IntegrationMethod = "agent_instruction"
	MethodEnvWrapper      IntegrationMethod = "env_wrapper"
	MethodConfig          IntegrationMethod = "config"
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
	ProviderID      string           `json:"provider_id"`
	Installed       bool             `json:"installed"`
	BinaryPath      string           `json:"binary_path,omitempty"`
	Version         string           `json:"version,omitempty"`
	SupportedHarnesses []HarnessSupport `json:"supported_harnesses"`
	ActiveHarnesses []string         `json:"active_harnesses,omitempty"`
	Issues          []string         `json:"issues,omitempty"`
	Recommendations []string         `json:"recommendations,omitempty"`
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
}

// Config represents the persisted context optimization settings in Downshift.
type Config struct {
	Enabled       bool     `json:"enabled"`
	Provider      string   `json:"provider"` // e.g. "rtk"
	ActiveHarness []string `json:"active_harnesses,omitempty"`
	Scope         struct {
		MainAgent     bool `json:"main_agent"`
		SpawnedAgents bool `json:"spawned_agents"`
	} `json:"scope"`
	Safety struct {
		PreserveOriginalOutput  bool `json:"preserve_original_output"`
		AllowFullContextRecovery bool `json:"allow_full_context_recovery"`
	} `json:"safety"`
	Observability struct {
		Enabled bool `json:"enabled"`
	} `json:"observability"`
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
