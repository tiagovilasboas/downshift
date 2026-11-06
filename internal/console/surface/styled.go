// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package surface

import "strings"

// outputFlags pick a report format by hand, so they never get the styled screen.
var outputFlags = []string{"--text", "--markdown"}

// Styled reports whether a report is drawn as the styled screen: a person at a
// terminal on both streams, outside an agent shell, with no plain request and
// no explicit output flag, and not in CI. NO_COLOR does not matter here: the theme drops
// colour itself. Doubt means plain.
func Styled(f Facts) bool {
	env := f.Getenv
	if env == nil {
		env = func(string) string { return "" }
	}
	if !f.StdinTTY || !f.StdoutTTY {
		return false
	}
	if env("DOWNSHIFT_PLAIN") != "" || env("CI") != "" || env("TERM") == "dumb" || agentVar(env) != "" {
		return false
	}
	if machineFlag(f.Args) != "" {
		return false
	}
	for _, a := range f.Args {
		for _, o := range outputFlags {
			if a == o || strings.HasPrefix(a, o+"=") {
				return false
			}
		}
	}
	return true
}
