// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

func TestFamilyOf_DenoAndZig(t *testing.T) {
	cases := []struct {
		command string
		want    Family
	}{
		{"deno test", FamilyDenoTest},
		{"deno test --no-check src/a_test.ts", FamilyDenoTest},
		{"deno check main.ts", FamilyDenoCheck},
		{"deno lint", FamilyDenoLint},
		{"deno lint src/", FamilyDenoLint},
		{"zig build", FamilyZigBuild},
		{"zig build test --summary all", FamilyZigBuild},
		{"zig build -Doptimize=ReleaseFast", FamilyZigBuild},
		{"zig test src/main.zig", FamilyZigTest},
		// refused: execute arbitrary code, install, or never exit
		{"deno run main.ts", FamilyNone},
		{"deno install -g main.ts", FamilyNone},
		{"deno task dev", FamilyNone},
		{"deno compile main.ts", FamilyNone},
		{"deno eval 1", FamilyNone},
		{"deno upgrade", FamilyNone},
		{"deno", FamilyNone},
		{"deno test --watch", FamilyNone},
		{"deno lint --fix", FamilyNone},
		{"deno test https://example.com/a_test.ts", FamilyNone},
		{"deno check http://example.com/a.ts", FamilyNone},
		{"zig run main.zig", FamilyNone},
		{"zig build run", FamilyNone},
		{"zig build run-server", FamilyNone},
		{"zig build --watch", FamilyNone},
		{"zig test --test-cmd qemu src/main.zig", FamilyNone},
		{"zig fmt .", FamilyNone},
		{"zig build-exe main.zig", FamilyNone},
		{"zig", FamilyNone},
	}
	for _, c := range cases {
		if got := FamilyOf(strings.Fields(c.command)); got != c.want {
			t.Errorf("FamilyOf(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}
