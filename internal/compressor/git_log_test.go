// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

const logHash = "4f2a9c1e0b7d6a5c4b3a2918f7e6d5c4b3a29180"

func logBlock(head, extra, msg string) string {
	return "commit " + logHash + head + "\n" + extra + "Author: Dev <d@example.com>\nDate:   Mon Oct 19 10:00:00 2026 +0000\n\n" + msg
}

func TestGitLogCompaction(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string // "" means the input must come back raw
		reason  string
		applied bool
	}{
		{
			name: "plain subject only", applied: true,
			in:   logBlock("", "", "    fix: parser\n") + "\n" + logBlock("", "", "    feat: cache\n"),
			want: "4f2a9c1 fix: parser\n4f2a9c1 feat: cache\n",
		},
		{
			name: "decorations kept", applied: true,
			in:   logBlock(" (HEAD -> main, tag: v1.0, origin/main)", "", "    release\n"),
			want: "4f2a9c1 (HEAD -> main, tag: v1.0, origin/main) release\n",
		},
		{
			name: "merge line and body dropped", applied: true,
			in:   logBlock("", "Merge: 1a2b3c4 5d6e7f8\n", "    Merge PR 12\n\n    body line\n    more body\n"),
			want: "4f2a9c1 Merge PR 12\n",
		},
		{
			// Owner decision: the body is dropped even when it mentions
			// fail/panic; the subject stays verbatim.
			name: "body with danger words dropped, subject verbatim", applied: true,
			in:   logBlock("", "", "    fix: flaky test\n\n    The old run printed:\n    FAIL\n    panic: boom\n"),
			want: "4f2a9c1 fix: flaky test\n",
		},
		{
			name: "subject with danger word kept verbatim", applied: true,
			in:   logBlock("", "", "    fix: panic on empty input\n\n    details here to be dropped\n"),
			want: "4f2a9c1 fix: panic on empty input\n",
		},
		{
			name:   "gpg lines preserved",
			in:     "commit " + logHash + "\ngpg: Signature made Mon\nAuthor: D <d@e.com>\nDate:   x\n\n    s\n\n",
			reason: "git_log_unrecognized_structure_preserved",
		},
		{
			name:   "stat block preserved",
			in:     logBlock("", "", "    s\n\n") + " a.go | 2 +-\n 1 file changed, 1 insertion(+)\n\n",
			reason: "git_log_unrecognized_structure_preserved",
		},
		{
			name:   "patch preserved",
			in:     logBlock("", "", "    s\n\n") + "diff --git a/a b/a\n",
			reason: "git_log_unrecognized_structure_preserved",
		},
		{
			name:   "missing author preserved",
			in:     "commit " + logHash + "\nDate:   x\n\n    s\n\n",
			reason: "git_log_unrecognized_structure_preserved",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := compressor.CompressExit([]byte(tc.in), compressor.ModeSafe, 0)
			if tc.applied {
				if !res.Applied || string(res.Output) != tc.want {
					t.Fatalf("applied=%v reason=%q\n got: %q\nwant: %q", res.Applied, res.Reason, res.Output, tc.want)
				}
				return
			}
			if res.Applied || string(res.Output) != tc.in {
				t.Fatalf("want raw, got applied=%v reason=%q output=%q", res.Applied, res.Reason, res.Output)
			}
			if tc.reason != "" && res.Reason != tc.reason {
				t.Fatalf("reason=%q, want %q", res.Reason, tc.reason)
			}
		})
	}
}

func TestGitLogNonZeroExitStaysRaw(t *testing.T) {
	in := logBlock("", "", "    fix: parser\n\n    body\n")
	res := compressor.CompressExit([]byte(in), compressor.ModeSafe, 128)
	if res.Applied || string(res.Output) != in {
		t.Fatalf("non-zero exit changed output: applied=%v reason=%q", res.Applied, res.Reason)
	}
}
