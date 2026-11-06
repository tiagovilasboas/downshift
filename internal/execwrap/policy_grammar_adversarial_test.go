// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"reflect"
	"strings"
	"testing"
)

// The adversarial table pins the compound and chain grammar as it is today.
// A refused row must come back unwrapped (no Rewrite, no Command) with the
// reason named; a wrap row must come back with its exact rewrite, segments and
// argv. Test-only: the policy is not changed here. Lines the current code wraps
// against the requested refusal list are not asserted; they are reported.

// grammarPad returns n filler bytes so a command's length is exact.
func grammarPad(n int) string { return strings.Repeat("a", n) }

func TestDecide_GrammarRefusesAdversarialLines(t *testing.T) {
	bareWrapped := "down" + "shift compact -- go test"
	quotedWrapped := "'" + exe + "' compact -- go test"
	sessionWrapped := "'" + exe + "' compact --session=" + hashA + " -- go test"
	cases := []struct {
		name, command, reason string
	}{
		{"pipe to grep", "go test ./... | grep x", ReasonUnsafeChar},
		{"background", "go test ./... &", ReasonUnsafeChar},
		{"lone ampersand in a chain", "go test & go vet", ReasonUnsafeChar},
		{"command substitution", "go test $(pwd)", ReasonUnsafeChar},
		{"backticks", "go test `pwd`", ReasonUnsafeChar},
		{"stdout redirect", "go test ./... > out.txt", ReasonUnsafeChar},
		{"devnull before a pipe", "go test ./... 2>/dev/null | tail -5", ReasonUnsafeChar},
		{"stdin redirect", "go test ./... < in.txt", ReasonUnsafeChar},
		{"subshell", "(go test ./...)", ReasonUnsafeChar},
		{"brace group", "{ go test ./...; }", ReasonUnsafeChar},
		{"mid pipe", "go test ./... | tail -5 | cat", ReasonUnsafeChar},
		{"tail from a line", "go test ./... | tail -n +5", ReasonUnsafeChar},
		{"tail six digits", "go test ./... | tail -999999", ReasonUnsafeChar},
		{"head byte count", "go build ./... | head -c 5", ReasonUnsafeChar},
		{"cd tilde", "cd ~ && go test ./...", ReasonUnsafeChar},
		{"cd tilde path", "cd ~/x && go test ./...", ReasonUnsafeChar},
		{"cd quoted dir", `cd "a b" && go test ./...`, ReasonUnsafeChar},
		{"cd variable", "cd $HOME && go test ./...", ReasonUnsafeChar},
		{"glob", "go test *.go", ReasonUnsafeChar},
		{"comment", "go test ./... # c", ReasonUnsafeChar},
		{"newline", "go test ./...\ngo vet", ReasonUnsafeChar},
		{"heredoc", "cat <<EOF", ReasonUnsafeChar},
		{"heredoc after allowlisted", "go test <<EOF", ReasonUnsafeChar},
		{"guarded rm in a chain", "gofmt -l x; rm -rf build", ReasonGuardedSegment},
		{"guarded rm after and", "go test ./... && rm -rf build", ReasonGuardedSegment},
		{"guarded mysql in a chain", "gofmt -l x; mysql -e x", ReasonGuardedSegment},
		{"already wrapped bare", bareWrapped, ReasonAlreadyWrapped},
		{"already wrapped quoted path", quotedWrapped, ReasonAlreadyWrapped},
		{"already wrapped with session", sessionWrapped, ReasonAlreadyWrapped},
		{"over 512 bytes, simple", "go test " + grammarPad(505), ReasonTooLong},
		{"over 512 bytes, chain", "gofmt -l x; go test " + grammarPad(500), ReasonTooLong},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.command, exe)
			if d.Rewrite || d.Command != "" || d.Reason != tc.reason {
				t.Fatalf("Decide(%q) = %+v, want unwrapped with reason %s", tc.command, d, tc.reason)
			}
		})
	}
}

func TestDecide_GrammarWrapsTheProvenShapes(t *testing.T) {
	w := "'" + exe + "' compact -- "
	cases := []struct {
		name, command, want, argv, reason string
		segments                          []string
	}{
		{"false then go test", "false && go test ./...", "false && " + w + "go test ./...",
			"go test ./...", ReasonAllowlistedChain, []string{"false"}},
		{"failing go test piped to tail", "go test ./... | tail -5", w + "go test ./... | tail -5",
			"go test ./...", ReasonAllowlistedCompound, []string{"tail -5"}},
		{"cd, then piped to tail", "cd x && go test ./... | tail -5", "cd x && " + w + "go test ./... | tail -5",
			"go test ./...", ReasonAllowlistedCompound, []string{"cd x", "tail -5"}},
		{"exactly 512 bytes", "go test " + grammarPad(504), w + "go test " + grammarPad(504),
			"go test " + grammarPad(504), ReasonAllowlisted, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.command, exe)
			if !d.Rewrite || d.Command != tc.want || d.Argv != tc.argv || d.Reason != tc.reason || !reflect.DeepEqual(d.Segments, tc.segments) {
				t.Fatalf("Decide(%q) = %+v, want command %q argv %q segments %q reason %s", tc.command, d, tc.want, tc.argv, tc.segments, tc.reason)
			}
		})
	}
}
