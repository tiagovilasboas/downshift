// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// NewInvertedIndex initializes an empty InvertedIndex.
func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		Documents:   make(map[string]*Document),
		Postings:    make(map[string][]Posting),
		TotalTokens: 0,
		AvgDocLen:   0,
		LastUpdated: time.Now().UTC(),
	}
}

// AddDocument inserts or replaces a document in the index.
func (idx *InvertedIndex) AddDocument(doc *Document) {
	if doc == nil || doc.RelativePath == "" {
		return
	}
	if idx.Documents == nil {
		idx.Documents = make(map[string]*Document)
	}
	idx.Documents[doc.RelativePath] = doc
}

// RemoveDocument removes a document from the index.
func (idx *InvertedIndex) RemoveDocument(relPath string) {
	delete(idx.Documents, relPath)
}

// UpdateIncremental updates the index incrementally based on discovered files.
// It detects changes via SHA-256 (skipping unchanged files if ModTime/Size match),
// parses new/modified files, purges removed files, and recalculates index statistics.
func (idx *InvertedIndex) UpdateIncremental(repoRoot string, files []DiscoveredFile) error {
	if idx.Documents == nil {
		idx.Documents = make(map[string]*Document)
	}
	if idx.Postings == nil {
		idx.Postings = make(map[string][]Posting)
	}

	currentPaths := make(map[string]bool, len(files))

	for _, file := range files {
		relPath := file.RelativePath
		currentPaths[relPath] = true

		existingDoc, exists := idx.Documents[relPath]
		if exists && existingDoc.Size == file.Size && existingDoc.ModTime.Equal(file.ModTime.UTC().Truncate(time.Second)) {
			// Fast path: file size and mod time unchanged. Verify SHA-256 only if needed or keep existing.
			continue
		}

		// Calculate current SHA-256
		fullPath := file.FullPath
		if fullPath == "" {
			fullPath = filepath.Join(repoRoot, relPath)
		}

		currentSHA, err := computeFileSHA256(fullPath)
		if err != nil {
			// Skip file if unreadable
			continue
		}

		if exists && existingDoc.SHA256 == currentSHA {
			// SHA-256 matches, just update ModTime/Size
			existingDoc.ModTime = file.ModTime.UTC().Truncate(time.Second)
			existingDoc.Size = file.Size
			continue
		}

		// File is new or changed: parse it
		doc, err := ParseFile(repoRoot, relPath)
		if err != nil {
			continue
		}

		idx.Documents[relPath] = doc
	}

	// Purge deleted documents
	for relPath := range idx.Documents {
		if !currentPaths[relPath] {
			delete(idx.Documents, relPath)
		}
	}

	// Rebuild postings list and recalculate statistics
	idx.RebuildPostings()
	idx.LastUpdated = time.Now().UTC()

	return nil
}

// RebuildPostings constructs deterministic postings lists from all current documents
// and recomputes AvgDocLen and TotalTokens.
func (idx *InvertedIndex) RebuildPostings() {
	newPostings := make(map[string][]Posting)
	var totalTokens int64

	// Sort document paths for deterministic iteration
	docPaths := make([]string, 0, len(idx.Documents))
	for p := range idx.Documents {
		docPaths = append(docPaths, p)
	}
	sort.Strings(docPaths)

	for _, p := range docPaths {
		doc := idx.Documents[p]
		totalTokens += int64(doc.TotalTerms)

		for term, freq := range doc.TermFreqs {
			newPostings[term] = append(newPostings[term], Posting{
				DocPath:   p,
				Frequency: freq,
			})
		}
	}

	// Ensure postings lists are sorted by DocPath
	for term := range newPostings {
		sort.Slice(newPostings[term], func(i, j int) bool {
			return newPostings[term][i].DocPath < newPostings[term][j].DocPath
		})
	}

	idx.Postings = newPostings
	idx.TotalTokens = totalTokens

	docCount := len(idx.Documents)
	if docCount > 0 {
		idx.AvgDocLen = float64(totalTokens) / float64(docCount)
	} else {
		idx.AvgDocLen = 0
	}
}

// computeFileSHA256 streams file content to compute SHA-256 without loading entire file into memory.
func computeFileSHA256(fullPath string) (string, error) {
	f, err := os.Open(fullPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
