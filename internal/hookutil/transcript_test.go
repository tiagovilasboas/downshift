// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package hookutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLastInTranscriptTail(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	pick := func(line []byte) string {
		s := string(line)
		if strings.HasPrefix(s, "m=") {
			return strings.TrimPrefix(s, "m=")
		}
		return ""
	}
	big := strings.Repeat("m=head\n"+strings.Repeat("x", 4096)+"\n", TranscriptTailBytes/4096+8) + "m=tail\nnoise\n"
	cases := []struct{ name, path, want string }{
		{"last value wins", write("a.jsonl", "m=one\nnoise\nm=two\n"), "two"},
		{"no match", write("b.jsonl", "noise\n"), ""},
		{"only the tail is read", write("c.jsonl", big), "tail"},
		{"missing file", filepath.Join(dir, "absent.jsonl"), ""},
		{"wrong extension", write("d.txt", "m=x\n"), ""},
		{"empty path", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := LastInTranscriptTail(c.path, pick); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TranscriptModel is the one line-pick both transcript readers share; the
// line type and the container of "model" are the only harness facts.
func TestTranscriptModel(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	claude := write("claude.jsonl", strings.Join([]string{
		`{"type":"user","message":{"role":"user","content":"hi"}}`,
		`{"type":"assistant","message":{"model":"m-one","content":[{"type":"text","text":"assistant"}]}}`,
		`{"type":"assistant","message":{"model":"m-two"}}`,
		`{"type":"assistant","message":{"model":"<synthetic>"}}`,
		`{"type":"system","message":{"model":"m-system"}}`,
		`{"type":"user","message":{"content":"the word assistant and \"type\":\"assistant\" in content"}}`,
		`not json "assistant"`,
	}, "\n")+"\n")
	codex := write("codex.jsonl", strings.Join([]string{
		`{"type":"turn_context","payload":{"model":"gpt-a"}}`,
		`{"type":"event_msg","payload":{"type":"turn_context","model":"gpt-not-a-turn"}}`,
		`{"type":"turn_context","payload":{"model":"gpt-b"}}`,
		`{"type":"turn_context","payload":"gpt-string-payload"}`,
	}, "\n")+"\n")
	cases := []struct{ name, path, lineType, container, want string }{
		{"claude: last real assistant model, placeholder skipped", claude, "assistant", "message", "m-two"},
		{"codex: last turn_context, nested type and non-object payload ignored", codex, "turn_context", "payload", "gpt-b"},
		{"wrong container reads nothing", claude, "assistant", "payload", ""},
		{"type without a model field", claude, "user", "message", ""},
		{"missing file", filepath.Join(dir, "absent.jsonl"), "assistant", "message", ""},
		{"empty path", "", "assistant", "message", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := TranscriptModel(c.path, c.lineType, c.container); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
