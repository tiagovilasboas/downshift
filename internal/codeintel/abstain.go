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
	"thanks": true, "thank you": true, "thanks for your help": true, "thank you for your help": true,
	"thanks for the help": true, "thank you so much": true, "muito obrigado": true,
	"obrigado": true, "valeu": true, "bye": true, "goodbye": true,
	"ok": true, "okay": true, "yes": true, "no": true, "sure": true, "cool": true,
	"how are you": true, "what's up": true, "whats up": true, "tudo bem": true, "como vai": true,
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

	var hasSymbols bool
	var hasPath bool
	var top *Candidate
	if len(candidates) > 0 {
		top = candidates[0]
		hasSymbols = len(top.MatchedSymbols) > 0
		hasPath = hasPathTokenMatch(top)
	}

	// 1. Check conversational greetings / chatter prefixes FIRST
	for greeting := range conversationalGreetings {
		if cleanPrompt == greeting {
			return true, "conversational_greeting"
		}
		if strings.HasPrefix(cleanPrompt, greeting+" ") ||
			strings.HasPrefix(cleanPrompt, greeting+"!") ||
			strings.HasPrefix(cleanPrompt, greeting+",") ||
			strings.HasPrefix(cleanPrompt, greeting+"?") {
			// If it begins with conversational greeting, only proceed if explicit symbol match exists
			if !hasSymbols {
				return true, "conversational_greeting"
			}
		}
	}

	if len(queryTokens) == 0 {
		return true, "no_query_tokens"
	}

	if len(candidates) == 0 {
		return true, "no_candidates_found"
	}

	// 2. Explicit symbol match
	if hasSymbols {
		return false, ""
	}

	// 3. Exact file path token match in matched terms
	if hasPath {
		return false, ""
	}

	// 4. At least 2 distinct domain terms matched with verified postings
	uniqueMatched := make(map[string]bool)
	for _, mt := range top.MatchedTerms {
		uniqueMatched[mt] = true
	}
	if len(uniqueMatched) >= 2 {
		return false, ""
	}

	return true, "insufficient_lexical_evidence"
}

func hasPathTokenMatch(cand *Candidate) bool {
	if cand == nil || cand.Document == nil {
		return false
	}
	pathTokens := Tokenize(cand.Document.RelativePath)
	for _, pt := range pathTokens {
		for _, mt := range cand.MatchedTerms {
			if pt == mt {
				return true
			}
		}
	}
	return false
}
