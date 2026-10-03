package task

import (
	"reflect"
	"testing"
)

func TestWordFreq(t *testing.T) {
	got := WordFreq("The cat, the HAT; the-end 42")
	want := map[string]int{"the": 3, "cat": 1, "hat": 1, "end": 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if len(WordFreq("  ... 123 ")) != 0 {
		t.Fatal("expected no words")
	}
}
