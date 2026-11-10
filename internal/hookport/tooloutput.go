// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package hookport

import "github.com/tiagovilasboas/downshift-ai/internal/core"

// ToolOutput is one finished tool call, read from a harness's post-run hook
// payload into harness-free fields. The adapter fills it (ParseToolOutputFunc)
// and writes it back into its own wire answer (ReplaceToolOutputFunc); the
// shared post-run policy (internal/postrun) only reads and changes the text
// fields. A Has* flag is false when the harness's result does not carry that
// field: the policy never adds one, and the adapter never writes it.
type ToolOutput struct {
	// Category is the tool's category from the harness capabilities
	// (core.HarnessCapabilities.CategoryOf).
	Category core.ToolCategory
	// Session is the harness's raw session id, or "". The runner hashes it
	// before anything is logged.
	Session string
	// Command is the shell command that ran (shell tools only).
	Command string
	// Stdout and Stderr are a shell tool's two streams.
	Stdout, Stderr       string
	HasStdout, HasStderr bool
	// Content is a search tool's matches as text, one per line.
	Content    string
	HasContent bool
	// Paths is a search tool's list of files.
	Paths    []string
	HasPaths bool
	// ExitCode is the shell exit code when the result carries one, else -1.
	ExitCode int
	// Opaque is set when the result must never be replaced: an interrupted
	// run, an image, or a shape the adapter does not know.
	Opaque bool
	// Native is the harness's own result object. Only the adapter reads it,
	// to keep every sibling field when it writes the answer.
	Native map[string]any
}

// ParseToolOutputFunc reads a post-run hook payload. ok is false when the
// payload is not a finished tool call the adapter can read.
type ParseToolOutputFunc func(raw []byte) (out ToolOutput, ok bool)

// ReplaceToolOutputFunc returns the hook answer that makes the harness show
// out instead of the tool's own result. prev is the answer the hook would
// have printed otherwise (the honor runner's), so nothing it carries is lost.
type ReplaceToolOutputFunc func(prev any, out ToolOutput) any
