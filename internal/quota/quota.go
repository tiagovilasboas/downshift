// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package quota supplies offline budget evidence for harness-downshift by
// Tiago de Carvalho Vilas Boas, https://github.com/tiagovilasboas/downshift.
package quota

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/quotamodel"
)

// Aliases keep adapters and commands on quota.Snapshot after the pure model
// moved to quotamodel. Methods stay on the aliased type.
type (
	Status   = quotamodel.Status
	Window   = quotamodel.Window
	Snapshot = quotamodel.Snapshot
)

const (
	MaxTTL     = quotamodel.MaxTTL
	StaleGrace = quotamodel.StaleGrace
	Available  = quotamodel.Available
	Exhausted  = quotamodel.Exhausted
	Unknown    = quotamodel.Unknown
	Stale      = quotamodel.Stale
)

// SourceName is the fixed provenance vocabulary safe for prompt-free logs.
func SourceName(source string) string { return quotamodel.SourceName(source) }

func init() {
	core.LoadQuotaHook = Load
	core.LoadNativeQuotaHook = LoadNative
	core.ParseNativeQuotaHook = ParseNative
	core.RegisterNativeQuotaHook = RegisterNativeSource
	core.PublishNativeQuotaSources()
}

func Path() (string, error) {
	if p := os.Getenv("DOWNSHIFT_QUOTA_FILE"); p != "" {
		return p, nil
	}
	return paths.Join("quota.json")
}

type File struct {
	Version   int                 `json:"version"`
	Harnesses map[string]Snapshot `json:"harnesses"`
}

// Load distinguishes missing evidence from malformed evidence; neither
// case permits a quota claim. No network or credential access occurs here.
func Load(harness string) (*Snapshot, bool) {
	p, err := Path()
	if err != nil {
		return nil, false
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, false
	}
	if err != nil {
		return nil, true
	}
	var f File
	if json.Unmarshal(data, &f) != nil || f.Version != 1 || f.Harnesses == nil {
		return nil, true
	}
	s, ok := f.Harnesses[harness]
	if !ok {
		return nil, true
	}
	return &s, true
}

// Store atomically replaces one harness's snapshot, preserving the others.
func Store(path string, s Snapshot) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	// Directory lock is portable and never used by the hook reader. A crashed
	// writer leaves a visible lock rather than silently losing another harness.
	deadline := time.Now().Add(time.Second)
	for {
		if err := os.Mkdir(path+".lock", 0700); err == nil {
			break
		} else if !os.IsExist(err) {
			return err
		}
		if time.Now().After(deadline) {
			return errors.New("quota cache busy; retry or inspect abandoned .lock directory")
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer os.Remove(path + ".lock")
	f := File{Version: 1, Harnesses: map[string]Snapshot{}}
	if b, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(b, &f) != nil || f.Version != 1 || f.Harnesses == nil {
			return errors.New("invalid quota cache; refusing to overwrite")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if previous, exists := f.Harnesses[s.Harness]; exists && previous.ObservedAt.After(s.ObservedAt) {
		return errors.New("quota observation is older than the cached observation")
	}
	f.Harnesses[s.Harness] = s
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".quota-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
