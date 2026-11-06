// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// Independent evaluation prompts: manually labeled by requested scope before
// observing candidate predictions. The three language variants of each
// scenario are CORRELATED, not 90 independent outcome observations.
// These are tier-label evaluations, not evidence of real model task success.
type precisionEvalCase struct {
	ID       string `json:"id"`
	Scenario string `json:"scenario"`
	Language string `json:"language"`
	Domain   string `json:"domain"`
	WantTier string `json:"want_tier"`
	Prompt   string `json:"prompt"`
}

type precisionStats struct {
	Total   int
	Correct int
	Under   int
	Over    int
	ByTier  map[string]map[string]int
	ByLang  map[string]map[string]int
}

func tierOrdinal(tier string) int {
	switch tier {
	case "small":
		return 0
	case "mid":
		return 1
	case "frontier":
		return 2
	default:
		return -1
	}
}

func newPrecisionStats() precisionStats {
	return precisionStats{
		ByTier: make(map[string]map[string]int),
		ByLang: make(map[string]map[string]int),
	}
}

func (m *precisionStats) observe(want, got, language string) {
	m.Total++
	if want == got {
		m.Correct++
	} else if tierOrdinal(got) < tierOrdinal(want) {
		m.Under++
	} else {
		m.Over++
	}
	if m.ByTier[want] == nil {
		m.ByTier[want] = make(map[string]int)
	}
	m.ByTier[want][got]++
	if m.ByLang[language] == nil {
		m.ByLang[language] = make(map[string]int)
	}
	m.ByLang[language][got]++
}

func (m precisionStats) summary() string {
	if m.Total == 0 {
		return "no cases"
	}
	return fmt.Sprintf("accuracy=%.1f%% (%d/%d), under=%d, over=%d",
		100*float64(m.Correct)/float64(m.Total), m.Correct, m.Total, m.Under, m.Over)
}

func precisionEvalCases(t *testing.T) []precisionEvalCase {
	t.Helper()
	data, err := os.ReadFile("testdata/domain_precision/dev_tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []precisionEvalCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 90 {
		t.Fatalf("expected 90 independent-label examples, got %d", len(cases))
	}
	ids := make(map[string]bool)
	scenarios := make(map[string]map[string]bool)
	tiers := make(map[string]int)
	for _, c := range cases {
		if c.ID == "" || c.Prompt == "" || c.Domain == "" || ids[c.ID] || tierOrdinal(c.WantTier) < 0 {
			t.Fatalf("invalid eval record %+v", c)
		}
		if c.Language != "en" && c.Language != "pt-BR" && c.Language != "mixed" {
			t.Fatalf("%s unsupported language: %s", c.ID, c.Language)
		}
		ids[c.ID] = true
		tiers[c.WantTier]++
		if scenarios[c.Scenario] == nil {
			scenarios[c.Scenario] = make(map[string]bool)
		}
		scenarios[c.Scenario][c.Language] = true
	}
	if len(scenarios) != 30 || tiers["small"] != 30 || tiers["mid"] != 30 || tiers["frontier"] != 30 {
		t.Fatalf("unbalanced tier dataset: scenarios=%d tiers=%v", len(scenarios), tiers)
	}
	for scenario, langs := range scenarios {
		if len(langs) != 3 || !langs["en"] || !langs["pt-BR"] || !langs["mixed"] {
			t.Errorf("%s missing multilingual variants: %+v", scenario, langs)
		}
	}
	return cases
}

// The complete production router is evaluated under three deterministic
// configurations with the SAME held labels. This test prints honest numbers
// and counterexamples; it does not assume an improvement or silently rewrite
// labels. One test corpus cannot establish real-world generalization.
func TestDomainPrecisionComparison(t *testing.T) {
	cases := precisionEvalCases(t)
	configs := []struct {
		name         string
		domain       string
		interactions string
	}{
		{"baseline_without_domains", "0", "0"},
		{"96_domains_without_pairs", "1", "0"},
		{"96_domains_with_pairs", "1", "1"},
	}
	t.Setenv("DOWNSHIFT_DOMAIN_DEDUP", "1")
	type scored struct {
		stats precisionStats
		pred  map[string]string
	}
	out := make(map[string]scored)
	for _, cfg := range configs {
		t.Setenv("DOWNSHIFT_DOMAIN_SIGNALS", cfg.domain)
		t.Setenv("DOWNSHIFT_DOMAIN_INTERACTIONS", cfg.interactions)
		st := newPrecisionStats()
		pred := make(map[string]string)
		for _, c := range cases {
			got := core.Route(c.Prompt, "claude-code", "", cat).Tier.String()
			st.observe(c.WantTier, got, c.Language)
			pred[c.ID] = got
		}
		out[cfg.name] = scored{stats: st, pred: pred}
		t.Logf("%s: %s", cfg.name, st.summary())
		for _, want := range []string{"small", "mid", "frontier"} {
			t.Logf("  want %-8s -> small=%d mid=%d frontier=%d",
				want, st.ByTier[want]["small"], st.ByTier[want]["mid"], st.ByTier[want]["frontier"])
		}
		domainParity := 0
		byScenario := make(map[string]map[string]string)
		for _, c := range cases {
			if byScenario[c.Scenario] == nil {
				byScenario[c.Scenario] = make(map[string]string)
			}
			byScenario[c.Scenario][c.Language] = pred[c.ID]
		}
		for _, byLang := range byScenario {
			if byLang["en"] == byLang["pt-BR"] && byLang["en"] == byLang["mixed"] {
				domainParity++
			}
		}
		t.Logf("  EN/PT-BR/mixed tier parity: %d/%d scenarios", domainParity, len(byScenario))
	}
	comparePrecisionPredictions(t, cases, out["baseline_without_domains"].pred, out["96_domains_without_pairs"].pred, "domains vs baseline")
	comparePrecisionPredictions(t, cases, out["96_domains_without_pairs"].pred, out["96_domains_with_pairs"].pred, "pairwise interactions vs domains")
	comparePrecisionPredictions(t, cases, out["baseline_without_domains"].pred, out["96_domains_with_pairs"].pred, "full candidate vs baseline")
	// Regression guard: expanded vocabulary should improve or retain tier
	// agreement on the fixed multilingual label set and avoid extra under-routing.
	baseline, candidate := out["baseline_without_domains"].stats, out["96_domains_with_pairs"].stats
	if candidate.Correct < baseline.Correct || candidate.Under > baseline.Under {
		t.Errorf("native domains regressed: baseline=%s candidate=%s", baseline.summary(), candidate.summary())
	}
}

// Each changed prediction is reported, including regressions. Pairwise
// McNemar significance should be assessed at SCENARIO rather than translated
// prompt level, to avoid overstating effective sample size.
func comparePrecisionPredictions(t *testing.T, cases []precisionEvalCase, before, after map[string]string, comparison string) {
	t.Helper()
	var improved, regressed, changed int
	var examples []string
	for _, c := range cases {
		b, a := before[c.ID], after[c.ID]
		if b == a {
			continue
		}
		changed++
		beforeCorrect, afterCorrect := b == c.WantTier, a == c.WantTier
		if !beforeCorrect && afterCorrect {
			improved++
		} else if beforeCorrect && !afterCorrect {
			regressed++
		}
		examples = append(examples, fmt.Sprintf("%s %s want=%s before=%s after=%s", c.ID, c.Domain, c.WantTier, b, a))
	}
	sort.Strings(examples)
	t.Logf("%s: %d changed, %d fixed, %d regressed, net %d/%d", comparison, changed, improved, regressed, improved-regressed, len(cases))
	for _, example := range examples {
		t.Logf("  %s", example)
	}
}

// The existing 200-case corpus remains separate from the new 90-case set:
// report paired impact on both so improvements on specialty prompts cannot
// hide accidental general-purpose regressions.
func TestDomainPrecisionLegacyCorpus(t *testing.T) {
	corpus := loadPromptCorpus(t)
	t.Setenv("DOWNSHIFT_DOMAIN_DEDUP", "1")
	configs := []struct{ name, domains, pairs string }{
		{"baseline", "0", "0"},
		{"domains_only", "1", "0"},
		{"candidate", "1", "1"},
	}
	for _, config := range configs {
		t.Setenv("DOWNSHIFT_DOMAIN_SIGNALS", config.domains)
		t.Setenv("DOWNSHIFT_DOMAIN_INTERACTIONS", config.pairs)
		stats := newPrecisionStats()
		files := make([]string, 0, len(corpus))
		for name := range corpus {
			files = append(files, name)
		}
		sort.Strings(files)
		for _, filename := range files {
			for _, row := range corpus[filename] {
				got := core.Route(row.Prompt, "claude-code", "", cat).Tier.String()
				stats.observe(row.WantTier, got, filename)
			}
		}
		t.Logf("%s on original corpus: %s", config.name, stats.summary())
	}
}

func BenchmarkDomainPrecisionRoute(b *testing.B) {
	b.Setenv("DOWNSHIFT_DOMAIN_SIGNALS", "1")
	b.Setenv("DOWNSHIFT_DOMAIN_DEDUP", "1")
	b.Setenv("DOWNSHIFT_DOMAIN_INTERACTIONS", "1")
	casesPath := filepath.Join("testdata", "domain_precision", "dev_tasks.json")
	data, err := os.ReadFile(casesPath)
	if err != nil {
		b.Fatal(err)
	}
	var cases []precisionEvalCase
	if err := json.Unmarshal(data, &cases); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = core.Route(cases[i%len(cases)].Prompt, "claude-code", "", cat)
	}
}
