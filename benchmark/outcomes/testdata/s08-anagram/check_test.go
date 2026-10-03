package task

import (
	"testing"
)

func TestIsAnagram(t *testing.T) {
	if !IsAnagram("Dormitory", "dirty room") || !IsAnagram("", "  ") || IsAnagram("abc", "abd") || IsAnagram("aab", "abb") {
		t.Fatal("wrong result")
	}
}
