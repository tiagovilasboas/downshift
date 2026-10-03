package task

import (
	"strings"
)

func IsAnagram(a, b string) bool {
	count := map[rune]int{}
	for _, r := range strings.ToLower(a) {
		if r != ' ' {
			count[r]++
		}
	}
	for _, r := range strings.ToLower(b) {
		if r != ' ' {
			count[r]--
		}
	}
	for _, c := range count {
		if c != 0 {
			return false
		}
	}
	return true
}
