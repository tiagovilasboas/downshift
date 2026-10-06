// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package orchestration_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/orchestration"
)

func validReq() orchestration.Request {
	return orchestration.Request{
		Source:        "hook",
		Task:          "route subagents for daily report",
		DelegateTasks: []string{"fetch jira", "fetch ado", "compile"},
		MaxDelegates:  8,
	}
}

func TestOrchestrate_AcceptsValidRequest(t *testing.T) {
	p := orchestration.Orchestrate(validReq())
	if p.Status != "planned" {
		t.Fatalf("expected planned, got %s (%s)", p.Status, p.ErrorCode)
	}
	if len(p.Delegations) != 3 {
		t.Fatalf("expected 3 delegations, got %d", len(p.Delegations))
	}
}

func TestOrchestrate_DeduplicatesTasks(t *testing.T) {
	req := validReq()
	req.DelegateTasks = []string{"fetch jira", "fetch jira", "compile", "fetch jira"}
	p := orchestration.Orchestrate(req)
	if p.Status != "planned" {
		t.Fatalf("expected planned, got %s", p.Status)
	}
	if len(p.Delegations) != 2 {
		t.Fatalf("expected 2 after dedup, got %d", len(p.Delegations))
	}
}

func TestOrchestrate_RespectsMaxDelegates(t *testing.T) {
	req := validReq()
	req.DelegateTasks = []string{"a", "b", "c", "d", "e"}
	req.MaxDelegates = 3
	p := orchestration.Orchestrate(req)
	if len(p.Delegations) != 3 {
		t.Fatalf("expected 3, got %d", len(p.Delegations))
	}
}

func TestOrchestrate_RespectsCapabilityLimit(t *testing.T) {
	req := validReq()
	req.DelegateTasks = []string{"a", "b", "c", "d"}
	req.MaxDelegates = 8
	req.Capabilities = orchestration.Caps{MaxParallelDelegates: 2}
	p := orchestration.Orchestrate(req)
	if len(p.Delegations) != 2 {
		t.Fatalf("expected cap=2, got %d", len(p.Delegations))
	}
}

func TestOrchestrate_OrdinalsStartAtOne(t *testing.T) {
	p := orchestration.Orchestrate(validReq())
	for i, d := range p.Delegations {
		if d.Ordinal != i+1 {
			t.Errorf("ordinal[%d] = %d, want %d", i, d.Ordinal, i+1)
		}
	}
}

func TestOrchestrate_RejectsInvalidSource(t *testing.T) {
	req := validReq()
	req.Source = "unknown"
	p := orchestration.Orchestrate(req)
	if p.Status != "rejected" || p.ErrorCode != "INVALID_SOURCE" {
		t.Fatalf("expected INVALID_SOURCE, got %s %s", p.Status, p.ErrorCode)
	}
}

func TestOrchestrate_RejectsEmptyTask(t *testing.T) {
	req := validReq()
	req.Task = "   "
	p := orchestration.Orchestrate(req)
	if p.ErrorCode != "INVALID_TASK" {
		t.Fatalf("expected INVALID_TASK, got %s", p.ErrorCode)
	}
}

func TestOrchestrate_RejectsEmptyDelegateTask(t *testing.T) {
	req := validReq()
	req.DelegateTasks = []string{"ok", "  "}
	p := orchestration.Orchestrate(req)
	if p.ErrorCode != "INVALID_DELEGATES" {
		t.Fatalf("expected INVALID_DELEGATES, got %s", p.ErrorCode)
	}
}

func TestOrchestrate_RejectsInvalidMaxDelegates(t *testing.T) {
	req := validReq()
	req.MaxDelegates = 99
	p := orchestration.Orchestrate(req)
	if p.ErrorCode != "INVALID_MAX_DELEGATES" {
		t.Fatalf("expected INVALID_MAX_DELEGATES, got %s", p.ErrorCode)
	}
}

func TestOrchestrate_RejectsInvalidCorrelationID(t *testing.T) {
	req := validReq()
	req.CorrelationID = "not-hex!!"
	p := orchestration.Orchestrate(req)
	if p.ErrorCode != "INVALID_CORRELATION_ID" {
		t.Fatalf("expected INVALID_CORRELATION_ID, got %s", p.ErrorCode)
	}
}

func TestOrchestrate_AcceptsValidCorrelationID(t *testing.T) {
	req := validReq()
	req.CorrelationID = "deadbeef12345678"
	p := orchestration.Orchestrate(req)
	if p.Status != "planned" {
		t.Fatalf("expected planned, got %s", p.Status)
	}
}

func TestOrchestrate_EmptyDelegatesReturnsPlanWithNone(t *testing.T) {
	req := validReq()
	req.DelegateTasks = []string{}
	p := orchestration.Orchestrate(req)
	if p.Status != "planned" {
		t.Fatalf("expected planned with empty list, got %s", p.Status)
	}
	if len(p.Delegations) != 0 {
		t.Fatalf("expected 0, got %d", len(p.Delegations))
	}
}

func TestOrchestrate_TraceIsAlwaysPresent(t *testing.T) {
	p := orchestration.Orchestrate(validReq())
	if len(p.Trace) == 0 {
		t.Fatal("trace should never be empty")
	}
}

func TestOrchestrate_SchemaVersion(t *testing.T) {
	p := orchestration.Orchestrate(validReq())
	if p.SchemaVersion != orchestration.SchemaVersion {
		t.Fatalf("schema mismatch: %s", p.SchemaVersion)
	}
}
