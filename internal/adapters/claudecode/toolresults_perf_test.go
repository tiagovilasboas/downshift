// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// perfIDs are the ids a doctor read asks for at the window's size.
func perfIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("%012x", 0xabc000+i)
	}
	return ids
}

// buildPerfSet writes files of about fileBytes each, in the shape of a real
// transcript: tool_result lines of a few KB that name no retrieve hint, and one
// line in fifty that carries a hint for one of ids.
func buildPerfSet(tb testing.TB, root string, files, fileBytes int, ids []string) {
	tb.Helper()
	// Hex-heavy on purpose: text full of digits and a-f defeats a first-byte
	// skip, as real command output does.
	filler := strings.Repeat("0123456789abcdef ", 180)
	now := time.Now()
	for f := 0; f < files; f++ {
		var lines []string
		size := 0
		for n := 0; size < fileBytes; n++ {
			text := filler
			if n%50 == 0 {
				text = filler + marker(ids[(f+n)%len(ids)])
			}
			line := resultLine(tb, "user", fmt.Sprintf("toolu_%d_%d", f, n), now, text)
			lines = append(lines, line)
			size += len(line) + 1
		}
		writeResultTranscript(tb, root, fmt.Sprintf("p%d", f), "s.jsonl", now.Add(-time.Duration(f)*time.Minute), lines...)
	}
}

// The read must cost one pass over the bytes, not one pass per id: with 200
// ids, per-id scanning of every line took 0.18 s per MiB, so this 12 MiB set
// took over two seconds. The bound is generous for a slow machine.
func TestReadToolResultsCostDoesNotGrowWithIDs(t *testing.T) {
	root := t.TempDir()
	ids := perfIDs(200)
	buildPerfSet(t, root, 6, 2<<20, ids)
	start := time.Now()
	res := readToolResults(root, ids, readLimits{maxFiles: 60, maxFileBytes: 8 << 20, maxTotalBytes: 64 << 20, timeout: time.Minute})
	elapsed := time.Since(start)
	if !res.Readable || res.Truncated || len(res.Samples) == 0 {
		t.Fatalf("got readable=%v truncated=%v samples=%d", res.Readable, res.Truncated, len(res.Samples))
	}
	if elapsed > 1500*time.Millisecond {
		t.Fatalf("12 MiB with %d ids took %v", len(ids), elapsed)
	}
}

func BenchmarkReadToolResults(b *testing.B) {
	root := b.TempDir()
	ids := perfIDs(200)
	buildPerfSet(b, filepath.Join(root, "projects"), 6, 2<<20, ids)
	lim := readLimits{maxFiles: 60, maxFileBytes: 8 << 20, maxTotalBytes: 64 << 20, timeout: time.Minute}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		readToolResults(filepath.Join(root, "projects"), ids, lim)
	}
}
