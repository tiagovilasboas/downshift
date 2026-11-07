// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// DOMAIN: Python quality tooling — linters, test runners and coverage.

// Python quality family constants. Their output is a log, so a clean run goes
// through the generic chain (pytest and mypy keep their own formats).
const (
	FamilyFlake8   Family = "flake8"
	FamilyPylint   Family = "pylint"
	FamilyUnittest Family = "unittest"
	FamilyTox      Family = "tox"
	FamilyNox      Family = "nox"
	FamilyCoverage Family = "coverage"
)

// pythonModuleFamilies is the explicit allowlist of `-m` modules. Any other
// module (pip, http.server, venv, a project script) is refused.
var pythonModuleFamilies = map[string]Family{
	"pytest":   FamilyPytest,
	"unittest": FamilyUnittest,
	"mypy":     FamilyMypy,
	"flake8":   FamilyFlake8,
	"pylint":   FamilyPylint,
}

// isPythonInterpreter reports python, python3 and python3.N.
func isPythonInterpreter(prog string) bool {
	if prog == "python" || prog == "python3" {
		return true
	}
	rest, ok := strings.CutPrefix(prog, "python3.")
	if !ok || rest == "" {
		return false
	}
	for _, c := range rest {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// pythonQualityFamily detects the standalone linters and runners (flake8,
// pylint, tox, nox, coverage) and `python -m <module>` for the allowlisted
// modules. `python <file>`, `python -c` and any other option or module
// before the module name are refused.
func pythonQualityFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	argv = stripVenvBin(argv)
	switch argv[0] {
	case "flake8":
		return FamilyFlake8
	case "pylint":
		return FamilyPylint
	case "nox":
		return FamilyNox
	case "tox":
		return toxFamily(argv[1:])
	case "coverage":
		return coverageFamily(argv[1:])
	}
	if isPythonInterpreter(argv[0]) && len(argv) >= 3 && argv[1] == "-m" {
		return pythonModuleFamilies[argv[2]]
	}
	return FamilyNone
}

// toxFamily is an allowlist: bare `tox`, or `run`/`r`, `list`/`l` or
// `config`/`c` followed only by `-e <envs>`, `-p [auto|N]` and `--recreate`.
// exec/e, devenv/d, quickstart/q, legacy/le, `-x`/`--override` (which can
// install remote packages), `--` posargs and any URL are refused.
func toxFamily(args []string) Family {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "run", "r", "list", "l", "config", "c":
			args = args[1:]
		default:
			return FamilyNone
		}
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.Contains(a, "://") {
			return FamilyNone
		}
		switch a {
		case "--recreate":
		case "-e":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") || strings.Contains(args[i+1], "://") {
				return FamilyNone
			}
			i++
		case "-p":
			if i+1 < len(args) && isToxParallelCount(args[i+1]) {
				i++
			}
		default:
			return FamilyNone
		}
	}
	return FamilyTox
}

// isToxParallelCount reports `auto` or a number, the value of `-p`.
func isToxParallelCount(s string) bool {
	if s == "auto" {
		return true
	}
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// coverageFamily accepts `coverage report` and `coverage run [-flags] -m
// pytest|unittest`. Every word between `run` and `-m` must be a flag, so a
// script (`run evil.py -m pytest`) is refused, as are --rcfile (it can load
// plugins) and any URL.
func coverageFamily(args []string) Family {
	if len(args) == 0 {
		return FamilyNone
	}
	for _, a := range args {
		if strings.Contains(a, "://") || strings.HasPrefix(a, "--rcfile") {
			return FamilyNone
		}
	}
	switch args[0] {
	case "report":
		return FamilyCoverage
	case "run":
		for i := 1; i < len(args); i++ {
			if args[i] == "-m" {
				if i+1 < len(args) && (args[i+1] == "pytest" || args[i+1] == "unittest") {
					return FamilyCoverage
				}
				return FamilyNone
			}
			if !strings.HasPrefix(args[i], "-") {
				return FamilyNone
			}
		}
	}
	return FamilyNone
}

// pythonManage returns the Django subcommand and its arguments for
// `python manage.py ...` (manage.py or ./manage.py only) and `django-admin ...`.
func pythonManage(argv []string) (sub string, rest []string, ok bool) {
	switch {
	case len(argv) >= 3 && isPythonInterpreter(argv[0]) && (argv[1] == "manage.py" || argv[1] == "./manage.py"):
		sub, rest = argv[2], argv[3:]
	case len(argv) >= 2 && argv[0] == "django-admin":
		sub, rest = argv[1], argv[2:]
	default:
		return "", nil, false
	}
	for _, a := range rest {
		if strings.HasPrefix(a, "--pythonpath") {
			return "", nil, false
		}
	}
	return sub, rest, true
}

// hasFlag reports whether args holds any of the given words.
func hasFlag(args []string, flags ...string) bool {
	for _, a := range args {
		for _, f := range flags {
			if a == f {
				return true
			}
		}
	}
	return false
}
