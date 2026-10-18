// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"context"
	"path/filepath"
	"sync"
	"time"
)

// Service coordinates code discovery, indexing, ranking, abstention, and context packaging.
type Service struct {
	repoRoot    string
	config      Config
	cachePath   string
	index       *InvertedIndex
	lastChecked time.Time
	mu          sync.RWMutex
}

// NewService creates a new DCI service instance for the given repository root and configuration.
func NewService(repoRoot string, cfg Config) *Service {
	cachePath := filepath.Join(repoRoot, DefaultCacheRelativePath)
	return &Service{
		repoRoot:  repoRoot,
		config:    cfg,
		cachePath: cachePath,
	}
}

// SetCachePath overrides the default cache file path (primarily for testing).
func (s *Service) SetCachePath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachePath = path
}

// GetIndex returns a reference to the active inverted index.
func (s *Service) GetIndex() *InvertedIndex {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.index
}

// EnsureIndex ensures that the inverted index is loaded and incrementally up-to-date.
func (s *Service) EnsureIndex(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If index is already loaded in memory and checked within the last 5 seconds, reuse it
	if s.index != nil && !s.lastChecked.IsZero() && time.Since(s.lastChecked) < 5*time.Second {
		return nil
	}

	// 1. Try loading cached index if not already in memory
	if s.index == nil {
		cached, err := LoadCache(s.cachePath)
		if err == nil && cached != nil {
			s.index = cached
		} else {
			s.index = NewInvertedIndex()
		}
	}

	// 2. Discover files within boundary and size limits
	discovered, err := DiscoverFiles(ctx, s.repoRoot, s.config)
	if err != nil {
		return err
	}

	// 3. Incremental update
	if err := s.index.UpdateIncremental(s.repoRoot, discovered); err != nil {
		return err
	}

	// 4. Persist cache atomically (fail-open: ignore cache save errors)
	_ = SaveCache(s.cachePath, s.index)
	s.lastChecked = time.Now()

	return nil
}

// ProcessPrompt runs the complete DCI pipeline for a user prompt with strict fail-open semantics.
// Returns ContextPack (or nil if abstained/disabled) and privacy-safe Telemetry.
func (s *Service) ProcessPrompt(ctx context.Context, prompt string) (*ContextPack, Telemetry) {
	start := time.Now()
	telemetry := Telemetry{
		InputBytes: len(prompt),
		Timestamp:  start.UTC(),
	}

	// Check operational mode
	if s.config.Mode == ModeOff {
		telemetry.TotalDurationMs = time.Since(start).Milliseconds()
		return nil, telemetry
	}

	// Fail-open guarantee: check context cancellation/timeout
	select {
	case <-ctx.Done():
		telemetry.ErrorCode = "context_cancelled"
		telemetry.TotalDurationMs = time.Since(start).Milliseconds()
		return nil, telemetry
	default:
	}

	// 1. Ensure index is ready
	indexStart := time.Now()
	if err := s.EnsureIndex(ctx); err != nil {
		telemetry.ErrorCode = "index_error"
		telemetry.TotalDurationMs = time.Since(start).Milliseconds()
		return nil, telemetry
	}
	telemetry.IndexDurationMs = time.Since(indexStart).Milliseconds()

	s.mu.RLock()
	idx := s.index
	telemetry.IndexedFiles = len(idx.Documents)
	s.mu.RUnlock()

	// 2. Retrieval & Ranking
	retrievalStart := time.Now()
	queryTokens := Tokenize(prompt)
	candidates := RankCandidates(idx, queryTokens, prompt)
	telemetry.RetrievalDurationMs = time.Since(retrievalStart).Milliseconds()
	telemetry.CandidateCount = len(candidates)

	// 3. Abstention Check
	abstain, reason := ShouldAbstain(prompt, queryTokens, candidates)
	if abstain {
		telemetry.AbstentionReason = reason
		telemetry.TotalDurationMs = time.Since(start).Milliseconds()
		return nil, telemetry
	}

	// 4. Prune Top-K (up to 3 files, 8 symbols)
	topCandidates := PruneTopK(candidates, MaxTopFiles, MaxTopSymbols)

	// 5. Extract Snippets (up to 4 snippets)
	ExtractSnippets(s.repoRoot, topCandidates, MaxTopSnippets)

	// 6. Context Packaging
	packStart := time.Now()
	maxBytes := s.config.MaxContextBytes
	if maxBytes <= 0 {
		maxBytes = 3500
	}
	pack := PackContext(topCandidates, MaxTopFiles, maxBytes)
	telemetry.PackDurationMs = time.Since(packStart).Milliseconds()

	if pack != nil {
		telemetry.SelectedFiles = len(pack.SelectedFiles)
		telemetry.SelectedSymbols = pack.SymbolsCount
		telemetry.OutputBytes = pack.OutputBytes
	}

	telemetry.TotalDurationMs = time.Since(start).Milliseconds()

	// In observe mode, record telemetry but do not return injected context
	if s.config.Mode == ModeObserve {
		telemetry.InjectionApplied = false
		return nil, telemetry
	}

	// Active mode: context injection applied
	telemetry.InjectionApplied = (pack != nil && pack.OutputBytes > 0)
	return pack, telemetry
}
