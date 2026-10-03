package task

import (
	"reflect"
	"testing"
)

func TestSpiral(t *testing.T) {
	cases := []struct {
		m    [][]int
		want []int
	}{
		{[][]int{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}, []int{1, 2, 3, 6, 9, 8, 7, 4, 5}},
		{[][]int{{1, 2, 3, 4}, {5, 6, 7, 8}, {9, 10, 11, 12}}, []int{1, 2, 3, 4, 8, 12, 11, 10, 9, 5, 6, 7}},
		{[][]int{{1}, {2}, {3}}, []int{1, 2, 3}},
		{[][]int{{1, 2, 3}}, []int{1, 2, 3}},
		{nil, []int{}},
	}
	for _, c := range cases {
		if got := Spiral(c.m); !reflect.DeepEqual(got, c.want) {
			t.Errorf("Spiral(%v)=%v want %v", c.m, got, c.want)
		}
	}
}
