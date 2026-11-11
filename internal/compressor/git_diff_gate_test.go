// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "testing"

// TestGitDiffUnsafeGate drives the gate directly: a danger word on a line the
// compressor keeps verbatim (change lines, headers, hunk headers, a git show
// commit block) or on an unchanged context line does not stop compaction; one
// on a stray line does, and so does a secret on a context line it may drop.
func TestGitDiffUnsafeGate(t *testing.T) {
	const head = "diff --git a/h.go b/h.go\n--- a/h.go\n+++ b/h.go\n@@ -1,3 +1,3 @@\n"
	tests := []struct {
		name string
		in   string
		safe bool
	}{
		{"context danger", head + " // error path\n-a\n+b\n c\n", true},
		{"removed danger", head + " x\n-return fail()\n+b\n c\n", true},
		{"added danger", head + " x\n-a\n+panic(1)\n c\n", true},
		{"stray error line", head + " x\n-a\n+b\n c\nerror: boom\n", false},
		{"stray warning line", head + " x\n-a\n+b\n c\nwarning: boom\n", false},
		{"indented line after hunk is not context", head + " x\n-a\n+b\n c\n  warning: late\n", false},
		{"hunk header danger", "diff --git a/h.go b/h.go\n--- a/h.go\n+++ b/h.go\n@@ -1,3 +1,3 @@ func onError()\n x\n-a\n+b\n c\n", true},
		{"commit message danger", "commit abc\n\n    fix failure\n\n" + head + " x\n-a\n+b\n c\n", false},
		{"omitted counts mean one line", "diff --git a/h.go b/h.go\n--- a/h.go\n+++ b/h.go\n@@ -1 +1 @@\n-a\n+b\n error late\n", false},
		{"secret line", head + " token = 1\n-a\n+b\n c\n", false},
		{"quoted secret key on context", head + " \"api_key\": \"abc\"\n-a\n+b\n c\n", false},
		{"prose that names tokens compacts", head + " tokens saved per harness\n-a\n+b\n c\n", true},
		{"identifier TokenCount compacts", head + " n := r.TokenCount\n-a\n+b\n c\n", true},
		{"secret on a change line is kept, so it compacts", head + " x\n-a\n+token = 1\n c\n", true},
		{"git show commit message danger", "commit 0123456789abcdef\nAuthor: a\n\n    fix failing test\n\n" + head + " x\n-a\n+b\n c\n", true},
		{"file named errors.go", "diff --git a/errors.go b/errors.go\n--- a/errors.go\n+++ b/errors.go\n@@ -1,3 +1,3 @@\n x\n-a\n+b\n c\n", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := gitDiffUnsafe([]byte(tc.in)); ok != tc.safe {
				t.Fatalf("safe=%v, want %v", ok, tc.safe)
			}
		})
	}
}
