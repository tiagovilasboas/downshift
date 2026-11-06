// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: GitLab CLI — read-only glab listings and views
// ════════════════════════════════════════════════════════════════════════════

// FamilyGlab is a read-only glab command. Only the shapes in glabReadShapes
// are a family; every other subcommand (api, auth, config, variable, ci trace,
// job, every write verb) is left as written, so it never reaches the
// compressor. The compacted view has GitLab tokens and credentials in URLs
// redacted, and the raw output is never stored (rawStoreRefused).
const FamilyGlab Family = "glab"

// glabReadShapes lists the command group and subcommand pairs that only read.
// `glab ci view` is a live terminal UI and `glab ci trace` streams a job log,
// so neither is listed. `glab pipeline` is the alias of `glab ci`.
var glabReadShapes = map[string]map[string]bool{
	"mr":       {"list": true, "view": true, "diff": true},
	"issue":    {"list": true, "view": true},
	"ci":       {"status": true, "list": true},
	"pipeline": {"list": true},
}

// glabRefusedFlags open a browser, print the token, or keep a view live.
var glabRefusedFlags = []string{"web", "w", "show-token", "live", "l"}

// glabFamily names FamilyGlab for glab <group> <subcommand> [flags], or
// FamilyNone.
func glabFamily(argv []string) Family {
	if len(argv) < 3 || argv[0] != "glab" || !glabReadShapes[argv[1]][argv[2]] {
		return FamilyNone
	}
	if namesFlag(argv[3:], glabRefusedFlags...) || glabShortCluster(argv[3:]) {
		return FamilyNone
	}
	return FamilyGlab
}

// glabShortCluster reports a joined short flags word such as -cw that holds
// one of the refused short flags (w opens a browser, l keeps the view live).
func glabShortCluster(words []string) bool {
	for _, w := range words {
		if len(w) > 2 && w[0] == '-' && w[1] != '-' && !strings.Contains(w, "=") && strings.ContainsAny(w, "wl") {
			return true
		}
	}
	return false
}
