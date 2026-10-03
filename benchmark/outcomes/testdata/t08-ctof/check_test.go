package task

import (
	"math"
	"testing"
)

func TestCToF(t *testing.T) {
	for c, f := range map[float64]float64{0: 32, 100: 212, -40: -40, 37: 98.6} {
		if got := CToF(c); math.Abs(got-f) > 1e-9 {
			t.Errorf("CToF(%v)=%v want %v", c, got, f)
		}
	}
}
