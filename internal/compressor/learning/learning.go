// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package learning observes unrecognised command outputs and identifies
// candidates for new compressors. It records frequency by command signature
// (the first N words, never the full command or its arguments) and keeps one
// sample per signature so a developer can inspect the output shape.
//
// This is a guia layer (Fowler): it surfaces patterns before the developer
// writes the compressor, and it is never in the hot path of the compressor
// itself.
//
// Performance: all writes are asynchronous and non-blocking. The hook runs
// in a separate goroutine and never delays command execution.
package learning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

func init() {
	// Register the learning hook only when enabled. This is fire-and-forget:
	// errors are silently ignored and the hook never blocks the command.
	// DOWNSHIFT_LEARNING=0 disables it entirely.
	if os.Getenv("DOWNSHIFT_LEARNING") == "0" {
		return
	}
	execwrap.ObserveUncompactedHook = func(argv []string, sample []byte, family execwrap.Family) {
		_ = Record(argv, sample, family)
	}
}

// StoreFileName is the local store under the state directory.
const StoreFileName = "compressor-learning.json"

// MaxSignatureWords limits the words in a command signature.
const MaxSignatureWords = 3

// MaxSampleBytes limits the sample output stored per signature.
const MaxSampleBytes = 4096

// MinCountToSuggest is the threshold for suggesting a new compressor.
const MinCountToSuggest = 5

// Signature identifies a repeating command pattern.
// It is the first N words of the command, never full arguments.
type Signature struct {
	// Words is the command signature, e.g. ["npm", "audit"] or ["pip", "show"].
	Words []string `json:"words"`
	// Hash is the sha256 of the joined words, for stable ordering.
	Hash string `json:"hash"`
}

// Entry is one observed command pattern.
type Entry struct {
	Signature   Signature `json:"signature"`
	Count       int64     `json:"count"`
	FirstSeen   string    `json:"first_seen"`
	LastSeen    string    `json:"last_seen"`
	TotalBytes  int64     `json:"total_bytes"`
	SampleBytes int       `json:"sample_bytes"`
	// Sample is a truncated output sample for inspection; never logged.
	Sample string `json:"sample,omitempty"`
	// Family is the execwrap family, if known.
	Family execwrap.Family `json:"family,omitempty"`
}

// Store is the persistent learning state.
type Store struct {
	// Version for future migrations.
	Version int `json:"version"`
	// Entries by signature hash.
	Entries map[string]*Entry `json:"entries"`
	// UpdatedAt is the last write timestamp.
	UpdatedAt string `json:"updated_at"`
}

// Suggestion is a candidate for a new compressor.
type Suggestion struct {
	Signature   Signature `json:"signature"`
	Count       int64     `json:"count"`
	TotalBytes  int64     `json:"total_bytes"`
	AvgBytes    int64     `json:"avg_bytes"`
	Sample      string    `json:"sample,omitempty"`
	Family      string    `json:"family,omitempty"`
	Recommended bool      `json:"recommended"`
}

var (
	storeMu   sync.Mutex
	storePath string
)

// StorePath returns the learning store path.
func StorePath() (string, error) {
	if storePath != "" {
		return storePath, nil
	}
	return paths.Join(StoreFileName)
}

// SetStorePath overrides the store path (for tests).
func SetStorePath(p string) { storePath = p }

// signatureOf extracts the command signature from argv.
func signatureOf(argv []string) Signature {
	n := len(argv)
	if n > MaxSignatureWords {
		n = MaxSignatureWords
	}
	words := make([]string, n)
	copy(words, argv[:n])

	// Normalise: lowercase, remove version-like suffixes
	for i, w := range words {
		words[i] = normaliseWord(w)
	}

	joined := strings.Join(words, " ")
	h := sha256.Sum256([]byte(joined))
	return Signature{
		Words: words,
		Hash:  hex.EncodeToString(h[:8]),
	}
}

// normaliseWord lowercases and strips version-like suffixes.
func normaliseWord(w string) string {
	w = strings.ToLower(w)
	// Strip @version from npm-style commands
	if i := strings.Index(w, "@"); i > 0 {
		w = w[:i]
	}
	return w
}

// Record observes one unrecognised command output.
// It is called only when the compressor returned FormatUnknown.
func Record(argv []string, output []byte, family execwrap.Family) error {
	if len(argv) == 0 {
		return nil
	}

	sig := signatureOf(argv)
	now := time.Now().UTC().Format(time.RFC3339)

	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := load()
	if err != nil {
		return err
	}

	entry, exists := store.Entries[sig.Hash]
	if !exists {
		entry = &Entry{
			Signature: sig,
			FirstSeen: now,
		}
		store.Entries[sig.Hash] = entry
	}

	entry.Count++
	entry.LastSeen = now
	entry.TotalBytes += int64(len(output))
	if family != "" && entry.Family == "" {
		entry.Family = family
	}

	// Keep a sample for inspection (truncated)
	if entry.Sample == "" && len(output) > 0 {
		sample := output
		if len(sample) > MaxSampleBytes {
			sample = sample[:MaxSampleBytes]
		}
		entry.Sample = string(sample)
		entry.SampleBytes = len(output)
	}

	store.UpdatedAt = now
	return save(store)
}

// load reads the store from disk. Returns an empty store if the file does not exist.
func load() (*Store, error) {
	p, err := StorePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Store{Version: 1, Entries: make(map[string]*Entry)}, nil
		}
		return nil, err
	}

	var store Store
	if err := json.Unmarshal(data, &store); err != nil {
		// Corrupted file: start fresh
		return &Store{Version: 1, Entries: make(map[string]*Entry)}, nil
	}
	if store.Entries == nil {
		store.Entries = make(map[string]*Entry)
	}
	return &store, nil
}

// save writes the store to disk atomically.
func save(store *Store) error {
	p, err := StorePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write via temp file
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Suggestions returns the top N command patterns that would benefit
// from a new compressor, sorted by total bytes (highest impact first).
func Suggestions(limit int) ([]Suggestion, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := load()
	if err != nil {
		return nil, err
	}

	var suggestions []Suggestion
	for _, entry := range store.Entries {
		if entry.Count < MinCountToSuggest {
			continue
		}

		suggestions = append(suggestions, Suggestion{
			Signature:   entry.Signature,
			Count:       entry.Count,
			TotalBytes:  entry.TotalBytes,
			AvgBytes:    entry.TotalBytes / entry.Count,
			Sample:      entry.Sample,
			Family:      string(entry.Family),
			Recommended: entry.Count >= 10 && entry.TotalBytes >= 10*1024,
		})
	}

	// Sort by total bytes descending (highest impact first)
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].TotalBytes > suggestions[j].TotalBytes
	})

	if limit > 0 && len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}
	return suggestions, nil
}

// Stats returns summary statistics of the learning store.
func Stats() (StoreStats, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := load()
	if err != nil {
		return StoreStats{}, err
	}

	stats := StoreStats{
		TotalSignatures: len(store.Entries),
		UpdatedAt:       store.UpdatedAt,
	}

	for _, entry := range store.Entries {
		stats.TotalCommands += entry.Count
		stats.TotalBytes += entry.TotalBytes
		if entry.Count >= MinCountToSuggest {
			stats.SuggestableCount++
		}
	}

	return stats, nil
}

// StoreStats summarises the learning store.
type StoreStats struct {
	TotalSignatures  int    `json:"total_signatures"`
	TotalCommands    int64  `json:"total_commands"`
	TotalBytes       int64  `json:"total_bytes"`
	SuggestableCount int    `json:"suggestable_count"`
	UpdatedAt        string `json:"updated_at"`
}

// Clear resets the learning store (for tests or user request).
func Clear() error {
	storeMu.Lock()
	defer storeMu.Unlock()

	p, err := StorePath()
	if err != nil {
		return err
	}

	return os.Remove(p)
}

// Export returns the raw store for inspection.
func Export() (*Store, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	return load()
}
