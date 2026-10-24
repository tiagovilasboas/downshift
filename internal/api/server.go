// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package api exposes a REST API for the Downshift dashboard. Every figure it
// returns is rendered from accounting.Compute, the same numbers as
// `downshift report --json` and `downshift session --panel`.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// Server is the HTTP server for the dashboard API.
type Server struct {
	listener net.Listener
	server   *http.Server
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
	s.registerAPI(mux)
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
	s.registerAPI(mux)
	// SPA routes - serve embedded assets or fallback to index.html
	mux.Handle("/", spaHandler())
	return s, nil
}

func (s *Server) registerAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("GET /api/session/current", s.handleCurrentSession)
	mux.HandleFunc("GET /api/session/{id}", s.handleSession)
	mux.HandleFunc("GET /api/health", s.handleHealth)
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
	writeJSON(w, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if e2eStubEnabled(r) {
		writeJSON(w, e2eSessionsStub())
		return
	}
	events := readDashboardEvents()
	execEvents := readExecEvents()
	writeJSON(w, buildSessionsResponse(events, execEvents, loadReport(execEvents)))
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
	execEvents := readExecEvents()
	writeJSON(w, buildSessionPanel(events, execEvents, loadReport(execEvents), hash))
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
	execEvents := readExecEvents()
	writeJSON(w, buildSessionPanel(events, execEvents, loadReport(execEvents), hash))
}

// writeJSON writes v as a JSON response with the dashboard content type.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
