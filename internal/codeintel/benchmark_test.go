// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"fmt"
	"strings"
	"testing"
)

// buildBenchmarkCorpus generates a synthetic codebase of N documents for benchmarking.
func buildBenchmarkCorpus(numDocs int) (map[string]string, *InvertedIndex) {
	rawFiles := make(map[string]string, numDocs)
	idx := NewInvertedIndex()

	termsPool := []string{
		"payment", "gateway", "processor", "transaction", "reconciliation",
		"customer", "account", "invoice", "balance", "ledger",
		"auth", "token", "session", "user", "permission",
		"webhook", "event", "dispatch", "handler", "receiver",
	}

	for i := 0; i < numDocs; i++ {
		relPath := fmt.Sprintf("pkg/domain_%d/service_%d.go", i/10, i)
		var contentBuilder strings.Builder
		contentBuilder.WriteString(fmt.Sprintf("package domain_%d\n\n", i/10))

		// Add some deterministic terms based on index
		docTerm1 := termsPool[i%len(termsPool)]
		docTerm2 := termsPool[(i+3)%len(termsPool)]
		docTerm3 := termsPool[(i+7)%len(termsPool)]

		contentBuilder.WriteString(fmt.Sprintf("// %s %s implementation\n", docTerm1, docTerm2))
		contentBuilder.WriteString(fmt.Sprintf("func Process%s%d() error {\n", strings.ToUpper(docTerm1[:1])+docTerm1[1:], i))
		contentBuilder.WriteString(fmt.Sprintf("    // handles %s and %s\n", docTerm2, docTerm3))
		contentBuilder.WriteString("    return nil\n}\n")

		content := contentBuilder.String()
		rawFiles[relPath] = content

		termFreqs, total := TermFrequencies(content)
		symbols := []Symbol{
			{
				Name:         fmt.Sprintf("Process%s%d", strings.ToUpper(docTerm1[:1])+docTerm1[1:], i),
				Kind:         KindFunction,
				RelativePath: relPath,
				StartLine:    3,
				EndLine:      6,
			},
		}

		doc := &Document{
			RelativePath: relPath,
			SHA256:       fmt.Sprintf("sha_%d", i),
			Size:         int64(len(content)),
			TermFreqs:    termFreqs,
			Symbols:      symbols,
			TotalTerms:   total,
		}
		idx.AddDocument(doc)
	}

	idx.RebuildPostings()
	return rawFiles, idx
}

// unindexedSearch simulates a linear brute-force scan across raw source files.
func unindexedSearch(rawFiles map[string]string, queryTokens []string) []string {
	var matchedFiles []string
	for path, content := range rawFiles {
		tokens := Tokenize(content)
		tokenSet := make(map[string]bool, len(tokens))
		for _, t := range tokens {
			tokenSet[t] = true
		}

		matches := 0
		for _, qt := range queryTokens {
			if tokenSet[qt] {
				matches++
			}
		}
		if matches > 0 {
			matchedFiles = append(matchedFiles, path)
		}
	}
	return matchedFiles
}

func BenchmarkUnindexedSearch(b *testing.B) {
	rawFiles, _ := buildBenchmarkCorpus(200)
	query := []string{"payment", "gateway"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := unindexedSearch(rawFiles, query)
		if len(results) == 0 {
			b.Fatal("expected search results")
		}
	}
}

func BenchmarkIndexedRetrieval(b *testing.B) {
	_, idx := buildBenchmarkCorpus(200)
	query := []string{"payment", "gateway"}
	prompt := "How does payment gateway process transactions?"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := RankCandidates(idx, query, prompt)
		if len(results) == 0 {
			b.Fatal("expected search results")
		}
	}
}

func BenchmarkContextPackager(b *testing.B) {
	_, idx := buildBenchmarkCorpus(50)
	query := []string{"payment", "gateway"}
	candidates := RankCandidates(idx, query, "payment gateway")
	top := PruneTopK(candidates, 3, 8)

	for _, c := range top {
		c.Snippets = []Snippet{
			{
				RelativePath: c.Document.RelativePath,
				StartLine:    1,
				EndLine:      6,
				Content:      "func ProcessPayment() error {\n    return nil\n}",
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pack := PackContext(top, 3, 3500)
		if pack == nil {
			b.Fatal("expected pack context")
		}
	}
}

func TestBenchmarkComparison_Integrity(t *testing.T) {
	rawFiles, idx := buildBenchmarkCorpus(100)
	query := []string{"transaction", "invoice"}
	prompt := "Inspect transaction invoice service"

	// 1. Unindexed search
	unindexedMatches := unindexedSearch(rawFiles, query)

	// 2. Indexed retrieval
	indexedCandidates := RankCandidates(idx, query, prompt)

	if len(unindexedMatches) == 0 {
		t.Fatal("expected unindexed matches")
	}
	if len(indexedCandidates) == 0 {
		t.Fatal("expected indexed candidates")
	}

	// 3. Packaging
	top := PruneTopK(indexedCandidates, 3, 8)
	pack := PackContext(top, 3, 3500)
	if pack == nil {
		t.Fatal("expected valid context pack")
	}

	if pack.OutputBytes > 3500 {
		t.Errorf("pack output bytes %d exceeds 3500", pack.OutputBytes)
	}
	if len(pack.SelectedFiles) == 0 {
		t.Error("expected selected files in pack")
	}
}
