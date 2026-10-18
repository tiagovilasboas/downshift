// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRecordTelemetry_Basic(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "dci-events.jsonl")

	ev1 := Telemetry{
		IndexedFiles:     10,
		CandidateCount:   5,
		SelectedFiles:    2,
		SelectedSymbols:  4,
		OutputBytes:      1200,
		TotalDurationMs:  45,
		InjectionApplied: true,
	}

	if err := RecordTelemetryTo(logPath, ev1); err != nil {
		t.Fatalf("RecordTelemetryTo failed: %v", err)
	}

	ev2 := Telemetry{
		TotalDurationMs:  12,
		InjectionApplied: false,
		AbstentionReason: "conversational_greeting",
	}

	if err := RecordTelemetryTo(logPath, ev2); err != nil {
		t.Fatalf("RecordTelemetryTo failed: %v", err)
	}

	events, err := ReadTelemetryEventsFrom(logPath)
	if err != nil {
		t.Fatalf("ReadTelemetryEventsFrom failed: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if !events[0].InjectionApplied || events[0].OutputBytes != 1200 {
		t.Errorf("unexpected event 0: %+v", events[0])
	}
	if events[0].Timestamp.IsZero() {
		t.Errorf("expected auto-populated timestamp, got zero")
	}

	if events[1].InjectionApplied || events[1].AbstentionReason != "conversational_greeting" {
		t.Errorf("unexpected event 1: %+v", events[1])
	}
}

func TestRecordTelemetry_Permissions(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "state", "nested")
	logPath := filepath.Join(subDir, "dci-events.jsonl")

	ev := Telemetry{
		TotalDurationMs:  25,
		InjectionApplied: true,
		OutputBytes:      850,
	}

	if err := RecordTelemetryTo(logPath, ev); err != nil {
		t.Fatalf("RecordTelemetryTo failed: %v", err)
	}

	fi, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("failed to stat log file: %v", err)
	}

	// Mode must be 0600 on UNIX
	perm := fi.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("expected file permissions 0600, got %04o", perm)
	}
}

func TestRecordTelemetry_Concurrent(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "dci-events.jsonl")

	const goroutines = 25
	const eventsPerGoroutine = 10
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				ev := Telemetry{
					IndexedFiles:     id,
					OutputBytes:      j * 100,
					TotalDurationMs:  int64(id + j),
					InjectionApplied: (j % 2) == 0,
				}
				if err := RecordTelemetryTo(logPath, ev); err != nil {
					t.Errorf("concurrent RecordTelemetryTo error: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	events, err := ReadTelemetryEventsFrom(logPath)
	if err != nil {
		t.Fatalf("ReadTelemetryEventsFrom failed: %v", err)
	}

	expectedCount := goroutines * eventsPerGoroutine
	if len(events) != expectedCount {
		t.Fatalf("expected %d events, got %d", expectedCount, len(events))
	}
}

func TestRecordTelemetry_DefaultPathEnvOverride(t *testing.T) {
	tempDir := t.TempDir()
	customPath := filepath.Join(tempDir, "custom-dci.jsonl")
	t.Setenv("DOWNSHIFT_DCI_EVENT_LOG", customPath)

	if got := DefaultDCIEventsPath(); got != customPath {
		t.Fatalf("DefaultDCIEventsPath = %s, want %s", got, customPath)
	}

	ev := Telemetry{
		TotalDurationMs:  15,
		InjectionApplied: true,
	}

	if err := RecordTelemetry(ev); err != nil {
		t.Fatalf("RecordTelemetry failed: %v", err)
	}

	events, err := ReadTelemetryEvents()
	if err != nil {
		t.Fatalf("ReadTelemetryEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestReadTelemetryEvents_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "does-not-exist.jsonl")

	events, err := ReadTelemetryEventsFrom(logPath)
	if err != nil {
		t.Fatalf("expected nil error for non-existent file, got %v", err)
	}
	if events != nil {
		t.Fatalf("expected nil slice for non-existent file, got %v", events)
	}
}

func TestReadTelemetryEvents_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "empty.jsonl")
	if err := os.WriteFile(logPath, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}

	events, err := ReadTelemetryEventsFrom(logPath)
	if err != nil {
		t.Fatalf("expected nil error for empty file, got %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestReadTelemetryEvents_CorruptedAndEmptyLines(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "corrupted.jsonl")

	content := strings.Join([]string{
		`{"total_duration_ms": 10, "injection_applied": true, "output_bytes": 500}`,
		``,
		`{not-valid-json`,
		`   `,
		`{"total_duration_ms": 20, "injection_applied": false, "abstention_reason": "chatter"}`,
		`malformed line entirely`,
		`{"total_duration_ms": 30, "injection_applied": true, "output_bytes": 800}`,
	}, "\n")

	if err := os.WriteFile(logPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write corrupted test file: %v", err)
	}

	events, err := ReadTelemetryEventsFrom(logPath)
	if err != nil {
		t.Fatalf("ReadTelemetryEventsFrom should not fail on corrupted lines, got error: %v", err)
	}

	// Should have parsed the 3 valid JSON lines, skipping empty and malformed lines
	if len(events) != 3 {
		t.Fatalf("expected 3 valid events, got %d", len(events))
	}
	if events[0].OutputBytes != 500 || !events[0].InjectionApplied {
		t.Errorf("unexpected event 0: %+v", events[0])
	}
	if events[1].AbstentionReason != "chatter" || events[1].InjectionApplied {
		t.Errorf("unexpected event 1: %+v", events[1])
	}
	if events[2].OutputBytes != 800 || !events[2].InjectionApplied {
		t.Errorf("unexpected event 2: %+v", events[2])
	}
}

func TestFilterTelemetryByDays(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	events := []Telemetry{
		{
			TotalDurationMs: 10,
			Timestamp:       now.Add(-2 * time.Hour), // today (0.08 days)
		},
		{
			TotalDurationMs: 20,
			Timestamp:       now.Add(-48 * time.Hour), // 2 days ago
		},
		{
			TotalDurationMs: 30,
			Timestamp:       now.Add(-10 * 24 * time.Hour), // 10 days ago
		},
		{
			TotalDurationMs: 40,
			Timestamp:       now.Add(-40 * 24 * time.Hour), // 40 days ago
		},
		{
			TotalDurationMs: 50,
			Timestamp:       time.Time{}, // dateless
		},
	}

	// 1. days <= 0 returns all events
	allZero := filterTelemetryByDaysAt(events, 0, now)
	if len(allZero) != len(events) {
		t.Errorf("days=0: expected %d events, got %d", len(events), len(allZero))
	}
	allNeg := filterTelemetryByDaysAt(events, -5, now)
	if len(allNeg) != len(events) {
		t.Errorf("days=-5: expected %d events, got %d", len(events), len(allNeg))
	}

	// 2. days = 1 (last 24 hours): only event 0 (dateless excluded)
	w1 := filterTelemetryByDaysAt(events, 1, now)
	if len(w1) != 1 || w1[0].TotalDurationMs != 10 {
		t.Errorf("days=1: expected 1 event (10ms), got %d", len(w1))
	}

	// 3. days = 7 (last 7 days): events 0 and 1
	w7 := filterTelemetryByDaysAt(events, 7, now)
	if len(w7) != 2 {
		t.Errorf("days=7: expected 2 events, got %d", len(w7))
	}

	// 4. days = 30 (last 30 days): events 0, 1, and 2
	w30 := filterTelemetryByDaysAt(events, 30, now)
	if len(w30) != 3 {
		t.Errorf("days=30: expected 3 events, got %d", len(w30))
	}

	// Verify standard public wrapper works without error
	filtered := FilterTelemetryByDays(events, 30)
	if len(filtered) < 1 {
		t.Errorf("FilterTelemetryByDays returned empty slice")
	}
}

func TestAggregateTelemetry_Empty(t *testing.T) {
	s := AggregateTelemetry(nil)
	if s.TotalPrompts != 0 {
		t.Errorf("TotalPrompts = %d, want 0", s.TotalPrompts)
	}
	if s.InjectionsApplied != 0 {
		t.Errorf("InjectionsApplied = %d, want 0", s.InjectionsApplied)
	}
	if s.InjectionRate != 0.0 || s.InjectionPct != 0.0 {
		t.Errorf("InjectionRate = %f, InjectionPct = %f, want 0", s.InjectionRate, s.InjectionPct)
	}
	if s.TotalContextBytes != 0 || s.AvgBytesPerInjection != 0.0 {
		t.Errorf("Bytes not zero: total=%d, avg=%f", s.TotalContextBytes, s.AvgBytesPerInjection)
	}
	if s.LatencyP50Ms != 0.0 || s.LatencyP95Ms != 0.0 {
		t.Errorf("Latencies not zero: p50=%f, p95=%f", s.LatencyP50Ms, s.LatencyP95Ms)
	}
	if s.AbstentionsByReason == nil {
		t.Errorf("AbstentionsByReason map should be initialized")
	}
}

func TestAggregateTelemetry_Calculations(t *testing.T) {
	events := []Telemetry{
		// 3 Injections
		{TotalDurationMs: 10, InjectionApplied: true, OutputBytes: 1000},
		{TotalDurationMs: 20, InjectionApplied: true, OutputBytes: 2000},
		{TotalDurationMs: 30, InjectionApplied: true, OutputBytes: 1500},

		// 3 Abstentions with different reasons
		{TotalDurationMs: 5, InjectionApplied: false, AbstentionReason: "conversational_greeting"},
		{TotalDurationMs: 8, InjectionApplied: false, AbstentionReason: "no_candidates_found"},
		{TotalDurationMs: 12, InjectionApplied: false, AbstentionReason: "insufficient_lexical_evidence"},
		{TotalDurationMs: 15, InjectionApplied: false, AbstentionReason: "chatter"},
	}

	s := AggregateTelemetry(events)

	if s.TotalPrompts != 7 {
		t.Fatalf("TotalPrompts = %d, want 7", s.TotalPrompts)
	}
	if s.InjectionsApplied != 3 {
		t.Fatalf("InjectionsApplied = %d, want 3", s.InjectionsApplied)
	}

	expectedRate := 3.0 / 7.0
	expectedPct := (3.0 / 7.0) * 100.0
	if fmt.Sprintf("%.2f", s.InjectionRate) != fmt.Sprintf("%.2f", expectedRate) {
		t.Errorf("InjectionRate = %f, want %f", s.InjectionRate, expectedRate)
	}
	if fmt.Sprintf("%.1f", s.InjectionPct) != fmt.Sprintf("%.1f", expectedPct) {
		t.Errorf("InjectionPct = %f, want %f", s.InjectionPct, expectedPct)
	}

	if s.Abstentions != 4 {
		t.Fatalf("Abstentions = %d, want 4", s.Abstentions)
	}

	// Verify normalization in AbstentionsByReason
	if s.AbstentionsByReason["conversational_greeting"] != 1 {
		t.Errorf("conversational_greeting count = %d, want 1", s.AbstentionsByReason["conversational_greeting"])
	}
	if s.AbstentionsByReason["no_candidates"] != 1 {
		t.Errorf("normalized no_candidates count = %d, want 1", s.AbstentionsByReason["no_candidates"])
	}
	if s.AbstentionsByReason["no_candidates_found"] != 1 {
		t.Errorf("raw no_candidates_found count = %d, want 1", s.AbstentionsByReason["no_candidates_found"])
	}
	if s.AbstentionsByReason["insufficient_evidence"] != 1 {
		t.Errorf("normalized insufficient_evidence count = %d, want 1", s.AbstentionsByReason["insufficient_evidence"])
	}
	if s.AbstentionsByReason["insufficient_lexical_evidence"] != 1 {
		t.Errorf("raw insufficient_lexical_evidence count = %d, want 1", s.AbstentionsByReason["insufficient_lexical_evidence"])
	}
	if s.AbstentionsByReason["chatter"] != 1 {
		t.Errorf("chatter count = %d, want 1", s.AbstentionsByReason["chatter"])
	}

	// Byte aggregations
	expectedTotalBytes := int64(1000 + 2000 + 1500)
	if s.TotalContextBytes != expectedTotalBytes {
		t.Errorf("TotalContextBytes = %d, want %d", s.TotalContextBytes, expectedTotalBytes)
	}
	expectedAvgBytes := float64(expectedTotalBytes) / 3.0
	if fmt.Sprintf("%.1f", s.AvgBytesPerInjection) != fmt.Sprintf("%.1f", expectedAvgBytes) {
		t.Errorf("AvgBytesPerInjection = %f, want %f", s.AvgBytesPerInjection, expectedAvgBytes)
	}
}

func TestAggregateTelemetry_LatencyPercentiles(t *testing.T) {
	// Create a dataset of 100 durations: 1ms to 100ms
	var events []Telemetry
	for i := 1; i <= 100; i++ {
		events = append(events, Telemetry{
			TotalDurationMs:  int64(i),
			InjectionApplied: true,
			OutputBytes:      100,
		})
	}

	s := AggregateTelemetry(events)

	// In 100 items (0..99 indices):
	// p50 index = 50 -> value 51
	// p95 index = 95 -> value 96
	if s.LatencyP50Ms != 51.0 {
		t.Errorf("LatencyP50Ms = %f, want 51.0", s.LatencyP50Ms)
	}
	if s.LatencyP95Ms != 96.0 {
		t.Errorf("LatencyP95Ms = %f, want 96.0", s.LatencyP95Ms)
	}
}

func TestPrintDCIStats_Empty(t *testing.T) {
	var buf bytes.Buffer
	PrintDCIStats(&buf, DCISummary{})
	if buf.Len() != 0 {
		t.Errorf("expected empty output for empty summary, got:\n%s", buf.String())
	}
}

func TestPrintDCIStats_Formatting(t *testing.T) {
	s := DCISummary{
		TotalPrompts:         100,
		InjectionsApplied:    25,
		InjectionRate:        0.25,
		InjectionPct:         25.0,
		Abstentions:          75,
		TotalContextBytes:    50000,
		AvgBytesPerInjection: 2000.0,
		LatencyP50Ms:         4.5,
		LatencyP95Ms:         15.2,
		AbstentionsByReason: map[string]int{
			"conversational_greeting": 30,
			"chatter":                 15,
			"no_candidates":           20,
			"insufficient_evidence":   10,
		},
	}

	var buf bytes.Buffer
	PrintDCIStats(&buf, s)
	out := buf.String()

	expectedSubstrings := []string{
		"Downshift Code Intelligence (DCI)",
		"Prompts evaluated          100",
		"Injections applied          25   25.0%",
		"Abstentions                 75   75.0%",
		"conversational greeting     30",
		"chatter                     15",
		"no candidates               20",
		"insufficient evidence       10",
		"Context bytes emitted    50000  (total)",
		"Avg bytes / injection   2000.0",
		"Latency (p50 / p95)      4.5 ms /   15.2 ms",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(out, sub) {
			t.Errorf("output missing expected substring %q\nActual output:\n%s", sub, out)
		}
	}

	// Verify method wrapper .Print(w) behaves identically
	var bufMethod bytes.Buffer
	s.Print(&bufMethod)
	if bufMethod.String() != out {
		t.Errorf("s.Print() output differed from PrintDCIStats(w, s)")
	}
}
