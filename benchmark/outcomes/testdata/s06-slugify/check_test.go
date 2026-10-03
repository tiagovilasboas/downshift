package task

import (
	"testing"
)

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{"Hello, World!": "hello-world", "  Go 1.22 -- release ": "go-1-22-release", "---": "", "Ünïcode Text": "n-code-text", "a": "a"} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q)=%q want %q", in, got, want)
		}
	}
}
