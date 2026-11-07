// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// pythonToolingFamily detects Python tooling: pip, uv, ruff, mypy, black, isort, poetry, pdm.
func pythonToolingFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	argv = stripVenvBin(argv)

	prog := argv[0]

	// pip
	if (prog == "pip" || prog == "pip3") && len(argv) >= 2 {
		switch argv[1] {
		case "list":
			return FamilyPipList
		case "show":
			return FamilyPipShow
		case "freeze":
			return FamilyPipFreeze
		case "check":
			return FamilyPipCheck
		}
	}

	// uv
	if prog == "uv" && len(argv) >= 2 {
		switch argv[1] {
		case "pip":
			if len(argv) >= 3 {
				switch argv[2] {
				case "list", "show", "freeze", "check", "compile", "sync":
					return FamilyUvPip
				}
			}
		case "run":
			if runsGuarded(argv) {
				return FamilyNone
			}
			return FamilyUvRun
		case "sync", "lock", "tree", "version":
			return FamilyUvPip
		}
	}

	// Ruff
	if prog == "ruff" && len(argv) >= 2 {
		switch argv[1] {
		case "check":
			return FamilyRuff
		case "format":
			for _, a := range argv[2:] {
				if a == "--check" || a == "--diff" {
					return FamilyRuff
				}
			}
		}
	}

	// Mypy
	if prog == "mypy" {
		return FamilyMypy
	}

	// Black (check mode only)
	if prog == "black" {
		for _, a := range argv[1:] {
			if a == "--check" || a == "--diff" {
				return FamilyBlack
			}
		}
	}

	// isort (check mode only)
	if prog == "isort" {
		for _, a := range argv[1:] {
			if a == "--check" || a == "--check-only" || a == "--diff" {
				return FamilyIsort
			}
		}
	}

	// Pyright
	if prog == "pyright" {
		return FamilyPyright
	}

	// Bandit
	if prog == "bandit" {
		return FamilyBandit
	}

	// Poetry (read-only commands)
	if prog == "poetry" && len(argv) >= 2 && !runsGuarded(argv) {
		switch argv[1] {
		case "show", "check", "lock", "export", "env", "version", "about", "list", "search", "run":
			return FamilyPoetry
		}
	}

	// PDM (read-only commands)
	if prog == "pdm" && len(argv) >= 2 && !runsGuarded(argv) {
		switch argv[1] {
		case "list", "show", "lock", "export", "info", "search", "run":
			return FamilyPdm
		}
	}

	return FamilyNone
}
