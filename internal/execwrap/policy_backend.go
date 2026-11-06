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

	// Dart/Flutter test
	FamilyFlutterTest Family = "flutter-test"
	FamilyDartTest    Family = "dart-test"

	// Flutter toolchain
	FamilyFlutterBuild   Family = "flutter-build"
	FamilyFlutterRun     Family = "flutter-run"
	FamilyFlutterAnalyze Family = "flutter-analyze"
	FamilyFlutterPub     Family = "flutter-pub"
	FamilyFlutterDoctor  Family = "flutter-doctor"
	FamilyFlutterClean   Family = "flutter-clean"
	FamilyFlutterDevices Family = "flutter-devices"

	// Dart toolchain
	FamilyDartRun     Family = "dart-run"
	FamilyDartAnalyze Family = "dart-analyze"
	FamilyDartPub     Family = "dart-pub"
	FamilyDartFormat  Family = "dart-format"
	FamilyDartCompile Family = "dart-compile"
	FamilyDartFix     Family = "dart-fix"

	// Java/Kotlin
	FamilyGradle Family = "gradle"
	FamilyMaven  Family = "maven"
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

	// PHPUnit (bare, vendor/bin, or through php)
	if isPHPUnit(argv) {
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
		case argv[0] == "cargo" && argv[1] == "test":
			return FamilyCargoTest
		}
	}

	// Flutter toolchain
	if family := flutterFamily(argv); family != FamilyNone {
		return family
	}

	// Dart toolchain
	if family := dartFamily(argv); family != FamilyNone {
		return family
	}

	return FamilyNone
}

// flutterFamily detects Flutter CLI commands.
func flutterFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "flutter" {
		return FamilyNone
	}

	cmd := argv[1]
	switch cmd {
	case "test":
		return FamilyFlutterTest
	case "build":
		return FamilyFlutterBuild
	case "run":
		return FamilyFlutterRun
	case "analyze":
		return FamilyFlutterAnalyze
	case "pub":
		// Only read-safe commands
		if len(argv) >= 3 {
			switch argv[2] {
			case "get", "upgrade", "outdated", "deps", "cache":
				return FamilyFlutterPub
			}
		}
	case "doctor":
		return FamilyFlutterDoctor
	case "clean":
		return FamilyFlutterClean
	case "devices":
		return FamilyFlutterDevices
	}
	return FamilyNone
}

// dartFamily detects Dart CLI commands.
func dartFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "dart" {
		return FamilyNone
	}

	cmd := argv[1]
	switch cmd {
	case "test":
		return FamilyDartTest
	case "run":
		return FamilyDartRun
	case "analyze":
		return FamilyDartAnalyze
	case "pub":
		// Only read-safe commands
		if len(argv) >= 3 {
			switch argv[2] {
			case "get", "upgrade", "outdated", "deps", "cache":
				return FamilyDartPub
			}
		}
	case "format":
		// Only check mode (--output=none or --set-exit-if-changed)
		for _, a := range argv[2:] {
			if a == "--output=none" || a == "--set-exit-if-changed" || a == "-o" {
				return FamilyDartFormat
			}
		}
	case "compile":
		return FamilyDartCompile
	case "fix":
		// Only dry-run
		for _, a := range argv[2:] {
			if a == "--dry-run" || a == "-n" {
				return FamilyDartFix
			}
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
