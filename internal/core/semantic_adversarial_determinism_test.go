// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

// TestSemAdvMonotonic: fusion never lowers the regex complexity, a confident
// regex result is never changed, and a raise always clears Confident.
func TestSemAdvMonotonic(t *testing.T) {
	semAdvEnv(t)
	prompts := semAdvAllPrompts()
	if n := len(semAdvTaskPrompts()); n < 80 {
		t.Fatalf("task corpus has %d prompts, want >= 80", n)
	}
	raised := 0
	for i, p := range prompts {
		t.Run(fmt.Sprintf("%03d_%s", i, semAdvSlug(p)), func(t *testing.T) {
			base := core.Classify(p)
			sem := core.ClassifyWithSemantic(p)
			if sem.Complexity < base.Complexity {
				t.Fatalf("semantic lowered complexity: base=%s sem=%s", base.Complexity, sem.Complexity)
			}
			if base.Confident && sem.Complexity != base.Complexity {
				t.Fatalf("semantic changed a confident result: base=%s sem=%s", base.Complexity, sem.Complexity)
			}
			if sem.Complexity != base.Complexity {
				raised++
				if sem.Confident {
					t.Fatalf("raised result must not be confident: base=%s sem=%s", base.Complexity, sem.Complexity)
				}
			}
			if base.RiskFloor && sem.Complexity < core.Medium {
				t.Fatalf("risk floor lost after fusion: sem=%s", sem.Complexity)
			}
			if base.NoSignal && sem.Complexity != base.Complexity {
				t.Fatalf("no-signal prompt changed: base=%s sem=%s", base.Complexity, sem.Complexity)
			}
		})
	}
	t.Logf("semantic raised %d of %d prompts", raised, len(prompts))
}

// TestSemAdvDeterministic: repeated and concurrent calls agree with a
// serial reference.
func TestSemAdvDeterministic(t *testing.T) {
	semAdvEnv(t)
	prompts := semAdvAllPrompts()
	ref := make([]semAdvResult, len(prompts))
	for i, p := range prompts {
		ref[i] = semAdvEval(p)
	}

	loop := semAdvLoopPrompts(prompts)

	t.Run("repeat", func(t *testing.T) {
		for _, i := range loop {
			for n := 0; n < semAdvRepeats(); n++ {
				if got := semAdvEval(prompts[i]); got != ref[i] {
					t.Fatalf("run %d of %q: got %+v want %+v", n, semAdvShort(prompts[i]), got, ref[i])
				}
			}
		}
	})

	t.Run("goroutines", func(t *testing.T) {
		workers := semAdvWorkers()
		var wg sync.WaitGroup
		errs := make(chan string, workers*len(loop))
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				// Each worker walks the corpus from a different offset so
				// different prompts run at the same time.
				for k := range loop {
					i := loop[(k+w*7)%len(loop)]
					if got := semAdvEval(prompts[i]); got != ref[i] {
						errs <- fmt.Sprintf("worker %d %q: got %+v want %+v", w, semAdvShort(prompts[i]), got, ref[i])
					}
				}
			}(w)
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			t.Error(e)
		}
	})
}
