// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

func TestCompress_GitStatus_CleanTree(t *testing.T) {
	in := []byte("On branch main\nYour branch is up to date.\nnothing to commit, working tree clean\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for clean git status")
	}
	if !strings.Contains(string(res.Output), "working tree clean") {
		t.Errorf("expected clean working tree summary, got %s", string(res.Output))
	}
}

func TestCompress_GitStatus_WithChanges_Preserved(t *testing.T) {
	in := []byte("On branch main\nChanges not staged for commit:\n  modified: foo.go\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if res.Applied {
		t.Errorf("status with actual changes should be preserved")
	}
	if !bytes.Equal(res.Output, in) {
		t.Errorf("output must be preserved")
	}
}

func TestCompress_GitStatus_KeepsTrackingAndDetached(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		applied bool
	}{
		{"up to date", "On branch main\nYour branch is up to date with 'origin/main'.\n\nnothing to commit, working tree clean\n", true},
		{"no upstream", "On branch main\nnothing to commit, working tree clean\n", true},
		{"ahead", "On branch main\nYour branch is ahead of 'origin/main' by 3 commits.\n  (use \"git push\" to publish your local commits)\n\nnothing to commit, working tree clean\n", false},
		{"behind", "On branch main\nYour branch is behind 'origin/main' by 1 commit, and can be fast-forwarded.\n\nnothing to commit, working tree clean\n", false},
		{"detached", "HEAD detached at abc1234\nnothing to commit, working tree clean\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.CompressExit(in, compressor.ModeSafe, 0)
			if res.Applied != tc.applied {
				t.Fatalf("applied = %v, want %v (%s)", res.Applied, tc.applied, res.Reason)
			}
			if !tc.applied && !bytes.Equal(res.Output, in) {
				t.Fatal("output must be byte-identical when not applied")
			}
		})
	}
}

func TestCompress_GitStatus_DirtyStripsAdviceAndKeepsChanges(t *testing.T) {
	in := []byte("On branch feat/compact-bench\n" +
		"Your branch is ahead of 'origin/feat/compact-bench' by 2 commits.\n" +
		"  (use \"git push\" to publish your local commits)\n" +
		"\n" +
		"Changes to be committed:\n" +
		"  (use \"git restore --staged <file>...\" to unstage)\n" +
		"\tnew file:   internal/compressor/bench_compare_test.go\n" +
		"\trenamed:    internal/compressor/old_name.go -> internal/compressor/new_name.go\n" +
		"\n" +
		"Changes not staged for commit:\n" +
		"  (use \"git add <file>...\" to update what will be committed)\n" +
		"  (use \"git restore <file>...\" to discard changes in working directory)\n" +
		"\tmodified:   internal/compressor/compressor.go\n" +
		"\tmodified:   internal/execwrap/policy.go\n" +
		"\n" +
		"Untracked files:\n" +
		"  (use \"git add <file>...\" to include in what will be committed)\n" +
		"\tscripts/bench/scratch.txt\n" +
		"\n" +
		"no changes added to commit (use \"git add\" and/or \"git commit -a\")\n")
	want := "On branch feat/compact-bench\n" +
		"Your branch is ahead of 'origin/feat/compact-bench' by 2 commits.\n" +
		"Changes to be committed:\n" +
		"\tnew file:   internal/compressor/bench_compare_test.go\n" +
		"\trenamed:    internal/compressor/old_name.go -> internal/compressor/new_name.go\n" +
		"Changes not staged for commit:\n" +
		"\tmodified:   internal/compressor/compressor.go\n" +
		"\tmodified:   internal/execwrap/policy.go\n" +
		"Untracked files:\n" +
		"\tscripts/bench/scratch.txt\n"

	res := compressor.CompressExit(in, compressor.ModeSafe, 0)
	if !res.Applied {
		t.Fatalf("expected dirty git status to compact, reason=%s", res.Reason)
	}
	if string(res.Output) != want {
		t.Fatalf("unexpected compacted output:\ngot:\n%s\nwant:\n%s", string(res.Output), want)
	}
}
