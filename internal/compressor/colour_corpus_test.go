// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// colourEveryLine wraps each line of in in a green SGR sequence, the way a
// test runner that forces colour prints it, and keeps every newline.
func colourEveryLine(in []byte) []byte {
	lines := strings.Split(string(in), "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = "\x1b[32m" + l + "\x1b[0m"
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// Colour never changes the answer: for every fixture in the corpus, the
// coloured transcript compacts to exactly what the plain one compacts to, and
// a transcript that stays raw comes back as its own coloured bytes.
func TestColourNeverChangesTheAnswer(t *testing.T) {
	for _, e := range loadManifest(t) {
		in := readFixture(t, e.file)
		if bytes.IndexByte(in, 0x1b) != -1 {
			continue // already coloured; the clean/preserve suites cover it
		}
		t.Run(e.file, func(t *testing.T) {
			plain := compressor.CompressExit(in, compressor.ModeSafe, 0)
			coloured := colourEveryLine(in)
			got := compressor.CompressExit(coloured, compressor.ModeSafe, 0)
			if got.Applied != plain.Applied || got.OriginalBytes != len(coloured) {
				t.Fatalf("applied=%v (plain %v) reason=%q (plain %q)", got.Applied, plain.Applied, got.Reason, plain.Reason)
			}
			want := coloured
			if plain.Applied {
				want = plain.Output
			}
			if !bytes.Equal(got.Output, want) {
				t.Fatalf("coloured output differs:\n got: %q\nwant: %q", got.Output, want)
			}
		})
	}
}

// Any terminal escape other than colour keeps a clean transcript raw: the
// bytes are a rendering (cursor moves, erased lines, links), not text.
func TestNonColourEscapeKeepsCleanOutputRaw(t *testing.T) {
	for _, e := range loadManifest(t) {
		if e.kind != kindClean {
			continue
		}
		in := readFixture(t, e.file)
		for _, esc := range []string{"\x1b[2K", "\x1b[1A", "\x1b]8;;https://example.com\x1b\\"} {
			mutated := append([]byte(esc), in...)
			r := compressor.CompressExit(mutated, compressor.ModeSafe, 0)
			if r.Applied || !bytes.Equal(r.Output, mutated) || r.Reason != "terminal_escape_preserved" {
				t.Errorf("%s with %q: applied=%v reason=%q", e.file, esc, r.Applied, r.Reason)
			}
		}
	}
}
