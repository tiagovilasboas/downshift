// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckQuotaStop(t *testing.T) {
	cases := []struct {
		name     string
		readings []QuotaReading
		stop     bool
		contains string
	}{
		{"below the line", []QuotaReading{{Provider: "claude", UsedPct: 40, Known: true}}, false, ""},
		{"past the line", []QuotaReading{{Provider: "claude", UsedPct: 95, Known: true}}, true, "claude quota is 95% used"},
		{"at the line", []QuotaReading{{Provider: "codex", UsedPct: 90, Known: true}}, true, "stop line of 90%"},
		{"unknown stays allow", []QuotaReading{{Provider: "claude", UsedPct: 99}}, false, ""},
		{"rate limited", []QuotaReading{{Provider: "claude", UsedPct: 10, RateLimited: true, Known: true}}, true, "rate limited"},
		{"no readings", nil, false, ""},
		{"second provider stops", []QuotaReading{{Provider: "a", UsedPct: 1, Known: true}, {Provider: "b", UsedPct: 91, Known: true}}, true, "b quota"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, stop := CheckQuotaStop(90, c.readings)
			if stop != c.stop {
				t.Fatalf("stop = %v, want %v", stop, c.stop)
			}
			if stop && !strings.Contains(s.Reason(), c.contains) {
				t.Fatalf("reason %q lacks %q", s.Reason(), c.contains)
			}
		})
	}
}

func TestQuotaStopConfig(t *testing.T) {
	if got := ResolveQuotaStop(0, 0); got != DefaultQuotaStopPct {
		t.Fatalf("default = %v", got)
	}
	if got := ResolveQuotaStop(70, 80); got != 70 {
		t.Fatalf("user wins = %v", got)
	}
	if got := ResolveQuotaStop(0, 80); got != 80 {
		t.Fatalf("file = %v", got)
	}
	env := func(v string) func(string) string { return func(string) string { return v } }
	if n, notes := EnvQuotaStop(env("75.5")); n != 75.5 || len(notes) != 0 {
		t.Fatalf("env = %v %v", n, notes)
	}
	for _, bad := range []string{"0", "101", "abc", "-3"} {
		if n, notes := EnvQuotaStop(env(bad)); n != 0 || len(notes) != 1 {
			t.Fatalf("env %q = %v %v", bad, n, notes)
		}
	}
	dir := t.TempDir()
	proj := filepath.Join(dir, "p.json")
	user := filepath.Join(dir, "u.json")
	if err := os.WriteFile(proj, []byte(`{"orchestration":{"quota_stop_pct":500}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(user, []byte(`{"orchestration":{"quota_stop_pct":85}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	n, notes := FilesQuotaStop(Locations{Project: proj, User: user})
	if n != 85 || len(notes) != 1 {
		t.Fatalf("files = %v %v", n, notes)
	}
	if n, notes := FilesQuotaStop(Locations{Project: filepath.Join(dir, "none.json")}); n != 0 || len(notes) != 0 {
		t.Fatalf("absent = %v %v", n, notes)
	}
}
