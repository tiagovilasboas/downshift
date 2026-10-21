// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

func TestSignatureOf(t *testing.T) {
	tests := []struct {
		name     string
		argv     []string
		wantLen  int
		wantWord string // first word
	}{
		{"single word", []string{"ls"}, 1, "ls"},
		{"two words", []string{"npm", "audit"}, 2, "npm"},
		{"three words", []string{"pip", "show", "requests"}, 3, "pip"},
		{"more than three", []string{"go", "test", "-v", "./..."}, 3, "go"},
		{"normalises case", []string{"NPM", "AUDIT"}, 2, "npm"},
		{"strips @version", []string{"npm@10.2.3", "audit"}, 2, "npm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := signatureOf(tt.argv)
			if len(sig.Words) != tt.wantLen {
				t.Errorf("got %d words, want %d", len(sig.Words), tt.wantLen)
			}
			if sig.Words[0] != tt.wantWord {
				t.Errorf("got first word %q, want %q", sig.Words[0], tt.wantWord)
			}
			if sig.Hash == "" {
				t.Error("hash should not be empty")
			}
		})
	}
}

func TestRecordAndSuggestions(t *testing.T) {
	// Use a temp directory for the store
	tmpDir := t.TempDir()
	SetStorePath(filepath.Join(tmpDir, "test-learning.json"))
	defer SetStorePath("")

	// Record several commands
	for i := 0; i < 10; i++ {
		err := Record([]string{"npm", "audit"}, []byte("output sample"), "")
		if err != nil {
			t.Fatalf("Record failed: %v", err)
		}
	}

	// Record fewer of another command
	for i := 0; i < 3; i++ {
		err := Record([]string{"pip", "show", "requests"}, []byte("pip output"), "")
		if err != nil {
			t.Fatalf("Record failed: %v", err)
		}
	}

	// Get suggestions
	suggestions, err := Suggestions(10)
	if err != nil {
		t.Fatalf("Suggestions failed: %v", err)
	}

	// npm audit should appear (10 >= MinCountToSuggest)
	// pip show should not (3 < MinCountToSuggest)
	if len(suggestions) != 1 {
		t.Fatalf("got %d suggestions, want 1", len(suggestions))
	}

	if suggestions[0].Signature.Words[0] != "npm" {
		t.Errorf("expected npm, got %v", suggestions[0].Signature.Words)
	}
	if suggestions[0].Count != 10 {
		t.Errorf("expected count 10, got %d", suggestions[0].Count)
	}
}

func TestStats(t *testing.T) {
	tmpDir := t.TempDir()
	SetStorePath(filepath.Join(tmpDir, "test-learning.json"))
	defer SetStorePath("")

	// Record some commands
	for i := 0; i < 7; i++ {
		_ = Record([]string{"docker", "compose", "ps"}, []byte("output"), "")
	}

	stats, err := Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.TotalSignatures != 1 {
		t.Errorf("expected 1 signature, got %d", stats.TotalSignatures)
	}
	if stats.TotalCommands != 7 {
		t.Errorf("expected 7 commands, got %d", stats.TotalCommands)
	}
	if stats.SuggestableCount != 1 {
		t.Errorf("expected 1 suggestable, got %d", stats.SuggestableCount)
	}
}

func TestClear(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "test-learning.json")
	SetStorePath(storePath)
	defer SetStorePath("")

	_ = Record([]string{"test", "cmd"}, []byte("output"), "")

	// Verify file exists
	if _, err := os.Stat(storePath); os.IsNotExist(err) {
		t.Fatal("store file should exist after Record")
	}

	// Clear
	if err := Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Error("store file should not exist after Clear")
	}
}

func TestExport(t *testing.T) {
	tmpDir := t.TempDir()
	SetStorePath(filepath.Join(tmpDir, "test-learning.json"))
	defer SetStorePath("")

	_ = Record([]string{"kubectl", "get", "pods"}, []byte("pod output"), execwrap.FamilyKubectlGet)

	store, err := Export()
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if store.Version != 1 {
		t.Errorf("expected version 1, got %d", store.Version)
	}
	if len(store.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(store.Entries))
	}
}

func TestEmptyArgv(t *testing.T) {
	tmpDir := t.TempDir()
	SetStorePath(filepath.Join(tmpDir, "test-learning.json"))
	defer SetStorePath("")

	// Should not error on empty argv
	err := Record(nil, []byte("output"), "")
	if err != nil {
		t.Errorf("Record with nil argv should not error: %v", err)
	}

	err = Record([]string{}, []byte("output"), "")
	if err != nil {
		t.Errorf("Record with empty argv should not error: %v", err)
	}
}
