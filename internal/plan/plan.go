// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package plan validates a delegation plan the lead LLM wrote, fills the
// defaults (tier, playbook, wave, route) and renders it. It never calls an
// LLM: the lead decides the parts, Downshift checks and tabulates them.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift-ai
package plan

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Worker values a part may name.
const (
	WorkerNative = "native"
	WorkerClaude = "claude"
	WorkerCodex  = "codex"
	RouteDirect  = "direct"
)

// Part is one slice of work as the lead wrote it.
type Part struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Agent     string   `json:"agent"`
	Points    int      `json:"points"`
	DependsOn []string `json:"depends_on,omitempty"`
	Files     []string `json:"files,omitempty"`
	Worker    string   `json:"worker,omitempty"`
	Playbook  string   `json:"playbook,omitempty"`
}

// Input is the plan JSON read from stdin.
type Input struct {
	Goal  string `json:"goal"`
	Parts []Part `json:"parts"`
}

// Gateway is the state of the model gateway the route depends on.
type Gateway struct {
	Enabled bool
	Name    string
}

// FilledPart is a Part with the defaults Downshift computed.
type FilledPart struct {
	Part
	Tier  string `json:"tier"`
	Wave  int    `json:"wave"`
	Route string `json:"route"`
	// Mode is the delegate mode (read or write) a worker part runs in.
	Mode string `json:"mode,omitempty"`
}

// Plan is the validated, filled plan.
type Plan struct {
	Goal     string       `json:"goal"`
	Parts    []FilledPart `json:"parts"`
	Points   int          `json:"total_points"`
	Waves    int          `json:"waves"`
	Warnings []string     `json:"warnings,omitempty"`
	Next     []Step       `json:"next"`
}

// ParseInput decodes the plan JSON.
func ParseInput(data []byte) (Input, error) {
	var in Input
	if err := json.Unmarshal(data, &in); err != nil {
		return in, fmt.Errorf("invalid JSON: %w", err)
	}
	return in, nil
}

// Resolve decodes the plan JSON and builds it: the one entry point shared by
// `downshift plan` and the MCP plan tool. It returns every problem found, a
// malformed document included.
func Resolve(data []byte, gw Gateway, w Workers, playbooks []string) (Plan, []string) {
	in, err := ParseInput(data)
	if err != nil {
		return Plan{}, []string{err.Error()}
	}
	return Build(in, gw, w, playbooks)
}

// Build validates in and returns the filled plan. playbooks lists the playbook
// ids a part may name. On invalid input it returns every error found.
func Build(in Input, gw Gateway, w Workers, playbooks []string) (Plan, []string) {
	errs := validate(in, playbooks)
	if len(errs) > 0 {
		return Plan{}, errs
	}
	waves := waveOf(in.Parts)
	p := Plan{Goal: strings.TrimSpace(in.Goal)}
	for _, part := range in.Parts {
		fp, warn := fill(part, gw, w)
		if warn != "" {
			p.Warnings = append(p.Warnings, warn)
		}
		fp.Wave = waves[part.ID]
		if fp.Wave > p.Waves {
			p.Waves = fp.Wave
		}
		p.Points += part.Points
		p.Parts = append(p.Parts, fp)
	}
	p.Warnings = append(p.Warnings, sharedFiles(p.Parts)...)
	p.Next = nextSteps(p.Parts)
	return p, nil
}
