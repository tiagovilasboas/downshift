// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package compressor provides a deterministic, zero-dependency Go compressor
// for recognized tool outputs (git, tests, searches, file listings, logs).
// It never relies on an LLM, requires no network or Python, and guarantees
// complete preservation of errors, panics, and stack traces.
package compressor

// Mode defines the compression operating mode.
type Mode string

const (
	// ModeOff disables compression completely.
	ModeOff Mode = "off"
	// ModeObserve estimates savings and detects format without modifying the output.
	ModeObserve Mode = "observe"
	// ModeSafe compresses recognized formats only when provably safe (e.g. 0 failures).
	ModeSafe Mode = "safe"
)

// Format identifies the recognized output structure.
type Format string

const (
	FormatUnknown      Format = "unknown"
	FormatGitStatus    Format = "git_status"
	FormatGitLog       Format = "git_log"
	FormatGoTest       Format = "go_test"
	FormatPytest       Format = "pytest"
	FormatVitest       Format = "vitest"
	FormatPlaywright   Format = "playwright"
	FormatCargoTest    Format = "cargo_test"
	FormatTSC          Format = "tsc"
	FormatJest         Format = "jest"
	FormatCypress      Format = "cypress"
	FormatNextBuild    Format = "next_build"
	FormatPHPUnit      Format = "phpunit"
	FormatFlutterTest  Format = "flutter_test" // flutter test and dart test share one reporter
	FormatSearch       Format = "search"       // rg, grep
	FormatFileList     Format = "file_listing" // ls -R, tree, find
	FormatLogs         Format = "logs"         // repetitive log lines
	FormatGradle       Format = "gradle"
	FormatMaven        Format = "maven"
	FormatRspec        Format = "rspec"         // Ruby RSpec
	FormatDotnetTest   Format = "dotnet_test"   // .NET test runner
	FormatNpmInstall   Format = "npm_install"   // npm/pnpm/yarn/bun install
	FormatPipInstall   Format = "pip_install"   // pip install
	FormatDockerPs     Format = "docker_ps"     // docker ps
	FormatDockerImages Format = "docker_images" // docker images
	FormatDockerLogs   Format = "docker_logs"   // docker logs (dedup)
	FormatKubectlPods  Format = "kubectl_pods"  // kubectl get pods
	FormatKubectlLogs  Format = "kubectl_logs"  // kubectl logs (dedup)
	FormatEslint       Format = "eslint"        // ESLint stylish formatter
	FormatRuff         Format = "ruff"          // Python ruff check
	FormatMypy         Format = "mypy"          // Python mypy type checker
	FormatGolangciLint Format = "golangci_lint" // Go golangci-lint
	FormatCargoClippy  Format = "cargo_clippy"  // Rust cargo clippy
	FormatGitDiff      Format = "git_diff"      // git diff (unified)
	FormatGitShow      Format = "git_show"      // git show (commit + diff)
	FormatGitStash     Format = "git_stash"     // git stash list
	FormatGhPr         Format = "gh_pr"         // gh pr view
	FormatGhChecks     Format = "gh_checks"     // gh pr checks
	FormatGhRunList    Format = "gh_run_list"   // gh run list
	FormatCargoBuild   Format = "cargo_build"   // Rust cargo build
	FormatGoBuild      Format = "go_build"      // Go go build
	FormatDotnetBuild  Format = "dotnet_build"  // .NET dotnet build (MSBuild)

	// Terraform
	FormatTerraformPlan Format = "terraform_plan" // terraform plan

	// Dependency trees
	FormatNpmLs      Format = "npm_ls"       // npm ls / pnpm ls / yarn list
	FormatCargoTree  Format = "cargo_tree"   // cargo tree
	FormatGoModGraph Format = "go_mod_graph" // go mod graph

	// Docker build/push
	FormatDockerBuild Format = "docker_build" // docker build
	FormatDockerPush  Format = "docker_push"  // docker push
)

// Result is the deterministic outcome of a compression attempt.
type Result struct {
	Output        []byte `json:"-"`
	Applied       bool   `json:"applied"`
	Format        Format `json:"format"`
	OriginalBytes int    `json:"original_bytes"`
	ReducedBytes  int    `json:"reduced_bytes"`
	Reason        string `json:"reason"`
}

// SavingsRatio returns the percentage of bytes reduced (0.0 to 100.0).
func (r Result) SavingsRatio() float64 {
	if r.OriginalBytes == 0 || r.ReducedBytes >= r.OriginalBytes {
		return 0.0
	}
	return float64(r.OriginalBytes-r.ReducedBytes) / float64(r.OriginalBytes) * 100.0
}
