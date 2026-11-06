// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ReadOnly reports whether command is one read-only command a harness may
// approve on Downshift's behalf: the rewrite answer of some harnesses (Codex)
// also approves the command, so the user is never asked. It is true only for
// a single command with no shell metacharacter (no list, pipe, background,
// substitution, redirection, quote or newline), no leading NAME=value
// assignment, and an argv in the read-only set below. Every runner, build,
// test and install family executes project code and is never read-only;
// neither is a subcommand or flag that writes local or remote state.
func ReadOnly(command string) bool {
	trimmed := strings.Trim(command, " ")
	if trimmed == "" || len(trimmed) > maxCommandBytes || !allSafe(trimmed) {
		return false
	}
	assigns, argv := splitEnvAssignments(strings.Fields(trimmed))
	if len(assigns) > 0 || len(argv) < 2 {
		return false
	}
	check, ok := readOnlyFamilies[FamilyOf(argv)]
	return ok && check(argv)
}

// readOnlyFamilies maps each family that has a read-only form to the check
// its argv must pass. A family absent from the map is never read-only.
var readOnlyFamilies = map[Family]func(argv []string) bool{
	FamilyGitStatus: gitReadArgs,
	FamilyGitLog:    gitReadArgs,
	FamilyGitDiff:   gitReadArgs,
	FamilyGitShow:   gitReadArgs,
	FamilyGitBranch: func(argv []string) bool { return onlyWords(argv[2:], gitBranchListFlags) },
	FamilyGitRemote: func(argv []string) bool { return onlyWords(argv[2:], gitRemoteListFlags) },
	FamilyGitStash:  func(argv []string) bool { return len(argv) >= 3 && argv[2] == "list" && gitReadArgs(argv) },
	FamilyGitOther:  func(argv []string) bool { return gitReadOther[argv[1]] && gitReadArgs(argv) },
	FamilyGhPr:      ghReadArgs,
	FamilyGhIssue:   ghReadArgs,
	FamilyGhRun:     ghReadArgs,
	FamilyGhRepo:    ghReadArgs,
	FamilyGhSearch:  func(argv []string) bool { return !namesFlag(argv[2:], ghBrowserFlags...) },
	FamilyGoVersion: func([]string) bool { return true },
	// FamilyGoDoc also holds `go tool`, which builds and runs programs: only
	// `go doc` is read-only, and never with -http (it runs pkgsite).
	FamilyGoDoc:  func(argv []string) bool { return argv[1] == "doc" && !namesFlag(argv[2:], "http") },
	FamilyGoEnv:  func(argv []string) bool { return !namesFlag(argv[2:], "w", "u") },
	FamilyGoList: func(argv []string) bool { return !namesFlag(argv[2:], goListWriteFlags...) },
	FamilyMise:   func(argv []string) bool { return versionListSubcommands[argv[1]] },
	FamilyAsdf:   func(argv []string) bool { return versionListSubcommands[argv[1]] },
	FamilyDirenv: func(argv []string) bool { return argv[1] == "status" },
	FamilyLs:     func([]string) bool { return true },
}

// gitReadOther names the read-only git subcommands that have no family of
// their own (FamilyGitOther).
var gitReadOther = map[string]bool{"rev-parse": true, "ls-files": true}

// gitWriteFlags write a file or run a configured program from a read
// subcommand (git diff --output, --ext-diff, --textconv).
var gitWriteFlags = []string{"output", "ext-diff", "textconv"}

// gitBranchListFlags and gitRemoteListFlags only list. A positional word
// would create, rename or delete a branch or remote, so none is accepted.
var gitBranchListFlags = map[string]bool{"-a": true, "--all": true, "-r": true, "--remotes": true,
	"-v": true, "-vv": true, "--verbose": true, "--list": true, "--show-current": true}
var gitRemoteListFlags = map[string]bool{"-v": true, "--verbose": true}

// ghReadSubcommands are the gh subcommands, per command group, that only
// read. `gh workflow` shares the gh-run family, so it is listed too.
var ghReadSubcommands = map[string]map[string]bool{
	"pr":       {"view": true, "list": true, "status": true, "diff": true, "checks": true},
	"issue":    {"view": true, "list": true, "status": true},
	"run":      {"view": true, "list": true},
	"workflow": {"view": true, "list": true},
	"repo":     {"view": true, "list": true},
	"release":  {"view": true, "list": true},
	"gist":     {"view": true, "list": true},
}

// goListWriteFlags make go list run a program or edit go.mod (-mod=mod).
var goListWriteFlags = []string{"toolexec", "exec", "export", "compiled", "mod"}

// versionListSubcommands are the mise/rtx/asdf subcommands that only list.
var versionListSubcommands = map[string]bool{"list": true, "ls": true, "current": true}

func gitReadArgs(argv []string) bool { return !namesFlag(argv[2:], gitWriteFlags...) }

func ghReadArgs(argv []string) bool {
	return len(argv) >= 3 && ghReadSubcommands[argv[1]][argv[2]] && !namesFlag(argv[3:], ghBrowserFlags...)
}

// ghBrowserFlags open a browser instead of printing (gh pr view --web).
var ghBrowserFlags = []string{"web", "w"}

// onlyWords reports whether every word is one of the allowed flags.
func onlyWords(words []string, allowed map[string]bool) bool {
	for _, w := range words {
		if !allowed[w] {
			return false
		}
	}
	return true
}

// namesFlag reports whether a word is one of the named flags, written with one
// or two dashes, alone or followed by "=value".
func namesFlag(words []string, names ...string) bool {
	for _, w := range words {
		if !strings.HasPrefix(w, "-") {
			continue
		}
		bare := strings.TrimLeft(w, "-")
		for _, n := range names {
			if bare == n || strings.HasPrefix(bare, n+"=") {
				return true
			}
		}
	}
	return false
}
