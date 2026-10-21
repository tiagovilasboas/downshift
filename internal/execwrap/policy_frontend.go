// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: Frontend — JS/TS bundlers, test runners, linters
// ════════════════════════════════════════════════════════════════════════════

// Frontend family constants
const (
	// Test runners
	FamilyVitest     Family = "vitest"
	FamilyJest       Family = "jest"
	FamilyPlaywright Family = "playwright"
	FamilyCypress    Family = "cypress"

	// Build tools
	FamilyTSC       Family = "tsc"
	FamilyNextBuild Family = "next-build"

	// Vite bundler
	FamilyViteBuild   Family = "vite-build"
	FamilyVitePreview Family = "vite-preview"

	// Turborepo
	FamilyTurboRun Family = "turbo-run"

	// Package manager test/build wrappers
	FamilyNpmTest   Family = "npm-test"
	FamilyNpmBuild  Family = "npm-build"
	FamilyPnpmTest  Family = "pnpm-test"
	FamilyPnpmBuild Family = "pnpm-build"
	FamilyYarnTest  Family = "yarn-test"
	FamilyYarnBuild Family = "yarn-build"
	FamilyBunTest   Family = "bun-test"
	FamilyBunBuild  Family = "bun-build"

	// Linters and formatters
	FamilyESLint   Family = "eslint"
	FamilyPrettier Family = "prettier"
	FamilyBiome    Family = "biome"
)

// frontendFamily detects frontend tooling: vitest, jest, playwright, cypress, tsc, next, vite, turbo.
func frontendFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}

	// Vitest
	if argv[0] == "vitest" || (len(argv) >= 2 && argv[0] == "npx" && argv[1] == "vitest") {
		return FamilyVitest
	}

	// Jest
	if argv[0] == "jest" || (len(argv) >= 2 && argv[0] == "npx" && argv[1] == "jest") {
		return FamilyJest
	}

	// TSC
	if argv[0] == "tsc" || (len(argv) >= 2 && argv[0] == "npx" && argv[1] == "tsc") {
		return FamilyTSC
	}

	// Cypress
	if (argv[0] == "cypress" && len(argv) >= 2 && argv[1] == "run") ||
		(len(argv) >= 3 && argv[0] == "npx" && argv[1] == "cypress" && argv[2] == "run") {
		return FamilyCypress
	}

	// Next.js build
	if (argv[0] == "next" && len(argv) >= 2 && argv[1] == "build") ||
		(len(argv) >= 3 && argv[0] == "npx" && argv[1] == "next" && argv[2] == "build") {
		return FamilyNextBuild
	}

	// Playwright
	if (argv[0] == "playwright" && len(argv) >= 2 && argv[1] == "test") ||
		(len(argv) >= 3 && argv[0] == "npx" && argv[1] == "playwright" && argv[2] == "test") {
		return FamilyPlaywright
	}

	// Vite
	if family := viteFamily(argv); family != FamilyNone {
		return family
	}

	// Turborepo
	if family := turboFamily(argv); family != FamilyNone {
		return family
	}

	// Package managers
	if family := packageManagerFamily(argv); family != FamilyNone {
		return family
	}

	// Linters
	if family := linterFamily(argv); family != FamilyNone {
		return family
	}

	return FamilyNone
}

// viteFamily detects Vite bundler commands: build, preview.
func viteFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}

	prog := argv[0]
	cmdIdx := 1

	// Handle npx vite
	if prog == "npx" && len(argv) >= 2 && argv[1] == "vite" {
		prog = "vite"
		cmdIdx = 2
	}

	if prog != "vite" {
		return FamilyNone
	}

	if len(argv) <= cmdIdx {
		return FamilyNone
	}

	switch argv[cmdIdx] {
	case "build":
		return FamilyViteBuild
	case "preview":
		return FamilyVitePreview
	}

	return FamilyNone
}

// turboFamily detects Turborepo commands: turbo run <task>.
func turboFamily(argv []string) Family {
	if len(argv) < 3 {
		return FamilyNone
	}

	prog := argv[0]
	cmdIdx := 1

	// Handle npx turbo
	if prog == "npx" && len(argv) >= 3 && argv[1] == "turbo" {
		prog = "turbo"
		cmdIdx = 2
	}

	if prog != "turbo" {
		return FamilyNone
	}

	if argv[cmdIdx] != "run" {
		return FamilyNone
	}

	// turbo run <task> — only safe tasks
	if len(argv) > cmdIdx+1 {
		task := argv[cmdIdx+1]
		switch task {
		case "build", "test", "lint", "typecheck", "check":
			return FamilyTurboRun
		}
	}

	return FamilyNone
}

// packageManagerFamily detects npm, pnpm, yarn, bun test/build commands.
// Only "test" and "build" (or "run test", "run build") are allowed.
func packageManagerFamily(argv []string) Family {
	if len(argv) < 2 {
		return FamilyNone
	}
	pm := argv[0]
	cmd := argv[1]
	// Handle "npm run test" / "pnpm run build" forms
	if cmd == "run" && len(argv) >= 3 {
		cmd = argv[2]
	}
	switch pm {
	case "npm":
		switch cmd {
		case "test", "t":
			return FamilyNpmTest
		case "build":
			return FamilyNpmBuild
		}
	case "pnpm":
		switch cmd {
		case "test", "t":
			return FamilyPnpmTest
		case "build":
			return FamilyPnpmBuild
		}
	case "yarn":
		switch cmd {
		case "test":
			return FamilyYarnTest
		case "build":
			return FamilyYarnBuild
		}
	case "bun":
		switch cmd {
		case "test":
			return FamilyBunTest
		case "build":
			return FamilyBunBuild
		}
	}
	return FamilyNone
}

// linterFamily detects eslint, prettier, biome.
func linterFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	prog := argv[0]
	// Also handle npx variants
	if prog == "npx" && len(argv) >= 2 {
		prog = argv[1]
	}
	switch prog {
	case "eslint":
		return FamilyESLint
	case "prettier":
		// Only check mode, not write
		for _, a := range argv {
			if a == "--check" || a == "-c" || a == "--list-different" || a == "-l" {
				return FamilyPrettier
			}
		}
	case "biome":
		if len(argv) >= 2 {
			cmd := argv[1]
			if prog == "npx" && len(argv) >= 3 {
				cmd = argv[2]
			}
			if cmd == "check" || cmd == "lint" || cmd == "format" {
				// format needs --write to actually write
				for _, a := range argv {
					if a == "--write" {
						return FamilyNone
					}
				}
				return FamilyBiome
			}
		}
	}
	return FamilyNone
}
