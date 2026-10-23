// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	srv, err := NewServer(0)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Use httptest to test the handler directly
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health: got status %d, want 200", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("health: invalid JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("health: got status %q, want %q", resp["status"], "ok")
	}
}

func TestCurrentSessionEndpoint(t *testing.T) {
	srv, err := NewServer(0)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/session/current", nil)
	w := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("session/current: got status %d, want 200", w.Code)
	}

	// Should return valid JSON even with no session data
	var resp panelOut
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("session/current: invalid JSON: %v", err)
	}
}

func TestCORSHeaders(t *testing.T) {
	srv, err := NewServer(0)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Test preflight
	req := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("CORS preflight: got status %d, want 204", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("CORS: got origin %q, want %q", got, "http://localhost:5173")
	}
}

func TestServerAddr(t *testing.T) {
	srv, err := NewServer(0)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	addr := srv.Addr()
	if addr == "" {
		t.Error("Addr: got empty address")
	}

	port := srv.Port()
	if port == 0 {
		t.Error("Port: got 0")
	}
}

func TestSessionsEndpointTokensField(t *testing.T) {
	srv, err := NewServer(0)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	w := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("sessions: got status %d, want 200", w.Code)
	}

	var resp sessionsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("sessions: invalid JSON: %v", err)
	}

	// Validate that each session has the tokens field with routing and optional compactor
	for _, s := range resp.Sessions {
		// Routing field must be present (even if zero)
		if s.Tokens.Routing.Children < 0 {
			t.Errorf("session %s: routing.children is negative", s.SessionID)
		}
		// TokensSaved must be computed from routing + compactor
		if s.TokensSaved < 0 {
			t.Errorf("session %s: tokens_saved is negative", s.SessionID)
		}
		// Compactor is optional (nil when no exec events for this session)
	}
}
