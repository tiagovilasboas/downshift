package task

import (
	"testing"
)

func TestSum(t *testing.T) {
	if Sum(nil) != 0 || Sum([]int{1, 2, 3}) != 6 || Sum([]int{-5, 5, 7}) != 7 {
		t.Fatal("wrong sum")
	}
}
