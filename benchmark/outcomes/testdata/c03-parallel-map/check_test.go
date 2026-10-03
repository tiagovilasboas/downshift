package task

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallelMap(t *testing.T) {
	var inFlight, peak int32
	fn := func(ctx context.Context, x int) (int, error) {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		return x * x, nil
	}
	in := make([]int, 40)
	for i := range in {
		in[i] = i
	}
	got, err := ParallelMap(context.Background(), in, 4, fn)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range got {
		if v != i*i {
			t.Fatalf("result %d = %d", i, v)
		}
	}
	if peak > 4 || peak < 2 {
		t.Fatalf("peak concurrency %d, want 2..4", peak)
	}

	boom := errors.New("boom")
	var started int32
	_, err = ParallelMap(context.Background(), make([]int, 200), 2, func(ctx context.Context, x int) (int, error) {
		if atomic.AddInt32(&started, 1) == 3 {
			return 0, boom
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(5 * time.Millisecond):
			return 1, nil
		}
	})
	if !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
	if s := atomic.LoadInt32(&started); s > 20 {
		t.Fatalf("work kept starting after the error: %d calls", s)
	}
}
