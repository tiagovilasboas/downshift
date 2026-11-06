// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "testing"

// Table tests for the command shapes added to the existing playwright, jest,
// vitest, tsc, next, cypress and eslint families, the PHPUnit shape through
// php, and the pnpm exec / yarn runners that reach those tools.
func TestDecideShapes(t *testing.T) {
	cases := []struct {
		name    string
		command string
		rewrite bool
		family  Family
		want    string
		reason  string
	}{
		{"pnpm exec playwright", "pnpm exec playwright test", true, FamilyPlaywright, "'/opt/ds/downshift' compact -- pnpm exec playwright test", ReasonAllowlisted},
		{"pnpm exec playwright flags", "pnpm exec playwright test --reporter=line tests/a.spec.ts", true, FamilyPlaywright, "'/opt/ds/downshift' compact -- pnpm exec playwright test --reporter=line tests/a.spec.ts", ReasonAllowlisted},
		{"yarn playwright", "yarn playwright test", true, FamilyPlaywright, "'/opt/ds/downshift' compact -- yarn playwright test", ReasonAllowlisted},
		{"yarn jest", "yarn jest", true, FamilyJest, "'/opt/ds/downshift' compact -- yarn jest", ReasonAllowlisted},
		{"yarn jest flags", "yarn jest --runInBand src", true, FamilyJest, "'/opt/ds/downshift' compact -- yarn jest --runInBand src", ReasonAllowlisted},
		{"pnpm exec jest", "pnpm exec jest", true, FamilyJest, "'/opt/ds/downshift' compact -- pnpm exec jest", ReasonAllowlisted},
		{"pnpm exec npx jest", "pnpm exec npx jest", true, FamilyJest, "'/opt/ds/downshift' compact -- pnpm exec npx jest", ReasonAllowlisted},
		{"pnpm exec vitest", "pnpm exec vitest run", true, FamilyVitest, "'/opt/ds/downshift' compact -- pnpm exec vitest run", ReasonAllowlisted},
		{"yarn vitest", "yarn vitest run", true, FamilyVitest, "'/opt/ds/downshift' compact -- yarn vitest run", ReasonAllowlisted},
		{"pnpm exec tsc", "pnpm exec tsc --noEmit", true, FamilyTSC, "'/opt/ds/downshift' compact -- pnpm exec tsc --noEmit", ReasonAllowlisted},
		{"yarn tsc", "yarn tsc", true, FamilyTSC, "'/opt/ds/downshift' compact -- yarn tsc", ReasonAllowlisted},
		{"pnpm exec next build", "pnpm exec next build", true, FamilyNextBuild, "'/opt/ds/downshift' compact -- pnpm exec next build", ReasonAllowlisted},
		{"yarn cypress run", "yarn cypress run", true, FamilyCypress, "'/opt/ds/downshift' compact -- yarn cypress run", ReasonAllowlisted},
		{"pnpm exec eslint", "pnpm exec eslint .", true, FamilyESLint, "'/opt/ds/downshift' compact -- pnpm exec eslint .", ReasonAllowlisted},
		{"php phpunit", "php vendor/bin/phpunit", true, FamilyPHPUnit, "'/opt/ds/downshift' compact -- php vendor/bin/phpunit", ReasonAllowlisted},
		{"php dot-slash phpunit", "php ./vendor/bin/phpunit --filter Login", true, FamilyPHPUnit, "'/opt/ds/downshift' compact -- php ./vendor/bin/phpunit --filter Login", ReasonAllowlisted},
		{"playwright with redirect stays a chain segment", "pnpm exec playwright test 2>&1 | tail -20", true, FamilyPlaywright, "'/opt/ds/downshift' compact -- pnpm exec playwright test 2>&1 | tail -20", ReasonAllowlistedCompound},

		{"pnpm exec playwright install", "pnpm exec playwright install", false, "", "", ReasonNotAllowlisted},
		{"pnpm exec prettier write", "pnpm exec prettier --write .", false, "", "", ReasonNotAllowlisted},
		{"pnpm exec git push", "pnpm exec git push", false, "", "", ReasonNotAllowlisted},
		{"pnpm exec git status", "pnpm exec git status", false, "", "", ReasonNotAllowlisted},
		{"pnpm exec rm", "pnpm exec rm -rf dist", false, "", "", ReasonNotAllowlisted},
		{"pnpm exec bare", "pnpm exec", false, "", "", ReasonNotAllowlisted},
		{"yarn nested yarn", "yarn yarn jest", false, "", "", ReasonNotAllowlisted},
		{"pnpm exec nested pnpm", "pnpm exec pnpm test", false, "", "", ReasonNotAllowlisted},
		{"yarn bare", "yarn", false, "", "", ReasonNotAllowlisted},
		{"yarn git push", "yarn git push", false, "", "", ReasonNotAllowlisted},
		{"php paratest", "php vendor/bin/paratest", false, "", "", ReasonNotAllowlisted},
		{"php other script", "php vendor/bin/other", false, "", "", ReasonNotAllowlisted},
		{"composer test", "composer test", false, "", "", ReasonNotAllowlisted},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := Decide(c.command, exe)
			if d.Rewrite != c.rewrite {
				t.Fatalf("Decide(%q).Rewrite = %v, want %v (reason %s)", c.command, d.Rewrite, c.rewrite, d.Reason)
			}
			if d.Reason != c.reason {
				t.Errorf("Decide(%q).Reason = %q, want %q", c.command, d.Reason, c.reason)
			}
			if d.Family != c.family {
				t.Errorf("Decide(%q).Family = %q, want %q", c.command, d.Family, c.family)
			}
			if c.rewrite && d.Command != c.want {
				t.Errorf("Decide(%q).Command = %q, want %q", c.command, d.Command, c.want)
			}
		})
	}
}
