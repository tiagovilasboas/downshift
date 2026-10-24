// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestE2EStubSessions is the server-layer sensor for the dashboard E2E harness
// (B-W2 / DASH-2): ?e2e=1 must return the fixed sessions listing, so the
// Playwright render is deterministic and never reads the user's events.jsonl.
func TestE2EStubSessions(t *testing.T) {
	srv, err := NewServer(0)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sessions?e2e=1", nil)
	w := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("sessions?e2e=1: got status %d, want 200", w.Code)
	}

	var resp sessionsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("sessions?e2e=1: invalid JSON: %v", err)
	}
	if len(resp.Sessions) != 2 {
		t.Fatalf("sessions?e2e=1: got %d sessions, want 2", len(resp.Sessions))
	}
	if len(resp.Harnesses) != 2 {
		t.Fatalf("sessions?e2e=1: got %d harnesses, want 2", len(resp.Harnesses))
	}
	if resp.Sessions[0].Harness != "claude-code" {
		t.Errorf("sessions?e2e=1: got first harness %q, want %q", resp.Sessions[0].Harness, "claude-code")
	}
}

// TestE2EStubPanel proves ?e2e=1 on the current-session endpoint returns the
// fixed panel, including a compactor block so every dashboard tab has content.
func TestE2EStubPanel(t *testing.T) {
	srv, err := NewServer(0)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/session/current?e2e=1", nil)
	w := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("session/current?e2e=1: got status %d, want 200", w.Code)
	}

	var resp sessionPanelResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("session/current?e2e=1: invalid JSON: %v", err)
	}
	if resp.Switched != 3 {
		t.Errorf("session/current?e2e=1: got switched %d, want 3", resp.Switched)
	}
	if resp.Compactor == nil {
		t.Fatal("session/current?e2e=1: compactor block missing")
	}
	if resp.Compactor.Commands != 12 {
		t.Errorf("session/current?e2e=1: got compactor commands %d, want 12", resp.Compactor.Commands)
	}
}

// TestE2EStubIsDeterministic proves two stub requests return identical bytes.
func TestE2EStubIsDeterministic(t *testing.T) {
	srv, err := NewServer(0)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	get := func(path string) string {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(w, req)
		return w.Body.String()
	}

	if a, b := get("/api/sessions?e2e=1"), get("/api/sessions?e2e=1"); a != b {
		t.Error("sessions?e2e=1: stub is not byte-for-byte deterministic")
	}
	if a, b := get("/api/session/current?e2e=1"), get("/api/session/current?e2e=1"); a != b {
		t.Error("session/current?e2e=1: stub is not byte-for-byte deterministic")
	}
}

// TestE2EStubOffByDefault proves a request without the flag never serves the
// stub: the sessions listing without ?e2e=1 must not carry the fixed harnesses.
func TestE2EStubOffByDefault(t *testing.T) {
	if e2eStubEnabled(httptest.NewRequest(http.MethodGet, "/api/sessions", nil)) {
		t.Error("e2eStubEnabled: true without ?e2e=1")
	}
	if e2eStubEnabled(httptest.NewRequest(http.MethodGet, "/api/sessions?e2e=0", nil)) {
		t.Error("e2eStubEnabled: true with ?e2e=0")
	}
	if !e2eStubEnabled(httptest.NewRequest(http.MethodGet, "/api/sessions?e2e=1", nil)) {
		t.Error("e2eStubEnabled: false with ?e2e=1")
	}
}
