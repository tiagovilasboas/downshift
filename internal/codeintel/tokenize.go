// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"strings"
	"unicode"
)

// defaultStopwords contains non-discriminative English and Portuguese stopwords.
// Technical terms, short acronyms and domain words are intentionally preserved.
var defaultStopwords = map[string]bool{
	// English
	"a": true, "an": true, "the": true, "and": true, "or": true, "of": true,
	"to": true, "in": true, "on": true, "at": true, "for": true, "with": true,
	"is": true, "it": true, "as": true, "be": true, "this": true, "that": true,
	// Portuguese
	"o": true, "os": true, "de": true, "do": true, "da": true, "dos": true,
	"das": true, "em": true, "no": true, "na": true, "nos": true, "nas": true,
	"por": true, "para": true, "com": true, "um": true, "uma": true, "uns": true,
	"umas": true, "se": true, "que": true, "como": true,
}

// Tokenize takes text (a prompt, code identifier, file path, or source line)
// and splits it into normalized, lowercase terms.
// It handles camelCase, PascalCase, snake_case, kebab-case, acronyms, and file paths.
func Tokenize(text string) []string {
	if text == "" {
		return nil
	}

	rawTokens := splitByPunctuationAndWhitespace(text)
	var result []string

	for _, raw := range rawTokens {
		subTokens := splitIdentifier(raw)
		for _, sub := range subTokens {
			norm := strings.ToLower(sub)
			if len(norm) == 0 {
				continue
			}
			// Skip stopwords unless it's a critical technical short identifier
			if defaultStopwords[norm] {
				continue
			}
			result = append(result, norm)
		}
	}

	return result
}

// splitByPunctuationAndWhitespace splits input string into alphanumeric chunks,
// separating on path separators, spaces, punctuation and symbols.
func splitByPunctuationAndWhitespace(text string) []string {
	var tokens []string
	var cur strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// splitIdentifier splits a single word into constituent parts handling:
// - camelCase (e.g. "processRecurringPayment" -> ["process", "Recurring", "Payment"])
// - PascalCase (e.g. "PaymentService" -> ["Payment", "Service"])
// - Acronyms (e.g. "HTTPServer" -> ["HTTP", "Server"], "userID" -> ["user", "ID"])
// - Acronyms mid-word (e.g. "processPIXPayment" -> ["process", "PIX", "Payment"])
func splitIdentifier(word string) []string {
	runes := []rune(word)
	n := len(runes)
	if n == 0 {
		return nil
	}

	var parts []string
	start := 0

	for i := 0; i < n; i++ {
		// Case 1: Transition from lowercase/digit to uppercase
		// e.g. "user" -> "I" in "userID", "process" -> "P" in "processPIXPayment"
		if i > 0 && unicode.IsUpper(runes[i]) && (!unicode.IsUpper(runes[i-1])) {
			if i > start {
				parts = append(parts, string(runes[start:i]))
				start = i
			}
			continue
		}

		// Case 2: Transition from acronym to capitalized word
		// e.g. "HTTP" -> "Server" where i is 'S', i-1 is 'P' (upper), and i+1 is 'e' (lower)
		if i > 0 && i+1 < n && unicode.IsUpper(runes[i]) && unicode.IsUpper(runes[i-1]) && unicode.IsLower(runes[i+1]) && (i-start > 1) {
			if i > start {
				parts = append(parts, string(runes[start:i]))
				start = i
			}
			continue
		}
	}

	if start < n {
		parts = append(parts, string(runes[start:]))
	}

	return parts
}

// TermFrequencies returns a map of unique terms to their occurrence counts in the given text.
func TermFrequencies(text string) (map[string]int, int) {
	tokens := Tokenize(text)
	counts := make(map[string]int, len(tokens))
	for _, t := range tokens {
		counts[t]++
	}
	return counts, len(tokens)
}
