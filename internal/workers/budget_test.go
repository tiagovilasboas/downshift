// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"errors"
	"testing"
	"time"
)

// finishWith ends a fake worker that reported total tokens.
func finishWith(p *fakeProc, total int64) {
	p.tokens = &Tokens{Input: total - 10, Output: 10, Total: total}
	p.finish("ok", nil)
}

func TestDeliveryCarriesTokensAndObserverSeesTheEnd(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 2, fixedNow)
	ends := make(chan End, 1)
	q.OnEnd(func(e End) { ends <- e })
	if _, err := q.Delegate([]Spec{spec("a", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	finishWith(l.proc("a"), 500)
	c := q.Collect(context.Background(), 0, true, 5*time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].Tokens == nil || c.Deliveries[0].Tokens.Total != 500 {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
	select {
	case e := <-ends:
		if e.Agent != "claude" || e.Outcome != Done || e.Tokens == nil || e.Tokens.Total != 500 {
			t.Fatalf("end = %+v", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("observer never heard the end")
	}
	if got := q.TokensUsed("claude"); got != 500 {
		t.Fatalf("used = %d", got)
	}
}

func TestDelegateRefusesAnAgentPastItsTokenBudget(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 2, fixedNow)
	q.SetTokenBudget(1000)
	if _, err := q.Delegate([]Spec{spec("a", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	finishWith(l.proc("a"), 1200)
	waitFor(t, "the end", func() bool { return q.TokensUsed("claude") == 1200 })

	_, err := q.Delegate([]Spec{spec("b", time.Minute)})
	var over ErrTokenBudget
	if !errors.As(err, &over) || over.Used != 1200 || over.Budget != 1000 || over.Agent != "claude" {
		t.Fatalf("err = %v, want a token budget refusal", err)
	}
	if len(l.started()) != 1 {
		t.Fatalf("a refused job started: %v", l.started())
	}
	codex := Spec{Agent: "codex", Dir: ".", Task: "c", Budget: time.Minute}
	if _, err := q.Delegate([]Spec{codex}); err != nil {
		t.Fatalf("the budget is per CLI; codex was refused: %v", err)
	}
}

func TestNoBudgetMeansNoRefusal(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{spec("a", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	finishWith(l.proc("a"), 5_000_000)
	waitFor(t, "the end", func() bool { return q.TokensUsed("claude") > 0 })
	if _, err := q.Delegate([]Spec{spec("b", time.Minute)}); err != nil {
		t.Fatalf("no budget set, yet refused: %v", err)
	}
}

func TestACancelledWorkerAddsNoTokens(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 1, fixedNow)
	ids, _ := q.Delegate([]Spec{spec("a", time.Minute)})
	q.Cancel(ids)
	c := q.Collect(context.Background(), 0, false, time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].Tokens != nil || q.TokensUsed("claude") != 0 {
		t.Fatalf("deliveries = %+v used = %d", c.Deliveries, q.TokensUsed("claude"))
	}
}
