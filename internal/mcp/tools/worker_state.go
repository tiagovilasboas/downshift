// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import "github.com/tiagovilasboas/downshift/internal/plan"

// WorkerState is the worker setting the server resolved at start: the mode,
// the agents it can run, where the setting came from and any ignored values.
// The status tool reports it and plan builds its next steps from it.
type WorkerState struct {
	Mode   string   `json:"mode"`
	Agents []string `json:"agents,omitempty"`
	Source string   `json:"source,omitempty"`
	Notes  []string `json:"notes,omitempty"`
}

// Plan is the part of the state the planner reads.
func (w WorkerState) Plan() plan.Workers {
	return plan.Workers{Mode: w.Mode, Agents: append([]string(nil), w.Agents...)}
}

// CallCounts are the numbers a plan or delegate call records for adoption.
// They are counts only: no task text, directory or file name.
type CallCounts struct {
	Steps, DelegateSteps, SpawnSteps int
	Tasks, Write, GatewayRouted      int
}

// recordCall reports one plan or delegate call; code is a refusal code or "".
func (d Deps) recordCall(tool, code string, c CallCounts) {
	if d.OnCall != nil {
		d.OnCall(tool, code, c)
	}
}
