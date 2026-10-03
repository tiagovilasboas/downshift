package task

import (
	"strings"
	"testing"
	"time"
)

func TestMatch(t *testing.T) {
	cases := []struct {
		p, s string
		want bool
	}{
		{"abc", "abc", true}, {"abc", "abcd", false}, {"a.c", "axc", true}, {"a*", "", true},
		{"a*b", "aaab", true}, {"a+b", "b", false}, {"a+b", "aab", true}, {"colou?r", "color", true},
		{"colou?r", "colour", true}, {"colou?r", "colouur", false}, {".*", "anything", true},
		{".*x", "abc", false}, {"a.*b.?c+", "a123bcc", true}, {"", "", true}, {"", "a", false},
		{"x*y*z*", "xxzz", true}, {"ab?", "a", true},
	}
	for _, c := range cases {
		if got := Match(c.p, c.s); got != c.want {
			t.Errorf("Match(%q,%q)=%v want %v", c.p, c.s, got, c.want)
		}
	}
	long := strings.Repeat("a", 40)
	start := time.Now()
	if Match(strings.Repeat("a*", 20)+"b", long) {
		t.Error("pathological pattern must not match")
	}
	if time.Since(start) > 2*time.Second {
		t.Error("exponential backtracking: memoise the matcher")
	}
}
