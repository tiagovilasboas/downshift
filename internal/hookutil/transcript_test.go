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
