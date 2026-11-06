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

func TestCompressPHPUnitCompactsCompletePassingTranscript(t *testing.T) {
	in := readPHPUnitFixture(t, "phpunit_pass_default.txt")
	out, applied, reason := compressPHPUnit(in)
	if !applied {
		t.Fatalf("expected compression, reason=%q", reason)
	}
	for _, fact := range []string{"PHPUnit: PASS", "58 tests", "143 assertions", "00:00.019", "12.00 MB"} {
		if !strings.Contains(string(out), fact) {
			t.Errorf("summary lost %q: %s", fact, out)
		}
	}
	if strings.Contains(string(out), "Runtime:") || len(out) >= len(in) {
		t.Fatalf("expected shorter summary without runtime metadata, got %d bytes of %d", len(out), len(in))
	}
}

func TestCompressPHPUnitPreservesFailureAndAmbiguousOutputByteIdentically(t *testing.T) {
	failure := readPHPUnitFixture(t, "phpunit_fail_actionable.txt")
	cases := [][]byte{
		failure,
		[]byte("PHPUnit 11.5.15 by Sebastian Bergmann and contributors.\n\n........... 11 / 11 (100%)\n\nTime: 00:00.010\n\nOK (12 tests, 12 assertions)\n"),
		[]byte("PHPUnit 11.5.15 by Sebastian Bergmann and contributors.\n\n......\nTests output that must remain visible\n...... 12 / 12 (100%)\n\nOK (12 tests, 12 assertions)\n"),
		[]byte("PHPUnit 11.5.15 by Sebastian Bergmann and contributors.\nWarning: xdebug warning\n\n............ 12 / 12 (100%)\n\nOK (12 tests, 12 assertions)\n"),
		[]byte("PHPUnit 11.5.15 by Sebastian Bergmann and contributors.\n\n............ 12 / 12 (100%)\n\nOK (12 tests, 12 assertions)\nExtra stdout after summary\n"),
		[]byte("PHPUnit 11.5.15 by Sebastian Bergmann and contributors.\n\n............ 12 / 12 (100%)\n\nOK, but there were issues!\n"),
		[]byte("PHPUnit 11.5.15 by Sebastian Bergmann and contributors.\n\n............ 12 / 12 (100%)\n\nOK (12 tests, 12 assertions)"),
	}
	for i, in := range cases {
		out, applied, _ := compressPHPUnit(in)
		if applied || !bytes.Equal(out, in) {
			t.Errorf("case %d must fall back byte-identically: applied=%v output=%q", i, applied, out)
		}
	}
}

func readPHPUnitFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "corpus", name))
	if err != nil {
		t.Fatalf("read PHPUnit fixture %s: %v", name, err)
	}
	return b
}
