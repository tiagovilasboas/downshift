// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPackContext_BudgetEnforcement(t *testing.T) {
	// Create candidates with very large snippets (10 KiB)
	largeSnippet := strings.Repeat("// A line of code with data and logic\n", 200)

	cands := []*Candidate{
		{
			Document: &Document{
				RelativePath: "pkg/payment/processor.go",
				Symbols: []Symbol{
					{Name: "ProcessPayment", Kind: KindFunction, RelativePath: "pkg/payment/processor.go", StartLine: 10, EndLine: 50},
					{Name: "ValidateCard", Kind: KindFunction, RelativePath: "pkg/payment/processor.go", StartLine: 55, EndLine: 80},
				},
			},
			Score:        25.0,
			MatchedTerms: []string{"payment", "processor"},
			MatchedSymbols: []Symbol{
				{Name: "ProcessPayment", Kind: KindFunction, RelativePath: "pkg/payment/processor.go", StartLine: 10, EndLine: 50},
			},
			Snippets: []Snippet{
				{
					RelativePath: "pkg/payment/processor.go",
					StartLine:    10,
					EndLine:      50,
					Content:      largeSnippet,
				},
			},
		},
	}

	budgets := []int{100, 250, 500, 1000, 2000, 3500}
	for _, maxBytes := range budgets {
		t.Run(fmt.Sprintf("Budget_%d_Bytes", maxBytes), func(t *testing.T) {
			pack := PackContext(cands, 3, maxBytes)
			if pack == nil {
				t.Fatalf("PackContext returned nil for budget %d", maxBytes)
			}

			rawBytes := []byte(pack.FormattedText)
			if len(rawBytes) > maxBytes {
				t.Errorf("len(rawBytes) = %d exceeds maxBytes %d", len(rawBytes), maxBytes)
			}
			if pack.OutputBytes != len(rawBytes) {
				t.Errorf("pack.OutputBytes = %d, want %d", pack.OutputBytes, len(rawBytes))
			}

			// Verify UTF-8 validity
			if !utf8.Valid(rawBytes) {
				t.Errorf("Output for budget %d is not valid UTF-8", maxBytes)
			}
		})
	}
}

func TestPackContext_ValidRuneBoundaries(t *testing.T) {
	// Create content with multi-byte UTF-8 runes (Portuguese accents and symbols)
	accentedSnippet := "func ValidarAçãoDeCréditoBancário() { /* 💳 autorização prévia */ }"

	cands := []*Candidate{
		{
			Document: &Document{
				RelativePath: "pkg/financeiro/autorização.go",
				Symbols: []Symbol{
					{Name: "ValidarAçãoDeCréditoBancário", Kind: KindFunction, RelativePath: "pkg/financeiro/autorização.go", StartLine: 1, EndLine: 5},
				},
			},
			Score:          15.0,
			MatchedTerms:   []string{"autorização", "crédito"},
			MatchedSymbols: []Symbol{{Name: "ValidarAçãoDeCréditoBancário", Kind: KindFunction, RelativePath: "pkg/financeiro/autorização.go", StartLine: 1, EndLine: 5}},
			Snippets: []Snippet{
				{
					RelativePath: "pkg/financeiro/autorização.go",
					StartLine:    1,
					EndLine:      5,
					Content:      accentedSnippet,
				},
			},
		},
	}

	// Test every boundary cutoff from 1 to 500 bytes to ensure NO invalid rune boundary is ever emitted
	for b := 1; b <= 500; b++ {
		pack := PackContext(cands, 3, b)
		if pack != nil {
			raw := []byte(pack.FormattedText)
			if len(raw) > b {
				t.Fatalf("at cutoff %d: output bytes %d > limit %d", b, len(raw), b)
			}
			if !utf8.Valid(raw) {
				t.Fatalf("at cutoff %d: truncated text contains invalid UTF-8 rune boundary", b)
			}
		}
	}
}

func TestPackContext_StructuralSketchAndSecurityMarkers(t *testing.T) {
	doc := &Document{
		RelativePath: "internal/service/user.go",
		Symbols: []Symbol{
			{Name: "UserService", Kind: KindStruct, RelativePath: "internal/service/user.go", StartLine: 5, EndLine: 10},
			{Name: "CreateUser", Kind: KindFunction, RelativePath: "internal/service/user.go", StartLine: 12, EndLine: 25},
		},
	}

	cands := []*Candidate{
		{
			Document:       doc,
			Score:          12.0,
			MatchedTerms:   []string{"user", "service"},
			MatchedSymbols: doc.Symbols,
			Snippets: []Snippet{
				{
					RelativePath: "internal/service/user.go",
					StartLine:    12,
					EndLine:      25,
					Content:      "func CreateUser(name string) error {\n    return nil\n}",
				},
			},
		},
	}

	pack := PackContext(cands, 3, 3500)
	if pack == nil {
		t.Fatal("expected non-nil ContextPack")
	}

	// 1. Prompt-injection header & notice
	if !strings.Contains(pack.FormattedText, UntrustedContextStartMarker) {
		t.Errorf("missing UntrustedContextStartMarker in formatted text")
	}
	if !strings.Contains(pack.FormattedText, UntrustedContextSecurityNotice) {
		t.Errorf("missing UntrustedContextSecurityNotice in formatted text")
	}

	// 2. Structural overview
	if !strings.Contains(pack.FormattedText, "internal/service/user.go") {
		t.Errorf("missing file path in structural overview")
	}
	if !strings.Contains(pack.FormattedText, "UserService [struct L5-L10]") {
		t.Errorf("missing symbol UserService with kind and line numbers")
	}
	if !strings.Contains(pack.FormattedText, "CreateUser [func L12-L25]") {
		t.Errorf("missing symbol CreateUser with kind and line numbers")
	}

	// 3. Extractive snippets
	if !strings.Contains(pack.FormattedText, "#### Relevant Snippets") {
		t.Errorf("missing Relevant Snippets header")
	}
	if !strings.Contains(pack.FormattedText, "func CreateUser(name string) error {") {
		t.Errorf("missing verbatim snippet content")
	}

	// 4. Prompt-injection footer
	if !strings.Contains(pack.FormattedText, UntrustedContextEndMarker) {
		t.Errorf("missing UntrustedContextEndMarker in formatted text")
	}

	// 5. Pack metadata
	if pack.SymbolsCount != 2 {
		t.Errorf("pack.SymbolsCount = %d, want 2", pack.SymbolsCount)
	}
	if pack.SnippetsCount != 1 {
		t.Errorf("pack.SnippetsCount = %d, want 1", pack.SnippetsCount)
	}
	if len(pack.SelectedFiles) != 1 || pack.SelectedFiles[0] != "internal/service/user.go" {
		t.Errorf("pack.SelectedFiles = %v, want ['internal/service/user.go']", pack.SelectedFiles)
	}
}

func TestPackContext_EdgeCases(t *testing.T) {
	// 1. Nil candidates
	if pack := PackContext(nil, 3, 3500); pack != nil {
		t.Error("expected nil pack for nil candidates")
	}

	// 2. Empty candidates
	if pack := PackContext([]*Candidate{}, 3, 3500); pack != nil {
		t.Error("expected nil pack for empty candidates")
	}

	// 3. Zero or negative maxBytes
	cands := []*Candidate{{Document: &Document{RelativePath: "a.go"}}}
	if pack := PackContext(cands, 3, 0); pack != nil {
		t.Error("expected nil pack for maxBytes=0")
	}
	if pack := PackContext(cands, 3, -10); pack != nil {
		t.Error("expected nil pack for negative maxBytes")
	}
}
