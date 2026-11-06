// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// DOMAIN: Django management commands that only read or check.

// Django family constants. Their output is a log (generic chain only).
const (
	FamilyDjangoTest       Family = "django-test"
	FamilyDjangoCheck      Family = "django-check"
	FamilyDjangoMigrations Family = "django-migrations"
)

// djangoFamily detects the read-only management commands. `test`, `check`
// and `showmigrations` are accepted as written; `makemigrations` needs
// `--dry-run` and `--noinput` and `migrate` needs `--plan`, so nothing is
// written to disk or to the database. Every other subcommand (runserver,
// shell, dbshell, flush, loaddata, createsuperuser, collectstatic) is refused.
func djangoFamily(argv []string) Family {
	sub, rest, ok := pythonManage(argv)
	if !ok {
		return FamilyNone
	}
	switch sub {
	case "test":
		return FamilyDjangoTest
	case "check":
		return FamilyDjangoCheck
	case "showmigrations":
		return FamilyDjangoMigrations
	case "makemigrations":
		if hasFlag(rest, "--merge", "--empty") {
			return FamilyNone
		}
		// --check alone writes files on Django 4.1 and older, and a prompt
		// would hang: both --dry-run and --noinput are required.
		if hasFlag(rest, "--dry-run") && hasFlag(rest, "--noinput", "--no-input") {
			return FamilyDjangoMigrations
		}
	case "migrate":
		if hasFlag(rest, "--plan") && allWordsIn(rest, djangoMigratePlanFlags) {
			return FamilyDjangoMigrations
		}
	}
	return FamilyNone
}

// djangoMigratePlanFlags is the allowlist for `migrate --plan`: any other flag
// (--prune, --fake) or positional word is refused.
var djangoMigratePlanFlags = map[string]bool{
	"--plan": true, "--noinput": true, "--no-input": true, "--skip-checks": true,
}

// allWordsIn reports whether every word of args is in allowed.
func allWordsIn(args []string, allowed map[string]bool) bool {
	for _, a := range args {
		if !allowed[a] {
			return false
		}
	}
	return true
}
