// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// EnvPath overrides the cache location (tests and operators).
const EnvPath = "DOWNSHIFT_DISCOVERED"

// Entry is the cached result for one harness.
type Entry struct {
	Source    string    `json:"source"`
	FetchedAt time.Time `json:"fetched_at"`
	Models    []Model   `json:"models"`
	Ranked
}

// File is discovered.json. core reads the `ordered` field of each harness
// with a minimal struct of its own; keep the field names stable.
type File struct {
	Version   int              `json:"version"`
	Harnesses map[string]Entry `json:"harnesses"`
}

// Path is the cache file: $DOWNSHIFT_DISCOVERED or <state dir>/discovered.json.
func Path() (string, error) {
	if p := os.Getenv(EnvPath); p != "" {
		return p, nil
	}
	return paths.Join("discovered.json")
}

// Read loads the cache. A missing or invalid file is an empty cache.
func Read(path string) File {
	f := File{Version: 1, Harnesses: map[string]Entry{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return f
	}
	var in File
	if json.Unmarshal(data, &in) != nil || in.Harnesses == nil {
		return f
	}
	in.Version = 1
	return in
}

// Merge replaces the given harnesses and keeps the others, then writes the
// file atomically with owner-only permissions. A harness whose discovery
// failed must not be passed: its previous entry stays.
func Merge(path string, updates map[string]Entry) error {
	f := Read(path)
	for h, e := range updates {
		f.Harnesses[h] = e
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".discovered.json-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
