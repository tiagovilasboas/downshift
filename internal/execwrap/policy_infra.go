// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: Infra — Kubernetes, Helm, Terraform, cloud CLIs, Docker, GitOps
// ════════════════════════════════════════════════════════════════════════════

// Infra family constants. Every family here is read-only or a safe, local
// build/plan that never mutates a live resource.
const (
	// Kubernetes (read-only)
	FamilyKubectlGet      Family = "kubectl-get"
	FamilyKubectlDescribe Family = "kubectl-describe"
	FamilyKubectlLogs     Family = "kubectl-logs"

	// Helm (read-only)
	FamilyHelmList   Family = "helm-list"
	FamilyHelmStatus Family = "helm-status"

	// Terraform (read-only / local)
	FamilyTerraformPlan Family = "terraform-plan"
	FamilyTerraformFmt  Family = "terraform-fmt"

	// Azure CLI (read-only)
	FamilyAzList Family = "az-list"
	FamilyAzShow Family = "az-show"

	// AWS CLI (read-only)
	FamilyAwsDescribe Family = "aws-describe"
	FamilyAwsList     Family = "aws-list"
	FamilyAwsGet      Family = "aws-get"
	FamilyAwsS3Ls     Family = "aws-s3-ls"
	FamilyAwsS3Cp     Family = "aws-s3-cp"
	FamilyAwsLogs     Family = "aws-logs"

	// Docker (read-only and safe commands)
	FamilyDockerPS     Family = "docker-ps"
	FamilyDockerLogs   Family = "docker-logs"
	FamilyDockerImages Family = "docker-images"
	FamilyDockerBuild  Family = "docker-build"

	// ArgoCD (read-only)
	FamilyArgoCDList Family = "argocd-list"
	FamilyArgoCDGet  Family = "argocd-get"
	FamilyArgoCDDiff Family = "argocd-diff"

	// Kustomize (local build only)
	FamilyKustomizeBuild Family = "kustomize-build"

	// Azure Pipelines (read-only)
	FamilyAzPipelines Family = "az-pipelines"
)

// infraFamily detects infrastructure tooling: kubectl, helm, terraform, az,
// aws, docker, argocd, kustomize. Only read-only or safe local build/plan
// subcommands are allowlisted; anything that applies, deletes, installs,
// upgrades or syncs a live resource returns FamilyNone and is left unwrapped.
func infraFamily(argv []string) Family {
	if len(argv) < 2 {
		return FamilyNone
	}
	prog := argv[0]
	cmd := argv[1]
	switch prog {
	case "kubectl":
		switch cmd {
		case "get":
			return FamilyKubectlGet
		case "describe":
			return FamilyKubectlDescribe
		case "logs":
			return FamilyKubectlLogs
		}
	case "helm":
		switch cmd {
		case "list", "ls":
			return FamilyHelmList
		case "status":
			return FamilyHelmStatus
		}
	case "terraform":
		switch cmd {
		case "plan":
			return FamilyTerraformPlan
		case "fmt":
			// Only -check mode (never rewrites files in place).
			for _, a := range argv[2:] {
				if a == "-check" || a == "--check" {
					return FamilyTerraformFmt
				}
			}
		}
	case "az":
		// "az pipelines ..." is read-only here; broad az list/show below.
		if cmd == "pipelines" {
			return azPipelinesFamily(argv[2:])
		}
		// Azure CLI: az <resource> ... <action>, e.g. az aks list,
		// az storage account show. Find the first list/show action before
		// any flag.
		for i := 2; i < len(argv); i++ {
			if argv[i] == "list" {
				return FamilyAzList
			}
			if argv[i] == "show" {
				return FamilyAzShow
			}
			if strings.HasPrefix(argv[i], "-") {
				break
			}
		}
	case "aws":
		return awsFamily(argv[1:])
	case "docker":
		return dockerFamily(argv)
	case "argocd":
		return argocdFamily(argv[1:])
	case "kustomize":
		if cmd == "build" {
			return FamilyKustomizeBuild
		}
	}
	return FamilyNone
}

// awsFamily detects read-only AWS CLI commands. Mutating verbs (create, put,
// delete, update, run, terminate) never match.
func awsFamily(args []string) Family {
	if len(args) == 0 {
		return FamilyNone
	}
	service := args[0]
	// Skip global options to find the subcommand.
	cmdIdx := 1
	for cmdIdx < len(args) && strings.HasPrefix(args[cmdIdx], "-") {
		cmdIdx++
		// Skip an option value that is not itself a flag.
		if cmdIdx < len(args) && !strings.HasPrefix(args[cmdIdx], "-") {
			cmdIdx++
		}
	}
	if cmdIdx >= len(args) {
		return FamilyNone
	}
	subCmd := args[cmdIdx]
	// S3 commands.
	if service == "s3" {
		switch subCmd {
		case "ls":
			return FamilyAwsS3Ls
		case "cp":
			return FamilyAwsS3Cp
		}
	}
	// CloudWatch Logs.
	if service == "logs" {
		switch subCmd {
		case "describe-log-groups", "describe-log-streams", "get-log-events", "filter-log-events", "tail":
			return FamilyAwsLogs
		}
	}
	// Generic read-only patterns across services.
	switch {
	case strings.HasPrefix(subCmd, "describe"):
		return FamilyAwsDescribe
	case strings.HasPrefix(subCmd, "list"):
		return FamilyAwsList
	case strings.HasPrefix(subCmd, "get") && !strings.Contains(subCmd, "password") && !strings.Contains(subCmd, "secret"):
		return FamilyAwsGet
	}
	return FamilyNone
}

// dockerFamily detects safe docker commands: ps, logs, images and a local
// image build. Mutating commands (run, rm, exec, push, rmi) never match.
func dockerFamily(argv []string) Family {
	if len(argv) < 2 || argv[0] != "docker" {
		return FamilyNone
	}
	switch argv[1] {
	case "ps":
		return FamilyDockerPS
	case "logs":
		return FamilyDockerLogs
	case "images":
		return FamilyDockerImages
	case "build":
		return FamilyDockerBuild
	}
	return FamilyNone
}

// argocdFamily detects read-only ArgoCD commands, given the words after
// "argocd". Only the list, get and diff subcommands of app/proj/repo/cluster
// are allowlisted; sync, create, delete and set never match.
func argocdFamily(args []string) Family {
	if len(args) < 2 {
		return FamilyNone
	}
	// Forms: "argocd app list", "argocd app get <name>", "argocd app diff <name>".
	switch args[0] {
	case "app", "proj", "repo", "cluster":
		switch args[1] {
		case "list":
			return FamilyArgoCDList
		case "get":
			return FamilyArgoCDGet
		case "diff":
			return FamilyArgoCDDiff
		}
	}
	return FamilyNone
}

// azPipelinesFamily detects read-only "az pipelines" commands, given the words
// after "az pipelines". Only list/show are allowlisted; run, create and delete
// never match.
func azPipelinesFamily(args []string) Family {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			break
		}
		if a == "list" || a == "show" {
			return FamilyAzPipelines
		}
	}
	return FamilyNone
}
