// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompressCypress_PassingRunKeepsSummaryFacts(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "corpus", "cypress_pass_clean.txt"))
	if err != nil {
		t.Fatal(err)
	}

	out, applied, reason := compressCypress(in)
	if !applied {
		t.Fatalf("expected compression, reason=%q", reason)
	}
	for _, fact := range []string{"24 passing", "1 pending", "0 skipped", "accounting.cy.ts", "8 seconds"} {
		if !strings.Contains(string(out), fact) {
			t.Errorf("summary lost %q: %s", fact, out)
		}
	}
	if len(out) >= len(in) {
		t.Fatalf("output did not shrink: %d -> %d", len(in), len(out))
	}
}

func TestCompressCypressPassesThroughPublicCompressor(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "corpus", "cypress_pass_clean.txt"))
	if err != nil {
		t.Fatal(err)
	}
	res := CompressExit(in, ModeSafe, 0)
	if !res.Applied || res.Format != FormatCypress {
		t.Fatalf("passing Cypress output was not compacted: applied=%v format=%q reason=%q", res.Applied, res.Format, res.Reason)
	}
}

func TestCompressCypressPreservesErrorOutputWithPassingSummary(t *testing.T) {
	in := []byte("(Run Starting)\nBrowser: Electron\nError: application console error\n(Run Finished)\n│ Tests: 1\n│ Passing: 1\n│ Failing: 0\n│ Pending: 0\n│ Skipped: 0\n│ Duration: 1 second\n")
	res := CompressExit(in, ModeSafe, 0)
	if res.Applied || res.Format != FormatCypress || !bytes.Equal(res.Output, in) {
		t.Fatalf("error output must be preserved: applied=%v format=%q reason=%q output=%q", res.Applied, res.Format, res.Reason, res.Output)
	}
}

func TestCompressCypress_FailurePreservesActionableOutput(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "corpus", "cypress_fail_actionable.txt"))
	if err != nil {
		t.Fatal(err)
	}

	out, applied, _ := compressCypress(in)
	if applied || !bytes.Equal(out, in) {
		t.Fatal("failing run must be preserved byte-identically")
	}
}

func TestCompressCypress_AmbiguousOutputPreserved(t *testing.T) {
	cases := []string{
		"(Run Finished)\n│ Tests: 1\n│ Passing: 1\n",
		"(Run Finished)\n│ Tests: 2\n│ Passing: 2\n│ Failing: 0\n│ Pending: 0\n│ Skipped: 0\n│ Tests: 2\n",
		"(Run Finished)\n│ Tests: 2\n│ Passing: 1\n│ Failing: 0\n│ Pending: 0\n│ Skipped: 0\n",
	}
	for i, input := range cases {
		in := []byte(input)
		out, applied, _ := compressCypress(in)
		if applied || !bytes.Equal(out, in) {
			t.Errorf("case %d should be preserved, applied=%v output=%q", i, applied, out)
		}
	}
}
