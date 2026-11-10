// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// A recursive package-manager run prefixes every line with the workspace and
// script ("packages/web test: PASS ..." or "packages/web test$ vitest run" in
// pnpm -r). The runner's own grammar sits behind the prefix, so
// the compressors read nothing until it is stripped.
var (
	reWorkspaceLine = lazyre.New(`^(\S+ test(?:[:.-][\w:.-]+)?)(?:: ?|\$ ?)(.*)$`)
	// reWorkspaceScript is the "name script$ command" line pnpm prints when a
	// workspace script starts; it plays the part of the "> command" banner.
	reWorkspaceScript = lazyre.New(`^\S+ test(?:[:.-][\w:.-]+)?\$ (\S.*)$`)
	reWorkspaceScope  = lazyre.New(`^Scope: .+$`)
)

// minWorkspaceLines is the fewest prefixed lines worth reading as a workspace run.
const minWorkspaceLines = 3

// stripWorkspacePrefix removes the prefix of a run whose every line carries
// the same one, and returns the text with its "> command" banner form and the
// prefix. A "Scope:" line and blank lines may stand unprefixed. Output with a
// second prefix, or any other unprefixed line, comes back unchanged with "".
func stripWorkspacePrefix(text []byte) ([]byte, string) {
	var out []string
	prefix, n := "", 0
	for _, line := range strings.Split(strings.TrimSuffix(string(text), "\n"), "\n") {
		if strings.TrimSpace(line) == "" || reWorkspaceScope.MatchString(line) {
			out = append(out, line)
			continue
		}
		m := reWorkspaceLine.FindStringSubmatch(line)
		if m == nil || prefix != "" && m[1] != prefix {
			return text, ""
		}
		prefix = m[1]
		n++
		body := m[2]
		if s := reWorkspaceScript.FindStringSubmatch(line); s != nil {
			body = "> " + s[1]
		}
		out = append(out, body)
	}
	if n < minWorkspaceLines {
		return text, ""
	}
	return []byte(strings.Join(out, "\n") + "\n"), prefix
}
