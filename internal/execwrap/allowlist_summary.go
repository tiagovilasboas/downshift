// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// allowlistExamples are representative command forms, one per common family.
// They are data only: AllowlistSummary keeps the ones FamilyOf accepts, so the
// refusal message and the compact steering never name a command the policy
// would refuse. TestAllowlistExamplesAreAllAccepted fails when an example
// stops routing, which keeps this table from going stale silently.
var allowlistExamples = []string{
	"go test",
	"go build",
	"git status",
	"git diff",
	"npm test",
	"pnpm test",
	"yarn test",
	"jest",
	"vitest",
	"pytest",
	"phpunit",
	"cargo test",
	"make",
	"tsc",
	"golangci-lint run",
	"playwright test",
	"gh pr",
	"gh run",
}

// AllowlistSummary is the one-line list of allowlisted command forms, in
// table order, that FamilyOf accepts. It is what the refusal names and what
// the compact steering embeds.
func AllowlistSummary() string {
	var accepted []string
	for _, example := range allowlistExamples {
		if FamilyOf(strings.Fields(example)) != FamilyNone {
			accepted = append(accepted, example)
		}
	}
	return strings.Join(accepted, ", ")
}
