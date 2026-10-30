// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package skills

import (
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/mcp/tools"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/playbook"
)

// Two promises about the text Downshift hands to agents, the installed skills
// and the worker playbooks:
//
//   - It names no model. Models come from the live `providers` tool, because
//     the harness decides what exists today and a literal goes stale.
//   - It names only MCP tools the server registers, in the form a client sees.

// modelFamily matches versioned model names the catalog may not list, such as
// "haiku-5-5", so a rename of an id cannot hide a literal.
var modelFamily = regexp.MustCompile(`^(claude-)?(haiku|sonnet|opus|fable|gpt|gemini|grok)-[0-9]`)

// mcpPrefixed is the name a client shows for a tool of this server.
var mcpPrefixed = regexp.MustCompile(`mcp__downshift__([a-z][a-z0-9_]*)`)

// mcpStale is a prefix form no client uses: a tool is `providers` or
// mcp__downshift__providers, never downshift_providers.
var mcpStale = regexp.MustCompile(`\bdownshift_[a-z][a-z0-9_]*`)

// mcpNamed catches a tool the prose introduces as "the MCP tool `name`".
var mcpNamed = regexp.MustCompile("MCP tools? `([a-z][a-z0-9_]*)`")

// tokens splits text into runs of identifier characters, with the dots and
// dashes at either end (sentence punctuation) removed.
func tokens(text string) []string {
	runs := strings.FieldsFunc(text, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.')
	})
	out := make([]string, 0, len(runs))
	for _, run := range runs {
		if tok := strings.Trim(run, ".-"); tok != "" {
			out = append(out, tok)
		}
	}
	return out
}

// modelLiterals returns the tokens of text that name a model: a catalog id, or
// a versioned family name.
func modelLiterals(text string, ids map[string]bool) []string {
	var found []string
	for _, tok := range tokens(text) {
		if ids[tok] || modelFamily.MatchString(tok) {
			found = append(found, tok)
		}
	}
	return found
}

// toolProblems returns every MCP tool mention in text that does not resolve to
// a registered tool.
func toolProblems(text string, registered map[string]bool) []string {
	var bad []string
	for _, re := range []*regexp.Regexp{mcpPrefixed, mcpNamed} {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			if !registered[m[1]] {
				bad = append(bad, m[0])
			}
		}
	}
	return append(bad, mcpStale.FindAllString(text, -1)...)
}

// registeredTools is the tool set `downshift mcp serve` registers
// (cmd/downshift/cmd_mcp.go joins the information and the worker tools).
func registeredTools(t *testing.T) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	for _, tl := range append(tools.Info(tools.Deps{}), tools.WorkTools(tools.Deps{})...) {
		names[tl.Name] = true
	}
	if len(names) == 0 {
		t.Fatal("the MCP server registers no tools")
	}
	return names
}

func catalogIDs(t *testing.T) map[string]bool {
	t.Helper()
	t.Setenv(paths.EnvStateDir, t.TempDir()) // the embedded catalog only, no user override
	ids := map[string]bool{}
	for _, e := range catalog.Load().Entries() {
		ids[e.ID] = true
	}
	if len(ids) == 0 {
		t.Fatal("the catalog lists no models")
	}
	return ids
}

// shippedTexts returns the text of every installed skill and every shipped
// worker playbook, keyed by where it came from.
func shippedTexts(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := fs.WalkDir(FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".md" {
			return err
		}
		b, err := FS.ReadFile(p)
		out["skill "+p] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	sets, err := playbook.Load(t.TempDir()) // an empty folder yields the shipped sets
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sets {
		out["playbook "+s.ID] = s.Text
	}
	return out
}

func TestShippedTextNamesNoModel(t *testing.T) {
	ids := catalogIDs(t)
	for from, text := range shippedTexts(t) {
		if found := modelLiterals(text, ids); len(found) > 0 {
			t.Errorf("%s names models %v; models come from the providers tool", from, found)
		}
	}
}

func TestShippedTextNamesOnlyRegisteredTools(t *testing.T) {
	registered := registeredTools(t)
	for from, text := range shippedTexts(t) {
		if bad := toolProblems(text, registered); len(bad) > 0 {
			t.Errorf("%s mentions MCP tools the server does not register: %v", from, bad)
		}
	}
}

// The checks are only worth having if they trip on the text this file was
// written for, the removed Universal Skill playbook.
func TestTextChecksTripOnTheRemovedUniversalSkill(t *testing.T) {
	tier := "downshift model tier to flash_lite / haiku-5-5"
	if got := modelLiterals(tier, map[string]bool{"flash_lite": true}); len(got) != 2 {
		t.Errorf("modelLiterals(%q) = %v, want flash_lite and haiku-5-5", tier, got)
	}
	coord := "query `downshift_providers`, `downshift_playbook`, and `downshift_status`"
	if got := toolProblems(coord, map[string]bool{"providers": true}); len(got) != 3 {
		t.Errorf("toolProblems(%q) = %v, want the three downshift_ names", coord, got)
	}
	named := "the Downshift MCP tool `providers`, then mcp__downshift__apply and mcp__downshift__steer"
	if got := toolProblems(named, map[string]bool{"providers": true, "apply": true}); len(got) != 1 {
		t.Errorf("toolProblems(%q) = %v, want only steer", named, got)
	}
}

// Short catalog ids such as "pro" and "flash" match whole tokens only.
func TestModelIDsMatchWholeTokensOnly(t *testing.T) {
	ids := map[string]bool{"pro": true, "flash": true}
	if got := modelLiterals("A magnum opus of prose, projects and flashes.", ids); len(got) != 0 {
		t.Errorf("modelLiterals found %v in plain prose", got)
	}
	if got := modelLiterals("Use flash, not pro.", ids); len(got) != 2 {
		t.Errorf("modelLiterals found %v, want flash and pro", got)
	}
}
