package task

import (
	"testing"
)

func TestClamp(t *testing.T) {
	cases := [][4]int{{5, 0, 10, 5}, {-3, 0, 10, 0}, {42, 0, 10, 10}, {0, 0, 0, 0}, {10, 0, 10, 10}}
	for _, c := range cases {
		if got := Clamp(c[0], c[1], c[2]); got != c[3] {
			t.Errorf("Clamp(%d,%d,%d)=%d want %d", c[0], c[1], c[2], got, c[3])
		}
	}
}
