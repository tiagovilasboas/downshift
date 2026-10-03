package task

import (
	"reflect"
	"testing"
)

func TestMerge(t *testing.T) {
	in := [][2]int{{8, 10}, {1, 3}, {2, 6}, {15, 18}, {6, 7}, {18, 20}}
	orig := append([][2]int(nil), in...)
	got := Merge(in)
	want := [][2]int{{1, 7}, {8, 10}, {15, 20}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if !reflect.DeepEqual(in, orig) {
		t.Fatal("input mutated")
	}
	if len(Merge(nil)) != 0 {
		t.Fatal("empty")
	}
	if got := Merge([][2]int{{1, 10}, {2, 3}}); !reflect.DeepEqual(got, [][2]int{{1, 10}}) {
		t.Fatalf("contained interval: %v", got)
	}
}
