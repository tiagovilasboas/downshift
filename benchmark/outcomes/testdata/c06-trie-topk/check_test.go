package task

import (
	"reflect"
	"testing"
)

func TestTrie(t *testing.T) {
	tr := NewTrie()
	for w, wt := range map[string]int{"car": 5, "card": 9, "care": 5, "cart": 1, "cat": 7, "dog": 10, "carbon": 5} {
		tr.Insert(w, wt)
	}
	if got := tr.Top("car", 3); !reflect.DeepEqual(got, []string{"card", "car", "carbon"}) {
		t.Fatalf("Top(car,3)=%v", got)
	}
	if got := tr.Top("ca", 10); !reflect.DeepEqual(got, []string{"card", "cat", "car", "carbon", "care", "cart"}) {
		t.Fatalf("Top(ca,10)=%v", got)
	}
	tr.Insert("cart", 100)
	if got := tr.Top("c", 1); !reflect.DeepEqual(got, []string{"cart"}) {
		t.Fatalf("re-insert must replace weight: %v", got)
	}
	if len(tr.Top("x", 3)) != 0 || len(tr.Top("car", 0)) != 0 {
		t.Fatal("missing prefix or k=0 must return nothing")
	}
	if got := tr.Top("", 1); !reflect.DeepEqual(got, []string{"cart"}) {
		t.Fatalf("empty prefix: %v", got)
	}
}
