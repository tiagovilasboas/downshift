// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

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
		return lernaFamily(argv)
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
		return direnvFamily(argv)
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
