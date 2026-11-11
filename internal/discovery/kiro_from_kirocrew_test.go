// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestKiroFromKiroCrew_Harness(t *testing.T) {
	src := KiroFromKiroCrew{}
	if got := src.Harness(); got != "kiro" {
		t.Errorf("Harness() = %q, want %q", got, "kiro")
	}
}

func TestKiroFromKiroCrew_Name(t *testing.T) {
	src := KiroFromKiroCrew{}
	if got := src.Name(); got != "kirocrew discovery (same account)" {
		t.Errorf("Name() = %q, want %q", got, "kirocrew discovery (same account)")
	}
}

func TestKiroFromKiroCrew_Discover_Fresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "discovered.json")
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

	cache := File{
		Version: 1,
		Harnesses: map[string]Entry{
			"kirocrew": {
				Source:    "kiro-cli chat --list-models",
				FetchedAt: now.Add(-1 * time.Hour), // 1 hour ago
				Models: []Model{
					{ID: "claude-opus-5", Display: "Claude Opus 5"},
					{ID: "claude-sonnet-4.6", Display: "Claude Sonnet 4.6"},
					{ID: "claude-haiku-4.5", Display: "Claude Haiku 4.5"},
				},
			},
		},
	}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	src := KiroFromKiroCrew{CachePath: path, Now: func() time.Time { return now }}
	models, err := src.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(models) != 3 {
		t.Errorf("got %d models, want 3", len(models))
	}
	if models[0].ID != "claude-opus-5" {
		t.Errorf("models[0].ID = %q, want %q", models[0].ID, "claude-opus-5")
	}
}

func TestKiroFromKiroCrew_Discover_Stale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "discovered.json")
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

	cache := File{
		Version: 1,
		Harnesses: map[string]Entry{
			"kirocrew": {
				Source:    "kiro-cli chat --list-models",
				FetchedAt: now.Add(-8 * 24 * time.Hour), // 8 days ago (stale)
				Models: []Model{
					{ID: "claude-sonnet-4.5"},
				},
			},
		},
	}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	src := KiroFromKiroCrew{CachePath: path, Now: func() time.Time { return now }}
	_, err := src.Discover(context.Background())
	if err == nil {
		t.Error("expected error for stale cache, got nil")
	}
}

func TestKiroFromKiroCrew_Discover_Missing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "discovered.json")

	// Empty cache, no kirocrew entry
	cache := File{Version: 1, Harnesses: map[string]Entry{}}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	src := KiroFromKiroCrew{CachePath: path}
	_, err := src.Discover(context.Background())
	if err == nil {
		t.Error("expected error for missing kirocrew entry, got nil")
	}
}

func TestKiroFromKiroCrew_Discover_CustomMaxAge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "discovered.json")
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

	cache := File{
		Version: 1,
		Harnesses: map[string]Entry{
			"kirocrew": {
				Source:    "kiro-cli chat --list-models",
				FetchedAt: now.Add(-2 * time.Hour), // 2 hours ago
				Models:    []Model{{ID: "claude-sonnet-4.6"}},
			},
		},
	}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// MaxAge = 1 hour, so 2-hour-old cache is stale
	src := KiroFromKiroCrew{CachePath: path, MaxAge: 1 * time.Hour, Now: func() time.Time { return now }}
	_, err := src.Discover(context.Background())
	if err == nil {
		t.Error("expected error for custom MaxAge exceeded, got nil")
	}
}
