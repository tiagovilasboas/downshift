package task

import (
	"sync"
	"testing"
	"time"
)

func TestLedger(t *testing.T) {
	l := NewLedger(map[string]int64{"alice": 1000, "bob": 1000, "carol": 0})
	for _, bad := range []struct {
		from, to string
		amt      int64
	}{{"alice", "bob", 0}, {"alice", "bob", -5}, {"alice", "alice", 1}, {"alice", "zed", 1}, {"carol", "bob", 1}} {
		if err := l.Transfer(bad.from, bad.to, bad.amt); err == nil {
			t.Fatalf("expected error for %+v", bad)
		}
	}
	if l.Balance("alice") != 1000 || l.Balance("carol") != 0 {
		t.Fatal("failed transfers must not change balances")
	}
	var wg sync.WaitGroup
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			names := []string{"alice", "bob", "carol"}
			for j := 0; j < 2000; j++ {
				from, to := names[(i+j)%3], names[(i+j+1+i%2)%3]
				if from != to {
					_ = l.Transfer(from, to, int64(1+j%7))
				}
			}
		}(i)
	}
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock: transfers did not finish")
	}
	total := l.Balance("alice") + l.Balance("bob") + l.Balance("carol")
	if total != 2000 {
		t.Fatalf("money created or destroyed: total %d", total)
	}
	for _, n := range []string{"alice", "bob", "carol"} {
		if l.Balance(n) < 0 {
			t.Fatalf("%s overdrawn", n)
		}
	}
}
