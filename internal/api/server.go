// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package api exposes a REST API for the Downshift dashboard.
// It serves the same data as `downshift session --panel` over HTTP.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Server is the HTTP server for the dashboard API.
type Server struct {
	listener net.Listener
	server   *http.Server
}

// panelOut mirrors cmd/downshift/sessionpanel.go for the API response.
// Keeping it here avoids import cycles.
type panelOut struct {
	telemetry.PanelView
	Compactor           *compactorPanel `json:"compactor,omitempty"`
	TokensSavedEstimate int64           `json:"tokens_saved_estimate"`
}

type compactorPanel struct {
	Scope                string `json:"scope"`
	Commands             int64  `json:"commands"`
	Compacted            int64  `json:"compacted"`
	RawBytes             int64  `json:"raw_bytes"`
	SavedBytes           int64  `json:"saved_bytes"`
	EstimatedTokensSaved int64  `json:"estimated_tokens_saved"`
	TokenEstimate        string `json:"token_estimate"`
}

// NewServer creates a server on the given port. Port 0 picks an available port.
func NewServer(port int) (*Server, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("api: listen %s: %w", addr, err)
	}

	mux := http.NewServeMux()
	s := &Server{
		listener: listener,
		server:   &http.Server{Handler: withCORS(mux)},
	}

	// Routes
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("GET /api/session/current", s.handleCurrentSession)
	mux.HandleFunc("GET /api/session/{id}", s.handleSession)
	mux.HandleFunc("GET /api/health", s.handleHealth)

	return s, nil
}

// Addr returns the address the server is listening on.
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

// Port returns the port the server is listening on.
func (s *Server) Port() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

// Start begins serving requests. It blocks until Shutdown is called.
func (s *Server) Start() error {
	return s.server.Serve(s.listener)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// withCORS adds CORS headers for the dashboard (running on a different port).
func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Allow localhost origins (dashboard dev server)
		if origin != "" && (strings.HasPrefix(origin, "http://localhost:") ||
			strings.HasPrefix(origin, "http://127.0.0.1:") ||
			strings.HasPrefix(origin, "http://192.168.")) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleCurrentSession(w http.ResponseWriter, r *http.Request) {
	events, _ := telemetry.ReadEventsTail(telemetry.DefaultTailBytes)
	hash := latestSessionHash(events)
	if hash == "" {
		// Check environment for session ID from any harness
		hash = envSessionHash()
	}
	s.writePanel(w, events, hash)
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"session id required"}`, http.StatusBadRequest)
		return
	}
	events, _ := telemetry.ReadEventsTail(telemetry.DefaultTailBytes)
	hash, err := sessionFromID(id, events)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadRequest)
		return
	}
	s.writePanel(w, events, hash)
}

func (s *Server) writePanel(w http.ResponseWriter, events []telemetry.Event, hash string) {
	v := telemetry.BuildPanel(events, hash)
	out := panelOut{PanelView: v, TokensSavedEstimate: telemetry.TokensSavedEstimate(v.Tokens, 0)}

	// Add compactor data
	path, err := execwrap.EventsPath()
	if err == nil {
		if sum, err := execwrap.Summarize(path); err == nil && sum.Commands > 0 {
			out.Compactor = &compactorPanel{
				Scope:                "all",
				Commands:             sum.Commands,
				Compacted:            sum.Compacted,
				RawBytes:             sum.RawBytes,
				SavedBytes:           sum.SavedBytes,
				EstimatedTokensSaved: sum.EstimatedTokensSaved,
				TokenEstimate:        sum.TokenEstimate,
			}
			out.TokensSavedEstimate = telemetry.TokensSavedEstimate(v.Tokens, sum.EstimatedTokensSaved)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// latestSessionHash returns the most recent session hash from the events.
func latestSessionHash(events []telemetry.Event) string {
	return telemetry.LatestSessionHash(events)
}

// envSessionHash checks environment variables from known harnesses.
func envSessionHash() string {
	// Check common harness session env vars
	envVars := []string{
		"CLAUDE_CODE_SESSION_ID",
		"CURSOR_SESSION_ID",
		"CODEX_SESSION_ID",
		"ANTIGRAVITY_SESSION_ID",
	}
	for _, env := range envVars {
		if raw := os.Getenv(env); raw != "" {
			if h := telemetry.HashSessionID(raw); h != "" {
				return h
			}
		}
	}
	return ""
}

// sessionFromID resolves a session ID (raw, hash, or prefix) to a full hash.
func sessionFromID(v string, events []telemetry.Event) (string, error) {
	if telemetry.ValidSessionHash(v) {
		return v, nil
	}
	if h := telemetry.HashSessionID(v); h != "" && !isHexPrefix(v) {
		return h, nil
	}
	if isHexPrefix(v) && len(v) >= 8 {
		found := ""
		for _, ev := range events {
			if telemetry.ValidSessionHash(ev.SessionID) && strings.HasPrefix(ev.SessionID, v) {
				if found != "" && found != ev.SessionID {
					return "", fmt.Errorf("session prefix %q is ambiguous", v)
				}
				found = ev.SessionID
			}
		}
		if found != "" {
			return found, nil
		}
	}
	if h := telemetry.HashSessionID(v); h != "" {
		return h, nil
	}
	return "", fmt.Errorf("invalid session id %q", v)
}

func isHexPrefix(v string) bool {
	if v == "" {
		return false
	}
	for _, c := range v {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// sessionSummary is the metadata for one session in the /api/sessions list.
type sessionSummary struct {
	SessionID   string `json:"session_id"`
	Harness     string `json:"harness"`
	FirstSeen   string `json:"first_seen"`
	LastSeen    string `json:"last_seen"`
	SpawnCount  int    `json:"spawn_count"`
	Switched    int    `json:"switched"`
	Held        int    `json:"held"`
	TokensSaved int64  `json:"tokens_saved"`
}

// sessionsResponse is the /api/sessions response.
type sessionsResponse struct {
	Sessions  []sessionSummary `json:"sessions"`
	Harnesses []string         `json:"harnesses"`
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	events, _ := telemetry.ReadEventsTail(telemetry.DefaultTailBytes)

	// Extract unique sessions with their harness
	sessions := make(map[string]*sessionSummary)
	harnessSet := make(map[string]bool)

	for _, ev := range events {
		if !telemetry.ValidSessionHash(ev.SessionID) {
			continue
		}

		sum, exists := sessions[ev.SessionID]
		if !exists {
			sum = &sessionSummary{
				SessionID: ev.SessionID,
				Harness:   ev.Harness,
				FirstSeen: ev.Timestamp,
				LastSeen:  ev.Timestamp,
			}
			sessions[ev.SessionID] = sum
		}

		// Update last seen
		if ev.Timestamp > sum.LastSeen {
			sum.LastSeen = ev.Timestamp
		}
		if ev.Timestamp < sum.FirstSeen {
			sum.FirstSeen = ev.Timestamp
		}

		// Count spawns
		if telemetry.IsDecisionRow(ev) {
			sum.SpawnCount++
			switch ev.Verdict {
			case "DOWNSHIFT":
				sum.Switched++
			case "OK", "UPSHIFT":
				if ev.Outcome == "allow" {
					sum.Held++
				}
			}
		}

		harnessSet[ev.Harness] = true
	}

	// Build response
	var list []sessionSummary
	for _, sum := range sessions {
		// Calculate tokens saved for this session
		panel := telemetry.BuildPanel(events, sum.SessionID)
		sum.TokensSaved = telemetry.TokensSavedEstimate(panel.Tokens, 0)
		list = append(list, *sum)
	}

	// Sort by last seen (most recent first)
	for i := 0; i < len(list)-1; i++ {
		for j := i + 1; j < len(list); j++ {
			if list[i].LastSeen < list[j].LastSeen {
				list[i], list[j] = list[j], list[i]
			}
		}
	}

	// Build harness list
	var harnesses []string
	for h := range harnessSet {
		harnesses = append(harnesses, h)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessionsResponse{Sessions: list, Harnesses: harnesses})
}
