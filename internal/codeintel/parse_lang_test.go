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

func TestParseTypeScriptJavaScript_MultiLineSymbols(t *testing.T) {
	tempDir := t.TempDir()

	tsSrc := `// Line 1: header comment
export interface UserSession {
	id: string;
	roles: string[];
}

export type AuthConfig = {
	issuer: string;
	audience: string;
};

export enum SessionState {
	Active = "ACTIVE",
	Expired = "EXPIRED",
	Revoked = "REVOKED",
}

export class SessionService {
	// Method comment with unmatched brace }
	async verifyToken(token: string): Promise<boolean> {
		const msg = "unmatched braces } } in string { literal";
		/* block comment with } } and { */
		if (!token) {
			return false;
		}
		return msg.length > 0;
	}

	revokeSession(sessionId: string): void {
		const note = 'single quote } brace';
		console.log(note, sessionId);
	}
}

export async function authenticateUser(username: string): Promise<string> {
	const raw = "string with } brace";
	if (username === "") {
		return "";
	}
	return raw;
}

export const validatePayload = (payload: Record<string, unknown>): boolean => {
	// line comment with } }
	const check = "nested { and } inside string";
	return payload !== null && check.length > 0;
};
`
	tsPath := filepath.Join(tempDir, "auth_service.ts")
	if err := os.WriteFile(tsPath, []byte(tsSrc), 0644); err != nil {
		t.Fatal(err)
	}

	doc, err := ParseFile(tempDir, "auth_service.ts")
	if err != nil {
		t.Fatalf("ParseFile(auth_service.ts) failed: %v", err)
	}

	symByName := make(map[string]Symbol)
	for _, s := range doc.Symbols {
		symByName[s.Name] = s
	}

	expectedSpans := []struct {
		name      string
		startLine int
		endLine   int
	}{
		{name: "UserSession", startLine: 2, endLine: 5},
		{name: "AuthConfig", startLine: 7, endLine: 10},
		{name: "SessionState", startLine: 12, endLine: 16},
		{name: "SessionService", startLine: 18, endLine: 33},
		{name: "SessionService.verifyToken", startLine: 20, endLine: 27},
		{name: "SessionService.revokeSession", startLine: 29, endLine: 32},
		{name: "authenticateUser", startLine: 35, endLine: 41},
		{name: "validatePayload", startLine: 43, endLine: 47},
	}

	for _, exp := range expectedSpans {
		sym, ok := symByName[exp.name]
		if !ok {
			t.Errorf("missing expected TS symbol %q (extracted: %+v)", exp.name, doc.Symbols)
			continue
		}
		if sym.StartLine != exp.startLine {
			t.Errorf("symbol %q StartLine = %d, want %d", exp.name, sym.StartLine, exp.startLine)
		}
		if sym.EndLine != exp.endLine {
			t.Errorf("symbol %q EndLine = %d, want %d (must be > StartLine %d)", exp.name, sym.EndLine, exp.endLine, sym.StartLine)
		}
		if sym.EndLine <= sym.StartLine {
			t.Errorf("symbol %q EndLine (%d) must be greater than StartLine (%d)", exp.name, sym.EndLine, sym.StartLine)
		}
	}
}

func TestParsePython_IndentationMultiLineSymbols(t *testing.T) {
	tempDir := t.TempDir()

	pySrc := `# Line 1: module docstring
class LedgerProcessor:
    def __init__(self, currency: str):
        self.currency = currency
        self.entries = []

    async def commit_batch(self, batch_id: str) -> bool:
        if not batch_id:
            return False

        # Comment inside method
        self.entries.append(batch_id)
        return True


def reconcile_balances(accounts: list) -> int:
    total = 0
    for acc in accounts:
        total += acc
    return total


async def fetch_exchange_rate(pair: str) -> float:
    if pair == "USD-BRL":
        return 5.15
    return 1.0
`
	pyPath := filepath.Join(tempDir, "ledger.py")
	if err := os.WriteFile(pyPath, []byte(pySrc), 0644); err != nil {
		t.Fatal(err)
	}

	doc, err := ParseFile(tempDir, "ledger.py")
	if err != nil {
		t.Fatalf("ParseFile(ledger.py) failed: %v", err)
	}

	symByName := make(map[string]Symbol)
	for _, s := range doc.Symbols {
		symByName[s.Name] = s
	}

	expectedSpans := []struct {
		name      string
		startLine int
		endLine   int
	}{
		{name: "LedgerProcessor", startLine: 2, endLine: 13},
		{name: "LedgerProcessor.__init__", startLine: 3, endLine: 5},
		{name: "LedgerProcessor.commit_batch", startLine: 7, endLine: 13},
		{name: "reconcile_balances", startLine: 16, endLine: 20},
		{name: "fetch_exchange_rate", startLine: 23, endLine: 26},
	}

	for _, exp := range expectedSpans {
		sym, ok := symByName[exp.name]
		if !ok {
			t.Errorf("missing expected Python symbol %q (extracted: %+v)", exp.name, doc.Symbols)
			continue
		}
		if sym.StartLine != exp.startLine {
			t.Errorf("symbol %q StartLine = %d, want %d", exp.name, sym.StartLine, exp.startLine)
		}
		if sym.EndLine != exp.endLine {
			t.Errorf("symbol %q EndLine = %d, want %d", exp.name, sym.EndLine, exp.endLine)
		}
		if sym.EndLine <= sym.StartLine {
			t.Errorf("symbol %q EndLine (%d) must be > StartLine (%d)", exp.name, sym.EndLine, sym.StartLine)
		}
	}
}

func TestParseRustPHPJava_MultiLineSpans(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Rust
	rsSrc := `pub struct RetryPolicy {
    pub max_retries: u32,
    pub backoff_ms: u64,
}

pub fn execute_with_retry(attempts: u32) -> bool {
    if attempts == 0 {
        return false;
    }
    true
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "retry.rs"), []byte(rsSrc), 0644); err != nil {
		t.Fatal(err)
	}
	rsDoc, err := ParseFile(tempDir, "retry.rs")
	if err != nil {
		t.Fatalf("ParseFile(retry.rs) failed: %v", err)
	}
	rsSyms := make(map[string]Symbol)
	for _, s := range rsDoc.Symbols {
		rsSyms[s.Name] = s
	}
	if sym, ok := rsSyms["RetryPolicy"]; !ok {
		t.Errorf("Rust symbol RetryPolicy not found in %+v", rsDoc.Symbols)
	} else if sym.StartLine != 1 || sym.EndLine != 4 {
		t.Errorf("RetryPolicy span = L%d-L%d, want L1-L4", sym.StartLine, sym.EndLine)
	}
	if sym, ok := rsSyms["execute_with_retry"]; !ok {
		t.Errorf("Rust symbol execute_with_retry not found in %+v", rsDoc.Symbols)
	} else if sym.StartLine != 6 || sym.EndLine != 11 {
		t.Errorf("execute_with_retry span = L%d-L%d, want L6-L11", sym.StartLine, sym.EndLine)
	}

	// 2. PHP
	phpSrc := `<?php
class WebhookDispatcher {
    public function dispatchPayload($payload) {
        if (empty($payload)) {
            return false;
        }
        return true;
    }
}

function verifyWebhookSignature($sig, $secret) {
    if (!$sig) {
        return false;
    }
    return hash_equals($sig, $secret);
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "webhook.php"), []byte(phpSrc), 0644); err != nil {
		t.Fatal(err)
	}
	phpDoc, err := ParseFile(tempDir, "webhook.php")
	if err != nil {
		t.Fatalf("ParseFile(webhook.php) failed: %v", err)
	}
	phpSyms := make(map[string]Symbol)
	for _, s := range phpDoc.Symbols {
		phpSyms[s.Name] = s
	}
	if sym, ok := phpSyms["WebhookDispatcher"]; !ok {
		t.Errorf("PHP symbol WebhookDispatcher not found in %+v", phpDoc.Symbols)
	} else if sym.StartLine != 2 || sym.EndLine != 9 {
		t.Errorf("WebhookDispatcher span = L%d-L%d, want L2-L9", sym.StartLine, sym.EndLine)
	}
	if sym, ok := phpSyms["verifyWebhookSignature"]; !ok {
		t.Errorf("PHP symbol verifyWebhookSignature not found in %+v", phpDoc.Symbols)
	} else if sym.StartLine != 11 || sym.EndLine != 16 {
		t.Errorf("verifyWebhookSignature span = L%d-L%d, want L11-L16", sym.StartLine, sym.EndLine)
	}

	// 3. Java
	javaSrc := `package com.example.billing;

public class InvoiceGenerator {
    public String generateInvoice(String orderId) {
        if (orderId == null) {
            return "";
        }
        return "INV-" + orderId;
    }
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "InvoiceGenerator.java"), []byte(javaSrc), 0644); err != nil {
		t.Fatal(err)
	}
	javaDoc, err := ParseFile(tempDir, "InvoiceGenerator.java")
	if err != nil {
		t.Fatalf("ParseFile(InvoiceGenerator.java) failed: %v", err)
	}
	javaSyms := make(map[string]Symbol)
	for _, s := range javaDoc.Symbols {
		javaSyms[s.Name] = s
	}
	if sym, ok := javaSyms["InvoiceGenerator"]; !ok {
		t.Errorf("Java symbol InvoiceGenerator not found in %+v", javaDoc.Symbols)
	} else if sym.StartLine != 3 || sym.EndLine != 10 {
		t.Errorf("InvoiceGenerator span = L%d-L%d, want L3-L10", sym.StartLine, sym.EndLine)
	}
}

func TestEndToEndRetrieval_MultiLineSnippets(t *testing.T) {
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

	tsContent := `export class TokenAuthenticator {
	refreshAccessToken(refreshToken: string): string {
		const prefix = "bearer_";
		if (!refreshToken.startsWith(prefix)) {
			throw new Error("invalid_refresh_token");
		}
		return refreshToken.slice(prefix.length) + "_renewed";
	}
}
`
	pyContent := `def reconcile_ledger_entries(entries: list) -> int:
    settled_sum = 0
    for item in entries:
        if item > 0:
            settled_sum += item
    return settled_sum
`
	if err := os.WriteFile(filepath.Join(tempDir, "authenticator.ts"), []byte(tsContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "reconcile.py"), []byte(pyContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	cfg.Mode = ModeActive
	cfg.MaxContextBytes = 3500

	svc := NewService(tempDir, cfg)
	svc.SetCachePath(filepath.Join(tempDir, ".downshift", "cache", "e2e_lang_cache.json"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Query TypeScript class method
	packTS, telemTS := svc.ProcessPrompt(ctx, "How does TokenAuthenticator refreshAccessToken handle invalid_refresh_token?")
	if packTS == nil || !telemTS.InjectionApplied {
		t.Fatalf("Expected context injection for TS query, got pack=%v telem=%+v", packTS, telemTS)
	}
	if !strings.Contains(packTS.FormattedText, "TokenAuthenticator.refreshAccessToken") {
		t.Errorf("Expected TokenAuthenticator.refreshAccessToken in pack, got:\n%s", packTS.FormattedText)
	}
	if !strings.Contains(packTS.FormattedText, `throw new Error("invalid_refresh_token")`) {
		t.Errorf("Expected multi-line TS body line in snippet, got:\n%s", packTS.FormattedText)
	}
	if !strings.Contains(packTS.FormattedText, `return refreshToken.slice(prefix.length) + "_renewed"`) {
		t.Errorf("Expected final TS body line in snippet, got:\n%s", packTS.FormattedText)
	}

	// 2. Query Python function
	packPy, telemPy := svc.ProcessPrompt(ctx, "How does reconcile_ledger_entries compute settled_sum?")
	if packPy == nil || !telemPy.InjectionApplied {
		t.Fatalf("Expected context injection for Python query, got pack=%v telem=%+v", packPy, telemPy)
	}
	if !strings.Contains(packPy.FormattedText, "reconcile_ledger_entries") {
		t.Errorf("Expected reconcile_ledger_entries in pack, got:\n%s", packPy.FormattedText)
	}
	if !strings.Contains(packPy.FormattedText, "settled_sum += item") {
		t.Errorf("Expected multi-line Python body in snippet, got:\n%s", packPy.FormattedText)
	}
	if !strings.Contains(packPy.FormattedText, "return settled_sum") {
		t.Errorf("Expected final Python return line in snippet, got:\n%s", packPy.FormattedText)
	}
}
