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

// tokProc is a worker that ends at once, reporting tokens (nil: no usage) and a
// session id, so it can be resumed.
type tokProc struct{ tokens *workers.Tokens }

func (tokProc) Wait() error               { return nil }
func (tokProc) Kill() error               { return nil }
func (tokProc) Report() string            { return "done" }
func (tokProc) Stderr() string            { return "" }
func (p tokProc) Tokens() *workers.Tokens { return p.tokens }
func (tokProc) SessionID() string         { return "sess-1" }

type tokLauncher tokProc

func (l tokLauncher) Launch(workers.Spec) (workers.Process, error) { return tokProc(l), nil }
func (l tokLauncher) LaunchResume(workers.Spec, string) (workers.Process, error) {
	return tokProc(l), nil
}

func budgetDeps(t *testing.T, tokens *workers.Tokens) Deps {
	t.Helper()
	d, _ := delegateDeps(t.TempDir(), false, nil)
	d.Work = workers.New(tokLauncher{tokens: tokens}, 4, time.Now)
	return d
}

func settled(t *testing.T, d Deps, agent string, want int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for d.Work.TokensUsed(agent) < want {
		if time.Now().After(deadline) {
			t.Fatalf("tokens never reached %d", want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

const oneTask = `{"tasks":["x"],"agent":"claude","mode":"read"%s}`

func TestDelegateRefusesANewWaveAtTheOrchestrationCap(t *testing.T) {
	d := budgetDeps(t, &workers.Tokens{Total: 1000})
	d.BudgetTokens = func() int64 { return 1000 }
	if _, err := d.delegate(context.Background(), json.RawMessage(strings.Replace(oneTask, "%s", "", 1))); err != nil {
		t.Fatalf("first wave: %v", err)
	}
	settled(t, d, "claude", 1000)
	_, err := d.delegate(context.Background(), json.RawMessage(strings.Replace(oneTask, "%s", "", 1)))
	if err == nil || !strings.Contains(err.Error(), RefusalBudget) || !strings.Contains(err.Error(), "1000") {
		t.Fatalf("err = %v, want code %q naming the spend", err, RefusalBudget)
	}
	d.BudgetTokens = nil
	if _, err := d.delegate(context.Background(), json.RawMessage(strings.Replace(oneTask, "%s", "", 1))); err != nil {
		t.Fatalf("no cap, yet refused: %v", err)
	}
}

func TestDelegateIgnoresAnUnknownSpend(t *testing.T) {
	d := budgetDeps(t, nil)
	d.BudgetTokens = func() int64 { return 1 }
	for i := 0; i < 3; i++ {
		if _, err := d.delegate(context.Background(), json.RawMessage(strings.Replace(oneTask, "%s", "", 1))); err != nil {
			t.Fatalf("call %d refused on an unknown spend: %v", i, err)
		}
	}
}

func TestSendRefusedOnceItsWaveIsOverBudget(t *testing.T) {
	d := budgetDeps(t, &workers.Tokens{Total: 1000})
	res, err := d.delegate(context.Background(), json.RawMessage(strings.Replace(oneTask, "%s", `,"budget_tokens":500`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Text, `"wave":1`) || !strings.Contains(res.Text, `"budget_tokens":500`) {
		t.Fatalf("delegate reply lacks the wave: %s", res.Text)
	}
	settled(t, d, "claude", 1000)
	_, err = d.send(context.Background(), json.RawMessage(`{"job_id":"w001","task":"more"}`))
	if err == nil || !strings.Contains(err.Error(), "wave 1") || !strings.Contains(err.Error(), "1000") || !strings.Contains(err.Error(), "500") {
		t.Fatalf("err = %v, want a wave budget refusal naming spent and budget", err)
	}
	col, err := d.collect(context.Background(), json.RawMessage(`{}`))
	if err != nil || !strings.Contains(col.Text, `"over_budget":true`) {
		t.Fatalf("collect must mark the wave over budget: %v %s", err, col.Text)
	}
}

func TestDelegateRefusesANegativeWaveBudget(t *testing.T) {
	d := budgetDeps(t, nil)
	if _, err := d.delegate(context.Background(), json.RawMessage(strings.Replace(oneTask, "%s", `,"budget_tokens":-1`, 1))); err == nil {
		t.Fatal("a negative budget_tokens was accepted")
	}
}
