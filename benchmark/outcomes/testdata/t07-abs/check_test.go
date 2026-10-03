package task

import (
	"testing"
)

func TestAbs(t *testing.T) {
	if Abs(-3) != 3 || Abs(0) != 0 || Abs(8) != 8 {
		t.Fatal("wrong abs")
	}
}
