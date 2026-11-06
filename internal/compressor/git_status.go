// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reGitLogCommit  = regexp.MustCompile(`(?m)^commit\s+[0-9a-f]{40}`)
	reGitStatusHead = regexp.MustCompile(`(?m)^(?:On branch |HEAD detached at )`)
)

// compressGitStatus squelches a clean working tree, and only when every
// non-empty line is the branch line, an "up to date" tracking line or the
// clean marker. Ahead, behind, diverged, detached or any other line keeps the
// original output: an agent deciding whether to push needs that line.
func compressGitStatus(input []byte) ([]byte, bool, string) {
	s := string(input)
	if !strings.Contains(s, "nothing to commit, working tree clean") {
		return input, false, "git_status_has_changes_preserved"
	}
	branch := ""
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.TrimSpace(line) == "":
		case strings.HasPrefix(line, "On branch ") && branch == "":
			branch = line
		case strings.HasPrefix(line, "Your branch is up to date"):
		case line == "nothing to commit, working tree clean":
		default:
			return input, false, "git_status_extra_lines_preserved"
		}
	}
	if branch == "" {
		return input, false, "git_status_no_branch_preserved"
	}
	return []byte(fmt.Sprintf("%s (working tree clean)\n", branch)), true, "squelched clean working tree"
}
