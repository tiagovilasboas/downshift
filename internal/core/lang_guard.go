// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"regexp"
	"strings"
	"unicode"
)

// The signal and risk tables cover English and Portuguese. In any other
// language a risk word is invisible, while an English verb ("rename",
// "format") or a code identifier in the same prompt still scores, so the
// classifier can be confident about a task whose risk it cannot read.
// unsupportedLanguage spots such prompts so Classify can drop confidence:
// every guardrail then holds the requested model. partialForeignLanguage is
// the weaker hint (one foreign cue) for mixed briefs that are still mostly
// English. Neither changes the class and neither raises cost.

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

// mixedEnglishHelperBrief matches the synthetic M3 mixed brief stub (mixed-01…).
var mixedEnglishHelperBrief = regexp.MustCompile(`(?i)^(rename|format|move|add)\s+this\s+helper;\s`)

// partialForeignAmbiguous lists function words that also appear in English
// prose (locale codes, Python del, etc.). They still count toward
// unsupportedLanguage at two hits but not toward partialForeignLanguage.
var partialForeignAmbiguous = map[string]bool{
	"en": true, "del": true, "y": true,
}

// languageGuardState classifies foreign-language coverage in one pass.
func languageGuardState(prompt string) (unsupported, partialForeign bool) {
	nonLatin := 0
	for _, r := range prompt {
		if unicode.IsLetter(r) && r >= 0x80 && !unicode.Is(unicode.Latin, r) {
			nonLatin++
		}
	}
	hits := 0
	partialCues := 0
	proseLetter := false
	seen := map[string]bool{}
	split := func(r rune) bool { return !unicode.IsLetter(r) }
	for _, w := range strings.FieldsFunc(prompt, split) {
		wl := strings.ToLower(w)
		if !proseLetter && !identifierToken(w) && strings.ContainsAny(wl, foreignLetters) {
			proseLetter = true
			hits++
			partialCues++
		}
		if foreignFunctionWords[wl] && !seen[wl] {
			seen[wl] = true
			hits++
			if !partialForeignAmbiguous[wl] {
				partialCues++
			}
		}
	}
	if nonLatin >= minNonLatinLetters || hits >= 2 {
		return true, false
	}
	return false, partialCues == 1
}

// identifierToken is true for camelCase or Studly tokens so a stray diacritic
// in a field name does not count as a foreign-language cue.
func identifierToken(w string) bool {
	for i, r := range w {
		if i > 0 && unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

// unsupportedLanguage reports whether the prompt reads as a language the
// signal and risk tables do not cover.
func unsupportedLanguage(prompt string) bool {
	u, _ := languageGuardState(prompt)
	return u
}

// partialForeignLanguage is true when the prompt shows a single foreign cue
// (one function word or one foreign letter in prose) but not enough to flag
// unsupportedLanguage. Classify drops confidence so mixed EN+foreign risk
// briefs are not downshifted confidently.
func partialForeignLanguage(prompt string) bool {
	u, p := languageGuardState(prompt)
	return !u && p
}
