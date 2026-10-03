package task

import (
	"math"
	"testing"
)

func TestEval(t *testing.T) {
	ok := map[string]float64{"1 + 2 * 3": 7, "(1 + 2) * 3": 9, "10 - 4 - 3": 3, "8 / 4 / 2": 1, "-3 + 5": 2, "-(2 + 3) * 2": -10, "2 * -3": -6, "1.5 + 2.25": 3.75, " 42 ": 42, "((7))": 7}
	for in, want := range ok {
		got, err := Eval(in)
		if err != nil || math.Abs(got-want) > 1e-9 {
			t.Errorf("Eval(%q)=%v,%v want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "1 +", "(1 + 2", "1 2", "1 / 0", "2 * )", "abc"} {
		if _, err := Eval(in); err == nil {
			t.Errorf("Eval(%q): expected error", in)
		}
	}
}
