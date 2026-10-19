// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package simbench

import (
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestScenarios_EmbeddedAreValid(t *testing.T) {
	scs, err := Scenarios()
	if err != nil {
		t.Fatal(err)
	}
	if len(scs) < 8 {
		t.Fatalf("got %d scenarios, want at least 8", len(scs))
	}
	seen := map[string]bool{}
	for _, sc := range scs {
		if seen[sc.Name] {
			t.Errorf("duplicate scenario %s", sc.Name)
		}
		seen[sc.Name] = true
	}
}

func TestLoadFS_RejectsBadScenarios(t *testing.T) {
	good := `{"name":"x","parent_tier":"mid","spawns":1,"tasks":["rename a ({canary})"],"usage":{"input":[1,2],"output":[1,2]}}`
	for _, tc := range []struct{ name, body, want string }{
		{"unknown field", `{"name":"x","bogus":1}`, "unknown field"},
		{"no canary", strings.Replace(good, " ({canary})", "", 1), "without {canary}"},
		{"bad tier", strings.Replace(good, `"mid"`, `"huge"`, 1), "parent_tier"},
		{"bad harness", strings.Replace(good, `"spawns"`, `"harnesses":["nope"],"spawns"`, 1), "unknown harness"},
		{"empty range", strings.Replace(good, `"input":[1,2]`, `"input":[3,2]`, 1), "bad usage range"},
		{"no spawns", strings.Replace(good, `"spawns":1`, `"spawns":0`, 1), "spawns must be positive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fsys := fstest.MapFS{"d/s.json": {Data: []byte(tc.body)}}
			if _, err := LoadFS(fsys, "d"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
	fsys := fstest.MapFS{"d/s.json": {Data: []byte(good)}}
	scs, err := LoadFS(fsys, "d")
	if err != nil || len(scs) != 1 || scs[0].Seed != DefaultSeed {
		t.Fatalf("good scenario: %v %+v", err, scs)
	}
}

func TestProfileDraw_DeterministicAndInRange(t *testing.T) {
	p := Profile{Input: Range{10, 20}, Output: Range{5, 5}, CacheRead: Range{100, 200}, CacheWrite: Range{0, 30}}
	a, b := rand.New(rand.NewPCG(1, 2)), rand.New(rand.NewPCG(1, 2))
	for range 200 {
		u, v := p.draw(a), p.draw(b)
		if u != v {
			t.Fatalf("same seed drew %+v and %+v", u, v)
		}
		read := u.CachedTokens - u.CacheWriteTokens
		if u.InputTokens < 10 || u.InputTokens > 20 || u.OutputTokens != 5 || read < 100 || read > 200 ||
			u.CacheWriteTokens < 0 || u.CacheWriteTokens > 30 {
			t.Fatalf("draw out of range: %+v", u)
		}
	}
}

func TestMaxSavingFraction(t *testing.T) {
	req := core.Model{InputM: 4, OutputM: 20, CacheReadM: 0.2, CacheWriteM: 5}
	ran := core.Model{InputM: 1, OutputM: 5, CacheReadM: 0.1, CacheWriteM: 1.25}
	for _, tc := range []struct {
		name string
		u    telemetry.TokenUsage
		want float64
	}{
		{"uncached only", telemetry.TokenUsage{InputTokens: 10, OutputTokens: 10}, 0.75},
		{"cache reads cap the gap at 50%", telemetry.TokenUsage{CachedTokens: 1000}, 0.5},
		{"mixed takes the largest class gap", telemetry.TokenUsage{InputTokens: 1, CachedTokens: 1000}, 0.75},
		{"nothing billed", telemetry.TokenUsage{}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := maxSavingFraction(req, ran, tc.u); math.Abs(got-tc.want) > 1e-12 {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
	// A pricier target can save nothing.
	if got := maxSavingFraction(ran, req, telemetry.TokenUsage{InputTokens: 5}); got > 0 {
		t.Errorf("upshift bound %v, want <= 0", got)
	}
}

func TestScanPromptFree(t *testing.T) {
	dir := t.TempDir()
	traces := []Trace{{Spawn: Spawn{Task: "rename x in internal/store/user.go (simcanary00000001)"}}}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("clean.jsonl", `{"outcome":"usage","session_id":"ab12"}`)
	if v, err := ScanPromptFree(traces, dir); err != nil || len(v) != 0 {
		t.Fatalf("clean dir: %v %v", v, err)
	}
	for _, leak := range []string{"internal/store/user.go", "simtool9f", "simcanary00000001"} {
		write("leak.json", leak)
		if v, _ := ScanPromptFree(traces, dir); len(v) == 0 {
			t.Errorf("leak %q not found", leak)
		}
	}
	// Agent definitions are input, not hook output.
	os.Remove(filepath.Join(dir, "leak.json"))
	os.MkdirAll(filepath.Join(dir, ".claude"), 0o700)
	write(".claude/def.md", "simcanary00000001")
	if v, _ := ScanPromptFree(traces, dir); len(v) != 0 {
		t.Errorf(".claude fixtures scanned: %v", v)
	}
}

func TestEnvVars_StayInsideRoot(t *testing.T) {
	root := t.TempDir()
	env, err := NewEnv(root, catalog.Load(), "")
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range env.Vars() {
		if strings.HasPrefix(v, "/") && !strings.HasPrefix(v, root) {
			t.Errorf("%s=%s escapes the sandbox", k, v)
		}
	}
	if env.Vars()["DOWNSHIFT_SCORES"] != "off" {
		t.Error("score refresh (the only network call on the hook path) must be off")
	}
	for _, name := range []string{"DOWNSHIFT_NO_ROUTE", "CLAUDE_CODE_SUBAGENT_MODEL", "ANTHROPIC_MODEL"} {
		if !Blank(name) {
			t.Errorf("%s is not blanked before a run", name)
		}
	}
}
