// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

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
	FamilyCargoBuild    Family = "cargo-build"
	FamilyCargoRun      Family = "cargo-run"
	FamilyCargoCheck    Family = "cargo-check"
	FamilyCargoClippy   Family = "cargo-clippy"
	FamilyCargoFmt      Family = "cargo-fmt"
	FamilyCargoDoc      Family = "cargo-doc"
	FamilyCargoAdd      Family = "cargo-add"
	FamilyCargoUpdate   Family = "cargo-update"
	FamilyCargoTree     Family = "cargo-tree"
	FamilyCargoAudit    Family = "cargo-audit"
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
	FamilyXcodebuild  Family = "xcodebuild"
	FamilyPodInstall  Family = "pod"
	FamilyExpo        Family = "expo"
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
