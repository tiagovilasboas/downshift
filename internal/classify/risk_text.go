// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"strings"
	"unicode"
)

// nonASCIIFold maps each non-ASCII rune that (?i) treats as an ASCII letter
// (U+212A KELVIN SIGN as k, U+017F LONG S as s) to that letter. Computed from
// unicode.SimpleFold, the same orbit the regexp engine uses for case folding.
var nonASCIIFold = func() map[rune]byte {
	out := map[rune]byte{}
	for c := 'a'; c <= 'z'; c++ {
		for r := unicode.SimpleFold(c); r != c; r = unicode.SimpleFold(r) {
			if r >= 0x80 {
				out[r] = byte(c)
			}
		}
	}
	return out
}()

// foldRiskText lowercases ASCII and maps the non-ASCII case-fold partners of
// ASCII letters onto those letters. It is used only for the literal prefilter.
// Invalid UTF-8 becomes U+FFFD, which never equals an ASCII literal byte.
func foldRiskText(text string) string {
	if !needsFold(text) {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteByte(byte(r) + ('a' - 'A'))
		case r < 0x80:
			b.WriteByte(byte(r))
		case nonASCIIFold[r] != 0:
			b.WriteByte(nonASCIIFold[r])
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func needsFold(text string) bool {
	for i := 0; i < len(text); i++ {
		if c := text[i]; (c >= 'A' && c <= 'Z') || c >= 0x80 {
			return true
		}
	}
	return false
}

func isLowerOrDigit(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') }
func isUpper(c byte) bool        { return c >= 'A' && c <= 'Z' }

// splitIdentifiers turns code identifiers into words so risk vocabulary is
// found inside them: creditCardNumber → "credit Card Number", api_key →
// "api key". It is the same as replacing each [a-z0-9][A-Z] pair with the pair
// and a space, then each "_" with a space, but it returns the prompt unchanged
// (no allocation) when there is nothing to split.
func splitIdentifiers(prompt string) string {
	if !needsSplit(prompt) {
		return prompt
	}
	var b strings.Builder
	b.Grow(len(prompt) + 16)
	for i := 0; i < len(prompt); i++ {
		c := prompt[i]
		if c == '_' {
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(c)
		if i+1 < len(prompt) && isLowerOrDigit(c) && isUpper(prompt[i+1]) {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

func needsSplit(prompt string) bool {
	for i := 0; i < len(prompt); i++ {
		c := prompt[i]
		if c == '_' || (i+1 < len(prompt) && isLowerOrDigit(c) && isUpper(prompt[i+1])) {
			return true
		}
	}
	return false
}
