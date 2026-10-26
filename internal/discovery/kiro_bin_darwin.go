// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

// bundledKiroCLI is the kiro-cli copy the macOS KiroCrew app ships inside its
// bundle. The chat subcommand used for listing is the same one the PATH binary
// answers.
func bundledKiroCLI() string {
	return "/Applications/KiroCrew.app/Contents/Resources/backend-dist/kiro-cli/kiro-cli-chat"
}
