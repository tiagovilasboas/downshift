package task

import (
	"testing"
)

func TestBinarySearch(t *testing.T) {
	xs := []int{-4, 0, 3, 8, 15, 23}
	for i, x := range xs {
		if got := BinarySearch(xs, x); got != i {
			t.Errorf("BinarySearch(%d)=%d want %d", x, got, i)
		}
	}
	for _, x := range []int{-5, 1, 24} {
		if BinarySearch(xs, x) != -1 {
			t.Errorf("missing %d must give -1", x)
		}
	}
	if BinarySearch(nil, 1) != -1 {
		t.Error("nil slice")
	}
}
