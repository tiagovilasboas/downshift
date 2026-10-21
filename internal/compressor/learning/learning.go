// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package learning observes unrecognised command outputs and identifies
// candidates for new compressors. It records frequency by command signature
// (the first N words, never the full command or its arguments) and keeps the
// Shape of one output per signature so a developer can judge whether a
// compressor is worth writing.
//
// Privacy: the store never holds output text. Schema 2 keeps, per signature:
//
//   - signature: the first MaxSignatureWords words, normalised
//   - count, first_seen, last_seen, total_bytes, family
//   - sample_bytes: the size of the output whose shape is kept
//   - shape: per-line class counts (blank, short, long, indented,
//     number_heavy, path_like, url_like, status_like), line-length buckets,
//     a byte-class histogram and a truncated hash of the first line's
//     character-class mask (see Shape)
//
// Schema 1 stored up to 4 KiB of raw output per signature. A schema 1 file
// is migrated on first load: the sample is dropped and the file rewritten.
//
// This is a guia layer (Fowler): it surfaces patterns before the developer
// writes the compressor, and it is never in the hot path of the compressor
// itself. The runner calls the hook after the command's output is written,
// under a short deadline, so a record is on disk when the command returns
// without delaying its output or changing its exit code.
package learning

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

func init() {
	// Register the learning hook only when enabled. Errors are ignored and
	// the hook never fails the command. DOWNSHIFT_LEARNING=0 disables it.
	if os.Getenv("DOWNSHIFT_LEARNING") == "0" {
		return
	}
	execwrap.ObserveUncompactedHook = func(argv []string, sample []byte, family execwrap.Family) {
		_ = Record(argv, sample, family)
	}
}

// MaxSignatureWords limits the words in a command signature.
const MaxSignatureWords = 3

// MinCountToSuggest is the threshold for suggesting a new compressor.
const MinCountToSuggest = 5

// Suggestion is a candidate for a new compressor.
type Suggestion struct {
	Signature   Signature `json:"signature"`
	Count       int64     `json:"count"`
	TotalBytes  int64     `json:"total_bytes"`
	AvgBytes    int64     `json:"avg_bytes"`
	Shape       *Shape    `json:"shape,omitempty"`
	Family      string    `json:"family,omitempty"`
	Recommended bool      `json:"recommended"`
}

// signatureOf extracts the command signature from argv.
func signatureOf(argv []string) Signature {
	n := len(argv)
	if n > MaxSignatureWords {
		n = MaxSignatureWords
	}
	words := make([]string, n)
	for i, w := range argv[:n] {
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

// Record observes one unrecognised command output. Only the output's size
// and Shape are kept; the bytes themselves are never written.
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

	if entry.Shape == nil && len(output) > 0 {
		shape := ShapeOf(output)
		entry.Shape = &shape
		entry.SampleBytes = len(output)
	}

	store.UpdatedAt = now
	return save(store)
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
			Shape:       entry.Shape,
			Family:      string(entry.Family),
			Recommended: entry.Count >= 10 && entry.TotalBytes >= 10*1024,
		})
	}

	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].TotalBytes > suggestions[j].TotalBytes
	})

	if limit > 0 && len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}
	return suggestions, nil
}

// StoreStats summarises the learning store.
type StoreStats struct {
	TotalSignatures  int    `json:"total_signatures"`
	TotalCommands    int64  `json:"total_commands"`
	TotalBytes       int64  `json:"total_bytes"`
	SuggestableCount int    `json:"suggestable_count"`
	UpdatedAt        string `json:"updated_at"`
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

// Clear resets the learning store (for tests or user request).
func Clear() error {
	storeMu.Lock()
	defer storeMu.Unlock()

	p, err := StorePath()
	if err != nil {
		return err
	}
	removeStaleTemp(p)
	return os.Remove(p)
}

// Export returns the store for inspection. It holds no output text.
func Export() (*Store, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	return load()
}
