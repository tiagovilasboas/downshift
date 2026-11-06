// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

// "stash@{0}: WIP on main: 1a2b3c4 commit subject" or
// "stash@{1}: On main: message".
var reGitStashEntry = regexp.MustCompile(`^stash@\{(\d+)\}: (.+)$`)

// isGitStashList reports whether every non-empty line is a stash entry.
func isGitStashList(input []byte) bool {
	any := false
	for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !reGitStashEntry.MatchString(line) {
			return false
		}
		any = true
	}
	return any
}

// compressGitStashList keeps one compact line per stash: "{index} <summary>",
// where the summary drops the redundant "WIP on <branch>:" / "On <branch>:"
// boilerplate but preserves the commit hint and subject.
func compressGitStashList(input []byte) ([]byte, bool, string) {
	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	var b strings.Builder
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := reGitStashEntry.FindStringSubmatch(line)
		if m == nil {
			return input, false, "git_stash_unrecognized_line_preserved"
		}
		idx, desc := m[1], strings.TrimSpace(m[2])
		// Drop the "WIP on <branch>: " / "On <branch>: " boilerplate prefix.
		if p := strings.Index(desc, ": "); p >= 0 {
			head := desc[:p]
			if strings.HasPrefix(head, "WIP on ") || strings.HasPrefix(head, "On ") {
				desc = strings.TrimSpace(desc[p+2:])
			}
		}
		fmt.Fprintf(&b, "{%s} %s\n", idx, desc)
		count++
	}
	if count == 0 {
		return input, false, "git_stash_empty"
	}
	out := b.String()
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("compressed git stash list (%d entries)", count)
}
