// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "testing"

func TestStripColour(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"plain\n", "plain\n"},
		{"\x1b[32mok\x1b[0m\n", "ok\n"},
		{"\x1b[mreset\n", "reset\n"},
		{"[\x1b[1;34mINFO\x1b[m] Scanning\n", "[INFO] Scanning\n"},
		{"\x1b[38;2;56;56;56m  ──\x1b[m\n", "  ──\n"},        // truecolour, as lefthook prints it
		{"\x1b[38:5:208mx\x1b[0m\n", "x\n"},                  // colon-separated parameters
		{" \x1b[32m✓\x1b[39m a.test.ts\n", " ✓ a.test.ts\n"}, // vitest, piped
	} {
		got, ok := stripColour([]byte(tc.in))
		if !ok || string(got) != tc.want {
			t.Errorf("stripColour(%q) = %q, %v; want %q, true", tc.in, got, ok, tc.want)
		}
	}
	for _, in := range []string{
		"\x1b[2Kerase\n",    // erase line
		"\x1b[1Aup\n",       // cursor up
		"\x1b[?25lhidden\n", // private mode
		"\x1b[10Gcolumn\n",  // cursor to column
		"\x1b(Bcharset\n",   // character set
		"\x1b]8;;https://example.com\x1b\\link\x1b]8;;\x1b\\\n", // hyperlink
		"bare escape\x1b",
		"cut \x1b[32",
		"cut \x1b[32;",
	} {
		if got, ok := stripColour([]byte(in)); ok {
			t.Errorf("stripColour(%q) = %q, true; want false (not colour)", in, got)
		}
	}
}

func TestScreenText(t *testing.T) {
	for _, tc := range []struct{ in, text, reason string }{
		{"ok\n", "ok\n", ""},
		{"\x1b[32mok\x1b[0m\n", "ok\n", ""},
		{"ok\n\x1b[0m", "ok\n", ""}, // a reset after the last newline ends nothing
		{"\x1b[2Kok\n", "", "terminal_escape_preserved"},
		{"\x1b[32mok\x1b[0m", "", "no_final_newline_preserved"},
		{"\x1b[0m", "", "no_final_newline_preserved"},
		{"a\r\nb\n", "", "carriage_return_preserved"},
		{"\x1b[32ma\x1b[0m\r\nb\n", "", "carriage_return_preserved"},
	} {
		text, reason := screenText([]byte(tc.in))
		if string(text) != tc.text || reason != tc.reason {
			t.Errorf("screenText(%q) = %q, %q; want %q, %q", tc.in, text, reason, tc.text, tc.reason)
		}
	}
}
