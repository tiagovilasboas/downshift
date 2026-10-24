// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: PHP — PHPUnit, Composer, Artisan, Sail, static analysis and formatters
// ════════════════════════════════════════════════════════════════════════════

const (
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

// phpunitBins are the PHPUnit entry points a wrapped command may name.
var phpunitBins = map[string]bool{
	"phpunit": true, "vendor/bin/phpunit": true, "./vendor/bin/phpunit": true,
}

// isPHPUnit reports a PHPUnit run, bare or through the php binary.
func isPHPUnit(argv []string) bool {
	if phpunitBins[argv[0]] {
		return true
	}
	return argv[0] == "php" && len(argv) >= 2 && phpunitBins[argv[1]]
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
