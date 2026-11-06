// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package agents embeds the squad's agent definitions. The Markdown files stay
// the single source: the Claude Code plugin serves them in place, and the
// routing reads each one's default_tier from the same bytes.
package agents

import "embed"

// FS holds the agent Markdown files. The folder holds only agents: the README,
// routing matrix and protocols live in docs/agents, because the plugin loads
// every Markdown file here as an agent.
//
//go:embed *.md
var FS embed.FS
