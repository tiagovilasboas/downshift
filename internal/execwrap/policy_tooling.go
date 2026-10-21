// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: Developer Tooling — CLI tools, build systems, linters, formatters
// ════════════════════════════════════════════════════════════════════════════

// Developer tooling family constants
const (
	// GitHub CLI — high usage, compacted via gh output parsing
	FamilyGhPr     Family = "gh-pr"
	FamilyGhRun    Family = "gh-run"
	FamilyGhApi    Family = "gh-api"
	FamilyGhIssue  Family = "gh-issue"
	FamilyGhRepo   Family = "gh-repo"
	FamilyGhAuth   Family = "gh-auth"
	FamilyGhSearch Family = "gh-search"

	// Go toolchain — high usage
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

	// Go linters — medium usage
	FamilyStaticcheck  Family = "staticcheck"
	FamilyGolangciLint Family = "golangci-lint"
	FamilyGovulncheck  Family = "govulncheck"

	// Rust toolchain — medium usage
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

	// Python tooling — high usage
	FamilyPipList   Family = "pip-list"
	FamilyPipShow   Family = "pip-show"
	FamilyPipFreeze Family = "pip-freeze"
	FamilyPipCheck  Family = "pip-check"
	FamilyUvPip     Family = "uv-pip"
	FamilyUvRun     Family = "uv-run"
	FamilyRuff      Family = "ruff"
	FamilyMypy      Family = "mypy"
	FamilyBlack     Family = "black"
	FamilyIsort     Family = "isort"
	FamilyPyright   Family = "pyright"
	FamilyBandit    Family = "bandit"
	FamilyPoetry    Family = "poetry"
	FamilyPdm       Family = "pdm"

	// Build systems — high usage
	FamilyMake       Family = "make"
	FamilyCMake      Family = "cmake"
	FamilyNinja      Family = "ninja"
	FamilyBazel      Family = "bazel"
	FamilyJust       Family = "just"
	FamilyTask       Family = "task"
	FamilyNx         Family = "nx"
	FamilyLerna      Family = "lerna"
	FamilyChangesets Family = "changesets"

	// Filesystem write commands — tracked for safety, not compacted
	FamilyMkdir Family = "mkdir"
	FamilyTouch Family = "touch"
	FamilyCp    Family = "cp"
	FamilyMv    Family = "mv"
	FamilyRm    Family = "rm"

	// AST tools — useful for refactoring
	FamilyAstGrep Family = "ast-grep"

	// .NET toolchain — rspec needs real compressor
	FamilyDotnetBuild Family = "dotnet-build"
	FamilyDotnetTest  Family = "dotnet-test"
	FamilyDotnetRun   Family = "dotnet-run"

	// Ruby ecosystem — rspec needs real compressor
	FamilyRspec   Family = "rspec"
	FamilyRubocop Family = "rubocop"

	// Version managers — lightweight, keep for compatibility
	FamilyMise   Family = "mise"
	FamilyAsdf   Family = "asdf"
	FamilyDirenv Family = "direnv"

	// Database CLI — read-only queries
	FamilyPsql Family = "psql"

	// Cloud CLI — GCP (symmetry with aws/az)
	FamilyGcloud Family = "gcloud"

	// System/host diagnostics
	FamilyJournalctl Family = "journalctl"
	FamilySystemctl  Family = "systemctl"

	// Kubernetes extended
	FamilyKubectlTop Family = "kubectl-top"

	// Mobile/Native build — iOS
	FamilyXcodebuild Family = "xcodebuild"
	FamilyPodInstall Family = "pod"
	FamilyExpo       Family = "expo"
	FamilyReactNative Family = "react-native"
	FamilyAdb         Family = "adb"

	// Note: Gradle is handled by FamilyGradle in policy_backend.go

	// Package managers — install commands (verbose output)
	// FamilyNpmInstall declared in policy_frontend.go
	FamilyPipInstall Family = "pip-install"

	// Other languages
	FamilyMix   Family = "mix"
	FamilySwift Family = "swift"
	FamilySbt   Family = "sbt"
	// FamilyFlutterBuild already declared in policy_backend.go
)

// toolingFamily detects developer tooling: gh, go build/run, cargo, make, etc.
func toolingFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}

	// GitHub CLI — very high usage
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

	// Version managers (only the most used)
	if family := versionManagerFamily(argv); family != FamilyNone {
		return family
	}

	// Filesystem write commands (tracked for safety)
	if family := filesystemWriteFamily(argv); family != FamilyNone {
		return family
	}

	// AST tools
	if family := astToolFamily(argv); family != FamilyNone {
		return family
	}

	// .NET toolchain
	if family := dotnetFamily(argv); family != FamilyNone {
		return family
	}

	// Ruby test runners
	if family := rubyTestFamily(argv); family != FamilyNone {
		return family
	}

	// Database CLI
	if family := databaseFamily(argv); family != FamilyNone {
		return family
	}

	// Cloud CLI (GCP)
	if family := gcloudFamily(argv); family != FamilyNone {
		return family
	}

	// System diagnostics
	if family := systemFamily(argv); family != FamilyNone {
		return family
	}

	// Kubernetes extended
	if family := kubectlExtFamily(argv); family != FamilyNone {
		return family
	}

	// Mobile/Native build
	if family := mobileNativeFamily(argv); family != FamilyNone {
		return family
	}

	// Package managers (install)
	if family := packageInstallFamily(argv); family != FamilyNone {
		return family
	}

	// Other languages
	if family := otherLanguagesFamily(argv); family != FamilyNone {
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
		if len(argv) >= 3 {
			switch argv[2] {
			case "status", "token", "switch":
				return FamilyGhAuth
			}
		}
	case "search":
		return FamilyGhSearch
	case "workflow":
		return FamilyGhRun
	case "release", "gist":
		if len(argv) >= 3 && (argv[2] == "list" || argv[2] == "view") {
			return FamilyGhRepo
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
		return FamilyGoBuild
	case "tool":
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
	case "clean", "fetch", "vendor":
		return FamilyCargoBuild
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
	if prog == "poetry" && len(argv) >= 2 {
		switch argv[1] {
		case "show", "check", "lock", "export", "env", "version", "about", "list", "search", "run":
			return FamilyPoetry
		}
	}

	// PDM (read-only commands)
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
	case "lerna":
		if len(argv) >= 2 {
			switch argv[1] {
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
	}

	return FamilyNone
}

// versionManagerFamily detects the most common version managers.
func versionManagerFamily(argv []string) Family {
	if len(argv) < 2 {
		return FamilyNone
	}

	prog := argv[0]
	switch prog {
	case "mise", "rtx":
		switch argv[1] {
		case "list", "ls", "current", "where", "which", "env", "doctor", "version", "plugins",
			"use", "global", "local", "shell", "install":
			return FamilyMise
		}
	case "asdf":
		switch argv[1] {
		case "list", "current", "where", "which", "info", "plugin", "global", "local", "shell":
			return FamilyAsdf
		}
	case "direnv":
		switch argv[1] {
		case "status", "allow", "deny", "reload", "exec", "export":
			return FamilyDirenv
		}
	}

	return FamilyNone
}

// filesystemWriteFamily detects filesystem write commands (tracked for safety, not compacted).
func filesystemWriteFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}

	prog := argv[0]
	switch prog {
	case "mkdir":
		return FamilyMkdir
	case "touch":
		return FamilyTouch
	case "cp", "gcp":
		return FamilyCp
	case "mv", "gmv":
		return FamilyMv
	case "rm", "grm":
		// Block dangerous rm -rf
		for _, a := range argv[1:] {
			if a == "-rf" || a == "-fr" || a == "--recursive" {
				return FamilyNone
			}
		}
		return FamilyRm
	}
	return FamilyNone
}

// astToolFamily detects AST-based search tools.
func astToolFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	if argv[0] == "ast-grep" || argv[0] == "sg" {
		return FamilyAstGrep
	}
	return FamilyNone
}

// dotnetFamily detects .NET CLI commands.
func dotnetFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "dotnet" {
		return FamilyNone
	}
	switch argv[1] {
	case "build":
		return FamilyDotnetBuild
	case "test":
		return FamilyDotnetTest
	case "run":
		return FamilyDotnetRun
	case "restore", "clean", "publish", "pack":
		return FamilyDotnetBuild
	}
	return FamilyNone
}

// rubyTestFamily detects Ruby test runners that need compaction.
func rubyTestFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	switch argv[0] {
	case "rspec":
		return FamilyRspec
	case "rubocop":
		return FamilyRubocop
	}
	return FamilyNone
}

// databaseFamily detects database CLI tools (read-only queries).
func databaseFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	if argv[0] == "psql" {
		// Allow read-only usage; actual query safety is the user's responsibility
		return FamilyPsql
	}
	return FamilyNone
}

// gcloudFamily detects GCP CLI (read-only commands).
func gcloudFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "gcloud" {
		return FamilyNone
	}
	// Allow describe, list, get patterns
	for _, a := range argv[1:] {
		if a == "describe" || a == "list" || a == "get" || a == "info" ||
			a == "logs" || a == "read" || a == "config" || a == "auth" {
			return FamilyGcloud
		}
	}
	return FamilyNone
}

// systemFamily detects system diagnostic commands.
func systemFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	switch argv[0] {
	case "journalctl":
		return FamilyJournalctl
	case "systemctl":
		if len(argv) >= 2 && argv[1] == "status" {
			return FamilySystemctl
		}
		// Also allow show, list-units, is-active (read-only)
		if len(argv) >= 2 {
			switch argv[1] {
			case "show", "list-units", "list-unit-files", "is-active", "is-enabled", "cat":
				return FamilySystemctl
			}
		}
	}
	return FamilyNone
}

// kubectlExtFamily detects extended kubectl commands.
func kubectlExtFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "kubectl" {
		return FamilyNone
	}
	if argv[1] == "top" {
		return FamilyKubectlTop
	}
	return FamilyNone
}

// mobileNativeFamily detects mobile/native build tools.
func mobileNativeFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	prog := argv[0]

	switch prog {
	case "xcodebuild":
		return FamilyXcodebuild
	case "pod":
		if len(argv) >= 2 {
			switch argv[1] {
			case "install", "update", "outdated", "repo", "spec", "list":
				return FamilyPodInstall
			}
		}
	case "expo":
		if len(argv) >= 2 {
			switch argv[1] {
			case "start", "prebuild", "export", "doctor", "install", "upgrade", "config", "diagnostics":
				return FamilyExpo
			}
		}
	case "react-native":
		if len(argv) >= 2 {
			switch argv[1] {
			case "run-android", "run-ios", "bundle", "start", "info", "doctor", "log-android", "log-ios":
				return FamilyReactNative
			}
		}
	case "npx":
		// Handle npx react-native and npx expo
		if len(argv) >= 2 {
			if argv[1] == "react-native" || strings.HasPrefix(argv[1], "react-native@") {
				return FamilyReactNative
			}
			if argv[1] == "expo" || strings.HasPrefix(argv[1], "expo@") {
				return FamilyExpo
			}
		}
	case "adb":
		if len(argv) >= 2 {
			switch argv[1] {
			case "install", "logcat", "devices", "shell", "push", "pull", "bugreport":
				return FamilyAdb
			}
		}
	// Note: gradle/gradlew handled by FamilyGradle in policy_backend.go
	}
	return FamilyNone
}

// packageInstallFamily detects package manager install commands.
func packageInstallFamily(argv []string) Family {
	if len(argv) < 2 {
		return FamilyNone
	}
	prog := argv[0]

	// npm/pnpm/yarn/bun install
	switch prog {
	case "npm", "pnpm", "yarn", "bun":
		if argv[1] == "install" || argv[1] == "i" || argv[1] == "add" || argv[1] == "ci" {
			return FamilyNpmInstall
		}
	case "pip", "pip3":
		if argv[1] == "install" {
			return FamilyPipInstall
		}
	case "uv":
		if len(argv) >= 2 && (argv[1] == "add" || argv[1] == "pip") {
			if argv[1] == "pip" && len(argv) >= 3 && argv[2] == "install" {
				return FamilyPipInstall
			}
			if argv[1] == "add" {
				return FamilyPipInstall
			}
		}
	}
	return FamilyNone
}

// otherLanguagesFamily detects other language toolchains.
func otherLanguagesFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	prog := argv[0]

	switch prog {
	case "mix":
		if len(argv) >= 2 {
			switch argv[1] {
			case "test", "compile", "deps.get", "deps.compile", "format", "clean", "run", "do", "hex.info":
				return FamilyMix
			}
		}
	case "swift":
		if len(argv) >= 2 {
			switch argv[1] {
			case "build", "test", "run", "package":
				return FamilySwift
			}
		}
	case "sbt":
		return FamilySbt // sbt is always verbose
	case "flutter":
		if len(argv) >= 2 {
			switch argv[1] {
			case "build", "analyze", "pub":
				return FamilyFlutterBuild
			}
		}
	case "dart":
		if len(argv) >= 2 {
			switch argv[1] {
			case "analyze", "pub":
				return FamilyFlutterBuild // Share with flutter
			}
		}
	}
	return FamilyNone
}