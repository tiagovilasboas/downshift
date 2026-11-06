// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"path"
	"strings"
)

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: Runners — families whose subcommand executes another program
// (`direnv exec`, `lerna exec`, `uv run`, ...), and the guarded programs they
// must never carry into a wrap. `adb shell` is not allowlisted at all: it runs
// a program on the device.
// ════════════════════════════════════════════════════════════════════════════

// guardedPrograms are the programs whose safety is decided by a guard of their
// own (a recursive rm, a mysql statement, a Redis write). A guard looks only at
// argv[0], so a runner or a raw chain segment that carries one of these must
// stay unwrapped.
var guardedPrograms = map[string]bool{
	"rm": true, "grm": true, "mysql": true, "redis-cli": true, "valkey-cli": true,
	// Database and cloud clients, infrastructure tools and remote shells: a raw
	// chain segment that names one rides into the rewritten line, and a harness
	// that approves the rewrite approves it (Codex emits allow). Their own
	// families accept only the read shapes they can prove.
	"psql": true, "kubectl": true, "helm": true, "terraform": true, "tofu": true, "terragrunt": true,
	"pulumi": true, "ansible": true, "ansible-playbook": true, "gcloud": true, "aws": true, "az": true,
	"doctl": true, "flyctl": true, "heroku": true, "vercel": true, "netlify": true, "docker": true,
	"podman": true, "ssh": true, "scp": true, "rsync": true, "dd": true, "mkfs": true, "shred": true,
	"chmod": true, "chown": true, "kill": true, "pkill": true, "killall": true, "reboot": true, "shutdown": true,
	"curl": true, "wget": true, "nc": true, "ast-grep": true, "sg": true,
}

// mentionsGuarded reports a word whose base name is a guarded program. It
// reads every word, not only the first, so `sudo rm`, `env rm`, `xargs rm`,
// `/bin/rm` and `FOO=1 rm` are caught. A false positive only leaves a command
// unwrapped.
func mentionsGuarded(words []string) bool {
	for _, w := range words {
		if guardedPrograms[path.Base(w)] {
			return true
		}
	}
	return false
}

// innerRunnerFamily is family when inner is itself an allowlisted command, and
// FamilyNone otherwise. FamilyOf refuses a recursive rm and an unsafe mysql or
// redis-cli call, so a runner never lends them its own family.
func innerRunnerFamily(family Family, inner []string) Family {
	if len(inner) == 0 || FamilyOf(inner) == FamilyNone {
		return FamilyNone
	}
	return family
}

// direnvFamily names `direnv exec <dir> <cmd...>` by its inner command, and the
// read-only direnv subcommands by their own family.
func direnvFamily(argv []string) Family {
	switch argv[1] {
	case "status", "allow", "deny", "reload", "export":
		return FamilyDirenv
	case "exec":
		if len(argv) >= 4 && !strings.HasPrefix(argv[2], "-") {
			return innerRunnerFamily(FamilyDirenv, argv[3:])
		}
	}
	return FamilyNone
}

// lernaFamily names the lerna subcommands. `lerna exec` is named by its inner
// command.
func lernaFamily(argv []string) Family {
	if len(argv) < 2 {
		return FamilyNone
	}
	switch argv[1] {
	case "run", "list", "info", "diff", "changed":
		return FamilyLerna
	case "exec":
		return lernaExecFamily(argv)
	}
	return FamilyNone
}

// lernaExecFamily names `lerna exec [flags] -- <cmd...>` and `lerna exec <cmd...>`
// by the inner command. Flags before the command are skipped only when a `--`
// separates them from it; without one, a leading flag leaves the command
// unwrapped because its value could be taken for the program.
func lernaExecFamily(argv []string) Family {
	args := argv[2:]
	for i, a := range args {
		if a == "--" {
			return innerRunnerFamily(FamilyLerna, args[i+1:])
		}
	}
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return innerRunnerFamily(FamilyLerna, args)
	}
	return FamilyNone
}

// runsGuarded reports a `<tool> run <cmd...>` whose command line names a
// guarded program: `uv run`, `poetry run` and `pdm run` start any program.
func runsGuarded(argv []string) bool {
	return len(argv) >= 2 && argv[1] == "run" && mentionsGuarded(argv[2:])
}
