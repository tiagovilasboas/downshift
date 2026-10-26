// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package limits

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The harness matrix is the evidence source for harness status. These tests
// read its "Status per harness" table and hold the registry to it, so a
// status cannot be raised here without the matrix showing the evidence.
var matrixPath = filepath.Join("..", "..", "docs", "reference", "harness-matrix.md")

// matrixVerdicts are the status words a matrix row may carry.
var matrixVerdicts = map[string]bool{
	"Completed":    true,
	"Functional":   true,
	"Beta":         true,
	"Experimental": true,
}

// matrixParityHarnesses are the harnesses whose registry entries must match
// their matrix rows.
var matrixParityHarnesses = []string{"claude-code", "cursor", "codex", "antigravity", "opencode", "kirocrew"}

// standardFeatures is every feature the registry must describe for a harness.
var standardFeatures = []string{
	FeatureModelRewrite,
	FeatureTokenCounts,
	FeatureCompaction,
	FeatureMidSessionRouting,
	FeatureSavingsEstimation,
}

// readMatrixVerdicts returns harness id to status word, from the rows of the
// "Status per harness" table. Other tables in that section are skipped
// because their second column is not a status word.
func readMatrixVerdicts(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatalf("read harness matrix: %v", err)
	}
	verdicts := map[string]string{}
	inSection := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "## ") {
			inSection = strings.TrimSpace(line) == "## Status per harness"
			continue
		}
		if !inSection || !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 3 {
			continue
		}
		id, verdict := strings.TrimSpace(cells[1]), strings.TrimSpace(cells[2])
		if !matrixVerdicts[verdict] {
			continue
		}
		if _, dup := verdicts[id]; dup {
			t.Fatalf("harness matrix has two status rows for %s", id)
		}
		verdicts[id] = verdict
	}
	return verdicts
}

// TestRegistryMatchesHarnessMatrix checks the registry against the matrix.
// The matrix defines Completed as: rewrite observed honored, usage and child
// end recorded. So only a Completed harness may claim full support, and every
// other harness must have no usage source (token_counts none), as the Child
// end table shows for codex, cursor, antigravity, opencode and kirocrew.
func TestRegistryMatchesHarnessMatrix(t *testing.T) {
	verdicts := readMatrixVerdicts(t)

	for id := range All() {
		if _, ok := verdicts[id]; !ok {
			t.Errorf("registry harness %q has no row in the harness matrix", id)
		}
	}

	for _, id := range matrixParityHarnesses {
		t.Run(id, func(t *testing.T) {
			verdict, ok := verdicts[id]
			if !ok {
				t.Fatalf("harness matrix has no status row for %s", id)
			}
			lims := ForHarness(id)
			if lims == nil {
				t.Fatalf("registry has no entry for %s", id)
			}
			for _, feature := range standardFeatures {
				lim := lims.GetLimitation(feature)
				if lim == nil {
					t.Errorf("no limitation for %s", feature)
					continue
				}
				if verdict == "Completed" {
					if lim.Status != StatusFull {
						t.Errorf("matrix says %s, so %s = %q; want %q", verdict, feature, lim.Status, StatusFull)
					}
				} else if lim.Status == StatusFull {
					t.Errorf("matrix says %s, so %s must not be %q", verdict, feature, StatusFull)
				}
			}
			if verdict == "Completed" {
				return
			}
			if lim := lims.GetLimitation(FeatureTokenCounts); lim != nil && lim.Status != StatusNone {
				t.Errorf("matrix says %s with no usage source, so %s = %q; want %q", verdict, FeatureTokenCounts, lim.Status, StatusNone)
			}
		})
	}
}
