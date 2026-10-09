// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package server provides the local HTTP server for the harness-hub dashboard.
// Used by both `downshift serve` and the standalone dsmon-server binary.
package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/sensor"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

const DefaultPort = "7474"

// estimatedCostPerUnitUSD is a placeholder conversion rate from normalised
// savings units to dollars. It is NOT billing; it matches the
// --cost-per-unit default semantics. Real billing needs provider token counts.
const estimatedCostPerUnitUSD = 0.01

type rawEvent struct {
	Timestamp  string  `json:"timestamp"`
	Harness    string  `json:"harness"`
	From       string  `json:"requested_model"`
	To         string  `json:"final_model"`
	RequestedEffort string `json:"requested_reasoning_effort,omitempty"`
	FinalEffort     string `json:"final_reasoning_effort,omitempty"`
	Verdict    string  `json:"verdict"`
	Complexity string  `json:"complexity"`
	Savings    float64 `json:"estimated_savings"`
	// Outcome and Corrections decide whether the decision changed the spawn
	// (telemetry.AppliedRewrite); only applied downshifts count as savings.
	Outcome     string   `json:"outcome,omitempty"`
	Corrections []string `json:"corrections,omitempty"`
	// InputTokens carries provider-reported input tokens (nil when untracked).
	InputTokens *int64 `json:"input_tokens,omitempty"`
	// OutputTokens carries provider-reported output tokens (nil when untracked).
	OutputTokens *int64 `json:"output_tokens,omitempty"`
	// CachedTokens carries provider-reported cached input tokens (nil when untracked).
	CachedTokens *int64 `json:"cached_tokens,omitempty"`
	// ActualCostUSD is the real routed cost in USD (nil when untracked).
	ActualCostUSD *float64 `json:"actual_cost_usd,omitempty"`
	// BaselineCostUSD is the real baseline cost in USD (nil when untracked).
	BaselineCostUSD *float64 `json:"baseline_cost_usd,omitempty"`
}

type agentEntry struct {
	Timestamp string `json:"timestamp"`
	Tool      string `json:"tool"`
	Session   string `json:"session"`
	Task      string `json:"task"`
	Model     string `json:"model"`
}

type statusResponse struct {
	Harnesses []string     `json:"harnesses"`
	Switches  []rawEvent   `json:"switches"`
	Agents     []agentEntry            `json:"agents"`
	Stats      statsBlock              `json:"stats"`
	Compaction sensor.CompactionSummary `json:"compaction"`
}

type statsBlock struct {
	Total int `json:"total"`
	Down  int `json:"down"`
	Up    int `json:"up"`
	OK    int `json:"ok"`
	// NotApplied counts classified shifts the hook left unchanged
	// (allow, held by a guardrail, or blocked).
	NotApplied int     `json:"not_applied"`
	EstUSD     float64 `json:"est_usd"`
	// EstUnits accumulates the normalised savings fraction per DOWNSHIFT event.
	EstUnits float64 `json:"est_units"`
	// RealSavedUSD accumulates baseline minus actual cost for events with real costs.
	RealSavedUSD float64 `json:"real_saved_usd"`
	// RealCostEvents counts events that carried both real cost fields.
	RealCostEvents int `json:"real_cost_events"`
	// IsEstimate is always true: dollar figures are estimates, not billing.
	IsEstimate bool `json:"is_estimate"`
}

var window = 24 * time.Hour

func readEvents() []rawEvent {
	evPath, err := paths.EventsPath()
	if err != nil {
		return nil
	}
	f, err := os.Open(evPath)
	if err != nil {
		return nil
	}
	defer f.Close()
	cutoff := time.Now().UTC().Add(-window)
	var out []rawEvent
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e rawEvent
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.Harness == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, e.Timestamp)
		if err != nil || t.Before(cutoff) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func readAgents() []agentEntry {
	agPath, err := paths.AgentsPath()
	if err != nil {
		return nil
	}
	f, err := os.Open(agPath)
	if err != nil {
		return nil
	}
	defer f.Close()
	cutoff := time.Now().UTC().Add(-window)
	var out []agentEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var a agentEntry
		// Privacy-compatible filter: new hook events carry an empty task
		// (no prompt storage) but always carry a tool name. Skip only
		// lines with neither, so new events stay visible on the dashboard.
		if json.Unmarshal(sc.Bytes(), &a) != nil || (a.Task == "" && a.Tool == "") {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, a.Timestamp)
		if err != nil || t.Before(cutoff) {
			continue
		}
		out = append(out, a)
	}
	return out
}

// isLocalHost reports whether a host[:port] names this machine's loopback.
func isLocalHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// localOnly rejects requests that did not come from a page served by this
// machine: a non-loopback Host (DNS rebinding) or a foreign Origin. The
// bundled dashboard is same-origin, so no CORS headers are sent.
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLocalHost(r.Host) {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" {
			u, err := url.Parse(o)
			if err != nil || !isLocalHost(u.Host) {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// handleEvents streams Server-Sent Events whenever the data files change.
// Each update event tells the client to re-fetch /api/status.
func handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	evPath, _ := paths.EventsPath()
	agPath, _ := paths.AgentsPath()
	sensorPath, _ := paths.Join("context-sensor.json")

	lastEvSize := fileSize(evPath)
	lastAgSize := fileSize(agPath)
	lastSensorSize := fileSize(sensorPath)

	// Send a keep-alive comment every 25s; check for changes every 500ms.
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	send := func(event string) {
		fmt.Fprintf(w, "event: %s\ndata: {\"type\":\"%s\"}\n\n", event, event)
		flusher.Flush()
	}
	send("connected")

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			fmt.Fprintf(w, ": keep-alive\n\n")
			flusher.Flush()
		case <-ticker.C:
			evSize := fileSize(evPath)
			agSize := fileSize(agPath)
			sensorSize := fileSize(sensorPath)
			if evSize != lastEvSize || agSize != lastAgSize || sensorSize != lastSensorSize {
				lastEvSize = evSize
				lastAgSize = agSize
				lastSensorSize = sensorSize
				send("update")
			}
		}
	}
}

func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return fi.Size()
}

func handleStatus(w http.ResponseWriter, _ *http.Request) {
	events := readEvents()
	agents := readAgents()

	// Chips are the product harnesses, not "whoever wrote in the last 24h".
	// KiroCrew's last event can sit outside the window; the tab still exists.
	harnesses := []string{"antigravity", "codex", "cursor", "claude-code", "kirocrew"}
	seen := map[string]bool{}
	for _, h := range harnesses {
		seen[h] = true
	}
	for i := len(events) - 1; i >= 0; i-- {
		h := events[i].Harness
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		harnesses = append(harnesses, h)
	}

	var st statsBlock
	// IsEstimate is always true: dollar figures are estimates, not provider billing.
	st.IsEstimate = true
	var decisions []rawEvent
	for _, e := range events {
		if telemetry.IsCostOnlyOutcome(e.Outcome) {
			// Usage records carry real cost; baseline (no-route) records
			// are control-group events. Neither is a routing decision.
			if e.Outcome == telemetry.OutcomeUsage && e.ActualCostUSD != nil && e.BaselineCostUSD != nil {
				if saved := *e.BaselineCostUSD - *e.ActualCostUSD; saved > 0 {
					st.RealSavedUSD += saved
				}
				st.RealCostEvents++
			}
			continue
		}
		decisions = append(decisions, e)
		applied := telemetry.AppliedRewrite(e.Outcome, e.Corrections)
		switch {
		case e.Verdict == "DOWNSHIFT" && applied:
			st.Down++
			st.EstUnits += e.Savings
			st.EstUSD = st.EstUnits * estimatedCostPerUnitUSD
		case e.Verdict == "UPSHIFT" && applied:
			st.Up++
		case e.Verdict == "DOWNSHIFT" || e.Verdict == "UPSHIFT":
			st.NotApplied++
		case e.Verdict == "OK":
			st.OK++
		}
	}
	st.Total = len(decisions)
	events = decisions

	sw := events
	if sw == nil {
		sw = []rawEvent{}
	}
	if len(sw) > 8 {
		sw = sw[len(sw)-8:]
	}
	ag := agents
	if ag == nil {
		ag = []agentEntry{}
	}
	if len(ag) > 6 {
		ag = ag[len(ag)-6:]
	}
	if harnesses == nil {
		harnesses = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statusResponse{
		Harnesses: harnesses, Switches: sw, Agents: ag, Stats: st,
		Compaction: compactionSummary(),
	})
}

func compactionSummary() sensor.CompactionSummary {
	st, err := sensor.DefaultStore()
	if err != nil || st == nil {
		return sensor.SummarizeCompaction(nil)
	}
	obs, err := st.GetSummary()
	if err != nil {
		return sensor.SummarizeCompaction(nil)
	}
	return sensor.SummarizeCompaction(obs)
}

// Run starts the dashboard server on the given port.
// webDir is the path to the static web/ directory; pass "" to skip file serving.
func Run(port, webDir string) error {
	addr := "127.0.0.1:" + port
	fmt.Printf("downshift serve · http://%s\n", addr)
	return http.ListenAndServe(addr, newHandler(webDir))
}

func newHandler(webDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/events", handleEvents)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"ok":true,"ts":"%s"}`, time.Now().UTC().Format(time.RFC3339))
	})
	if webDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(webDir)))
	}
	return localOnly(mux)
}
