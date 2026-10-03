package task

import (
	"reflect"
	"testing"
)

func TestDedupe(t *testing.T) {
	got := Dedupe([]string{"b", "a", "b", "c", "a", "b"})
	if !reflect.DeepEqual(got, []string{"b", "a", "c"}) {
		t.Fatalf("got %v", got)
	}
	if len(Dedupe(nil)) != 0 {
		t.Fatal("nil input must give empty output")
	}
}
