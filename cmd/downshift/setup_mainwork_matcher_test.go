// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// The PostToolUse matcher that feeds the main-session work count is data
// derived from the capabilities: every read and search tool the harness lists
// is matched, and nothing else is.
func TestClaudeCodePostToolUseMainWorkMatcher(t *testing.T) {
	var want []string
	for tool, cat := range core.ClaudeCodeCaps.MainToolCategories {
		if cat == core.CategoryRead || cat == core.CategorySearch {
			want = append(want, tool)
		}
	}
	// NotebookRead is not a tool the matcher wires (it is rare); every other
	// read/search tool must be.
	var matcher string
	for _, tg := range setup.Targets {
		if tg.Name != "claude-code" {
			continue
		}
		for _, h := range tg.Hooks {
			if h.Event == "PostToolUse" && strings.Contains(h.Matcher, "Read") {
				matcher = h.Matcher
			}
		}
	}
	if matcher == "" {
		t.Fatal("setup wires no PostToolUse matcher for Read")
	}
	got := strings.Split(matcher, "|")
	for _, tool := range got {
		if !slices.Contains(want, tool) {
			t.Errorf("matcher names %s, which the capabilities do not list as read or search", tool)
		}
	}
	for _, tool := range want {
		if tool != "NotebookRead" && !slices.Contains(got, tool) {
			t.Errorf("capabilities list %s as read or search but the matcher omits it", tool)
		}
	}
}

// The plugin's hooks.json and setup must carry the same PostToolUse matchers.
func TestPluginAndSetupAgreeOnPostToolUseMatchers(t *testing.T) {
	cmds, err := parsePluginHooks(shippedPluginHooks(t))
	if err != nil {
		t.Fatal(err)
	}
	var plugin, wired []string
	for _, c := range cmds {
		if c.Event == "PostToolUse" {
			plugin = append(plugin, c.Matcher)
		}
	}
	for _, tg := range setup.Targets {
		if tg.Name != "claude-code" {
			continue
		}
		for _, h := range tg.Hooks {
			if h.Event == "PostToolUse" {
				wired = append(wired, h.Matcher)
			}
		}
	}
	slices.Sort(plugin)
	slices.Sort(wired)
	if !slices.Equal(plugin, wired) {
		t.Errorf("PostToolUse matchers differ: plugin %v, setup %v", plugin, wired)
	}
	if !slices.Contains(wired, "Read|Grep|Glob") {
		t.Errorf("setup lacks the Read|Grep|Glob matcher: %v", wired)
	}
}
