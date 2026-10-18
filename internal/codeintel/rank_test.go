// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"testing"
	"time"
)

func TestRankCandidatesFieldWeightsAndBonuses(t *testing.T) {
	idx := NewInvertedIndex()

	// Document 1: term "auth" in content only
	doc1 := &Document{
		RelativePath: "pkg/content/worker.go",
		TotalTerms:   100,
		TermFreqs:    map[string]int{"auth": 2, "worker": 10},
		Symbols:      []Symbol{{Name: "RunWorker", Kind: KindFunction, StartLine: 1, EndLine: 10}},
	}

	// Document 2: term "auth" in filename and content
	doc2 := &Document{
		RelativePath: "pkg/auth/client.go",
		TotalTerms:   100,
		TermFreqs:    map[string]int{"auth": 2, "client": 10},
		Symbols:      []Symbol{{Name: "NewClient", Kind: KindFunction, StartLine: 1, EndLine: 10}},
	}

	// Document 3: term "auth" in symbol name, filename, and exact match
	doc3 := &Document{
		RelativePath: "pkg/auth/authenticator.go",
		TotalTerms:   100,
		TermFreqs:    map[string]int{"auth": 2, "authenticator": 5},
		Symbols: []Symbol{
			{Name: "Authenticate", Kind: KindFunction, RelativePath: "pkg/auth/authenticator.go", StartLine: 10, EndLine: 20},
		},
	}

	idx.Documents[doc1.RelativePath] = doc1
	idx.Documents[doc2.RelativePath] = doc2
	idx.Documents[doc3.RelativePath] = doc3
	idx.RebuildPostings()

	candidates := RankCandidates(idx, []string{"auth"}, "Authenticate")
	if len(candidates) != 3 {
		t.Fatalf("Expected 3 candidates, got %d", len(candidates))
	}

	// doc3 should rank first because of symbol match and exact symbol bonus
	if candidates[0].Document.RelativePath != doc3.RelativePath {
		t.Errorf("Top candidate = %s, want %s (Score: %f vs %f)",
			candidates[0].Document.RelativePath, doc3.RelativePath, candidates[0].Score, candidates[1].Score)
	}

	// doc2 should rank second because of filename match weight
	if candidates[1].Document.RelativePath != doc2.RelativePath {
		t.Errorf("Second candidate = %s, want %s (Score: %f vs %f)",
			candidates[1].Document.RelativePath, doc2.RelativePath, candidates[1].Score, candidates[2].Score)
	}

	// doc1 should rank third
	if candidates[2].Document.RelativePath != doc1.RelativePath {
		t.Errorf("Third candidate = %s, want %s", candidates[2].Document.RelativePath, doc1.RelativePath)
	}
}

func TestDeterministicTieBreaking(t *testing.T) {
	// Two candidates with identical score
	docA := &Document{
		RelativePath: "a/service.go",
		TotalTerms:   50,
		TermFreqs:    map[string]int{"service": 2},
	}
	docB := &Document{
		RelativePath: "b/service.go",
		TotalTerms:   50,
		TermFreqs:    map[string]int{"service": 2},
	}

	cands := []*Candidate{
		{Document: docB, Score: 10.0},
		{Document: docA, Score: 10.0},
	}

	SortCandidates(cands)

	if cands[0].Document.RelativePath != "a/service.go" {
		t.Errorf("Tie breaking failed: first doc = %s, want a/service.go", cands[0].Document.RelativePath)
	}
}

func TestPruneTopK(t *testing.T) {
	cands := make([]*Candidate, 5)
	for i := 0; i < 5; i++ {
		doc := &Document{
			RelativePath: string(rune('a'+i)) + ".go",
			ModTime:      time.Now(),
		}
		var syms []Symbol
		for j := 0; j < 4; j++ {
			syms = append(syms, Symbol{Name: "Sym", StartLine: j + 1})
		}
		cands[i] = &Candidate{
			Document:       doc,
			Score:          float64(10 - i),
			MatchedSymbols: syms,
		}
	}

	pruned := PruneTopK(cands, 3, 8)
	if len(pruned) != 3 {
		t.Fatalf("Expected 3 candidates after prune, got %d", len(pruned))
	}

	totalSymbols := 0
	for _, c := range pruned {
		totalSymbols += len(c.MatchedSymbols)
	}

	if totalSymbols > 8 {
		t.Errorf("Total symbols = %d, want <= 8", totalSymbols)
	}
}

func TestBM25DeterminismAndRepeatability(t *testing.T) {
	idx := NewInvertedIndex()

	doc1 := &Document{
		RelativePath: "pkg/payment/pix.go",
		TotalTerms:   50,
		TermFreqs:    map[string]int{"payment": 5, "pix": 4, "process": 2},
		Symbols: []Symbol{
			{Name: "ProcessPIXPayment", Kind: KindFunction, RelativePath: "pkg/payment/pix.go", StartLine: 10, EndLine: 25},
			{Name: "ValidatePIXKey", Kind: KindFunction, RelativePath: "pkg/payment/pix.go", StartLine: 30, EndLine: 40},
		},
	}

	doc2 := &Document{
		RelativePath: "pkg/payment/boleto.go",
		TotalTerms:   50,
		TermFreqs:    map[string]int{"payment": 5, "boleto": 4, "process": 2},
		Symbols: []Symbol{
			{Name: "ProcessBoletoPayment", Kind: KindFunction, RelativePath: "pkg/payment/boleto.go", StartLine: 10, EndLine: 25},
		},
	}

	doc3 := &Document{
		RelativePath: "pkg/payment/card.go",
		TotalTerms:   50,
		TermFreqs:    map[string]int{"payment": 5, "card": 4, "process": 2},
		Symbols: []Symbol{
			{Name: "ProcessCardPayment", Kind: KindFunction, RelativePath: "pkg/payment/card.go", StartLine: 10, EndLine: 25},
		},
	}

	idx.AddDocument(doc1)
	idx.AddDocument(doc2)
	idx.AddDocument(doc3)
	idx.RebuildPostings()

	query := []string{"process", "pix", "payment"}
	prompt := "How does ProcessPIXPayment work?"

	// Run 50 times to guarantee strict determinism across runs
	var baselinePaths []string
	for i := 0; i < 50; i++ {
		res := RankCandidates(idx, query, prompt)
		if len(res) == 0 {
			t.Fatal("expected results from RankCandidates")
		}
		if i == 0 {
			for _, r := range res {
				baselinePaths = append(baselinePaths, r.Document.RelativePath)
			}
			// Verify exact match bonus prioritizes pix.go
			if baselinePaths[0] != "pkg/payment/pix.go" {
				t.Fatalf("expected top result pkg/payment/pix.go, got %s", baselinePaths[0])
			}
		} else {
			for idxR, r := range res {
				if r.Document.RelativePath != baselinePaths[idxR] {
					t.Fatalf("non-deterministic ranking at iteration %d index %d: got %s, want %s",
						i, idxR, r.Document.RelativePath, baselinePaths[idxR])
				}
			}
		}
	}
}
