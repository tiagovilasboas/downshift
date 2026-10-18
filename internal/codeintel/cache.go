// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultCacheRelativePath is the standard location for DCI cache relative to repo root.
const DefaultCacheRelativePath = ".downshift/cache/codeintel.json"

// LoadCache loads the serialized InvertedIndex from disk.
// Returns an error if the file does not exist or cannot be parsed.
func LoadCache(cacheFilePath string) (*InvertedIndex, error) {
	if cacheFilePath == "" {
		return nil, errors.New("empty cache file path")
	}

	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return nil, err
	}

	var idx InvertedIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("unmarshal cache json: %w", err)
	}

	if idx.Documents == nil {
		idx.Documents = make(map[string]*Document)
	}
	if idx.Postings == nil {
		idx.Postings = make(map[string][]Posting)
	}

	return &idx, nil
}

// SaveCache atomically persists the InvertedIndex to disk.
// It creates any required parent directories with 0700 permissions
// and writes the cache file with 0600 permissions.
// Sensitive data (raw source code, prompts, conversational context) is NEVER stored.
func SaveCache(cacheFilePath string, idx *InvertedIndex) error {
	if cacheFilePath == "" {
		return errors.New("empty cache file path")
	}
	if idx == nil {
		return errors.New("cannot save nil index")
	}

	dir := filepath.Dir(cacheFilePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("mkdir cache dir: %w", err)
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache index: %w", err)
	}

	// Write to temporary file in the same directory for atomic rename
	tmpFile, err := os.CreateTemp(dir, "dci_cache_*.tmp")
	if err != nil {
		return fmt.Errorf("create temp cache file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure cleanup in case of failure before rename
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	// Ensure mode 0600
	if err := tmpFile.Chmod(0600); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp cache file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("sync temp cache file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp cache file: %w", err)
	}

	// Atomic replace
	if err := os.Rename(tmpPath, cacheFilePath); err != nil {
		return fmt.Errorf("atomic rename cache file: %w", err)
	}

	success = true
	return nil
}
