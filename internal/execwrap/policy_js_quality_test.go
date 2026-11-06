// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

func TestFamilyOf_JSQuality(t *testing.T) {
	cases := []struct {
		command string
		want    Family
	}{
		{"next lint", FamilyNextLint},
		{"npx next lint", FamilyNextLint},
		{"next lint --dir src", FamilyNextLint},
		{"npm run lint", FamilyJSCheckRun},
		{"npm run typecheck", FamilyJSCheckRun},
		{"npm run type-check", FamilyJSCheckRun},
		{"npm run check", FamilyJSCheckRun},
		{"pnpm lint", FamilyJSCheckRun},
		{"pnpm run lint", FamilyJSCheckRun},
		{"pnpm typecheck", FamilyJSCheckRun},
		{"yarn lint", FamilyJSCheckRun},
		{"yarn run type-check", FamilyJSCheckRun},
		{"bun run lint", FamilyJSCheckRun},
		{"bun run check", FamilyJSCheckRun},
		{"knip", FamilyKnip},
		{"npx knip", FamilyKnip},
		{"knip --production", FamilyKnip},
		{"depcheck", FamilyDepcheck},
		{"npx depcheck ./src", FamilyDepcheck},
		{"madge --circular src", FamilyMadge},
		{"npx madge --circular --extensions ts,tsx src", FamilyMadge},
		// refused: rewrite files, run code, fetch remote code or run arbitrary scripts
		{"next lint --fix", FamilyNone},
		{"npx next lint --fix-type=problem", FamilyNone},
		{"next dev", FamilyNone},
		{"npm run lint -- --fix", FamilyNone},
		{"pnpm run lint --write", FamilyNone},
		{"npm run dev", FamilyNone},
		{"npm run deploy", FamilyNone},
		{"npm lint", FamilyNone},
		{"bun lint", FamilyNone},
		{"bun run start", FamilyNone},
		{"yarn start", FamilyNone},
		{"pnpm run", FamilyNone},
		{"knip --fix", FamilyNone},
		{"npx knip --fix", FamilyNone},
		{"knip --config evil.js", FamilyNone},
		{"depcheck --parsers=./evil.js", FamilyNone},
		{"madge src", FamilyNone},
		{"madge --image graph.svg --circular src", FamilyNone},
		{"madge --circular", FamilyNone},
		{"madge --circular https://example.com/a.js", FamilyNone},
		{"npx knip https://example.com/x", FamilyNone},
		{"npx some-pkg", FamilyNone},
		{"npx", FamilyNone},
		{"node script.js", FamilyNone},
		{"tsx script.ts", FamilyNone},
		{"ts-node script.ts", FamilyNone},
		{"npx tsx script.ts", FamilyNone},
	}
	for _, c := range cases {
		if got := FamilyOf(strings.Fields(c.command)); got != c.want {
			t.Errorf("FamilyOf(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}
