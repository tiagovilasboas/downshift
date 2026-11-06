// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"regexp"
	"strings"
)

// Package-manager script banner. `npm test` and `pnpm test` print two lines
// before the runner starts, "> pkg@1.0.0 test [cwd]" and "> vitest run", each
// followed by a blank line. They say which script ran, which the command line
// already says, so a run that compacts drops them. Only the leading block is
// read as a banner: a "> " line after the runner started is test output.

var (
	// yarn 1 prints "yarn run v1.22.19" and "$ command" instead, and
	// "Done in 2.1s." at the end.
	reScriptBannerLine = regexp.MustCompile(`^(?:> \S.*|\$ \S.*|yarn run v\d+\.\d+\.\d+)$`)
	reYarnDoneLine     = regexp.MustCompile(`^Done in \d+(?:\.\d+)?s\.$`)
	// npm prints its own warnings around the banner ("npm warn ...", "npm
	// WARN ..." before npm 7). They are information, so they are kept.
	reNpmWarnLine = regexp.MustCompile(`^npm (?:warn|WARN) .+$`)
)

// dropScriptBanner returns input without its leading script banner and the
// blank lines around it; the npm warnings in that block stay, in order. Input
// with no banner is returned as it came.
func dropScriptBanner(input []byte) []byte {
	lines := strings.SplitAfter(string(input), "\n")
	var kept []string
	banner, i := 0, 0
	for ; i < len(lines); i++ {
		l := strings.TrimSuffix(lines[i], "\n")
		if reScriptBannerLine.MatchString(l) {
			banner++
		} else if reNpmWarnLine.MatchString(l) {
			kept = append(kept, lines[i])
		} else if strings.TrimSpace(l) != "" {
			break
		}
	}
	if banner == 0 {
		return input
	}
	rest := lines[i:]
	for n := len(rest); n > 0 && (strings.TrimSpace(rest[n-1]) == "" || reYarnDoneLine.MatchString(strings.TrimSpace(rest[n-1]))); n-- {
		if reYarnDoneLine.MatchString(strings.TrimSpace(rest[n-1])) {
			rest = append(rest[:n-1:n-1], rest[n:]...)
			break
		}
	}
	return []byte(strings.Join(kept, "") + strings.Join(rest, ""))
}
