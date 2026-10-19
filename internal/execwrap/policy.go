// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package execwrap is the opt-in shell-command wrapper: a harness-free rewrite
// policy (Decide) that turns an allowlisted shell command into
// `downshift exec -- <argv>`, and the runner (Run) that executes it, keeps the
// exit code and compacts only unambiguously clean output with the internal
// compressor. Nothing here knows a harness; adapters only expose the command
// and emit the rewritten input.
package execwrap

import "strings"

// Family names an allowlisted command family. It is the only command fact
// that reaches telemetry: never the command text, arguments or paths.
type Family string

const (
	FamilyNone      Family = ""
	FamilyGoTest    Family = "go-test"
	FamilyGitStatus Family = "git-status"
)

// WrapPrefix is what an allowlisted command is prefixed with. The command
// after it is the original, token for token.
const WrapPrefix = "downshift exec -- "

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
func Decide(command string) Decision {
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
	return Decision{
		Rewrite: true,
		Command: WrapPrefix + strings.Join(argv, " "),
		Family:  family,
		Reason:  ReasonAllowlisted,
	}
}

// FamilyOf names the allowlisted family of an argv, or FamilyNone. The
// program must be the bare name ("go", "git"), not a path.
func FamilyOf(argv []string) Family {
	if len(argv) < 2 {
		return FamilyNone
	}
	switch {
	case argv[0] == "go" && argv[1] == "test":
		return FamilyGoTest
	case argv[0] == "git" && argv[1] == "status":
		return FamilyGitStatus
	}
	return FamilyNone
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
