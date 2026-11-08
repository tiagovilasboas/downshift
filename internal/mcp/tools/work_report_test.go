// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/workers"
)

func TestReleaseRefusesAnUnknownMark(t *testing.T) {
	d := budgetDeps(t, nil)
	_, err := d.release(context.Background(), json.RawMessage(`{"mark":"kept"}`))
	if err == nil || !strings.Contains(err.Error(), "mark must be used or discarded") {
		t.Fatalf("err = %v", err)
	}
}

func TestStatusListsReportsAndTheMarkSurvivesRelease(t *testing.T) {
	d := budgetDeps(t, &workers.Tokens{Total: 300})
	task := "secret task text that must never be listed"
	raw, _ := json.Marshal(map[string]any{"tasks": []string{task}, "agent": "claude", "mode": "read"})
	if _, err := d.delegate(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	settled(t, d, "claude", 300)
	if _, err := d.release(context.Background(), json.RawMessage(`{"job_ids":["w001"],"mark":"discarded"}`)); err != nil {
		t.Fatal(err)
	}
	block := statusWorkers(t, d)
	if len(block.Shown) != 0 {
		t.Fatalf("the released worker is still shown: %+v", block.Shown)
	}
	if len(block.Reports) != 1 {
		t.Fatalf("reports = %+v", block.Reports)
	}
	r := block.Reports[0]
	if r.Job != "w001" || r.Status != "discarded" || r.Tokens != 300 || r.ResultBytes != len("done") || r.TranscriptOnDisk || r.Wave != 1 {
		t.Fatalf("report = %+v", r)
	}
	env, _ := json.Marshal(block)
	if strings.Contains(string(env), task) {
		t.Fatalf("status leaks the task text: %s", env)
	}
}
