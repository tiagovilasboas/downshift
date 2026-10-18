// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"strings"
	"unicode"
)

var conversationalGreetings = map[string]bool{
	"hi": true, "hello": true, "hey": true, "hello there": true, "hi there": true, "hey there": true,
	"good morning": true, "good evening": true, "good afternoon": true,
	"ola": true, "olá": true, "oi": true, "bom dia": true, "boa tarde": true, "boa noite": true, "e ai": true, "e aí": true,
	"thanks": true, "thank you": true, "obrigado": true, "valeu": true, "bye": true, "goodbye": true,
	"ok": true, "okay": true, "yes": true, "no": true, "sure": true, "cool": true,
}

// ShouldAbstain evaluates whether DCI should abstain from injecting context.
// Abstention reasons include conversational greetings, short chat, lack of code relevance,
// lack of symbol or path evidence, or zero candidate matches.
// Requires explicit symbol/path OR >=2 distinct domain terms with verified index postings.
func ShouldAbstain(prompt string, queryTokens []string, candidates []*Candidate) (bool, string) {
	cleanPrompt := strings.ToLower(strings.TrimFunc(prompt, func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSpace(r)
	}))

	if cleanPrompt == "" {
		return true, "empty_prompt"
	}

	if conversationalGreetings[cleanPrompt] {
		return true, "conversational_greeting"
	}

	if len(queryTokens) == 0 {
		return true, "no_query_tokens"
	}

	if len(candidates) == 0 {
		return true, "no_candidates_found"
	}

	// Check if top candidate has sufficient relevance
	top := candidates[0]

	// 1. Explicit symbol match
	if len(top.MatchedSymbols) > 0 {
		return false, ""
	}

	// 2. Exact file path token match in matched terms
	pathTokens := Tokenize(top.Document.RelativePath)
	hasPathMatch := false
	for _, pt := range pathTokens {
		for _, mt := range top.MatchedTerms {
			if pt == mt {
				hasPathMatch = true
				break
			}
		}
		if hasPathMatch {
			break
		}
	}
	if hasPathMatch {
		return false, ""
	}

	// 3. At least 2 distinct domain terms matched
	uniqueMatched := make(map[string]bool)
	for _, mt := range top.MatchedTerms {
		uniqueMatched[mt] = true
	}
	if len(uniqueMatched) >= 2 {
		return false, ""
	}

	return true, "insufficient_lexical_evidence"
}
