// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/desktopview"
)

const desktopFixture = "../compressor/testdata/corpus/agent_desktop_snapshot_chat.txt"

// desktopRun runs a fake agent-desktop that prints the file in $AD_FIXTURE.
func desktopRun(t *testing.T, fixture string, o Options) (Result, string) {
	t.Helper()
	abs, err := filepath.Abs(fixture)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AD_FIXTURE", abs)
	var out, errb bytes.Buffer
	o.Stdin, o.Stdout, o.Stderr, o.Compact = strings.NewReader(""), &out, &errb, true
	r := Run([]string{"agent-desktop", "snapshot", "--app", "Chat", "-i"}, o)
	if errb.Len() > 0 {
		t.Fatalf("stderr: %s", errb.String())
	}
	return r, out.String()
}

// TestRun_AgentDesktopViewThenDelta is the run-level proof: the first read
// prints the whole view with every ref (no fold, although the raw output is
// kept), the second read of the changed window prints only the delta, and
// the marker names the kept raw output both times.
func TestRun_AgentDesktopViewThenDelta(t *testing.T) {
	fakeBin(t, "agent-desktop", `cat "$AD_FIXTURE"`)
	stored := map[string][]byte{}
	keep := func(raw []byte) (string, bool) {
		stored["0123456789ab"] = raw
		return "0123456789ab", true
	}
	store := desktopview.Store{Dir: t.TempDir(), TTL: time.Minute, MaxBytes: 1 << 20}
	key := DesktopScope("sess", []string{"agent-desktop", "snapshot", "--app", "Chat", "-i"})
	o := Options{Keep: keep, Desktop: func(v []byte) []byte { return store.Apply(key, v) }}

	r, out := desktopRun(t, desktopFixture, o)
	if !r.Compacted || r.Family != FamilyAgentDesktop {
		t.Fatalf("first read: compacted=%v family=%q passthrough=%s", r.Compacted, r.Family, r.Passthrough)
	}
	for _, want := range []string{"refs: eN means @s9x0lmz4:eN\n", "e173 button \"More actions\"", "e61 button \"Add reaction\"", "; full output: downshift retrieve 0123456789ab]\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("first read lacks %q", want)
		}
	}

	raw, _ := os.ReadFile(desktopFixture)
	next := strings.ReplaceAll(string(raw), "s9x0lmz4", "s2nd7k1q")
	next = strings.Replace(next, `"name":"Send now","ref_id":"@s2nd7k1q:e175","role":"button","states":["disabled"]`, `"name":"Send now","ref_id":"@s2nd7k1q:e175","role":"button"`, 1)
	changed := filepath.Join(t.TempDir(), "next.txt")
	if err := os.WriteFile(changed, []byte(next), 0o600); err != nil {
		t.Fatal(err)
	}
	r2, out2 := desktopRun(t, changed, o)
	if !r2.Compacted || !strings.Contains(out2, "delta from snapshot s9x0lmz4") || !strings.Contains(out2, "+    e175 button \"Send now\"\n") {
		t.Fatalf("second read is not a delta:\n%s", out2)
	}
	if !strings.HasSuffix(out2, "; full output: downshift retrieve 0123456789ab]\n") || int(r2.EmittedBytes) != len(out2) {
		t.Fatalf("marker or byte count wrong (emitted %d, printed %d):\n%s", r2.EmittedBytes, len(out2), out2)
	}
	if len(out2)*5 > len(out) {
		t.Errorf("delta %d bytes is not much shorter than the view %d bytes", len(out2), len(out))
	}
}

// TestRun_AgentDesktopNoDeltaWithoutKeptRaw: with CCR off there is nothing
// the agent can fall back on, so every read is the whole view.
func TestRun_AgentDesktopNoDeltaWithoutKeptRaw(t *testing.T) {
	fakeBin(t, "agent-desktop", `cat "$AD_FIXTURE"`)
	calls := 0
	o := Options{Desktop: func(v []byte) []byte { calls++; return []byte("DELTA\n") }}
	_, out := desktopRun(t, desktopFixture, o)
	if calls != 0 || strings.Contains(out, "DELTA") || !strings.Contains(out, "refs: eN means") {
		t.Fatalf("calls=%d\n%s", calls, out)
	}
}

// TestRun_AgentDesktopErrorStaysRaw: a failed read (exit 1, ok false) prints
// the envelope byte for byte, with its exit code.
func TestRun_AgentDesktopErrorStaysRaw(t *testing.T) {
	const env = `{"version":"2.6","ok":false,"command":"snapshot","error":{"code":"APP_NOT_FOUND","message":"no app named Slak"}}` + "\n"
	fakeBin(t, "agent-desktop", shScript(env, 1))
	var out, errb bytes.Buffer
	r := Run([]string{"agent-desktop", "snapshot", "--app", "Slak"}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true})
	if r.ExitCode != 1 || r.Compacted || out.String() != env {
		t.Fatalf("exit=%d compacted=%v out=%q", r.ExitCode, r.Compacted, out.String())
	}
}
