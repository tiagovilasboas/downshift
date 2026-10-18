// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServicePipeline(t *testing.T) {
	tempDir := t.TempDir()

	// Init git repo
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

	// Create source code files
	createFile := func(relPath, content string) {
		t.Helper()
		full := filepath.Join(tempDir, relPath)
		_ = os.MkdirAll(filepath.Dir(full), 0755)
		_ = os.WriteFile(full, []byte(content), 0644)
	}

	createFile("pkg/payment/service.go", `package payment

// PaymentProcessor handles transactions.
type PaymentProcessor struct {}

func ProcessPayment(amount int) error {
	return nil
}
`)

	cfg := DefaultConfig()
	cfg.Mode = ModeActive
	cfg.MaxContextBytes = 2000

	svc := NewService(tempDir, cfg)
	svc.SetCachePath(filepath.Join(tempDir, ".downshift", "cache", "test_cache.json"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Test Mode Off
	svc.config.Mode = ModeOff
	pack, telem := svc.ProcessPrompt(ctx, "How does ProcessPayment handle amount?")
	if pack != nil {
		t.Error("Expected nil pack in ModeOff")
	}
	if telem.InjectionApplied {
		t.Error("InjectionApplied should be false in ModeOff")
	}

	// 2. Test Greeting (Abstention)
	svc.config.Mode = ModeActive
	pack, telem = svc.ProcessPrompt(ctx, "Hello there!")
	if pack != nil {
		t.Error("Expected nil pack for greeting")
	}
	if telem.AbstentionReason != "conversational_greeting" {
		t.Errorf("AbstentionReason = %q, want conversational_greeting", telem.AbstentionReason)
	}

	// 3. Test Active Mode with Relevant Code Prompt
	pack, telem = svc.ProcessPrompt(ctx, "Where is ProcessPayment implemented?")
	if pack == nil {
		t.Fatal("Expected non-nil pack for relevant code prompt")
	}
	if !telem.InjectionApplied {
		t.Error("Expected InjectionApplied = true")
	}
	if telem.SelectedFiles == 0 {
		t.Error("Expected at least 1 selected file")
	}
	if !strings.Contains(pack.FormattedText, "pkg/payment/service.go") {
		t.Errorf("FormattedText does not mention pkg/payment/service.go: %s", pack.FormattedText)
	}
	if !strings.Contains(pack.FormattedText, UntrustedContextStartMarker) {
		t.Error("FormattedText missing UntrustedContextStartMarker")
	}
	if !strings.Contains(pack.FormattedText, UntrustedContextEndMarker) {
		t.Error("FormattedText missing UntrustedContextEndMarker")
	}

	// 4. Test Observe Mode (Metrics recorded, but no injected context returned)
	svc.config.Mode = ModeObserve
	pack, telem = svc.ProcessPrompt(ctx, "Where is ProcessPayment implemented?")
	if pack != nil {
		t.Error("Expected nil pack in ModeObserve")
	}
	if telem.InjectionApplied {
		t.Error("InjectionApplied should be false in ModeObserve")
	}
	if telem.CandidateCount == 0 {
		t.Error("CandidateCount should be recorded in ModeObserve")
	}

	// 5. Test Context Timeout / Cancellation Fail-Open
	canceledCtx, cancelNow := context.WithCancel(context.Background())
	cancelNow() // cancel immediately
	svc.config.Mode = ModeActive
	pack, telem = svc.ProcessPrompt(canceledCtx, "ProcessPayment")
	if pack != nil {
		t.Error("Expected nil pack on canceled context")
	}
	if telem.ErrorCode != "context_cancelled" {
		t.Errorf("ErrorCode = %q, want context_cancelled", telem.ErrorCode)
	}
}
