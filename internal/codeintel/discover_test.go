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

func TestDiscoverFiles(t *testing.T) {
	tempDir := t.TempDir()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v, out: %s", args, err, string(out))
		}
	}

	runGit("init")
	runGit("config", "user.name", "Test User")
	runGit("config", "user.email", "test@example.com")

	createFile := func(relPath string, content []byte) {
		t.Helper()
		fullPath := filepath.Join(tempDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(fullPath, content, 0644); err != nil {
			t.Fatalf("write file failed: %v", err)
		}
	}

	createFile("main.go", []byte("package main\n\nfunc main() {}\n"))
	createFile("pkg/util.go", []byte("package pkg\n\nfunc Helper() string { return \"\" }\n"))
	createFile("node_modules/foo.js", []byte("console.log('excluded');"))
	createFile("vendor/dep.go", []byte("package vendor"))
	createFile(".env", []byte("SECRET_KEY=12345"))
	createFile("config/api_token.json", []byte("{\"token\": \"xyz\"}"))
	createFile("assets/image.png", []byte("PNG header dummy"))
	createFile("binary_with_null.dat", []byte("abc\x00def"))
	createFile("oversized.txt", make([]byte, 1024)) // 1024 bytes

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := Config{
		MaxFiles:         100,
		MaxFileSizeBytes: 500, // 500 bytes -> oversized.txt will be skipped
		MaxRunBytes:      1024 * 1024,
		Excludes:         DefaultConfig().Excludes,
	}

	files, err := DiscoverFiles(ctx, tempDir, cfg)
	if err != nil {
		t.Fatalf("DiscoverFiles failed: %v", err)
	}

	fileMap := make(map[string]bool)
	for _, f := range files {
		fileMap[f.RelativePath] = true
	}

	if !fileMap["main.go"] {
		t.Errorf("expected main.go to be discovered")
	}
	if !fileMap["pkg/util.go"] {
		t.Errorf("expected pkg/util.go to be discovered")
	}

	// Check excluded
	if fileMap["node_modules/foo.js"] {
		t.Errorf("node_modules/foo.js should be excluded")
	}
	if fileMap["vendor/dep.go"] {
		t.Errorf("vendor/dep.go should be excluded")
	}
	if fileMap[".env"] {
		t.Errorf(".env should be excluded")
	}
	if fileMap["config/api_token.json"] {
		t.Errorf("config/api_token.json should be excluded")
	}
	if fileMap["assets/image.png"] {
		t.Errorf("assets/image.png should be excluded")
	}
	if fileMap["binary_with_null.dat"] {
		t.Errorf("binary_with_null.dat should be excluded")
	}
	if fileMap["oversized.txt"] {
		t.Errorf("oversized.txt should be excluded due to size limit")
	}
}

func TestDiscoverFiles_Boundaries(t *testing.T) {
	tempDir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v, out: %s", err, string(out))
	}

	for i := 0; i < 10; i++ {
		p := filepath.Join(tempDir, filepath.Clean(filepath.Join("pkg", string(rune('a'+i))+".go")))
		_ = os.MkdirAll(filepath.Dir(p), 0755)
		_ = os.WriteFile(p, []byte("package pkg\nfunc F() {}\n"), 0644)
	}

	ctx := context.Background()

	// 1. MaxFiles Limit
	cfgFiles := Config{
		MaxFiles:         4,
		MaxFileSizeBytes: 1024,
		MaxRunBytes:      1024 * 1024,
		Excludes:         DefaultConfig().Excludes,
	}
	files, err := DiscoverFiles(ctx, tempDir, cfgFiles)
	if err != nil {
		t.Fatalf("DiscoverFiles failed: %v", err)
	}
	if len(files) != 4 {
		t.Errorf("expected 4 files with MaxFiles=4, got %d", len(files))
	}

	// 2. MaxRunBytes Limit
	// Each file is ~24 bytes. MaxRunBytes = 50 should allow ~2 files.
	cfgBytes := Config{
		MaxFiles:         100,
		MaxFileSizeBytes: 1024,
		MaxRunBytes:      50,
		Excludes:         DefaultConfig().Excludes,
	}
	filesBytes, err := DiscoverFiles(ctx, tempDir, cfgBytes)
	if err != nil {
		t.Fatalf("DiscoverFiles failed: %v", err)
	}
	if len(filesBytes) > 3 {
		t.Errorf("expected <= 3 files with MaxRunBytes=50, got %d", len(filesBytes))
	}

	// 3. Empty repository root
	_, err = DiscoverFiles(ctx, "", cfgFiles)
	if err == nil {
		t.Error("expected error with empty repository root")
	}

	// 4. Nonexistent root
	_, err = DiscoverFiles(ctx, filepath.Join(tempDir, "does-not-exist"), cfgFiles)
	if err == nil {
		t.Error("expected error with nonexistent repository root")
	}

	// 5. Cancelled context
	cancCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = DiscoverFiles(cancCtx, tempDir, cfgFiles)
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}
