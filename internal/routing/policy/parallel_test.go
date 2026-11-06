// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePolicy(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestResolveParallelPrecedence(t *testing.T) {
	cases := []struct {
		name               string
		user, harness, pol int
		wantMax            int
		wantSource         string
		wantStrained       int
	}{
		{"default", 0, 0, 0, 6, ParallelFromDefault, 3},
		{"policy", 0, 0, 8, 8, ParallelFromPolicy, 4},
		{"harness over policy", 0, 4, 8, 4, ParallelFromHarness, 2},
		{"user over all", 2, 4, 8, 2, ParallelFromUser, 1},
		{"one stays one", 1, 0, 0, 1, ParallelFromUser, 1},
		{"odd rounds down", 5, 0, 0, 5, ParallelFromUser, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := ResolveParallel(c.user, c.harness, c.pol, nil)
			if p.Max != c.wantMax || p.Source != c.wantSource || p.WhenStrained != c.wantStrained {
				t.Fatalf("got %+v", p)
			}
		})
	}
}

func TestFilesParallelParseAndInvalid(t *testing.T) {
	ok := writePolicy(t, `{"version":1,"orchestration":{"max_parallel":9}}`)
	if n, notes := FilesParallel(Locations{Project: ok}); n != 9 || len(notes) != 0 {
		t.Fatalf("n=%d notes=%v", n, notes)
	}
	for _, bad := range []string{`0`, `-2`, `"6"`, `6.5`, `65`, `null`} {
		bp := writePolicy(t, `{"version":1,"orchestration":{"max_parallel":`+bad+`}}`)
		n, notes := FilesParallel(Locations{Project: bp})
		if bad == "null" {
			continue
		}
		if n != 0 || len(notes) != 1 || !strings.Contains(notes[0], "max_parallel") {
			t.Errorf("%s: n=%d notes=%v", bad, n, notes)
		}
	}
	// An invalid project value falls through to the user file, with a note.
	bp := writePolicy(t, `{"orchestration":{"max_parallel":0}}`)
	n, notes := FilesParallel(Locations{Project: bp, User: ok})
	if n != 9 || len(notes) != 1 {
		t.Fatalf("n=%d notes=%v", n, notes)
	}
	// Project wins over user; no file and no key hold no value.
	other := writePolicy(t, `{"orchestration":{"max_parallel":3}}`)
	if n, _ := FilesParallel(Locations{Project: other, User: ok}); n != 3 {
		t.Fatalf("project should win, got %d", n)
	}
	if n, notes := FilesParallel(Locations{Project: filepath.Join(t.TempDir(), "none.json")}); n != 0 || notes != nil {
		t.Fatalf("n=%d notes=%v", n, notes)
	}
	notObj := writePolicy(t, `{"orchestration":3}`)
	if n, notes := FilesParallel(Locations{Project: notObj}); n != 0 || len(notes) != 1 {
		t.Fatalf("n=%d notes=%v", n, notes)
	}
}

func TestEnvParallel(t *testing.T) {
	get := func(v string) func(string) string { return func(string) string { return v } }
	if n, notes := EnvParallel(get("")); n != 0 || notes != nil {
		t.Fatal("empty is no override")
	}
	if n, _ := EnvParallel(get(" 4 ")); n != 4 {
		t.Fatalf("n=%d", n)
	}
	for _, bad := range []string{"0", "x", "100"} {
		if n, notes := EnvParallel(get(bad)); n != 0 || len(notes) != 1 {
			t.Errorf("%s: n=%d notes=%v", bad, n, notes)
		}
	}
}

// The floor decoder is strict; the orchestration key must not refuse the file.
func TestFloorAcceptsOrchestrationKey(t *testing.T) {
	p := writePolicy(t, `{"version":1,"orchestration":{"max_parallel":4}}`)
	if _, err := LoadFloor(p); err != nil {
		t.Fatal(err)
	}
}
