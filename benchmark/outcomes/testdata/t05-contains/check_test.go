package task

import (
	"testing"
)

func TestContains(t *testing.T) {
	xs := []string{"a", "b", ""}
	if !Contains(xs, "b") || !Contains(xs, "") || Contains(xs, "c") || Contains(nil, "a") {
		t.Fatal("wrong result")
	}
}
