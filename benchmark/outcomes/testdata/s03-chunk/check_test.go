package task

import (
	"reflect"
	"testing"
)

func TestChunk(t *testing.T) {
	got := Chunk([]int{1, 2, 3, 4, 5}, 2)
	if !reflect.DeepEqual(got, [][]int{{1, 2}, {3, 4}, {5}}) {
		t.Fatalf("got %v", got)
	}
	if Chunk([]int{1}, 0) != nil || Chunk([]int{1}, -1) != nil {
		t.Fatal("n<=0 must return nil")
	}
	if len(Chunk(nil, 3)) != 0 {
		t.Fatal("empty input")
	}
}
