// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/codeintel"
)

func TestDCIHookFailOpenScenarios(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// 1. Non-supported harness exits 0 immediately
	code := runDCIHook("cursor", strings.NewReader(""), &stdout, &stderr)
	if code != 0 || stdout.Len() != 0 {
		t.Errorf("expected exit 0 and empty output for non-supported harness, got code=%d out=%s", code, stdout.String())
	}

	// 2. Empty stdin exits 0 with empty output
	stdout.Reset()
	code = runDCIHook("claude-code", strings.NewReader(""), &stdout, &stderr)
	if code != 0 || stdout.Len() != 0 {
		t.Errorf("expected exit 0 and empty output for empty stdin, got code=%d out=%s", code, stdout.String())
	}

	// 3. Malformed JSON exits 0 with empty output
	stdout.Reset()
	code = runDCIHook("claude-code", strings.NewReader("{invalid json"), &stdout, &stderr)
	if code != 0 || stdout.Len() != 0 {
		t.Errorf("expected exit 0 and empty output for malformed json, got code=%d out=%s", code, stdout.String())
	}
}

func TestDCIHookActivePipeline(t *testing.T) {
	tempDir := t.TempDir()

	// Create sample go source in tempDir
	srcFile := filepath.Join(tempDir, "payment.go")
	if err := os.WriteFile(srcFile, []byte("package payment\n\n// ProcessPIXPayment handles PIX transactions\nfunc ProcessPIXPayment(amount int) error {\n\treturn nil\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Enable DCI in active mode
	if err := codeintel.SetMode(tempDir, codeintel.ModeActive); err != nil {
		t.Fatal(err)
	}

	// 1. Greeting prompt -> should abstain (exit 0, empty output)
	greetingPayload := map[string]string{
		"hook_event_name": "UserPromptSubmit",
		"cwd":             tempDir,
		"prompt":          "Good morning!",
	}
	payloadBytes, _ := json.Marshal(greetingPayload)

	var stdout, stderr bytes.Buffer
	code := runDCIHook("claude-code", bytes.NewReader(payloadBytes), &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit 0 on greeting, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty output on greeting abstention, got: %s", stdout.String())
	}

	// 2. Relevant code query -> should inject context
	codePayload := map[string]string{
		"hook_event_name": "UserPromptSubmit",
		"cwd":             tempDir,
		"prompt":          "How does ProcessPIXPayment handle transactions?",
	}
	payloadBytes, _ = json.Marshal(codePayload)

	stdout.Reset()
	stderr.Reset()
	code = runDCIHook("claude-code", bytes.NewReader(payloadBytes), &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit 0 on code query, got %d", code)
	}

	var resp userPromptSubmitOutput
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal hook output: %v. Raw: %s", err, stdout.String())
	}

	if resp.HookSpecificOutput.HookEventName != "UserPromptSubmit" {
		t.Errorf("expected HookEventName 'UserPromptSubmit', got %q", resp.HookSpecificOutput.HookEventName)
	}

	ctxText := resp.HookSpecificOutput.AdditionalContext
	if !strings.Contains(ctxText, codeintel.UntrustedContextSecurityNotice) {
		t.Error("expected untrusted context security notice in additionalContext")
	}
	if !strings.Contains(ctxText, "payment.go") {
		t.Error("expected payment.go in additionalContext")
	}
	if !strings.Contains(ctxText, "ProcessPIXPayment") {
		t.Error("expected ProcessPIXPayment symbol in additionalContext")
	}
}
