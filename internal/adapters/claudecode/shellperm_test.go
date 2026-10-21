// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBashRuleMatches(t *testing.T) {
	cases := []struct {
		rule, cmd string
		want      bool
	}{
		{"Bash", "go test ./...", true},
		{"Bash(*)", "git status", true},
		{"Bash(go test:*)", "go test ./...", true},
		{"Bash(go test:*)", "go test", true},
		{"Bash(go test:*)", "go testing", false},
		{"Bash(go test *)", "go test ./...", true},
		{"Bash(go test *)", "go test", true},
		{"Bash(git status)", "git status", true},
		{"Bash(git status)", "git status -s", false},
		{"Bash(go:*)", "go test ./...", true},
		{"Read(go test:*)", "go test ./...", false},
		{"Bash(go test:*", "go test ./...", false},
		{"Bash(npm run *)", "go test ./...", false},
	}
	for _, tc := range cases {
		if got := bashRuleMatches(tc.rule, tc.cmd); got != tc.want {
			t.Errorf("bashRuleMatches(%q, %q) = %v, want %v", tc.rule, tc.cmd, got, tc.want)
		}
	}
}

// A settings file is read up to a cap: a cloned repository cannot make every
// Bash call slurp a huge file, and an oversize file is an error (no rewrite).
func TestReadSettings_CapsTheFile(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "small.json")
	if err := os.WriteFile(small, []byte(`{"permissions":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if data, err := readSettings(small); err != nil || string(data) != `{"permissions":{}}` {
		t.Fatalf("small file: %q, %v", data, err)
	}
	big := filepath.Join(dir, "big.json")
	if err := os.WriteFile(big, []byte(strings.Repeat(" ", maxSettingsBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSettings(big); !errors.Is(err, errSettingsTooLarge) {
		t.Fatalf("oversize file error = %v, want errSettingsTooLarge", err)
	}
	exact := filepath.Join(dir, "exact.json")
	if err := os.WriteFile(exact, []byte(strings.Repeat(" ", maxSettingsBytes)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSettings(exact); err != nil {
		t.Fatalf("a file of exactly the cap must be read: %v", err)
	}
	if _, err := readSettings(filepath.Join(dir, "missing.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file error = %v, want ErrNotExist", err)
	}
}
