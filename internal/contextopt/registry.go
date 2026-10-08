// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contextopt

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// DefaultConfigFileName is the configuration file name in Downshift's state directory.
const DefaultConfigFileName = "context-optimization.json"

// Registry manages registered context optimization providers and configuration.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry creates a new empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register registers a provider.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.ID()] = p
}

// Get retrieves a provider by ID.
func (r *Registry) Get(id string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[id]
	return p, ok
}

// List returns all registered providers.
func (r *Registry) List() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		res = append(res, p)
	}
	return res
}

// ConfigPath resolves the path to context-optimization.json in the state dir.
func ConfigPath() (string, error) {
	return paths.Join(DefaultConfigFileName)
}

// DefaultConfig returns the safe default configuration with Native Context Compressor enabled.
func DefaultConfig() Config {
	var cfg Config
	cfg.Enabled = true
	cfg.Provider = "native"
	cfg.ActiveHarness = []string{"claude-code", "cursor", "codex", "antigravity", "kirocrew"}
	cfg.Scope.MainAgent = true
	cfg.Scope.SpawnedAgents = true
	cfg.Safety.PreserveOriginalOutput = true
	cfg.Safety.AllowFullContextRecovery = true
	cfg.Observability.Enabled = true
	return cfg
}

// LoadConfig loads the context optimization config, or returns the default enabled config if not found.
func LoadConfig() (Config, error) {
	p, err := ConfigPath()
	if err != nil {
		return DefaultConfig(), err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), err
	}
	return cfg, nil
}

// SaveConfig atomically writes the context optimization config.
func SaveConfig(cfg Config) error {
	p, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
