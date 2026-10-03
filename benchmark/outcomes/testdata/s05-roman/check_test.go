package task

import (
	"testing"
)

func TestToRoman(t *testing.T) {
	for n, want := range map[int]string{1: "I", 4: "IV", 9: "IX", 14: "XIV", 40: "XL", 90: "XC", 400: "CD", 1994: "MCMXCIV", 2024: "MMXXIV", 3999: "MMMCMXCIX"} {
		if got := ToRoman(n); got != want {
			t.Errorf("ToRoman(%d)=%q want %q", n, got, want)
		}
	}
}
