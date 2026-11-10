// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// FormatNpmAudit marks the human report of `npm audit`. npm exits non-zero
// when it finds a vulnerability at or above --audit-level, so a run is
// compacted only when it exits 0: a clean report, or findings below the level
// the user set (for example `--audit-level=critical`).
const FormatNpmAudit Format = "npm_audit"

var (
	reNpmAuditHeader   = lazyre.New(`(?m)^# npm audit report$`)
	reNpmAuditPackage  = lazyre.New(`^\S.*  \S.*$`)
	reNpmAuditSeverity = lazyre.New(`^Severity: (low|moderate|high|critical)$`)
	reNpmAuditBody     = lazyre.New("^(?:.+ - https://github\\.com/advisories/GHSA-[\\w-]+|" +
		"Depends on vulnerable versions of \\S+|fix available via .+|No fix available|Will install .+|\\s*node_modules/\\S+)$")
	reNpmAuditTail = lazyre.New("^(?:\\d+ vulnerabilit(?:y|ies) \\(.*\\)|found \\d+ vulnerabilities|" +
		"To address .*:|  npm audit fix(?: --force)?|Some issues need review.*|)$")
)

// isNpmAudit reports whether input carries an npm audit report.
func isNpmAudit(input []byte) bool { return reNpmAuditHeader.Match(input) }

// auditBlock is one vulnerable package of the report.
type auditBlock struct {
	name, severity string
	lines          []string
}

// compressNpmAudit lists the low and moderate packages by name and keeps every
// high and critical block, advisory titles and URLs included, verbatim. A line
// the report grammar does not know (an npm warn, an error, a new field) keeps
// the whole report raw.
func compressNpmAudit(input []byte) ([]byte, bool, string) {
	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	blocks, tail, ok := parseNpmAudit(lines)
	if !ok {
		return input, false, "npm_audit_unrecognized_line_preserved"
	}
	counts := map[string][]string{}
	var kept []string
	for _, b := range blocks {
		if b.severity == "high" || b.severity == "critical" {
			kept = append(kept, b.lines...)
			kept = append(kept, "")
			continue
		}
		counts[b.severity] = append(counts[b.severity], b.name)
	}
	var out []string
	out = append(out, "# npm audit report", "")
	for _, sev := range []string{"moderate", "low"} {
		if names := counts[sev]; len(names) > 0 {
			out = append(out, fmt.Sprintf("%d %s (advisories cut): %s", len(names), sev, strings.Join(names, ", ")))
		}
	}
	if len(counts) > 0 {
		out = append(out, "")
	}
	out = append(out, kept...)
	out = append(out, tail...)
	res := strings.Join(out, "\n") + "\n"
	if len(res) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(res), true, "npm audit summarised by severity"
}

// parseNpmAudit splits the report into package blocks and the lines after the
// last block. It reports false on any line outside the grammar.
func parseNpmAudit(lines []string) (blocks []auditBlock, tail []string, ok bool) {
	i := 0
	if len(lines) == 0 || lines[0] != "# npm audit report" {
		return nil, nil, false
	}
	for i = 1; i < len(lines); {
		if lines[i] == "" {
			i++
			continue
		}
		if i+1 >= len(lines) || !reNpmAuditPackage.MatchString(lines[i]) {
			break
		}
		m := reNpmAuditSeverity.FindStringSubmatch(lines[i+1])
		if m == nil {
			break
		}
		b := auditBlock{name: strings.Fields(lines[i])[0], severity: m[1], lines: []string{lines[i], lines[i+1]}}
		for i += 2; i < len(lines) && lines[i] != ""; i++ {
			if !reNpmAuditBody.MatchString(lines[i]) {
				return nil, nil, false
			}
			b.lines = append(b.lines, lines[i])
		}
		blocks = append(blocks, b)
	}
	for ; i < len(lines); i++ {
		if !reNpmAuditTail.MatchString(lines[i]) {
			return nil, nil, false
		}
		tail = append(tail, lines[i])
	}
	return blocks, tail, true
}
