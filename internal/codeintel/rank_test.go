// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"os"
	"path/filepath"
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

func TestParseGoAST_CallsAndImports(t *testing.T) {
	tempDir := t.TempDir()
	goSrc := `package service

import (
	"fmt"
	"net/http"
	"strings"
)

func ProcessOrder(id string) error {
	n := len(id)
	_ = append([]int{}, n)
	_ = make(map[string]int)
	_ = fmt.Sprintf("order-%s", id)
	items := FetchItems(id)
	return ValidateCart(items)
}

func FetchItems(id string) []string {
	return []string{strings.ToLower(id)}
}

func ValidateCart(items []string) error {
	return nil
}
`
	filePath := filepath.Join(tempDir, "service.go")
	if err := os.WriteFile(filePath, []byte(goSrc), 0644); err != nil {
		t.Fatal(err)
	}

	doc, err := ParseFile(tempDir, "service.go")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// 1. Verify Document.Imports
	wantImports := map[string]bool{
		"fmt":      true,
		"net/http": true,
		"strings":  true,
	}
	if len(doc.Imports) != len(wantImports) {
		t.Errorf("len(doc.Imports) = %d, want %d: %v", len(doc.Imports), len(wantImports), doc.Imports)
	}
	for _, imp := range doc.Imports {
		if !wantImports[imp] {
			t.Errorf("unexpected import in doc.Imports: %q", imp)
		}
	}

	// 2. Verify Symbol.Calls and exclusion of Go builtins (len, append, make)
	var processOrderSym *Symbol
	for _, sym := range doc.Symbols {
		if sym.Name == "ProcessOrder" {
			processOrderSym = &sym
			break
		}
	}
	if processOrderSym == nil {
		t.Fatal("symbol ProcessOrder not found")
	}

	wantCalls := map[string]bool{
		"Sprintf":      true,
		"FetchItems":   true,
		"ValidateCart": true,
	}
	for _, call := range processOrderSym.Calls {
		if call == "len" || call == "append" || call == "make" {
			t.Errorf("builtin function %q leaked into Symbol.Calls: %v", call, processOrderSym.Calls)
		}
		delete(wantCalls, call)
	}
	if len(wantCalls) > 0 {
		t.Errorf("missing expected calls in ProcessOrder: %v (got %v)", wantCalls, processOrderSym.Calls)
	}
}

func TestRankCandidates_ProductionFirstVsTestFiles(t *testing.T) {
	idx := NewInvertedIndex()

	docProd := &Document{
		RelativePath: "pkg/payment/service.go",
		TotalTerms:   100,
		TermFreqs:    map[string]int{"process": 5, "payment": 5},
		Symbols: []Symbol{
			{Name: "ProcessPayment", Kind: KindFunction, RelativePath: "pkg/payment/service.go", StartLine: 10, EndLine: 30},
		},
	}

	docTest := &Document{
		RelativePath: "pkg/payment/service_test.go",
		TotalTerms:   100,
		TermFreqs:    map[string]int{"process": 5, "payment": 5},
		Symbols: []Symbol{
			{Name: "ProcessPayment", Kind: KindFunction, RelativePath: "pkg/payment/service_test.go", StartLine: 10, EndLine: 30},
		},
	}

	idx.AddDocument(docProd)
	idx.AddDocument(docTest)
	idx.RebuildPostings()

	// 1. Query for "ProcessPayment" (non-test query): service.go should rank #1
	prodQuery := Tokenize("ProcessPayment")
	cands1 := RankCandidates(idx, prodQuery, "ProcessPayment")
	if len(cands1) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(cands1))
	}
	if cands1[0].Document.RelativePath != "pkg/payment/service.go" {
		t.Errorf("query 'ProcessPayment': expected top candidate 'pkg/payment/service.go', got %q (prod score: %f vs test score: %f)",
			cands1[0].Document.RelativePath, cands1[0].Score, cands1[1].Score)
	}

	// 2. Query for "test ProcessPayment benchmark" (test query): service_test.go should rank #1
	testQuery := Tokenize("test ProcessPayment benchmark")
	cands2 := RankCandidates(idx, testQuery, "test ProcessPayment benchmark")
	if len(cands2) != 2 {
		t.Fatalf("expected 2 candidates for test query, got %d", len(cands2))
	}
	if cands2[0].Document.RelativePath != "pkg/payment/service_test.go" {
		t.Errorf("query 'test ProcessPayment benchmark': expected top candidate 'pkg/payment/service_test.go', got %q (test score: %f vs prod score: %f)",
			cands2[0].Document.RelativePath, cands2[0].Score, cands2[1].Score)
	}
}

func TestRankCandidates_CallGraphCoReferenceBoost(t *testing.T) {
	idx := NewInvertedIndex()

	// Top-1 candidate for "HandleCheckout"
	docHandler := &Document{
		RelativePath: "pkg/checkout/handler.go",
		TotalTerms:   80,
		TermFreqs:    map[string]int{"checkout": 10, "handle": 8},
		Symbols: []Symbol{
			{
				Name:         "HandleCheckout",
				Kind:         KindFunction,
				RelativePath: "pkg/checkout/handler.go",
				StartLine:    15,
				EndLine:      45,
				Calls:        []string{"ValidateCart"},
			},
		},
	}

	// Callee candidate defining ValidateCart called by HandleCheckout
	docCart := &Document{
		RelativePath: "pkg/cart/cart.go",
		TotalTerms:   80,
		TermFreqs:    map[string]int{"checkout": 2},
		Symbols: []Symbol{
			{
				Name:         "ValidateCart",
				Kind:         KindFunction,
				RelativePath: "pkg/cart/cart.go",
				StartLine:    20,
				EndLine:      40,
			},
		},
	}

	// Unrelated candidate with identical base frequency on "checkout"
	docUnrelated := &Document{
		RelativePath: "pkg/order/summary.go",
		TotalTerms:   80,
		TermFreqs:    map[string]int{"checkout": 2},
		Symbols: []Symbol{
			{
				Name:         "ShowSummary",
				Kind:         KindFunction,
				RelativePath: "pkg/order/summary.go",
				StartLine:    10,
				EndLine:      25,
			},
		},
	}

	idx.AddDocument(docHandler)
	idx.AddDocument(docCart)
	idx.AddDocument(docUnrelated)
	idx.RebuildPostings()

	cands := RankCandidates(idx, Tokenize("HandleCheckout"), "HandleCheckout")
	if len(cands) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(cands))
	}

	// docHandler is top-1
	if cands[0].Document.RelativePath != "pkg/checkout/handler.go" {
		t.Fatalf("expected top-1 to be handler.go, got %s", cands[0].Document.RelativePath)
	}

	// docCart must receive CallGraphBoost (+2.0) and rank above docUnrelated
	var cartCandidate *Candidate
	var unrelatedCandidate *Candidate
	for _, c := range cands {
		if c.Document.RelativePath == "pkg/cart/cart.go" {
			cartCandidate = c
		} else if c.Document.RelativePath == "pkg/order/summary.go" {
			unrelatedCandidate = c
		}
	}

	if cartCandidate == nil || unrelatedCandidate == nil {
		t.Fatal("expected both cart.go and summary.go in candidates")
	}

	// Verify cartCandidate has CallGraphBoost (+2.0) compared to identical base docUnrelated
	scoreDiff := cartCandidate.Score - unrelatedCandidate.Score
	if scoreDiff < 1.99 || scoreDiff > 2.01 {
		t.Errorf("expected score difference of ~2.0 between boosted cart.go and summary.go, got %f (cart: %f, summary: %f)",
			scoreDiff, cartCandidate.Score, unrelatedCandidate.Score)
	}

	// Verify cart.go includes ValidateCart in its MatchedSymbols
	hasValidateCart := false
	for _, sym := range cartCandidate.MatchedSymbols {
		if sym.Name == "ValidateCart" {
			hasValidateCart = true
			break
		}
	}
	if !hasValidateCart {
		t.Errorf("cart.go MatchedSymbols does not contain ValidateCart: %v", cartCandidate.MatchedSymbols)
	}
}
