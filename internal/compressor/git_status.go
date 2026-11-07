// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	reGitLogCommit            = regexp.MustCompile(`(?m)^commit\s+[0-9a-f]{40}`)
	reGitStatusHead           = regexp.MustCompile(`(?m)^(?:On branch |HEAD detached at )`)
	reGitStatusAdvice         = regexp.MustCompile(`^\s+\(use "git [^"]+" .*\)$`)
	reGitStatusChangeLine     = regexp.MustCompile(`^\t(modified|new file|deleted|renamed|copied|typechange):\s+(.+)$`)
	reGitStatusBranch         = regexp.MustCompile(`^On branch (\S+)`)
	reGitStatusUpToDate       = regexp.MustCompile(`^Your branch is up to date with '([^']+)'`)
	reGitStatusAhead          = regexp.MustCompile(`^Your branch is ahead of '([^']+)' by (\d+) commit`)
	reGitStatusBehind         = regexp.MustCompile(`^Your branch is behind '([^']+)' by (\d+) commit`)
	reGitStatusDiverged       = regexp.MustCompile(`^Your branch and '([^']+)' have diverged`)
	reGitStatusDivergedDetail = regexp.MustCompile(`^and have (\d+) and (\d+) different commits each`)
)

type gitStatusEntry struct {
	staged   byte
	unstaged byte
	path     string
}

// compressGitStatus squelches a clean working tree to one line, or compacts a
// dirty working tree to a short status format: branch line with tracking and
// ahead/behind counts, followed by XY status code rows for changed files.
// Detached HEAD, conflicts, and rebase/merge states stay raw.
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
	branch := ""
	upstream := ""
	ahead := 0
	behind := 0
	section := ""
	var entries []gitStatusEntry
	entryIndex := make(map[string]int)

	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.TrimSpace(line) == "":
		case strings.HasPrefix(line, "On branch ") && branch == "" && len(entries) == 0:
			m := reGitStatusBranch.FindStringSubmatch(line)
			if m == nil {
				return input, false, "git_status_extra_lines_preserved"
			}
			branch = m[1]
		case reGitStatusUpToDate.MatchString(line):
			m := reGitStatusUpToDate.FindStringSubmatch(line)
			upstream = m[1]
		case reGitStatusAhead.MatchString(line):
			m := reGitStatusAhead.FindStringSubmatch(line)
			upstream = m[1]
			ahead, _ = strconv.Atoi(m[2])
		case reGitStatusBehind.MatchString(line):
			m := reGitStatusBehind.FindStringSubmatch(line)
			upstream = m[1]
			behind, _ = strconv.Atoi(m[2])
		case reGitStatusDiverged.MatchString(line):
			m := reGitStatusDiverged.FindStringSubmatch(line)
			upstream = m[1]
			if i+1 < len(lines) && reGitStatusDivergedDetail.MatchString(lines[i+1]) {
				i++
				dm := reGitStatusDivergedDetail.FindStringSubmatch(lines[i])
				ahead, _ = strconv.Atoi(dm[1])
				behind, _ = strconv.Atoi(dm[2])
			}
		case reGitStatusAdvice.MatchString(line):
		case line == "Changes to be committed:":
			section = "staged"
		case line == "Changes not staged for commit:":
			section = "unstaged"
		case line == "Untracked files:", line == "Ignored files:":
			section = "untracked"
		case section == "staged" && reGitStatusChangeLine.MatchString(line):
			m := reGitStatusChangeLine.FindStringSubmatch(line)
			code := gitStatusCode(m[1])
			if code == 0 {
				return input, false, "git_status_extra_lines_preserved"
			}
			entries = append(entries, gitStatusEntry{staged: code, unstaged: ' ', path: m[2]})
			entryIndex[m[2]] = len(entries) - 1
		case section == "unstaged" && reGitStatusChangeLine.MatchString(line):
			m := reGitStatusChangeLine.FindStringSubmatch(line)
			code := gitStatusCode(m[1])
			if code == 0 {
				return input, false, "git_status_extra_lines_preserved"
			}
			path := m[2]
			if idx, exists := entryIndex[path]; exists {
				entries[idx].unstaged = code
			} else {
				entries = append(entries, gitStatusEntry{staged: ' ', unstaged: code, path: path})
				entryIndex[path] = len(entries) - 1
			}
		case section == "untracked" && strings.HasPrefix(line, "\t") && strings.TrimSpace(line) != "":
			path := strings.TrimSpace(line)
			entries = append(entries, gitStatusEntry{staged: '?', unstaged: '?', path: path})
		case strings.HasPrefix(line, "no changes added to commit"),
			strings.HasPrefix(line, "nothing added to commit but untracked files present"):
		default:
			return input, false, "git_status_extra_lines_preserved"
		}
	}
	if branch == "" || len(entries) == 0 {
		return input, false, "git_status_has_changes_preserved"
	}
	var b strings.Builder
	b.WriteString(formatGitBranchHeader(branch, upstream, ahead, behind))
	b.WriteByte('\n')
	for _, e := range entries {
		b.WriteByte(e.staged)
		b.WriteByte(e.unstaged)
		b.WriteByte(' ')
		b.WriteString(e.path)
		b.WriteByte('\n')
	}
	out := []byte(b.String())
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, "compacted dirty git status"
}

func gitStatusCode(changeType string) byte {
	switch changeType {
	case "new file":
		return 'A'
	case "modified":
		return 'M'
	case "deleted":
		return 'D'
	case "renamed":
		return 'R'
	case "copied":
		return 'C'
	case "typechange":
		return 'T'
	default:
		return 0
	}
}

func formatGitBranchHeader(branch, upstream string, ahead, behind int) string {
	if upstream == "" {
		return "* " + branch
	}
	tracking := "* " + branch + "..." + upstream
	switch {
	case ahead > 0 && behind > 0:
		return fmt.Sprintf("%s [ahead %d, behind %d]", tracking, ahead, behind)
	case ahead > 0:
		return fmt.Sprintf("%s [ahead %d]", tracking, ahead)
	case behind > 0:
		return fmt.Sprintf("%s [behind %d]", tracking, behind)
	default:
		return tracking
	}
}
