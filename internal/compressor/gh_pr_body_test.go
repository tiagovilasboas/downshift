// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
)

func ghPrViewSample(bodyLines int) string {
	var b strings.Builder
	b.WriteString("title:\tfeat: add retry to the uploader\nstate:\tOPEN\nauthor:\ttiagovilasboas\n" +
		"labels:\tenhancement, uploader\nassignees:\t\nreviewers:\talice (Approved), bob (Requested)\n" +
		"projects:\t\nmilestone:\t\nnumber:\t812\nurl:\thttps://github.com/acme/app/pull/812\n" +
		"additions:\t412\ndeletions:\t18\nauto-merge:\tdisabled\n--\n")
	for i := 0; i < bodyLines; i++ {
		fmt.Fprintf(&b, "description line %d of the change\n", i)
	}
	return b.String()
}

func TestCompressGhPrViewKeepsFacts(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantHas   []string
		wantNot   []string
		wantApply bool
	}{
		{
			name:      "short body is kept whole with url, labels and reviewers",
			input:     ghPrViewSample(5),
			wantHas:   []string{"url: https://github.com/acme/app/pull/812", "labels: enhancement, uploader", "reviewers: alice (Approved), bob (Requested)", "description line 4 of the change"},
			wantNot:   []string{"additions", "[…"},
			wantApply: true,
		},
		{
			name:      "long body is capped with an explicit marker",
			input:     ghPrViewSample(100),
			wantHas:   []string{"url: https://github.com/acme/app/pull/812", "description line 29 of the change", "[… 70 lines cut]"},
			wantNot:   []string{"description line 30 of the change"},
			wantApply: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, applied, reason := compressGhPrView([]byte(tc.input))
			if applied != tc.wantApply {
				t.Fatalf("applied=%v reason=%q", applied, reason)
			}
			for _, w := range tc.wantHas {
				if !strings.Contains(string(out), w) {
					t.Errorf("missing %q in:\n%s", w, out)
				}
			}
			for _, w := range tc.wantNot {
				if strings.Contains(string(out), w) {
					t.Errorf("unexpected %q in:\n%s", w, out)
				}
			}
		})
	}
}
