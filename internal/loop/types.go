// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"time"
)

// Step identifies the current phase in the autonomous engineering loop.
type Step string

const (
	StepPlan      Step = "plan"
	StepImplement Step = "implement"
	StepSensor    Step = "sensor"
	StepCascade   Step = "cascade"
	StepAccept    Step = "accept"
	StepComplete  Step = "complete"
	StepFailed    Step = "failed"
)

// Config defines the parameters for one autonomous engineering loop run.
type Config struct {
	Task          string        // High-level engineering objective or prompt
	SensorCmd     string        // Verification command (e.g. "go test ./...")
	SessionID     string        // Session correlation identifier
	Harness       string        // Harness context (e.g. "claude-code")
	RepoDir       string        // Target repository root
	Model         string        // Specific model running the task (if known)
	MaxIterations int           // Max retry attempts before tripwire stop (default 3)
	Timeout       time.Duration // Max wall-clock time for the entire loop
}

// Result captures the outcome, telemetry and accounting of a loop execution.
type Result struct {
	Success       bool          `json:"success"`
	Iterations    int           `json:"iterations"`
	FinalStep     Step          `json:"final_step"`
	SensorPassed  bool          `json:"sensor_passed"`
	CascadeActive bool          `json:"cascade_active"`
	Accepted      bool          `json:"accepted"`
	Duration      time.Duration `json:"duration"`
	Message       string        `json:"message"`
}
