// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

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

	// Python linters, runners and `python -m <module>`; Django checks
	if family := pythonQualityFamily(argv); family != FamilyNone {
		return family
	}
	if family := djangoFamily(argv); family != FamilyNone {
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

	// Filesystem listing and search: only the forms whose output is a plain list
	if family := listingFamily(argv); family != FamilyNone {
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

	// Deno and Zig: check, lint, build and test forms only
	if family := denoFamily(argv); family != FamilyNone {
		return family
	}
	if family := zigFamily(argv); family != FamilyNone {
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
