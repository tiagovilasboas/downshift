package task

import (
	"reflect"
	"testing"
)

func TestWrap(t *testing.T) {
	got := Wrap("the quick  brown fox jumps over the lazy dog", 10)
	want := []string{"the quick", "brown fox", "jumps over", "the lazy", "dog"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
	got = Wrap("a supercalifragilistic word", 6)
	want = []string{"a", "supercalifragilistic", "word"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("long word: got %q", got)
	}
	if len(Wrap("   ", 5)) != 0 {
		t.Fatal("blank text")
	}
}
