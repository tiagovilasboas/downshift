// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

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
