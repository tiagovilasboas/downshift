// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package skills embeds the skills `downshift setup` installs into a harness's
// skills directory. The SKILL.md files stay the single source: the Claude Code
// plugin reads them in place, setup writes the same bytes.
package skills

import "embed"

// Audit is the read-only inspection skill; its folder name is its skill name.
const Audit = "downshift-audit"

// FS holds every skill setup installs, one folder each.
//
//go:embed downshift-audit
var FS embed.FS
