package task

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRetry(t *testing.T) {
	var slept []time.Duration
	sleep := func(d time.Duration) { slept = append(slept, d) }
	boom := errors.New("boom")
	calls := 0
	err := Retry(context.Background(), 4, 10*time.Millisecond, sleep, func() error { calls++; return boom })
	if !errors.Is(err, boom) || calls != 4 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	if !reflect.DeepEqual(slept, []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond}) {
		t.Fatalf("backoff %v", slept)
	}
	calls, slept = 0, nil
	err = Retry(context.Background(), 5, time.Second, sleep, func() error {
		calls++
		if calls < 3 {
			return boom
		}
		return nil
	})
	if err != nil || calls != 3 || len(slept) != 2 {
		t.Fatalf("success path: err=%v calls=%d slept=%v", err, calls, slept)
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls = 0
	err = Retry(ctx, 5, time.Second, func(time.Duration) { cancel() }, func() error { calls++; return boom })
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("cancel path: err=%v calls=%d", err, calls)
	}
}
