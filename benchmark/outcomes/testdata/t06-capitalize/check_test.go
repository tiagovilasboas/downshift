package task

import (
	"testing"
)

func TestCapitalize(t *testing.T) {
	for in, want := range map[string]string{"": "", "go": "Go", "Go": "Go", "élan vital": "Élan vital", "1abc": "1abc", "hELLO": "HELLO"} {
		if got := Capitalize(in); got != want {
			t.Errorf("Capitalize(%q)=%q want %q", in, got, want)
		}
	}
}
