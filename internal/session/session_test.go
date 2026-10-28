// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/quotamodel"
)

// pin installs c for one test and isolates the package hooks.
func pin(t *testing.T, c Config) {
	t.Helper()
	t.Cleanup(SetConfig(c))
	q, nq, ns := LoadQuotaHook, LoadNativeQuotaHook, LoadNativeSessionHook
	LoadQuotaHook, LoadNativeQuotaHook, LoadNativeSessionHook = nil, nil, nil
	t.Cleanup(func() { LoadQuotaHook, LoadNativeQuotaHook, LoadNativeSessionHook = q, nq, ns })
}

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestConfigFromEnv_OnlyDocumentedLiterals(t *testing.T) {
	env := map[string]string{
		EnvQuotaMode: "Required", EnvExplicitUpshift: "true", EnvDiscovery: "OFF",
	}
	c := ConfigFromEnv(func(k string) string { return env[k] })
	if c.QuotaRequired || c.ExplicitUpshift || c.DiscoveryOff {
		t.Fatalf("non-literal values switched a flag: %+v", c)
	}
	if c.DiscoveryTTL != DefaultDiscoveryTTL || c.DiscoveryTTLInvalid {
		t.Fatalf("unset TTL: %+v", c)
	}
	env = map[string]string{EnvQuotaMode: "required", EnvExplicitUpshift: "1", EnvDiscovery: "off", EnvDiscoveryTTL: "0"}
	c = ConfigFromEnv(func(k string) string { return env[k] })
	if !c.QuotaRequired || !c.ExplicitUpshift || !c.DiscoveryOff || c.DiscoveryTTL != 0 || c.DiscoveryTTLInvalid {
		t.Fatalf("literal values: %+v", c)
	}
	for _, bad := range []string{"-1h", "soon"} {
		c = ConfigFromEnv(func(k string) string {
			if k == EnvDiscoveryTTL {
				return bad
			}
			return ""
		})
		if !c.DiscoveryTTLInvalid {
			t.Errorf("TTL %q should be invalid", bad)
		}
	}
}

func TestSetConfig_RestoresAndFallsBackToLookup(t *testing.T) {
	SetLookup(func(k string) string {
		if k == EnvExplicitUpshift {
			return "1"
		}
		return ""
	})
	t.Cleanup(func() { SetLookup(nil) })
	if !explicitUpshiftEnv() {
		t.Fatal("lookup not used without a pinned config")
	}
	restore := SetConfig(Config{})
	if explicitUpshiftEnv() {
		t.Fatal("pinned config ignored")
	}
	restore()
	if !explicitUpshiftEnv() {
		t.Fatal("restore did not bring back the lookup")
	}
}

func TestSessionFromHook(t *testing.T) {
	if _, ok := SessionFromHook(nil, nil); ok {
		t.Fatal("absent lists must not be a session")
	}
	empty := []string{}
	s, ok := SessionFromHook(nil, &empty)
	if !ok || !s.Known || len(s.IDs) != 0 || !s.Unordered {
		t.Fatalf("empty second list must be a known, unordered, empty session: %+v", s)
	}
	first := []string{"a"}
	s, _ = SessionFromHook(&first, &empty)
	if s.Unordered || !s.Contains("a") {
		t.Fatalf("first list wins and is ordered: %+v", s)
	}
}

func TestLoadSessionFileForID(t *testing.T) {
	for name, body := range map[string]string{"invalid json": "{", "no key": `{"codex":["x"]}`, "wrong type": `{"claude-code":"x"}`} {
		if s := LoadSessionFile("claude-code", write(t, body)); s.Known {
			t.Errorf("%s: want unknown, got %+v", name, s)
		}
	}
	if s := LoadSessionFile("claude-code", filepath.Join(t.TempDir(), "missing")); s.Known {
		t.Error("missing file must be unknown")
	}
	p := write(t, `{
		"claude-code": ["haiku", "sonnet"],
		"sessions": {"claude-code": {"s1": ["opus"]}},
		"quota": {"claude-code": {"exhausted": ["sonnet"]}, "codex": {"included": ["gpt"]}}
	}`)
	s := LoadSessionFileForID("claude-code", "s1", p)
	if !reflect.DeepEqual(s.IDs, []string{"opus"}) {
		t.Errorf("session-scoped list must win: %v", s.IDs)
	}
	s = LoadSessionFileForID("claude-code", "other", p)
	if !reflect.DeepEqual(s.IDs, []string{"haiku", "sonnet"}) {
		t.Errorf("unknown session id falls back to the harness list: %v", s.IDs)
	}
	if !s.CreditsReported || !s.Blocks("sonnet") || s.IsIncluded("gpt") || len(s.Included) != 0 {
		t.Errorf("only this harness's quota marks apply: %+v", s)
	}
	if s := LoadSessionFile("", p); s.Known {
		t.Error("empty harness must be unknown")
	}
}

func TestLoadDiscoveredSession(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	p := write(t, `{"harnesses":{"claude-code":{"fetched_at":"2026-10-21T10:00:00Z","ordered":["haiku","opus"]},"codex":{"fetched_at":"2026-10-21T10:00:00Z","ordered":[]}}}`)
	cases := []struct {
		name string
		c    Config
		h    string
		at   time.Time
		want bool
	}{
		{"fresh", Config{DiscoveredPath: p, DiscoveryTTL: 24 * time.Hour}, "claude-code", now, true},
		{"expired", Config{DiscoveredPath: p, DiscoveryTTL: time.Hour}, "claude-code", now, false},
		{"ttl 0 keeps forever", Config{DiscoveredPath: p}, "claude-code", now.Add(1000 * time.Hour), true},
		{"future beyond skew", Config{DiscoveredPath: p, DiscoveryTTL: 24 * time.Hour}, "claude-code", now.Add(-3 * time.Hour), false},
		{"invalid ttl", Config{DiscoveredPath: p, DiscoveryTTL: 24 * time.Hour, DiscoveryTTLInvalid: true}, "claude-code", now, false},
		{"discovery off", Config{DiscoveredPath: p, DiscoveryTTL: 24 * time.Hour, DiscoveryOff: true}, "claude-code", now, false},
		{"empty ordered", Config{DiscoveredPath: p, DiscoveryTTL: 24 * time.Hour}, "codex", now, false},
		{"other harness", Config{DiscoveredPath: p, DiscoveryTTL: 24 * time.Hour}, "cursor", now, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pin(t, tc.c)
			if got := LoadDiscoveredSession(tc.h, tc.at); got.Known != tc.want {
				t.Fatalf("Known = %v, want %v (%+v)", got.Known, tc.want, got)
			}
		})
	}
}

func TestLoadDiscoveredSession_MCPEntryExpiresAfterOneHour(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	body := func(src string) string {
		return `{"harnesses":{"claude-code":{"source":"` + src + `","fetched_at":"2026-10-28T10:30:00Z","ordered":["a","b"]}}}`
	}
	cases := []struct {
		name, src string
		ttl       time.Duration
		want      bool
	}{
		{"mcp older than 1h expires", "mcp", 24 * time.Hour, false},
		{"discover entry of same age lasts", "cli", 24 * time.Hour, true},
		{"mcp with ttl 0 still capped", "mcp", 0, false},
		{"mcp shorter ttl wins", "mcp", 30 * time.Minute, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pin(t, Config{DiscoveredPath: write(t, body(tc.src)), DiscoveryTTL: tc.ttl})
			load := LoadDiscoveredSession
			if tc.src == "mcp" {
				load = LoadReportedSession
			}
			if got := load("claude-code", now); got.Known != tc.want {
				t.Fatalf("Known = %v, want %v", got.Known, tc.want)
			}
		})
	}
	pin(t, Config{DiscoveredPath: write(t, body("mcp")), DiscoveryTTL: 24 * time.Hour})
	if !LoadReportedSession("claude-code", now.Add(-45*time.Minute)).Known {
		t.Error("mcp entry inside 1h must be known to the providers hint")
	}
	if LoadDiscoveredSession("claude-code", now.Add(-45*time.Minute)).Known {
		t.Error("routing must never read an mcp-reported entry")
	}
	pin(t, Config{DiscoveredPath: write(t, body("cli")), DiscoveryTTL: 24 * time.Hour})
	if LoadReportedSession("claude-code", now).Known {
		t.Error("a detected entry is not an mcp hint")
	}
}

func TestResolveSessionForID_MCPHintNeverBeatsTheUserFile(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	disc := write(t, `{"harnesses":{"cursor":{"source":"mcp","fetched_at":"`+now+`","ordered":["injected"]}}}`)
	user := write(t, `{"cursor":["operator"]}`)
	pin(t, Config{DiscoveredPath: disc, SessionModelsPath: user, DiscoveryTTL: time.Hour})
	s := ResolveSession("cursor")
	if s.Contains("injected") || !s.Contains("operator") {
		t.Errorf("operator file must win over an mcp hint: %v", s.IDs)
	}
}

func TestResolveSessionForID_Precedence(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	disc := write(t, `{"harnesses":{"claude-code":{"fetched_at":"`+now+`","ordered":["discovered"]}}}`)
	user := write(t, `{"claude-code":["user"]}`)
	pin(t, Config{DiscoveredPath: disc, SessionModelsPath: user, DiscoveryTTL: time.Hour})

	hook := []string{"hook"}
	if s := ResolveSession("claude-code", &hook); !s.Contains("hook") {
		t.Errorf("hook list first: %v", s.IDs)
	}
	if s := ResolveSession("claude-code"); !s.Contains("discovered") {
		t.Errorf("discovered before the user file: %v", s.IDs)
	}
	LoadNativeSessionHook = func(string, time.Time) (SessionList, bool) { return UnknownSession(), true }
	if s := ResolveSession("claude-code"); s.Known {
		t.Errorf("a present native export is authoritative even when unknown: %+v", s)
	}
	LoadNativeSessionHook = nil
	pin(t, Config{DiscoveryOff: true, SessionModelsPath: user})
	if s := ResolveSession("claude-code"); !s.Contains("user") {
		t.Errorf("user file last: %v", s.IDs)
	}
}

func TestExplicitUpshiftEnabled(t *testing.T) {
	pin(t, Config{SessionModelsPath: write(t, `{"explicit_upshift": true}`)})
	if !ExplicitUpshiftEnabled() {
		t.Error("file flag ignored")
	}
	pin(t, Config{SessionModelsPath: write(t, `{}`)})
	if ExplicitUpshiftEnabled() {
		t.Error("default must be off")
	}
	pin(t, Config{ExplicitUpshift: true, SessionModelsPath: filepath.Join(t.TempDir(), "none")})
	if !ExplicitUpshiftEnabled() {
		t.Error("env flag ignored")
	}
}

func TestQuotaMarks(t *testing.T) {
	if UnknownSession().Contains("x") || UnknownSession().WithHookQuota(&[]string{"x"}, nil).IsIncluded("x") {
		t.Fatal("an unknown session holds no ids and takes no marks")
	}
	s := KnownSession([]string{"a", "b"})
	if s.CreditsReported || s.QuotaStatus("h", "a") != quotamodel.Unknown {
		t.Fatal("allowlist membership is not credit evidence")
	}
	s = s.WithHookQuota(&[]string{"a"}, &[]string{"b"})
	if !s.CreditsReported || s.QuotaStatus("h", "a") != quotamodel.Available || s.QuotaStatus("h", "b") != quotamodel.Exhausted {
		t.Fatalf("hook marks: %+v", s)
	}
	if s.Contains("") {
		t.Error("empty id never matches")
	}
}

func TestWithUsageQuota_GateClosesOnEvidence(t *testing.T) {
	pin(t, Config{})
	s := KnownSession([]string{"a"}).WithUsageQuota("h", nil)
	if s.QuotaRequired || !s.QuotaAllows("h", "a") {
		t.Fatalf("no evidence in legacy mode keeps routing: %+v", s)
	}
	pin(t, Config{QuotaRequired: true})
	if s := KnownSession([]string{"a"}).WithUsageQuota("h", nil); !s.QuotaRequired || s.QuotaAllows("h", "a") {
		t.Fatalf("required mode with no evidence must hold: %+v", s)
	}
	pin(t, Config{})
	called := false
	LoadQuotaHook = func(string) (*quotamodel.Snapshot, bool) { called = true; return nil, true }
	if s := KnownSession([]string{"a"}).WithUsageQuota("h", nil); !called || !s.QuotaRequired {
		t.Fatalf("present (even malformed) evidence closes the gate: %+v", s)
	}
	native := KnownSession([]string{"a"})
	native.NativeAvailability = true
	called = false
	if s := native.WithUsageQuota("h", nil); called || !s.QuotaRequired {
		t.Fatalf("native availability keeps its own observation: %+v", s)
	}
}
