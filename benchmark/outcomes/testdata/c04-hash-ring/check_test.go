package task

import (
	"fmt"
	"testing"
)

func TestRing(t *testing.T) {
	r := NewRing(100)
	if r.Get("x") != "" {
		t.Fatal("empty ring must return empty string")
	}
	nodes := []string{"node-a", "node-b", "node-c", "node-d"}
	for _, n := range nodes {
		r.Add(n)
	}
	before := map[string]string{}
	count := map[string]int{}
	for i := 0; i < 4000; i++ {
		k := fmt.Sprintf("key-%d", i)
		before[k] = r.Get(k)
		if r.Get(k) != before[k] {
			t.Fatal("non-deterministic")
		}
		count[before[k]]++
	}
	for _, n := range nodes {
		if count[n] < 500 || count[n] > 1600 {
			t.Fatalf("poor balance: %v", count)
		}
	}
	r.Remove("node-b")
	for k, old := range before {
		now := r.Get(k)
		if now == "node-b" {
			t.Fatal("removed node still owns keys")
		}
		if old != "node-b" && now != old {
			t.Fatalf("key %s moved from %s to %s although its node stayed", k, old, now)
		}
	}
	r.Add("node-b")
	for k, old := range before {
		if r.Get(k) != old {
			t.Fatalf("re-adding node-b did not restore mapping for %s", k)
		}
	}
}
