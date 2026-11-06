// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// ToolCategory groups a main-session tool call by the kind of work it does.
// It is a code, never the call's text: the main-session work log stores the
// category and nothing the call carried.
type ToolCategory string

const (
	CategoryRead   ToolCategory = "read"
	CategorySearch ToolCategory = "search"
	CategoryShell  ToolCategory = "shell"
	CategoryEdit   ToolCategory = "edit"
	CategoryOther  ToolCategory = "other"
)

// MechanicalCategories are the categories a small-tier subagent could do.
var MechanicalCategories = []ToolCategory{CategoryRead, CategorySearch, CategoryShell}

// claudeCodeToolCategories names Claude Code's built-in tools.
var claudeCodeToolCategories = map[string]ToolCategory{
	"Read": CategoryRead, "NotebookRead": CategoryRead,
	"Grep": CategorySearch, "Glob": CategorySearch,
	"Bash": CategoryShell,
	"Edit": CategoryEdit, "MultiEdit": CategoryEdit, "Write": CategoryEdit, "NotebookEdit": CategoryEdit,
}

// MainWorkObserved reports whether the harness delivers a per-tool event for
// the main session.
func (c HarnessCapabilities) MainWorkObserved() bool { return c.MainToolEvent != "" }

// CategoryOf returns the category of a tool the harness named, or
// CategoryOther when the harness does not list it.
func (c HarnessCapabilities) CategoryOf(tool string) ToolCategory {
	if cat, ok := c.MainToolCategories[tool]; ok {
		return cat
	}
	return CategoryOther
}

// IsMechanical reports whether the category is read, search or shell work.
func (c ToolCategory) IsMechanical() bool {
	for _, m := range MechanicalCategories {
		if c == m {
			return true
		}
	}
	return false
}
