package task

import (
	"strings"
	"unicode"
)

func WordFreq(text string) map[string]int {
	out := map[string]int{}
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) }) {
		out[w]++
	}
	return out
}
