package task

import (
	"testing"
)

func TestCountEven(t *testing.T) {
	if CountEven([]int{1, 2, 3, 4, -6, 0, -1}) != 4 || CountEven(nil) != 0 {
		t.Fatal("wrong count")
	}
}
