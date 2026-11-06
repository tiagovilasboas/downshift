// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: git grep — match rows (read-only)
// ════════════════════════════════════════════════════════════════════════════

// FamilyGitGrep is git grep -n, whose rows are path:line:text like grep -rn.
const FamilyGitGrep Family = "git-grep"

// gitGrepShort are the clusterable short flags; -e takes the next argument.
// -O (--open-files-in-pager) runs a program, so it is never in the set.
const gitGrepShort = "niwFEI"

var gitGrepLong = []string{"line-number", "ignore-case", "word-regexp", "fixed-strings", "extended-regexp"}

// gitGrepFamily names git grep (args are the words after "git grep") as
// FamilyGitGrep when the form is accepted, and FamilyGitOther otherwise.
func gitGrepFamily(args []string) Family {
	if gitGrepForm(args) {
		return FamilyGitGrep
	}
	return FamilyGitOther
}

// gitGrepForm needs -n, only allowlisted flags, a pattern (operand or -e) and
// optional pathspecs, which may follow a literal "--".
func gitGrepForm(args []string) bool {
	numbered, explicit := false, false
	operands := 0
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			operands += len(args) - i - 1
			return numbered && (explicit || operands > 0)
		case strings.HasPrefix(a, "--"):
			if !inList(gitGrepLong, a[2:]) {
				return false
			}
			numbered = numbered || a == "--line-number"
		case strings.HasPrefix(a, "-") && a != "-":
			cluster := a[1:]
			for j := 0; j < len(cluster); j++ {
				c := cluster[j]
				switch {
				case strings.IndexByte(gitGrepShort, c) >= 0:
					numbered = numbered || c == 'n'
				case c == 'e' && j == len(cluster)-1 && i+1 < len(args):
					explicit = true
					i++
				default:
					return false
				}
			}
		case a == "-":
			return false
		default:
			operands++
		}
	}
	return numbered && (explicit || operands > 0)
}
