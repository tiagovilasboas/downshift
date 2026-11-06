// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestOverBudget(t *testing.T) {
	cases := []struct {
		name         string
		spent, limit int64
		want         bool
	}{
		{"no limit", 9999, 0, false},
		{"negative limit", 9999, -5, false},
		{"under", 99, 100, false},
		{"at the limit", 100, 100, true},
		{"past", 101, 100, true},
		{"nothing spent", 0, 100, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := OverBudget(c.spent, c.limit); got != c.want {
				t.Fatalf("OverBudget(%d,%d) = %v", c.spent, c.limit, got)
			}
		})
	}
}

func TestWaveErrorsNameSpentAndBudget(t *testing.T) {
	w := ErrWaveBudget{Wave: 2, Spent: 700, Budget: 500}.Error()
	o := ErrOrchestrationBudget{Spent: 900, Cap: 800}.Error()
	for _, want := range []string{"wave 2", "700", "500"} {
		if !strings.Contains(w, want) {
			t.Fatalf("%q lacks %q", w, want)
		}
	}
	for _, want := range []string{"900", "800", "orchestration"} {
		if !strings.Contains(o, want) {
			t.Fatalf("%q lacks %q", o, want)
		}
	}
}

func TestOrchestrationCapRefusesTheNextWave(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 4, fixedNow)
	lim := WaveLimits{Orchestration: 1000}
	if _, n, err := q.DelegateWave([]Spec{spec("a", time.Minute)}, lim); err != nil || n != 1 {
		t.Fatalf("first wave = %d, %v", n, err)
	}
	finishWith(l.proc("a"), 600)
	waitFor(t, "the first end", func() bool { return q.TokensUsed("claude") == 600 })
	if _, _, err := q.DelegateWave([]Spec{spec("b", time.Minute)}, lim); err != nil {
		t.Fatalf("under the cap, yet refused: %v", err)
	}
	finishWith(l.proc("b"), 400)
	waitFor(t, "the second end", func() bool { return q.TokensUsed("claude") == 1000 })

	_, _, err := q.DelegateWave([]Spec{spec("c", time.Minute)}, lim)
	var over ErrOrchestrationBudget
	if !errors.As(err, &over) || over.Spent != 1000 || over.Cap != 1000 {
		t.Fatalf("err = %v, want an orchestration refusal", err)
	}
	if len(l.started()) != 2 {
		t.Fatalf("a refused wave started: %v", l.started())
	}
	if _, _, err := q.DelegateWave([]Spec{spec("d", time.Minute)}, WaveLimits{}); err != nil {
		t.Fatalf("no cap in this call, yet refused: %v", err)
	}
}

func TestUnknownTokensNeverBlockAWave(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 2, fixedNow)
	lim := WaveLimits{Wave: 1, Orchestration: 1}
	if _, _, err := q.DelegateWave([]Spec{spec("a", time.Minute)}, lim); err != nil {
		t.Fatal(err)
	}
	l.proc("a").finish("ok", nil) // the CLI wrote no usage
	waitFor(t, "the end", func() bool { return q.Collect(t.Context(), 0, false, 0).Active == 0 })
	if w := q.Waves(); len(w) != 1 || w[0].Over || w[0].Spent != 0 {
		t.Fatalf("waves = %+v", w)
	}
	if _, _, err := q.DelegateWave([]Spec{spec("b", time.Minute)}, lim); err != nil {
		t.Fatalf("unknown spend blocked a wave: %v", err)
	}
}

func TestWaveOverBudgetRefusesSendToItsWorkersOnly(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 4, fixedNow)
	if _, n, err := q.DelegateWave([]Spec{spec("a", time.Minute)}, WaveLimits{Wave: 500}); err != nil || n != 1 {
		t.Fatalf("wave = %d, %v", n, err)
	}
	if _, n, err := q.DelegateWave([]Spec{spec("b", time.Minute)}, WaveLimits{}); err != nil || n != 2 {
		t.Fatalf("wave = %d, %v", n, err)
	}
	finishWith(l.proc("a"), 400)
	finishWith(l.proc("b"), 9000)
	waitFor(t, "both ends", func() bool { return q.TokensUsed("claude") == 9400 })

	if w := q.Waves(); len(w) != 1 || w[0].Over || w[0].Spent != 400 {
		t.Fatalf("waves = %+v, want wave 1 under budget", w)
	}
	if err := q.Continue("w001", "more"); err != nil {
		t.Fatalf("send under the wave budget: %v", err)
	}
	finishWith(l.proc("more"), 200) // resumed turn pushes wave 1 to 600
	waitFor(t, "the resumed end", func() bool { return q.TokensUsed("claude") == 9600 })

	w := q.Waves()
	if len(w) != 1 || !w[0].Over || w[0].Spent != 600 || w[0].Budget != 500 {
		t.Fatalf("waves = %+v, want wave 1 over", w)
	}
	var over ErrWaveBudget
	if err := q.Continue("w001", "again"); !errors.As(err, &over) || over.Wave != 1 || over.Spent != 600 || over.Budget != 500 {
		t.Fatalf("err = %v, want a wave budget refusal", err)
	}
	if err := q.Continue("w002", "other"); err != nil {
		t.Fatalf("a worker of a wave with no budget was refused: %v", err)
	}
}
