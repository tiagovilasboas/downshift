// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package quota

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func number(v float64) *float64 { return &v }
func fixture(now time.Time) Snapshot {
	return Snapshot{Version: 1, Harness: "codex", Source: "codex-transcript", ObservedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute), Windows: []Window{{ID: "shared", Scope: "harness", UsedPercent: number(40), ResetsAt: now.Add(time.Hour)}}}
}

func TestEvidenceStatesAndHarnessIsolation(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name           string
		change         func(*Snapshot)
		harness, model string
		want           Status
	}{
		{"shared budget applies without inventing per-model amounts", func(*Snapshot) {}, "codex", "unknown-model", Available},
		{"wrong harness", func(*Snapshot) {}, "cursor", "unknown-model", Unknown},
		{"exhausted", func(s *Snapshot) { s.Windows[0].UsedPercent = number(100) }, "codex", "m", Exhausted},
		{"missing percentage", func(s *Snapshot) { s.Windows[0].UsedPercent = nil }, "codex", "m", Unknown},
		{"future observation", func(s *Snapshot) { s.ObservedAt = now.Add(time.Second) }, "codex", "m", Unknown},
		{"expired snapshot", func(s *Snapshot) { s.ExpiresAt = now }, "codex", "m", Stale},
		{"reset reached", func(s *Snapshot) { s.Windows[0].ResetsAt = now }, "codex", "m", Stale},
		{"missing reset", func(s *Snapshot) { s.Windows[0].ResetsAt = time.Time{} }, "codex", "m", Unknown},
		{"invalid percentage", func(s *Snapshot) { s.Windows[0].UsedPercent = number(101) }, "codex", "m", Unknown},
		{"unbounded TTL", func(s *Snapshot) { s.ExpiresAt = s.ObservedAt.Add(24 * time.Hour) }, "codex", "m", Unknown},
		{"model pool exact", func(s *Snapshot) { s.Windows[0].Scope = "models"; s.Windows[0].ModelIDs = []string{"m"} }, "codex", "m", Available},
		{"model pool cannot leak", func(s *Snapshot) { s.Windows[0].Scope = "models"; s.Windows[0].ModelIDs = []string{"m"} }, "codex", "m-new-version", Unknown},
		{"secondary exhausted vetoes primary", func(s *Snapshot) {
			s.Windows = append(s.Windows, Window{ID: "secondary", Scope: "harness", UsedPercent: number(100), ResetsAt: now.Add(time.Hour)})
		}, "codex", "m", Exhausted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(now)
			tc.change(&s)
			if got := s.Evaluate(tc.harness, tc.model, now); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestNativeSourcesDoNotConfuseContextOrPurchasedCredits(t *testing.T) {
	now := time.Now().UTC()
	reset := now.Add(time.Hour).Unix()
	b, _ := json.Marshal(map[string]any{"context_window": map[string]any{"used_percentage": 100}, "rate_limits": map[string]any{"five_hour": map[string]any{"used_percentage": 20, "resets_at": reset}, "seven_day": map[string]any{"used_percentage": 30, "resets_at": reset}}})
	s, err := ParseNative("claude-statusline", b, now, 5*time.Minute)
	if err != nil || s.Evaluate("claude-code", "m", now) != Available {
		t.Fatalf("context mistaken for quota: %+v %v", s, err)
	}
	b, _ = json.Marshal(map[string]any{"credits": map[string]any{"hasCredits": false, "balance": "0"}, "rateLimitsByLimitId": map[string]any{"codex": map[string]any{"primary": map[string]any{"usedPercent": 20, "resetsAt": reset}, "secondary": map[string]any{"usedPercent": 30, "resetsAt": reset}}, "base_model_inference": map[string]any{"normalModelSlug": "exact-model", "primary": map[string]any{"usedPercent": 100, "resetsAt": reset}, "secondary": nil}}})
	s, err = ParseNative("codex-usage", b, now, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if s.Evaluate("codex", "another-model", now) != Available || s.Evaluate("codex", "exact-model", now) != Exhausted {
		t.Fatalf("credit or model coverage wrong: %+v", s)
	}
	if _, err = ParseNative("codex-usage", []byte(`{"used_percent":0}`), now, 5*time.Minute); err == nil {
		t.Fatal("accepted fabricated universal shape")
	}
	missing, _ := ParseNative("claude-statusline", []byte(`{"rate_limits":{"five_hour":{"used_percentage":0}}}`), now, 5*time.Minute)
	if missing.Evaluate("claude-code", "m", now) == Available {
		t.Fatal("missing window granted budget")
	}
}

func TestCursorPoolsRequireRuntimeMembership(t *testing.T) {
	now := time.Now().UTC()
	p := map[string]any{"plan_usage": map[string]any{"auto_percent_used": 60, "api_percent_used": 100}, "billing_cycle_end": now.Add(time.Hour).UnixMilli()}
	parse := func() Snapshot {
		b, _ := json.Marshal(p)
		s, e := ParseNative("cursor-usage", b, now, 5*time.Minute)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	if parse().Evaluate("cursor", "native-auto", now) != Unknown {
		t.Fatal("inferred membership from catalog")
	}
	p["auto_bucket_models"] = []string{"native-auto"}
	s := parse()
	if s.Evaluate("cursor", "native-auto", now) != Available || s.Evaluate("cursor", "other", now) != Unknown {
		t.Fatal("pool coverage leaked")
	}
	p["available_models"] = []string{"native-auto", "other"}
	p["available_models_complete"] = true
	s = parse()
	if s.Evaluate("cursor", "other", now) != Exhausted {
		t.Fatal("other-models exhaustion ignored")
	}
	old := now.Add(-10 * time.Minute)
	p["observed_at"] = old
	s = parse()
	if !s.ObservedAt.Equal(old) || s.Evaluate("cursor", "native-auto", now) != Stale {
		t.Fatal("replaying Cursor export renewed budget")
	}
	b, _ := json.Marshal(map[string]any{"planUsage": map[string]any{"autoPercentUsed": 10, "apiPercentUsed": 100}, "autoBucketModels": []string{"dynamic-model"}, "billingCycleEnd": now.Add(time.Hour).UnixMilli(), "observedAt": now})
	s, err := ParseNative("cursor-usage", b, now, 5*time.Minute)
	if err != nil || s.Evaluate("cursor", "dynamic-model", now) != Available {
		t.Fatalf("verified camelCase schema failed %+v %v", s, err)
	}
}

func TestLoadNativeCursorExportIsBoundedAndFailClosed(t *testing.T) {
	now := time.Now().UTC()
	reset := now.Add(time.Hour).UnixMilli()
	payload, _ := json.Marshal(map[string]any{
		"observed_at":        now,
		"plan_usage":         map[string]any{"auto_percent_used": 20},
		"billing_cycle_end":  reset,
		"auto_bucket_models": []string{"native-auto"},
	})
	path := filepath.Join(t.TempDir(), "cursor-usage.json")
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOWNSHIFT_CURSOR_NATIVE_FILE", path)
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "missing-quota.json"))
	s, present := LoadNative("cursor")
	if !present || s == nil || s.Evaluate("cursor", "native-auto", now) != Available {
		t.Fatalf("native cursor export was not loaded: present=%v snapshot=%+v", present, s)
	}
	if err := os.WriteFile(path, []byte(`{"plan_usage":`), 0600); err != nil {
		t.Fatal(err)
	}
	if s, present := LoadNative("cursor"); !present || s != nil {
		t.Fatalf("invalid configured native export did not fail closed: present=%v snapshot=%+v", present, s)
	}
	if err := os.WriteFile(path, []byte(`{"plan_usage":{"auto_percent_used":20},"billing_cycle_end":9999999999999,"auto_bucket_models":["native-auto"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if s, present := LoadNative("cursor"); !present || s != nil {
		t.Fatalf("timestamp-free native export renewed quota: present=%v snapshot=%+v", present, s)
	}
}

func TestStoreSerializesHarnessesAndRejectsOlderObservation(t *testing.T) {
	now := time.Now().UTC()
	path := filepath.Join(t.TempDir(), "quota.json")
	a, b := fixture(now), fixture(now)
	b.Harness = "cursor"
	var wg sync.WaitGroup
	for _, s := range []Snapshot{a, b} {
		wg.Add(1)
		go func(s Snapshot) {
			defer wg.Done()
			if err := Store(path, s); err != nil {
				t.Error(err)
			}
		}(s)
	}
	wg.Wait()
	data, _ := os.ReadFile(path)
	var f File
	if json.Unmarshal(data, &f) != nil || len(f.Harnesses) != 2 {
		t.Fatal("concurrent harness snapshot lost")
	}
	a.Windows[0].UsedPercent = number(100)
	a.ObservedAt = now
	a.ExpiresAt = now.Add(time.Minute)
	if err := Store(path, a); err != nil {
		t.Fatal(err)
	}
	old := fixture(now)
	if err := Store(path, old); err == nil {
		t.Fatal("older healthy report overwrote exhaustion")
	}
	t.Setenv("DOWNSHIFT_QUOTA_FILE", path)
	loaded, present := Load("codex")
	if !present || loaded.Evaluate("codex", "m", now) != Exhausted {
		t.Fatal("exhaustion lost")
	}
	if _, present = Load("claude-code"); !present {
		t.Fatal("existing quota cache reopened legacy for another harness")
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"harnesses":null}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got, present := Load("codex"); got != nil || !present {
		t.Fatal("malformed cache reopened legacy")
	}
}

func TestCodexCollectOriginalTimeAndLatestInvalidCloses(t *testing.T) {
	now := time.Now().UTC()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	event := func(at time.Time, used any) []byte {
		b, _ := json.Marshal(map[string]any{"timestamp": at, "type": "event_msg", "payload": map[string]any{"type": "token_count", "rate_limits": map[string]any{"limit_id": "codex", "primary": map[string]any{"used_percent": used, "resets_at": now.Add(time.Hour).Unix()}, "secondary": map[string]any{"used_percent": 20, "resets_at": now.Add(time.Hour).Unix()}}}})
		return append(b, '\n')
	}
	old := now.Add(-10 * time.Minute)
	if err := os.WriteFile(path, event(old, 20), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := CollectCodex(path, 5*time.Minute)
	if err != nil || !s.ObservedAt.Equal(old) || s.Evaluate("codex", "m", now) != Stale {
		t.Fatalf("old transcript renewed: %+v %v", s, err)
	}
	for _, bad := range []any{101, "invalid"} {
		data := append(event(now.Add(-time.Minute), 20), event(now, bad)...)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		s, err := CollectCodex(path, 5*time.Minute)
		if err != nil || s.Evaluate("codex", "m", now) != Unknown {
			t.Fatalf("invalid latest fell back healthy: %+v %v", s, err)
		}
	}
}
