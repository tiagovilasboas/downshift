// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"strings"
	"testing"
)

func TestSearchBM25(t *testing.T) {
	idx := NewInvertedIndex()

	doc1 := &Document{
		RelativePath: "pkg/auth/token.go",
		TotalTerms:   10,
		TermFreqs:    map[string]int{"jwt": 5, "token": 3, "auth": 2},
		Symbols: []Symbol{
			{Name: "ValidateJWT", Kind: KindFunction, RelativePath: "pkg/auth/token.go", StartLine: 10, EndLine: 20},
		},
	}

	doc2 := &Document{
		RelativePath: "pkg/payment/service.go",
		TotalTerms:   20,
		TermFreqs:    map[string]int{"payment": 6, "pix": 4, "service": 2},
		Symbols: []Symbol{
			{Name: "ProcessPayment", Kind: KindFunction, RelativePath: "pkg/payment/service.go", StartLine: 15, EndLine: 35},
		},
	}

	idx.AddDocument(doc1)
	idx.AddDocument(doc2)
	idx.RebuildPostings()

	// Query for payment pix
	queryTokens := []string{"payment", "pix"}
	results := Search(idx, queryTokens)

	if len(results) == 0 {
		t.Fatal("Expected search results, got 0")
	}
	if results[0].Document.RelativePath != "pkg/payment/service.go" {
		t.Errorf("Top result = %s, want pkg/payment/service.go", results[0].Document.RelativePath)
	}
	if len(results[0].MatchedSymbols) == 0 {
		t.Error("Expected matched symbol ProcessPayment")
	}
}

func TestAbstention(t *testing.T) {
	// Greeting test
	abstain, reason := ShouldAbstain("Hello there!", []string{"hello"}, nil)
	if !abstain || reason != "conversational_greeting" {
		t.Errorf("Expected greeting abstention, got abstain=%v, reason=%s", abstain, reason)
	}

	// Empty prompt
	abstain, reason = ShouldAbstain("", nil, nil)
	if !abstain || reason != "empty_prompt" {
		t.Errorf("Expected empty prompt abstention, got abstain=%v, reason=%s", abstain, reason)
	}

	// Insufficient evidence
	cands := []*Candidate{
		{
			Document:     &Document{RelativePath: "foo.go"},
			MatchedTerms: []string{"random"},
		},
	}
	abstain, reason = ShouldAbstain("do random stuff", []string{"random"}, cands)
	if !abstain || reason != "insufficient_lexical_evidence" {
		t.Errorf("Expected insufficient lexical evidence, got abstain=%v, reason=%s", abstain, reason)
	}
}

func TestPackContext(t *testing.T) {
	doc := &Document{
		RelativePath: "pkg/user/service.go",
		Symbols: []Symbol{
			{Name: "GetUser", Kind: KindFunction, StartLine: 10, EndLine: 20},
		},
	}
	cands := []*Candidate{
		{
			Document:       doc,
			Score:          10.5,
			MatchedTerms:   []string{"user"},
			MatchedSymbols: doc.Symbols,
			Snippets: []Snippet{
				{
					RelativePath: "pkg/user/service.go",
					StartLine:    10,
					EndLine:      12,
					Content:      "func GetUser() error {\n    return nil\n}",
				},
			},
		},
	}

	pack := PackContext(cands, 3, 3500)
	if pack == nil {
		t.Fatal("PackContext returned nil")
	}
	if !strings.Contains(pack.FormattedText, UntrustedContextSecurityNotice) {
		t.Error("Formatted text missing prompt injection disclaimer")
	}
	if !strings.Contains(pack.FormattedText, "pkg/user/service.go") {
		t.Error("Formatted text missing file path")
	}
	if !strings.Contains(pack.FormattedText, "GetUser") {
		t.Error("Formatted text missing symbol name")
	}

	// Test truncation UTF-8 boundary
	shortPack := PackContext(cands, 3, 50)
	if len([]byte(shortPack.FormattedText)) > 50 {
		t.Errorf("Output length %d exceeds max 50 bytes", len([]byte(shortPack.FormattedText)))
	}
}
