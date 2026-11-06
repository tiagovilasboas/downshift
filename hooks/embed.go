// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package hooks embeds the per-harness glue Downshift distributes and
// `downshift setup` installs. Each folder is one harness's bridge into the
// shared Go layers; the folder stays the single source, and the binary carries
// exactly the bytes setup writes.
package hooks

import "embed"

// OpenCodePlugin is the bridge file inside FS: the plugin OpenCode loads from
// its config directory (the folder name is the plugin name).
const OpenCodePlugin = "opencode/index.ts"

// OpenCodeModules lists the other files of the plugin folder: modules that
// index.ts imports. OpenCode loads a plugin per folder under plugins/, so
// siblings of index.ts are not separate plugins. Setup writes index.ts and
// every module; a test keeps this list equal to the embedded folder.
var OpenCodeModules = []string{
	"opencode/models.ts",
	"opencode/shell_sessions.ts",
	"opencode/shell_trust.ts",
	"opencode/wire.ts",
}

// FS holds the harness bridges. `all:` keeps every file, including ones a
// plain pattern would skip.
//
//go:embed all:opencode
var FS embed.FS
