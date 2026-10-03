package task

import (
	"strings"
	"testing"
)

func TestEvalRPN(t *testing.T) {
	ok := map[string]int{"2 1 + 3 *": 9, "4 13 5 / +": 6, "10 6 9 3 + -11 * / * 17 + 5 +": 22, "7 -2 /": -3, "5": 5}
	for in, want := range ok {
		got, err := EvalRPN(strings.Fields(in))
		if err != nil || got != want {
			t.Errorf("%q => %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"1 +", "1 2", "1 0 /", "a 1 +", ""} {
		if _, err := EvalRPN(strings.Fields(in)); err == nil {
			t.Errorf("%q: expected error", in)
		}
	}
}
