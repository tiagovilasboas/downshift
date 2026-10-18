// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"testing"
)

func TestAbstain_Greetings(t *testing.T) {
	greetings := []string{
		"oi",
		"Oi!",
		"obrigado",
		"Obrigado!",
		"ola",
		"olá",
		"bom dia",
		"boa tarde",
		"boa noite",
		"valeu",
		"hello",
		"Hello there!",
		"hi",
		"hey",
		"thanks",
		"thank you",
		"ok",
		"okay",
		"yes",
		"sure",
		"cool",
		"bye",
	}

	for _, g := range greetings {
		t.Run(g, func(t *testing.T) {
			queryTokens := Tokenize(g)
			abstain, reason := ShouldAbstain(g, queryTokens, nil)
			if !abstain {
				t.Errorf("ShouldAbstain(%q) = false, want true", g)
			}
			if reason != "conversational_greeting" {
				t.Errorf("ShouldAbstain(%q) reason = %q, want 'conversational_greeting'", g, reason)
			}
		})
	}
}

func TestAbstain_ChatterAndEmpty(t *testing.T) {
	// 1. Empty and whitespace-only prompts
	emptyPrompts := []string{"", "   ", "\t\n"}
	for _, p := range emptyPrompts {
		abstain, reason := ShouldAbstain(p, Tokenize(p), nil)
		if !abstain || reason != "empty_prompt" {
			t.Errorf("ShouldAbstain(%q) = (%v, %s), want (true, 'empty_prompt')", p, abstain, reason)
		}
	}

	// 2. Short chatter with no candidate match
	chatter := "continue"
	abstain, reason := ShouldAbstain(chatter, Tokenize(chatter), nil)
	if !abstain {
		t.Errorf("ShouldAbstain(%q) with nil candidates = false, want true", chatter)
	}
	if reason != "no_candidates_found" && reason != "conversational_greeting" {
		t.Errorf("ShouldAbstain(%q) unexpected reason: %s", chatter, reason)
	}
}

func TestAbstain_TechnicalQueriesTriggering(t *testing.T) {
	sampleDoc := &Document{
		RelativePath: "pkg/payment/service.go",
		TotalTerms:   100,
		TermFreqs:    map[string]int{"payment": 5, "pix": 4, "service": 2},
		Symbols: []Symbol{
			{Name: "ProcessPayment", Kind: KindFunction, RelativePath: "pkg/payment/service.go", StartLine: 10, EndLine: 30},
		},
	}

	// Case 1: Explicit symbol match triggers injection
	t.Run("ExplicitSymbolMatch", func(t *testing.T) {
		cands := []*Candidate{
			{
				Document:       sampleDoc,
				Score:          15.0,
				MatchedTerms:   []string{"payment"},
				MatchedSymbols: sampleDoc.Symbols,
			},
		}
		prompt := "Onde fica o ProcessPayment?"
		abstain, reason := ShouldAbstain(prompt, Tokenize(prompt), cands)
		if abstain {
			t.Errorf("expected triggering on explicit symbol match, got abstain=true, reason=%s", reason)
		}
	})

	// Case 2: Exact file path token match triggers injection
	t.Run("FilePathMatch", func(t *testing.T) {
		cands := []*Candidate{
			{
				Document:       sampleDoc,
				Score:          10.0,
				MatchedTerms:   []string{"service"}, // "service" matches token in "pkg/payment/service.go"
				MatchedSymbols: nil,
			},
		}
		prompt := "Check service file"
		abstain, reason := ShouldAbstain(prompt, Tokenize(prompt), cands)
		if abstain {
			t.Errorf("expected triggering on file path token match, got abstain=true, reason=%s", reason)
		}
	})

	// Case 3: At least 2 distinct domain terms triggers injection
	t.Run("TwoDistinctDomainTerms", func(t *testing.T) {
		docGeneric := &Document{
			RelativePath: "internal/core/handler.go",
			TotalTerms:   100,
			TermFreqs:    map[string]int{"invoice": 3, "recurrence": 2},
		}
		cands := []*Candidate{
			{
				Document:       docGeneric,
				Score:          8.0,
				MatchedTerms:   []string{"invoice", "recurrence"}, // 2 distinct terms
				MatchedSymbols: nil,
			},
		}
		prompt := "Handle invoice recurrence calculation"
		abstain, reason := ShouldAbstain(prompt, Tokenize(prompt), cands)
		if abstain {
			t.Errorf("expected triggering on 2 distinct domain terms, got abstain=true, reason=%s", reason)
		}
	})
}

func TestAbstain_InsufficientLexicalEvidence(t *testing.T) {
	// Candidate has only 1 matching term, no symbol match, no path match
	docGeneric := &Document{
		RelativePath: "internal/core/handler.go",
		TotalTerms:   100,
		TermFreqs:    map[string]int{"data": 1},
	}
	cands := []*Candidate{
		{
			Document:       docGeneric,
			Score:          1.0,
			MatchedTerms:   []string{"data"}, // only 1 term
			MatchedSymbols: nil,
		},
	}
	prompt := "show me data"
	abstain, reason := ShouldAbstain(prompt, Tokenize(prompt), cands)
	if !abstain {
		t.Error("expected abstention on single generic term without symbol/path match")
	}
	if reason != "insufficient_lexical_evidence" {
		t.Errorf("reason = %s, want 'insufficient_lexical_evidence'", reason)
	}
}
