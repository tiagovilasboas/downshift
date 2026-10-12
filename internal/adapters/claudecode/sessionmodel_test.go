// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTranscript(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSessionModelFromTranscript(t *testing.T) {
	last := `{"type":"assistant","message":{"model":"m-old"}}
{"type":"user","message":{"content":"the word \"assistant\" in a prompt"}}
{"type":"assistant","message":{"model":"m-new"}}
{"type":"assistant","message":{"model":"<synthetic>"}}
`
	cases := []struct {
		name, path, want string
	}{
		{"last real assistant model wins", writeTranscript(t, last), "m-new"},
		{"missing file fails open", filepath.Join(t.TempDir(), "absent.jsonl"), ""},
		{"empty path", "", ""},
		{"not a jsonl", writeTranscript(t, last) + ".txt", ""},
		{"no assistant rows", writeTranscript(t, `{"type":"user"}`+"\n"), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sessionModelFromTranscript(c.path); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// Only the tail is read; the partial first line after the seek is skipped.
func TestSessionModelFromTranscript_ReadsTailOfLargeFile(t *testing.T) {
	filler := `{"type":"assistant","message":{"model":"m-head","content":"` + strings.Repeat("x", 4096) + `"}}` + "\n"
	body := strings.Repeat(filler, (sessionTailBytes/len(filler))+64) + `{"type":"assistant","message":{"model":"m-tail"}}` + "\n"
	if got := sessionModelFromTranscript(writeTranscript(t, body)); got != "m-tail" {
		t.Errorf("got %q, want m-tail", got)
	}
}

func TestInheritsSessionModel(t *testing.T) {
	cases := []struct {
		ti   map[string]any
		want bool
	}{
		{map[string]any{}, true},
		{map[string]any{"subagent_type": "general-purpose"}, true},
		{map[string]any{"subagent_type": "Explore"}, false},
		{map[string]any{"model": "haiku"}, false},
	}
	for _, c := range cases {
		if got := inheritsSessionModel(c.ti); got != c.want {
			t.Errorf("inheritsSessionModel(%v) = %v, want %v", c.ti, got, c.want)
		}
	}
}
