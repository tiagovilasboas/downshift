// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

func TestRun_SearchFormsCompactLikeGrepAndFind(t *testing.T) {
	cases := []struct {
		name   string
		bin    string
		argv   []string
		raw    string
		format compressor.Format
		family Family
	}{
		{"rg", "rg", []string{"rg", "-n", "func", "internal"}, grepRaw(), compressor.FormatGrep, FamilyRg},
		{"rg-files", "rg", []string{"rg", "--files", "internal"}, findRaw(), compressor.FormatFind, FamilyRgFiles},
		{"git-grep", "git", []string{"git", "grep", "-n", "func", "internal"}, grepRaw(), compressor.FormatGrep, FamilyGitGrep},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeBin(t, tc.bin, "cat <<'EOF'\n"+tc.raw+"EOF")
			r, out, _ := run(t, tc.argv, true)
			want := compressor.CompressCommandOutput([]byte(tc.raw), tc.format, 0).Output
			if r.ExitCode != 0 || r.Family != tc.family || !r.Compacted || out != string(want)+Marker(len(tc.raw), len(want)) {
				t.Fatalf("exit=%d family=%q compacted=%v out=%.200q", r.ExitCode, r.Family, r.Compacted, out)
			}
			if len(out) >= len(tc.raw) {
				t.Fatalf("output not smaller: %d >= %d", len(out), len(tc.raw))
			}
		})
	}
}
