// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"math"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// BM25 parameters
	DefaultK1 = 1.2
	DefaultB  = 0.75

	// Field Weights
	SymbolFieldWeight   = 5.0
	FilenameFieldWeight = 3.0
	ContentFieldWeight  = 1.0

	// Exact match bonuses
	ExactSymbolMatchBonus   = 5.0
	ExactFilenameMatchBonus = 3.0

	// Call-Graph Boost
	CallGraphBoost = 2.0

	// Top-K Selection Limits
	MaxTopFiles    = 3
	MaxTopSymbols  = 8
	MaxTopSnippets = 4
)

// RankCandidates applies field-weighted BM25 scoring, exact match bonuses,
// and deterministic sorting to retrieval candidates.
func RankCandidates(idx *InvertedIndex, queryTokens []string, rawPrompt string) []*Candidate {
	if idx == nil || len(idx.Documents) == 0 || len(queryTokens) == 0 {
		return nil
	}

	N := float64(len(idx.Documents))
	avgDocLen := idx.AvgDocLen
	if avgDocLen <= 0 {
		avgDocLen = 1.0
	}

	// Deduplicate query tokens while preserving order
	uniqueTerms := make([]string, 0, len(queryTokens))
	seenTerms := make(map[string]bool, len(queryTokens))
	for _, t := range queryTokens {
		if !seenTerms[t] {
			seenTerms[t] = true
			uniqueTerms = append(uniqueTerms, t)
		}
	}

	scores := make(map[string]float64)
	matchedTermsMap := make(map[string][]string)

	for _, term := range uniqueTerms {
		postings, exists := idx.Postings[term]
		if !exists || len(postings) == 0 {
			continue
		}

		nq := float64(len(postings))
		idf := math.Log((N-nq+0.5)/(nq+0.5) + 1.0)
		if idf < 0.1 {
			idf = 0.1 // clamp negative or near-zero idf
		}

		for _, post := range postings {
			doc, ok := idx.Documents[post.DocPath]
			if !ok || doc == nil {
				continue
			}

			// Calculate field-weighted term frequency
			weightedTF := computeWeightedTF(doc, term, post.Frequency)
			docLen := float64(doc.TotalTerms)
			if docLen <= 0 {
				docLen = 1.0
			}

			// BM25 calculation
			num := weightedTF * (DefaultK1 + 1.0)
			denom := weightedTF + DefaultK1*(1.0-DefaultB+DefaultB*(docLen/avgDocLen))
			bm25Term := idf * (num / denom)

			scores[post.DocPath] += bm25Term
			matchedTermsMap[post.DocPath] = append(matchedTermsMap[post.DocPath], term)
		}
	}

	if len(scores) == 0 {
		return nil
	}

	var candidates []*Candidate
	for docPath, score := range scores {
		doc := idx.Documents[docPath]
		matchedSymbols := findDocMatchedSymbols(doc, uniqueTerms)

		// Apply exact match bonuses
		score += computeExactMatchBonus(doc, matchedSymbols, uniqueTerms, rawPrompt)

		// Production-First vs Test Ranking
		if isTestFile(doc.RelativePath) {
			if !isTestQuery(uniqueTerms) {
				score *= 0.65
			} else {
				score += 2.0
			}
		}

		candidates = append(candidates, &Candidate{
			Document:       doc,
			Score:          score,
			MatchedTerms:   matchedTermsMap[docPath],
			MatchedSymbols: matchedSymbols,
		})
	}

	// Deterministic sorting: Score DESC, RelativePath ASC
	SortCandidates(candidates)

	// Call-Graph Co-Reference Boost (CallGraphBoost = 2.0)
	if len(candidates) > 1 && candidates[0] != nil {
		top := candidates[0]
		topTargetSymbols := top.MatchedSymbols
		if len(topTargetSymbols) == 0 && top.Document != nil {
			topTargetSymbols = top.Document.Symbols
		}

		topCalled := make(map[string]bool)
		for _, sym := range topTargetSymbols {
			for _, call := range sym.Calls {
				topCalled[call] = true
			}
		}

		topMatchedNames := make(map[string]bool)
		for _, sym := range topTargetSymbols {
			topMatchedNames[sym.Name] = true
			if dotIdx := strings.LastIndex(sym.Name, "."); dotIdx != -1 {
				topMatchedNames[sym.Name[dotIdx+1:]] = true
			}
		}

		for _, c := range candidates[1:] {
			if c == nil || c.Document == nil {
				continue
			}
			boosted := false
			for _, sym := range c.Document.Symbols {
				symName := sym.Name
				methodPart := symName
				if dotIdx := strings.LastIndex(symName, "."); dotIdx != -1 {
					methodPart = symName[dotIdx+1:]
				}

				calledByTop := topCalled[symName] || topCalled[methodPart]
				callsTop := false
				for _, call := range sym.Calls {
					if topMatchedNames[call] {
						callsTop = true
						break
					}
				}

				if calledByTop || callsTop {
					if !boosted {
						c.Score += CallGraphBoost
						boosted = true
					}
					alreadyPresent := false
					for _, existing := range c.MatchedSymbols {
						if existing.Name == sym.Name && existing.StartLine == sym.StartLine {
							alreadyPresent = true
							break
						}
					}
					if !alreadyPresent {
						c.MatchedSymbols = append(c.MatchedSymbols, sym)
					}
				}
			}
		}

		// Re-run SortCandidates after applying call-graph boosts
		SortCandidates(candidates)
	}

	return candidates
}

// computeWeightedTF weights term matches according to field importance:
// Symbol (5.0), Filename (3.0), Content (1.0).
func computeWeightedTF(doc *Document, term string, rawFreq int) float64 {
	hasSymbolMatch := false
	for _, sym := range doc.Symbols {
		for _, st := range Tokenize(sym.Name) {
			if st == term {
				hasSymbolMatch = true
				break
			}
		}
		if hasSymbolMatch {
			break
		}
	}

	hasFilenameMatch := false
	for _, pt := range Tokenize(doc.RelativePath) {
		if pt == term {
			hasFilenameMatch = true
			break
		}
	}

	var weight float64 = ContentFieldWeight
	if hasSymbolMatch {
		weight += SymbolFieldWeight
	}
	if hasFilenameMatch {
		weight += FilenameFieldWeight
	}

	return float64(rawFreq) * weight
}

// computeExactMatchBonus awards additional score when query terms exactly match
// an entire symbol identifier or filename basename.
func computeExactMatchBonus(doc *Document, symbols []Symbol, queryTerms []string, rawPrompt string) float64 {
	var bonus float64
	promptLower := strings.ToLower(rawPrompt)
	base := strings.ToLower(filepath.Base(doc.RelativePath))

	// Exact filename bonus
	for _, t := range queryTerms {
		if t == base || strings.TrimSuffix(base, filepath.Ext(base)) == t {
			bonus += ExactFilenameMatchBonus
			break
		}
	}

	// Exact symbol identifier bonus
	for _, sym := range symbols {
		symLower := strings.ToLower(sym.Name)
		for _, t := range queryTerms {
			if symLower == t {
				bonus += ExactSymbolMatchBonus
				break
			}
		}
		if strings.Contains(promptLower, symLower) {
			bonus += ExactSymbolMatchBonus
		}
	}

	return bonus
}

// isTestFile returns true if relPath corresponds to a test or spec file.
func isTestFile(relPath string) bool {
	p := strings.ToLower(filepath.ToSlash(relPath))
	base := filepath.Base(p)
	if strings.HasSuffix(p, "_test.go") ||
		strings.HasSuffix(p, ".test.ts") ||
		strings.HasSuffix(p, ".spec.ts") ||
		strings.HasSuffix(p, ".test.js") ||
		strings.HasSuffix(p, ".spec.js") ||
		strings.HasPrefix(base, "test_") {
		return true
	}
	return false
}

// isTestQuery returns true if any query term indicates test or benchmark intent.
func isTestQuery(queryTerms []string) bool {
	for _, term := range queryTerms {
		switch strings.ToLower(term) {
		case "test", "tests", "testing", "benchmark", "benchmarks", "bench":
			return true
		}
	}
	return false
}

// findDocMatchedSymbols extracts symbols matching query terms and sorts them deterministically (StartLine ASC, Name ASC).
func findDocMatchedSymbols(doc *Document, queryTerms []string) []Symbol {
	if doc == nil || len(doc.Symbols) == 0 {
		return nil
	}

	var matched []Symbol
	for _, sym := range doc.Symbols {
		symTokens := Tokenize(sym.Name)
		matchedAny := false
		for _, st := range symTokens {
			for _, qt := range queryTerms {
				if st == qt {
					matchedAny = true
					break
				}
			}
			if matchedAny {
				break
			}
		}
		if matchedAny {
			matched = append(matched, sym)
		}
	}

	// Deterministic symbol sort: StartLine ASC, Name ASC
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].StartLine != matched[j].StartLine {
			return matched[i].StartLine < matched[j].StartLine
		}
		return matched[i].Name < matched[j].Name
	})

	return matched
}

// SortCandidates sorts candidates deterministically: Score DESC, RelativePath ASC.
func SortCandidates(candidates []*Candidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if math.Abs(candidates[i].Score-candidates[j].Score) > 1e-6 {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].Document.RelativePath < candidates[j].Document.RelativePath
	})
}

// PruneTopK limits candidates to max 3 files, 8 symbols, and ensures deterministic selection.
func PruneTopK(candidates []*Candidate, maxFiles, maxSymbols int) []*Candidate {
	if len(candidates) == 0 {
		return nil
	}

	if maxFiles <= 0 {
		maxFiles = MaxTopFiles
	}
	if maxSymbols <= 0 {
		maxSymbols = MaxTopSymbols
	}

	limit := len(candidates)
	if limit > maxFiles {
		limit = maxFiles
	}

	selected := make([]*Candidate, limit)
	totalSymbols := 0

	for i := 0; i < limit; i++ {
		c := candidates[i]
		clone := &Candidate{
			Document:     c.Document,
			Score:        c.Score,
			MatchedTerms: c.MatchedTerms,
			Snippets:     c.Snippets,
			ImpactRadius: c.ImpactRadius,
		}

		for _, sym := range c.MatchedSymbols {
			if totalSymbols < maxSymbols {
				clone.MatchedSymbols = append(clone.MatchedSymbols, sym)
				totalSymbols++
			}
		}

		selected[i] = clone
	}

	return selected
}
