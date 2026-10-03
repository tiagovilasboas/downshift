package task

import (
	"reflect"
	"testing"
)

func TestTopoSort(t *testing.T) {
	got, err := TopoSort(map[string][]string{"app": {"lib", "log"}, "lib": {"log"}, "cli": {"app"}, "zz": nil})
	want := []string{"log", "lib", "app", "cli", "zz"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, %v want %v", got, err, want)
	}
	if _, err := TopoSort(map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"a"}}); err == nil {
		t.Fatal("expected cycle error")
	}
	if _, err := TopoSort(map[string][]string{"a": {"a"}}); err == nil {
		t.Fatal("expected self-cycle error")
	}
}
