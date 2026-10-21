// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

// StoreFileName is the local store under the state directory.
const StoreFileName = "compressor-learning.json"

// SchemaVersion is the current store schema. Version 1 kept a raw output
// sample per entry; version 2 keeps only a Shape. Loading a version 1 file
// drops the sample and rewrites the file at version 2.
const SchemaVersion = 2

// Signature identifies a repeating command pattern.
// It is the first N words of the command, never full arguments.
type Signature struct {
	// Words is the command signature, e.g. ["npm", "audit"] or ["pip", "show"].
	Words []string `json:"words"`
	// Hash is the sha256 of the joined words, for stable ordering.
	Hash string `json:"hash"`
}

// Entry is one observed command pattern. It carries no output text: see
// Shape for what is kept.
type Entry struct {
	Signature  Signature `json:"signature"`
	Count      int64     `json:"count"`
	FirstSeen  string    `json:"first_seen"`
	LastSeen   string    `json:"last_seen"`
	TotalBytes int64     `json:"total_bytes"`
	// SampleBytes is the size of the output whose Shape is kept.
	SampleBytes int `json:"sample_bytes"`
	// Shape is the structure of the first observed output.
	Shape *Shape `json:"shape,omitempty"`
	// Family is the execwrap family, if known.
	Family execwrap.Family `json:"family,omitempty"`
}

// Store is the persistent learning state.
type Store struct {
	// Version is the schema version (SchemaVersion).
	Version int `json:"version"`
	// Entries by signature hash.
	Entries map[string]*Entry `json:"entries"`
	// UpdatedAt is the last write timestamp.
	UpdatedAt string `json:"updated_at"`
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

func emptyStore() *Store {
	return &Store{Version: SchemaVersion, Entries: make(map[string]*Entry)}
}

// load reads the store from disk. A missing or corrupted file yields an empty
// store. A file at an older schema is migrated: the Entry type has no sample
// field, so the raw text is dropped on decode, and the file is rewritten so
// the text leaves the disk on first use.
func load() (*Store, error) {
	p, err := StorePath()
	if err != nil {
		return nil, err
	}
	removeStaleTemp(p)

	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return emptyStore(), nil
		}
		return nil, err
	}

	var store Store
	if err := json.Unmarshal(data, &store); err != nil {
		return emptyStore(), nil
	}
	if store.Entries == nil {
		store.Entries = make(map[string]*Entry)
	}
	if store.Version < SchemaVersion {
		store.Version = SchemaVersion
		if err := save(&store); err != nil {
			return nil, err
		}
	}
	return &store, nil
}

// save writes the store atomically: a fresh temp file in the same directory,
// then a rename. A failed write never leaves a partial store behind.
func save(store *Store) error {
	p, err := StorePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, StoreFileName+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, p); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

// staleTempAge is how old a ".*.tmp" must be before it is treated as the
// leftover of an interrupted save rather than another process's write in
// flight.
const staleTempAge = time.Minute

// removeStaleTemp deletes temp files an interrupted save left next to the
// store: the fixed-name ".tmp" of schema 1 (always stale) and any ".*.tmp"
// older than staleTempAge. They are never a valid store, so nothing is lost.
func removeStaleTemp(p string) {
	_ = os.Remove(p + ".tmp")
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(p), StoreFileName+".*.tmp"))
	for _, m := range matches {
		if info, err := os.Stat(m); err == nil && time.Since(info.ModTime()) > staleTempAge {
			_ = os.Remove(m)
		}
	}
}
