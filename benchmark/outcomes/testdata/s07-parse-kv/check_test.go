package task

import (
	"reflect"
	"testing"
)

func TestParseKV(t *testing.T) {
	got := ParseKV("a=1; b = 2 ;junk; =x; c=; d=e=f")
	want := map[string]string{"a": "1", "b": "2", "c": "", "d": "e=f"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
