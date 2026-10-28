// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestParseBatch(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantN   int
		wantErr string
	}{
		{name: "valid", in: `[{"task":"a","sensor":"true","model":"m"},{"task":"b"}]`, wantN: 2},
		{name: "empty list", in: `[]`, wantErr: "no tasks"},
		{name: "not an array", in: `{"task":"a"}`, wantErr: "parse batch"},
		{name: "unknown field", in: `[{"task":"a","sensr":"true"}]`, wantErr: "unknown field"},
		{name: "blank task", in: `[{"task":"  "}]`, wantErr: "item 1 has no task"},
		{name: "trailing data", in: `[{"task":"a"}] x`, wantErr: "trailing"},
		{name: "too many", in: "[" + strings.Repeat(`{"task":"a"},`, maxBatchItems) + `{"task":"a"}]`, wantErr: "limit"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			items, err := ParseBatch(strings.NewReader(c.in))
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, c.wantErr)
				}
				return
			}
			if err != nil || len(items) != c.wantN {
				t.Fatalf("items = %d, err = %v, want %d", len(items), err, c.wantN)
			}
		})
	}
}

func TestItemConfigOverlay(t *testing.T) {
	base := Config{SensorCmd: "base-sensor", Model: "base-model", SessionID: "s"}
	got := itemConfig(base, Item{Task: "t", Sensor: "x"})
	if got.Task != "t" || got.SensorCmd != "x" || got.Model != "base-model" || got.SessionID != "s" {
		t.Errorf("itemConfig = %+v", got)
	}
}

// scripted returns a RunFunc that answers by task name.
func scripted(outcomes map[string]Result, errs map[string]error) (RunFunc, *[]string) {
	var ran []string
	return func(_ context.Context, c Config) (Result, error) {
		ran = append(ran, c.Task)
		return outcomes[c.Task], errs[c.Task]
	}, &ran
}

func TestRunBatch(t *testing.T) {
	pass := Result{Success: true}
	recovered := Result{Success: true, CascadeActive: true}
	fail := Result{FinalStep: StepFailed, Message: "tripwire"}
	items := []Item{{Task: "a"}, {Task: "b"}, {Task: "c"}}
	cases := []struct {
		name     string
		outcomes map[string]Result
		errs     map[string]error
		cont     bool
		ran      string
		passed   int
		failed   int
		skipped  int
		recov    int
		ok       bool
	}{
		{name: "all pass", outcomes: map[string]Result{"a": pass, "b": recovered, "c": pass},
			ran: "a,b,c", passed: 3, recov: 1, ok: true},
		{name: "fail fast skips the rest", outcomes: map[string]Result{"a": pass, "b": fail, "c": pass},
			ran: "a,b", passed: 1, failed: 1, skipped: 1},
		{name: "continue on error runs all", outcomes: map[string]Result{"a": fail, "b": pass, "c": pass}, cont: true,
			ran: "a,b,c", passed: 2, failed: 1},
		{name: "runner error counts as failure", errs: map[string]error{"a": errors.New("boom")},
			outcomes: map[string]Result{"b": pass, "c": pass},
			ran:      "a", failed: 1, skipped: 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run, ran := scripted(c.outcomes, c.errs)
			got := RunBatch(context.Background(), BatchConfig{Items: items, ContinueOnError: c.cont, Run: run})
			if strings.Join(*ran, ",") != c.ran {
				t.Errorf("ran = %v, want %s", *ran, c.ran)
			}
			if got.Passed != c.passed || got.Failed != c.failed || got.Skipped != c.skipped || got.Recovered != c.recov {
				t.Errorf("got %+v", got)
			}
			if got.OK() != c.ok || len(got.Items) != 3 || got.Total != 3 {
				t.Errorf("OK = %v, items = %d", got.OK(), len(got.Items))
			}
		})
	}
}

func TestRunBatchRunnerErrorKeepsMessage(t *testing.T) {
	run, _ := scripted(nil, map[string]error{"a": errors.New("boom")})
	got := RunBatch(context.Background(), BatchConfig{Items: []Item{{Task: "a"}}, Run: run})
	if got.Items[0].Message != "boom" {
		t.Errorf("message = %q", got.Items[0].Message)
	}
}

func TestRunBatchCancelledContextSkipsAll(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	run, ran := scripted(nil, nil)
	got := RunBatch(ctx, BatchConfig{Items: []Item{{Task: "a"}, {Task: "b"}}, Run: run})
	if len(*ran) != 0 || got.Skipped != 2 {
		t.Errorf("ran %v, skipped %d", *ran, got.Skipped)
	}
}
