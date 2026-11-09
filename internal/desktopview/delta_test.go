// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package desktopview

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

const electron = "../compressor/testdata/corpus/agent_desktop_snapshot_electron.txt"

// views returns the compact view of the electron fixture, and of the same
// window after the agent typed into the message field: the field has a value,
// Send is enabled, and agent-desktop minted a new snapshot id.
func views(t *testing.T) (base, cur []byte) {
	t.Helper()
	raw, err := os.ReadFile(electron)
	if err != nil {
		t.Fatal(err)
	}
	next := strings.ReplaceAll(string(raw), "s9x0lmz4", "s2nd7k1q")
	next = strings.Replace(next, `"ref_id":"@s2nd7k1q:e174","role":"textfield"`, `"ref_id":"@s2nd7k1q:e174","role":"textfield","value":"hello team"`, 1)
	next = strings.Replace(next, `"name":"Send now","ref_id":"@s2nd7k1q:e175","role":"button","states":["disabled"]`, `"name":"Send now","ref_id":"@s2nd7k1q:e175","role":"button"`, 1)
	return compact(t, raw), compact(t, []byte(next))
}

func compact(t *testing.T, in []byte) []byte {
	t.Helper()
	r := compressor.CompressCommandOutput(in, compressor.FormatAgentDesktop, 0)
	if !r.Applied {
		t.Fatalf("view not built: %s", r.Reason)
	}
	return r.Output
}

func TestDeltaSendsOnlyTheChangedLines(t *testing.T) {
	base, cur := views(t)
	d, ok := Delta(base, cur, 42*time.Second)
	if !ok {
		t.Fatal("no delta")
	}
	out := string(d)
	for _, want := range []string{
		"refs: eN means @s2nd7k1q:eN\n",
		"delta from snapshot s9x0lmz4 (same window, read 42s ago): 4 of",
		`-    e174 textfield "Message #general" actions=Click,SetFocus .ql-editor ph="Message #general"` + "\n",
		`+    e174 textfield "Message #general" actions=Click,SetFocus .ql-editor value="hello team" ph="Message #general"` + "\n",
		`-    e175 button "Send now" [disabled]` + "\n",
		`+    e175 button "Send now"` + "\n",
		"@@ line ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("delta lacks %q:\n%s", want, out)
		}
	}
	if len(d)*10 > len(cur) {
		t.Errorf("delta is %d bytes for a %d-byte view; want under a tenth", len(d), len(cur))
	}
	t.Logf("view %d bytes, delta %d bytes", len(cur), len(d))
}

func TestDeltaOfAnUnchangedWindowIsOneLine(t *testing.T) {
	base, _ := views(t)
	raw, _ := os.ReadFile(electron)
	again := compact(t, []byte(strings.ReplaceAll(string(raw), "s9x0lmz4", "s3rdz9")))
	d, ok := Delta(base, again, time.Second)
	if !ok || !strings.Contains(string(d), "no change from snapshot s9x0lmz4") || strings.Contains(string(d), "@@") {
		t.Fatalf("ok=%v\n%s", ok, d)
	}
}

func TestDeltaFallsBackToTheFullView(t *testing.T) {
	base, cur := views(t)
	raw, _ := os.ReadFile(electron)
	// A new ref-able node before the others renumbers every later ref: no
	// line stays the same, so the delta is no shorter than the view.
	shifted := strings.Replace(string(raw), `{"available_actions":["Click"],"name":"Acme"`, `{"available_actions":["Click"],"name":"New","ref_id":"@s9x0lmz4:e0","role":"button"},{"available_actions":["Click"],"name":"Acme"`, 1)
	partial := strings.Replace(string(raw), `"complete":true`, `"complete":false,"nodes_observed":40,"truncated":true`, 1)
	find, _ := os.ReadFile("../compressor/testdata/corpus/agent_desktop_find_buttons.txt")
	cases := map[string][2][]byte{
		"renumbered":         {base, compact(t, []byte(renumber(shifted)))},
		"partial read":       {base, compact(t, []byte(partial))},
		"partial base":       {compact(t, []byte(partial)), cur},
		"find":               {compact(t, find), compact(t, find)},
		"not a view":         {[]byte("hello\n"), cur},
		"different legends":  {[]byte(strings.Replace(string(base), "has Click;", "has Press;", 1)), cur},
		"another window app": {cur, []byte(strings.Replace(string(cur), "agent-desktop snapshot", "agent-desktop find", 1))},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if d, ok := Delta(c[0], c[1], time.Second); ok {
				t.Fatalf("delta sent:\n%s", d)
			}
		})
	}
}

// renumber gives refs e1.. in document order, as agent-desktop would.
func renumber(s string) string {
	var b strings.Builder
	n := 0
	for {
		i := strings.Index(s, `"ref_id":"@s9x0lmz4:e`)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		j := i + len(`"ref_id":"@s9x0lmz4:e`)
		k := j + strings.Index(s[j:], `"`)
		n++
		b.WriteString(s[:j] + itoa(n))
		s = s[k:]
	}
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

func TestStoreApplyKeepsTheBaseAndExpiresIt(t *testing.T) {
	base, cur := views(t)
	now := time.Unix(1_800_000_000, 0)
	s := Store{Dir: filepath.Join(t.TempDir(), "dv"), TTL: DefaultTTL, MaxBytes: DefaultMaxBytes, Now: func() time.Time { return now }}
	if got := s.Apply("session\x00agent-desktop snapshot --app Slack", base); string(got) != string(base) {
		t.Fatal("the first read must be the full view")
	}
	now = now.Add(30 * time.Second)
	if got := s.Apply("session\x00agent-desktop snapshot --app Slack", cur); !strings.Contains(string(got), "delta from snapshot s9x0lmz4 (same window, read 30s ago)") {
		t.Fatalf("second read is not a delta:\n%s", got)
	}
	if got := s.Apply("other\x00agent-desktop snapshot --app Slack", cur); string(got) != string(cur) {
		t.Fatal("another scope must get the full view")
	}
	now = now.Add(DefaultTTL + time.Second)
	if got := s.Apply("session\x00agent-desktop snapshot --app Slack", cur); string(got) != string(cur) {
		t.Fatal("an expired base must give the full view")
	}
	if runtime.GOOS != "windows" {
		files, _ := filepath.Glob(filepath.Join(s.Dir, "*.view"))
		for _, f := range files {
			if info, err := os.Stat(f); err != nil || info.Mode().Perm() != 0o600 {
				t.Errorf("%s: mode %v, want 0600", f, info.Mode().Perm())
			}
			if strings.Contains(f, "Slack") {
				t.Errorf("file name holds the scope: %s", f)
			}
		}
	}
}

func TestStoreDoesNotKeepAnOversizedView(t *testing.T) {
	s := Store{Dir: t.TempDir(), MaxBytes: 10}
	if err := s.Save("k", []byte("more than ten bytes")); err != ErrTooLarge {
		t.Fatalf("err = %v", err)
	}
	if _, _, ok := s.Load("k"); ok {
		t.Fatal("an oversized view was kept")
	}
}
