// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package archtest

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// savedSum matches an addition that has a saved-tokens or saved-bytes field on
// either side: `a.WorkSaved + b`, `x += ev.SavedBytes`, `n + t.CacheSaved`.
var savedSum = regexp.MustCompile(`\b(WorkSaved|CacheSaved|SavedBytes)\b\s*\+|\+\s*[\w.\[\]()]*\b(WorkSaved|CacheSaved|SavedBytes)\b`)

// summingScan is where a surface could add up telemetry fields on its own.
var summingScan = []string{"internal/api", "internal/accounting", "cmd/downshift"}

// summingAllowed holds the files that may add saved fields. compute.go is the
// one Compute implementation. The others are legacy aggregations that no
// surface calls and A6 deletes; keep that list shrinking, never growing.
var summingAllowed = map[string]string{
	"internal/accounting/compute.go": "the Compute implementation (compactorTotals)",
	"internal/accounting/global.go":  "legacy BuildGlobalSummary, deleted by A6",
	"internal/accounting/harness.go": "legacy AggregateByHarness, deleted by A6",
	"internal/accounting/savings.go": "legacy CalculateSavings, deleted by A6",
}

// summingFiles returns the production files of dir that add saved fields,
// skipping comment-only lines.
func summingFiles(t *testing.T, dir string) []string {
	t.Helper()
	var hits []string
	for _, rel := range productionGoFiles(t, dir) {
		raw, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if code := strings.TrimSpace(line); !strings.HasPrefix(code, "//") && savedSum.MatchString(code) {
				hits = append(hits, rel)
				break
			}
		}
	}
	return hits
}

// TestNoSurfaceArithmetic: report, panel and API render Numbers; only Compute
// adds telemetry fields. A new file with `WorkSaved +` arithmetic fails; so does
// an allowlist entry that no longer matches (delete it).
func TestNoSurfaceArithmetic(t *testing.T) {
	found := map[string]bool{}
	for _, dir := range summingScan {
		for _, f := range summingFiles(t, dir) {
			found[f] = true
		}
	}
	var names []string
	for f := range found {
		names = append(names, f)
	}
	sort.Strings(names)
	for _, f := range names {
		if _, ok := summingAllowed[f]; !ok {
			t.Errorf("%s adds WorkSaved/CacheSaved/SavedBytes itself; take the figure from accounting.Numbers (or add it to accounting.Compute)", f)
		}
	}
	for f, why := range summingAllowed {
		if !found[f] {
			t.Errorf("allowlist entry %s (%s) no longer adds saved fields: remove it from summingAllowed", f, why)
		}
	}
}

func TestSavedSumPattern(t *testing.T) {
	for line, want := range map[string]bool{
		"total := a.WorkSaved + b.CacheSaved":         true,
		"c.SavedBytes += raw - emitted":               true,
		"n := base + ev.SavedBytes":                   true,
		"fmt.Println(n.Compactor.SavedBytes)":         false,
		"x := s.WorkSaved * 2":                        false,
		"Saved bytes: %d":                             false,
		"routing := saved.WorkSaved+saved.CacheSaved": true,
	} {
		if got := savedSum.MatchString(line); got != want {
			t.Errorf("savedSum(%q) = %v, want %v", line, got, want)
		}
	}
}
