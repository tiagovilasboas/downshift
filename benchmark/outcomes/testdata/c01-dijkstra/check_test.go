package task

import (
	"reflect"
	"testing"
)

func TestShortestPath(t *testing.T) {
	edges := [][3]int{{0, 1, 4}, {0, 2, 1}, {2, 1, 2}, {1, 3, 1}, {2, 3, 5}, {3, 4, 3}, {5, 4, 1}}
	d, p, ok := ShortestPath(6, edges, 0, 4)
	if !ok || d != 7 || !reflect.DeepEqual(p, []int{0, 2, 1, 3, 4}) {
		t.Fatalf("got %d %v %v", d, p, ok)
	}
	if _, _, ok := ShortestPath(6, edges, 0, 5); ok {
		t.Fatal("5 is unreachable from 0")
	}
	if d, p, ok := ShortestPath(6, edges, 3, 3); !ok || d != 0 || !reflect.DeepEqual(p, []int{3}) {
		t.Fatalf("self path: %d %v %v", d, p, ok)
	}
	// larger chain with a tempting but longer shortcut
	var big [][3]int
	for i := 0; i < 999; i++ {
		big = append(big, [3]int{i, i + 1, 1})
	}
	big = append(big, [3]int{0, 999, 1000})
	if d, _, ok := ShortestPath(1000, big, 0, 999); !ok || d != 999 {
		t.Fatalf("chain: %d %v", d, ok)
	}
}
