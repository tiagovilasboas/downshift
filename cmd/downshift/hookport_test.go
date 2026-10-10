// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/hookport"
)

func TestRunHonorNilCursorPort(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))

	port := hookport.Port{ID: "cursor"}
	var rc int
	var stderr string
	stdout := captureStdout(func() {
		stderr = captureStderr(func() {
			rc = runHonor(strings.NewReader(`{}`), port, nil)
		})
	})

	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if stdout != "{}\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "{}\n")
	}
	if stderr != "downshift: honor unobserved for cursor\n" {
		t.Fatalf("stderr = %q, want %q", stderr, "downshift: honor unobserved for cursor\n")
	}
}

func TestRunUsageNilCursorPort(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))

	port := hookport.Port{ID: "cursor"}
	var rc int
	var stderr string
	stdout := captureStdout(func() {
		stderr = captureStderr(func() {
			rc = runUsage(strings.NewReader(`{}`), port, nil)
		})
	})

	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if stderr != "downshift: usage unobserved for cursor\n" {
		t.Fatalf("stderr = %q, want %q", stderr, "downshift: usage unobserved for cursor\n")
	}
}

func TestPortForClaudeCodeRegisteredCursorNil(t *testing.T) {
	claude := portFor("claude-code")
	if claude.ID != "claude-code" {
		t.Fatalf("claude-code ID = %q, want %q", claude.ID, "claude-code")
	}
	if claude.Honor == nil {
		t.Fatal("claude-code Honor is nil")
	}
	if claude.Usage == nil {
		t.Fatal("claude-code Usage is nil")
	}

	cursor := portFor("cursor")
	if cursor.ID != "cursor" {
		t.Fatalf("cursor ID = %q, want %q", cursor.ID, "cursor")
	}
	if cursor.Honor != nil {
		t.Fatal("cursor Honor is set")
	}
	if cursor.Usage != nil {
		t.Fatal("cursor Usage is set")
	}
}
