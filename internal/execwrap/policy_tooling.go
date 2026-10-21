// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: Developer Tooling — CLI tools, build systems, linters, formatters
// ════════════════════════════════════════════════════════════════════════════

// Developer tooling family constants
const (
	// GitHub CLI
	FamilyGhPr     Family = "gh-pr"
	FamilyGhRun    Family = "gh-run"
	FamilyGhApi    Family = "gh-api"
	FamilyGhIssue  Family = "gh-issue"
	FamilyGhRepo   Family = "gh-repo"
	FamilyGhAuth   Family = "gh-auth"
	FamilyGhSearch Family = "gh-search"

	// Go toolchain
	FamilyGoBuild    Family = "go-build"
	FamilyGoRun      Family = "go-run"
	FamilyGoInstall  Family = "go-install"
	FamilyGoMod      Family = "go-mod"
	FamilyGoGenerate Family = "go-generate"
	FamilyGoFmt      Family = "go-fmt"
	FamilyGoVet      Family = "go-vet"
	FamilyGoDoc      Family = "go-doc"
	FamilyGoVersion  Family = "go-version"
	FamilyGoEnv      Family = "go-env"
	FamilyGoList     Family = "go-list"
	FamilyGoGet      Family = "go-get"
	FamilyGoWork     Family = "go-work"

	// Go linters
	FamilyStaticcheck  Family = "staticcheck"
	FamilyGolangciLint Family = "golangci-lint"
	FamilyGovulncheck  Family = "govulncheck"

	// Rust toolchain
	FamilyCargoBuild   Family = "cargo-build"
	FamilyCargoRun     Family = "cargo-run"
	FamilyCargoCheck   Family = "cargo-check"
	FamilyCargoClippy  Family = "cargo-clippy"
	FamilyCargoFmt     Family = "cargo-fmt"
	FamilyCargoDoc     Family = "cargo-doc"
	FamilyCargoAdd     Family = "cargo-add"
	FamilyCargoUpdate  Family = "cargo-update"
	FamilyCargoTree    Family = "cargo-tree"
	FamilyCargoAudit   Family = "cargo-audit"
	FamilyCargoOutdated Family = "cargo-outdated"

	// Python tooling
	FamilyPipList     Family = "pip-list"
	FamilyPipShow     Family = "pip-show"
	FamilyPipFreeze   Family = "pip-freeze"
	FamilyPipCheck    Family = "pip-check"
	FamilyUvPip       Family = "uv-pip"
	FamilyUvRun       Family = "uv-run"
	FamilyRuff        Family = "ruff"
	FamilyMypy        Family = "mypy"
	FamilyBlack       Family = "black"
	FamilyIsort       Family = "isort"
	FamilyPyright     Family = "pyright"
	FamilyBandit      Family = "bandit"
	FamilyPoetry      Family = "poetry"
	FamilyPdm         Family = "pdm"

	// Build systems
	FamilyMake       Family = "make"
	FamilyCMake      Family = "cmake"
	FamilyNinja      Family = "ninja"
	FamilyBazel      Family = "bazel"
	FamilyJust       Family = "just"
	FamilyTask       Family = "task"
	FamilyNx         Family = "nx"
	FamilyLerna      Family = "lerna"
	FamilyChangesets Family = "changesets"

	// Version managers / misc
	FamilyNvm     Family = "nvm"
	FamilyRbenv   Family = "rbenv"
	FamilyPyenv   Family = "pyenv"
	FamilyAsdf    Family = "asdf"
	FamilyRtx     Family = "rtx"
	FamilyMise    Family = "mise"
	FamilyFnm     Family = "fnm"
	FamilyVolta   Family = "volta"
	FamilyProto   Family = "proto"
	FamilyDirenv  Family = "direnv"
)

// toolingFamily detects developer tooling: gh, go build/run, cargo, make, etc.
func toolingFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}

	// GitHub CLI
	if family := ghFamily(argv); family != FamilyNone {
		return family
	}

	// Go toolchain (beyond go test)
	if family := goToolchainFamily(argv); family != FamilyNone {
		return family
	}

	// Go linters
	if family := goLinterFamily(argv); family != FamilyNone {
		return family
	}

	// Rust toolchain (beyond cargo test)
	if family := cargoToolchainFamily(argv); family != FamilyNone {
		return family
	}

	// Python tooling
	if family := pythonToolingFamily(argv); family != FamilyNone {
		return family
	}

	// Build systems
	if family := buildSystemFamily(argv); family != FamilyNone {
		return family
	}

	// Version managers
	if family := versionManagerFamily(argv); family != FamilyNone {
		return family
	}

	return FamilyNone
}

// ghFamily detects GitHub CLI commands.
func ghFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "gh" {
		return FamilyNone
	}

	cmd := argv[1]
	switch cmd {
	case "pr":
		return FamilyGhPr
	case "run":
		return FamilyGhRun
	case "api":
		return FamilyGhApi
	case "issue":
		return FamilyGhIssue
	case "repo":
		return FamilyGhRepo
	case "auth":
		// Only safe auth commands
		if len(argv) >= 3 {
			switch argv[2] {
			case "status", "token", "switch":
				return FamilyGhAuth
			}
		}
	case "search":
		return FamilyGhSearch
	case "workflow":
		return FamilyGhRun // workflow is part of run family
	case "release":
		// Only list/view, not create/delete
		if len(argv) >= 3 {
			switch argv[2] {
			case "list", "view":
				return FamilyGhRepo
			}
		}
	case "gist":
		// Only list/view
		if len(argv) >= 3 {
			switch argv[2] {
			case "list", "view":
				return FamilyGhRepo
			}
		}
	case "browse", "status":
		return FamilyGhRepo
	}

	return FamilyNone
}

// goToolchainFamily detects Go toolchain commands beyond test.
func goToolchainFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "go" {
		return FamilyNone
	}

	cmd := argv[1]
	switch cmd {
	case "build":
		return FamilyGoBuild
	case "run":
		return FamilyGoRun
	case "install":
		return FamilyGoInstall
	case "mod":
		if len(argv) >= 3 {
			switch argv[2] {
			case "tidy", "download", "verify", "graph", "why":
				return FamilyGoMod
			}
		}
	case "generate":
		return FamilyGoGenerate
	case "fmt":
		return FamilyGoFmt
	case "vet":
		return FamilyGoVet
	case "doc":
		return FamilyGoDoc
	case "version":
		return FamilyGoVersion
	case "env":
		return FamilyGoEnv
	case "list":
		return FamilyGoList
	case "get":
		return FamilyGoGet
	case "work":
		if len(argv) >= 3 {
			switch argv[2] {
			case "use", "sync", "edit", "init":
				return FamilyGoWork
			}
		}
	case "clean":
		// go clean is safe
		return FamilyGoBuild
	case "tool":
		// go tool pprof, go tool trace, etc.
		return FamilyGoDoc
	}

	return FamilyNone
}

// goLinterFamily detects Go linters: staticcheck, golangci-lint, govulncheck.
func goLinterFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}

	prog := argv[0]

	// Handle go run <linter>@version pattern
	if prog == "go" && len(argv) >= 3 && argv[1] == "run" {
		tool := argv[2]
		if strings.Contains(tool, "staticcheck") {
			return FamilyStaticcheck
		}
		if strings.Contains(tool, "golangci-lint") {
			return FamilyGolangciLint
		}
		if strings.Contains(tool, "govulncheck") {
			return FamilyGovulncheck
		}
	}

	switch prog {
	case "staticcheck":
		return FamilyStaticcheck
	case "golangci-lint":
		if len(argv) >= 2 && argv[1] == "run" {
			return FamilyGolangciLint
		}
	case "govulncheck":
		return FamilyGovulncheck
	}

	return FamilyNone
}

// cargoToolchainFamily detects Rust/Cargo commands beyond test.
func cargoToolchainFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "cargo" {
		return FamilyNone
	}

	cmd := argv[1]
	switch cmd {
	case "build", "b":
		return FamilyCargoBuild
	case "run", "r":
		return FamilyCargoRun
	case "check", "c":
		return FamilyCargoCheck
	case "clippy":
		return FamilyCargoClippy
	case "fmt":
		// Only check mode
		for _, a := range argv[2:] {
			if a == "--check" {
				return FamilyCargoFmt
			}
		}
	case "doc":
		return FamilyCargoDoc
	case "add":
		return FamilyCargoAdd
	case "update":
		return FamilyCargoUpdate
	case "tree":
		return FamilyCargoTree
	case "audit":
		return FamilyCargoAudit
	case "outdated":
		return FamilyCargoOutdated
	case "clean":
		return FamilyCargoBuild
	case "fetch":
		return FamilyCargoUpdate
	case "vendor":
		return FamilyCargoUpdate
	case "metadata":
		return FamilyCargoTree
	}

	return FamilyNone
}

// pythonToolingFamily detects Python tooling: pip, uv, ruff, mypy, black, isort, poetry, pdm.
func pythonToolingFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}

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
			return FamilyUvRun
		case "sync", "lock", "tree", "version":
			return FamilyUvPip
		}
	}

	// Ruff
	if prog == "ruff" {
		if len(argv) >= 2 {
			switch argv[1] {
			case "check", "format":
				// format needs --check to not modify
				if argv[1] == "format" {
					for _, a := range argv[2:] {
						if a == "--check" || a == "--diff" {
							return FamilyRuff
						}
					}
					return FamilyNone
				}
				return FamilyRuff
			}
		}
	}

	// Mypy
	if prog == "mypy" {
		return FamilyMypy
	}

	// Black
	if prog == "black" {
		// Only check mode
		for _, a := range argv[1:] {
			if a == "--check" || a == "--diff" {
				return FamilyBlack
			}
		}
	}

	// isort
	if prog == "isort" {
		// Only check mode
		for _, a := range argv[1:] {
			if a == "--check" || a == "--check-only" || a == "--diff" {
				return FamilyIsort
			}
		}
	}

	// Pyright
	if prog == "pyright" || prog == "npx" && len(argv) >= 2 && argv[1] == "pyright" {
		return FamilyPyright
	}

	// Bandit
	if prog == "bandit" {
		return FamilyBandit
	}

	// Poetry
	if prog == "poetry" && len(argv) >= 2 {
		switch argv[1] {
		case "show", "check", "lock", "export", "env", "version", "about", "list", "search":
			return FamilyPoetry
		case "run":
			return FamilyPoetry
		case "install":
			// install --dry-run or --no-root
			for _, a := range argv[2:] {
				if a == "--dry-run" {
					return FamilyPoetry
				}
			}
		}
	}

	// PDM
	if prog == "pdm" && len(argv) >= 2 {
		switch argv[1] {
		case "list", "show", "lock", "export", "info", "search", "run":
			return FamilyPdm
		}
	}

	return FamilyNone
}

// buildSystemFamily detects build systems: make, cmake, ninja, bazel, just, task, nx, lerna.
func buildSystemFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}

	prog := argv[0]
	switch prog {
	case "make", "gmake":
		// All make targets allowed (it's user-defined)
		return FamilyMake
	case "cmake":
		return FamilyCMake
	case "ninja":
		return FamilyNinja
	case "bazel", "bazelisk":
		if len(argv) >= 2 {
			switch argv[1] {
			case "build", "test", "run", "query", "info", "version", "fetch", "sync":
				return FamilyBazel
			}
		}
	case "just":
		return FamilyJust
	case "task":
		return FamilyTask
	case "nx":
		if len(argv) >= 2 {
			switch argv[1] {
			case "run", "build", "test", "lint", "e2e", "affected", "graph", "list", "show":
				return FamilyNx
			}
		}
	case "lerna", "npx":
		cmdIdx := 1
		if prog == "npx" && len(argv) >= 2 && argv[1] == "lerna" {
			cmdIdx = 2
		}
		if (prog == "lerna" || argv[1] == "lerna") && len(argv) > cmdIdx {
			switch argv[cmdIdx] {
			case "run", "exec", "list", "info", "diff", "changed":
				return FamilyLerna
			}
		}
	case "changeset", "changesets":
		if len(argv) >= 2 {
			switch argv[1] {
			case "status", "version", "publish":
				return FamilyChangesets
			}
		}
		// Bare changeset is interactive add
		return FamilyNone
	}

	return FamilyNone
}

// versionManagerFamily detects version managers: nvm, rbenv, pyenv, asdf, rtx, mise, fnm, volta, proto.
func versionManagerFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}

	prog := argv[0]
	switch prog {
	case "nvm":
		if len(argv) >= 2 {
			switch argv[1] {
			case "list", "ls", "current", "version", "which", "alias":
				return FamilyNvm
			case "use", "install":
				return FamilyNvm
			}
		}
	case "rbenv":
		if len(argv) >= 2 {
			switch argv[1] {
			case "versions", "version", "which", "whence", "global", "local", "shell":
				return FamilyRbenv
			}
		}
	case "pyenv":
		if len(argv) >= 2 {
			switch argv[1] {
			case "versions", "version", "which", "whence", "global", "local", "shell", "virtualenvs":
				return FamilyPyenv
			}
		}
	case "asdf":
		if len(argv) >= 2 {
			switch argv[1] {
			case "list", "current", "where", "which", "info", "plugin":
				return FamilyAsdf
			case "global", "local", "shell":
				return FamilyAsdf
			}
		}
	case "rtx", "mise":
		if len(argv) >= 2 {
			switch argv[1] {
			case "list", "ls", "current", "where", "which", "env", "doctor", "version", "plugins":
				if prog == "rtx" {
					return FamilyRtx
				}
				return FamilyMise
			case "use", "global", "local", "shell", "install":
				if prog == "rtx" {
					return FamilyRtx
				}
				return FamilyMise
			}
		}
	case "fnm":
		if len(argv) >= 2 {
			switch argv[1] {
			case "list", "ls", "current", "default", "env":
				return FamilyFnm
			case "use", "install":
				return FamilyFnm
			}
		}
	case "volta":
		if len(argv) >= 2 {
			switch argv[1] {
			case "list", "which", "pin", "run":
				return FamilyVolta
			}
		}
	case "proto":
		if len(argv) >= 2 {
			switch argv[1] {
			case "list", "bin", "run", "status", "tools":
				return FamilyProto
			}
		}
	case "direnv":
		if len(argv) >= 2 {
			switch argv[1] {
			case "status", "allow", "deny", "reload", "exec", "export":
				return FamilyDirenv
			}
		}
	}

	return FamilyNone
}
