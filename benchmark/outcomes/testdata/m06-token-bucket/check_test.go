package task

import (
	"testing"
	"time"
)

func TestBucket(t *testing.T) {
	clock := time.Unix(1000, 0)
	b := NewBucket(3, 2, func() time.Time { return clock })
	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("request %d should pass on a full bucket", i)
		}
	}
	if b.Allow() {
		t.Fatal("bucket should be empty")
	}
	clock = clock.Add(250 * time.Millisecond) // +0.5 token
	if b.Allow() {
		t.Fatal("half a token is not enough")
	}
	clock = clock.Add(250 * time.Millisecond) // +0.5 token -> 1
	if !b.Allow() {
		t.Fatal("one token should be available")
	}
	clock = clock.Add(time.Hour) // refill capped at 3
	n := 0
	for b.Allow() {
		n++
		if n > 10 {
			break
		}
	}
	if n != 3 {
		t.Fatalf("capacity cap broken: allowed %d", n)
	}
}
