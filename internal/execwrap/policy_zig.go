// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// DOMAIN: Backend — Zig.

// Zig family constants.
const (
	FamilyZigBuild Family = "zig-build"
	FamilyZigTest  Family = "zig-test"
)

// zigFamily detects `zig build` and `zig test`. Their output is a log, so a
// clean run goes through the generic chain only.
//
// `zig test` runs the project's own tests, the same trust `go test` already
// has. `zig run` and every other subcommand are refused. A `zig build` step
// named `run` (or starting with `run`) executes the built program, so it is
// refused, as are `--watch` and `--test-cmd*` (they run another command or
// never exit).
func zigFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "zig" {
		return FamilyNone
	}
	for _, a := range argv[2:] {
		if strings.HasPrefix(a, "--watch") || strings.HasPrefix(a, "--test-cmd") {
			return FamilyNone
		}
		if argv[1] == "build" && strings.HasPrefix(a, "run") {
			return FamilyNone
		}
	}
	switch argv[1] {
	case "build":
		return FamilyZigBuild
	case "test":
		return FamilyZigTest
	}
	return FamilyNone
}
