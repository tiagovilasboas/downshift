// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// Leading environment assignments: `NAME=value` words before an allowlisted
// command, as in `FOO=1 go test ./...`. The policy looks past them to find the
// command, then keeps them in front of the quoted binary. The shell applies
// them to the wrapper, whose child inherits the environment, so the command
// runs with the same environment it would have had without the wrapper.
//
// Only the words are classified here. The value is not checked: the safe-byte
// gate in Decide already refuses quotes, `$`, backticks and spaces.

// Names that change what runs, where git or a loader looks, or which control
// downshift itself. An agent command cannot set them: the assignment would
// reach the wrapper and its child, and the permission rules match only the
// command words, so a PATH or GOFLAGS override would run unseen by a rule.
// DOWNSHIFT_EXEC_DEPTH=1 would also make the wrapper stand down.
var reservedEnvNames = map[string]bool{
	"PATH": true, "BASH_ENV": true, "ENV": true, "GOFLAGS": true, "GOENV": true,
	"NODE_OPTIONS": true, "PYTHONPATH": true, "PYTHONSTARTUP": true,
}

var reservedEnvPrefixes = []string{"DOWNSHIFT_", "GIT_", "LD_", "DYLD_"}

// splitEnvAssignments returns the leading assignment words of argv and the
// words that follow them, which begin with the command name.
func splitEnvAssignments(argv []string) (assigns, words []string) {
	n := 0
	for n < len(argv) && isEnvAssignment(argv[n]) {
		n++
	}
	return argv[:n], argv[n:]
}

// isEnvAssignment reports whether word is NAME=value with NAME matching
// [A-Za-z_][A-Za-z0-9_]* and not reserved.
func isEnvAssignment(word string) bool {
	name, _, found := strings.Cut(word, "=")
	if !found || !validEnvName(name) || reservedEnvNames[name] {
		return false
	}
	for _, p := range reservedEnvPrefixes {
		if strings.HasPrefix(name, p) {
			return false
		}
	}
	return true
}

func validEnvName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		letter := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
		if !letter && (i == 0 || c < '0' || c > '9') {
			return false
		}
	}
	return true
}
