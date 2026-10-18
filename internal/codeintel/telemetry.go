// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// DCISummary aggregates telemetry events across an evaluation window.
type DCISummary struct {
	TotalPrompts         int            `json:"total_prompts"`
	InjectionsApplied    int            `json:"injections_applied"`
	InjectionRate        float64        `json:"injection_rate"` // 0.0 - 1.0 fraction
	InjectionPct         float64        `json:"injection_pct"`  // 0.0 - 100.0 percentage
	Abstentions          int            `json:"abstentions"`
	AbstentionsByReason  map[string]int `json:"abstentions_by_reason"`
	TotalContextBytes    int64          `json:"total_context_bytes"`
	AvgBytesPerInjection float64        `json:"avg_bytes_per_injection"`
	LatencyP50Ms         float64        `json:"latency_p50_ms"`
	LatencyP95Ms         float64        `json:"latency_p95_ms"`
}

var recordMu sync.Mutex

// DefaultDCIEventsPath returns the path to dci-events.jsonl.
// Precedence:
// 1. $DOWNSHIFT_DCI_EVENT_LOG environment variable override
// 2. Repo-local .downshift/dci-events.jsonl if it exists
// 3. User state directory dci-events.jsonl via paths.DCIEventsPath()
// 4. Fallback: .downshift/dci-events.jsonl
func DefaultDCIEventsPath() string {
	if p := os.Getenv("DOWNSHIFT_DCI_EVENT_LOG"); p != "" {
		return p
	}
	repoPath := filepath.Join(".downshift", "dci-events.jsonl")
	if info, err := os.Stat(repoPath); err == nil && !info.IsDir() {
		return repoPath
	}
	p, err := paths.DCIEventsPath()
	if err == nil {
		return p
	}
	return repoPath
}

// RecordTelemetry appends a telemetry event to the default DCI event log.
// It enforces fail-open behavior: errors can be handled or ignored by callers.
func RecordTelemetry(t Telemetry) error {
	return RecordTelemetryTo(DefaultDCIEventsPath(), t)
}

// RecordTelemetryTo appends a telemetry event to the specified path.
func RecordTelemetryTo(path string, t Telemetry) error {
	if strings.ContainsRune(path, 0) {
		return fmt.Errorf("invalid path: contains null byte")
	}

	if err := VerifyTelemetryPrivacy(t); err != nil {
		return fmt.Errorf("telemetry privacy violation: %w", err)
	}

	recordMu.Lock()
	defer recordMu.Unlock()

	if t.Timestamp.IsZero() {
		t.Timestamp = time.Now().UTC()
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	_ = f.Chmod(0o600)

	line, err := json.Marshal(t)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(f, "%s\n", line); err != nil {
		return err
	}
	return f.Sync()
}

// ReadTelemetryEvents reads all DCI events from the default event log.
// Returns an empty slice (not an error) when the file does not exist.
func ReadTelemetryEvents() ([]Telemetry, error) {
	return ReadTelemetryEventsFrom(DefaultDCIEventsPath())
}

// ReadTelemetryEventsFrom reads all DCI telemetry events from path.
func ReadTelemetryEventsFrom(path string) ([]Telemetry, error) {
	if strings.ContainsRune(path, 0) {
		return nil, fmt.Errorf("invalid path: contains null byte")
	}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []Telemetry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev Telemetry
		if err := json.Unmarshal(line, &ev); err != nil {
			continue // skip malformed lines (fail-open)
		}
		events = append(events, ev)
	}
	return events, sc.Err()
}

// FilterTelemetryByDays returns telemetry events within the last n days.
// When n <= 0, all events are returned.
func FilterTelemetryByDays(events []Telemetry, days int) []Telemetry {
	return filterTelemetryByDaysAt(events, days, time.Now().UTC())
}

func filterTelemetryByDaysAt(events []Telemetry, days int, now time.Time) []Telemetry {
	if days <= 0 {
		return events
	}
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	var out []Telemetry
	for _, ev := range events {
		if ev.Timestamp.IsZero() {
			continue // fail-closed for dateless events in windowing
		}
		if ev.Timestamp.After(cutoff) {
			out = append(out, ev)
		}
	}
	return out
}

// AggregateTelemetry computes summary statistics from a slice of Telemetry events.
func AggregateTelemetry(events []Telemetry) DCISummary {
	s := DCISummary{
		TotalPrompts:        len(events),
		AbstentionsByReason: make(map[string]int),
	}

	if len(events) == 0 {
		return s
	}

	var totalBytes int64
	var latencies []float64

	for _, ev := range events {
		latencies = append(latencies, float64(ev.TotalDurationMs))
		if ev.InjectionApplied {
			s.InjectionsApplied++
			totalBytes += int64(ev.OutputBytes)
		} else {
			s.Abstentions++
			raw := ev.AbstentionReason
			norm := normalizeAbstentionReason(raw, ev.ErrorCode)
			s.AbstentionsByReason[norm]++
			if norm != raw && raw != "" {
				s.AbstentionsByReason[raw]++
			}
		}
	}

	s.TotalContextBytes = totalBytes
	if s.TotalPrompts > 0 {
		s.InjectionRate = float64(s.InjectionsApplied) / float64(s.TotalPrompts)
		s.InjectionPct = s.InjectionRate * 100.0
	}
	if s.InjectionsApplied > 0 {
		s.AvgBytesPerInjection = float64(totalBytes) / float64(s.InjectionsApplied)
	}

	sort.Float64s(latencies)
	s.LatencyP50Ms = calcPercentile(latencies, 0.50)
	s.LatencyP95Ms = calcPercentile(latencies, 0.95)

	return s
}

func normalizeAbstentionReason(raw string, errorCode string) string {
	switch raw {
	case "no_candidates_found", "no_candidates":
		return "no_candidates"
	case "insufficient_lexical_evidence", "insufficient_evidence":
		return "insufficient_evidence"
	case "conversational_greeting":
		return "conversational_greeting"
	case "chatter":
		return "chatter"
	case "":
		if errorCode != "" {
			return errorCode
		}
		return "unspecified"
	default:
		return raw
	}
}

func calcPercentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p * float64(len(sorted)))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// PrintDCIStats writes a human-readable DCI summary to w.
func PrintDCIStats(w io.Writer, s DCISummary) {
	if s.TotalPrompts == 0 {
		return
	}

	fmt.Fprintf(w, "Downshift Code Intelligence (DCI)\n")
	fmt.Fprintf(w, "─────────────────────────────────────\n")
	fmt.Fprintf(w, "Prompts evaluated     %8d\n", s.TotalPrompts)
	fmt.Fprintf(w, "Injections applied    %8d  %5.1f%%\n", s.InjectionsApplied, s.InjectionPct)
	fmt.Fprintf(w, "Abstentions           %8d  %5.1f%%\n", s.Abstentions, pct(s.Abstentions, s.TotalPrompts))

	standardReasons := []struct {
		key   string
		label string
	}{
		{"conversational_greeting", "conversational greeting"},
		{"chatter", "chatter"},
		{"no_candidates", "no candidates"},
		{"insufficient_evidence", "insufficient evidence"},
	}
	for _, r := range standardReasons {
		if count := s.AbstentionsByReason[r.key]; count > 0 {
			fmt.Fprintf(w, "  %-23s %6d\n", r.label, count)
		}
	}

	// Print any other non-standard reasons
	var otherKeys []string
	for k := range s.AbstentionsByReason {
		if k == "no_candidates_found" || k == "insufficient_lexical_evidence" {
			continue
		}
		isStandard := false
		for _, r := range standardReasons {
			if k == r.key {
				isStandard = true
				break
			}
		}
		if !isStandard && s.AbstentionsByReason[k] > 0 {
			otherKeys = append(otherKeys, k)
		}
	}
	sort.Strings(otherKeys)
	for _, k := range otherKeys {
		fmt.Fprintf(w, "  %-23s %6d\n", k, s.AbstentionsByReason[k])
	}

	fmt.Fprintf(w, "Context bytes emitted %8d  (total)\n", s.TotalContextBytes)
	fmt.Fprintf(w, "Avg bytes / injection %8.1f\n", s.AvgBytesPerInjection)
	fmt.Fprintf(w, "Latency (p50 / p95)   %6.1f ms / %6.1f ms\n", s.LatencyP50Ms, s.LatencyP95Ms)
	fmt.Fprintf(w, "─────────────────────────────────────\n")
}

// Print writes the DCI summary to w.
func (s DCISummary) Print(w io.Writer) {
	PrintDCIStats(w, s)
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}
