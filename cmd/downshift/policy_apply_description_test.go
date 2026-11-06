// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// splitAgent returns the frontmatter values by key, and the text after it.
func splitAgent(t *testing.T, raw string) (front map[string]string, body string) {
	t.Helper()
	lines := strings.Split(raw, "\n")
	if lines[0] != "---" {
		t.Fatalf("no frontmatter:\n%s", raw)
	}
	end := frontmatterEnd(lines)
	if end < 0 {
		t.Fatalf("frontmatter never closes:\n%s", raw)
	}
	front = map[string]string{}
	for _, l := range lines[1:end] {
		k, v, _ := strings.Cut(l, ": ")
		front[k] = v
	}
	return front, strings.Join(lines[end+1:], "\n")
}

// A generated agent file has exactly name, description and model, and a body.
// The description round-trips for any text, because it is written as a quoted scalar.
func TestPolicyApply_GeneratedAgentCarriesDescriptionAndBody(t *testing.T) {
	cases := []struct{ name, description string }{
		{"plain", "Reads code and reports findings"},
		{"colon and hash", "Reviews: auth # flows"},
		{"double quotes", `Says "no" to risky diffs`},
		{"multiline", "Line one\nline two"},
		{"leading dash", "- not a list item"},
		{"unicode", "Revisão de código"},
		{"surrounding blanks trimmed", "  Padded  "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, home := setupPolicyApplyEnv(t, routingPolicy(profileJSON("agent", "small", c.description)))
			if code, _, stderr := runApply(t, dir, home); code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			front, body := splitAgent(t, readTestFile(t, filepath.Join(dir, ".claude", "agents", "agent.md")))

			var keys []string
			for k := range front {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if strings.Join(keys, ",") != "description,model,name" {
				t.Errorf("frontmatter keys = %v, want description, model, name", keys)
			}
			if front["name"] != "agent" || front["model"] != tierModel(core.TierSmall) {
				t.Errorf("name/model = %q/%q", front["name"], front["model"])
			}
			var got string
			if err := json.Unmarshal([]byte(front["description"]), &got); err != nil {
				t.Fatalf("description is not a quoted scalar: %q (%v)", front["description"], err)
			}
			want := strings.TrimSpace(c.description)
			if got != want {
				t.Errorf("description = %q, want %q", got, want)
			}
			if strings.TrimSpace(body) != want {
				t.Errorf("body = %q, want the description %q", body, want)
			}
		})
	}
}

// A profile with no description cannot create a file. Nothing is written, the
// message names the profile, and the other profiles wait with it.
func TestPolicyApply_RefusesToCreateAgentWithoutDescription(t *testing.T) {
	cases := []struct {
		name    string
		profile string
		dryRun  bool
	}{
		{"key absent", profileJSON("scout", "small", ""), false},
		{"blank value", profileJSON("scout", "small", "   \n\t"), false},
		{"dry run refuses too", profileJSON("scout", "small", ""), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, home := setupPolicyApplyEnv(t, routingPolicy(profileJSON("fine", "mid", "Has one"), c.profile))
			code, stdout, stderr := runApply(t, dir, home, func(o *policyApplyOptions) { o.DryRun = c.dryRun })
			if code != 1 {
				t.Fatalf("exit %d, want 1\nstdout: %s", code, stdout)
			}
			for _, want := range []string{`profile "scout"`, "description", "scout.md", "nothing was written"} {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr lacks %q:\n%s", want, stderr)
				}
			}
			if strings.Contains(stderr, `profile "fine"`) {
				t.Errorf("the profile that has a description is blamed:\n%s", stderr)
			}
			if !missing(t, filepath.Join(dir, ".claude")) {
				t.Error("something was written although one profile was refused")
			}
		})
	}
}

// A description is needed only where the file would get new frontmatter. An
// existing agent keeps its own frontmatter, so the profile may leave it out.
func TestPolicyApply_DescriptionOnlyNeededForNewFrontmatter(t *testing.T) {
	const bare = "Review the diff and list problems.\n"
	cases := []struct {
		name        string
		existing    string // "" means the file does not exist
		description string
		wantCode    int
		wantFile    string // expected content, "" when the file must stay as it was
	}{
		{
			name:     "frontmatter present, no description in profile",
			existing: "---\nname: a\ndescription: mine\nmodel: old\n---\nbody\n",
			wantCode: 0,
			wantFile: "---\nname: a\ndescription: mine\nmodel: MODEL\n---\nbody\n",
		},
		{
			name:     "frontmatter present without model gets one",
			existing: "---\nname: a\ndescription: mine\n---\nbody\n",
			wantCode: 0,
			wantFile: "---\nname: a\ndescription: mine\nmodel: MODEL\n---\nbody\n",
		},
		{
			name:        "no frontmatter, description given, text kept as the body",
			existing:    bare,
			description: "Reviews diffs",
			wantCode:    0,
			wantFile:    "---\nname: a\ndescription: \"Reviews diffs\"\nmodel: MODEL\n---\n\n" + bare,
		},
		{
			name:     "no frontmatter, no description: refused, file untouched",
			existing: bare,
			wantCode: 1,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, home := setupPolicyApplyEnv(t, routingPolicy(profileJSON("a", "small", c.description)))
			path := filepath.Join(dir, ".claude", "agents", "a.md")
			writeTestFile(t, path, c.existing, 0o644)

			code, _, stderr := runApply(t, dir, home)
			if code != c.wantCode {
				t.Fatalf("exit %d, want %d: %s", code, c.wantCode, stderr)
			}
			want := c.existing
			if c.wantFile != "" {
				want = strings.ReplaceAll(c.wantFile, "MODEL", tierModel(core.TierSmall))
			}
			if got := readTestFile(t, path); got != want {
				t.Errorf("file =\n%q\nwant\n%q", got, want)
			}
		})
	}
}
