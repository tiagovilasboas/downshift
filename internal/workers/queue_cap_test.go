// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"errors"
	"testing"
	"time"
)

func TestQueueRefusesJobsPastTheCap(t *testing.T) {
	q := New(newFakeLauncher(), 4, fixedNow)
	for i := 0; i < DefaultMaxJobs; i++ {
		if _, err := q.Delegate([]Spec{spec("t", time.Minute)}); err != nil {
			t.Fatalf("job %d: %v", i+1, err)
		}
	}
	if _, err := q.Delegate([]Spec{spec("t", time.Minute)}); !errors.Is(err, ErrCapReached) {
		t.Fatalf("33rd job: got %v, want ErrCapReached", err)
	}
}

func TestQueueCapRefusesAWholeBatchAndZeroMeansNoCap(t *testing.T) {
	q := New(newFakeLauncher(), 4, fixedNow)
	q.SetMaxJobs(2)
	batch := []Spec{spec("a", time.Minute), spec("b", time.Minute), spec("c", time.Minute)}
	if _, err := q.Delegate(batch); !errors.Is(err, ErrCapReached) {
		t.Fatalf("got %v, want ErrCapReached", err)
	}
	if len(q.Statuses()) != 0 {
		t.Fatal("a refused batch must start nothing")
	}
	q.SetMaxJobs(0)
	if _, err := q.Delegate(batch); err != nil {
		t.Fatalf("no cap: %v", err)
	}
}
