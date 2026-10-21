// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"regexp"
	"strings"
)

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: Backend — PHP, Go, Rust, Java, Python, Dart/Flutter
// ════════════════════════════════════════════════════════════════════════════

// Backend family constants
const (
	// Go
	FamilyGoTest Family = "go-test"

	// Python
	FamilyPytest Family = "pytest"

	// Rust
	FamilyCargoTest Family = "cargo-test"

	// Dart/Flutter
	FamilyFlutterTest Family = "flutter-test"
	FamilyDartTest    Family = "dart-test"

	// Java/Kotlin
	FamilyGradle Family = "gradle"
	FamilyMaven  Family = "maven"

	// PHP ecosystem
	FamilyPHPUnit    Family = "phpunit"
	FamilyComposer   Family = "composer"
	FamilyArtisan    Family = "artisan"
	FamilyPHPStan    Family = "phpstan"
	FamilyPsalm      Family = "psalm"
	FamilyPHPCSFixer Family = "php-cs-fixer"
	FamilyPint       Family = "pint"
	FamilyPest       Family = "pest"
	FamilyRector     Family = "rector"

	// Laravel Sail (Docker wrapper)
	FamilySailTest    Family = "sail-test"
	FamilySailArtisan Family = "sail-artisan"
)

// backendFamily detects backend tooling: go, pytest, cargo, flutter, dart, gradle, maven, php ecosystem.
func backendFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}

	// Pytest (standalone)
	if argv[0] == "pytest" {
		return FamilyPytest
	}

	// PHPUnit
	if argv[0] == "phpunit" || argv[0] == "vendor/bin/phpunit" || argv[0] == "./vendor/bin/phpunit" {
		return FamilyPHPUnit
	}

	// Maven
	if isMavenCommand(argv) {
		return FamilyMaven
	}

	// Gradle
	if len(argv) >= 2 && isGradleProgram(argv[0]) && gradleTask(argv[1:]) {
		return FamilyGradle
	}

	// PHP ecosystem
	if family := phpEcosystemFamily(argv); family != FamilyNone {
		return family
	}

	// Laravel Sail
	if family := sailFamily(argv); family != FamilyNone {
		return family
	}

	// Commands with subcommand pattern
	if len(argv) >= 2 {
		switch {
		case argv[0] == "go" && argv[1] == "test":
			return FamilyGoTest
		case argv[0] == "flutter" && argv[1] == "test":
			return FamilyFlutterTest
		case argv[0] == "dart" && argv[1] == "test":
			return FamilyDartTest
		case argv[0] == "cargo" && argv[1] == "test":
			return FamilyCargoTest
		}
	}

	return FamilyNone
}

// phpEcosystemFamily detects PHP tools: composer, artisan, phpstan, psalm, php-cs-fixer, pint, pest, rector.
func phpEcosystemFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	prog := argv[0]

	// Pest (PHP test runner)
	if prog == "pest" || prog == "vendor/bin/pest" || prog == "./vendor/bin/pest" {
		return FamilyPest
	}

	// Rector (dry-run only)
	if prog == "rector" || prog == "vendor/bin/rector" || prog == "./vendor/bin/rector" {
		if len(argv) >= 2 && argv[1] == "process" {
			for _, a := range argv[2:] {
				if a == "--dry-run" {
					return FamilyRector
				}
			}
		}
		return FamilyNone
	}

	// Composer: only read-only and validation commands
	if prog == "composer" && len(argv) >= 2 {
		switch argv[1] {
		case "validate", "check-platform-reqs", "show", "outdated", "licenses", "why", "why-not", "diagnose", "audit":
			return FamilyComposer
		}
	}

	// Artisan: only safe commands (no make:*, migrate, db:*, etc.)
	if (prog == "artisan" || prog == "php" && len(argv) >= 2 && argv[1] == "artisan") && len(argv) >= 2 {
		artisanCmd := argv[1]
		if prog == "php" && len(argv) >= 3 {
			artisanCmd = argv[2]
		}
		switch artisanCmd {
		case "test", "route:list", "config:show", "env", "about", "list", "help":
			return FamilyArtisan
		}
	}

	// PHPStan
	if prog == "phpstan" || prog == "vendor/bin/phpstan" || prog == "./vendor/bin/phpstan" {
		return FamilyPHPStan
	}

	// Psalm
	if prog == "psalm" || prog == "vendor/bin/psalm" || prog == "./vendor/bin/psalm" {
		return FamilyPsalm
	}

	// PHP-CS-Fixer (dry-run only)
	if (prog == "php-cs-fixer" || prog == "vendor/bin/php-cs-fixer" || prog == "./vendor/bin/php-cs-fixer") && len(argv) >= 2 {
		if argv[1] == "fix" {
			for _, a := range argv[2:] {
				if a == "--dry-run" {
					return FamilyPHPCSFixer
				}
			}
		}
		if argv[1] == "check" {
			return FamilyPHPCSFixer
		}
	}

	// Laravel Pint (dry-run only)
	if prog == "pint" || prog == "vendor/bin/pint" || prog == "./vendor/bin/pint" {
		for _, a := range argv[1:] {
			if a == "--test" {
				return FamilyPint
			}
		}
	}

	return FamilyNone
}

// sailFamily detects Laravel Sail commands: test and safe artisan commands.
func sailFamily(argv []string) Family {
	if len(argv) < 2 {
		return FamilyNone
	}

	prog := argv[0]
	if prog != "sail" && prog != "./vendor/bin/sail" && prog != "vendor/bin/sail" {
		return FamilyNone
	}

	cmd := argv[1]

	// sail test
	if cmd == "test" {
		return FamilySailTest
	}

	// sail artisan <safe-cmd>
	if cmd == "artisan" && len(argv) >= 3 {
		switch argv[2] {
		case "test", "route:list", "config:show", "env", "about", "list", "help":
			return FamilySailArtisan
		}
	}

	return FamilyNone
}

// mavenGoals are the lifecycle phases the maven family wraps.
var mavenGoals = map[string]bool{
	"test": true, "verify": true, "package": true, "install": true, "clean": true,
}

// mavenFlags are the only options a wrapped Maven command may carry.
var mavenFlags = map[string]bool{
	"-o": true, "-q": true, "-B": true, "-DskipTests": true, "-DskipITs": true,
}

func isMavenCommand(argv []string) bool {
	switch argv[0] {
	case "mvn", "mvnw", "./mvnw":
		if len(argv) < 2 || !mavenGoals[argv[1]] {
			return false
		}
		for _, a := range argv[2:] {
			if strings.HasPrefix(a, "-") {
				if !mavenFlags[a] {
					return false
				}
			} else if !mavenGoals[a] {
				return false
			}
		}
		return true
	}
	return false
}

func isGradleProgram(p string) bool {
	return p == "gradle" || p == "gradlew" || p == "./gradlew"
}

// gradleTaskName is the exact allowlist of task names.
var gradleTaskName = regexp.MustCompile(`^(?:build|check|clean|test|assemble|lint|connectedAndroidTest|testDebugUnitTest|testReleaseUnitTest|assembleDebug|assembleRelease|lintDebug|lintRelease)$`)

// gradleTask reports whether there is at least one positional task and every
// positional task is allowlisted.
func gradleTask(args []string) bool {
	found := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-x" || a == "--exclude-task" {
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		name := a[strings.LastIndexByte(a, ':')+1:]
		if !gradleTaskName.MatchString(name) {
			return false
		}
		found = true
	}
	return found
}
