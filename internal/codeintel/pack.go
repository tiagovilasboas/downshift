// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// PackContext formats scored candidates and extracted snippets into a compact,
// structured context pack strictly within maxBytes UTF-8.
// It implements structural sketch + extractive selection with prompt-injection defenses
// and safe fallbacks to structural-only when the budget is constrained.
func PackContext(candidates []*Candidate, maxFiles, maxBytes int) *ContextPack {
	if len(candidates) == 0 || maxBytes <= 0 {
		return nil
	}

	limitFiles := maxFiles
	if limitFiles <= 0 || limitFiles > 3 {
		limitFiles = 3
	}

	topCands := candidates
	if len(topCands) > limitFiles {
		topCands = topCands[:limitFiles]
	}

	var selectedFiles []string
	var relevantTerms []string
	seenTerms := make(map[string]bool)
	symbolsCount := 0
	snippetsCount := 0

	// 1. Build Header
	var header strings.Builder
	header.WriteString(UntrustedContextStartMarker)
	header.WriteString("\n")
	header.WriteString(UntrustedContextSecurityNotice)
	header.WriteString("\n\n### Repository Code Intelligence\n")

	// 2. Build Structural Sketch
	var structSketch strings.Builder
	var allImpactRadius []string
	seenImpactGlobal := make(map[string]bool)

	for _, cand := range topCands {
		selectedFiles = append(selectedFiles, cand.Document.RelativePath)
		for _, t := range cand.MatchedTerms {
			if !seenTerms[t] {
				seenTerms[t] = true
				relevantTerms = append(relevantTerms, t)
			}
		}

		structSketch.WriteString(fmt.Sprintf("\n- File: `%s`", cand.Document.RelativePath))
		if len(cand.MatchedSymbols) > 0 {
			structSketch.WriteString(" (Symbols: ")
			var symStrs []string
			for _, sym := range cand.MatchedSymbols {
				symStrs = append(symStrs, fmt.Sprintf("%s [%s L%d-L%d]", sym.Name, sym.Kind, sym.StartLine, sym.EndLine))
				symbolsCount++
			}
			structSketch.WriteString(strings.Join(symStrs, ", "))
			structSketch.WriteString(")")
		}

		// Impact Radius: callers of matched symbols or document
		if len(cand.ImpactRadius) > 0 {
			callersToShow := cand.ImpactRadius
			if len(callersToShow) > 4 {
				callersToShow = callersToShow[:4]
			}
			structSketch.WriteString(fmt.Sprintf(" (Impact Radius: called by %s)", strings.Join(callersToShow, ", ")))
			for _, c := range callersToShow {
				if !seenImpactGlobal[c] {
					seenImpactGlobal[c] = true
					allImpactRadius = append(allImpactRadius, c)
				}
			}
		}
	}
	structSketch.WriteString("\n")

	// 3. Build Extractive Snippets (Grouped by file, literal contiguous snippets)
	var snippetsBlock strings.Builder
	hasSnippets := false
	for _, cand := range topCands {
		for _, snip := range cand.Snippets {
			if !hasSnippets {
				snippetsBlock.WriteString("\n#### Relevant Snippets\n")
				hasSnippets = true
			}
			snippetsBlock.WriteString(fmt.Sprintf("\n```\n// File: %s (Lines %d-%d)\n%s\n```\n",
				snip.RelativePath, snip.StartLine, snip.EndLine, snip.Content))
			snippetsCount++
		}
	}

	footer := "\n" + UntrustedContextEndMarker + "\n"

	// Attempt Full: Header + Structural + Snippets + Footer
	fullAttempt := header.String() + structSketch.String() + snippetsBlock.String() + footer
	if len([]byte(fullAttempt)) <= maxBytes {
		return &ContextPack{
			FormattedText: fullAttempt,
			OutputBytes:   len([]byte(fullAttempt)),
			SelectedFiles: selectedFiles,
			SymbolsCount:  symbolsCount,
			SnippetsCount: snippetsCount,
			RelevantTerms: relevantTerms,
			ImpactRadius:  allImpactRadius,
		}
	}

	// Fallback 1: Structural-only (Header + Structural + Footer) without snippets
	structAttempt := header.String() + structSketch.String() + footer
	if len([]byte(structAttempt)) <= maxBytes {
		return &ContextPack{
			FormattedText: structAttempt,
			OutputBytes:   len([]byte(structAttempt)),
			SelectedFiles: selectedFiles,
			SymbolsCount:  symbolsCount,
			SnippetsCount: 0, // snippets dropped due to budget
			RelevantTerms: relevantTerms,
			ImpactRadius:  allImpactRadius,
		}
	}

	// Fallback 2: Truncate structural safely to fit maxBytes
	truncated := truncateUTF8(structAttempt, maxBytes)
	return &ContextPack{
		FormattedText: truncated,
		OutputBytes:   len([]byte(truncated)),
		SelectedFiles: selectedFiles,
		SymbolsCount:  symbolsCount,
		SnippetsCount: 0,
		RelevantTerms: relevantTerms,
		ImpactRadius:  allImpactRadius,
	}
}

// truncateUTF8 truncates string s so that len([]byte(s)) <= maxBytes,
// respecting UTF-8 rune boundaries.
func truncateUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}

	b := []byte(s[:maxBytes])
	for len(b) > 0 && !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	return string(b)
}
