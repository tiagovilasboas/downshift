// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// A user override replaces the candidates of a harness it names (an empty
// list turns them off) and keeps the embedded ones of every other harness.
func TestSessionCandidatesUserOverride(t *testing.T) {
	embedded, err := decode(embeddedJSON)
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		body string
		want int
	}{
		"names none keeps embedded": {`{"entries":[]}`, len(embedded.SessionCandidates["claude-code"])},
		"empty list turns off":      {`{"entries":[],"session_candidates":{"claude-code":[]}}`, 0},
		"replaces the list":         {`{"entries":[],"session_candidates":{"claude-code":["sonnet"]}}`, 1},
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			p, err := paths.CatalogPath()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if got := load(&strings.Builder{}).SessionCandidates("claude-code"); len(got) != tc.want {
				t.Fatalf("got %v, want %d names", got, tc.want)
			}
		})
	}
}
