// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// withLoaders swaps the package load seams for the duration of a test and
// restores them afterward, so tests never touch the real event logs.
func withLoaders(
	t *testing.T,
	routing func() ([]telemetry.Event, error),
	compactorSummary func() (execwrap.Summary, error),
	compactorEvents func() ([]execwrap.Event, error),
) {
	t.Helper()
	origRouting := loadRoutingEvents
	origSummary := loadCompactorSummary
	origEvents := loadCompactorEvents
	t.Cleanup(func() {
		loadRoutingEvents = origRouting
		loadCompactorSummary = origSummary
		loadCompactorEvents = origEvents
	})
	if routing != nil {
		loadRoutingEvents = routing
	}
	if compactorSummary != nil {
		loadCompactorSummary = compactorSummary
	}
	if compactorEvents != nil {
		loadCompactorEvents = compactorEvents
	}
}

// i64p returns a pointer to v, for the Event's optional token fields.
func i64p(v int64) *int64 { return &v }

// usageEvent builds a decision + its linked usage record on a cheaper model so
// TokenSavings counts the work/cache as saved from the requested model.
func usageEvent(stamp time.Time, corrID string, input, output, cache int64) []telemetry.Event {
	decision := telemetry.Event{
		CorrelationID: corrID,
		Timestamp:     stamp.Format(time.RFC3339Nano),
		Harness:       "claude-code",
		Verdict:       "DOWNSHIFT",
		Outcome:       telemetry.OutcomeRewriteEmitted,
		FromModel:     "claude-opus-5-5",
		ToModel:       "claude-haiku-4-5",
	}
	usage := telemetry.Event{
		CorrelationID:  "usage-" + corrID,
		Timestamp:      stamp.Format(time.RFC3339Nano),
		Harness:        "claude-code",
		Outcome:        telemetry.OutcomeUsage,
		LinkedDecision: telemetry.DecisionKey(decision),
		ToModel:        "claude-haiku-4-5",
		FromModel:      "claude-opus-5-5",
		InputTokens:    i64p(input),
		OutputTokens:   i64p(output),
		CachedTokens:   i64p(cache),
	}
	return []telemetry.Event{decision, usage}
}

func TestCalculateSavings_Empty(t *testing.T) {
	withLoaders(t,
		func() ([]telemetry.Event, error) { return nil, nil },
		func() (execwrap.Summary, error) { return execwrap.Summary{}, nil },
		nil,
	)
	s, err := CalculateSavings(7)
	if err != nil {
		t.Fatalf("CalculateSavings: %v", err)
	}
	if s.Period != "7d" {
		t.Errorf("Period = %q, want 7d", s.Period)
	}
	if s.TotalTokens != 0 {
		t.Errorf("TotalTokens = %d, want 0", s.TotalTokens)
	}
}

func TestCalculateSavings_AllTimeLabel(t *testing.T) {
	withLoaders(t,
		func() ([]telemetry.Event, error) { return nil, nil },
		func() (execwrap.Summary, error) { return execwrap.Summary{}, nil },
		nil,
	)
	s, _ := CalculateSavings(0)
	if s.Period != "all time" {
		t.Errorf("Period = %q, want %q", s.Period, "all time")
	}
}

func TestCalculateSavings_CombinesRoutingAndCompactor(t *testing.T) {
	now := time.Now().UTC()
	events := usageEvent(now, "tu-1", 1000, 500, 200) // work=1500, cache=200 saved
	withLoaders(t,
		func() ([]telemetry.Event, error) { return events, nil },
		func() (execwrap.Summary, error) {
			return execwrap.Summary{EstimatedTokensSaved: 400}, nil
		},
		nil,
	)
	s, err := CalculateSavings(30)
	if err != nil {
		t.Fatalf("CalculateSavings: %v", err)
	}
	if s.RoutingTokens != 1700 {
		t.Errorf("RoutingTokens = %d, want 1700", s.RoutingTokens)
	}
	if s.CompactorTokens != 400 {
		t.Errorf("CompactorTokens = %d, want 400", s.CompactorTokens)
	}
	if s.TotalTokens != 2100 {
		t.Errorf("TotalTokens = %d, want 2100", s.TotalTokens)
	}
}

func TestCalculateSavings_CompactorErrorIsNonFatal(t *testing.T) {
	now := time.Now().UTC()
	events := usageEvent(now, "tu-2", 400, 0, 0) // work=400 saved
	withLoaders(t,
		func() ([]telemetry.Event, error) { return events, nil },
		func() (execwrap.Summary, error) {
			return execwrap.Summary{}, errTestCompactor
		},
		nil,
	)
	s, err := CalculateSavings(7)
	if err != nil {
		t.Fatalf("CalculateSavings should swallow compactor error, got %v", err)
	}
	if s.CompactorTokens != 0 {
		t.Errorf("CompactorTokens = %d, want 0 on compactor error", s.CompactorTokens)
	}
	if s.RoutingTokens != 400 {
		t.Errorf("RoutingTokens = %d, want 400", s.RoutingTokens)
	}
}

func TestCalculateSavings_RoutingLoadError(t *testing.T) {
	withLoaders(t,
		func() ([]telemetry.Event, error) { return nil, errTestRouting },
		nil, nil,
	)
	if _, err := CalculateSavings(7); err == nil {
		t.Fatal("expected error from routing load, got nil")
	}
}

func TestFormatTokenCount(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1500, "1.5k"},
		{1_500_000, "1.5M"},
	}
	for _, c := range cases {
		if got := formatTokenCount(c.in); got != c.want {
			t.Errorf("formatTokenCount(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatSavings_Shape(t *testing.T) {
	s := UnifiedSavings{
		Period:          "7d",
		RoutingTokens:   1700,
		CompactorTokens: 400,
		TotalTokens:     2100,
	}
	got := FormatSavings(s)
	want := "7d: saved ~2.1k tokens (routing 1.7k | compactor ~400 est.)"
	if got != want {
		t.Errorf("FormatSavings = %q, want %q", got, want)
	}
}

func TestBuildDailyTrend_BucketsByDay(t *testing.T) {
	day1 := time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)

	var events []telemetry.Event
	e1 := usageEvent(day1, "d1", 100, 0, 0)
	e1[0].SessionID = telemetry.HashSessionID("s1")
	e1[1].SessionID = telemetry.HashSessionID("s1")
	events = append(events, e1...)
	e2 := usageEvent(day2, "d2", 200, 0, 0)
	e2[0].SessionID = telemetry.HashSessionID("s2")
	e2[1].SessionID = telemetry.HashSessionID("s2")
	events = append(events, e2...)

	execEvents := []execwrap.Event{
		{TS: day1.Format(time.RFC3339), Family: "git", RawBytes: 1000, EmittedBytes: 600, Compacted: true},
		{TS: day2.Format(time.RFC3339), Family: "git", RawBytes: 800, EmittedBytes: 800, Compacted: false},
	}

	withLoaders(t,
		func() ([]telemetry.Event, error) { return events, nil },
		nil,
		func() ([]execwrap.Event, error) { return execEvents, nil },
	)

	trend, err := BuildDailyTrend(0)
	if err != nil {
		t.Fatalf("BuildDailyTrend: %v", err)
	}
	if len(trend) != 2 {
		t.Fatalf("expected 2 days, got %d", len(trend))
	}
	// Oldest first.
	if trend[0].Date != "2026-10-18" || trend[1].Date != "2026-10-19" {
		t.Errorf("dates out of order: %q, %q", trend[0].Date, trend[1].Date)
	}
	if trend[0].Sessions != 1 {
		t.Errorf("day1 sessions = %d, want 1", trend[0].Sessions)
	}
	if trend[0].Compactor.SavedBytes != 400 {
		t.Errorf("day1 saved_bytes = %d, want 400", trend[0].Compactor.SavedBytes)
	}
	if trend[0].Compactor.EstimatedTokensSaved != 100 {
		t.Errorf("day1 est tokens = %d, want 100", trend[0].Compactor.EstimatedTokensSaved)
	}
	if trend[0].Routing.WorkSaved != 100 {
		t.Errorf("day1 work saved = %d, want 100", trend[0].Routing.WorkSaved)
	}
}

func TestBuildDailyTrend_RoutingLoadError(t *testing.T) {
	withLoaders(t,
		func() ([]telemetry.Event, error) { return nil, errTestRouting },
		nil, nil,
	)
	if _, err := BuildDailyTrend(7); err == nil {
		t.Fatal("expected error from routing load, got nil")
	}
}

var (
	errTestRouting   = testErr("routing load failed")
	errTestCompactor = testErr("compactor load failed")
)

type testErr string

func (e testErr) Error() string { return string(e) }
