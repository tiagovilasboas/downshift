// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"strings"
	"unicode"
)

// The signal and risk tables cover English and Portuguese. In any other
// language a risk word is invisible, while an English verb ("rename",
// "format") or a code identifier in the same prompt still scores, so the
// classifier can be confident about a task whose risk it cannot read.
// unsupportedLanguage spots such prompts so Classify can drop confidence:
// every guardrail then holds the requested model. It never changes the class
// and never raises cost.

// foreignFunctionWords are frequent Spanish, French and German function words
// that are not English or Portuguese words. Words shared with Portuguese (de,
// que, para, por, das) or common in English prose (es, est) are left out;
// "die" and "den" are kept because two hits are required.
var foreignFunctionWords = map[string]bool{
	// Spanish
	"el": true, "los": true, "las": true, "del": true, "y": true, "en": true, "con": true,
	"una": true, "cuando": true, "pero": true, "está": true,
	// French
	"le": true, "la": true, "les": true, "des": true, "du": true, "et": true, "dans": true,
	"qui": true, "une": true, "pour": true, "avec": true, "sur": true, "puis": true, "pas": true,
	// German
	"der": true, "die": true, "den": true, "dem": true, "und": true, "mit": true, "zu": true,
	"im": true, "ist": true, "nicht": true, "ein": true, "eine": true, "auf": true, "aus": true,
	"für": true, "von": true,
}

// foreignLetters are Latin letters used by Spanish, French or German but not
// by current Portuguese spelling. Any of them counts as one hit.
const foreignLetters = "ñ¿¡üöäßèùëïîûœìò"

// minNonLatinLetters tolerates a stray Greek symbol (λ, μs) in an English
// prompt while catching a sentence in another script (Japanese, Chinese,
// Korean, Cyrillic, Arabic...).
const minNonLatinLetters = 4

// unsupportedLanguage reports whether the prompt reads as a language the
// signal and risk tables do not cover.
func unsupportedLanguage(prompt string) bool {
	nonLatin := 0
	for _, r := range prompt {
		if unicode.IsLetter(r) && r >= 0x80 && !unicode.Is(unicode.Latin, r) {
			nonLatin++
			if nonLatin >= minNonLatinLetters {
				return true
			}
		}
	}
	lower := strings.ToLower(prompt)
	hits := 0
	if strings.ContainsAny(lower, foreignLetters) {
		hits++
	}
	seen := map[string]bool{}
	for _, w := range strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r)
	}) {
		if foreignFunctionWords[w] && !seen[w] {
			seen[w] = true
			hits++
			if hits >= 2 {
				return true
			}
		}
	}
	return hits >= 2
}
