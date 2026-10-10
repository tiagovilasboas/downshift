// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package adapt

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// DefaultMemoryPath returns adapt-memory.json under the state directory.
func DefaultMemoryPath() (string, error) {
	return paths.Join("adapt-memory.json")
}

// Load reads tier memory from path. A missing file returns (nil, nil).
// Read or parse errors return the original routing decision upstream (fail-open).
func Load(path string) (*Memory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var mem Memory
	if err := json.Unmarshal(data, &mem); err != nil {
		return nil, err
	}
	if mem.Version == 0 {
		mem.Version = fileVersion
	}
	if mem.Shapes == nil {
		mem.Shapes = make(map[string]ShapeRecord)
	}
	if mem.empty() {
		return nil, nil
	}
	return &mem, nil
}

// LoadDefault reads memory from the default state path.
func LoadDefault() (*Memory, error) {
	path, err := DefaultMemoryPath()
	if err != nil {
		return nil, err
	}
	return Load(path)
}

// Save writes memory atomically to path.
func Save(path string, mem Memory) error {
	if mem.Version == 0 {
		mem.Version = fileVersion
	}
	if mem.Shapes == nil {
		mem.Shapes = make(map[string]ShapeRecord)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(mem)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return nil
}

// SaveDefault persists memory under the default state path.
func SaveDefault(mem Memory) error {
	path, err := DefaultMemoryPath()
	if err != nil {
		return err
	}
	return Save(path, mem)
}

// ErrEmptyMemory is returned when there is nothing worth persisting.
var ErrEmptyMemory = errors.New("adapt: empty memory")
