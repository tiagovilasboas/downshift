// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// gitConfigReadFlags are the config options that only read. A config call
// without one of them can write or edit, so it stays in FamilyGitOther.
var gitConfigReadFlags = map[string]bool{
	"--get": true, "--get-all": true, "--get-regexp": true, "--get-urlmatch": true,
	"--list": true, "-l": true,
}

// gitConfigFamily names git config (args are the words after "git config") as
// FamilyGitConfig when it carries a read flag, and FamilyGitOther otherwise.
func gitConfigFamily(args []string) Family {
	for _, a := range args {
		if gitConfigReadFlags[a] {
			return FamilyGitConfig
		}
	}
	return FamilyGitOther
}
