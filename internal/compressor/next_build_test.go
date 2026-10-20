// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompressNextBuildSuccessfulRouteAndChunkTables(t *testing.T) {
	input := readNextBuildFixture(t, "next_build_pass.txt")
	output, applied, reason := compressNextBuild(input)
	if !applied {
		t.Fatalf("expected build output compaction, reason=%q", reason)
	}
	if got, want := string(output), "Next.js Build: SUCCESS (5 routes: 3 app, 2 pages; shared First Load JS 96.7 kB in 8.2s)\n"; got != want {
		t.Fatalf("unexpected summary: got %q, want %q", got, want)
	}
	if len(output) >= len(input) {
		t.Fatalf("expected smaller output, got %d bytes from %d", len(output), len(input))
	}
}

func TestCompressNextBuildPreservesUnsafeOrAmbiguousOutput(t *testing.T) {
	fixtures := []string{
		"next_build_warning.txt",
		"next_build_truncated.txt",
		"next_build_unknown_output.txt",
	}
	for _, name := range fixtures {
		t.Run(strings.TrimSuffix(name, ".txt"), func(t *testing.T) {
			input := readNextBuildFixture(t, name)
			output, applied, _ := compressNextBuild(input)
			if applied || !bytes.Equal(output, input) {
				t.Fatalf("unsafe output must be preserved byte-for-byte: applied=%v output=%q", applied, output)
			}
		})
	}
}

func TestCompressNextBuildPreservesIncompleteAndOversizedInput(t *testing.T) {
	valid := readNextBuildFixture(t, "next_build_pass.txt")
	incomplete := bytes.TrimSuffix(valid, []byte("\n"))
	output, applied, _ := compressNextBuild(incomplete)
	if applied || !bytes.Equal(output, incomplete) {
		t.Fatal("unterminated transcript must be preserved")
	}

	oversized := bytes.Repeat([]byte{'x'}, MaxProcessableBytes+1)
	output, applied, _ = compressNextBuild(oversized)
	if applied || !bytes.Equal(output, oversized) {
		t.Fatal("oversized input must be preserved")
	}
}

func readNextBuildFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "corpus", name))
	if err != nil {
		t.Fatalf("read Next.js build fixture %s: %v", name, err)
	}
	return data
}
