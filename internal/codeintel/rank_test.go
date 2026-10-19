// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
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

func TestMultiLanguageCallAndImportExtraction(t *testing.T) {
	tempDir := t.TempDir()

	// 1. TypeScript snippet with imports and calls
	tsCode := `import { AuthClient } from "./client";
import axios from "axios";

export class TokenManager {
	async refreshToken(token: string): Promise<boolean> {
		const valid = validateToken(token);
		if (!valid) {
			notifyExpiry(token);
			return false;
		}
		emitTelemetry("refreshed");
		return true;
	}
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "token_manager.ts"), []byte(tsCode), 0644); err != nil {
		t.Fatal(err)
	}

	docTS, err := ParseFile(tempDir, "token_manager.ts")
	if err != nil {
		t.Fatalf("ParseFile(token_manager.ts) failed: %v", err)
	}

	// Verify TS imports
	if len(docTS.Imports) == 0 {
		t.Errorf("Expected docTS.Imports to be non-empty, got 0")
	}
	hasClientImport := false
	for _, imp := range docTS.Imports {
		if strings.Contains(imp, "./client") || imp == "axios" {
			hasClientImport = true
			break
		}
	}
	if !hasClientImport {
		t.Errorf("Expected './client' or 'axios' in docTS.Imports, got: %v", docTS.Imports)
	}

	// Verify TS calls
	var tsMethod *Symbol
	for _, s := range docTS.Symbols {
		if s.Name == "TokenManager.refreshToken" {
			tsMethod = &s
			break
		}
	}
	if tsMethod == nil {
		t.Fatalf("Symbol TokenManager.refreshToken not found in %v", docTS.Symbols)
	}
	hasValidateToken := false
	for _, c := range tsMethod.Calls {
		if c == "validateToken" {
			hasValidateToken = true
			break
		}
	}
	if !hasValidateToken {
		t.Errorf("Expected validateToken in tsMethod.Calls, got: %v", tsMethod.Calls)
	}

	// 2. Python snippet with imports and calls
	pyCode := `from payment.gateway import charge_card
import logging

def process_checkout(order_id: str, amount: int):
    charge_card(amount)
    send_receipt(order_id)
`
	if err := os.WriteFile(filepath.Join(tempDir, "checkout.py"), []byte(pyCode), 0644); err != nil {
		t.Fatal(err)
	}

	docPy, err := ParseFile(tempDir, "checkout.py")
	if err != nil {
		t.Fatalf("ParseFile(checkout.py) failed: %v", err)
	}

	// Verify Python imports
	if len(docPy.Imports) == 0 {
		t.Errorf("Expected docPy.Imports to be non-empty, got 0")
	}
	hasPyImport := false
	for _, imp := range docPy.Imports {
		if strings.Contains(imp, "payment.gateway") || strings.Contains(imp, "logging") {
			hasPyImport = true
			break
		}
	}
	if !hasPyImport {
		t.Errorf("Expected payment.gateway or logging in docPy.Imports, got: %v", docPy.Imports)
	}

	// Verify Python calls
	var pyFunc *Symbol
	for _, s := range docPy.Symbols {
		if s.Name == "process_checkout" {
			pyFunc = &s
			break
		}
	}
	if pyFunc == nil {
		t.Fatalf("Symbol process_checkout not found in %v", docPy.Symbols)
	}
	hasChargeCard := false
	for _, c := range pyFunc.Calls {
		if c == "charge_card" {
			hasChargeCard = true
			break
		}
	}
	if !hasChargeCard {
		t.Errorf("Expected charge_card in pyFunc.Calls, got: %v", pyFunc.Calls)
	}

	// 3. Rust snippet with imports and calls
	rsCode := `use std::sync::Arc;
use crate::database::Pool;

pub fn execute_transaction(pool: &Pool) -> bool {
    begin_tx(pool);
    commit_tx();
    true
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "tx.rs"), []byte(rsCode), 0644); err != nil {
		t.Fatal(err)
	}

	docRS, err := ParseFile(tempDir, "tx.rs")
	if err != nil {
		t.Fatalf("ParseFile(tx.rs) failed: %v", err)
	}

	// Verify Rust imports
	if len(docRS.Imports) == 0 {
		t.Errorf("Expected docRS.Imports to be non-empty, got 0")
	}
	hasRSImport := false
	for _, imp := range docRS.Imports {
		if strings.Contains(imp, "std::sync::Arc") || strings.Contains(imp, "crate::database::Pool") {
			hasRSImport = true
			break
		}
	}
	if !hasRSImport {
		t.Errorf("Expected std::sync::Arc in docRS.Imports, got: %v", docRS.Imports)
	}

	// Verify Rust calls
	var rsFunc *Symbol
	for _, s := range docRS.Symbols {
		if s.Name == "execute_transaction" {
			rsFunc = &s
			break
		}
	}
	if rsFunc == nil {
		t.Fatalf("Symbol execute_transaction not found in %v", docRS.Symbols)
	}
	hasBeginTx := false
	for _, c := range rsFunc.Calls {
		if c == "begin_tx" {
			hasBeginTx = true
			break
		}
	}
	if !hasBeginTx {
		t.Errorf("Expected begin_tx in rsFunc.Calls, got: %v", rsFunc.Calls)
	}
}

func TestReverseCallGraph_ComputationAndCircularDependencyResistance(t *testing.T) {
	idx := NewInvertedIndex()

	// Circular call graph:
	// File A: defines FuncA, calls FuncB
	docA := &Document{
		RelativePath: "pkg/cycle/service_a.go",
		TotalTerms:   50,
		TermFreqs:    map[string]int{"cycle": 2},
		Symbols: []Symbol{
			{Name: "FuncA", Kind: KindFunction, RelativePath: "pkg/cycle/service_a.go", StartLine: 1, EndLine: 10, Calls: []string{"FuncB"}},
		},
	}

	// File B: defines FuncB, calls FuncA (circular with A)
	docB := &Document{
		RelativePath: "pkg/cycle/service_b.go",
		TotalTerms:   50,
		TermFreqs:    map[string]int{"cycle": 2},
		Symbols: []Symbol{
			{Name: "FuncB", Kind: KindFunction, RelativePath: "pkg/cycle/service_b.go", StartLine: 1, EndLine: 10, Calls: []string{"FuncA"}},
		},
	}

	// File C: defines FuncC, calls both FuncA and FuncB
	docC := &Document{
		RelativePath: "pkg/cycle/service_c.go",
		TotalTerms:   50,
		TermFreqs:    map[string]int{"cycle": 2},
		Symbols: []Symbol{
			{Name: "FuncC", Kind: KindFunction, RelativePath: "pkg/cycle/service_c.go", StartLine: 1, EndLine: 10, Calls: []string{"FuncA", "FuncB"}},
		},
	}

	idx.AddDocument(docA)
	idx.AddDocument(docB)
	idx.AddDocument(docC)
	idx.RebuildPostings()

	// Check reverse callers for FuncA
	callersA := idx.GetCallers("FuncA")
	if len(callersA) != 2 {
		t.Fatalf("Expected 2 callers for FuncA, got %d: %v", len(callersA), callersA)
	}
	// Callers must be deterministically sorted
	if callersA[0] != "pkg/cycle/service_b.go:FuncB" || callersA[1] != "pkg/cycle/service_c.go:FuncC" {
		t.Errorf("Unexpected callers for FuncA: %v", callersA)
	}

	// Check reverse callers for FuncB
	callersB := idx.GetCallers("FuncB")
	if len(callersB) != 2 {
		t.Fatalf("Expected 2 callers for FuncB, got %d: %v", len(callersB), callersB)
	}
	if callersB[0] != "pkg/cycle/service_a.go:FuncA" || callersB[1] != "pkg/cycle/service_c.go:FuncC" {
		t.Errorf("Unexpected callers for FuncB: %v", callersB)
	}

	// Check unknown symbol returns nil
	callersUnknown := idx.GetCallers("NonExistentFunc")
	if callersUnknown != nil {
		t.Errorf("Expected nil for NonExistentFunc, got: %v", callersUnknown)
	}
}

func TestPackContext_BudgetAdherenceWithImpactRadius(t *testing.T) {
	cands := []*Candidate{
		{
			Document: &Document{
				RelativePath: "pkg/auth/login.go",
				Symbols: []Symbol{
					{Name: "HandleLogin", Kind: KindFunction, RelativePath: "pkg/auth/login.go", StartLine: 10, EndLine: 50},
				},
			},
			Score:        20.0,
			MatchedTerms: []string{"auth", "login"},
			MatchedSymbols: []Symbol{
				{Name: "HandleLogin", Kind: KindFunction, RelativePath: "pkg/auth/login.go", StartLine: 10, EndLine: 50},
			},
			ImpactRadius: []string{
				"pkg/api/router.go:RegisterAuth",
				"pkg/gateway/proxy.go:ProxyAuth",
				"pkg/worker/audit.go:RecordLogin",
			},
			Snippets: []Snippet{
				{
					RelativePath: "pkg/auth/login.go",
					StartLine:    10,
					EndLine:      20,
					Content:      "func HandleLogin(w http.ResponseWriter, r *http.Request) {\n\t// Authenticate\n}\n",
				},
			},
		},
	}

	budgets := []int{100, 250, 400, 600, 1000, 3500}
	for _, maxBytes := range budgets {
		t.Run(fmt.Sprintf("Budget_%d", maxBytes), func(t *testing.T) {
			pack := PackContext(cands, 3, maxBytes)
			if pack == nil {
				t.Fatalf("PackContext returned nil for budget %d", maxBytes)
			}
			raw := []byte(pack.FormattedText)
			if len(raw) > maxBytes {
				t.Errorf("Output bytes %d strictly exceeds maxBytes %d", len(raw), maxBytes)
			}
			if !utf8.Valid(raw) {
				t.Errorf("Output is not valid UTF-8 for budget %d", maxBytes)
			}
			// When budget is comfortably large (>= 600), verify Impact Radius appears
			if maxBytes >= 600 {
				if !strings.Contains(pack.FormattedText, "Impact Radius:") {
					t.Errorf("Expected 'Impact Radius:' in pack text for budget %d, got:\n%s", maxBytes, pack.FormattedText)
				}
				if len(pack.ImpactRadius) == 0 {
					t.Errorf("Expected non-empty pack.ImpactRadius for budget %d", maxBytes)
				}
			}
		})
	}
}

