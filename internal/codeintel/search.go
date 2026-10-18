// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Search executes BM25 search across the index for the given query tokens.
// Returns scored Candidates sorted deterministically.
func Search(idx *InvertedIndex, queryTokens []string) []*Candidate {
	return RankCandidates(idx, queryTokens, "")
}

// ExtractSnippets extracts literal source lines around matched symbols or terms for top candidates.
// It enforces the maximum snippet limit (default 4) and reads directly from disk without altering the index.
func ExtractSnippets(repoRoot string, candidates []*Candidate, maxSnippets int) {
	if maxSnippets <= 0 {
		maxSnippets = MaxTopSnippets
	}

	snippetsCount := 0

	for _, cand := range candidates {
		if snippetsCount >= maxSnippets {
			break
		}

		fullPath := filepath.Join(repoRoot, cand.Document.RelativePath)
		file, err := os.Open(fullPath)
		if err != nil {
			continue
		}

		// Read lines
		var lines []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		file.Close()

		if len(lines) == 0 {
			continue
		}

		// Extract snippets for matched symbols first
		for _, sym := range cand.MatchedSymbols {
			if snippetsCount >= maxSnippets {
				break
			}

			start := sym.StartLine - 1
			if start < 0 {
				start = 0
			}
			end := sym.EndLine
			if end > len(lines) {
				end = len(lines)
			}
			// Limit single snippet to 25 contiguous lines
			if end-start > 25 {
				end = start + 25
			}

			snippetText := strings.Join(lines[start:end], "\n")
			cand.Snippets = append(cand.Snippets, Snippet{
				RelativePath: cand.Document.RelativePath,
				StartLine:    start + 1,
				EndLine:      end,
				Content:      snippetText,
			})
			snippetsCount++
		}
	}
}
