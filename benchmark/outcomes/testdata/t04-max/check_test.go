package task

import (
	"testing"
)

func TestMaxInt(t *testing.T) {
	if _, ok := MaxInt(nil); ok {
		t.Fatal("empty slice must return false")
	}
	if m, ok := MaxInt([]int{-7, -2, -9}); !ok || m != -2 {
		t.Fatalf("got %d %v", m, ok)
	}
	if m, _ := MaxInt([]int{3, 9, 1}); m != 9 {
		t.Fatalf("got %d", m)
	}
}
