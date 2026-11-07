// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reGitLogCommit        = regexp.MustCompile(`(?m)^commit\s+[0-9a-f]{40}`)
	reGitStatusHead       = regexp.MustCompile(`(?m)^(?:On branch |HEAD detached at )`)
	reGitStatusAdvice     = regexp.MustCompile(`^\s+\(use "git [^"]+" .*\)$`)
	reGitStatusChangeLine = regexp.MustCompile(`^\t(?:modified|new file|deleted|renamed|copied|typechange):\s+\S`)
)

// compressGitStatus squelches a clean working tree to one line, or strips git's
// "(use "git ...")" advice, "up to date" boilerplate and status footers from a
// dirty working tree while keeping the branch line, ahead/behind tracking,
// section headers and every file line verbatim. Detached HEAD, conflicts,
// rebase/merge states and diverged branches stay raw.
func compressGitStatus(input []byte) ([]byte, bool, string) {
	s := string(input)
	if !strings.Contains(s, "nothing to commit, working tree clean") {
		return compressDirtyGitStatus(input, s)
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

func compressDirtyGitStatus(input []byte, s string) ([]byte, bool, string) {
	var kept []string
	branch := ""
	section := ""
	fileLines := 0
	for _, line := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		switch {
		case strings.TrimSpace(line) == "":
		case strings.HasPrefix(line, "On branch ") && branch == "" && len(kept) == 0:
			branch = line
			kept = append(kept, line)
		case strings.HasPrefix(line, "Your branch is up to date"):
		case strings.HasPrefix(line, "Your branch is ahead of "),
			strings.HasPrefix(line, "Your branch is behind "):
			kept = append(kept, line)
		case reGitStatusAdvice.MatchString(line):
		case line == "Changes to be committed:", line == "Changes not staged for commit:":
			section = "changes"
			kept = append(kept, line)
		case line == "Untracked files:", line == "Ignored files:":
			section = "untracked"
			kept = append(kept, line)
		case section == "changes" && reGitStatusChangeLine.MatchString(line):
			fileLines++
			kept = append(kept, line)
		case section == "untracked" && strings.HasPrefix(line, "\t") && strings.TrimSpace(line) != "":
			fileLines++
			kept = append(kept, line)
		case strings.HasPrefix(line, "no changes added to commit"),
			strings.HasPrefix(line, "nothing added to commit but untracked files present"):
		default:
			return input, false, "git_status_extra_lines_preserved"
		}
	}
	if branch == "" || fileLines == 0 {
		return input, false, "git_status_has_changes_preserved"
	}
	out := []byte(strings.Join(kept, "\n") + "\n")
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, "compacted dirty git status"
}
