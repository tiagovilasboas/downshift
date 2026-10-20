// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package execwrap is the opt-in shell-command wrapper: a harness-free rewrite
// policy (Decide) that turns an allowlisted shell command into
// `downshift compact -- <argv>`, and the runner (Run) that executes it, keeps the
// exit code and compacts only unambiguously clean output with the internal
// compressor. Nothing here knows a harness; adapters only expose the command
// and emit the rewritten input.
package execwrap

import (
	"regexp"
	"strings"
)

// Family names an allowlisted command family. It is the only command fact
// that reaches telemetry: never the command text, arguments or paths.
type Family string

const (
	FamilyNone        Family = ""
	FamilyGoTest      Family = "go-test"
	FamilyGitStatus   Family = "git-status"
	FamilyPytest      Family = "pytest"
	FamilyVitest      Family = "vitest"
	FamilyPlaywright  Family = "playwright"
	FamilyCargoTest   Family = "cargo-test"
	FamilyTSC         Family = "tsc"
	FamilyJest        Family = "jest"
	FamilyCypress     Family = "cypress"
	FamilyNextBuild   Family = "next-build"
	FamilyPHPUnit     Family = "phpunit"
	FamilyFlutterTest Family = "flutter-test"
	FamilyDartTest    Family = "dart-test"
	FamilyGradle      Family = "gradle"
	FamilyMaven       Family = "maven"
)

// execTail follows the quoted binary path in a wrapped command.
const execTail = " compact -- "

// QuoteExecutable single-quotes an absolute binary path for the shell. It
// reports false for a path that is not absolute or holds a quote or a control
// character, which the rewrite never emits.
func QuoteExecutable(exe string) (string, bool) {
	if exe == "" || exe[0] != '/' {
		return "", false
	}
	for i := 0; i < len(exe); i++ {
		if exe[i] == '\'' || exe[i] < 0x20 || exe[i] == 0x7f {
			return "", false
		}
	}
	return "'" + exe + "'", true
}

// WithSession puts the hashed session on a wrapped command so the runner can
// log it: `<exe> compact -- x` becomes `<exe> compact --session=<hash> -- x`. A
// session that is not lowercase hex, or a command that is not wrapped, is
// returned unchanged.
func WithSession(command, session string) string {
	if session == "" || strings.Contains(command, " compact --session=") {
		return command
	}
	for _, c := range session {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return command
		}
	}
	return strings.Replace(command, execTail, " compact --session="+session+" -- ", 1)
}

// maxCommandBytes bounds the command the policy considers. Longer commands
// are left alone.
const maxCommandBytes = 512

// Decision is the policy answer for one shell command.
type Decision struct {
	// Rewrite is true only when Command should replace the original.
	Rewrite bool
	// Command is the wrapped command; empty when Rewrite is false.
	Command string
	Family  Family
	// Argv is the command with whitespace normalised: the text a permission
	// rule of the original command is matched against.
	Argv string
	// Reason is a fixed code, safe to log.
	Reason string
}

// Reason codes.
const (
	ReasonAllowlisted    = "allowlisted"
	ReasonEmpty          = "empty"
	ReasonTooLong        = "too_long"
	ReasonUnsafeChar     = "unsafe_character"
	ReasonAlreadyWrapped = "already_wrapped"
	ReasonNotAllowlisted = "not_allowlisted"
	ReasonBadExecutable  = "bad_executable"
)

// Decide applies the rewrite policy to a shell command string. It does not
// know whether the feature is enabled; the caller checks that first.
//
// A command is wrapped only when every byte is in a small safe set (ASCII
// letters, digits, space and - _ . / : = , + @ %), so no pipe, list,
// redirection, expansion, substitution, quote, glob, escape, comment, tilde
// or newline can appear, and its first two words name an allowlisted family.
// Env assignments and sudo fail the family check because the first word must
// be exactly "go" or "git". The shell still parses the wrapped command, so the
// words after the prefix expand exactly as they would have.
//
// exe is the absolute path of the running downshift binary; the wrapped
// command invokes exactly that file, single-quoted, never a name looked up on
// the Bash tool's PATH.
func Decide(command, exe string) Decision {
	trimmed := strings.Trim(command, " ")
	switch {
	case trimmed == "":
		return Decision{Reason: ReasonEmpty}
	case len(trimmed) > maxCommandBytes:
		return Decision{Reason: ReasonTooLong}
	}
	for i := 0; i < len(trimmed); i++ {
		if !safeByte(trimmed[i]) {
			return Decision{Reason: ReasonUnsafeChar}
		}
	}
	argv := strings.Fields(trimmed)
	if argv[0] == "downshift" {
		return Decision{Reason: ReasonAlreadyWrapped}
	}
	family := FamilyOf(argv)
	if family == FamilyNone {
		return Decision{Reason: ReasonNotAllowlisted}
	}
	quoted, ok := QuoteExecutable(exe)
	if !ok {
		return Decision{Family: family, Reason: ReasonBadExecutable}
	}
	return Decision{
		Rewrite: true,
		Command: quoted + execTail + strings.Join(argv, " "),
		Argv:    strings.Join(argv, " "),
		Family:  family,
		Reason:  ReasonAllowlisted,
	}
}

// FamilyOf names the allowlisted family of an argv, or FamilyNone. Commands
// must match an explicit program and subcommand prefix; only PHPUnit also
// accepts its common project-local vendor/bin paths.
func FamilyOf(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	if argv[0] == "pytest" {
		return FamilyPytest
	}
	if argv[0] == "vitest" || (len(argv) >= 2 && argv[0] == "npx" && argv[1] == "vitest") {
		return FamilyVitest
	}
	if argv[0] == "jest" || (len(argv) >= 2 && argv[0] == "npx" && argv[1] == "jest") {
		return FamilyJest
	}
	if argv[0] == "tsc" || (len(argv) >= 2 && argv[0] == "npx" && argv[1] == "tsc") {
		return FamilyTSC
	}
	if (argv[0] == "cypress" && len(argv) >= 2 && argv[1] == "run") || (len(argv) >= 3 && argv[0] == "npx" && argv[1] == "cypress" && argv[2] == "run") {
		return FamilyCypress
	}
	if (argv[0] == "next" && len(argv) >= 2 && argv[1] == "build") || (len(argv) >= 3 && argv[0] == "npx" && argv[1] == "next" && argv[2] == "build") {
		return FamilyNextBuild
	}
	if argv[0] == "phpunit" || argv[0] == "vendor/bin/phpunit" || argv[0] == "./vendor/bin/phpunit" {
		return FamilyPHPUnit
	}
	if isMavenCommand(argv) {
		return FamilyMaven
	}
	if len(argv) >= 2 && isGradleProgram(argv[0]) && gradleTask(argv[1:]) {
		return FamilyGradle
	}
	if len(argv) >= 3 && argv[0] == "npx" && argv[1] == "playwright" && argv[2] == "test" {
		return FamilyPlaywright
	}
	if len(argv) < 2 {
		return FamilyNone
	}
	switch {
	case argv[0] == "go" && argv[1] == "test":
		return FamilyGoTest
	case argv[0] == "git" && argv[1] == "status":
		return FamilyGitStatus
	case argv[0] == "flutter" && argv[1] == "test":
		return FamilyFlutterTest
	case argv[0] == "dart" && argv[1] == "test":
		return FamilyDartTest
	case argv[0] == "cargo" && argv[1] == "test":
		return FamilyCargoTest
	case argv[0] == "playwright" && argv[1] == "test":
		return FamilyPlaywright
	}
	return FamilyNone
}

// mavenGoals are the lifecycle phases the maven family wraps. A phase must be
// the first argument and every other positional must be one too, so option-first forms ("mvn -q test") and plugin goals
// ("mvn spring-boot:run") are never wrapped.
var mavenGoals = map[string]bool{
	"test": true, "verify": true, "package": true, "install": true, "clean": true,
}

// mavenFlags are the only options a wrapped Maven command may carry. Each takes
// no value, so no file, settings or profile path can ride along.
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

// gradleTaskName is the exact allowlist of task names: the plain tasks, and
// the unit-test, assemble and lint tasks of the Debug and Release build types.
// Nothing else is allowlisted; there is no open suffix, so names such as
// testDebugPublish or lintDebugUpload are not wrapped.
var gradleTaskName = regexp.MustCompile(`^(?:build|check|clean|test|assemble|lint|connectedAndroidTest|testDebugUnitTest|testReleaseUnitTest|assembleDebug|assembleRelease|lintDebug|lintRelease)$`)

// gradleTask reports whether there is at least one positional task and every
// positional task is allowlisted (build, check, clean, test*, assemble*,
// lint*), optionally qualified by a project path such as :app:assembleDebug.
// The value of -x / --exclude-task is skipped; any other option that takes a
// separate value leaves that value as a positional word, which fails the
// check and keeps the command unwrapped.
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

func safeByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	switch c {
	case ' ', '-', '_', '.', '/', ':', '=', ',', '+', '@', '%':
		return true
	}
	return false
}
