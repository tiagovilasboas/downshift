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
		{"node_modules dot-slash vitest", "./node_modules/.bin/vitest run", true, FamilyVitest, "'/opt/ds/downshift' compact -- ./node_modules/.bin/vitest run", ReasonAllowlisted},
		{"node_modules vitest", "node_modules/.bin/vitest run vt/ok1.test.ts", true, FamilyVitest, "'/opt/ds/downshift' compact -- node_modules/.bin/vitest run vt/ok1.test.ts", ReasonAllowlisted},
		{"node_modules jest", "./node_modules/.bin/jest --runInBand", true, FamilyJest, "'/opt/ds/downshift' compact -- ./node_modules/.bin/jest --runInBand", ReasonAllowlisted},
		{"node_modules tsc", "./node_modules/.bin/tsc -p .", true, FamilyTSC, "'/opt/ds/downshift' compact -- ./node_modules/.bin/tsc -p .", ReasonAllowlisted},
		{"node_modules playwright", "./node_modules/.bin/playwright test", true, FamilyPlaywright, "'/opt/ds/downshift' compact -- ./node_modules/.bin/playwright test", ReasonAllowlisted},
		{"node_modules eslint", "./node_modules/.bin/eslint .", true, FamilyESLint, "'/opt/ds/downshift' compact -- ./node_modules/.bin/eslint .", ReasonAllowlisted},
		{"node_modules prettier check", "./node_modules/.bin/prettier --check .", true, FamilyPrettier, "'/opt/ds/downshift' compact -- ./node_modules/.bin/prettier --check .", ReasonAllowlisted},
		{"venv dot-slash pytest", "./.venv/bin/pytest -q", true, FamilyPytest, "'/opt/ds/downshift' compact -- ./.venv/bin/pytest -q", ReasonAllowlisted},
		{"venv pytest", ".venv/bin/pytest tests/", true, FamilyPytest, "'/opt/ds/downshift' compact -- .venv/bin/pytest tests/", ReasonAllowlisted},
		{"venv ruff check", "./venv/bin/ruff check .", true, FamilyRuff, "'/opt/ds/downshift' compact -- ./venv/bin/ruff check .", ReasonAllowlisted},
		{"venv python -m pytest", ".venv/bin/python -m pytest", true, FamilyPytest, "'/opt/ds/downshift' compact -- .venv/bin/python -m pytest", ReasonAllowlisted},
		{"playwright with redirect stays a chain segment", "pnpm exec playwright test 2>&1 | tail -20", true, FamilyPlaywright, "'/opt/ds/downshift' compact -- pnpm exec playwright test 2>&1 | tail -20", ReasonAllowlistedCompound},

		{"node_modules vitest watch", "./node_modules/.bin/vitest watch", false, "", "", ReasonNotAllowlisted},
		{"node_modules vitest -w", "./node_modules/.bin/vitest -w", false, "", "", ReasonNotAllowlisted},
		{"node_modules vite preview", "./node_modules/.bin/vite preview", false, "", "", ReasonNotAllowlisted},
		{"node_modules prettier write", "./node_modules/.bin/prettier --write .", false, "", "", ReasonNotAllowlisted},
		{"node_modules package manager", "./node_modules/.bin/npm test", false, "", "", ReasonNotAllowlisted},
		{"node_modules rm", "./node_modules/.bin/rm -rf dist", false, "", "", ReasonNotAllowlisted},
		{"node_modules nested path", "./node_modules/.bin/sub/vitest run", false, "", "", ReasonNotAllowlisted},
		{"venv pip", "./.venv/bin/pip list", false, "", "", ReasonNotAllowlisted},
		{"venv uv run", ".venv/bin/uv run pytest", false, "", "", ReasonNotAllowlisted},
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
