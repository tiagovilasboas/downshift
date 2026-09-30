// dsmon-server — local HTTP server for harness-hub PWA
// Serves routing events and agent spawns from ~/.harness-downshift/
// at http://localhost:7474.
//
// Build:  go build -o dsmon-server ./cmd/dsmon-server
// Run:    ./dsmon-server
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const port = "7474"

type rawEvent struct {
	Timestamp string  `json:"timestamp"`
	Harness   string  `json:"harness"`
	From      string  `json:"requested_model"`
	To        string  `json:"final_model"`
	Verdict   string  `json:"verdict"`
	Complexity string `json:"complexity"`
	Savings   float64 `json:"estimated_savings"`
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

var home, _ = os.UserHomeDir()
var window = 24 * time.Hour

func readEvents() ([]rawEvent, error) {
	path := filepath.Join(home, ".harness-downshift", "events.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []rawEvent
	cutoff := time.Now().UTC().Add(-window)
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
		events = append(events, e)
	}
	return events, nil
}

func readAgents() ([]agentEntry, error) {
	path := filepath.Join(home, ".harness-downshift", "agents.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var agents []agentEntry
	cutoff := time.Now().UTC().Add(-window)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var a agentEntry
		if json.Unmarshal(sc.Bytes(), &a) != nil || a.Task == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, a.Timestamp)
		if err != nil || t.Before(cutoff) {
			continue
		}
		agents = append(agents, a)
	}
	return agents, nil
}

func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	events, _ := readEvents()
	agents, _ := readAgents()

	// deduplicate harnesses in order of last seen
	seen := map[string]bool{}
	var harnesses []string
	for i := len(events) - 1; i >= 0; i-- {
		h := events[i].Harness
		if h != "" && !seen[h] {
			seen[h] = true
			harnesses = append([]string{h}, harnesses...)
		}
	}

	// stats
	var stats statsBlock
	stats.Total = len(events)
	for _, e := range events {
		switch e.Verdict {
		case "DOWNSHIFT":
			stats.Down++
			stats.EstUSD += e.Savings * 0.01
		case "UPSHIFT":
			stats.Up++
		case "OK":
			stats.OK++
		}
	}

	// last 8 switches
	switches := events
	if len(switches) > 8 {
		switches = switches[len(switches)-8:]
	}
	// last 6 agents
	ags := agents
	if len(ags) > 6 {
		ags = ags[len(ags)-6:]
	}

	resp := statusResponse{
		Harnesses: harnesses,
		Switches:  switches,
		Agents:    ags,
		Stats:     stats,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"ok":true,"ts":"%s"}`, time.Now().UTC().Format(time.RFC3339))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", cors(handleStatus))
	mux.HandleFunc("/health",     cors(handleHealth))

	// Serve the PWA static files if public/ is present alongside the binary
	mux.Handle("/", http.FileServer(http.Dir("public")))

	addr := "127.0.0.1:" + port
	fmt.Printf("harness-hub server · http://%s\n", addr)
	fmt.Printf("  /api/status  routing events + agents (last 24h)\n")
	fmt.Printf("  /health      liveness check\n")
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
