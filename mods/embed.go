// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package mods embeds the Claude Code mods `downshift setup` installs as
// skills-dir plugins. Each folder stays the single source: the repository's
// marketplace serves it in place, setup writes the same bytes.
package mods

import "embed"

// Agents is the live agents band and pane; its folder name is its plugin name.
const Agents = "downshift-agents"

// FS holds every mod setup installs, one folder each. `all:` keeps the
// `.claude-plugin` manifest, which a plain pattern would skip.
//
//go:embed all:downshift-agents
var FS embed.FS
