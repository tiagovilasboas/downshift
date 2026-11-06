// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: Container exec — `docker exec`, `docker compose exec` and
// `docker-compose exec`, which run a command inside a running container (the
// usual test runner of a Compose service).
// ════════════════════════════════════════════════════════════════════════════

// The wrapper runs the whole docker command and compacts its output. The
// family is the INNER command's (phpunit, npm-test, artisan, ...), so the
// matching compressor runs. The container adds no family of its own.
//
// Only the flags in containerExecFlag are accepted before the service or
// container name. Any other flag (detached run, index, privileged, unknown)
// leaves the command unwrapped.
//
// TTY: -t and -it are not accepted, so a command that asks for a
// pseudo-terminal stays as written. A TTY changes the bytes the command prints
// (carriage returns, colour codes), and the wrapper reads stdout through a
// pipe, so the compressor would not see the stream the user sees. -T and
// --no-TTY turn the TTY off and are accepted. -i only forwards stdin, which
// Run passes through unchanged, so it is accepted.

// containerExecFamily names the family of the inner command of a container
// exec, or FamilyNone when the shape, a flag, a value or the inner command is
// not allowlisted.
func containerExecFamily(argv []string) Family {
	words, ok := containerExecWords(argv)
	if !ok {
		return FamilyNone
	}
	i := 0
	for i < len(words) && strings.HasPrefix(words[i], "-") {
		n := containerExecFlag(words[i:])
		if n == 0 {
			return FamilyNone
		}
		i += n
	}
	// words[i] names the service or container; the rest is the inner command.
	if i >= len(words)-1 || !plainWord(words[i]) {
		return FamilyNone
	}
	inner := words[i+1:]
	if inner[0] == "docker" || inner[0] == "docker-compose" {
		return FamilyNone
	}
	return FamilyOf(inner)
}

// containerExecWords returns the words after "exec" for the three program
// shapes, and false for any other argv.
func containerExecWords(argv []string) ([]string, bool) {
	switch {
	case len(argv) >= 2 && argv[0] == "docker-compose" && argv[1] == "exec":
		return argv[2:], true
	case len(argv) >= 3 && argv[0] == "docker" && argv[1] == "compose" && argv[2] == "exec":
		return argv[3:], true
	case len(argv) >= 2 && argv[0] == "docker" && argv[1] == "exec":
		return argv[2:], true
	}
	return nil, false
}

// containerExecFlag returns how many words the flag at words[0] takes, or 0
// when it is not an accepted flag or its value is not plain.
func containerExecFlag(words []string) int {
	switch words[0] {
	case "-T", "--no-TTY", "-i":
		return 1
	case "-u", "--user", "-w", "--workdir":
		if len(words) >= 2 && plainWord(words[1]) {
			return 2
		}
	case "-e", "--env":
		if len(words) >= 2 && plainWord(words[1]) && strings.Contains(words[1], "=") {
			return 2
		}
	}
	return 0
}

// plainWord reports a non-empty word that is not an option and holds only
// bytes the safe-byte gate allows.
func plainWord(s string) bool {
	return s != "" && !strings.HasPrefix(s, "-") && allSafe(s)
}
