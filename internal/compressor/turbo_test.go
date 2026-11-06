// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

const turboHead = "• Packages in scope: web\n• Running build in 1 packages\nweb:build: cache miss, executing 9f8e7d6c\nweb:build: \nweb:build: > web@0.0.0 build /w\nweb:build: > tsc\nweb:build: \n"
const turboTail = "\n Tasks:    1 successful, 1 total\nCached:    0 cached, 1 total\n  Time:    2.1s \n"

func TestCompressTurbo(t *testing.T) {
	cases := []struct {
		name, in, want string // want empty means raw
	}{
		{"clean", turboHead + turboTail, "[x] turbo: 1 successful, 1 total (0 cached) in 2.1s\n"},
		{"task output line", turboHead + "web:build: dist/index.js 12 kB\n" + turboTail, ""},
		{"warning", turboHead + "web:build: WARNING something\n" + turboTail, ""},
		{"error", turboHead + "web:build: error TS2304\n" + turboTail, ""},
		{"failed task", turboHead + turboTail + " Failed:    web#build\n", ""},
		{"not all successful", strings.Replace(turboHead+turboTail, "1 successful, 1 total", "0 successful, 1 total", 1), ""},
		{"stack frame", turboHead + "web:build:     at run (/w/x.js:1:1)\n" + turboTail, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Compress([]byte(tc.in), ModeSafe)
			if tc.want == "" {
				if res.Applied || string(res.Output) != tc.in {
					t.Fatalf("expected raw, got applied=%v %q", res.Applied, res.Output)
				}
				return
			}
			if !res.Applied || res.Format != FormatTurbo || string(res.Output) != tc.want {
				t.Fatalf("got %+v %q", res, res.Output)
			}
		})
	}
}
