// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift/internal/setup"
)

// securityAuditSkill is Cloudflare's security-audit skill (MIT), which the
// security agent loads in guidance mode when it is installed. Downshift never
// installs or bundles it; doctor only says whether it is there.
const (
	securityAuditSkill   = "security-audit"
	securityAuditInstall = "npx skills add https://github.com/cloudflare/security-audit-skill --skill security-audit --global"
)

// checkSecurityAuditSkill reports, for each harness setup knows a skills
// directory for and whose config directory exists, whether the skill is
// installed. Both answers are ok: the skill is optional. The directories come
// from setup.Targets, so no harness is named here.
func checkSecurityAuditSkill(rep *doctorReport, env doctorEnv) {
	if env.Home == "" {
		return
	}
	for _, t := range setup.Targets {
		if t.SkillsDir == "" || t.ConfigDir == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(env.Home, filepath.FromSlash(t.ConfigDir))); err != nil {
			continue
		}
		skill := filepath.Join(env.Home, filepath.FromSlash(t.SkillsDir), securityAuditSkill, "SKILL.md")
		detail := "optional, not installed: " + securityAuditInstall
		if _, err := os.Stat(skill); err == nil {
			detail = "found (security agent uses it in guidance mode)"
		}
		rep.add(doctorCheck{ID: "security skill", Harness: t.Name, Status: doctorOK, Detail: detail})
	}
}
