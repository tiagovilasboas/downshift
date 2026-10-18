// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheSaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "subdir", "cache.json")

	idx := NewInvertedIndex()
	idx.Documents["main.go"] = &Document{
		RelativePath: "main.go",
		SHA256:       "abc12345",
		Size:         100,
		ModTime:      time.Now().UTC().Truncate(time.Second),
		TermFreqs:    map[string]int{"main": 1, "run": 2},
		Symbols: []Symbol{
			{Name: "main", Kind: KindFunction, RelativePath: "main.go", StartLine: 1, EndLine: 5},
		},
		TotalTerms: 3,
	}
	idx.RebuildPostings()

	if err := SaveCache(cacheFile, idx); err != nil {
		t.Fatalf("SaveCache failed: %v", err)
	}

	// Verify permissions
	fi, err := os.Stat(cacheFile)
	if err != nil {
		t.Fatalf("Stat cache file failed: %v", err)
	}
	// On Unix, check permission is 0600
	if fi.Mode().Perm() != 0600 {
		t.Errorf("Cache file mode = %o, want 0600", fi.Mode().Perm())
	}

	dirFi, err := os.Stat(filepath.Dir(cacheFile))
	if err != nil {
		t.Fatalf("Stat cache dir failed: %v", err)
	}
	if dirFi.Mode().Perm() != 0700 {
		t.Errorf("Cache dir mode = %o, want 0700", dirFi.Mode().Perm())
	}

	loaded, err := LoadCache(cacheFile)
	if err != nil {
		t.Fatalf("LoadCache failed: %v", err)
	}

	if len(loaded.Documents) != 1 {
		t.Fatalf("Loaded doc count = %d, want 1", len(loaded.Documents))
	}
	doc := loaded.Documents["main.go"]
	if doc.SHA256 != "abc12345" {
		t.Errorf("Loaded doc SHA256 = %q, want abc12345", doc.SHA256)
	}
	if len(loaded.Postings["main"]) == 0 {
		t.Errorf("Loaded postings for 'main' missing")
	}
}

func TestCacheCorruptionRecovery(t *testing.T) {
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "corrupted_cache.json")

	// 1. Corrupt JSON content
	if err := os.WriteFile(cacheFile, []byte("{ invalid json content: true,,, "), 0600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadCache(cacheFile)
	if err == nil {
		t.Error("expected error loading corrupted json cache")
	}
	if loaded != nil {
		t.Error("expected nil index on corrupted cache load")
	}

	// 2. Empty cache file
	emptyFile := filepath.Join(tempDir, "empty_cache.json")
	if err := os.WriteFile(emptyFile, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = LoadCache(emptyFile)
	if err == nil {
		t.Error("expected error loading empty cache file")
	}

	// 3. Service recovery from corrupted cache
	repoDir := filepath.Join(tempDir, "repo")
	_ = os.MkdirAll(repoDir, 0755)
	mainGo := filepath.Join(repoDir, "main.go")
	_ = os.WriteFile(mainGo, []byte("package main\nfunc Recovered() {}\n"), 0644)

	svc := NewService(repoDir, DefaultConfig())
	svc.SetCachePath(cacheFile) // points to corrupted cache file

	// EnsureIndex must not fail when cache is corrupted; it must gracefully recover
	ctx := context.Background()
	// Initialize git in repoDir so DiscoverFiles works
	runGit := exec.Command("git", "init")
	runGit.Dir = repoDir
	_ = runGit.Run()

	err = svc.EnsureIndex(ctx)
	if err != nil {
		t.Fatalf("Service.EnsureIndex failed to recover from corrupted cache: %v", err)
	}

	idx := svc.GetIndex()
	if idx == nil {
		t.Fatal("expected non-nil index after recovery")
	}
	if len(idx.Documents) != 1 {
		t.Errorf("expected 1 document indexed after recovery, got %d", len(idx.Documents))
	}

	// 4. Cache Save validation
	if err := SaveCache("", idx); err == nil {
		t.Error("expected error saving to empty cache path")
	}
	if err := SaveCache(filepath.Join(tempDir, "valid.json"), nil); err == nil {
		t.Error("expected error saving nil index")
	}
}
