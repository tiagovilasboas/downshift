// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

// Routing quality non-regression gate (offline).
//
// It measures the tier the classifier recommends (ClassifyWithSemantic, then
// TierOf) against hand labels on the two committed label sets: the 200-prompt
// corpus (testdata/corpus) and the 90-prompt domain precision set
// (testdata/domain_precision). Labels were written without running the router.
// This is label agreement, not task success.
//
// testdata/quality/baseline.json records, per set, the agreement count and the
// under-routed count (recommended tier below the label) measured at a named
// commit, plus a digest of the prompts and labels. The test recomputes both
// counts and fails when agreement falls, or under-routing rises, by more than
// tolerance_pp percentage points, or when the prompts or labels no longer match
// the digest. A loss, or a change to a set, therefore needs an edit to
// baseline.json in the same reviewed diff; -update-stability cannot absorb it.
//
// To re-measure after an intended change, run the command recorded in the
// baseline with -v and copy the logged sets. Never lower a value to hide a loss.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

const qualityBaselineFile = "testdata/quality/baseline.json"

// qualityScore is one set's measurement. Digest covers the ids, prompts and
// want_tier labels, so editing a label to raise a score changes the digest.
type qualityScore struct {
	Cases       int    `json:"cases"`
	TierCorrect int    `json:"tier_correct"`
	UnderRouted int    `json:"under_routed"`
	Digest      string `json:"digest_sha256"`
}

type qualityBaseline struct {
	MeasuredAtCommit string                  `json:"measured_at_commit"`
	Command          string                  `json:"command"`
	TolerancePP      float64                 `json:"tolerance_pp"`
	Sets             map[string]qualityScore `json:"sets"`
}

type labelledPrompt struct{ id, prompt, wantTier string }

// scoreQuality runs the classifier alone over rows and counts tier agreement
// and under-routing with the same bookkeeping as the precision evaluation.
func scoreQuality(rows []labelledPrompt) qualityScore {
	stats := newPrecisionStats()
	digest := sha256.New()
	for _, r := range rows {
		got := classifiedRoute(r.prompt, "claude-code", "", cat).Tier.String()
		stats.observe(r.wantTier, got, "")
		fmt.Fprintf(digest, "%s\t%s\t%s\n", r.id, r.wantTier, r.prompt)
	}
	return qualityScore{
		Cases:       stats.Total,
		TierCorrect: stats.Correct,
		UnderRouted: stats.Under,
		Digest:      hex.EncodeToString(digest.Sum(nil)),
	}
}

// qualityRegressions lists how got falls short of base; empty means the gate
// holds. Counts are compared as percentage points of the set, so one tolerance
// serves sets of different size. Different cases are not comparable.
func qualityRegressions(name string, base, got qualityScore, tolerancePP float64) []string {
	const eps = 1e-9
	if got.Digest != base.Digest || got.Cases != base.Cases {
		return []string{fmt.Sprintf("%s: prompts or labels changed (%d cases, digest %.12s; baseline %d, %.12s): re-measure and update the baseline in the same reviewed change",
			name, got.Cases, got.Digest, base.Cases, base.Digest)}
	}
	var out []string
	if drop := 100 * float64(base.TierCorrect-got.TierCorrect) / float64(base.Cases); drop > tolerancePP+eps {
		out = append(out, fmt.Sprintf("%s: tier agreement fell %.2fpp (%d to %d of %d), tolerance %.2fpp",
			name, drop, base.TierCorrect, got.TierCorrect, base.Cases, tolerancePP))
	}
	if rise := 100 * float64(got.UnderRouted-base.UnderRouted) / float64(base.Cases); rise > tolerancePP+eps {
		out = append(out, fmt.Sprintf("%s: under-routing rose %.2fpp (%d to %d of %d), tolerance %.2fpp",
			name, rise, base.UnderRouted, got.UnderRouted, base.Cases, tolerancePP))
	}
	return out
}

func corpusRows(t *testing.T) []labelledPrompt {
	t.Helper()
	files, _ := filepath.Glob("testdata/corpus/*.json")
	if len(files) == 0 {
		t.Fatal("testdata/corpus is missing: the quality gate must not skip")
	}
	corpus := loadPromptCorpus(t)
	names := make([]string, 0, len(corpus))
	for name := range corpus {
		names = append(names, name)
	}
	sort.Strings(names)
	var rows []labelledPrompt
	for _, name := range names {
		for _, c := range corpus[name] {
			rows = append(rows, labelledPrompt{name + "/" + c.ID, c.Prompt, c.WantTier})
		}
	}
	return rows
}

func precisionRows(t *testing.T) []labelledPrompt {
	t.Helper()
	var rows []labelledPrompt
	for _, c := range precisionEvalCases(t) {
		rows = append(rows, labelledPrompt{c.ID, c.Prompt, c.WantTier})
	}
	return rows
}

func loadQualityBaseline(t *testing.T) qualityBaseline {
	t.Helper()
	raw, err := os.ReadFile(qualityBaselineFile)
	if err != nil {
		t.Fatalf("%v: the quality gate needs its recorded baseline", err)
	}
	var b qualityBaseline
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("%s: %v", qualityBaselineFile, err)
	}
	if b.MeasuredAtCommit == "" || b.Command == "" || b.TolerancePP < 0 || len(b.Sets) == 0 {
		t.Fatalf("%s: measured_at_commit, command, tolerance_pp >= 0 and sets are required", qualityBaselineFile)
	}
	for name, s := range b.Sets {
		if s.Cases <= 0 || s.TierCorrect < 0 || s.UnderRouted < 0 || s.TierCorrect > s.Cases || s.Digest == "" {
			t.Fatalf("%s: set %q is malformed: %+v", qualityBaselineFile, name, s)
		}
	}
	return b
}

func TestRoutingQualityBaseline(t *testing.T) {
	// Production defaults: an empty value leaves every knob at its default.
	for _, knob := range []string{
		"DOWNSHIFT_SEMANTIC_BOOST", "DOWNSHIFT_MINILM",
		"DOWNSHIFT_DOMAIN_SIGNALS", "DOWNSHIFT_DOMAIN_INTERACTIONS", "DOWNSHIFT_DOMAIN_DEDUP",
	} {
		t.Setenv(knob, "")
	}
	got := map[string]qualityScore{
		"corpus_200":          scoreQuality(corpusRows(t)),
		"domain_precision_90": scoreQuality(precisionRows(t)),
	}
	measured, _ := json.MarshalIndent(got, "", "  ")
	t.Logf("measured sets:\n%s", measured)

	base := loadQualityBaseline(t)
	for name, g := range got {
		b, ok := base.Sets[name]
		if !ok {
			t.Errorf("%s: no recorded baseline", name)
			continue
		}
		for _, msg := range qualityRegressions(name, b, g, base.TolerancePP) {
			t.Error(msg)
		}
		if g.Digest == b.Digest && (g.TierCorrect > b.TierCorrect || g.UnderRouted < b.UnderRouted) {
			t.Logf("%s improved on the baseline (%d/%d correct, %d under-routed); raise the recorded values in the same change", name, g.TierCorrect, g.Cases, g.UnderRouted)
		}
	}
	for name := range base.Sets {
		if _, ok := got[name]; !ok {
			t.Errorf("%s: recorded in the baseline but not measured", name)
		}
	}
}
