// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// rubocopWriteFlags are the long and short flags that rewrite source files, write
// a report or a generated config. The compressor reads only the default
// (clang) formatter, so a run that changes files or picks another format is
// left as written.
var rubocopWriteFlags = []string{
	"a", "A", "x", "o", "out",
	"auto-correct", "auto-correct-all", "autocorrect", "autocorrect-all",
	"safe-auto-correct", "fix-layout", "auto-gen-config", "regenerate-todo",
}

// refusedRubocop reports whether a rubocop argv writes files or selects a
// formatter other than the default clang one.
func refusedRubocop(argv []string) bool {
	words := argv[1:]
	if namesFlag(words, rubocopWriteFlags...) {
		return true
	}
	for i, w := range words {
		if strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "--") && strings.ContainsAny(w, "aAxo") && !strings.HasPrefix(w, "-f") {
			return true // a short-flag cluster such as -aF
		}
		if v, ok := rubocopFormatValue(words, i); ok && v != "clang" {
			return true
		}
	}
	return false
}

// rubocopFormatValue returns the value of a --format/-f flag at words[i].
func rubocopFormatValue(words []string, i int) (string, bool) {
	w := words[i]
	switch {
	case w == "-f" || w == "--format":
		if i+1 < len(words) {
			return words[i+1], true
		}
		return "", true
	case strings.HasPrefix(w, "--format="):
		return strings.TrimPrefix(w, "--format="), true
	case strings.HasPrefix(w, "-f") && !strings.HasPrefix(w, "--"):
		return strings.TrimPrefix(w, "-f"), true
	}
	return "", false
}
