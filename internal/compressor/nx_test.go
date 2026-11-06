// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "testing"

const nxHead = "\n> nx run myapp:build\n\n> myapp@0.0.0 build\n> tsc\n\n"
const nxDone = "\n NX   Successfully ran target build for project myapp (2s)\n"

func TestCompressNx(t *testing.T) {
	cases := []struct {
		name, in, want string // want empty means raw
	}{
		{"clean", nxHead + nxDone, "[x] nx: ran target build for project myapp in 2s\n"},
		{"run-many", "\n> nx run-many --target=build\n\n NX   Running target build for 2 projects\n\n" +
			"> nx run a:build\n> a@1.0.0 build\n> tsc\n\n——————————————————————————————————————————————————\n\n" +
			" NX   Successfully ran target build for 2 projects (5s)\n", "[x] nx: ran target build for 2 projects in 5s\n"},
		{"task output line", nxHead + "dist/index.js 12 kB\n" + nxDone, ""},
		{"warning", nxHead + " NX   Warning: cache disabled\n" + nxDone, ""},
		{"error", nxHead + "error TS2304: nope\n" + nxDone, ""},
		{"failed", nxHead + " NX   Running target build for project myapp failed\n", ""},
		{"stack frame", nxHead + "    at run (/w/x.js:1:1)\n" + nxDone, ""},
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
			if !res.Applied || res.Format != FormatNx || string(res.Output) != tc.want {
				t.Fatalf("got %+v %q", res, res.Output)
			}
		})
	}
}
