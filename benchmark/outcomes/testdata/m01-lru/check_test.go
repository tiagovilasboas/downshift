package task

import (
	"testing"
)

func TestLRU(t *testing.T) {
	c := NewLRU(2)
	c.Put("a", 1)
	c.Put("b", 2)
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatal("a missing")
	}
	c.Put("c", 3) // evicts b (a was used more recently)
	if _, ok := c.Get("b"); ok {
		t.Fatal("b should be evicted")
	}
	c.Put("a", 10) // update, makes a most recent
	c.Put("d", 4)  // evicts c
	if _, ok := c.Get("c"); ok {
		t.Fatal("c should be evicted")
	}
	if v, ok := c.Get("a"); !ok || v != 10 {
		t.Fatalf("a=%d %v", v, ok)
	}
	if v, ok := c.Get("d"); !ok || v != 4 {
		t.Fatal("d missing")
	}
}
