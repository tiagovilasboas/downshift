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
	"sort"
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

// NewDashboardServer creates a server that serves both the API and the SPA.
func NewDashboardServer(port int) (*Server, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dashboard: listen %s: %w", addr, err)
	}

	mux := http.NewServeMux()
	s := &Server{
		listener: listener,
		server:   &http.Server{Handler: withCORS(mux)},
	}

	// API routes
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("GET /api/session/current", s.handleCurrentSession)
	mux.HandleFunc("GET /api/session/{id}", s.handleSession)
	mux.HandleFunc("GET /api/health", s.handleHealth)

	// SPA routes - serve embedded assets or fallback to index.html
	mux.Handle("/", spaHandler())

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
	if e2eStubEnabled(r) {
		writeJSON(w, e2ePanelStub())
		return
	}
	events := readDashboardEvents()
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
	events := readDashboardEvents()
	hash, err := sessionFromID(id, events)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadRequest)
		return
	}
	s.writePanel(w, events, hash)
}

func (s *Server) writePanel(w http.ResponseWriter, events []telemetry.Event, hash string) {
	writeJSON(w, buildPanelOut(events, hash, execLogPath()))
}

// buildPanelOut is one session's panel: the routing view, the session's
// Compactor block and the final tokens-saved figure.
func buildPanelOut(events []telemetry.Event, hash, execPath string) panelOut {
	v := telemetry.BuildPanel(events, hash)
	comp := sessionCompactor(execPath, hash)
	return panelOut{
		PanelView:           v,
		Compactor:           comp,
		TokensSavedEstimate: telemetry.TokensSavedEstimate(v.Tokens, compactorTokens(comp)),
	}
}

// readDashboardEvents is the event window every endpoint reads, so the session
// list and the session panel count the same events.
func readDashboardEvents() []telemetry.Event {
	events, _ := telemetry.ReadEventsTail(telemetry.DefaultTailBytes)
	return events
}

// execLogPath is the exec log path, or "" when the state directory is unknown.
func execLogPath() string {
	path, err := execwrap.EventsPath()
	if err != nil {
		return ""
	}
	return path
}

// sessionCompactor is the Compactor block of one session: the exec-log runs
// that carry its hash. Nil without a hash or without a run, never zeros.
func sessionCompactor(execPath, hash string) *compactorPanel {
	if execPath == "" || hash == "" {
		return nil
	}
	sum, err := execwrap.SummarizeSession(execPath, hash)
	if err != nil || sum.Commands == 0 {
		return nil
	}
	scope := hash
	if len(scope) > 8 {
		scope = scope[:8]
	}
	return &compactorPanel{
		Scope:                scope,
		Commands:             sum.Commands,
		Compacted:            sum.Compacted,
		RawBytes:             sum.RawBytes,
		SavedBytes:           sum.SavedBytes,
		EstimatedTokensSaved: sum.EstimatedTokensSaved,
		TokenEstimate:        sum.TokenEstimate,
	}
}

func compactorTokens(c *compactorPanel) int64 {
	if c == nil {
		return 0
	}
	return c.EstimatedTokensSaved
}

// writeJSON writes v as a JSON response with the dashboard content type.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
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
// The counts are the session panel's (telemetry.PanelView), so a row and its
// panel always agree: Switched = ModelSwitched + EffortChanged, and Held =
// HeldGuardrail + UpshiftNotApplied among the held spawns.
type sessionSummary struct {
	SessionID         string       `json:"session_id"`
	Harness           string       `json:"harness"`
	FirstSeen         string       `json:"first_seen"`
	LastSeen          string       `json:"last_seen"`
	SpawnCount        int          `json:"spawn_count"`
	Switched          int          `json:"switched"`
	ModelSwitched     int          `json:"model_switched"`
	EffortChanged     int          `json:"effort_changed"`
	Held              int          `json:"held"`
	HeldGuardrail     int          `json:"held_guardrail"`
	UpshiftNotApplied int          `json:"upshift_not_applied"`
	TokensSaved       int64        `json:"tokens_saved"`
	Tokens            tokensDetail `json:"tokens"`
}

// tokensDetail exposes routing and compactor savings for one session.
type tokensDetail struct {
	Routing   telemetry.TokenSaving `json:"routing"`
	Compactor *compactorPanel       `json:"compactor,omitempty"`
}

// sessionsResponse is the /api/sessions response.
type sessionsResponse struct {
	Sessions  []sessionSummary `json:"sessions"`
	Harnesses []string         `json:"harnesses"`
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if e2eStubEnabled(r) {
		writeJSON(w, e2eSessionsStub())
		return
	}
	writeJSON(w, buildSessionsResponse(readDashboardEvents(), execLogPath()))
}

// buildSessionsResponse lists the sessions in events, most recent first. Each
// row takes its counts and tokens from that session's panel, and its Compactor
// block from the same per-session exec-log scope as /api/session/{id}.
func buildSessionsResponse(events []telemetry.Event, execPath string) sessionsResponse {
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
		if ev.Timestamp > sum.LastSeen {
			sum.LastSeen = ev.Timestamp
		}
		if ev.Timestamp < sum.FirstSeen {
			sum.FirstSeen = ev.Timestamp
		}
		if telemetry.IsDecisionRow(ev) {
			sum.SpawnCount++
		}
		harnessSet[ev.Harness] = true
	}

	var list []sessionSummary
	for id, sum := range sessions {
		panel := telemetry.BuildPanel(events, id)
		sum.Switched = panel.Switched
		sum.ModelSwitched = panel.ModelSwitched
		sum.EffortChanged = panel.EffortChanged
		sum.Held = panel.Held
		sum.HeldGuardrail = panel.HeldGuardrail
		sum.UpshiftNotApplied = panel.UpshiftNotApplied

		sum.Tokens.Routing = panel.Tokens
		sum.Tokens.Compactor = sessionCompactor(execPath, id)
		sum.TokensSaved = telemetry.TokensSavedEstimate(panel.Tokens, compactorTokens(sum.Tokens.Compactor))
		list = append(list, *sum)
	}

	// Most recent first; equal times fall back to the id so the order is stable.
	sort.Slice(list, func(i, j int) bool {
		if list[i].LastSeen != list[j].LastSeen {
			return list[i].LastSeen > list[j].LastSeen
		}
		return list[i].SessionID < list[j].SessionID
	})

	var harnesses []string
	for h := range harnessSet {
		harnesses = append(harnesses, h)
	}
	return sessionsResponse{Sessions: list, Harnesses: harnesses}
}
