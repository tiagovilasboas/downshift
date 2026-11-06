// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"regexp"
	"strings"
)

// The install compressor cuts only lines it can name. Every line of an npm,
// pnpm, yarn or bun install transcript is classified as noise (cut, the
// summary says the same in fewer words), a finding (kept verbatim) or
// unknown. One unknown line, a warning and a deprecation notice among them,
// keeps the whole run raw.
var (
	reInstallNoise = regexp.MustCompile(`^(?:` +
		// npm
		`added \d+ packages?(?:, and audited \d+ packages?)? in [\d.]+m?s|` +
		`up to date(?:, audited \d+ packages?)?(?: in [\d.]+m?s)?|` +
		`\d+ packages? (?:are|is) looking for funding|  run .npm fund. for details|found 0 vulnerabilities|` +
		// pnpm
		`Lockfile is up to date, resolution step is skipped|Progress: resolved .*|Packages: \+\d+.*|` +
		`[+-]+|(?:dependencies|devDependencies|optionalDependencies):|[+-] \S+ \S+.*|` +
		`Done in [\d.]+m?s\.?|Scope: .*|` +
		// yarn classic
		`yarn (?:install|add) v[\d.]+|\[\d/\d\] \w[\w ]*\.\.\.|success Saved lockfile\.|` +
		`success Saved \d+ new dependenc(?:y|ies)\.|info No lockfile found\.|` +
		// bun
		`bun (?:install|add) v[\d.]+ \(\w+\)|\+ \S+@\S+|installed \S+@\S+.*|Saved lockfile|` +
		`\s*\d+ packages? installed \[[\d.]+m?s\]|Checked \d+ installs across \d+ packages \(no changes\) \[[\d.]+m?s\]|` +
		// every manager
		`)$`)

	// reInstallFinding lines are kept verbatim after the summary: they report
	// the vulnerabilities the install found and how to act on them.
	reInstallFinding = regexp.MustCompile(`^(?:\d+ (?:low|moderate|high|critical) severity vulnerabilit.*|` +
		`\d+ vulnerabilit(?:y|ies) \(.*\)|To address .*|  npm audit fix(?: --force)?|` +
		`Some issues need review.*|Run .npm audit. for details\.)$`)
)

// classifyInstall splits the lines of a transcript into the findings to keep.
// It reports false when any line is neither noise nor a finding.
func classifyInstall(input []byte) (findings []string, ok bool) {
	for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		switch {
		case line == "" || reInstallNoise.MatchString(line):
		case reInstallFinding.MatchString(line):
			findings = append(findings, line)
		default:
			return nil, false
		}
	}
	return findings, true
}
