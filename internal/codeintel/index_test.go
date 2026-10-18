// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInvertedIndexIncrementalUpdate(t *testing.T) {
	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "auth.go")
	if err := os.WriteFile(file1, []byte("package auth\nfunc Authenticate() bool { return true }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	info1, _ := os.Stat(file1)

	file2 := filepath.Join(tempDir, "user.go")
	if err := os.WriteFile(file2, []byte("package auth\ntype User struct { ID string }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	info2, _ := os.Stat(file2)

	idx := NewInvertedIndex()
	discovered := []DiscoveredFile{
		{RelativePath: "auth.go", FullPath: file1, Size: info1.Size(), ModTime: info1.ModTime()},
		{RelativePath: "user.go", FullPath: file2, Size: info2.Size(), ModTime: info2.ModTime()},
	}

	if err := idx.UpdateIncremental(tempDir, discovered); err != nil {
		t.Fatalf("UpdateIncremental failed: %v", err)
	}

	if len(idx.Documents) != 2 {
		t.Fatalf("Expected 2 documents, got %d", len(idx.Documents))
	}

	if len(idx.Postings["authenticate"]) == 0 {
		t.Errorf("Expected posting for 'authenticate'")
	}
	if len(idx.Postings["user"]) == 0 {
		t.Errorf("Expected posting for 'user'")
	}

	// Test incremental update: modify file1, delete file2, add file3
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(file1, []byte("package auth\nfunc AuthenticateOAuth() bool { return true }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	info1Modified, _ := os.Stat(file1)

	file3 := filepath.Join(tempDir, "token.go")
	if err := os.WriteFile(file3, []byte("package auth\nfunc ValidateToken() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	info3, _ := os.Stat(file3)

	discoveredUpdated := []DiscoveredFile{
		{RelativePath: "auth.go", FullPath: file1, Size: info1Modified.Size(), ModTime: info1Modified.ModTime()},
		{RelativePath: "token.go", FullPath: file3, Size: info3.Size(), ModTime: info3.ModTime()},
	}

	if err := idx.UpdateIncremental(tempDir, discoveredUpdated); err != nil {
		t.Fatalf("UpdateIncremental failed on step 2: %v", err)
	}

	if len(idx.Documents) != 2 {
		t.Fatalf("Expected 2 documents after purge, got %d", len(idx.Documents))
	}
	if _, exists := idx.Documents["user.go"]; exists {
		t.Errorf("user.go should have been purged")
	}
	if _, exists := idx.Documents["token.go"]; !exists {
		t.Errorf("token.go should have been added")
	}
	if len(idx.Postings["oauth"]) == 0 {
		t.Errorf("Expected posting for 'oauth' after modifying auth.go")
	}
}
