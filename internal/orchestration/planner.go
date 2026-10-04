// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

// Package orchestration provides deterministic fan-out planning for delegated
// subtasks. It is the Go equivalent of orchestration/src/.../graph.py — same
// contract, zero Python, zero latency overhead.
package orchestration

import (
	"regexp"
	"strings"
)

const (
	SchemaVersion = "harness-downshift.orchestration.v1"
	MaxDelegates  = 8
)

var (
	allowedSources      = map[string]bool{"cli": true, "hook": true, "service": true}
	correlationIDRegexp = regexp.MustCompile(`^[a-f0-9]{16,64}$`)
)

// Request is the external JSON contract for a planning call.
// Provider, model, and credential data are explicitly excluded.
type Request struct {
	Source        string   `json:"source"`
	Task          string   `json:"task"`
	DelegateTasks []string `json:"delegate_tasks"`
	MaxDelegates  int      `json:"max_delegates"`
	Capabilities  Caps     `json:"capabilities"`
	CorrelationID string   `json:"correlation_id,omitempty"`
}

// Caps holds caller-owned execution limits.
type Caps struct {
	MaxParallelDelegates int `json:"max_parallel_delegates,omitempty"`
}

// Delegation is a planned subtask; execution is the caller's responsibility.
type Delegation struct {
	Ordinal int    `json:"ordinal"`
	Task    string `json:"task"`
}

// Plan is the output of a planning call.
type Plan struct {
	SchemaVersion string       `json:"schema_version"`
	Status        string       `json:"status"`
	ErrorCode     string       `json:"error_code,omitempty"`
	Delegations   []Delegation `json:"delegations,omitempty"`
	Trace         []string     `json:"trace"`
}

func reject(code string) Plan {
	return Plan{
		SchemaVersion: SchemaVersion,
		Status:        "rejected",
		ErrorCode:     code,
		Trace:         []string{"validate:rejected"},
	}
}

// Orchestrate validates and plans delegation without calling any LLM or network.
func Orchestrate(req Request) Plan {
	// --- validate ---
	if !allowedSources[req.Source] {
		return reject("INVALID_SOURCE")
	}
	if req.CorrelationID != "" && !correlationIDRegexp.MatchString(req.CorrelationID) {
		return reject("INVALID_CORRELATION_ID")
	}
	if strings.TrimSpace(req.Task) == "" {
		return reject("INVALID_TASK")
	}
	for _, t := range req.DelegateTasks {
		if strings.TrimSpace(t) == "" {
			return reject("INVALID_DELEGATES")
		}
	}
	limit := req.MaxDelegates
	if limit < 0 || limit > MaxDelegates {
		return reject("INVALID_MAX_DELEGATES")
	}
	if limit == 0 {
		limit = MaxDelegates
	}
	capLimit := req.Capabilities.MaxParallelDelegates
	if capLimit < 0 || capLimit > MaxDelegates {
		return reject("INVALID_CAPABILITY_LIMIT")
	}
	if capLimit > 0 && capLimit < limit {
		limit = capLimit
	}

	// --- plan: dedup + order + apply limit ---
	seen := make(map[string]bool, len(req.DelegateTasks))
	unique := make([]string, 0, len(req.DelegateTasks))
	for _, t := range req.DelegateTasks {
		n := strings.TrimSpace(t)
		if !seen[n] {
			seen[n] = true
			unique = append(unique, n)
		}
	}
	if limit > len(unique) {
		limit = len(unique)
	}
	selected := unique[:limit]
	delegations := make([]Delegation, len(selected))
	for i, t := range selected {
		delegations[i] = Delegation{Ordinal: i + 1, Task: t}
	}

	return Plan{
		SchemaVersion: SchemaVersion,
		Status:        "planned",
		Delegations:   delegations,
		Trace:         []string{"validate:accepted", "plan:complete"},
	}
}
