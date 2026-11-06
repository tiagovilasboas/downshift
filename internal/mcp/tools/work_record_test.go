// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/workers"
)

// With workers off, delegate refuses before anything is queued, says how to
// turn workers on, and records the refusal code with no task text.
func TestDelegateRefusesWhenWorkersOffAndRecords(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state WorkerState
		agent string
		want  string
	}{
		{"off", WorkerState{Mode: "off"}, "claude", "worker tools are off"},
		{"agent not enabled", WorkerState{Mode: "read", Agents: []string{"claude"}}, "codex", "agent codex is not enabled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotTool, gotCode string
			calls := 0
			d := Deps{Work: workers.New(newViewLauncher(), 4, time.Now), Workers: tc.state, Root: t.TempDir(),
				OnCall: func(tool, code string, _ CallCounts) { gotTool, gotCode = tool, code; calls++ }}
			raw, _ := json.Marshal(map[string]any{"tasks": []string{"secret task text"}, "agent": tc.agent, "mode": "read"})
			_, err := d.delegate(context.Background(), raw)
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "downshift setup --workers read") {
				t.Fatalf("err = %v, want %q with a fix line", err, tc.want)
			}
			if calls != 1 || gotTool != "delegate" || gotCode != RefusalWorkersOff {
				t.Errorf("recorded %d calls (%q, %q), want 1 (delegate, %s)", calls, gotTool, gotCode, RefusalWorkersOff)
			}
			if n := len(d.Work.Workers()); n != 0 {
				t.Errorf("%d workers queued, want none", n)
			}
		})
	}
}

// The status tool reports the resolved worker mode when the server has one.
func TestStatusReportsWorkerMode(t *testing.T) {
	d := fullDeps()
	d.Workers = WorkerState{Mode: "read", Agents: []string{"claude"}, Source: "default"}
	env, _ := call(t, d, "status", `{}`)
	var s statusData
	_ = json.Unmarshal(env["data"], &s)
	if s.WorkerMode == nil || s.WorkerMode.Mode != "read" || s.WorkerMode.Source != "default" {
		t.Fatalf("worker_mode = %+v", s.WorkerMode)
	}
}
