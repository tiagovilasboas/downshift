// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"fmt"
	"strings"
	"testing"
)

func verboseRun(n int) []byte {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "=== RUN   TestThing%d\n--- PASS: TestThing%d (0.00s)\n", i, i)
	}
	b.WriteString("PASS\nok  \texample.com/x/y\t0.512s\n")
	return []byte(b.String())
}

func memKeep(stored *[][]byte) func([]byte) (string, bool) {
	return func(raw []byte) (string, bool) {
		*stored = append(*stored, raw)
		return "0123456789ab", true
	}
}

func TestCompactCaptured(t *testing.T) {
	big := verboseRun(800)
	var stored [][]byte
	cases := []struct {
		name       string
		argv       []string
		family     Family
		data       []byte
		keep       func([]byte) (string, bool)
		compacted  bool
		wantStored bool
		anyResult  bool // only the store is checked
	}{
		{"family compressor, lossless", []string{"go", "test", "-v", "./..."}, FamilyGoTest, verboseRun(20), nil, true, false, false},
		{"unknown command, balanced with CCR", nil, FamilyNone, big, memKeep(&stored), true, true, false},
		{"unknown command, no CCR stays raw", nil, FamilyNone, big, nil, false, false, false},
		{"file read never cut", []string{"cat", "f"}, FamilyFileRead, big, memKeep(&stored), false, false, false},
		{"raw of a fetch never stored", []string{"curl", "-s", "x"}, FamilyCurl, big, memKeep(&stored), false, false, true},
		{"panic in keep is raw", nil, FamilyNone, big, func([]byte) (string, bool) { panic("x") }, false, false, false},
		{"empty output", nil, FamilyNone, nil, memKeep(&stored), false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stored = nil
			got := CompactCaptured(c.argv, c.family, c.data, 0, c.keep, true)
			if c.anyResult {
				if len(stored) > 0 || got.KeptID != "" {
					t.Errorf("raw output stored for %s", c.family)
				}
				return
			}
			if got.Compacted != c.compacted {
				t.Fatalf("compacted = %v (%s/%s), want %v", got.Compacted, got.Passthrough, got.Detail, c.compacted)
			}
			if !got.Compacted && string(got.Output) != string(c.data) {
				t.Error("an output left raw changed")
			}
			if got.Compacted && len(got.Output) >= len(c.data) {
				t.Errorf("compacted output %d bytes from %d", len(got.Output), len(c.data))
			}
			if (len(stored) > 0) != c.wantStored || (c.wantStored && got.KeptID != "0123456789ab") {
				t.Errorf("stored %d, kept id %q", len(stored), got.KeptID)
			}
		})
	}
}

func TestSimpleFamily(t *testing.T) {
	for cmd, want := range map[string]Family{
		"go test ./...":                       FamilyGoTest,
		"CGO_ENABLED=0 go test ./x":           FamilyGoTest,
		"go test ./... | tail -5":             FamilyNone,
		"go test ./... > out.txt":             FamilyNone,
		"downshift compact -- ls":             FamilyNone,
		"":                                    FamilyNone,
		"go test " + strings.Repeat("x", 600): FamilyNone,
	} {
		if got, _ := SimpleFamily(cmd); got != want {
			t.Errorf("SimpleFamily(%q) = %q, want %q", cmd, got, want)
		}
	}
}

func TestIsWrappedAndHasMarker(t *testing.T) {
	for cmd, want := range map[string]bool{
		"'/usr/bin/downshift' compact -- git status":                           true,
		"'/b/downshift' compact --session=ab --harness=claude-code -- go test": true,
		"downshift compact -- go test ./...":                                   true,
		"cd x && '/b/downshift' compact -- go test":                            true,
		"go test ./... | tail -5":                                              false,
		"git commit -m compaction":                                             false,
	} {
		if got := IsWrapped(cmd); got != want {
			t.Errorf("IsWrapped(%q) = %v, want %v", cmd, got, want)
		}
	}
	for text, want := range map[string]bool{
		"ok\n" + MarkerWithID(10, 2, "0123456789ab"):                                   true,
		"ok\n" + Marker(10, 2) + "[downshift compact: exit 1, returned 0 (|| true)]\n": true,
		"+\tconst markerPrefix = \"[downshift compact: \"\nmore\n":                     false,
		"": false,
	} {
		if got := HasMarker(text); got != want {
			t.Errorf("HasMarker(%q) = %v, want %v", text, got, want)
		}
	}
}
