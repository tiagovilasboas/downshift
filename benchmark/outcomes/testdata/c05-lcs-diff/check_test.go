package task

import (
	"strings"
	"testing"
)

func lcs(a, b []string) int {
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	return dp[len(a)][len(b)]
}

func TestDiff(t *testing.T) {
	cases := [][2][]string{
		{strings.Split("a b c d e f", " "), strings.Split("a c d x f g", " ")},
		{nil, []string{"x", "y"}},
		{[]string{"x", "y"}, nil},
		{strings.Split("same lines here", " "), strings.Split("same lines here", " ")},
		{strings.Split("a b a b a", " "), strings.Split("b a b a b", " ")},
	}
	for _, c := range cases {
		a, b := c[0], c[1]
		d := Diff(a, b)
		var ra, rb []string
		same := 0
		for _, line := range d {
			if len(line) < 2 {
				t.Fatalf("bad line %q", line)
			}
			body := line[2:]
			switch line[:2] {
			case "  ":
				ra, rb = append(ra, body), append(rb, body)
				same++
			case "- ":
				ra = append(ra, body)
			case "+ ":
				rb = append(rb, body)
			default:
				t.Fatalf("bad prefix in %q", line)
			}
		}
		if strings.Join(ra, "|") != strings.Join(a, "|") || strings.Join(rb, "|") != strings.Join(b, "|") {
			t.Fatalf("diff does not rebuild inputs: %q", d)
		}
		if same != lcs(a, b) {
			t.Fatalf("diff not minimal: %d unchanged, LCS %d", same, lcs(a, b))
		}
	}
}
