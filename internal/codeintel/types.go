// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"time"
)

// Mode defines the operational mode of DCI.
type Mode string

const (
	ModeOff     Mode = "off"
	ModeObserve Mode = "observe"
	ModeActive  Mode = "active"
)

// SymbolKind indicates the semantic category of an extracted symbol.
type SymbolKind string

const (
	KindFunction  SymbolKind = "func"
	KindMethod    SymbolKind = "method"
	KindStruct    SymbolKind = "struct"
	KindInterface SymbolKind = "interface"
	KindType      SymbolKind = "type"
	KindVariable  SymbolKind = "var"
	KindConstant  SymbolKind = "const"
	KindOther     SymbolKind = "other"
)

// Symbol represents an extracted code identifier with location.
type Symbol struct {
	Name         string     `json:"name"`
	Kind         SymbolKind `json:"kind"`
	RelativePath string     `json:"path"`
	StartLine    int        `json:"start_line"`
	EndLine      int        `json:"end_line"`
	Calls        []string   `json:"calls,omitempty"`
}

// Document holds the indexed representation of a source file.
// Note: Raw source code is NEVER persisted in the index.
type Document struct {
	RelativePath string         `json:"path"`
	SHA256       string         `json:"sha256"`
	Size         int64          `json:"size"`
	ModTime      time.Time      `json:"mod_time"`
	TermFreqs    map[string]int `json:"term_freqs"`
	Symbols      []Symbol       `json:"symbols"`
	Imports      []string       `json:"imports,omitempty"`
	TotalTerms   int            `json:"total_terms"`
}

// InvertedIndex stores terms mapped to postings (document relative paths with frequencies).
type InvertedIndex struct {
	Documents   map[string]*Document `json:"documents"`
	Postings    map[string][]Posting `json:"postings"`
	TotalTokens int64                `json:"total_tokens"`
	AvgDocLen   float64              `json:"avg_doc_len"`
	LastUpdated time.Time            `json:"last_updated"`
}

// Posting represents a term match in a document.
type Posting struct {
	DocPath   string `json:"doc_path"`
	Frequency int    `json:"freq"`
}

// Config controls DCI behavior, discovery limits and context budgets.
type Config struct {
	Mode             Mode     `json:"mode"`
	MaxFiles         int      `json:"max_files"`
	MaxFileSizeBytes int64    `json:"max_file_size_bytes"`
	MaxRunBytes      int64    `json:"max_run_bytes"`
	MaxContextBytes  int      `json:"max_context_bytes"`
	Excludes         []string `json:"excludes"`
}

// DefaultConfig provides conservative defaults as per DCI specification.
func DefaultConfig() Config {
	return Config{
		Mode:             ModeOff,
		MaxFiles:         2000,
		MaxFileSizeBytes: 512 * 1024,       // 512 KiB
		MaxRunBytes:      16 * 1024 * 1024, // 16 MiB
		MaxContextBytes:  3500,             // 3500 bytes UTF-8
		Excludes: []string{
			"node_modules",
			"vendor",
			".git",
			"dist",
			"build",
			"coverage",
		},
	}
}

// Snippet is a contiguous verbatim excerpt from a source file.
type Snippet struct {
	RelativePath string `json:"path"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	Content      string `json:"content"`
}

// Candidate represents a scored retrieval candidate for packaging.
type Candidate struct {
	Document       *Document `json:"document"`
	Score          float64   `json:"score"`
	MatchedTerms   []string  `json:"matched_terms"`
	MatchedSymbols []Symbol  `json:"matched_symbols"`
	Snippets       []Snippet `json:"snippets"`
}

// ContextPack represents the compact structural context produced for prompt injection.
type ContextPack struct {
	FormattedText string   `json:"formatted_text"`
	OutputBytes   int      `json:"output_bytes"`
	SelectedFiles []string `json:"selected_files"`
	SymbolsCount  int      `json:"symbols_count"`
	SnippetsCount int      `json:"snippets_count"`
	RelevantTerms []string `json:"relevant_terms"`
}

// Telemetry captures privacy-safe execution metrics for DCI operations.
// Text prompts, source code and injected contents are NEVER stored.
type Telemetry struct {
	IndexedFiles        int       `json:"indexed_files"`
	CandidateCount      int       `json:"candidate_count"`
	SelectedFiles       int       `json:"selected_files"`
	SelectedSymbols     int       `json:"selected_symbols"`
	InputBytes          int       `json:"input_bytes"`
	OutputBytes         int       `json:"output_bytes"`
	IndexDurationMs     int64     `json:"index_duration_ms"`
	RetrievalDurationMs int64     `json:"retrieval_duration_ms"`
	PackDurationMs      int64     `json:"pack_duration_ms"`
	TotalDurationMs     int64     `json:"total_duration_ms"`
	InjectionApplied    bool      `json:"injection_applied"`
	AbstentionReason    string    `json:"abstention_reason,omitempty"`
	ErrorCode           string    `json:"error_code,omitempty"`
	Timestamp           time.Time `json:"timestamp"`
}
