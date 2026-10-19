// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"strings"
	"testing"
	"time"
)

// Stdout is not held past HoldBound: a slow program shows its first output
// while it is still running, so a wrapper killed later has shown it, and the
// run is never compacted.
func TestRun_HoldIsBounded(t *testing.T) {
	old := HoldBound
	HoldBound = 100 * time.Millisecond
	t.Cleanup(func() { HoldBound = old })

	var suite strings.Builder
	for i := 0; i < 8; i++ {
		suite.WriteString("ok  \\tgithub.com/x/p\\t0.1s\\n")
	}
	fakeBin(t, "go", "printf '"+suite.String()+"'; sleep 4; printf 'ok  \\tgithub.com/x/q\\t0.1s\\n'")
	var merged lockedBuf
	seen := make(chan string, 1)
	go func() {
		var early string
		for end := time.Now().Add(3 * time.Second); time.Now().Before(end); time.Sleep(25 * time.Millisecond) {
			merged.mu.Lock()
			early = merged.b.String()
			merged.mu.Unlock()
			if strings.Contains(early, "github.com/x/p") {
				break
			}
		}
		seen <- early
	}()
	r := Run([]string{"go", "test"}, Options{Stdin: strings.NewReader(""), Stdout: &merged, Stderr: &merged, Compact: true})
	if early := <-seen; !strings.Contains(early, "github.com/x/p") {
		t.Fatalf("nothing shown while the program was still running: %q", early)
	}
	if r.Compacted || strings.Contains(merged.b.String(), "downshift exec:") {
		t.Fatalf("a run past the hold bound must stay raw: %+v %q", r, merged.b.String())
	}
	if r.RawBytes != r.EmittedBytes {
		t.Fatalf("raw=%d emitted=%d", r.RawBytes, r.EmittedBytes)
	}
}
