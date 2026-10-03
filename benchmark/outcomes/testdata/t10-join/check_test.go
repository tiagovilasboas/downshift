package task

import (
	"testing"
)

func TestJoinComma(t *testing.T) {
	if JoinComma(nil) != "" || JoinComma([]string{"a"}) != "a" || JoinComma([]string{"a", "b", "c"}) != "a, b, c" {
		t.Fatal("wrong join")
	}
}
