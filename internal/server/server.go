// Package server provides the local HTTP server for the harness-hub dashboard.
// Used by both `downshift serve` and the standalone dsmon-server binary.
package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const DefaultPort = "7474"

type rawEvent struct {
	Timestamp  string  `json:"timestamp"`
	Harness    string  `json:"harness"`
	From       string  `json:"requested_model"`
	To         string  `json:"final_model"`
	Verdict    string  `json:"verdict"`
	Complexity string  `json:"complexity"`
	Savings    float64 `json:"estimated_savings"`
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
	Agents    []agentEntry `json:"agents"`
	Stats     statsBlock   `json:"stats"`
}

type statsBlock struct {
	Total  int     `json:"total"`
	Down   int     `json:"down"`
	Up     int     `json:"up"`
	OK     int     `json:"ok"`
	EstUSD float64 `json:"est_usd"`
}

var window = 24 * time.Hour

func home() string { h, _ := os.UserHomeDir(); return h }

func readEvents() []rawEvent {
	f, err := os.Open(filepath.Join(home(), ".harness-downshift", "events.jsonl"))
	if err != nil { return nil }
	defer f.Close()
	cutoff := time.Now().UTC().Add(-window)
	var out []rawEvent
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e rawEvent
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.Harness == "" { continue }
		t, err := time.Parse(time.RFC3339Nano, e.Timestamp)
		if err != nil || t.Before(cutoff) { continue }
		out = append(out, e)
	}
	return out
}

func readAgents() []agentEntry {
	f, err := os.Open(filepath.Join(home(), ".harness-downshift", "agents.jsonl"))
	if err != nil { return nil }
	defer f.Close()
	cutoff := time.Now().UTC().Add(-window)
	var out []agentEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var a agentEntry
		if json.Unmarshal(sc.Bytes(), &a) != nil || a.Task == "" { continue }
		t, err := time.Parse(time.RFC3339Nano, a.Timestamp)
		if err != nil || t.Before(cutoff) { continue }
		out = append(out, a)
	}
	return out
}

func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent); return }
		next(w, r)
	}
}

// handleEvents streams Server-Sent Events whenever the data files change.
// Each update event tells the client to re-fetch /api/status.
func handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	evPath := filepath.Join(home(), ".harness-downshift", "events.jsonl")
	agPath := filepath.Join(home(), ".harness-downshift", "agents.jsonl")

	lastEvSize  := fileSize(evPath)
	lastAgSize  := fileSize(agPath)

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
			if evSize != lastEvSize || agSize != lastAgSize {
				lastEvSize = evSize
				lastAgSize = agSize
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

	seen := map[string]bool{}
	var harnesses []string
	for i := len(events) - 1; i >= 0; i-- {
		if h := events[i].Harness; h != "" && !seen[h] {
			seen[h] = true
			harnesses = append([]string{h}, harnesses...)
		}
	}

	var st statsBlock
	st.Total = len(events)
	for _, e := range events {
		switch e.Verdict {
		case "DOWNSHIFT": st.Down++; st.EstUSD += e.Savings * 0.01
		case "UPSHIFT":   st.Up++
		case "OK":        st.OK++
		}
	}

	sw := events
	if len(sw) > 8 { sw = sw[len(sw)-8:] }
	ag := agents
	if len(ag) > 6 { ag = ag[len(ag)-6:] }

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statusResponse{Harnesses: harnesses, Switches: sw, Agents: ag, Stats: st})
}

// Run starts the dashboard server on the given port.
// webDir is the path to the static web/ directory; pass "" to skip file serving.
func Run(port, webDir string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", cors(handleStatus))
	mux.HandleFunc("/events",     cors(handleEvents))
	mux.HandleFunc("/health", cors(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"ok":true,"ts":"%s"}`, time.Now().UTC().Format(time.RFC3339))
	}))
	if webDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(webDir)))
	}

	addr := "127.0.0.1:" + port
	fmt.Printf("downshift serve · http://%s\n", addr)
	return http.ListenAndServe(addr, mux)
}
