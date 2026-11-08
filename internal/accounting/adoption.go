// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// Adoption is how much the MCP plan and delegate tools were used over a
// Scope, from mcp_tool and worker_ended events. Counts only: no task text, no
// tokens, no money.
type Adoption struct {
	Plans         int `json:"plans"`
	Steps         int `json:"steps"`
	SpawnSteps    int `json:"spawn_steps"`
	DelegateSteps int `json:"delegate_steps"`
	// Delegated is the tasks accepted by delegate calls; Done is the workers
	// that ended done.
	Delegated int `json:"delegated"`
	Done      int `json:"done"`
	// Refused counts refused calls by code, across plan and delegate.
	Refused       map[string]int `json:"refused,omitempty"`
	GatewayRouted int            `json:"gateway_routed"`
}

// BuildAdoption tallies the plan and delegate calls in the window of scope.
func BuildAdoption(events []telemetry.Event, scope Scope) Adoption {
	a := Adoption{}
	for _, ev := range scopeTelemetry(events, scope) {
		switch ev.Outcome {
		case telemetry.OutcomeWorkerEnded:
			if ev.WorkerOutcome == "done" {
				a.Done++
			}
		case telemetry.OutcomeMCPTool:
			a.add(ev)
		}
	}
	return a
}

func (a *Adoption) add(ev telemetry.Event) {
	var c telemetry.ToolCounts
	if ev.Tool != nil {
		c = *ev.Tool
	}
	if ev.ErrorCode != "" {
		if a.Refused == nil {
			a.Refused = map[string]int{}
		}
		a.Refused[ev.ErrorCode]++
		return
	}
	switch ev.Agent {
	case "plan":
		a.Plans++
		a.Steps += c.Steps
		a.SpawnSteps += c.SpawnSteps
		a.DelegateSteps += c.DelegateSteps
	case "delegate":
		a.Delegated += c.Tasks
		a.GatewayRouted += c.GatewayRouted
	}
}

func (a Adoption) refusedTotal() int {
	n := 0
	for _, v := range a.Refused {
		n += v
	}
	return n
}

// AdoptionLine is the printed planner and workers line. With no calls at all
// it says so, so adoption at zero stays visible.
func AdoptionLine(a Adoption) string {
	if a.Plans == 0 && a.Delegated == 0 && a.Done == 0 && a.refusedTotal() == 0 {
		return "Planner and workers: no calls yet"
	}
	line := fmt.Sprintf("Planner: %d plans, %d steps (%d spawn · %d delegate) · Workers: %d delegated, %d done",
		a.Plans, a.Steps, a.SpawnSteps, a.DelegateSteps, a.Delegated, a.Done)
	if n := a.refusedTotal(); n > 0 {
		codes := make([]string, 0, len(a.Refused))
		for c := range a.Refused {
			codes = append(codes, c)
		}
		sort.Strings(codes)
		parts := make([]string, 0, len(codes))
		for _, c := range codes {
			if len(codes) == 1 {
				parts = append(parts, c)
			} else {
				parts = append(parts, fmt.Sprintf("%s %d", c, a.Refused[c]))
			}
		}
		line += fmt.Sprintf(", %d refused (%s)", n, strings.Join(parts, ", "))
	}
	return line + fmt.Sprintf(" · gateway %d/%d", a.GatewayRouted, a.Delegated)
}
