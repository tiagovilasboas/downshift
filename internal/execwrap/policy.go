// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package execwrap is the opt-in shell-command wrapper: a harness-free rewrite
// policy (Decide) that turns an allowlisted shell command into
// `downshift compact -- <argv>`, and the runner (Run) that executes it, keeps the
// exit code and compacts only unambiguously clean output with the internal
// compressor. Nothing here knows a harness; adapters only expose the command
// and emit the rewritten input.
//
// policy.go is the router. It owns the policy machinery (Decide, quoting,
// session tagging, the safe-byte gate) and the cross-cutting git family. Each
// action domain owns its own constants and detection in a sibling file:
// frontend, backend, infra, db and observability. FamilyOf only routes.
package execwrap

import (
	"strings"
)

// Family names an allowlisted command family. It is the only command fact
// that reaches telemetry: never the command text, arguments or paths.
//
// Family constants live in the domain file that detects them (policy_*.go);
// the registry test parses every policy file, so a new family is checked
// against the corpus wherever it is declared. Only the git families and the
// FamilyNone sentinel stay here, with the git router.
type Family string

const (
	FamilyNone      Family = ""
	FamilyGitStatus Family = "git-status"
	FamilyGitLog    Family = "git-log"
	FamilyGitDiff   Family = "git-diff"
	FamilyGitShow   Family = "git-show"
	FamilyGitBranch Family = "git-branch"
	FamilyGitFetch  Family = "git-fetch"
	FamilyGitPull   Family = "git-pull"
	FamilyGitPush   Family = "git-push"
	FamilyGitCommit Family = "git-commit"
	FamilyGitAdd    Family = "git-add"
	FamilyGitStash  Family = "git-stash"
	FamilyGitRemote Family = "git-remote"
	FamilyGitTag    Family = "git-tag"
	FamilyGitConfig Family = "git-config-read"
	FamilyGitOther  Family = "git-other"
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
// log it: `<exe> compact -- x` becomes `<exe> compact --session=<hash> -- x`,
// once per wrapped segment of a chain. Only the quoted binary is tagged, so a
// segment left as written is never touched. A session that is not lowercase
// hex, or a command that is not wrapped or already tagged, is returned
// unchanged.
func WithSession(command, session string) string {
	if session == "" || strings.Contains(command, " compact --session=") {
		return command
	}
	for _, c := range session {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return command
		}
	}
	return strings.ReplaceAll(command, "'"+execTail, "' compact --session="+session+" -- ")
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
	// Segments are the other commands of a compound shape (`cd <dir>`,
	// `tail -N`), kept as written; each must also match no deny or ask rule.
	Segments []string
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
//
// A command that fails the safe-byte gate gets one more chance as one of the
// compound shapes in policy_compound.go, then as a `;`/`&&` chain in
// policy_chain.go.
func Decide(command, exe string) Decision {
	d := decideSimple(command, exe)
	if d.Reason != ReasonUnsafeChar {
		return d
	}
	trimmed := strings.Trim(command, " ")
	if d = decideCompound(trimmed, exe); d.Reason != ReasonUnsafeChar {
		return d
	}
	return decideChain(trimmed, exe)
}

// decideSimple is the policy for one command with no shell metacharacter.
func decideSimple(command, exe string) Decision {
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

// SplitSingleWord is for the direct `downshift compact -- "go test ./..."`
// call, where a caller quotes a whole command as one argv word. A lone word
// that holds spaces and only safe bytes is split on those spaces; any other
// argv, including a word with a metacharacter, a tab or a newline, is returned
// unchanged and keeps its refusal. The split never reaches the hook path,
// which already emits one word per argument.
func SplitSingleWord(argv []string) []string {
	if len(argv) != 1 || !strings.Contains(argv[0], " ") || !allSafe(argv[0]) {
		return argv
	}
	if words := strings.Fields(argv[0]); len(words) > 1 {
		return words
	}
	return argv
}

// FamilyOf names the allowlisted family of an argv, or FamilyNone. It is a
// pure router: each action domain owns its detection in a sibling file. The
// cross-cutting git family is handled here because git spans every domain and
// has its own global-option rules.
func FamilyOf(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	if argv[0] == "git" && len(argv) >= 2 {
		return gitFamilyOf(argv[1:])
	}
	if family := frontendFamily(argv); family != FamilyNone {
		return family
	}
	if family := backendFamily(argv); family != FamilyNone {
		return family
	}
	if family := infraFamily(argv); family != FamilyNone {
		return family
	}
	if family := dbFamily(argv); family != FamilyNone {
		return family
	}
	if family := observabilityFamily(argv); family != FamilyNone {
		return family
	}
	if family := toolingFamily(argv); family != FamilyNone {
		return family
	}
	return FamilyNone
}

// gitSubcommands names the family of each git subcommand that has one. Every
// other subcommand is FamilyGitOther.
var gitSubcommands = map[string]Family{
	"status": FamilyGitStatus,
	"log":    FamilyGitLog,
	"diff":   FamilyGitDiff,
	"show":   FamilyGitShow,
	"branch": FamilyGitBranch,
	"fetch":  FamilyGitFetch,
	"pull":   FamilyGitPull,
	"push":   FamilyGitPush,
	"commit": FamilyGitCommit,
	"add":    FamilyGitAdd,
	"stash":  FamilyGitStash,
	"remote": FamilyGitRemote,
	"tag":    FamilyGitTag,
}

// gitConfigReadFlags are the config options that only read. A config call
// without one of them can write or edit, so it stays in FamilyGitOther.
var gitConfigReadFlags = map[string]bool{
	"--get": true, "--get-all": true, "--get-regexp": true, "--get-urlmatch": true,
	"--list": true, "-l": true,
}

// gitFamilyOf names the family of a git command, given its words after
// "git". A first word that starts with "-" is a global option (-c, -C,
// --git-dir, --work-tree, --exec-path, ...) and is never wrapped: a config
// injection or a repository override must not hide behind the wrapper.
func gitFamilyOf(args []string) Family {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return FamilyNone
	}
	if args[0] == "config" {
		for _, a := range args[1:] {
			if gitConfigReadFlags[a] {
				return FamilyGitConfig
			}
		}
		return FamilyGitOther
	}
	if family, ok := gitSubcommands[args[0]]; ok {
		return family
	}
	return FamilyGitOther
}

// safeByte reports whether a byte may appear in a command the policy will
// wrap. The set excludes every shell metacharacter, so a wrapped command can
// hold no pipe, redirection, substitution, quote, glob or newline.
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
