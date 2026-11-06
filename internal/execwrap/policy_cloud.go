// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
)

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: Database and cloud CLIs — psql, gcloud, ast-grep.
// Each of them takes a command word or a flag that turns a read into a write.
// The families below accept the read shapes they can prove and refuse every
// other spelling, the way the mysql and redis-cli parses do. A refused command
// still runs, only without the compact wrapper.
// ════════════════════════════════════════════════════════════════════════════

// psqlValueFlags take the next word (or the rest of the cluster) as a value;
// psqlPlainFlags take none. Both lists are closed: -f and -o read and write
// files, -L logs to a file, -v sets variables, and a flag the parse does not
// know may hide a command, so every other flag refuses.
var (
	psqlValueFlags = map[string]bool{"h": true, "p": true, "U": true, "d": true, "c": true}
	psqlLongValue  = map[string]bool{"host": true, "port": true, "username": true, "dbname": true, "command": true}
	psqlPlainFlags = map[string]bool{"X": true, "q": true, "A": true, "t": true, "x": true, "w": true, "l": true}
	psqlLongPlain  = map[string]bool{"no-psqlrc": true, "quiet": true, "no-align": true, "tuples-only": true, "expanded": true, "no-password": true, "list": true}
)

// databaseFamily detects psql running one plain read statement (-c), or
// listing databases (-l), or printing its version.
func databaseFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "psql" {
		return FamilyNone
	}
	var stmts []string
	operands := 0
	informational := false
	for i := 1; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-V" || a == "--version" || a == "-?" || a == "--help":
			informational = true
		case strings.HasPrefix(a, "--"):
			name, value, attached := strings.Cut(a[2:], "=")
			switch {
			case psqlLongPlain[name] && !attached:
				informational = informational || name == "list"
			case psqlLongValue[name]:
				if !attached {
					if i+1 >= len(argv) || strings.HasPrefix(argv[i+1], "-") {
						return FamilyNone
					}
					i++
					value = argv[i]
				}
				if name == "command" {
					stmts = append(stmts, value)
				}
			default:
				return FamilyNone
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			var ok bool
			var stmt string
			i, stmt, ok = psqlShortCluster(argv, i)
			if !ok {
				return FamilyNone
			}
			if stmt != "" {
				stmts = append(stmts, stmt)
			}
			if strings.Contains(a, "l") && !strings.ContainsAny(a, "hpUdc") {
				informational = true
			}
		default:
			// dbname and user only; a connection string can carry options.
			if !plainOperand(a) {
				return FamilyNone
			}
			operands++
		}
	}
	if operands > 2 {
		return FamilyNone
	}
	switch {
	case len(stmts) == 1 && mysqlPlainRead(stmts[0]):
		return FamilyPsql
	case len(stmts) == 0 && informational:
		return FamilyPsql
	}
	return FamilyNone
}

// psqlShortCluster reads the short-option word argv[i] (value-less flags, then
// at most one option that takes a value). It returns the index of the last word
// it used and, for -c, the statement. ok is false for a letter the lists do not
// hold or a missing value.
func psqlShortCluster(argv []string, i int) (next int, stmt string, ok bool) {
	word := argv[i][1:]
	for j := 0; j < len(word); j++ {
		c := word[j : j+1]
		switch {
		case psqlPlainFlags[c]:
		case psqlValueFlags[c]:
			value := word[j+1:]
			if value == "" {
				if i+1 >= len(argv) || strings.HasPrefix(argv[i+1], "-") {
					return i, "", false
				}
				i++
				value = argv[i]
			}
			if c == "c" {
				stmt = value
			}
			return i, stmt, true
		default:
			return i, "", false
		}
	}
	return i, "", true
}

// plainOperand reports a word made only of letters, digits, '_', '.' and '-':
// a database or user name, never a connection string (those hold '=' or '://').
func plainOperand(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '.' || r == '-') {
			return false
		}
	}
	return true
}

// gcloudReadVerbs end a command path that only reads. gcloudWriteVerbs are the
// verbs that change state or run something; the first verb found in the path
// decides, so `compute ssh vm describe` is a write and `compute instances
// describe vm` is a read.
var (
	gcloudReadVerbs  = map[string]bool{"describe": true, "list": true, "get": true, "info": true, "read": true, "get-value": true}
	gcloudWriteVerbs = map[string]bool{
		"ssh": true, "scp": true, "delete": true, "create": true, "update": true, "set": true, "unset": true,
		"add": true, "remove": true, "apply": true, "deploy": true, "reset": true, "start": true, "stop": true,
		"resize": true, "patch": true, "import": true, "export": true, "login": true, "revoke": true,
		"activate-service-account": true, "run": true, "execute": true, "submit": true, "enable": true,
		"disable": true, "attach": true, "detach": true, "move": true, "copy": true, "cp": true, "rm": true,
		"mv": true, "rsync": true, "ssh-to": true, "connect": true, "proxy": true, "print-access-token": true,
		"print-identity-token": true, "application-default": true, "impersonate": true,
	}
)

// gcloudFamily detects the read-only GCP CLI: the first verb in the command
// path (the words before the first flag) must be a read verb, and `--command`
// (which runs a program on a VM) or an impersonation flag refuses.
func gcloudFamily(argv []string) Family {
	if len(argv) < 3 || argv[0] != "gcloud" {
		return FamilyNone
	}
	for _, a := range argv[1:] {
		if a == "--command" || strings.HasPrefix(a, "--command=") || strings.HasPrefix(a, "--impersonate-service-account") {
			return FamilyNone
		}
	}
	for _, a := range argv[1:] {
		if strings.HasPrefix(a, "-") {
			break
		}
		switch {
		case gcloudWriteVerbs[a]:
			return FamilyNone
		case gcloudReadVerbs[a]:
			return FamilyGcloud
		}
	}
	return FamilyNone
}

// astUpdateFlags apply a rewrite to the files or open an editor-like session
// that does: ast-grep run/scan with -U or --update-all writes the files, and
// --interactive offers the same per match.
var astUpdateFlags = map[string]bool{"-U": true, "--update-all": true, "-i": true, "--interactive": true}

// astToolFamily detects ast-grep searches. The name `sg` is also the Linux
// command that runs a program under another group (`sg group -c cmd`), so a
// `-c` refuses, and so does any flag that updates files.
func astToolFamily(argv []string) Family {
	if len(argv) == 0 || (argv[0] != "ast-grep" && argv[0] != "sg") {
		return FamilyNone
	}
	for _, a := range argv[1:] {
		if astUpdateFlags[a] || a == "-c" || a == "--command" || strings.HasPrefix(a, "--update-all") || strings.HasPrefix(a, "--interactive") {
			return FamilyNone
		}
		// A short cluster holding U or i (-Ui, -iA) updates too.
		if len(a) > 2 && a[0] == '-' && a[1] != '-' && strings.ContainsAny(a[1:], "Ui") {
			return FamilyNone
		}
	}
	return FamilyAstGrep
}
