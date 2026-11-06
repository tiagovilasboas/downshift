// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package skills embeds the skills `downshift setup` installs into a harness's
// skills directory. The SKILL.md files stay the single source: the Claude Code
// plugin reads them in place, setup writes the same bytes.
package skills

import "embed"

// Audit is the read-only inspection skill; its folder name is its skill name.
const Audit = "downshift-audit"

// Orchestrator is the playbook the lead session follows when it delegates.
const Orchestrator = "downshift-orchestrator"

// Effort is the advice that matches reasoning and verification effort to stakes.
const Effort = "downshift-effort"

// Squad invokes Downshift agent squads for coordinated multi-agent work.
const Squad = "downshift-squad"

// Ponytail is the lazy-senior-dev coding philosophy: the smallest complete change.
const Ponytail = "ponytail"

// FS holds every skill setup installs, one folder each.
//
//go:embed downshift-audit downshift-effort downshift-orchestrator downshift-squad ponytail
var FS embed.FS
