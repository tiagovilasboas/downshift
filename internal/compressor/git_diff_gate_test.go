// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "testing"

// TestGitDiffUnsafeGate drives the gate directly: only a danger word on an
// unchanged context line, counted from the hunk header, is exempt.
func TestGitDiffUnsafeGate(t *testing.T) {
	const head = "diff --git a/h.go b/h.go\n--- a/h.go\n+++ b/h.go\n@@ -1,3 +1,3 @@\n"
	tests := []struct {
		name string
		in   string
		safe bool
	}{
		{"context danger", head + " // error path\n-a\n+b\n c\n", true},
		{"removed danger", head + " x\n-return fail()\n+b\n c\n", false},
		{"added danger", head + " x\n-a\n+panic(1)\n c\n", false},
		{"stray error line", head + " x\n-a\n+b\n c\nerror: boom\n", false},
		{"stray warning line", head + " x\n-a\n+b\n c\nwarning: boom\n", false},
		{"indented line after hunk is not context", head + " x\n-a\n+b\n c\n  warning: late\n", false},
		{"hunk header danger", "diff --git a/h.go b/h.go\n--- a/h.go\n+++ b/h.go\n@@ -1,3 +1,3 @@ func onError()\n x\n-a\n+b\n c\n", false},
		{"commit message danger", "commit abc\n\n    fix failure\n\n" + head + " x\n-a\n+b\n c\n", false},
		{"omitted counts mean one line", "diff --git a/h.go b/h.go\n--- a/h.go\n+++ b/h.go\n@@ -1 +1 @@\n-a\n+b\n error late\n", false},
		{"secret line", head + " token = 1\n-a\n+b\n c\n", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := gitDiffUnsafe([]byte(tc.in)); ok != tc.safe {
				t.Fatalf("safe=%v, want %v", ok, tc.safe)
			}
		})
	}
}
