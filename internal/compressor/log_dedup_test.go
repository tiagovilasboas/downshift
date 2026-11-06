// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
)

func timedLines(n int, format string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, format+"\n", 10+i/60, i%60, i)
	}
	return b.String()
}

func TestLogDedupCollapsesNearIdenticalRuns(t *testing.T) {
	in := "service starting\n" +
		timedLines(30, "2026-11-06T10:%02d:%02dZ INFO poll queue depth=%d") +
		"2026-11-06T10:31:00Z INFO shutting down\n" +
		strings.Repeat("retrying connection\n", 12)
	out, ok, _ := compressFileRead([]byte(in), CommandHint{Name: "app.log", First: 1})
	if !ok {
		t.Fatalf("not compacted:\n%s", out)
	}
	text := string(out)
	for _, want := range []string{
		"service starting\n",
		"2026-11-06T10:10:00Z INFO poll queue depth=0\n[... 29 similar lines folded (timestamps and numbers differ); last: 2026-11-06T10:10:29Z INFO poll queue depth=29]\n",
		"2026-11-06T10:31:00Z INFO shutting down\n",
		"retrying connection\n[... 11 identical lines folded]\n",
		"full text: sed -n '1,44p' app.log]\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("view lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "depth=15\n") {
		t.Errorf("a folded line is still shown:\n%s", text)
	}
}

func TestLogDedupKeepsVaryingDangerLines(t *testing.T) {
	in := "boot\n" + timedLines(30, "2026-11-06T10:%02d:%02dZ ERROR upstream failed after %d ms")
	out, ok, reason := compressFileRead([]byte(in), CommandHint{Name: "app.log"})
	if ok || string(out) != in {
		t.Errorf("ok=%v reason=%q: varying error lines were cut", ok, reason)
	}
	same := "boot\n" + strings.Repeat("ERROR upstream failed\n", 45)
	out, ok, _ = compressFileRead([]byte(same), CommandHint{Name: "app.log"})
	if !ok || !strings.Contains(string(out), "[... 44 identical lines folded]") {
		t.Errorf("identical error lines were not counted: ok=%v\n%s", ok, out)
	}
}

func TestLogDedupStaysRaw(t *testing.T) {
	distinct := func(n int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "event %c happened\n", 'a'+i%26)
			fmt.Fprintf(&b, "different line %s\n", strings.Repeat("x", i%7+1))
		}
		return b.String()
	}
	for _, tc := range []struct {
		name, in, cause string
	}{
		{"short", strings.Repeat("same\n", 20), "no_compression_gain"},
		{"no repeats", distinct(30), "log_no_repeats_preserved"},
		{"runs of two", strings.Repeat("a 1\na 2\nb x\n", 20), "log_no_repeats_preserved"},
	} {
		out, ok, reason := compressFileRead([]byte(tc.in), CommandHint{Name: "app.log"})
		if ok || string(out) != tc.in || reason != tc.cause {
			t.Errorf("%s: ok=%v reason=%q (want raw, %q)", tc.name, ok, reason, tc.cause)
		}
	}
	// Only .log files are read as logs.
	in := strings.Repeat("same line\n", 100)
	if out, ok, _ := compressFileRead([]byte(in), CommandHint{Name: "notes.txt"}); ok || string(out) != in {
		t.Error("a .txt file was deduplicated")
	}
}

func TestLogShape(t *testing.T) {
	a := logShape("2026-11-06 10:00:01 req 123e4567-e89b-12d3-a456-426614174000 took 15ms")
	b := logShape("2026-11-06 10:00:02 req 223e4567-e89b-12d3-a456-426614174001 took 9ms")
	if a != b {
		t.Errorf("%q != %q", a, b)
	}
	if logShape("user alice logged in") == logShape("user bob logged in") {
		t.Error("lines that differ in words share a shape")
	}
}
