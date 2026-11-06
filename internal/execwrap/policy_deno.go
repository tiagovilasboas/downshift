// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// DOMAIN: Frontend runtimes — Deno.

// Deno family constants.
const (
	FamilyDenoTest  Family = "deno-test"
	FamilyDenoCheck Family = "deno-check"
	FamilyDenoLint  Family = "deno-lint"
)

// denoFamily detects `deno test`, `deno check` and `deno lint`. Their output
// is a log, so a clean run goes through the generic chain only.
//
// `deno test` runs the project's own test code, the same trust `go test` and
// `npm test` already have. Every other subcommand is refused: `run`,
// `install`, `task`, `compile`, `eval`, `repl`, `upgrade` execute arbitrary
// code or change the machine. So are `--watch` (never exits), `--fix`
// (rewrites files) and any argument that names a remote module (`://`),
// because a URL makes deno fetch and run code the project does not own.
func denoFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "deno" {
		return FamilyNone
	}
	for _, a := range argv[2:] {
		if strings.Contains(a, "://") || a == "--fix" || strings.HasPrefix(a, "--watch") {
			return FamilyNone
		}
	}
	switch argv[1] {
	case "test":
		return FamilyDenoTest
	case "check":
		return FamilyDenoCheck
	case "lint":
		return FamilyDenoLint
	}
	return FamilyNone
}
