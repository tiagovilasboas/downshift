// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import "testing"

func TestBashRuleMatches(t *testing.T) {
	cases := []struct {
		rule, cmd string
		wide      bool
		want      bool
	}{
		{"Bash", "go test ./...", false, true},
		{"Bash(*)", "git status", false, true},
		{"Bash(go test:*)", "go test ./...", false, true},
		{"Bash(go test:*)", "go test", false, true},
		{"Bash(go test:*)", "go testing", false, false},
		{"Bash(go test *)", "go test ./...", false, true},
		{"Bash(go test *)", "go test", false, false},
		{"Bash(go test *)", "go test", true, true},
		{"Bash(git status)", "git status", false, true},
		{"Bash(git status)", "git status -s", false, false},
		{"Bash(go:*)", "go test ./...", false, true},
		{"Read(go test:*)", "go test ./...", false, false},
		{"Bash(go test:*", "go test ./...", false, false},
		{"Bash(npm run *)", "go test ./...", false, false},
	}
	for _, tc := range cases {
		if got := bashRuleMatches(tc.rule, tc.cmd, tc.wide); got != tc.want {
			t.Errorf("bashRuleMatches(%q, %q, wide=%v) = %v, want %v", tc.rule, tc.cmd, tc.wide, got, tc.want)
		}
	}
}
